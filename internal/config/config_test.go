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
	if cfg.Merge != MergeModeDeep {
		t.Errorf("expected default merge=merge, got %q", cfg.Merge)
	}
	if cfg.Encryption.Engine != "sops+age" {
		t.Errorf("unexpected engine: %q", cfg.Encryption.Engine)
	}
}

func TestLoad_explicit_merge(t *testing.T) {
	path := writeTemp(t, `merge: override`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Merge != MergeModeOverride {
		t.Errorf("expected override, got %q", cfg.Merge)
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

func TestLoad_sources(t *testing.T) {
	path := writeTemp(t, `
sources:
  - path: ../commons/secrets
  - path: /org/infra/secrets
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(cfg.Sources))
	}
	if cfg.Sources[0].Path != "../commons/secrets" {
		t.Errorf("unexpected source path: %q", cfg.Sources[0].Path)
	}
}
