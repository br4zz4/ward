package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestLoad_default_key_file_when_exists(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, ".ward.key")
	if err := os.WriteFile(keyPath, []byte("AGE-SECRET-KEY-1FAKE"), 0600); err != nil {
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
		t.Errorf("expected default key_file .ward.key, got %q", cfg.Encryption.KeyFile)
	}
}

func TestLoad_no_default_key_file_when_missing(t *testing.T) {
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
		t.Errorf("expected empty key_file when .ward.key missing, got %q", cfg.Encryption.KeyFile)
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
  - path: /org/infra/secrets
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

func TestLoad_sources_legacy_compat(t *testing.T) {
	path := writeTemp(t, `
sources:
  - path: ../commons/secrets
  - path: /org/infra/secrets
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
