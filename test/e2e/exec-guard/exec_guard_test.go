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

func TestExec_ai_mode_blocks_env(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "1")
	out, stderr, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "env")
	if code == 0 {
		t.Fatalf("expected blocked (non-zero exit), got 0")
	}
	clean := testutil.StripANSI(out+stderr)
	if !testutil.Contains(clean, "AI mode") {
		t.Errorf("expected AI mode guard message, got: %q", clean)
	}
	// secrets must not leak
	if testutil.Contains(clean, "region=us-east-1") {
		t.Errorf("secret leaked in AI mode exec: %q", clean)
	}
}

func TestExec_ai_mode_blocks_echo_var(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "1")
	_, stderr, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "sh", "-c", "echo $DEPLOY_MAIN_REGION")
	if code == 0 {
		t.Fatalf("expected blocked (non-zero exit), got 0")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "AI mode") {
		t.Errorf("expected AI mode guard message, got: %q", stderr)
	}
}

func TestExec_ai_mode_allows_normal_command(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "1")
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "sh", "-c", "echo hello")
	if code != 0 {
		t.Fatalf("expected normal command to run, got %d", code)
	}
	if !testutil.Contains(out, "hello") {
		t.Errorf("expected hello in output, got: %q", out)
	}
}

func TestExec_normal_mode_env_still_works(t *testing.T) {
	// without AI mode, env dumping is allowed (human in a shell they control)
	out, _, code := testutil.Run(t, bin, fix("basic"), "exec", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "region=us-east-1") {
		t.Errorf("expected region=us-east-1 injected, got: %q", out)
	}
}