//go:build e2e

package override_test

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

func fix(name string) string { return testutil.FixtureDir("override", name) }

func TestOverride_replaces_content(t *testing.T) {
	dir := t.TempDir()
	// Copy fixture to temp dir so we don't mutate the original
	copyFixture(t, fix("basic"), dir)

	newContent := "app:\n  name: updated\n  token: new-token\n"
	cmd := testutil.RunWithStdin(t, bin, dir, newContent, "override", ".ward/vault/app.ward")
	if cmd != 0 {
		t.Fatalf("override exit %d", cmd)
	}

	out, _, code := testutil.Run(t, bin, dir, "get", "app.name")
	if code != 0 {
		t.Fatalf("get exit %d", code)
	}
	if !testutil.Contains(out, "updated") {
		t.Errorf("expected updated after override, got: %q", out)
	}
}

func TestOverride_missing_file_fails(t *testing.T) {
	code := testutil.RunWithStdin(t, bin, fix("basic"), "app:\n  name: x\n", "override", ".ward/vault/nonexistent.ward")
	if code == 0 {
		t.Fatal("expected non-zero exit for missing file")
	}
}

// copyFixture copies a fixture directory to dst using the OS.
func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	cmd := testutil.RunCmd(t, "cp", "-r", src+"/.", dst)
	if cmd != 0 {
		t.Fatalf("copy fixture failed")
	}
}
