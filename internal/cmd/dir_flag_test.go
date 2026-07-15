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
