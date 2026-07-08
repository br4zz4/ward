//go:build e2e

package unset_test

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

func fix(name string) string { return testutil.FixtureDir("unset", name) }

// copyFixture copies a fixture directory to dst so tests can mutate it.
func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	if testutil.RunCmd(t, "cp", "-r", src+"/.", dst) != 0 {
		t.Fatalf("copy fixture failed")
	}
}

func TestUnset_removes_existing_key(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, _, code := testutil.Run(t, bin, dir, "unset", "app.main.token")

	// assert
	if code != 0 {
		t.Fatalf("unset exit %d", code)
	}
	_, _, getCode := testutil.Run(t, bin, dir, "get", "app.main.token")
	if getCode == 0 {
		t.Error("expected key to be gone after unset")
	}
}

func TestUnset_keeps_sibling_key(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	testutil.Run(t, bin, dir, "unset", "app.main.token")

	// assert: sibling survives and structure stays valid
	out, _, code := testutil.Run(t, bin, dir, "get", "app.main.name")
	if code != 0 {
		t.Fatalf("get sibling exit %d", code)
	}
	if !testutil.Contains(out, "my-service") {
		t.Errorf("expected sibling to remain, got: %q", out)
	}
}

func TestUnset_missing_key_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "unset", "app.main.nonexistent")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for missing key")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "key not found") {
		t.Errorf("expected key-not-found error, got: %q", stderr)
	}
}

func TestUnset_unknown_vault_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "unset", "unknown.main.token")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown vault")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "not found") {
		t.Errorf("expected vault-not-found error, got: %q", stderr)
	}
}
