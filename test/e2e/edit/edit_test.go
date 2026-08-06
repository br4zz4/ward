//go:build e2e

package edit_test

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

func fix(name string) string { return testutil.FixtureDir("edit", name) }

// A scope cannot be resolved without a key, because resolution reads the
// encrypted contents. The missing key must be reported as such, not disguised
// as a missing file.
func TestEdit_scope_without_key_reports_the_key_error(t *testing.T) {
	// act
	_, stderr, code := testutil.Run(t, bin, fix("no-key"), "edit", "app:infra.production")

	// assert
	if code == 0 {
		t.Fatalf("expected a non-zero exit, got %d", code)
	}
	// The load error says how to supply the key; the generic per-file message
	// that the fallthrough produced does not.
	if !testutil.Contains(stderr, "WARD_KEY") {
		t.Errorf("expected the actionable load error naming WARD_KEY, got: %q", stderr)
	}
	if testutil.Contains(stderr, "no such file") {
		t.Errorf("missing key must not be reported as a missing file, got: %q", stderr)
	}
}

// A qualified scope names its vault explicitly, so an unknown one is a vault
// error — reported before anything is decrypted.
func TestEdit_scope_with_unknown_vault_reports_vault_not_found(t *testing.T) {
	// act
	_, stderr, code := testutil.Run(t, bin, fix("no-key"), "edit", "nope:infra.production")

	// assert
	if code == 0 {
		t.Fatalf("expected a non-zero exit, got %d", code)
	}
	if !testutil.Contains(stderr, "vault") || !testutil.Contains(stderr, "not found") {
		t.Errorf("expected the vault-not-found error, got: %q", stderr)
	}
	if testutil.Contains(stderr, "no such file") {
		t.Errorf("unknown vault must not be reported as a missing file, got: %q", stderr)
	}
}

// The <vault> <path> form resolves on the filesystem, so it still reaches the
// file when no key is available — only the decrypt step then fails.
func TestEdit_vault_and_path_resolves_without_key(t *testing.T) {
	// act
	_, stderr, code := testutil.Run(t, bin, fix("no-key"), "edit", "app", "infra/production")

	// assert
	if code == 0 {
		t.Fatalf("expected a non-zero exit, got %d", code)
	}
	if !testutil.Contains(stderr, "infra/production.ward") {
		t.Errorf("expected the resolved file path in the error, got: %q", stderr)
	}
}
