//go:build e2e

package view_test

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

func fix(name string) string { return testutil.FixtureDir("view", name) }

// ── basic ────────────────────────────────────────────────────────────────────

func TestView_shows_tree(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "view")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "app") {
		t.Errorf("expected app key, got: %q", out)
	}
}

func TestView_shows_origin_arrow(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "view")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "←") {
		t.Errorf("expected origin arrow ←, got: %q", out)
	}
}

func TestView_shows_legend(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "view")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out), "active") {
		t.Errorf("expected legend with 'active', got: %q", out)
	}
}

func TestView_subtree(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "view", "app.db")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "host") {
		t.Errorf("expected host in subtree, got: %q", out)
	}
}

// ── conflict-file ────────────────────────────────────────────────────────────

func TestView_conflict_file_succeeds(t *testing.T) {
	// view always produces output even with conflicts
	out, _, code := testutil.Run(t, bin, fix("conflict-file"), "view")
	if code != 0 {
		t.Fatalf("view should exit 0 even with conflicts, got %d", code)
	}
	if !testutil.Contains(out, "←") {
		t.Errorf("expected origin arrows, got: %q", out)
	}
}

func TestView_conflict_file_shows_both_values(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-file"), "view")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	// Both conflicting values should be visible
	if !testutil.Contains(out, "key-from-a") || !testutil.Contains(out, "key-from-b") {
		t.Errorf("expected both conflict values visible, got: %q", out)
	}
}

func TestView_conflict_file_legend_shows_conflict(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-file"), "view")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out), "conflict") {
		t.Errorf("expected conflict in legend, got: %q", out)
	}
}

// ── conflict-envvar ──────────────────────────────────────────────────────────

func TestView_conflict_envvar_shows_tree(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("conflict-envvar"), "view")
	if code != 0 {
		t.Fatalf("view should exit 0 even with env collisions, got %d", code)
	}
	if !testutil.Contains(out, "token") {
		t.Errorf("expected token keys visible, got: %q", out)
	}
}

func TestView_conflict_envvar_warns(t *testing.T) {
	out, stderr, code := testutil.Run(t, bin, fix("conflict-envvar"), "view")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	combined := testutil.StripANSI(out + stderr)
	if !testutil.Contains(combined, "collision") {
		t.Errorf("expected env var collision warning, got stdout: %q stderr: %q", out, stderr)
	}
}

// ── override (shadow rule) ───────────────────────────────────────────────────

func TestView_override_shows_overridden_label(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("override"), "view")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out), "overridden") {
		t.Errorf("expected overridden label, got: %q", out)
	}
}

func TestView_override_shows_both_values(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("override"), "view")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "info") || !testutil.Contains(out, "debug") {
		t.Errorf("expected both log_level values visible, got: %q", out)
	}
}

func TestView_override_legend_shows_overrides(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("override"), "view")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out), "overrides") {
		t.Errorf("expected overrides in legend, got: %q", out)
	}
}
