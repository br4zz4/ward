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

// A config ward cannot parse must be reported as such. Blaming the vault name
// sends the user looking for a vault that is in fact declared.
func TestEdit_malformed_config_is_not_reported_as_unknown_vault(t *testing.T) {
	// act
	_, stderr, code := testutil.Run(t, bin, fix("bad-config"), "edit", "app:main.name")

	// assert
	if code == 0 {
		t.Fatalf("expected a non-zero exit, got %d", code)
	}
	if testutil.Contains(stderr, "not found") {
		t.Errorf("a parse failure must not be reported as a missing vault, got: %q", stderr)
	}
	if !testutil.Contains(stderr, "config.yaml") {
		t.Errorf("expected the config file named in the error, got: %q", stderr)
	}
}

// The same must hold for the bare <vault> form.
func TestEdit_malformed_config_on_bare_vault_form(t *testing.T) {
	// act
	_, stderr, code := testutil.Run(t, bin, fix("bad-config"), "edit", "app")

	// assert
	if code == 0 {
		t.Fatalf("expected a non-zero exit, got %d", code)
	}
	if !testutil.Contains(stderr, "config.yaml") {
		t.Errorf("expected the config file named in the error, got: %q", stderr)
	}
}

// When one vault loads and another is skipped for a missing key, the skipped
// files are invisible to scope resolution. Saying "no such file" would blame
// the argument for a key problem.
func TestEdit_scope_in_partially_readable_project_reports_the_skip(t *testing.T) {
	// act
	_, stderr, code := testutil.Run(t, bin, fix("partial-key"), "edit", "locked:infra.production")

	// assert
	if code == 0 {
		t.Fatalf("expected a non-zero exit, got %d", code)
	}
	if !testutil.Contains(stderr, "missing key for vault locked") {
		t.Errorf("expected the skipped vault named, got: %q", stderr)
	}
	if testutil.Contains(stderr, "no such file") {
		t.Errorf("a skipped vault must not be reported as a missing file, got: %q", stderr)
	}
}

// A readable project that genuinely has no such scope still says so plainly.
func TestEdit_unmatched_scope_in_readable_vault_is_not_masked(t *testing.T) {
	// act — the app vault loads fine; this path simply does not exist
	_, stderr, code := testutil.Run(t, bin, fix("partial-key"), "edit", "app:nope.missing")

	// assert
	if code == 0 {
		t.Fatalf("expected a non-zero exit, got %d", code)
	}
	if testutil.Contains(stderr, "missing key for vault locked") {
		t.Errorf("an unrelated readable vault must not blame the skipped one, got: %q", stderr)
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
