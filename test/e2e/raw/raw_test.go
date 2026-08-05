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
	out, _, code := testutil.Run(t, bin, fix("basic"), "raw", ".ward/vaults/app/main.plain.ward")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "rawapp") {
		t.Errorf("expected rawapp in output, got: %q", out)
	}
}

func TestRaw_contains_all_keys(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "raw", ".ward/vaults/app/main.plain.ward")
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

// ── no arg: dump every file with a header ────────────────────────────────────

func TestRaw_no_arg_shows_all_files_with_headers(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("multi"), "raw")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	clean := testutil.StripANSI(out)
	// both files appear as headers
	if !testutil.Contains(clean, "app/main.plain.ward") || !testutil.Contains(clean, "app/db.plain.ward") {
		t.Errorf("expected both file headers, got: %q", clean)
	}
	// and their contents
	if !testutil.Contains(clean, "rawapp") || !testutil.Contains(clean, "localhost") {
		t.Errorf("expected both file contents, got: %q", clean)
	}
}

func TestRaw_single_file_has_no_header(t *testing.T) {
	// piping a single file must stay clean (just the YAML, no header/rule)
	out, _, code := testutil.Run(t, bin, fix("basic"), "raw", ".ward/vaults/app/main.plain.ward")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if testutil.Contains(out, "─") || testutil.Contains(testutil.StripANSI(out), ".ward/vaults/app/main.plain.ward") {
		t.Errorf("single-file raw should not print a header, got: %q", out)
	}
}
