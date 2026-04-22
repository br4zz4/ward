//go:build e2e

package export_test

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

func fix(name string) string { return testutil.FixtureDir("export", name) }

func TestExport_prints_to_stdout(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vault/app.ward")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "exportapp") {
		t.Errorf("expected exportapp in output, got: %q", out)
	}
}

func TestExport_to_file(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.yaml")
	_, _, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vault/app.ward", outFile)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if !testutil.Contains(string(data), "exportapp") {
		t.Errorf("expected exportapp in file, got: %q", string(data))
	}
}

func TestExport_missing_file_fails(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vault/nonexistent.ward")
	if code == 0 {
		t.Fatal("expected non-zero exit for missing file")
	}
}
