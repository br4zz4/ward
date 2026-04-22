//go:build integration

// Package integration tests internal components together with real files on disk.
package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/sops"
	"github.com/br4zz4/ward/internal/ward"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func newEngine(t *testing.T, vaultPaths ...string) *ward.Engine {
	t.Helper()
	sources := make([]config.Source, len(vaultPaths))
	for i, p := range vaultPaths {
		sources[i] = config.Source{Path: p}
	}
	cfg := &config.Config{Vaults: sources}
	return ward.NewEngine(cfg, sops.MockDecryptor{})
}

// ── Merge: coexistence ────────────────────────────────────────────────────────

func TestMerge_different_dotpaths_coexist(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.ward", "app:\n  name: myapp\n  region: us-east-1\n")
	writeFile(t, dir, "b.ward", "app:\n  db:\n    host: localhost\n    port: \"5432\"\n")

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, sops.MockDecryptor{})
	tree, err := secrets.Merge(files, config.MergeModeError, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	app := tree["app"].Children
	if app["name"] == nil {
		t.Error("expected app.name")
	}
	if app["db"] == nil || app["db"].Children["host"] == nil {
		t.Error("expected app.db.host")
	}
}

// ── Merge: conflict detection ─────────────────────────────────────────────────

func TestMerge_same_dotpath_conflicts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.ward", "app:\n  secret_key: key-a\n")
	writeFile(t, dir, "b.ward", "app:\n  secret_key: key-b\n")

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, sops.MockDecryptor{})
	_, err := secrets.Merge(files, config.MergeModeError, "")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	ce, ok := err.(*secrets.ConflictError)
	if !ok {
		t.Fatalf("expected *ConflictError, got %T", err)
	}
	if ce.Conflicts[0].Key != "app.secret_key" {
		t.Errorf("expected conflict on app.secret_key, got %q", ce.Conflicts[0].Key)
	}
	if len(ce.Conflicts[0].Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(ce.Conflicts[0].Sources))
	}
}

func TestMerge_three_file_conflict_accumulates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.ward", "k: v1\n")
	writeFile(t, dir, "b.ward", "k: v2\n")
	writeFile(t, dir, "c.ward", "k: v3\n")

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, sops.MockDecryptor{})
	_, err := secrets.Merge(files, config.MergeModeError, "")
	ce, ok := err.(*secrets.ConflictError)
	if !ok {
		t.Fatalf("expected ConflictError, got %T", err)
	}
	if len(ce.Conflicts[0].Sources) < 2 {
		t.Errorf("expected all sources in conflict, got %d", len(ce.Conflicts[0].Sources))
	}
}

// ── Merge: scope prefix ───────────────────────────────────────────────────────

func TestMerge_scope_prefix_ignores_outside_conflicts(t *testing.T) {
	dir := t.TempDir()
	// conflict on app.api_key — outside scope
	writeFile(t, dir, "a.ward", "app:\n  api_key: key-a\n  db:\n    host: localhost\n")
	writeFile(t, dir, "b.ward", "app:\n  api_key: key-b\n")

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, sops.MockDecryptor{})

	// scoped to app.db — conflict on app.api_key should not block
	tree, err := secrets.Merge(files, config.MergeModeError, "app.db")
	if err != nil {
		t.Fatalf("scoped merge should not block on outside conflict: %v", err)
	}
	if tree["app"].Children["db"] == nil {
		t.Error("expected app.db in result")
	}
}

func TestMerge_scope_prefix_blocks_inside_conflicts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.ward", "app:\n  staging:\n    token: token-a\n")
	writeFile(t, dir, "b.ward", "app:\n  staging:\n    token: token-b\n")

	paths, _ := secrets.Discover([]string{dir})
	files, _ := secrets.LoadAll(paths, sops.MockDecryptor{})

	_, err := secrets.Merge(files, config.MergeModeError, "app.staging")
	if err == nil {
		t.Fatal("expected conflict inside scope to block")
	}
}

// ── Engine.MergeScoped ────────────────────────────────────────────────────────

func TestEngine_MergeScoped_clean(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	writeFile(t, vault, "app.ward", "app:\n  name: myapp\n  version: \"1.0\"\n")

	eng := newEngine(t, vault)
	result, err := eng.MergeScoped("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tree["app"] == nil {
		t.Error("expected app in tree")
	}
}

func TestEngine_MergeScoped_conflict_blocked(t *testing.T) {
	dir := t.TempDir()
	vaultA := filepath.Join(dir, "vault-a")
	vaultB := filepath.Join(dir, "vault-b")
	writeFile(t, vaultA, "app.ward", "app:\n  secret: from-a\n")
	writeFile(t, vaultB, "app.ward", "app:\n  secret: from-b\n")

	eng := newEngine(t, vaultA, vaultB)
	_, err := eng.MergeScoped("")
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestEngine_MergeScoped_with_prefix_skips_outside(t *testing.T) {
	dir := t.TempDir()
	vaultA := filepath.Join(dir, "vault-a")
	vaultB := filepath.Join(dir, "vault-b")
	writeFile(t, vaultA, "app.ward", "app:\n  secret: from-a\n  db:\n    host: localhost\n")
	writeFile(t, vaultB, "app.ward", "app:\n  secret: from-b\n")

	eng := newEngine(t, vaultA, vaultB)
	result, err := eng.MergeScoped("app.db")
	if err != nil {
		t.Fatalf("scoped merge should succeed: %v", err)
	}
	if result.Tree["app"].Children["db"] == nil {
		t.Error("expected app.db in scoped result")
	}
}

// ── EnvVars: flat and prefixed ────────────────────────────────────────────────

func TestEnvVars_flat_no_collision(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	writeFile(t, vault, "app.ward", "app:\n  name: myapp\n  region: us-east-1\n")

	eng := newEngine(t, vault)
	result, _ := eng.MergeScoped("")
	entries, err := eng.EnvVars(result, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries["NAME"].Value != "myapp" {
		t.Errorf("expected NAME=myapp, got %q", entries["NAME"].Value)
	}
	if entries["REGION"].Value != "us-east-1" {
		t.Errorf("expected REGION=us-east-1, got %q", entries["REGION"].Value)
	}
}

func TestEnvVars_prefixed(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	writeFile(t, vault, "app.ward", "app:\n  name: myapp\n")

	eng := newEngine(t, vault)
	result, _ := eng.MergeScoped("")
	entries, err := eng.EnvVars(result, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries["APP_NAME"].Value != "myapp" {
		t.Errorf("expected APP_NAME=myapp, got %q", entries["APP_NAME"].Value)
	}
}

func TestEnvVars_flat_collision_errors(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	writeFile(t, vault, "staging.ward", "app:\n  staging:\n    token: staging-token\n")
	writeFile(t, vault, "production.ward", "app:\n  production:\n    token: prod-token\n")

	eng := newEngine(t, vault)
	result, _ := eng.MergeScoped("")
	_, err := eng.EnvVars(result, false)
	if err == nil {
		t.Fatal("expected env var collision error")
	}
}

func TestEnvVars_prefer_prefix_resolves_collision(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	writeFile(t, vault, "staging.ward", "app:\n  staging:\n    token: staging-token\n")
	writeFile(t, vault, "production.ward", "app:\n  production:\n    token: prod-token\n")

	eng := newEngine(t, vault)
	result, _ := eng.MergeScoped("")
	entries, err := eng.EnvVarsPrefer(result, false, "app.staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries["TOKEN"].Value != "staging-token" {
		t.Errorf("expected TOKEN=staging-token, got %q", entries["TOKEN"].Value)
	}
}

// ── Shadow rule ───────────────────────────────────────────────────────────────

func TestEnvVars_shadow_deeper_wins(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	writeFile(t, vault, "app.ward", "app:\n  log_level: info\n  config:\n    log_level: debug\n")

	eng := newEngine(t, vault)
	result, _ := eng.MergeForView() // MarkShadowed is called here
	entries, err := eng.EnvVars(result, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries["LOG_LEVEL"].Value != "debug" {
		t.Errorf("expected deeper LOG_LEVEL=debug to win, got %q", entries["LOG_LEVEL"].Value)
	}
}

// ── MergeForView: always produces tree ───────────────────────────────────────

func TestMergeForView_produces_tree_despite_conflicts(t *testing.T) {
	dir := t.TempDir()
	vaultA := filepath.Join(dir, "vault-a")
	vaultB := filepath.Join(dir, "vault-b")
	writeFile(t, vaultA, "app.ward", "app:\n  secret: from-a\n")
	writeFile(t, vaultB, "app.ward", "app:\n  secret: from-b\n")

	eng := newEngine(t, vaultA, vaultB)
	result, err := eng.MergeForView()
	if err != nil {
		t.Fatalf("MergeForView should not error: %v", err)
	}
	if result.ConflictErr == nil {
		t.Error("expected ConflictErr to be set")
	}
	if result.Tree["app"] == nil {
		t.Error("expected tree to contain app despite conflict")
	}
}

// ── Inspect ───────────────────────────────────────────────────────────────────

func TestInspect_clean(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "vault")
	writeFile(t, vault, "app.ward", "app:\n  name: clean\n")

	eng := newEngine(t, vault)
	if err := eng.Inspect(); err != nil {
		t.Fatalf("expected clean inspect, got: %v", err)
	}
}

func TestInspect_conflict(t *testing.T) {
	dir := t.TempDir()
	vaultA := filepath.Join(dir, "vault-a")
	vaultB := filepath.Join(dir, "vault-b")
	writeFile(t, vaultA, "app.ward", "app:\n  secret: from-a\n")
	writeFile(t, vaultB, "app.ward", "app:\n  secret: from-b\n")

	eng := newEngine(t, vaultA, vaultB)
	if err := eng.Inspect(); err == nil {
		t.Fatal("expected conflict error from Inspect")
	}
}
