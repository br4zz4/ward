package cmd

import (
	"testing"
)

func TestUnsetLeaf_removes_existing(t *testing.T) {
	// arrange
	data := map[string]interface{}{
		"app": map[string]interface{}{"db": map[string]interface{}{
			"token": "x",
			"host":  "localhost",
		}},
	}

	// act
	result := unsetLeaf(data, "app.db.token")

	// assert
	if result != unsetRemoved {
		t.Fatalf("expected unsetRemoved, got %v", result)
	}
	db := data["app"].(map[string]interface{})["db"].(map[string]interface{})
	if _, exists := db["token"]; exists {
		t.Fatal("expected token to be removed")
	}
	if db["host"] != "localhost" {
		t.Fatal("expected sibling host to remain")
	}
}

func TestUnsetLeaf_missing_key_returns_false(t *testing.T) {
	// arrange
	data := map[string]interface{}{
		"app": map[string]interface{}{"db": map[string]interface{}{"token": "x"}},
	}

	// act
	result := unsetLeaf(data, "app.db.missing")

	// assert
	if result != unsetNotFound {
		t.Fatalf("expected unsetNotFound, got %v", result)
	}
}

func TestUnsetLeaf_group_path_is_not_removed(t *testing.T) {
	// arrange: app.db is a group (has children), not a leaf
	data := map[string]interface{}{
		"app": map[string]interface{}{"db": map[string]interface{}{
			"host": "localhost",
			"port": "5432",
		}},
	}

	// act
	result := unsetLeaf(data, "app.db")

	// assert: reports group, removes nothing
	if result != unsetIsGroup {
		t.Fatalf("expected unsetIsGroup, got %v", result)
	}
	db, ok := data["app"].(map[string]interface{})["db"].(map[string]interface{})
	if !ok || len(db) != 2 {
		t.Fatal("expected group and its children to be left intact")
	}
}

func TestUnsetLeaf_keeps_scaffold_structure(t *testing.T) {
	// arrange
	data := map[string]interface{}{
		"app": map[string]interface{}{"db": map[string]interface{}{"token": "x"}},
	}

	// act
	unsetLeaf(data, "app.db.token")

	// assert: the scaffold (app.db) is preserved so the file stays structurally valid,
	// only the leaf is gone.
	app, ok := data["app"].(map[string]interface{})
	if !ok {
		t.Fatal("expected root key 'app' to be preserved")
	}
	db, ok := app["db"].(map[string]interface{})
	if !ok {
		t.Fatal("expected scaffold key 'db' to be preserved")
	}
	if _, exists := db["token"]; exists {
		t.Fatal("expected leaf 'token' to be removed")
	}
}
