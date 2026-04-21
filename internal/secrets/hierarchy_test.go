package secrets

import (
	"testing"
)

func TestFilterByAnchor_includes_ancestor_excludes_sibling_leaf(t *testing.T) {
	// company.ward is ancestor of staging.ward (has company.sectors.one.name — a leaf)
	// production.ward is NOT ancestor (has company.sectors.one.production — a map branch
	// that staging.ward doesn't have)
	company := ParsedFile{
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
	}
	staging := ParsedFile{
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
	}
	production := ParsedFile{
		File: "production.ward",
		Data: map[string]interface{}{
			"company": map[string]interface{}{
				"sectors": map[string]interface{}{
					"one": map[string]interface{}{
						"production": map[string]interface{}{
							"database_url": "postgres://production",
						},
					},
				},
			},
		},
	}

	result := FilterByAnchor(staging, []ParsedFile{company, staging, production})

	if len(result) != 2 {
		t.Fatalf("expected 2 files (company.ward + staging.ward), got %d: %v",
			len(result), fileNames(result))
	}
	if result[0].File != "company.ward" {
		t.Errorf("expected company.ward first (ancestor), got %q", result[0].File)
	}
	if result[1].File != "staging.ward" {
		t.Errorf("expected staging.ward last (anchor), got %q", result[1].File)
	}
}

func TestFilterByAnchor_excludes_unrelated_root_key(t *testing.T) {
	anchor := ParsedFile{
		File: "anchor.ward",
		Data: map[string]interface{}{"app": map[string]interface{}{"key": "val"}},
	}
	unrelated := ParsedFile{
		File: "unrelated.ward",
		Data: map[string]interface{}{"infra": map[string]interface{}{"key": "val"}},
	}
	result := FilterByAnchor(anchor, []ParsedFile{unrelated, anchor})
	if len(result) != 1 {
		t.Fatalf("expected only anchor, got %d files", len(result))
	}
	if result[0].File != "anchor.ward" {
		t.Errorf("expected anchor.ward, got %q", result[0].File)
	}
}

func TestSortBySpecificity(t *testing.T) {
	leaf := ParsedFile{
		File: "leaf.ward",
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": "deep",
				},
			},
		},
	}
	root := ParsedFile{
		File: "root.ward",
		Data: map[string]interface{}{
			"a": "shallow",
		},
	}

	sorted := SortBySpecificity([]ParsedFile{leaf, root})
	if sorted[0].File != "root.ward" {
		t.Errorf("expected root first, got %q", sorted[0].File)
	}
	if sorted[1].File != "leaf.ward" {
		t.Errorf("expected leaf second, got %q", sorted[1].File)
	}
}

func TestFilterByAnchor_ordered_by_specificity(t *testing.T) {
	generic := ParsedFile{
		File: "generic.ward",
		Data: map[string]interface{}{
			"app": map[string]interface{}{"name": "x"},
		},
	}
	specific := ParsedFile{
		File: "specific.ward",
		Data: map[string]interface{}{
			"app": map[string]interface{}{
				"env": map[string]interface{}{"name": "x"},
			},
		},
	}
	anchor := ParsedFile{
		File: "anchor.ward",
		Data: map[string]interface{}{
			"app": map[string]interface{}{
				"env": map[string]interface{}{
					"db": "x",
				},
			},
		},
	}

	result := FilterByAnchor(anchor, []ParsedFile{specific, generic, anchor})
	if result[0].File != "generic.ward" {
		t.Errorf("expected generic first (least specific), got %q", result[0].File)
	}
	if result[len(result)-1].File != "anchor.ward" {
		t.Errorf("expected anchor last, got %q", result[len(result)-1].File)
	}
}

func fileNames(files []ParsedFile) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.File
	}
	return names
}
