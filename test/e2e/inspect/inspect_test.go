//go:build e2e

package inspect_test

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

func fix(name string) string { return testutil.FixtureDir("inspect", name) }

// ── clean ─────────────────────────────────────────────────────────────────────

func TestInspect_clean_exits_zero(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("clean"), "inspect")
	if code != 0 {
		t.Fatalf("expected exit 0 for clean fixture, got %d", code)
	}
}

func TestInspect_clean_shows_checkmark(t *testing.T) {
	out, _, _ := testutil.Run(t, bin, fix("clean"), "inspect")
	if !testutil.Contains(testutil.StripANSI(out), "no conflicts") {
		t.Errorf("expected no conflicts message, got: %q", out)
	}
}

// ── multi-vault (formerly conflict-file) ─────────────────────────────────────

func TestInspect_multi_vault_clean(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fix("conflict-file"), "inspect")
	if code != 0 {
		t.Fatalf("expected exit 0 for multi-vault fixture, got %d", code)
	}
}

// ── conflict-envvar ──────────────────────────────────────────────────────────
// inspect only detects file conflicts, not env var collisions

func TestInspect_conflict_envvar_exits_zero(t *testing.T) {
	// env var collision is not a merge conflict — inspect passes
	_, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "inspect")
	if code != 0 {
		t.Fatalf("inspect should pass for env var collision (not a merge conflict), got %d", code)
	}
}

// ── structure-violation ───────────────────────────────────────────────────────

func TestInspect_structure_violation_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("structure-violation")+"/.", dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "inspect")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for structure violation")
	}
	if !strings.Contains(stderr, "vault structure violations") {
		t.Errorf("expected 'vault structure violations' in stderr, got: %s", stderr)
	}
}
