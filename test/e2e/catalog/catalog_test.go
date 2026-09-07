//go:build e2e

package catalog_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

var bin string

func TestMain(m *testing.M) {
	b, err := testutil.BuildBin()
	if err != nil {
		panic(err)
	}
	bin = b
	code := m.Run()
	os.Remove(b)
	os.Exit(code)
}

func fix(name string) string { return testutil.FixtureDir("catalog", name) }

func TestCatalog_shows_paths_with_metadata(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "catalog")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	clean := testutil.StripANSI(out)
	// paths are shown
	if !testutil.Contains(clean, "app.main.name") {
		t.Errorf("expected path app.main.name in output, got: %q", clean)
	}
	if !testutil.Contains(clean, "app.main.db.password") {
		t.Errorf("expected path app.main.db.password in output, got: %q", clean)
	}
	// descriptions from _meta are shown
	if !testutil.Contains(clean, "Nombre de la aplicación") {
		t.Errorf("expected description, got: %q", clean)
	}
	// types from _meta are shown
	if !testutil.Contains(clean, "number") {
		t.Errorf("expected type number, got: %q", clean)
	}
	// values must NEVER appear
	if testutil.Contains(clean, "supersecret") {
		t.Errorf("secret value leaked in catalog: %q", clean)
	}
}

func TestCatalog_json_is_structured(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "catalog", "--json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var entries []map[string]string
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("expected valid JSON array, got: %q (%v)", out, err)
	}
	found := false
	for _, e := range entries {
		if e["path"] == "app.main.db.password" {
			found = true
			if e["description"] == "" {
				t.Errorf("expected description for app.main.db.password, got: %v", e)
			}
			if e["type"] != "string" {
				t.Errorf("expected type string, got: %q", e["type"])
			}
		}
	}
	if !found {
		t.Errorf("expected app.main.db.password in JSON entries, got: %q", out)
	}
}

func TestCatalog_paths_without_meta_show_empty_description(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "catalog")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	clean := testutil.StripANSI(out)
	// app.main.region has no _meta entry — must still be listed (path visible)
	if !testutil.Contains(clean, "app.main.region") {
		t.Errorf("expected undocumented path app.main.region in output, got: %q", clean)
	}
}