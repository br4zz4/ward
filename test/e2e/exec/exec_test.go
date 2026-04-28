//go:build e2e

package exec_test

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
	if !testutil.Contains(out, "DEPLOY_MAIN_REGION=us-east-1") {
		t.Errorf("expected DEPLOY_MAIN_REGION=us-east-1, got: %q", out)
	}
}

func TestExec_flat_does_not_have_prefixed_key(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if testutil.Contains(out, "DEPLOY_MAIN_REGION=") {
		t.Errorf("flat mode should not have DEPLOY_MAIN_REGION, got: %q", out)
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

// ── multi-vault (formerly conflict-file) ────────────────────────────────────

func TestExec_multi_vault_injects_both(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-file"), "exec", "--prefixed", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "key-from-a") {
		t.Errorf("expected key-from-a from vault-a, got: %q", out)
	}
	if !testutil.Contains(out, "key-from-b") {
		t.Errorf("expected key-from-b from vault-b, got: %q", out)
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

// ── working directory ────────────────────────────────────────────────────────

func TestExec_runs_in_caller_working_directory(t *testing.T) {
	// arrange
	subdir := fix("subdir") + "/workdir"

	// act: run ward exec from a subdirectory of the fixture
	out, _, code := testutil.Run(t, bin, subdir, "exec", "--", "pwd")

	// assert
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "workdir") {
		t.Errorf("expected command to run in caller working directory (workdir), got: %q", out)
	}
}

// ── structure-violation ───────────────────────────────────────────────────────

func TestExec_structure_violation_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("structure-violation")+"/.", dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "exec", "--", "env")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for structure violation")
	}
	if !strings.Contains(stderr, "vault structure violations") {
		t.Errorf("expected 'vault structure violations' in stderr, got: %s", stderr)
	}
}
