//go:build e2e

package file_test

import (
	"os"
	"path/filepath"
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

func fix(name string) string { return testutil.FixtureDir("file", name) }

func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	if testutil.RunCmd(t, "cp", "-r", src+"/.", dst) != 0 {
		t.Fatalf("copy fixture failed")
	}
}

// --- ward file import ---

func TestFileImport_creates_ward_file(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)
	src := filepath.Join(dir, "service-account.json")
	if err := os.WriteFile(src, []byte(`{"type":"service_account"}`), 0600); err != nil {
		t.Fatal(err)
	}

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "file", "import", src, "app")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	wardPath := filepath.Join(dir, ".ward", "vaults", "app", "service-account.json.ward")
	if _, err := os.Stat(wardPath); err != nil {
		t.Errorf("expected .ward file at %s: %v", wardPath, err)
	}
}

func TestFileImport_key_readable_via_get(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)
	src := filepath.Join(dir, "service-account.json")
	content := `{"type":"service_account","project_id":"my-project"}`
	if err := os.WriteFile(src, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	testutil.Run(t, bin, dir, "file", "import", src, "app")

	// act
	out, _, code := testutil.Run(t, bin, dir, "get", "app.service_account_json")

	// assert
	if code != 0 {
		t.Fatalf("get exit %d", code)
	}
	if !testutil.Contains(out, "my-project") {
		t.Errorf("expected content in get output, got: %q", out)
	}
}

func TestFileImport_errors_if_target_exists(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)
	src := filepath.Join(dir, "service-account.json")
	if err := os.WriteFile(src, []byte(`{"type":"service_account"}`), 0600); err != nil {
		t.Fatal(err)
	}
	testutil.Run(t, bin, dir, "file", "import", src, "app")

	// act — import again, same file
	_, stderr, code := testutil.Run(t, bin, dir, "file", "import", src, "app")

	// assert
	if code == 0 {
		t.Error("expected non-zero exit when target already exists")
	}
	if !testutil.Contains(stderr, "already exists") {
		t.Errorf("expected 'already exists' in stderr, got: %q", stderr)
	}
}

func TestFileImport_errors_if_source_not_found(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "file", "import", "nonexistent.json", "app")

	// assert
	if code == 0 {
		t.Error("expected non-zero exit when source file not found")
	}
	if !testutil.Contains(stderr, "nonexistent.json") {
		t.Errorf("expected filename in stderr, got: %q", stderr)
	}
}

func TestFileImport_errors_if_vault_not_found(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)
	src := filepath.Join(dir, "service-account.json")
	if err := os.WriteFile(src, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "file", "import", src, "nonexistent-vault")

	// assert
	if code == 0 {
		t.Error("expected non-zero exit when vault not found")
	}
	if !testutil.Contains(stderr, "nonexistent-vault") {
		t.Errorf("expected vault name in stderr, got: %q", stderr)
	}
}

// --- ward file export ---

func TestFileExport_restores_original_file(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)
	src := filepath.Join(dir, "service-account.json")
	content := `{"type":"service_account","project_id":"my-project"}`
	if err := os.WriteFile(src, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	testutil.Run(t, bin, dir, "file", "import", src, "app")

	outDir := t.TempDir()

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "file", "export", "service-account.json", outDir)

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got, err := os.ReadFile(filepath.Join(outDir, "service-account.json"))
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if string(got) != content {
		t.Errorf("expected %q, got %q", content, string(got))
	}
}

func TestFileExport_defaults_to_cwd(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)
	src := filepath.Join(dir, "service-account.json")
	content := `{"type":"service_account"}`
	if err := os.WriteFile(src, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	testutil.Run(t, bin, dir, "file", "import", src, "app")
	os.Remove(src)

	// act — no dest arg, should write to CWD (dir)
	_, stderr, code := testutil.Run(t, bin, dir, "file", "export", "service-account.json")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got, err := os.ReadFile(filepath.Join(dir, "service-account.json"))
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if string(got) != content {
		t.Errorf("expected %q, got %q", content, string(got))
	}
}

func TestFileExport_errors_if_secret_not_found(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "file", "export", "nonexistent.json")

	// assert
	if code == 0 {
		t.Error("expected non-zero exit when secret not found")
	}
	if !testutil.Contains(stderr, "nonexistent.json") {
		t.Errorf("expected filename in stderr, got: %q", stderr)
	}
}

func TestFileExport_errors_if_dest_exists(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)
	src := filepath.Join(dir, "service-account.json")
	content := `{"type":"service_account"}`
	if err := os.WriteFile(src, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	testutil.Run(t, bin, dir, "file", "import", src, "app")

	// act — export to same dir where source file still exists
	_, stderr, code := testutil.Run(t, bin, dir, "file", "export", "service-account.json", dir)

	// assert
	if code == 0 {
		t.Error("expected non-zero exit when destination file already exists")
	}
	if !testutil.Contains(stderr, "already exists") {
		t.Errorf("expected 'already exists' in stderr, got: %q", stderr)
	}
}
