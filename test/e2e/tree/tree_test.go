//go:build e2e

package tree_test

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

func fix(name string) string { return testutil.FixtureDir("tree", name) }

func TestTree_shows_tree_with_origin(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "tree")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "app") || !testutil.Contains(out, "←") {
		t.Errorf("expected tree with origin arrow, got: %q", out)
	}
}

func TestTree_subtree(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "tree", "app.main.db")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "host") {
		t.Errorf("expected host in subtree, got: %q", out)
	}
}

func TestTree_long_value_truncated(t *testing.T) {
	// arrange + act
	out, _, code := testutil.Run(t, bin, fix("long-value"), "tree")

	// assert: exits clean and the long value is truncated with ellipsis
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	clean := testutil.StripANSI(out)
	if !testutil.Contains(clean, "…") {
		t.Errorf("expected long value to be truncated with ellipsis, got: %q", clean)
	}
	// short key must still appear untruncated
	if !testutil.Contains(clean, "hello") {
		t.Errorf("expected short_key value 'hello' present, got: %q", clean)
	}
	// origin annotation must appear on every leaf line
	if !testutil.Contains(clean, "←") {
		t.Errorf("expected origin arrow ← in output, got: %q", clean)
	}
}

func TestTree_long_value_arrow_not_pushed_far_right(t *testing.T) {
	// arrange + act
	out, _, code := testutil.Run(t, bin, fix("long-value"), "tree")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	// assert: no line is longer than 200 visible chars before the ← arrow
	clean := testutil.StripANSI(out)
	for _, line := range strings.Split(clean, "\n") {
		idx := strings.Index(line, "←")
		if idx < 0 {
			continue
		}
		if idx > 200 {
			t.Errorf("← pushed too far right (%d cols); long values should be truncated: %q", idx, line)
		}
	}
}

func TestTree_conflict_envvar_warns(t *testing.T) {
	out, stderr, code := testutil.Run(t, bin, fix("conflict-envvar"), "tree")
	if code != 0 {
		t.Fatalf("tree should exit 0 even with env collisions, got %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out+stderr), "collision") {
		t.Errorf("expected collision warning, got: %q / %q", out, stderr)
	}
}
