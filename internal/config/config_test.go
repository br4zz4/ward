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
	// Default: no merge field → OnConflictError
	if cfg.OnConflict != OnConflictError {
		t.Errorf("expected default on_conflict=error, got %q", cfg.OnConflict)
	}
	if cfg.Encryption.Engine != "sops+age" {
		t.Errorf("unexpected engine: %q", cfg.Encryption.Engine)
	}
}

func TestLoad_explicit_merge_override(t *testing.T) {
	path := writeTemp(t, `merge: override`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Legacy merge: override → on_conflict: override
	if cfg.OnConflict != OnConflictOverride {
		t.Errorf("expected on_conflict=override, got %q", cfg.OnConflict)
	}
}

func TestLoad_explicit_on_conflict(t *testing.T) {
	path := writeTemp(t, `on_conflict: override`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OnConflict != OnConflictOverride {
		t.Errorf("expected on_conflict=override, got %q", cfg.OnConflict)
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
