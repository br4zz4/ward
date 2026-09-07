package secrets_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/sops"
)

// writeWardPlain writes a plain .ward file for tests.
func writeWardPlain(t *testing.T, base, rel, content string) string {
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

func TestLoad_meta_extracted_from_data(t *testing.T) {
	dir := t.TempDir()
	path := writeWardPlain(t, dir, "app.plain.ward", `
app:
  db:
    host: localhost
    password: secret
_meta:
  app.db.host:
    description: "Hostname del banco"
    type: string
  app.db.password:
    description: "Password del banco"
    type: string
`)

	pf, err := secrets.Load(path, "app", dir, sops.MockDecryptor{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// _meta must NOT be part of the secret data (would merge/env/leak as a secret)
	if _, ok := pf.Data["_meta"]; ok {
		t.Errorf("_meta leaked into Data: %v", pf.Data)
	}
	// ... but must be captured in Meta
	meta, ok := pf.Meta["app.db.host"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected meta entry for app.db.host, got %T", pf.Meta["app.db.host"])
	}
	if meta["description"] != "Hostname del banco" {
		t.Errorf("expected description, got %v", meta["description"])
	}
}

func TestLoad_meta_absent_yields_empty_meta(t *testing.T) {
	dir := t.TempDir()
	path := writeWardPlain(t, dir, "app.plain.ward", "app:\n  db:\n    host: localhost\n")

	pf, err := secrets.Load(path, "app", dir, sops.MockDecryptor{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Meta) != 0 {
		t.Errorf("expected empty Meta, got %v", pf.Meta)
	}
}

func TestMerge_meta_does_not_reach_merged_tree(t *testing.T) {
	dir := t.TempDir()
	_ = writeWardPlain(t, dir, "app.plain.ward", `
app:
  db:
    host: localhost
_meta:
  app.db.host:
    description: "Hostname del banco"
`)

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, nil, func(_ string) sops.Decryptor { return sops.MockDecryptor{} })
	tree, err := secrets.Merge(secrets.SortBySpecificity(files), config.MergeModeError, "")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, ok := tree["_meta"]; ok {
		t.Errorf("_meta leaked into merged tree: %v", tree)
	}
	if _, err := secrets.Lookup(tree, "app.db.host"); err != nil {
		t.Errorf("expected app.db.host to still exist: %v", err)
	}
}