//go:build e2e

package init_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oporpino/ward/test/e2e/testutil"
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

func TestInit_creates_ward_dir(t *testing.T) {
	dir := t.TempDir()
	_, _, code := testutil.Run(t, bin, dir, "init")
	if code != 0 {
		t.Fatalf("init exit %d", code)
	}
	for _, path := range []string{
		filepath.Join(dir, ".ward"),
		filepath.Join(dir, ".ward", "config.yaml"),
		filepath.Join(dir, ".ward", "vault"),
		filepath.Join(dir, ".ward", ".key"),
	} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after init", path)
		}
	}
}

func TestInit_creates_secrets_file(t *testing.T) {
	dir := t.TempDir()
	_, _, code := testutil.Run(t, bin, dir, "init")
	if code != 0 {
		t.Fatalf("init exit %d", code)
	}
	vault := filepath.Join(dir, ".ward", "vault")
	entries, err := os.ReadDir(vault)
	if err != nil {
		t.Fatalf("reading vault dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one .ward file in vault after init")
	}
}

func TestInit_fails_if_already_initialized(t *testing.T) {
	dir := t.TempDir()
	testutil.Run(t, bin, dir, "init")
	_, _, code := testutil.Run(t, bin, dir, "init")
	if code == 0 {
		t.Fatal("expected non-zero exit when init called twice")
	}
}

func TestInit_get_works_after_init(t *testing.T) {
	dir := t.TempDir()
	_, _, code := testutil.Run(t, bin, dir, "init")
	if code != 0 {
		t.Fatalf("init exit %d", code)
	}
	// After init, inspect should pass (clean state)
	_, _, code = testutil.Run(t, bin, dir, "inspect")
	if code != 0 {
		t.Fatalf("inspect should pass after init, got %d", code)
	}
}
