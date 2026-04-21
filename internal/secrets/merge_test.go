package secrets

import (
	"errors"
	"testing"

	"github.com/oporpino/ward/internal/config"
)

func strVal(n *Node) string {
	if n == nil {
		return "<nil>"
	}
	s, _ := n.Value.(string)
	return s
}

func TestMerge_deep_merge(t *testing.T) {
	files := []ParsedFile{
		{
			File: "company.ward",
			Data: map[string]interface{}{
				"company": map[string]interface{}{
					"name": "acme",
					"sectors": map[string]interface{}{
						"one": map[string]interface{}{
							"name": "sector 1",
						},
					},
				},
			},
		},
		{
			File: "company/sectors/one/staging.ward",
			Data: map[string]interface{}{
				"company": map[string]interface{}{
					"sectors": map[string]interface{}{
						"one": map[string]interface{}{
							"name":         "override sector 1",
							"database_url": "postgres://staging",
						},
					},
				},
			},
		},
	}

	tree, err := Merge(files, config.MergeModeDeep, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	company := tree["company"].Children
	// name at company level preserved
	if strVal(company["name"]) != "acme" {
		t.Errorf("company.name: got %q, want %q", strVal(company["name"]), "acme")
	}

	one := company["sectors"].Children["one"].Children
	// leaf overrides ancestor
	if strVal(one["name"]) != "override sector 1" {
		t.Errorf("one.name: got %q, want %q", strVal(one["name"]), "override sector 1")
	}
	if strVal(one["database_url"]) != "postgres://staging" {
		t.Errorf("one.database_url: got %q", strVal(one["database_url"]))
	}
}

func TestMerge_leaf_wins_over_ancestor(t *testing.T) {
	files := []ParsedFile{
		{File: "base.ward", Data: map[string]interface{}{"key": "base"}},
		{File: "leaf.ward", Data: map[string]interface{}{"key": "leaf"}},
	}
	tree, err := Merge(files, config.MergeModeDeep, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strVal(tree["key"]) != "leaf" {
		t.Errorf("expected leaf to win, got %q", strVal(tree["key"]))
	}
	if tree["key"].Origin.File != "leaf.ward" {
		t.Errorf("expected origin leaf.ward, got %q", tree["key"].Origin.File)
	}
}

func TestMerge_conflict_same_level(t *testing.T) {
	// staging and production are siblings — same ancestry level
	// both define company.sectors.one.database_url → conflict
	files := []ParsedFile{
		{
			File: "staging.ward",
			Data: map[string]interface{}{
				"company": map[string]interface{}{
					"sectors": map[string]interface{}{
						"one": map[string]interface{}{
							"staging": map[string]interface{}{
								"database_url": "postgres://staging",
							},
						},
					},
				},
			},
		},
		{
			File: "production.ward",
			Data: map[string]interface{}{
				"company": map[string]interface{}{
					"sectors": map[string]interface{}{
						"one": map[string]interface{}{
							"staging": map[string]interface{}{
								"database_url": "postgres://production",
							},
						},
					},
				},
			},
		},
	}

	_, err := Merge(files, config.MergeModeError, "")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConflictError, got %T: %v", err, err)
	}
	if len(ce.Conflicts) == 0 {
		t.Fatal("expected at least one conflict")
	}
	if ce.Conflicts[0].Key != "company.sectors.one.staging.database_url" {
		t.Errorf("unexpected conflict key: %q", ce.Conflicts[0].Key)
	}
}

func TestMerge_override_mode(t *testing.T) {
	files := []ParsedFile{
		{File: "a.ward", Data: map[string]interface{}{"key": "first"}},
		{File: "b.ward", Data: map[string]interface{}{"key": "second"}},
	}
	tree, err := Merge(files, config.MergeModeOverride, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strVal(tree["key"]) != "second" {
		t.Errorf("expected second, got %q", strVal(tree["key"]))
	}
}

func TestMerge_error_mode_conflict(t *testing.T) {
	files := []ParsedFile{
		{File: "a.ward", Data: map[string]interface{}{"key": "x"}},
		{File: "b.ward", Data: map[string]interface{}{"key": "y"}},
	}
	_, err := Merge(files, config.MergeModeError, "")
	if err == nil {
		t.Fatal("expected error")
	}
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected ConflictError, got %T", err)
	}
}

func TestMerge_no_files(t *testing.T) {
	tree, err := Merge(nil, config.MergeModeDeep, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tree) != 0 {
		t.Errorf("expected empty tree")
	}
}

func TestMerge_origin_tracked(t *testing.T) {
	files := []ParsedFile{
		{File: "base.ward", Data: map[string]interface{}{"x": "1"}},
	}
	tree, err := Merge(files, config.MergeModeDeep, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tree["x"].Origin.File != "base.ward" {
		t.Errorf("expected origin base.ward, got %q", tree["x"].Origin.File)
	}
}
