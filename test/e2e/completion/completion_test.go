//go:build e2e

package completion_test

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

func TestInstallCompletion_zsh_writes_file_to_output(t *testing.T) {
	dir := t.TempDir()
	out, _, code := testutil.Run(t, bin, dir, "install", "completion", "zsh", "--output", dir)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	target := filepath.Join(dir, "_ward")
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", target, err)
	}
	if !testutil.Contains(string(content), "compdef _ward ward") {
		t.Errorf("expected compdef in file, got: %q", content)
	}
	clean := testutil.StripANSI(out)
	if !testutil.Contains(clean, target) {
		t.Errorf("expected path %s in output, got: %q", target, clean)
	}
	if !testutil.Contains(clean, "✓") {
		t.Errorf("expected ✓ in output, got: %q", clean)
	}
}

func TestInstallCompletion_fish_short_output_flag(t *testing.T) {
	dir := t.TempDir()
	_, _, code := testutil.Run(t, bin, dir, "install", "completion", "fish", "-o", dir)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	content, err := os.ReadFile(filepath.Join(dir, "ward.fish"))
	if err != nil {
		t.Fatalf("expected ward.fish to exist: %v", err)
	}
	if !testutil.Contains(string(content), "fish completion") {
		t.Errorf("expected fish completion in file, got: %q", content)
	}
}

func TestInstallCompletion_bash_stdout_creates_no_file(t *testing.T) {
	dir := t.TempDir()
	out, _, code := testutil.Run(t, bin, dir, "install", "completion", "bash", "--stdout")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "bash completion") {
		t.Errorf("expected bash completion in stdout, got: %q", out)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files created, got: %v", entries)
	}
}

func TestInstallCompletion_unsupported_shell_fails(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := testutil.Run(t, bin, dir, "install", "completion", "tcsh", "--stdout")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !testutil.Contains(stderr, "unsupported shell") {
		t.Errorf("expected 'unsupported shell' in stderr, got: %q", stderr)
	}
}
