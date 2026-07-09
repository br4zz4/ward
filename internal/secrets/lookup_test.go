package secrets

import (
	"strings"
	"testing"
)

func TestLookup_found_returns_node(t *testing.T) {
	// arrange
	tree := map[string]*Node{
		"marketing": {Children: map[string]*Node{
			"automation": {Children: map[string]*Node{
				"token": {Value: "t"},
			}},
		}},
	}

	// act
	node, err := Lookup(tree, "marketing.automation.token")

	// assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Value != "t" {
		t.Fatalf("expected value t, got %v", node.Value)
	}
}

func TestLookup_break_at_top_lists_vaults(t *testing.T) {
	// arrange
	tree := map[string]*Node{
		"marketing": {Children: map[string]*Node{}},
		"infra":     {Children: map[string]*Node{}},
	}

	// act
	_, err := Lookup(tree, "automation")

	// assert
	knf, ok := err.(*KeyNotFoundError)
	if !ok {
		t.Fatalf("expected *KeyNotFoundError, got %T", err)
	}
	if knf.AtPath != "" {
		t.Errorf("expected break at top level (empty AtPath), got %q", knf.AtPath)
	}
	msg := knf.Error()
	if !strings.Contains(msg, "available at top level") ||
		!strings.Contains(msg, "infra") || !strings.Contains(msg, "marketing") {
		t.Errorf("expected top-level vaults listed, got:\n%s", msg)
	}
}

func TestLookup_break_mid_path_lists_that_level(t *testing.T) {
	// arrange
	tree := map[string]*Node{
		"marketing": {Children: map[string]*Node{
			"automation": {Children: map[string]*Node{}},
			"web":        {Children: map[string]*Node{}},
		}},
	}

	// act
	_, err := Lookup(tree, "marketing.xpto")

	// assert
	knf, ok := err.(*KeyNotFoundError)
	if !ok {
		t.Fatalf("expected *KeyNotFoundError, got %T", err)
	}
	if knf.AtPath != "marketing" {
		t.Errorf("expected break under 'marketing', got %q", knf.AtPath)
	}
	msg := knf.Error()
	if !strings.Contains(msg, "available under marketing") ||
		!strings.Contains(msg, "automation") || !strings.Contains(msg, "web") {
		t.Errorf("expected siblings under marketing listed, got:\n%s", msg)
	}
}
