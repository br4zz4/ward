//go:build integration

package new_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oporpino/ward/test/integration/testutil"
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
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)

	// new requires interactive confirmation — answer y via stdin
	code := testutil.RunWithStdin(t, bin, dir, "y\n", "new", "mysecrets")
	if code != 0 {
		t.Fatalf("new exit %d", code)
	}

	expected := filepath.Join(dir, ".ward", "vault", "mysecrets.ward")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("expected %s to be created", expected)
	}
}

func TestNew_aborted_does_not_create_file(t *testing.T) {
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)

	code := testutil.RunWithStdin(t, bin, dir, "n\n", "new", "shouldnotexist")
	if code != 0 {
		t.Fatalf("new abort exit %d", code)
	}

	unexpected := filepath.Join(dir, ".ward", "vault", "shouldnotexist.ward")
	if _, err := os.Stat(unexpected); err == nil {
		t.Errorf("expected %s NOT to be created after abort", unexpected)
	}
}

func TestNew_duplicate_fails(t *testing.T) {
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)

	// Create the file first
	testutil.RunWithStdin(t, bin, dir, "y\n", "new", "duplicate")

	// Try to create again — should fail
	code := testutil.RunWithStdin(t, bin, dir, "y\n", "new", "duplicate")
	if code == 0 {
		t.Fatal("expected non-zero exit when creating duplicate file")
	}
}
