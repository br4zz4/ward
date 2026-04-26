package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/br4zz4/ward/internal/config"
)

func TestValidateVaultStructure_clean(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, ".ward", "vaults", "myapp")
	_ = os.MkdirAll(vaultDir, 0755)
	_ = os.WriteFile(filepath.Join(vaultDir, "secrets.ward"), []byte("myapp:\n  key: val\n"), 0644)
	cfgPath := filepath.Join(dir, ".ward", "config.yaml")
	_ = os.WriteFile(cfgPath, []byte("vaults:\n  - name: myapp\n    path: .ward/vaults/myapp\n"), 0644)
	cfg := &config.Config{Vaults: []config.Source{{Name: "myapp", Path: ".ward/vaults/myapp"}}}

	// act
	_ = os.Chdir(dir)
	violations := validateVaultStructure(cfg, ".ward/config.yaml")

	// assert
	if len(violations) != 0 {
		t.Errorf("expected no violations, got: %v", violations)
	}
}

func TestValidateVaultStructure_wrong_root_key(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, ".ward", "vaults", "myapp")
	_ = os.MkdirAll(vaultDir, 0755)
	_ = os.WriteFile(filepath.Join(vaultDir, "secrets.ward"), []byte("wrongkey:\n  key: val\n"), 0644)
	cfg := &config.Config{Vaults: []config.Source{{Name: "myapp", Path: ".ward/vaults/myapp"}}}

	// act
	_ = os.Chdir(dir)
	violations := validateVaultStructure(cfg, ".ward/config.yaml")

	// assert
	if len(violations) == 0 {
		t.Error("expected violation for wrong root key")
	}
}

func TestValidateVaultStructure_encrypted_file_skipped(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, ".ward", "vaults", "myapp")
	_ = os.MkdirAll(vaultDir, 0755)
	// Write a fake encrypted file (starts with age header)
	_ = os.WriteFile(filepath.Join(vaultDir, "secrets.ward"), []byte("-----BEGIN AGE ENCRYPTED FILE-----\nfakedata\n-----END AGE ENCRYPTED FILE-----\n"), 0644)
	cfg := &config.Config{Vaults: []config.Source{{Name: "myapp", Path: ".ward/vaults/myapp"}}}

	// act
	_ = os.Chdir(dir)
	violations := validateVaultStructure(cfg, ".ward/config.yaml")

	// assert
	if len(violations) != 0 {
		t.Errorf("expected no violations for encrypted file, got: %v", violations)
	}
}
