package ward

import (
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

func mr() *MergeResult {
	leaf := func(v string) *secrets.Node { return &secrets.Node{Value: v} }
	return &MergeResult{Tree: map[string]*secrets.Node{
		"commons": {Children: map[string]*secrets.Node{"infra": {Children: map[string]*secrets.Node{
			"staging": {Children: map[string]*secrets.Node{"A": leaf("1")}}}}}},
		"trgclub": {Children: map[string]*secrets.Node{"infra": {Children: map[string]*secrets.Node{
			"staging": {Children: map[string]*secrets.Node{"B": leaf("2")}}}}}},
	}}
}

func TestEnvVarsForScopes_overlay(t *testing.T) {
	e := &Engine{}
	got, err := e.EnvVarsForScopes(mr(), false, []string{"infra.staging"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["A"]; !ok {
		t.Error("expected A")
	}
	if _, ok := got["B"]; !ok {
		t.Error("expected B")
	}
	if len(got) != 2 {
		t.Errorf("expected 2 leaves, got %v", got)
	}
}

func TestEnvVarsForScopes_prefixed(t *testing.T) {
	e := &Engine{}
	got, err := e.EnvVarsForScopes(mr(), true, []string{"commons:infra.staging"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["commons_infra_staging_A"]; !ok {
		t.Errorf("expected full-path key, got %v", got)
	}
}
