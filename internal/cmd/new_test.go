package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func writeWardYAML(t *testing.T, dir, vaults string) string {
	t.Helper()
	content := "encryption:\n  key_file: .ward/.key\nvaults:\n" + vaults
	if err := os.MkdirAll(filepath.Join(dir, ".ward"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".ward", "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readSources(t *testing.T, cfgPath string) []string {
	t.Helper()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Vaults []struct {
			Path string `yaml:"path"`
		} `yaml:"vaults"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(cfg.Vaults))
	for i, s := range cfg.Vaults {
		paths[i] = s.Path
	}
	return paths
}

func TestNewFileStub_uses_vault_name_as_root(t *testing.T) {
	// vaultName = "myapp", file inside vault with subdir
	// expected root key = myapp, then subpath + stem
	parent := t.TempDir()
	projectDir := filepath.Join(parent, "qwert")
	cfgPath := writeWardYAML(t, projectDir, "  - name: myapp\n    path: .ward/vaults/myapp\n")
	filePath := filepath.Join(projectDir, ".ward", "vaults", "myapp", "environments", "staging.ward")

	got := newFileStub("myapp", filePath, cfgPath)

	if !strings.HasPrefix(got, "myapp:\n") {
		t.Errorf("expected root key 'myapp', got:\n%s", got)
	}
	if !strings.Contains(got, "environments:") {
		t.Errorf("expected 'environments:' in stub, got:\n%s", got)
	}
	if !strings.Contains(got, "staging:") {
		t.Errorf("expected 'staging:' in stub, got:\n%s", got)
	}
}

func TestNewFileStub_internal_vault_no_subdir(t *testing.T) {
	// file directly in vault root → myapp:\n  staging:\n    secret_1: …
	parent := t.TempDir()
	projectDir := filepath.Join(parent, "qwert")
	cfgPath := writeWardYAML(t, projectDir, "  - name: myapp\n    path: .ward/vaults/myapp\n")
	filePath := filepath.Join(projectDir, ".ward", "vaults", "myapp", "staging.ward")

	got := newFileStub("myapp", filePath, cfgPath)

	if !strings.HasPrefix(got, "myapp:\n") {
		t.Errorf("expected root key 'myapp', got:\n%s", got)
	}
	if !strings.Contains(got, "staging:") {
		t.Errorf("expected 'staging:' in stub, got:\n%s", got)
	}
}

func TestNewFileStub_external_vault_uses_vault_segments(t *testing.T) {
	// vaultName = "commons", file inside external vault
	// expected root key = commons
	parent := t.TempDir()
	projectDir := filepath.Join(parent, "myapp")
	externalVault := filepath.Join(parent, ".commons", "stacks", "ruby")

	if err := os.MkdirAll(filepath.Join(projectDir, ".ward"), 0755); err != nil {
		t.Fatal(err)
	}

	// Write relative vault path (../.commons/stacks/ruby) in config
	vaultRelPath, _ := filepath.Rel(projectDir, externalVault)
	cfgPath := writeWardYAML(t, projectDir, "  - name: commons\n    path: "+vaultRelPath+"\n")

	filePath := filepath.Join(externalVault, "staging.ward")

	got := newFileStub("commons", filePath, cfgPath)

	if !strings.HasPrefix(got, "commons:\n") {
		t.Errorf("expected root key 'commons', got:\n%s", got)
	}
	if !strings.Contains(got, "staging:") {
		t.Errorf("expected 'staging:' in stub, got:\n%s", got)
	}
}

func TestResolveNewPath_bare_name_goes_to_vault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".ward", "config.yaml")

	got := resolveNewPath("staging", ".ward/vaults/myapp", cfgPath)
	want := filepath.Join(dir, ".ward", "vaults", "myapp", "staging.ward")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveNewPath_with_extension_goes_to_vault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".ward", "config.yaml")

	got := resolveNewPath("staging.ward", ".ward/vaults/myapp", cfgPath)
	want := filepath.Join(dir, ".ward", "vaults", "myapp", "staging.ward")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveNewPath_slash_path_goes_to_vault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".ward", "config.yaml")

	got := resolveNewPath("infra/prod.ward", ".ward/vaults/myapp", cfgPath)
	want := filepath.Join(dir, ".ward", "vaults", "myapp", "infra", "prod.ward")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

