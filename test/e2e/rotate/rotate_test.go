//go:build e2e

package rotate_test

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	wardage "github.com/br4zz4/ward/internal/age"
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
	originalKey, err := os.ReadFile(filepath.Join(dir, ".ward", vaultName(dir)+"::default.key"))
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
		if strings.HasPrefix(e.Name(), vaultName(dir)+"::default.key.bkp-") {
			bkpContent, err = os.ReadFile(filepath.Join(dir, ".ward", e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if bkpContent == nil {
		t.Fatal("expected <keyfile>.bkp-<timestamp> to exist after rotation")
	}
	if string(bkpContent) != string(originalKey) {
		t.Fatal("backup file does not match original key")
	}
}

func TestRotateKey_new_key_differs_from_old(t *testing.T) {
	// arrange
	dir := t.TempDir()
	wardProject(t, dir)
	keyPath := filepath.Join(dir, ".ward", vaultName(dir)+"::default.key")
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
	if !testutil.Contains(stripped, "::default.key.bkp-") {
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

func newMyfishTwoVaults(t *testing.T) string {
	t.Helper()
	for _, k := range []string{"WARD_KEY", "WARD_KEY_MYFISH", "WARD_KEY_MYFISH_DEFAULT", "WARD_KEY_MYFISH_RESTRICTED"} {
		t.Setenv(k, "")
	}
	dir := filepath.Join(t.TempDir(), "myfish")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	if _, stderr, code := testutil.Run(t, bin, dir, "vault", "add", "myfish::restricted"); code != 0 {
		t.Fatalf("vault add failed: %s", stderr)
	}
	if _, stderr, code := testutil.Run(t, bin, dir, "set", "myfish::restricted:secrets.production.DATABASE_URL", "postgres://prod"); code != 0 {
		t.Fatalf("set failed: %s", stderr)
	}
	if _, stderr, code := testutil.Run(t, bin, dir, "set", "myfish:secrets.dev.DATABASE_URL", "postgres://dev"); code != 0 {
		t.Fatalf("set failed: %s", stderr)
	}
	return dir
}

func readTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		out[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRotateKey_subvault_rotates_only_that_vault(t *testing.T) {
	// arrange
	dir := newMyfishTwoVaults(t)
	defaultDir := filepath.Join(dir, ".ward", "vaults", "myfish::default")
	restrictedDir := filepath.Join(dir, ".ward", "vaults", "myfish::restricted")
	defaultBefore := readTree(t, defaultDir)
	restrictedBefore := readTree(t, restrictedDir)
	defaultKeyBefore, _ := os.ReadFile(filepath.Join(dir, ".ward", "myfish::default.key"))
	restrictedKeyBefore, _ := os.ReadFile(filepath.Join(dir, ".ward", "myfish::restricted.key"))

	// act
	stdout, stderr, code := testutil.Run(t, bin, dir, "rotate-key", "myfish::restricted")

	// assert
	if code != 0 {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	defaultAfter := readTree(t, defaultDir)
	if len(defaultAfter) != len(defaultBefore) {
		t.Fatalf("default files changed: %v -> %v", defaultBefore, defaultAfter)
	}
	for path, before := range defaultBefore {
		if defaultAfter[path] != before {
			t.Errorf("default file %s changed", path)
		}
	}
	changed := false
	for path, before := range restrictedBefore {
		if readTree(t, restrictedDir)[path] != before {
			changed = true
		}
	}
	if !changed {
		t.Error("restricted files were not re-encrypted")
	}
	defaultKeyAfter, _ := os.ReadFile(filepath.Join(dir, ".ward", "myfish::default.key"))
	if string(defaultKeyAfter) != string(defaultKeyBefore) {
		t.Error("default key must not change")
	}
	restrictedKeyAfter, _ := os.ReadFile(filepath.Join(dir, ".ward", "myfish::restricted.key"))
	if string(restrictedKeyAfter) == string(restrictedKeyBefore) {
		t.Error("restricted key must change")
	}
	got, _, getCode := testutil.Run(t, bin, dir, "get", "myfish::restricted:secrets.production.DATABASE_URL")
	if getCode != 0 || !testutil.Contains(got, "postgres://prod") {
		t.Errorf("restricted unreadable after rotation: code=%d out=%q", getCode, got)
	}
	dev, _, devCode := testutil.Run(t, bin, dir, "get", "myfish:secrets.dev.DATABASE_URL")
	if devCode != 0 || !testutil.Contains(dev, "postgres://dev") {
		t.Errorf("default unreadable: code=%d out=%q", devCode, dev)
	}
}

func TestRotateKey_subvault_backup_and_token(t *testing.T) {
	// arrange
	dir := newMyfishTwoVaults(t)
	keyBefore, _ := os.ReadFile(filepath.Join(dir, ".ward", "myfish::restricted.key"))

	// act
	stdout, stderr, code := testutil.Run(t, bin, dir, "rotate-key", "myfish::restricted")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	clean := testutil.StripANSI(stdout)
	if !testutil.Contains(clean, "WARD_KEY_MYFISH_RESTRICTED=ward-") {
		t.Errorf("expected token, got %q", clean)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".ward", "myfish::restricted.key.bkp-*"))
	if len(matches) != 1 {
		t.Fatalf("expected one backup, got %v", matches)
	}
	bkp, _ := os.ReadFile(matches[0])
	if string(bkp) != string(keyBefore) {
		t.Error("backup must hold the previous key")
	}
}

func TestRotateKey_without_vault_with_multiple_keys_errors(t *testing.T) {
	// arrange
	dir := newMyfishTwoVaults(t)
	before := readTree(t, dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "rotate-key")

	// assert
	if code == 0 {
		t.Fatal("expected failure")
	}
	if !testutil.Contains(stderr, "ward rotate-key <vault>") {
		t.Errorf("expected suggestion, got %q", stderr)
	}
	if !testutil.Contains(stderr, "myfish::restricted") {
		t.Errorf("expected vault list, got %q", stderr)
	}
	after := readTree(t, dir)
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s changed after failed rotate", path)
		}
	}
}

func TestRotateKey_env_only_vault_requires_key_file(t *testing.T) {
	// arrange
	dir := newMyfishTwoVaults(t)
	keyPath := filepath.Join(dir, ".ward", "myfish::restricted.key")
	raw, _ := os.ReadFile(keyPath)
	if err := os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WARD_KEY_MYFISH_RESTRICTED", "ward-"+base64.URLEncoding.EncodeToString(raw))

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "rotate-key", "myfish::restricted")

	// assert
	if code == 0 {
		t.Fatal("expected failure")
	}
	if !testutil.Contains(stderr, "rotate-key requires a key file for vault myfish::restricted") {
		t.Errorf("got %q", stderr)
	}
}

func TestRotateKey_single_key_project_without_arg(t *testing.T) {
	// arrange
	dir := filepath.Join(t.TempDir(), "myfish")
	os.MkdirAll(dir, 0755)
	t.Setenv("WARD_KEY_MYFISH_DEFAULT", "")
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	testutil.Run(t, bin, dir, "set", "myfish:secrets.dev.X", "v1")

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "rotate-key")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	out, _, getCode := testutil.Run(t, bin, dir, "get", "myfish:secrets.dev.X")
	if getCode != 0 || !testutil.Contains(out, "v1") {
		t.Errorf("unreadable after rotation: code=%d out=%q", getCode, out)
	}
}

func TestRotateKey_default_aliases_are_same_target(t *testing.T) {
	for _, arg := range []string{"myfish", "myfish::default"} {
		// arrange
		dir := newMyfishTwoVaults(t)
		restrictedDir := filepath.Join(dir, ".ward", "vaults", "myfish::restricted")
		restrictedBefore := readTree(t, restrictedDir)
		keyBefore, _ := os.ReadFile(filepath.Join(dir, ".ward", "myfish::default.key"))

		// act
		_, stderr, code := testutil.Run(t, bin, dir, "rotate-key", arg)

		// assert
		if code != 0 {
			t.Fatalf("%s: exit %d: %s", arg, code, stderr)
		}
		keyAfter, _ := os.ReadFile(filepath.Join(dir, ".ward", "myfish::default.key"))
		if string(keyAfter) == string(keyBefore) {
			t.Errorf("%s: default key must change", arg)
		}
		for path, before := range restrictedBefore {
			if readTree(t, restrictedDir)[path] != before {
				t.Errorf("%s: restricted file %s changed", arg, path)
			}
		}
		out, _, getCode := testutil.Run(t, bin, dir, "get", "myfish:secrets.dev.DATABASE_URL")
		if getCode != 0 || !testutil.Contains(out, "postgres://dev") {
			t.Errorf("%s: default unreadable: %q", arg, out)
		}
	}
}

func initMyfish(t *testing.T) string {
	t.Helper()
	for _, k := range []string{"WARD_KEY", "WARD_KEY_MYFISH", "WARD_KEY_MYFISH_DEFAULT", "WARD_KEY_MYFISH_RESTRICTED"} {
		t.Setenv(k, "")
	}
	dir := filepath.Join(t.TempDir(), "myfish")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	if _, stderr, code := testutil.Run(t, bin, dir, "set", "myfish:secrets.dev.X", "v1"); code != 0 {
		t.Fatalf("set failed: %s", stderr)
	}
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRotateKey_default_with_both_key_files_updates_both(t *testing.T) {
	// arrange
	dir := initMyfish(t)
	long := filepath.Join(dir, ".ward", "myfish::default.key")
	short := filepath.Join(dir, ".ward", "myfish.key")
	original, err := os.ReadFile(long)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(short, original, 0600); err != nil {
		t.Fatal(err)
	}

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "rotate-key")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	a, _ := os.ReadFile(long)
	b, _ := os.ReadFile(short)
	if string(a) != string(b) {
		t.Error("both key files must hold the same new key")
	}
	if string(a) == string(original) {
		t.Error("key must have changed")
	}
	out, getErr, getCode := testutil.Run(t, bin, dir, "get", "myfish:secrets.dev.X")
	if getCode != 0 || !testutil.Contains(out, "v1") {
		t.Errorf("project must still load: code=%d out=%q err=%q", getCode, out, getErr)
	}
}

func TestRotateKey_shared_explicit_key_file_errors_and_changes_nothing(t *testing.T) {
	// arrange
	dir := newMyfishTwoVaults(t)
	cfg := "vaults:\n" +
		"  - name: myfish::default\n    path: .ward/vaults/myfish::default\n    encryption:\n      key_file: .ward/shared.key\n" +
		"  - name: myfish::restricted\n    path: .ward/vaults/myfish::restricted\n    encryption:\n      key_file: .ward/shared.key\n"
	raw, _ := os.ReadFile(filepath.Join(dir, ".ward", "myfish::default.key"))
	writeFile(t, filepath.Join(dir, ".ward", "shared.key"), string(raw))
	writeFile(t, filepath.Join(dir, ".ward", "config.yaml"), cfg)
	before := readTree(t, dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "rotate-key", "myfish::restricted")

	// assert
	if code == 0 {
		t.Fatal("expected failure")
	}
	for _, want := range []string{"myfish::restricted", "myfish", ".ward/shared.key"} {
		if !testutil.Contains(stderr, want) {
			t.Errorf("stderr %q missing %q", stderr, want)
		}
	}
	after := readTree(t, dir)
	if len(after) != len(before) {
		t.Errorf("files appeared or vanished: %d -> %d", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s changed", path)
		}
	}
}

func TestRotateKey_vault_using_global_key_suggests_project_rotation(t *testing.T) {
	// arrange
	dir := initMyfish(t)
	commons := filepath.Join(dir, "shared", ".ward", "vaults", "commons")
	writeFile(t, filepath.Join(commons, "main.ward"), "-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n-----END AGE ENCRYPTED FILE-----\n")
	if _, stderr, code := testutil.Run(t, bin, dir, "vault", "add", "commons", "shared/.ward/vaults/commons"); code != 0 {
		t.Fatalf("vault add: %s", stderr)
	}

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "rotate-key", "commons")

	// assert
	if code == 0 {
		t.Fatal("expected failure")
	}
	if !testutil.Contains(stderr, "ward rotate-key") || !testutil.Contains(stderr, "without a vault") {
		t.Errorf("expected suggestion to rotate without a vault, got %q", stderr)
	}
}

func TestRotateKey_backup_outside_ward_dir_is_gitignored(t *testing.T) {
	// arrange
	for _, k := range []string{"WARD_KEY", "WARD_KEY_APP"} {
		t.Setenv(k, "")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	keyPath := filepath.Join(dir, "secrets", "app.key")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := wardage.GenerateKey(keyPath); err != nil {
		t.Fatal(err)
	}
	vaultDir := filepath.Join(dir, ".ward", "vaults", "app")
	os.MkdirAll(vaultDir, 0755)
	if err := (wardage.AgeArmorDecryptor{KeyFile: keyPath}).Encrypt(filepath.Join(vaultDir, "main.ward"), []byte("app:\n  main:\n    k: v\n")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, ".ward", "config.yaml"),
		"encryption:\n  key_file: secrets/app.key\nvaults:\n  - name: app\n    path: .ward/vaults/app\n")

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "rotate-key")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "secrets", "app.key.bkp-*"))
	if len(matches) != 1 {
		t.Fatalf("expected backup next to the key, got %v", matches)
	}
	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !testutil.Contains(string(gi), "secrets/*.bkp-*") {
		t.Errorf(".gitignore missing secrets/*.bkp-*: %q", gi)
	}
}
