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
  - path: ../vault1/secrets
  - path: /org/vault2/creds
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Vaults) != 2 {
		t.Fatalf("expected 2 vaults, got %d", len(cfg.Vaults))
	}
	if cfg.Vaults[0].Path != "../vault1/secrets" {
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
  - path: ../vault1/secrets
  - path: /org/vault2/creds
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Vaults) != 2 {
		t.Fatalf("expected 2 vaults (migrated from sources), got %d", len(cfg.Vaults))
	}
	if cfg.Vaults[0].Path != "../vault1/secrets" {
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

func TestLoad_per_vault_key_auto_detected(t *testing.T) {
	dir := t.TempDir()
	wardDir := filepath.Join(dir, ".ward")
	if err := os.MkdirAll(wardDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wardDir, "vault2.key"), []byte("AGE-SECRET-KEY-1FAKE"), 0600); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	cfgPath := writeConfig(t, dir, `
vaults:
  - name: myapp
    path: .ward/vaults/myapp
  - name: vault2
    path: .ward/vaults/vault2
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Vaults[0].Encryption.KeyFile != "" {
		t.Errorf("myapp: expected no key_file (no .ward/myapp.key), got %q", cfg.Vaults[0].Encryption.KeyFile)
	}
	if cfg.Vaults[1].Encryption.KeyFile != ".ward/vault2.key" {
		t.Errorf("vault2: expected .ward/vault2.key, got %q", cfg.Vaults[1].Encryption.KeyFile)
	}
}

func TestLoad_per_vault_key_not_set_when_explicit_key_file(t *testing.T) {
	dir := t.TempDir()
	wardDir := filepath.Join(dir, ".ward")
	if err := os.MkdirAll(wardDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wardDir, "myapp.key"), []byte("AGE-SECRET-KEY-1FAKE"), 0600); err != nil {
		t.Fatal(err)
	}
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	cfgPath := writeConfig(t, dir, `
vaults:
  - name: myapp
    path: .ward/vaults/myapp
    encryption:
      key_file: .ward/custom.key
`)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Vaults[0].Encryption.KeyFile != ".ward/custom.key" {
		t.Errorf("expected explicit key_file to be preserved, got %q", cfg.Vaults[0].Encryption.KeyFile)
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

func TestExpandPath_no_dollar_passthrough(t *testing.T) {
	got, err := expandPath("/some/static/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/some/static/path" {
		t.Errorf("expected passthrough, got %q", got)
	}
}

func TestExpandPath_dollar_var(t *testing.T) {
	t.Setenv("WARD_TEST_DIR", "/tmp/testdir")
	got, err := expandPath("$WARD_TEST_DIR/vault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/testdir/vault" {
		t.Errorf("expected /tmp/testdir/vault, got %q", got)
	}
}

func TestExpandPath_braced_var(t *testing.T) {
	t.Setenv("WARD_TEST_DIR", "/tmp/testdir")
	got, err := expandPath("${WARD_TEST_DIR}/vault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/testdir/vault" {
		t.Errorf("expected /tmp/testdir/vault, got %q", got)
	}
}

func TestExpandPath_command_substitution(t *testing.T) {
	got, err := expandPath("$(echo /tmp)/vault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/vault" {
		t.Errorf("expected /tmp/vault, got %q", got)
	}
}

func TestExpandPath_undefined_var_empty(t *testing.T) {
	os.Unsetenv("WARD_UNDEFINED_XYZ")
	got, err := expandPath("$WARD_UNDEFINED_XYZ/vault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/vault" {
		t.Errorf("expected /vault (empty var), got %q", got)
	}
}

func TestExpandPath_bad_command_returns_error(t *testing.T) {
	_, err := expandPath("$(ward_nonexistent_cmd_xyz)/vault")
	if err == nil {
		t.Fatal("expected error from unknown command in substitution")
	}
}

func TestLoad_vault_path_expands_env_var(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "myvault")
	if err := os.MkdirAll(vaultDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WARD_TEST_VAULT", vaultDir)

	path := writeTemp(t, `vaults:
  - name: test
    path: $WARD_TEST_VAULT
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Vaults[0].Path != vaultDir {
		t.Errorf("expected %q, got %q", vaultDir, cfg.Vaults[0].Path)
	}
}

func TestLoad_vault_path_expand_error_propagates(t *testing.T) {
	path := writeTemp(t, `vaults:
  - name: test
    path: $(ward_nonexistent_cmd_xyz)/vault
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error from failing command substitution in vault path")
	}
}

func chdirTemp(t *testing.T, dir string) {
	t.Helper()
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
}

func writeKeyFile(t *testing.T, dir, name, content string) {
	t.Helper()
	wardDir := filepath.Join(dir, ".ward")
	if err := os.MkdirAll(wardDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wardDir, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_rejects_invalid_vault_name(t *testing.T) {
	for _, name := range []string{"a::b::c", "::b", "a::", "a:b"} {
		// arrange
		dir := t.TempDir()
		cfgPath := writeConfig(t, dir, "vaults:\n  - name: \""+name+"\"\n    path: .ward/vaults/x\n")
		// act
		_, err := Load(cfgPath)
		// assert
		if err == nil {
			t.Errorf("%q: expected error", name)
			continue
		}
		if !strings.Contains(err.Error(), "invalid vault name") {
			t.Errorf("%q: unexpected error: %v", name, err)
		}
	}
}

func TestLoad_rejects_default_declared_twice(t *testing.T) {
	// arrange
	dir := t.TempDir()
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: myfish
    path: .ward/vaults/myfish
  - name: myfish::default
    path: .ward/vaults/myfish::default
`)
	// act
	_, err := Load(cfgPath)
	// assert
	if err == nil {
		t.Fatal("expected error for duplicate canonical name")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_rejects_colliding_derived_envs(t *testing.T) {
	// arrange
	dir := t.TempDir()
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: myfish-restricted
    path: .ward/vaults/a
  - name: myfish::restricted
    path: .ward/vaults/b
`)
	// act
	_, err := Load(cfgPath)
	// assert
	if err == nil {
		t.Fatal("expected error for colliding envs")
	}
	for _, want := range []string{"WARD_KEY_MYFISH_RESTRICTED", "myfish-restricted", "myfish::restricted"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestLoad_subvault_key_file_auto_detected(t *testing.T) {
	// arrange
	dir := t.TempDir()
	writeKeyFile(t, dir, "myfish::restricted.key", "AGE-SECRET-KEY-1FAKE")
	chdirTemp(t, dir)
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: myfish::restricted
    path: .ward/vaults/myfish::restricted
`)
	// act
	cfg, err := Load(cfgPath)
	// assert
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Vaults[0].Encryption.KeyFile != ".ward/myfish::restricted.key" {
		t.Errorf("got %q", cfg.Vaults[0].Encryption.KeyFile)
	}
	if cfg.Vaults[0].Name != "myfish::restricted" {
		t.Errorf("name changed: %q", cfg.Vaults[0].Name)
	}
}

func TestLoad_default_vault_detects_legacy_key_file(t *testing.T) {
	// arrange
	dir := t.TempDir()
	writeKeyFile(t, dir, "myfish.key", "AGE-SECRET-KEY-1FAKE")
	chdirTemp(t, dir)
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: myfish::default
    path: .ward/vaults/myfish::default
`)
	// act
	cfg, err := Load(cfgPath)
	// assert
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Vaults[0].Encryption.KeyFile != ".ward/myfish.key" {
		t.Errorf("got %q", cfg.Vaults[0].Encryption.KeyFile)
	}
	if cfg.Vaults[0].Name != "myfish::default" {
		t.Errorf("name changed: %q", cfg.Vaults[0].Name)
	}
}

func TestLoad_default_vault_divergent_key_files_error(t *testing.T) {
	// arrange
	dir := t.TempDir()
	writeKeyFile(t, dir, "myfish.key", "AGE-SECRET-KEY-1AAA")
	writeKeyFile(t, dir, "myfish::default.key", "AGE-SECRET-KEY-1BBB")
	chdirTemp(t, dir)
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: myfish
    path: .ward/vaults/myfish
`)
	// act
	_, err := Load(cfgPath)
	// assert
	if err == nil {
		t.Fatal("expected error for divergent key files")
	}
	for _, want := range []string{".ward/myfish.key", ".ward/myfish::default.key"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestLoad_default_vault_equal_key_files_ok(t *testing.T) {
	// arrange
	dir := t.TempDir()
	writeKeyFile(t, dir, "myfish.key", "AGE-SECRET-KEY-1AAA")
	writeKeyFile(t, dir, "myfish::default.key", "AGE-SECRET-KEY-1AAA")
	chdirTemp(t, dir)
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: myfish
    path: .ward/vaults/myfish
`)
	// act
	cfg, err := Load(cfgPath)
	// assert
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Vaults[0].Encryption.KeyFile != ".ward/myfish.key" {
		t.Errorf("got %q", cfg.Vaults[0].Encryption.KeyFile)
	}
}

func TestLoad_legacy_app_and_app_default_coexist(t *testing.T) {
	// arrange
	dir := t.TempDir()
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: app
    path: .ward/vaults/app
  - name: app-default
    path: .ward/vaults/app-default
`)
	// act
	cfg, err := Load(cfgPath)
	// assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Vaults) != 2 {
		t.Errorf("expected 2 vaults, got %d", len(cfg.Vaults))
	}
}

func TestLoad_restricted_env_collision_still_rejected(t *testing.T) {
	// arrange
	dir := t.TempDir()
	cfgPath := writeConfig(t, dir, `
vaults:
  - name: myfish-restricted
    path: .ward/vaults/a
  - name: myfish::restricted
    path: .ward/vaults/b
`)
	// act
	_, err := Load(cfgPath)
	// assert
	if err == nil || !strings.Contains(err.Error(), "WARD_KEY_MYFISH_RESTRICTED") {
		t.Errorf("expected env collision error, got %v", err)
	}
}

func TestLoad_ai_mode_values(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"masked", "ai_mode: masked\nvaults: []\n", "masked"},
		{"strict", "ai_mode: strict\nvaults: []\n", "strict"},
		{"absent", "vaults: []\n", ""},
		{"invalid is preserved raw", "ai_mode: loose\nvaults: []\n", "loose"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			path := writeTemp(t, tc.yaml)

			// act
			cfg, err := Load(path)

			// assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.AIMode != tc.want {
				t.Errorf("AIMode = %q, want %q", cfg.AIMode, tc.want)
			}
		})
	}
}
