package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	wardDir := filepath.Join(dir, ".ward")
	if err := os.MkdirAll(wardDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(wardDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "ward*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_default_key_file_ward_dir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ward"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ward", ".key"), []byte("AGE-SECRET-KEY-1FAKE"), 0600); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	path := writeTemp(t, `vaults: []`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Encryption.KeyFile != ".ward/.key" {
		t.Errorf("expected .ward/.key, got %q", cfg.Encryption.KeyFile)
	}
}

func TestLoad_default_key_file_root_fallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ward.key"), []byte("AGE-SECRET-KEY-1FAKE"), 0600); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	path := writeTemp(t, `vaults: []`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Encryption.KeyFile != ".ward.key" {
		t.Errorf("expected .ward.key fallback, got %q", cfg.Encryption.KeyFile)
	}
}

func TestLoad_no_key_file_when_missing(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	path := writeTemp(t, `vaults: []`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Encryption.KeyFile != "" {
		t.Errorf("expected empty key_file when no key file exists, got %q", cfg.Encryption.KeyFile)
	}
}

func TestLoad_no_default_key_file_when_key_env_set(t *testing.T) {
	path := writeTemp(t, `
encryption:
  key_env: MY_KEY
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Encryption.KeyFile != "" {
		t.Errorf("expected empty key_file when key_env set, got %q", cfg.Encryption.KeyFile)
	}
}

func TestLoad_defaults(t *testing.T) {
	path := writeTemp(t, `
encryption:
  engine: sops+age
  key_env: WARD_AGE_KEY
  key_file: .ward.key
sources:
  - path: ./secrets
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Encryption.Engine != "sops+age" {
		t.Errorf("unexpected engine: %q", cfg.Encryption.Engine)
	}
}

func TestLoad_legacy_sources_migrated(t *testing.T) {
	path := writeTemp(t, `sources:
  - path: .ward/vault
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Vaults) != 1 || cfg.Vaults[0].Path != ".ward/vault" {
		t.Errorf("expected sources migrated to vaults, got %v", cfg.Vaults)
	}
}

func TestLoad_invalid_yaml(t *testing.T) {
	path := writeTemp(t, `merge: [unclosed`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestLoad_missing_file(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_vaults(t *testing.T) {
	path := writeTemp(t, `
vaults:
  - path: ../commons/secrets
  - path: /org/infra/creds
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Vaults) != 2 {
		t.Fatalf("expected 2 vaults, got %d", len(cfg.Vaults))
	}
	if cfg.Vaults[0].Path != "../commons/secrets" {
		t.Errorf("unexpected vault path: %q", cfg.Vaults[0].Path)
	}
}

func TestFindConfigFile_currentDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".ward"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ward", "config.yaml"), []byte("vaults: []"), 0644); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(root)

	got, _, err := FindConfigFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ".ward/config.yaml" {
		t.Errorf("got %q, want .ward/config.yaml", got)
	}
}

func TestFindConfigFile_parentDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".ward"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ward", "config.yaml"), []byte("vaults: []"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, ".ward", "vault", "deep")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(sub)

	got, origDir, err := FindConfigFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != ".ward/config.yaml" {
		t.Errorf("got %q, want .ward/config.yaml", got)
	}
	realOrig, _ := filepath.EvalSymlinks(origDir)
	realSub, _ := filepath.EvalSymlinks(sub)
	if realOrig != realSub {
		t.Errorf("originalDir wrong: got %q, want %q", realOrig, realSub)
	}
	wd, _ := os.Getwd()
	realWd, _ := filepath.EvalSymlinks(wd)
	realRoot, _ := filepath.EvalSymlinks(root)
	if realWd != realRoot {
		t.Errorf("cwd should be project root: got %q, want %q", realWd, realRoot)
	}
}

func TestFindConfigFile_notFound(t *testing.T) {
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(t.TempDir())

	_, _, err := FindConfigFile()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoad_sources_legacy_compat(t *testing.T) {
	path := writeTemp(t, `
sources:
  - path: ../commons/secrets
  - path: /org/infra/creds
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Vaults) != 2 {
		t.Fatalf("expected 2 vaults (migrated from sources), got %d", len(cfg.Vaults))
	}
	if cfg.Vaults[0].Path != "../commons/secrets" {
		t.Errorf("unexpected vault path: %q", cfg.Vaults[0].Path)
	}
}

func TestLoad_adds_name_from_path_when_absent(t *testing.T) {
	// arrange
	dir := t.TempDir()
	cfgPath := writeConfig(t, dir, `
vaults:
  - path: .ward/vaults/myapp
`)
	// act
	loaded, err := Load(cfgPath)
	// assert
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Vaults[0].Name != "myapp" {
		t.Errorf("expected name 'myapp', got %q", loaded.Vaults[0].Name)
	}
}

func TestLoad_rejects_duplicate_vault_names(t *testing.T) {
	// arrange
	dir := t.TempDir()
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: shared
    path: .ward/vaults/shared
  - name: shared
    path: .ward/vaults/other
`)
	// act
	_, err := Load(cfgPath)
	// assert
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
	if !strings.Contains(err.Error(), "duplicate vault name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_rejects_duplicate_vault_paths(t *testing.T) {
	// arrange
	dir := t.TempDir()
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: a
    path: .ward/vaults/shared
  - name: b
    path: .ward/vaults/shared
`)
	// act
	_, err := Load(cfgPath)
	// assert
	if err == nil {
		t.Fatal("expected error for duplicate path")
	}
	if !strings.Contains(err.Error(), "duplicate vault path") {
		t.Errorf("unexpected error: %v", err)
	}
}
