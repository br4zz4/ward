package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/sops"
	"github.com/br4zz4/ward/internal/ward"
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

func TestSet_new_key_preserves_existing_keys_in_file(t *testing.T) {
	// arrange: project root with a vault dir and a .ward file that already has two keys.
	// vaultRelDir must be relative so resolveNewPath (which joins projectRoot+vaultPath)
	// produces a path inside root without duplication.
	root := t.TempDir()
	vaultRelDir := "vault"
	vaultAbsDir := filepath.Join(root, vaultRelDir)
	wardFile := filepath.Join(vaultAbsDir, "staging.ward")
	if err := os.MkdirAll(vaultAbsDir, 0755); err != nil {
		t.Fatal(err)
	}
	existing := "app:\n  staging:\n    token: secret\n    db_host: localhost\n"
	if err := os.WriteFile(wardFile, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	// cfgPath = <root>/.ward/config.yaml so that projectRoot = root
	cfgPath := filepath.Join(root, ".ward", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Vaults: []config.Source{{Path: vaultAbsDir, Name: "app"}}}
	eng := ward.NewEngine(cfg, sops.MockDecryptor{})
	files, err := eng.LoadFiles()
	if err != nil {
		t.Fatal(err)
	}

	ed := &secretEditor{eng: eng, cfg: cfg, cfgPath: cfgPath, files: files}

	// targets is empty because app.staging.api_key does not exist yet → created=true
	targets := secrets.FilesMatching(files, "app.staging.api_key", secrets.IsLeaf)
	// pass vaultRelDir so resolveNewPath builds root+vault+staging.ward correctly
	targetPath, created := resolveSetTarget(targets, "app.staging.api_key", vaultRelDir, cfgPath)
	if !created {
		t.Fatal("expected created=true for a new key")
	}
	if targetPath != wardFile {
		t.Fatalf("expected targetPath=%s, got %s", wardFile, targetPath)
	}

	// act: run the fixed set.go flow — loads file when it exists on disk,
	// regardless of whether the key was newly derived (created=true)
	_ = created
	tree := secrets.NewTree(nil)
	if _, statErr := os.Stat(targetPath); statErr == nil {
		tree = ed.load(targetPath)
	}
	tree.Set("app.staging.api_key", "my-api-key")
	ed.save(targetPath, tree)

	// assert: reload and verify the original keys survived
	reloaded := ed.load(targetPath)
	staging := reloaded.Root()["app"].(map[string]interface{})["staging"].(map[string]interface{})
	if staging["token"] != "secret" {
		t.Fatalf("expected token=secret to be preserved, got %v", staging["token"])
	}
	if staging["db_host"] != "localhost" {
		t.Fatalf("expected db_host=localhost to be preserved, got %v", staging["db_host"])
	}
	if staging["api_key"] != "my-api-key" {
		t.Fatalf("expected api_key=my-api-key, got %v", staging["api_key"])
	}
}

func TestSetLeaf_preserves_sibling_keys_at_same_level(t *testing.T) {
	// arrange: file already has two siblings at the same level
	data := map[string]interface{}{
		"app": map[string]interface{}{
			"staging": map[string]interface{}{
				"existing_key": "existing_value",
				"other_key":    "other_value",
			},
		},
	}

	// act: set a new key at the same level
	setLeaf(data, "app.staging.new_key", "new_value")

	// assert: siblings must survive
	staging := data["app"].(map[string]interface{})["staging"].(map[string]interface{})
	if staging["existing_key"] != "existing_value" {
		t.Fatalf("expected existing_key to be preserved, got %v", staging["existing_key"])
	}
	if staging["other_key"] != "other_value" {
		t.Fatalf("expected other_key to be preserved, got %v", staging["other_key"])
	}
	if staging["new_key"] != "new_value" {
		t.Fatalf("expected new_key=new_value, got %v", staging["new_key"])
	}
}

func TestSetLeaf_setting_new_key_does_not_drop_file_content(t *testing.T) {
	// arrange: simulate a file that already has several keys — adding a new one must not drop the others
	data := map[string]interface{}{
		"app": map[string]interface{}{
			"staging": map[string]interface{}{
				"token":   "secret-token",
				"db_host": "localhost",
				"db_port": "5432",
			},
		},
	}

	// act: add a brand-new key (the scenario that triggers the bug in ward set)
	setLeaf(data, "app.staging.api_key", "my-api-key")

	// assert: the three pre-existing keys must all survive
	staging := data["app"].(map[string]interface{})["staging"].(map[string]interface{})
	for _, key := range []string{"token", "db_host", "db_port"} {
		if _, ok := staging[key]; !ok {
			t.Fatalf("expected key %q to be preserved after set, but it was dropped", key)
		}
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

