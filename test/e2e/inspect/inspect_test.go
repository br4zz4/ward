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
	if !testutil.Contains(testutil.StripANSI(out), "0 conflicts") {
		t.Errorf("expected clean summary with 0 counts, got: %q", out)
	}
}

// ── multi-vault ──────────────────────────────────────────────────────────────

func TestInspect_multi_vault_clean(t *testing.T) {
	// two vaults with distinct keys → no conflict, no collision
	_, _, code := testutil.Run(t, bin, fix("conflict-file"), "inspect")
	if code != 0 {
		t.Fatalf("expected exit 0 for a clean multi-vault fixture, got %d", code)
	}
}

func TestInspect_multi_vault_collision_fails(t *testing.T) {
	// two vaults defining the same leaf name → Type-2 env var collision
	_, stderr, code := testutil.Run(t, bin, fix("multi-vault-collision"), "inspect")
	if code == 0 {
		t.Fatal("expected non-zero exit for a cross-vault env var collision")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "env var collision") {
		t.Errorf("expected env var collision message, got: %q", stderr)
	}
}

// ── conflict-envvar (Type-2 env var collision) ───────────────────────────────
// inspect surfaces env var collisions and exits non-zero, like envs/exec.

func TestInspect_conflict_envvar_fails(t *testing.T) {
	// act
	_, stderr, code := testutil.Run(t, bin, fix("conflict-envvar"), "inspect")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for env var collision")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "env var collision") {
		t.Errorf("expected env var collision message, got: %q", stderr)
	}
}

func TestInspect_conflict_envvar_prefixed_ok(t *testing.T) {
	// --prefixed models full-path names, which never collide → clean
	_, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "inspect", "--prefixed")
	if code != 0 {
		t.Fatalf("expected exit 0 with --prefixed, got %d", code)
	}
}

func TestInspect_conflict_envvar_scoped_leaf_ok(t *testing.T) {
	// scoping to one side of the collision disambiguates → clean
	_, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "inspect", "app.staging")
	if code != 0 {
		t.Fatalf("expected exit 0 when scoped to a single side, got %d", code)
	}
}

func TestInspect_conflict_envvar_scoped_parent_fails(t *testing.T) {
	// scoping to the shared parent still contains both sides → collision
	_, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "inspect", "app")
	if code == 0 {
		t.Fatal("expected non-zero exit when scoped to the shared parent")
	}
}

// ── scope: qualified inspect on a multi-vault fixture ─────────────────────────

func TestInspect_scope_qualified_clean(t *testing.T) {
	// scoping to a single vault's subtree contains no collision → clean
	_, _, code := testutil.Run(t, bin, fix("mv"), "inspect", "vault1:group")
	if code != 0 {
		t.Fatalf("expected exit 0 when scoped to vault1:group, got %d", code)
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

// ── summary footer ────────────────────────────────────────────────────────────

func TestInspect_clean_shows_summary_footer(t *testing.T) {
	// arrange + act
	out, stderr, _ := testutil.Run(t, bin, fix("clean"), "inspect")
	combined := testutil.StripANSI(out + stderr)

	// assert: footer with all-zero counts
	if !strings.Contains(combined, "0 conflicts") {
		t.Errorf("expected '0 conflicts' in summary footer, got: %q", combined)
	}
	if !strings.Contains(combined, "0 env collisions") {
		t.Errorf("expected '0 env collisions' in summary footer, got: %q", combined)
	}
	if !strings.Contains(combined, "0 structure errors") {
		t.Errorf("expected '0 structure errors' in summary footer, got: %q", combined)
	}
}

func TestInspect_env_collision_shows_summary_footer(t *testing.T) {
	// arrange + act
	_, stderr, code := testutil.Run(t, bin, fix("conflict-envvar"), "inspect")
	clean := testutil.StripANSI(stderr)

	// assert: non-zero exit and summary with env collision count
	if code == 0 {
		t.Fatal("expected non-zero exit for env var collision")
	}
	if !strings.Contains(clean, "env collision") {
		t.Errorf("expected 'env collision' count in summary footer, got: %q", clean)
	}
}

func TestInspect_structure_violation_shows_summary_footer(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("structure-violation")+"/.", dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "inspect")
	clean := testutil.StripANSI(stderr)

	// assert: non-zero exit and summary with structure error count
	if code == 0 {
		t.Fatal("expected non-zero exit for structure violation")
	}
	if !strings.Contains(clean, "structure error") {
		t.Errorf("expected 'structure error' count in summary footer, got: %q", clean)
	}
}

func TestInspect_all_errors_shows_combined_summary(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("all-errors")+"/.", dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "inspect")
	clean := testutil.StripANSI(stderr)

	// assert: non-zero exit and summary contains all three error types
	if code == 0 {
		t.Fatal("expected non-zero exit for fixture with all error types")
	}
	if !strings.Contains(clean, "conflict") {
		t.Errorf("expected 'conflict' count in summary, got: %q", clean)
	}
	if !strings.Contains(clean, "env collision") {
		t.Errorf("expected 'env collision' count in summary, got: %q", clean)
	}
	if !strings.Contains(clean, "structure error") {
		t.Errorf("expected 'structure error' count in summary, got: %q", clean)
	}
}
