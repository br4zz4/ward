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
	ok := unsetLeaf(data, "app.db.token")

	// assert
	if !ok {
		t.Fatal("expected unsetLeaf to report success")
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
	ok := unsetLeaf(data, "app.db.missing")

	// assert
	if ok {
		t.Fatal("expected unsetLeaf to report not found")
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
