//go:build e2e

package tree_test

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

func TestTree_conflict_envvar_warns(t *testing.T) {
	out, stderr, code := testutil.Run(t, bin, fix("conflict-envvar"), "tree")
	if code != 0 {
		t.Fatalf("tree should exit 0 even with env collisions, got %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out+stderr), "collision") {
		t.Errorf("expected collision warning, got: %q / %q", out, stderr)
	}
}
