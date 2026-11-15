package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyDirFlag_changes_working_dir(t *testing.T) {
	// arrange
	target := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	// act
	err = applyDirFlag(target)

	// assert
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, _ := os.Getwd()
	// resolve symlinks so /var/... == /private/var/... on macOS
	gotReal, _ := filepath.EvalSymlinks(got)
	targetReal, _ := filepath.EvalSymlinks(target)
	if gotReal != targetReal {
		t.Errorf("expected cwd %q, got %q", targetReal, gotReal)
	}
}

func TestApplyDirFlag_empty_is_noop(t *testing.T) {
	// arrange
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// act
	err = applyDirFlag("")

	// assert
	if err != nil {
		t.Fatalf("expected no error on empty dir, got %v", err)
	}
	got, _ := os.Getwd()
	if got != orig {
		t.Errorf("expected cwd unchanged %q, got %q", orig, got)
	}
}

func TestResolveInvocationPath_absolute_is_returned_unchanged(t *testing.T) {
	// arrange
	abs := filepath.Join(t.TempDir(), "v")
	prev := invocationDir
	t.Cleanup(func() { invocationDir = prev })
	invocationDir = t.TempDir()

	// act
	got := resolveInvocationPath(abs)

	// assert
	if got != abs {
		t.Errorf("expected %q, got %q", abs, got)
	}
}

func TestResolveInvocationPath_relative_joins_invocation_dir(t *testing.T) {
	// arrange
	base := t.TempDir()
	prev := invocationDir
	t.Cleanup(func() { invocationDir = prev })
	invocationDir = base

	// act
	got := resolveInvocationPath("v")

	// assert
	want := filepath.Join(base, "v")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveInvocationPath_empty_invocation_dir_uses_cwd(t *testing.T) {
	// arrange
	prev := invocationDir
	t.Cleanup(func() { invocationDir = prev })
	invocationDir = ""
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// act
	got := resolveInvocationPath("v")

	// assert
	want := filepath.Join(cwd, "v")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveInvocationPath_after_dir_flag_resolves_against_original_cwd(t *testing.T) {
	// arrange
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	prev := invocationDir
	t.Cleanup(func() { invocationDir = prev })
	target := t.TempDir()

	// act
	if err := applyDirFlag(target); err != nil {
		t.Fatal(err)
	}
	got := resolveInvocationPath("v")

	// assert
	want := filepath.Join(orig, "v")
	if got != want {
		t.Errorf("expected %q (original cwd), got %q", want, got)
	}
}

func TestApplyDirFlag_empty_records_invocation_dir(t *testing.T) {
	// arrange
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	prev := invocationDir
	t.Cleanup(func() { invocationDir = prev })
	invocationDir = ""

	// act
	if err := applyDirFlag(""); err != nil {
		t.Fatal(err)
	}

	// assert
	if invocationDir != orig {
		t.Errorf("expected invocationDir %q, got %q", orig, invocationDir)
	}
}

func TestApplyDirFlag_nonexistent_returns_error(t *testing.T) {
	// arrange
	bad := "/tmp/ward-nonexistent-99999999"

	// act
	err := applyDirFlag(bad)

	// assert
	if err == nil {
		t.Error("expected error for nonexistent dir, got nil")
	}
}
