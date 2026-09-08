//go:build e2e

package exec_test

import (
	"fmt"
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

// injectCheck builds a `sh -c` command that uses the injected secret without
// ever printing its value: `test "$VAR" = "want" && echo ok`. The guard allows
// it (no leak), and the test confirms injection by checking for "ok".
func injectCheck(envName, wantValue string) (sh, dashC, script string) {
	return "sh", "-c", fmt.Sprintf(`test "$%s" = "%s" && echo ok`, envName, wantValue)
}

func TestExec_injects_vars(t *testing.T) {
	sh, dashC, script := injectCheck("region", "us-east-1")
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--", sh, dashC, script)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "ok") {
		t.Errorf("expected region injected as us-east-1, got: %q", out)
	}
}

func TestExec_prefixed_injects_full_path(t *testing.T) {
	sh, dashC, script := injectCheck("deploy_main_region", "us-east-1")
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--prefixed", "--", sh, dashC, script)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "ok") {
		t.Errorf("expected deploy_main_region injected, got: %q", out)
	}
}

func TestExec_flat_does_not_have_prefixed_key(t *testing.T) {
	// deploy_main_region must NOT exist in flat mode → test fails → no "ok"
	sh, dashC, script := injectCheck("deploy_main_region", "us-east-1")
	out, _, _ := testutil.Run(t, bin, fix("basic"), "exec", "--", sh, dashC, script)
	if testutil.Contains(out, "ok") {
		t.Errorf("flat mode should not have deploy_main_region, got: %q", out)
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
	// vault-a.main.secret_key=key-from-a, vault-b.main.secret_key=key-from-b
	sh, dashC, script := injectCheck("vault_a_main_secret_key", "key-from-a")
	out, _, code := testutil.Run(t, bin, fix("conflict-file"), "exec", "--prefixed", "--", sh, dashC, script)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "ok") {
		t.Errorf("expected key-from-a from vault-a injected, got: %q", out)
	}
	sh, dashC, script = injectCheck("vault_b_main_secret_key", "key-from-b")
	out, _, code = testutil.Run(t, bin, fix("conflict-file"), "exec", "--prefixed", "--", sh, dashC, script)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "ok") {
		t.Errorf("expected key-from-b from vault-b injected, got: %q", out)
	}
}

// ── conflict-envvar ──────────────────────────────────────────────────────────

func TestExec_conflict_envvar_flat_blocked(t *testing.T) {
	// flat mode collides (staging.token vs production.token) → non-zero before running
	_, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "exec", "--", "sh", "-c", "true")
	if code == 0 {
		t.Fatal("expected non-zero exit due to env var collision")
	}
}

func TestExec_conflict_envvar_prefixed_runs(t *testing.T) {
	sh, dashC, script := injectCheck("app_staging_token", "staging-token")
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "exec", "--prefixed", "--", sh, dashC, script)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "ok") {
		t.Errorf("expected app_staging_token injected, got: %q", out)
	}
}

func TestExec_conflict_envvar_hint_runs(t *testing.T) {
	sh, dashC, script := injectCheck("token", "staging-token")
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "exec", "app:staging", "--", sh, dashC, script)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "ok") {
		t.Errorf("expected token injected from app:staging, got: %q", out)
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

	// act: a safe command still trips the structure check
	_, stderr, code := testutil.Run(t, bin, dir, "exec", "--", "sh", "-c", "true")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for structure violation")
	}
	if !strings.Contains(stderr, "vault structure violations") {
		t.Errorf("expected 'vault structure violations' in stderr, got: %s", stderr)
	}
}
