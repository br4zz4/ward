//go:build integration

package exec_test

import (
	"os"
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

func fix(name string) string { return testutil.FixtureDir("exec", name) }

// ── basic ────────────────────────────────────────────────────────────────────

func TestExec_injects_vars(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "REGION=us-east-1") {
		t.Errorf("expected REGION=us-east-1 injected, got: %q", out)
	}
}

func TestExec_prefixed_injects_full_path(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--prefixed", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "DEPLOY_REGION=us-east-1") {
		t.Errorf("expected DEPLOY_REGION=us-east-1, got: %q", out)
	}
}

func TestExec_flat_does_not_have_prefixed_key(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if testutil.Contains(out, "DEPLOY_REGION=") {
		t.Errorf("flat mode should not have DEPLOY_REGION, got: %q", out)
	}
}

// ── exit-code propagation ────────────────────────────────────────────────────

func TestExec_propagates_exit_code(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("exit-code"), "exec", "--", "sh", "-c", "exit 42")
	if code != 42 {
		t.Errorf("expected exit code 42, got %d", code)
	}
}

func TestExec_propagates_exit_zero(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("exit-code"), "exec", "--", "sh", "-c", "exit 0")
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// ── conflict-file ────────────────────────────────────────────────────────────

func TestExec_conflict_file_blocked(t *testing.T) {
	_, stderr, code := testutil.Run(t, bin, fix("conflict-file"), "exec", "--", "env")
	if code == 0 {
		t.Fatal("expected non-zero exit due to file conflict")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "conflict") {
		t.Errorf("expected conflict error, got: %q", stderr)
	}
}

func TestExec_conflict_file_always_errors(t *testing.T) {
	// File conflict always errors — there is no override mode
	_, stderr, code := testutil.Run(t, bin, fix("conflict-file"), "exec", "--", "env")
	if code == 0 {
		t.Fatal("expected non-zero exit — conflict has no automatic resolution")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "to resolve") {
		t.Errorf("expected resolution hints, got: %q", stderr)
	}
}

func TestExec_conflict_file_error_mentions_both_files(t *testing.T) {
	_, stderr, _ := testutil.Run(t, bin, fix("conflict-file"), "exec", "--", "env")
	clean := testutil.StripANSI(stderr)
	if !testutil.Contains(clean, "vault-a") || !testutil.Contains(clean, "vault-b") {
		t.Errorf("expected both vault sources in error, got: %q", stderr)
	}
}

// ── conflict-envvar ──────────────────────────────────────────────────────────

func TestExec_conflict_envvar_flat_blocked(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "exec", "--", "env")
	if code == 0 {
		t.Fatal("expected non-zero exit due to env var collision")
	}
}

func TestExec_conflict_envvar_prefixed_runs(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "exec", "--prefixed", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "APP_STAGING_TOKEN=staging-token") {
		t.Errorf("expected APP_STAGING_TOKEN injected, got: %q", out)
	}
}

func TestExec_conflict_envvar_hint_runs(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "exec", "app.staging", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "TOKEN=staging-token") {
		t.Errorf("expected TOKEN=staging-token injected, got: %q", out)
	}
}
