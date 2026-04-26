//go:build e2e

package new_test

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

func fix(name string) string { return testutil.FixtureDir("new", name) }

func TestNew_creates_ward_file(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".ward", "vaults", "basic"), 0755); err != nil {
		t.Fatal(err)
	}

	// act
	code := testutil.RunWithStdin(t, bin, dir, "y\n", "new", "basic", "mysecrets")

	// assert
	if code != 0 {
		t.Fatalf("new exit %d", code)
	}
	expected := filepath.Join(dir, ".ward", "vaults", "basic", "mysecrets.ward")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected %s to be created", expected)
	}
}

func TestNew_aborted_does_not_create_file(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".ward", "vaults", "basic"), 0755); err != nil {
		t.Fatal(err)
	}

	// act
	code := testutil.RunWithStdin(t, bin, dir, "n\n", "new", "basic", "shouldnotexist")

	// assert
	if code != 0 {
		t.Fatalf("new abort exit %d", code)
	}
	unexpected := filepath.Join(dir, ".ward", "vaults", "basic", "shouldnotexist.ward")
	if _, err := os.Stat(unexpected); err == nil {
		t.Errorf("expected %s NOT to be created after abort", unexpected)
	}
}

func TestNew_duplicate_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".ward", "vaults", "basic"), 0755); err != nil {
		t.Fatal(err)
	}

	// act - create once
	testutil.RunWithStdin(t, bin, dir, "y\n", "new", "basic", "duplicate")
	// act - create again
	code := testutil.RunWithStdin(t, bin, dir, "y\n", "new", "basic", "duplicate")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit when creating duplicate file")
	}
}

func TestNew_unknown_vault_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)

	// act
	code := testutil.RunWithStdin(t, bin, dir, "y\n", "new", "nonexistent", "mysecrets")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown vault")
	}
}
