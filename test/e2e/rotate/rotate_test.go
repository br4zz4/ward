//go:build e2e

package rotate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

var bin string

func TestMain(m *testing.M) {
	b, err := testutil.BuildBin()
	if err != nil {
		panic(err)
	}
	bin = b
	code := m.Run()
	os.Remove(b)
	os.Exit(code)
}

// wardProject initialises a ward project in dir and sets a secret.
// The vault name matches the dir basename (ward init convention).
func wardProject(t *testing.T, dir string) {
	t.Helper()
	vault := filepath.Base(dir)
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatalf("ward init failed in %s", dir)
	}
	if _, stderr, code := testutil.Run(t, bin, dir, "set", vault+":main.token", "secret123"); code != 0 {
		t.Fatalf("ward set failed (vault=%s): %s", vault, stderr)
	}
}

// vaultName returns the ward vault name for dir (basename convention).
func vaultName(dir string) string {
	return filepath.Base(dir)
}

func TestRotateKey_secret_readable_after_rotation(t *testing.T) {
	// arrange
	dir := t.TempDir()
	wardProject(t, dir)

	// act
	stdout, stderr, code := testutil.Run(t, bin, dir, "rotate-key")

	// assert: command succeeds
	if code != 0 {
		t.Fatalf("rotate-key exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// assert: secret is still readable with the new key
	out, _, _ := testutil.Run(t, bin, dir, "get", vaultName(dir)+":main.token")
	if !testutil.Contains(out, "secret123") {
		t.Errorf("expected secret123 after rotation, got: %q", out)
	}
}

func TestRotateKey_creates_key_backup(t *testing.T) {
	// arrange
	dir := t.TempDir()
	wardProject(t, dir)
	originalKey, err := os.ReadFile(filepath.Join(dir, ".ward", vaultName(dir)+".key"))
	if err != nil {
		t.Fatal(err)
	}

	// act
	stdout, stderr, code := testutil.Run(t, bin, dir, "rotate-key")
	if code != 0 {
		t.Fatalf("rotate-key exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// assert: backup file exists with original key content
	entries, err := os.ReadDir(filepath.Join(dir, ".ward"))
	if err != nil {
		t.Fatal(err)
	}
	var bkpContent []byte
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".key.bkp-") {
			bkpContent, err = os.ReadFile(filepath.Join(dir, ".ward", e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if bkpContent == nil {
		t.Fatal("expected .key.bkp-<timestamp> to exist after rotation")
	}
	if string(bkpContent) != string(originalKey) {
		t.Fatal("backup file does not match original key")
	}
}

func TestRotateKey_new_key_differs_from_old(t *testing.T) {
	// arrange
	dir := t.TempDir()
	wardProject(t, dir)
	keyPath := filepath.Join(dir, ".ward", vaultName(dir)+".key")
	originalKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	// act
	_, _, code := testutil.Run(t, bin, dir, "rotate-key")
	if code != 0 {
		t.Fatalf("rotate-key exit %d", code)
	}

	// assert: key file changed
	newKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(newKey) == string(originalKey) {
		t.Fatal("expected key to change after rotation")
	}
}

func TestRotateKey_output_shows_backup_path(t *testing.T) {
	// arrange
	dir := t.TempDir()
	wardProject(t, dir)

	// act
	stdout, stderr, code := testutil.Run(t, bin, dir, "rotate-key")
	if code != 0 {
		t.Fatalf("rotate-key exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	// assert: stdout mentions the backup path
	stripped := testutil.StripANSI(stdout)
	if !testutil.Contains(stripped, "Key rotated successfully") {
		t.Errorf("expected success message, got: %q", stripped)
	}
	if !testutil.Contains(stripped, ".key.bkp-") {
		t.Errorf("expected backup path in output, got: %q", stripped)
	}
}

func TestRotateKey_no_staging_files_remain(t *testing.T) {
	// arrange
	dir := t.TempDir()
	wardProject(t, dir)

	// act
	_, _, code := testutil.Run(t, bin, dir, "rotate-key")
	if code != 0 {
		t.Fatalf("rotate-key exit %d", code)
	}

	// assert: no .ward.new files left behind
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".ward.new") {
			t.Errorf("unexpected staging file: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRotateKey_with_file_secret(t *testing.T) {
	// arrange: init project and add a file secret
	dir := t.TempDir()
	wardProject(t, dir)

	jsonFile := filepath.Join(dir, "service-account.json")
	if err := os.WriteFile(jsonFile, []byte(`{"type":"service_account"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := testutil.Run(t, bin, dir, "file", "add", jsonFile, vaultName(dir)); code != 0 {
		t.Fatalf("ward file add failed: %s", stderr)
	}

	// act
	_, _, code := testutil.Run(t, bin, dir, "rotate-key")
	if code != 0 {
		t.Fatalf("rotate-key exit %d", code)
	}

	// assert: file secret is still extractable
	dest := t.TempDir()
	_, _, code = testutil.Run(t, bin, dir, "file", "extract", "service-account.json", dest)
	if code != 0 {
		t.Fatalf("file extract failed after rotation, exit %d", code)
	}
	extracted, err := os.ReadFile(filepath.Join(dest, "service-account.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(extracted), "service_account") {
		t.Errorf("expected original content, got: %s", string(extracted))
	}
}
