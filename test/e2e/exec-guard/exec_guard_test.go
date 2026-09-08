//go:build e2e

package exec_guard_test

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

func fix(name string) string { return testutil.FixtureDir("exec", name) }

// The guard is UNCONDITIONAL: env/echo $VAR are blocked even without AI mode.

func TestExec_blocks_env(t *testing.T) {
	out, stderr, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "env")
	if code == 0 {
		t.Fatalf("expected blocked (non-zero exit), got 0")
	}
	clean := testutil.StripANSI(out+stderr)
	if !testutil.Contains(clean, "blocked") {
		t.Errorf("expected guard message, got: %q", clean)
	}
	// secrets must not leak
	if testutil.Contains(clean, "region=us-east-1") {
		t.Errorf("secret leaked in blocked exec: %q", clean)
	}
}

func TestExec_blocks_echo_var(t *testing.T) {
	_, stderr, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "sh", "-c", "echo $region")
	if code == 0 {
		t.Fatalf("expected blocked (non-zero exit), got 0")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "blocked") {
		t.Errorf("expected guard message, got: %q", stderr)
	}
}

func TestExec_blocks_env_in_ai_mode_tool(t *testing.T) {
	// MCP server spawns subprocesses with WARD_AI_MODE=1; the guard is the same.
	t.Setenv("WARD_AI_MODE", "1")
	_, stderr, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "printenv")
	if code == 0 {
		t.Fatalf("expected blocked (non-zero exit), got 0")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "blocked") {
		t.Errorf("expected guard message, got: %q", stderr)
	}
}

func TestExec_allows_normal_command(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "sh", "-c", "echo hello")
	if code != 0 {
		t.Fatalf("expected normal command to run, got %d", code)
	}
	if !testutil.Contains(out, "hello") {
		t.Errorf("expected hello in output, got: %q", out)
	}
}

func TestExec_allows_secret_use_without_printing(t *testing.T) {
	// the intended pattern: use the secret inside sh -c without printing it
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "sh", "-c", `test "$region" = "us-east-1" && echo ok`)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "ok") {
		t.Errorf("expected ok (secret used, not printed), got: %q", out)
	}
}