package ward

import (
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

func mr() *MergeResult {
	leaf := func(v string) *secrets.Node { return &secrets.Node{Value: v} }
	return &MergeResult{Tree: map[string]*secrets.Node{
		"vault1": {Children: map[string]*secrets.Node{"group": {Children: map[string]*secrets.Node{
			"sub1": {Children: map[string]*secrets.Node{"key1": leaf("value1")}}}}}},
		"vault2": {Children: map[string]*secrets.Node{"group": {Children: map[string]*secrets.Node{
			"sub1": {Children: map[string]*secrets.Node{"key2": leaf("value2")}}}}}},
	}}
}

func TestEnvVarsForScopes_overlay(t *testing.T) {
	e := &Engine{}
	got, err := e.EnvVarsForScopes(mr(), false, []string{"group.sub1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["key1"]; !ok {
		t.Error("expected key1")
	}
	if _, ok := got["key2"]; !ok {
		t.Error("expected key2")
	}
	if len(got) != 2 {
		t.Errorf("expected 2 leaves, got %v", got)
	}
}

func TestEnvVarsForScopes_prefixed(t *testing.T) {
	e := &Engine{}
	got, err := e.EnvVarsForScopes(mr(), true, []string{"vault1:group.sub1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["vault1_group_sub1_key1"]; !ok {
		t.Errorf("expected full-path key, got %v", got)
	}
}
