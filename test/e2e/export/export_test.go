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
	out, _, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vaults/app/main.plain.ward")
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
	_, _, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vaults/app/main.plain.ward", outFile)
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
	_, _, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vaults/app/nonexistent.ward")
	if code == 0 {
		t.Fatal("expected non-zero exit for missing file")
	}
}

func TestExport_stdout_refused_in_ai_mode_env(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "1")

	// act
	out, stderr, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vaults/app/main.plain.ward")

	// assert
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
	if out != "" {
		t.Errorf("expected empty stdout, got: %q", out)
	}
	if testutil.Contains(out+stderr, "export-token") || testutil.Contains(out+stderr, "exportapp") {
		t.Errorf("value leaked: %q", out+stderr)
	}
}

func TestExport_stdout_refused_with_ai_mode_flag(t *testing.T) {
	// act
	out, stderr, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vaults/app/main.plain.ward", "--ai-mode")

	// assert
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
	if out != "" {
		t.Errorf("expected empty stdout, got: %q", out)
	}
	if testutil.Contains(out+stderr, "export-token") || testutil.Contains(out+stderr, "exportapp") {
		t.Errorf("value leaked: %q", out+stderr)
	}
}

func TestExport_to_file_allowed_in_ai_mode_env(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "1")
	outFile := filepath.Join(t.TempDir(), "out.yaml")

	// act
	_, _, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vaults/app/main.plain.ward", outFile)

	// assert
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

func TestExport_to_file_allowed_with_ai_mode_flag(t *testing.T) {
	// arrange
	outFile := filepath.Join(t.TempDir(), "out.yaml")

	// act
	_, _, code := testutil.Run(t, bin, fix("basic"), "export", ".ward/vaults/app/main.plain.ward", outFile, "--ai-mode")

	// assert
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("expected output file: %v", err)
	}
}
