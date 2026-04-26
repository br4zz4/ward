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

func TestMaybeAddSource_outside_adds_source(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeWardYAML(t, dir, "  - path: ./.ward/vault\n")

	newFile := filepath.Join(dir, "infra", "prod.ward")
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sources := readSources(t, cfgPath)
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d: %v", len(sources), sources)
	}
	if !strings.Contains(sources[1], "infra") {
		t.Errorf("expected new source to contain 'infra', got %q", sources[1])
	}
}

func TestMaybeAddSource_inside_source_no_change(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeWardYAML(t, dir, "  - path: ./.ward/vault\n")

	newFile := filepath.Join(dir, ".ward", "vault", "staging.ward")
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sources := readSources(t, cfgPath)
	if len(sources) != 1 {
		t.Errorf("expected 1 source (no change), got %d: %v", len(sources), sources)
	}
}

func TestMaybeAddSource_exact_source_dir_no_change(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeWardYAML(t, dir, "  - path: ./.ward/vault\n")

	newFile := filepath.Join(dir, ".ward", "vault", "prod.ward")
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sources := readSources(t, cfgPath)
	if len(sources) != 1 {
		t.Errorf("expected no new source, got %d: %v", len(sources), sources)
	}
}

func TestMaybeAddSource_idempotent(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeWardYAML(t, dir, "  - path: ./.ward/vault\n")

	newFile := filepath.Join(dir, "infra", "prod.ward")
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Fatal(err)
	}
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Fatal(err)
	}

	sources := readSources(t, cfgPath)
	if len(sources) != 2 {
		t.Errorf("expected idempotent: still 2 sources, got %d: %v", len(sources), sources)
	}
}

func TestMaybeAddSource_no_double_slash(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeWardYAML(t, dir, "  - path: ./.ward/vault\n")

	newFile := filepath.Join(dir, ".commons", "ward", "vault", "shared.ward")
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Fatal(err)
	}

	sources := readSources(t, cfgPath)
	for _, s := range sources {
		if strings.Contains(s, "//") {
			t.Errorf("source path contains double slash: %q", s)
		}
	}
}

func TestMaybeAddSource_missing_config_is_noop(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".ward", "config.yaml") // does not exist

	newFile := filepath.Join(dir, "infra", "prod.ward")
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Errorf("expected silent no-op, got error: %v", err)
	}
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

func TestMaybeAddSource_subfolder_of_project(t *testing.T) {
	// CWD is project/services/api, projectRoot is project/
	// ward new ./.commons/vault/staging → file at project/services/api/.commons/vault/staging.ward
	// config entry must be: services/api/.commons/vault (relative to projectRoot)
	projectDir := t.TempDir()
	cfgPath := writeWardYAML(t, projectDir, "  - path: ./.ward/vault\n")

	// newFile is inside a subfolder of projectRoot
	newFile := filepath.Join(projectDir, "services", "api", ".commons", "vault", "staging.ward")
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sources := readSources(t, cfgPath)
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d: %v", len(sources), sources)
	}
	want := "services/api/.commons/vault"
	if sources[1] != want {
		t.Errorf("got %q, want %q", sources[1], want)
	}
}

func TestMaybeAddSource_outside_project_root_uses_dotdot(t *testing.T) {
	// Simulates: projectRoot = dir, newFile is in dir/../sibling/vault/
	// The config is at dir/.ward/config.yaml
	// newFile dir is outside projectRoot → path in config must start with ../
	parent := t.TempDir()
	projectDir := filepath.Join(parent, "myapp")
	sibling := filepath.Join(parent, ".commons", "ward", "vaults", "ruby")

	if err := os.MkdirAll(filepath.Join(projectDir, ".ward"), 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := writeWardYAML(t, projectDir, "  - path: ./.ward/vault\n")

	newFile := filepath.Join(sibling, "staging.ward")
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sources := readSources(t, cfgPath)
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d: %v", len(sources), sources)
	}
	if !strings.HasPrefix(sources[1], "..") {
		t.Errorf("expected path outside project root to start with '..', got %q", sources[1])
	}
	if strings.Contains(sources[1], "//") {
		t.Errorf("path contains double slash: %q", sources[1])
	}
}
