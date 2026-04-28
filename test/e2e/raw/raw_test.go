//go:build e2e

package raw_test

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

func fix(name string) string { return testutil.FixtureDir("raw", name) }

func TestRaw_prints_yaml(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "raw", ".ward/vaults/app/main.ward")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "rawapp") {
		t.Errorf("expected rawapp in output, got: %q", out)
	}
}

func TestRaw_contains_all_keys(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "raw", ".ward/vaults/app/main.ward")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, key := range []string{"name", "secret_key"} {
		if !testutil.Contains(out, key) {
			t.Errorf("expected key %q in raw output, got: %q", key, out)
		}
	}
}

func TestRaw_missing_file_fails(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("basic"), "raw", ".ward/vaults/app/nonexistent.ward")
	if code == 0 {
		t.Fatal("expected non-zero exit for missing file")
	}
}
