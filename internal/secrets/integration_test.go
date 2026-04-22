package secrets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/brazza-tech/ward/internal/config"
	"github.com/brazza-tech/ward/internal/secrets"
	"github.com/brazza-tech/ward/internal/sops"
)

// writeWard creates a .ward file (plain YAML for tests) at the given relative path.
func writeWard(t *testing.T, base, rel, content string) string {
	t.Helper()
	full := filepath.Join(base, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return full
}

func TestIntegration_discover_and_merge(t *testing.T) {
	dir := t.TempDir()
	dec := sops.MockDecryptor{}

	writeWard(t, dir, "company.ward", `
company:
  name: acme
  sectors:
    one:
      name: sector 1
`)
	writeWard(t, dir, "company/sectors/one/staging.ward", `
company:
  sectors:
    one:
      staging:
        database_url: "postgres://staging"
        redis_url: "redis://staging"
`)

	paths, err := secrets.Discover([]string{dir})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(paths), paths)
	}

	files, err := secrets.LoadAll(paths, dec)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Use staging as anchor
	var anchor secrets.ParsedFile
	for _, f := range files {
		if filepath.Base(f.File) == "staging.ward" {
			anchor = f
		}
	}

	ordered := secrets.FilterByAnchor(anchor, files)
	if len(ordered) != 2 {
		t.Fatalf("expected 2 files in ancestor line, got %d", len(ordered))
	}

	tree, err := secrets.Merge(ordered, config.MergeModeError, "")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	// company.name from ancestor
	company := tree["company"].Children
	if v, _ := company["name"].Value.(string); v != "acme" {
		t.Errorf("company.name: got %q, want acme", v)
	}

	// staging keys from anchor
	staging := company["sectors"].Children["one"].Children["staging"].Children
	if v, _ := staging["database_url"].Value.(string); v != "postgres://staging" {
		t.Errorf("database_url: got %q", v)
	}
	if v, _ := staging["redis_url"].Value.(string); v != "redis://staging" {
		t.Errorf("redis_url: got %q", v)
	}
}

func TestIntegration_anchor_includes_all_ancestors(t *testing.T) {
	dir := t.TempDir()
	dec := sops.MockDecryptor{}

	// company.ward is ancestor of staging.ward (shares company.* content)
	// production.ward has unrelated root key — excluded
	writeWard(t, dir, "company.ward", `
company:
  name: acme
`)
	writeWard(t, dir, "staging.ward", `
company:
  staging:
    database_url: "postgres://staging"
`)
	writeWard(t, dir, "unrelated.ward", `
infra:
  region: us-east-1
`)

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, dec)

	var anchor secrets.ParsedFile
	for _, f := range files {
		if filepath.Base(f.File) == "staging.ward" {
			anchor = f
		}
	}

	ordered := secrets.FilterByAnchor(anchor, files)

	// unrelated.ward should NOT be included (different root key)
	for _, f := range ordered {
		if filepath.Base(f.File) == "unrelated.ward" {
			t.Errorf("unrelated.ward should not be loaded (different root key)")
		}
	}

	// company.ward should be included (ancestor)
	found := false
	for _, f := range ordered {
		if filepath.Base(f.File) == "company.ward" {
			found = true
		}
	}
	if !found {
		t.Error("company.ward should be included as ancestor of staging.ward")
	}
}

func TestIntegration_anchor_conflict_detected_by_merge(t *testing.T) {
	dir := t.TempDir()
	dec := sops.MockDecryptor{}

	// staging and production both define company.env — conflict detected at merge time
	writeWard(t, dir, "staging.ward", `
company:
  env: staging
  database_url: "postgres://staging"
`)
	writeWard(t, dir, "production.ward", `
company:
  env: production
  database_url: "postgres://production"
`)

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, dec)
	sorted := secrets.SortBySpecificity(files)

	_, err := secrets.Merge(sorted, config.MergeModeError, "")
	if err == nil {
		t.Fatal("expected conflict error for duplicate key company.env")
	}
}

func TestIntegration_conflict_same_level(t *testing.T) {
	dir := t.TempDir()
	dec := sops.MockDecryptor{}

	// Two files at the same level defining the same key
	writeWard(t, dir, "a.ward", `app:
  database_url: "postgres://a"
`)
	writeWard(t, dir, "b.ward", `app:
  database_url: "postgres://b"
`)

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, dec)
	sorted := secrets.SortBySpecificity(files)

	_, err := secrets.Merge(sorted, config.MergeModeError, "")
	if err == nil {
		t.Fatal("expected conflict error for same-level duplicate key")
	}
}

func TestIntegration_leaf_origin_tracked(t *testing.T) {
	dir := t.TempDir()
	dec := sops.MockDecryptor{}

	writeWard(t, dir, "base.ward", `app:
  name: myapp
`)

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, dec)
	sorted := secrets.SortBySpecificity(files)

	tree, err := secrets.Merge(sorted, config.MergeModeError, "")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	node := tree["app"].Children["name"]
	if node.Origin.File == "" {
		t.Error("expected origin to be set")
	}
}
