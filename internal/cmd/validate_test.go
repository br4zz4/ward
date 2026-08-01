package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/br4zz4/ward/internal/config"
)

func TestValidateVaultStructure_clean(t *testing.T) {
	// arrange: vault "myapp", file "secrets.ward" → expected path "myapp.secrets"
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, ".ward", "vaults", "myapp")
	_ = os.MkdirAll(vaultDir, 0755)
	_ = os.WriteFile(filepath.Join(vaultDir, "secrets.ward"), []byte("myapp:\n  secrets:\n    key: val\n"), 0644)
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

func TestValidateVaultStructure_clean_subdir(t *testing.T) {
	// arrange: vault "myapp", file "sub/test.ward" → expected path "myapp.sub.test"
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, ".ward", "vaults", "myapp", "sub")
	_ = os.MkdirAll(vaultDir, 0755)
	_ = os.WriteFile(filepath.Join(vaultDir, "test.ward"), []byte("myapp:\n  sub:\n    test:\n      key: val\n"), 0644)
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
	// arrange: vault "myapp", file "secrets.ward" → expected "myapp.secrets", got "wrongkey.secrets"
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, ".ward", "vaults", "myapp")
	_ = os.MkdirAll(vaultDir, 0755)
	_ = os.WriteFile(filepath.Join(vaultDir, "secrets.ward"), []byte("wrongkey:\n  secrets:\n    key: val\n"), 0644)
	cfg := &config.Config{Vaults: []config.Source{{Name: "myapp", Path: ".ward/vaults/myapp"}}}

	// act
	_ = os.Chdir(dir)
	violations := validateVaultStructure(cfg, ".ward/config.yaml")

	// assert
	if len(violations) == 0 {
		t.Error("expected violation for wrong root key")
	}
}

func TestValidateVaultStructure_wrong_subdir_key(t *testing.T) {
	// arrange: vault "myapp", file "secrets/test.ward" → expected "myapp.secrets.test", got "myapp.secretis.test"
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, ".ward", "vaults", "myapp", "secrets")
	_ = os.MkdirAll(vaultDir, 0755)
	_ = os.WriteFile(filepath.Join(vaultDir, "test.ward"), []byte("myapp:\n  secretis:\n    test:\n      key: val\n"), 0644)
	cfg := &config.Config{Vaults: []config.Source{{Name: "myapp", Path: ".ward/vaults/myapp"}}}

	// act
	_ = os.Chdir(dir)
	violations := validateVaultStructure(cfg, ".ward/config.yaml")

	// assert
	if len(violations) == 0 {
		t.Error("expected violation for wrong subdir key")
	}
}

func TestValidateVaultStructure_file_secret_exempt(t *testing.T) {
	// arrange: file-secret "service-account.json.ward" must not trigger a violation
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, ".ward", "vaults", "myapp")
	_ = os.MkdirAll(vaultDir, 0755)
	_ = os.WriteFile(filepath.Join(vaultDir, "service-account.json.ward"), []byte("service_account_json: value\n"), 0644)
	cfg := &config.Config{Vaults: []config.Source{{Name: "myapp", Path: ".ward/vaults/myapp"}}}

	// act
	_ = os.Chdir(dir)
	violations := validateVaultStructure(cfg, ".ward/config.yaml")

	// assert
	if len(violations) != 0 {
		t.Errorf("expected no violations for file-secret, got: %v", violations)
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

func TestExpectedFileDotPath_plain_strips_suffix(t *testing.T) {
	vaultAbs := "/proj/.ward/vaults/app"
	file := filepath.Join(vaultAbs, "config.plain.ward")
	got := expectedFileDotPath("app", vaultAbs, file)
	if got != "app.config" {
		t.Errorf("expected app.config, got %q", got)
	}
}

func TestExpectedFileDotPath_plain_with_subdir(t *testing.T) {
	vaultAbs := "/proj/.ward/vaults/app"
	file := filepath.Join(vaultAbs, "svc", "db.plain.ward")
	got := expectedFileDotPath("app", vaultAbs, file)
	if got != "app.svc.db" {
		t.Errorf("expected app.svc.db, got %q", got)
	}
}

func TestLeadingKeyPath_reads_plain_ward(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	os.WriteFile(p, []byte("app:\n  config:\n    x: \"1\"\n"), 0644)
	got, err := leadingKeyPath(p, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "app.config" {
		t.Errorf("expected app.config, got %q", got)
	}
}
