//go:build e2e

package set_test

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

func fix(name string) string { return testutil.FixtureDir("set", name) }

// copyFixture copies a fixture directory to dst so tests can mutate it.
func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	if testutil.RunCmd(t, "cp", "-r", src+"/.", dst) != 0 {
		t.Fatalf("copy fixture failed")
	}
}

func TestSet_updates_existing_key(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, _, code := testutil.Run(t, bin, dir, "set", "app.main.token", "updated")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d", code)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "app.main.token")
	if !testutil.Contains(out, "updated") {
		t.Errorf("expected updated, got: %q", out)
	}
}

func TestSet_creates_new_file_with_notice(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app.prod.apikey", "xyz")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "a new file was created") {
		t.Errorf("expected new-file notice, got: %q", stderr)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "app.prod.apikey")
	if !testutil.Contains(out, "xyz") {
		t.Errorf("expected xyz, got: %q", out)
	}
}

func TestSet_shallow_path_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app.token", "x")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for shallow path")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "too shallow") {
		t.Errorf("expected too-shallow error, got: %q", stderr)
	}
}

func TestSet_unknown_vault_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "unknown.main.token", "x")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown vault")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "not found") {
		t.Errorf("expected vault-not-found error, got: %q", stderr)
	}
}

func TestSet_new_key_preserves_sibling_keys_in_same_file(t *testing.T) {
	// arrange: basic fixture has app.main.{name, token}
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act: add a brand-new key to the same file
	_, _, code := testutil.Run(t, bin, dir, "set", "app.main.password", "s3cr3t")

	// assert: the new key was written
	if code != 0 {
		t.Fatalf("set exit %d", code)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "app.main.password")
	if !testutil.Contains(out, "s3cr3t") {
		t.Errorf("expected password=s3cr3t, got: %q", out)
	}

	// assert: pre-existing sibling keys must still be present
	out, _, _ = testutil.Run(t, bin, dir, "get", "app.main.token")
	if !testutil.Contains(out, "original") {
		t.Errorf("expected token=original to be preserved, got: %q", out)
	}
	out, _, _ = testutil.Run(t, bin, dir, "get", "app.main.name")
	if !testutil.Contains(out, "my-service") {
		t.Errorf("expected name=my-service to be preserved, got: %q", out)
	}
}

func TestSet_type2_collision_writes_and_warns(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("collision"), dir)

	// act: vb.prod.token collides with va.staging.token on env var TOKEN
	_, stderr, code := testutil.Run(t, bin, dir, "set", "vb.prod.token", "from-vb")

	// assert: write succeeds AND a warning is printed
	if code != 0 {
		t.Fatalf("set exit %d — expected success with warning", code)
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "collides") {
		t.Errorf("expected collision warning, got: %q", stderr)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "vb.prod.token")
	if !testutil.Contains(out, "from-vb") {
		t.Errorf("expected value written despite collision, got: %q", out)
	}
}
