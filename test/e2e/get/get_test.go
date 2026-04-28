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
	out, _, code := testutil.Run(t, bin, fix("basic"), "get", "app.main.name")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "my-service") {
		t.Errorf("expected my-service, got: %q", out)
	}
}

func TestGet_nested_value(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "get", "app.main.db.host")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "localhost") {
		t.Errorf("expected localhost, got: %q", out)
	}
}

func TestGet_numeric_value(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "get", "app.main.port")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "8080") {
		t.Errorf("expected 8080, got: %q", out)
	}
}

func TestGet_missing_key_fails(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("missing-key"), "get", "app.main.nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit for missing key")
	}
}

func TestGet_no_args_fails(t *testing.T) {
	_, stderr, code := testutil.Run(t, bin, fix("basic"), "get")
	if code == 0 {
		t.Fatal("expected non-zero exit when no args")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "missing dot-path") {
		t.Errorf("expected missing dot-path error, got: %q", stderr)
	}
}

func TestGet_multi_vault_vault_a_key(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-file"), "get", "vault-a.main.vault_a_only")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "value-from-a") {
		t.Errorf("expected value-from-a, got: %q", out)
	}
}

func TestGet_conflict_envvar_specific_path(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "get", "app.staging.token")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "staging-token") {
		t.Errorf("expected staging-token, got: %q", out)
	}
}

func TestGet_conflict_envvar_other_path(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "get", "app.production.token")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "production-token") {
		t.Errorf("expected production-token, got: %q", out)
	}
}
