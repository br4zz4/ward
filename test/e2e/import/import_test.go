//go:build e2e

package import_test

import (
	"os"
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

func fix(name string) string { return testutil.FixtureDir("import", name) }

// copyFixture copies a fixture directory to dst so tests can mutate it.
func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	if testutil.RunCmd(t, "cp", "-r", src+"/.", dst) != 0 {
		t.Fatalf("copy fixture failed")
	}
}

func TestImport_replaces_content(t *testing.T) {
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	newContent := "app:\n  main:\n    name: updated\n    token: new-token\n"
	code := testutil.RunWithStdin(t, bin, dir, newContent, "import", ".ward/vaults/app/main.plain.ward")
	if code != 0 {
		t.Fatalf("import exit %d", code)
	}

	out, _, getCode := testutil.Run(t, bin, dir, "get", "app.main.name")
	if getCode != 0 {
		t.Fatalf("get exit %d", getCode)
	}
	if !testutil.Contains(out, "updated") {
		t.Errorf("expected updated after import, got: %q", out)
	}
}

func TestImport_missing_file_fails(t *testing.T) {
	code := testutil.RunWithStdin(t, bin, fix("basic"), "app:\n  main:\n    name: x\n", "import", ".ward/vaults/app/nonexistent.ward")
	if code == 0 {
		t.Fatal("expected non-zero exit for missing file")
	}
}
