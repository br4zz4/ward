//go:build e2e

package envs_test

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

func fix(name string) string { return testutil.FixtureDir("envs", name) }

// ── basic ────────────────────────────────────────────────────────────────────

func TestEnvs_flat_keys_present(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "envs")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, key := range []string{"secret_key", "api_url", "timeout"} {
		if !testutil.Contains(testutil.StripANSI(out), key) {
			t.Errorf("expected %s in output, got: %q", key, out)
		}
	}
}

func TestEnvs_flat_value_present(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "envs")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "abc123") {
		t.Errorf("expected value abc123, got: %q", out)
	}
}

func TestEnvs_prefixed_keys(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "envs", "--prefixed")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out), "service_main_secret_key") {
		t.Errorf("expected service_main_secret_key in prefixed output, got: %q", out)
	}
}

// ── multi-vault (formerly conflict-file) ────────────────────────────────────

func TestEnvs_multi_vault_all_keys_present(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-file"), "envs", "--prefixed")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	clean := testutil.StripANSI(out)
	if !testutil.Contains(clean, "db.vault-a.internal") {
		t.Errorf("expected db.vault-a.internal in output, got: %q", out)
	}
	if !testutil.Contains(clean, "db.vault-b.internal") {
		t.Errorf("expected db.vault-b.internal in output, got: %q", out)
	}
}

// ── conflict-envvar ──────────────────────────────────────────────────────────

func TestEnvs_conflict_envvar_flat_blocked(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "envs")
	if code == 0 {
		t.Fatal("expected non-zero exit due to env var collision")
	}
}

func TestEnvs_conflict_envvar_shows_envs_examples(t *testing.T) {
	// the resolution examples must reference the command that ran (envs), not exec
	_, stderr, _ := testutil.Run(t, bin, fix("conflict-envvar"), "envs")
	clean := testutil.StripANSI(stderr)
	if !testutil.Contains(clean, "ward envs app.staging") {
		t.Errorf("expected 'ward envs' example, got: %q", stderr)
	}
	if testutil.Contains(clean, "ward exec") {
		t.Errorf("did not expect 'ward exec' examples in envs output, got: %q", stderr)
	}
}

func TestEnvs_conflict_envvar_prefixed_succeeds(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "envs", "--prefixed")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out), "app_staging_secret_key") {
		t.Errorf("expected prefixed keys, got: %q", out)
	}
}

func TestEnvs_conflict_envvar_hint_staging(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "envs", "app.staging")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "staging-secret") {
		t.Errorf("expected staging-secret, got: %q", out)
	}
}

func TestEnvs_conflict_envvar_hint_production(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "envs", "app.production")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "production-secret") {
		t.Errorf("expected production-secret, got: %q", out)
	}
}

// ── override (shadow rule) ───────────────────────────────────────────────────

func TestEnvs_override_deeper_wins(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("override"), "envs")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	// app.config.log_level (debug) shadows app.log_level (info)
	if !testutil.Contains(out, "debug") {
		t.Errorf("expected deeper log_level=debug to win, got: %q", out)
	}
}

func TestEnvs_override_max_retries_present(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("override"), "envs")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out), "max_retries") {
		t.Errorf("expected max_retries in output, got: %q", out)
	}
}

// ── prefixed (multiple dot-paths, no collision with --prefixed) ──────────────

func TestEnvs_prefixed_both_envs_present(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("prefixed"), "envs", "--prefixed")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	clean := testutil.StripANSI(out)
	if !testutil.Contains(clean, "app_main_staging_api_key") {
		t.Errorf("expected app_main_staging_api_key, got: %q", out)
	}
	if !testutil.Contains(clean, "app_main_production_api_key") {
		t.Errorf("expected app_main_production_api_key, got: %q", out)
	}
}
