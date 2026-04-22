//go:build e2e

package envs_test

import (
	"os"
	"testing"

	"github.com/brazza-tech/ward/test/e2e/testutil"
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
	for _, key := range []string{"SECRET_KEY", "API_URL", "TIMEOUT"} {
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
	if !testutil.Contains(testutil.StripANSI(out), "SERVICE_SECRET_KEY") {
		t.Errorf("expected SERVICE_SECRET_KEY in prefixed output, got: %q", out)
	}
}

// ── conflict-file ────────────────────────────────────────────────────────────

func TestEnvs_conflict_file_blocked(t *testing.T) {
	_, stderr, code := testutil.Run(t, bin, fix("conflict-file"), "envs")
	if code == 0 {
		t.Fatal("expected non-zero exit due to file conflict")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "conflict") {
		t.Errorf("expected conflict error, got: %q", stderr)
	}
}

func TestEnvs_conflict_file_always_errors(t *testing.T) {
	// File conflict always errors — there is no override mode
	_, stderr, code := testutil.Run(t, bin, fix("conflict-file"), "envs")
	if code == 0 {
		t.Fatal("expected non-zero exit — conflict has no automatic resolution")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "to resolve") {
		t.Errorf("expected resolution hints, got: %q", stderr)
	}
}

func TestEnvs_conflict_file_different_values_shown(t *testing.T) {
	_, stderr, code := testutil.Run(t, bin, fix("conflict-file"), "envs")
	if code == 0 {
		t.Fatal("expected conflict")
	}
	// Error should mention both files
	clean := testutil.StripANSI(stderr)
	if !testutil.Contains(clean, "vault-a") || !testutil.Contains(clean, "vault-b") {
		t.Errorf("expected both vault sources in error, got: %q", stderr)
	}
}

// ── conflict-envvar ──────────────────────────────────────────────────────────

func TestEnvs_conflict_envvar_flat_blocked(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "envs")
	if code == 0 {
		t.Fatal("expected non-zero exit due to env var collision")
	}
}

func TestEnvs_conflict_envvar_prefixed_succeeds(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "envs", "--prefixed")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out), "APP_STAGING_SECRET_KEY") {
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
	if !testutil.Contains(testutil.StripANSI(out), "MAX_RETRIES") {
		t.Errorf("expected MAX_RETRIES in output, got: %q", out)
	}
}

// ── prefixed (multiple dot-paths, no collision with --prefixed) ──────────────

func TestEnvs_prefixed_both_envs_present(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("prefixed"), "envs", "--prefixed")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	clean := testutil.StripANSI(out)
	if !testutil.Contains(clean, "APP_STAGING_API_KEY") {
		t.Errorf("expected APP_STAGING_API_KEY, got: %q", out)
	}
	if !testutil.Contains(clean, "APP_PRODUCTION_API_KEY") {
		t.Errorf("expected APP_PRODUCTION_API_KEY, got: %q", out)
	}
}
