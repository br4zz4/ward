package cmd

import (
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

func TestResolveTargetFiles_single_file(t *testing.T) {
	// arrange
	files := []secrets.ParsedFile{
		{File: "a.ward", Data: map[string]interface{}{
			"app": map[string]interface{}{"db": map[string]interface{}{"token": "x"}},
		}},
		{File: "b.ward", Data: map[string]interface{}{
			"app": map[string]interface{}{"api": map[string]interface{}{"key": "y"}},
		}},
	}

	// act
	got := resolveTargetFiles(files, "app.db.token")

	// assert
	if len(got) != 1 || got[0] != "a.ward" {
		t.Fatalf("expected [a.ward], got %v", got)
	}
}

func TestResolveTargetFiles_no_file(t *testing.T) {
	// arrange
	files := []secrets.ParsedFile{
		{File: "a.ward", Data: map[string]interface{}{
			"app": map[string]interface{}{"db": map[string]interface{}{"token": "x"}},
		}},
	}

	// act
	got := resolveTargetFiles(files, "app.new.key")

	// assert
	if len(got) != 0 {
		t.Fatalf("expected no files, got %v", got)
	}
}

func TestResolveTargetFiles_conflict_multiple_files(t *testing.T) {
	// arrange
	files := []secrets.ParsedFile{
		{File: "a.ward", Data: map[string]interface{}{
			"app": map[string]interface{}{"db": map[string]interface{}{"token": "x"}},
		}},
		{File: "b.ward", Data: map[string]interface{}{
			"app": map[string]interface{}{"db": map[string]interface{}{"token": "y"}},
		}},
	}

	// act
	got := resolveTargetFiles(files, "app.db.token")

	// assert
	if len(got) != 2 {
		t.Fatalf("expected 2 conflicting files, got %v", got)
	}
}

func TestSetLeaf_updates_existing(t *testing.T) {
	// arrange
	data := map[string]interface{}{
		"app": map[string]interface{}{"db": map[string]interface{}{"token": "old"}},
	}

	// act
	setLeaf(data, "app.db.token", "new")

	// assert
	got := data["app"].(map[string]interface{})["db"].(map[string]interface{})["token"]
	if got != "new" {
		t.Fatalf("expected token=new, got %v", got)
	}
}

func TestSetLeaf_creates_intermediate_maps(t *testing.T) {
	// arrange
	data := map[string]interface{}{
		"app": map[string]interface{}{},
	}

	// act
	setLeaf(data, "app.new.deep.key", "v")

	// assert
	got := data["app"].(map[string]interface{})["new"].(map[string]interface{})["deep"].(map[string]interface{})["key"]
	if got != "v" {
		t.Fatalf("expected key=v, got %v", got)
	}
}

func TestEnvCollisionFor_detects_type2(t *testing.T) {
	// arrange: two different dot-paths whose leaf both map to env var TOKEN
	tree := map[string]*secrets.Node{
		"app": {Children: map[string]*secrets.Node{
			"staging": {Children: map[string]*secrets.Node{
				"token": {Value: "a", Origin: secrets.Origin{File: "s.ward"}},
			}},
			"prod": {Children: map[string]*secrets.Node{
				"token": {Value: "b", Origin: secrets.Origin{File: "p.ward"}},
			}},
		}},
	}

	// act
	collision := envCollisionFor(tree, "app.prod.token")

	// assert
	if collision == nil {
		t.Fatal("expected a Type-2 collision for app.prod.token")
	}
	if collision.EnvKey != "token" {
		t.Fatalf("expected env key 'token', got %q", collision.EnvKey)
	}
}

func TestEnvCollisionFor_no_collision(t *testing.T) {
	// arrange: unique env var names, no collision
	tree := map[string]*secrets.Node{
		"app": {Children: map[string]*secrets.Node{
			"db": {Children: map[string]*secrets.Node{
				"host": {Value: "localhost", Origin: secrets.Origin{File: "a.ward"}},
			}},
		}},
	}

	// act
	collision := envCollisionFor(tree, "app.db.host")

	// assert
	if collision != nil {
		t.Fatalf("expected no collision, got %+v", collision)
	}
}

