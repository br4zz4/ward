//go:build e2e

package completion_test

import (
	"strings"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

func complete(t *testing.T, args ...string) (candidates []string, directive string) {
	t.Helper()
	dir := testutil.FixtureDir("catalog", "basic")
	full := append([]string{"__complete"}, args...)
	out, _, code := testutil.Run(t, bin, dir, full...)
	if code != 0 {
		t.Fatalf("exit %d: %q", code, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ":") {
			directive = line
			break
		}
		if line != "" {
			candidates = append(candidates, line)
		}
	}
	return candidates, directive
}

func TestScopeCompletion_get_empty_lists_vault_with_colon_and_nospace(t *testing.T) {
	got, directive := complete(t, "get", "")
	if len(got) != 1 || got[0] != "app:" {
		t.Errorf("expected [app:], got %v", got)
	}
	if directive != ":6" {
		t.Errorf("expected directive :6, got %q", directive)
	}
}

func TestScopeCompletion_get_vault_colon_lists_secret_paths(t *testing.T) {
	got, directive := complete(t, "get", "app:")
	want := []string{"app:main.db.host", "app:main.db.password", "app:main.db.port", "app:main.name", "app:main.region"}
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %q in %v", w, got)
		}
	}
	if directive != ":4" {
		t.Errorf("expected directive :4, got %q", directive)
	}
}

func TestScopeCompletion_get_vault_prefix_filters(t *testing.T) {
	got, _ := complete(t, "get", "app:main.db")
	if len(got) == 0 {
		t.Fatal("expected candidates")
	}
	for _, g := range got {
		if !strings.HasPrefix(g, "app:main.db") {
			t.Errorf("unexpected candidate %q", g)
		}
	}
	for _, g := range got {
		if g == "app:main.name" || g == "app:main.region" {
			t.Errorf("candidate %q should be filtered out", g)
		}
	}
}

func TestScopeCompletion_set_second_argument_lists_nothing(t *testing.T) {
	got, _ := complete(t, "set", "app:main.name", "")
	if len(got) != 0 {
		t.Errorf("expected no candidates, got %v", got)
	}
}

func TestScopeCompletion_get_unknown_vault_lists_nothing(t *testing.T) {
	got, _ := complete(t, "get", "inexistente:")
	if len(got) != 0 {
		t.Errorf("expected no candidates, got %v", got)
	}
}
