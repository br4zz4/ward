//go:build e2e

package get_test

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

func fix(name string) string { return testutil.FixtureDir("get", name) }

func TestGet_leaf_value(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "get", "app:main.name")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "my-service") {
		t.Errorf("expected my-service, got: %q", out)
	}
}

func TestGet_nested_value(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "get", "app:main.db.host")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "localhost") {
		t.Errorf("expected localhost, got: %q", out)
	}
}

func TestGet_numeric_value(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "get", "app:main.port")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "8080") {
		t.Errorf("expected 8080, got: %q", out)
	}
}

func TestGet_missing_key_fails(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("missing-key"), "get", "app:main.nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit for missing key")
	}
}

func TestGet_missing_key_lists_available(t *testing.T) {
	// the error should name the level where the path broke and its keys
	_, stderr, _ := testutil.Run(t, bin, fix("missing-key"), "get", "app:main.nonexistent")
	clean := testutil.StripANSI(stderr)
	if !testutil.Contains(clean, "available under app.main") || !testutil.Contains(clean, "name") {
		t.Errorf("expected available-keys hint under app.main, got: %q", stderr)
	}
}

func TestGet_no_args_fails(t *testing.T) {
	_, stderr, code := testutil.Run(t, bin, fix("basic"), "get")
	if code == 0 {
		t.Fatal("expected non-zero exit when no args")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "missing scope") {
		t.Errorf("expected missing scope error, got: %q", stderr)
	}
}

func TestGet_multi_vault_vault_a_key(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-file"), "get", "vault-a:main.vault_a_only")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "value-from-a") {
		t.Errorf("expected value-from-a, got: %q", out)
	}
}

func TestGet_conflict_envvar_specific_path(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "get", "app:staging.token")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "staging-token") {
		t.Errorf("expected staging-token, got: %q", out)
	}
}

func TestGet_conflict_envvar_other_path(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "get", "app:production.token")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "production-token") {
		t.Errorf("expected production-token, got: %q", out)
	}
}

// ── scope: qualified / unqualified / ambiguity ────────────────────────────────

func TestGet_scope_qualified_colon(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("mv"), "get", "vault1:group.key1")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "value1") {
		t.Errorf("expected value1, got: %q", out)
	}
}

func TestGet_scope_qualified_flags(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("mv"), "get", "--vault", "vault1", "--secret", "group.key1")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "value1") {
		t.Errorf("expected value1, got: %q", out)
	}
}

func TestGet_scope_unqualified_unique(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("mv"), "get", "group.shared_only")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "onlyvalue") {
		t.Errorf("expected onlyvalue, got: %q", out)
	}
}

func TestGet_scope_unqualified_ambiguous_fails(t *testing.T) {
	_, stderr, code := testutil.Run(t, bin, fix("mv"), "get", "group.key1")
	if code == 0 {
		t.Fatal("expected non-zero exit for an ambiguous key across vaults")
	}
	clean := testutil.StripANSI(stderr)
	if !testutil.Contains(clean, "vault1") || !testutil.Contains(clean, "vault2") {
		t.Errorf("expected both vaults named in ambiguity error, got: %q", stderr)
	}
}

func TestGet_scope_unknown_vault_fails(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("mv"), "get", "nope:group.shared_only")
	if code == 0 {
		t.Fatal("expected non-zero exit for an unknown vault")
	}
}
