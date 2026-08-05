package secrets

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseScope(t *testing.T) {
	cases := []struct {
		in         string
		wantVault  string
		wantSecret string
	}{
		{"vault1:group.sub1", "vault1", "group.sub1"},
		{"group.sub1", "", "group.sub1"},
		{":group.sub1", "", "group.sub1"},
		{"", "", ""},
		{"vault1:", "vault1", ""},
		{"group.sub1:weird", "", "group.sub1:weird"},
	}
	for _, c := range cases {
		got := ParseScope(c.in)
		if got.Vault != c.wantVault || got.SecretPath != c.wantSecret {
			t.Errorf("ParseScope(%q) = {%q,%q}, want {%q,%q}",
				c.in, got.Vault, got.SecretPath, c.wantVault, c.wantSecret)
		}
	}
}

func mkTree() map[string]*Node {
	leaf := func(v string) *Node { return &Node{Value: v} }
	return map[string]*Node{
		"vault1": {Children: map[string]*Node{
			"group": {Children: map[string]*Node{
				"sub1": {Children: map[string]*Node{"key1": leaf("value1")}},
				"sub2": {Children: map[string]*Node{"key1": leaf("value2")}},
			}},
		}},
		"vault2": {Children: map[string]*Node{
			"group": {Children: map[string]*Node{
				"sub1": {Children: map[string]*Node{"key2": leaf("value3")}},
			}},
		}},
	}
}

func rootPaths(rs []ScopedRoot) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.DotPath
	}
	sort.Strings(out)
	return out
}

func TestResolveScopes_qualified(t *testing.T) {
	rs, err := ResolveScopes(mkTree(), []Scope{{Vault: "vault1", SecretPath: "group.sub1"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := rootPaths(rs); len(got) != 1 || got[0] != "vault1.group.sub1" {
		t.Errorf("got %v", got)
	}
}

func TestResolveScopes_qualified_unknown_vault(t *testing.T) {
	_, err := ResolveScopes(mkTree(), []Scope{{Vault: "nope", SecretPath: "group.sub1"}})
	if err == nil {
		t.Fatal("expected error for unknown vault")
	}
}

func TestResolveScopes_unqualified_overlay(t *testing.T) {
	rs, err := ResolveScopes(mkTree(), []Scope{{SecretPath: "group.sub1"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"vault1.group.sub1", "vault2.group.sub1"}
	if got := rootPaths(rs); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestResolveScopes_unqualified_no_hit(t *testing.T) {
	_, err := ResolveScopes(mkTree(), []Scope{{SecretPath: "nope.nope"}})
	if err == nil {
		t.Fatal("expected key-not-found")
	}
}

func TestResolveScopes_depth1_only(t *testing.T) {
	tree := map[string]*Node{
		"vault1": {Children: map[string]*Node{
			"regions": {Children: map[string]*Node{
				"us": {Children: map[string]*Node{
					"group": {Children: map[string]*Node{
						"sub1": {Children: map[string]*Node{"X": {Value: "deep"}}},
					}},
				}},
			}},
		}},
	}
	_, err := ResolveScopes(tree, []Scope{{SecretPath: "group.sub1"}})
	if err == nil {
		t.Fatal("deep match must not resolve at depth 1")
	}
}

func TestResolveScopes_multi_union_dedup(t *testing.T) {
	rs, err := ResolveScopes(mkTree(), []Scope{
		{Vault: "vault1", SecretPath: "group.sub1"},
		{Vault: "vault1", SecretPath: "group.sub1"},
		{Vault: "vault2", SecretPath: "group.sub1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := rootPaths(rs); len(got) != 2 {
		t.Errorf("expected 2 deduped roots, got %v", got)
	}
}

func TestResolveScopes_empty_whole_tree(t *testing.T) {
	rs, err := ResolveScopes(mkTree(), []Scope{{SecretPath: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].DotPath != "" {
		t.Errorf("expected whole-tree root, got %v", rootPaths(rs))
	}
}
