//go:build e2e

package tree_test

import (
	"os"
	"strings"
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

func fix(name string) string { return testutil.FixtureDir("tree", name) }

// clearKeyEnv unsets every env var that could supply a key to the missing-key
// fixtures, so a developer's environment cannot unlock them and mask a failure.
// The child process inherits the parent's environment.
func clearKeyEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"WARD_KEY",                // global token, checked first
		"WARD_KEY_LOCKED",         // derived from vault name "locked"
		"WARD_KEY_APP",            // derived from vault name "app"
		"WARD_KEY_LOCKED_FIXTURE", // declared via key_env
		"WARD_KEY_GLOBAL_FIXTURE", // declared via global key_env
	} {
		t.Setenv(name, "")
	}
}

func TestTree_shows_tree_with_origin(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "tree")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "app") || !testutil.Contains(out, "←") {
		t.Errorf("expected tree with origin arrow, got: %q", out)
	}
}

func TestTree_subtree(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "tree", "app.main.db")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "host") {
		t.Errorf("expected host in subtree, got: %q", out)
	}
}

func TestTree_long_value_truncated(t *testing.T) {
	// arrange + act
	out, _, code := testutil.Run(t, bin, fix("long-value"), "tree")

	// assert: exits clean and the long value is truncated with ellipsis
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	clean := testutil.StripANSI(out)
	if !testutil.Contains(clean, "…") {
		t.Errorf("expected long value to be truncated with ellipsis, got: %q", clean)
	}
	// short key must still appear untruncated
	if !testutil.Contains(clean, "hello") {
		t.Errorf("expected short_key value 'hello' present, got: %q", clean)
	}
	// origin annotation must appear on every leaf line
	if !testutil.Contains(clean, "←") {
		t.Errorf("expected origin arrow ← in output, got: %q", clean)
	}
}

func TestTree_long_value_arrow_not_pushed_far_right(t *testing.T) {
	// arrange + act
	out, _, code := testutil.Run(t, bin, fix("long-value"), "tree")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	// assert: no line is longer than 200 visible chars before the ← arrow
	clean := testutil.StripANSI(out)
	for _, line := range strings.Split(clean, "\n") {
		idx := strings.Index(line, "←")
		if idx < 0 {
			continue
		}
		if idx > 200 {
			t.Errorf("← pushed too far right (%d cols); long values should be truncated: %q", idx, line)
		}
	}
}

// A vault whose key_env is unset must not abort the whole command: the readable
// vaults are shown and the locked one is reported as a warning.
func TestTree_missing_vault_key_warns_and_shows_other_vaults(t *testing.T) {
	// arrange: clear every env var that could unlock the fixture's vault —
	// the derived name and the global one outrank the declared key_env
	clearKeyEnv(t)

	// act
	out, stderr, code := testutil.Run(t, bin, fix("missing-key"), "tree")

	// assert
	if code != 0 {
		t.Fatalf("tree should exit 0 when another vault is readable, got %d — %q", code, stderr)
	}
	if !testutil.Contains(testutil.StripANSI(out), "myapp") {
		t.Errorf("expected readable vault app in output, got: %q", out)
	}
	clean := testutil.StripANSI(stderr)
	if !testutil.Contains(clean, "missing key for vault locked") {
		t.Errorf("expected warning for vault locked, got: %q", clean)
	}
	if !testutil.Contains(clean, "WARD_KEY_LOCKED_FIXTURE") {
		t.Errorf("expected warning to name the env var to set, got: %q", clean)
	}
}

// When no vault can be decrypted there is nothing to show, so it stays fatal.
func TestTree_all_vaults_missing_key_fails(t *testing.T) {
	// arrange
	clearKeyEnv(t)

	// act
	_, stderr, code := testutil.Run(t, bin, fix("all-missing-key"), "tree")

	// assert
	if code == 0 {
		t.Fatalf("expected non-zero exit when no vault is readable")
	}
	clean := testutil.StripANSI(stderr)
	if !testutil.Contains(clean, "no encryption key") {
		t.Errorf("expected missing-key error, got: %q", clean)
	}
	if !testutil.Contains(clean, "WARD_KEY_LOCKED_FIXTURE") {
		t.Errorf("expected error to name the env var to set, got: %q", clean)
	}
}

func TestTree_conflict_envvar_warns(t *testing.T) {
	out, stderr, code := testutil.Run(t, bin, fix("conflict-envvar"), "tree")
	if code != 0 {
		t.Fatalf("tree should exit 0 even with env collisions, got %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out+stderr), "collision") {
		t.Errorf("expected collision warning, got: %q / %q", out, stderr)
	}
}

// A globally declared key that is unavailable must not abort the command either:
// vaults that do not need it still render.
func TestTree_missing_global_key_does_not_abort(t *testing.T) {
	// arrange
	clearKeyEnv(t)

	// act
	out, stderr, code := testutil.Run(t, bin, fix("global-key-missing"), "tree")

	// assert
	if code != 0 {
		t.Fatalf("tree should exit 0 when the vault needs no key, got %d — %q", code, stderr)
	}
	if !testutil.Contains(testutil.StripANSI(out), "myapp") {
		t.Errorf("expected the readable vault to render, got: %q", out)
	}
}
