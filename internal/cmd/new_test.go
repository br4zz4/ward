package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oporpino/ward/internal/config"
	"gopkg.in/yaml.v3"
)

func writeWardYAML(t *testing.T, dir, vaults string) string {
	t.Helper()
	content := "encryption:\n  key_file: .ward.key\nvaults:\n" + vaults
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

func TestMaybeAddSource_missing_config_is_noop(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".ward", "config.yaml") // does not exist

	newFile := filepath.Join(dir, "infra", "prod.ward")
	if err := maybeAddSource(cfgPath, newFile); err != nil {
		t.Errorf("expected silent no-op, got error: %v", err)
	}
}

func TestResolveNewPath_bare_name_goes_to_default_dir(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".ward", "config.yaml")
	cfg := &config.Config{}

	got := resolveNewPath("staging", cfgPath, cfg)
	want := filepath.Join(dir, ".ward", "vault", "staging.ward")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveNewPath_with_extension_bare_goes_to_default_dir(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".ward", "config.yaml")
	cfg := &config.Config{}

	got := resolveNewPath("staging.ward", cfgPath, cfg)
	want := filepath.Join(dir, ".ward", "vault", "staging.ward")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveNewPath_slash_path_stays_relative(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".ward", "config.yaml")
	cfg := &config.Config{}

	got := resolveNewPath("infra/prod.ward", cfgPath, cfg)
	if got != "infra/prod.ward" {
		t.Errorf("got %q, want %q", got, "infra/prod.ward")
	}
}

func TestResolveNewPath_custom_default_dir(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".ward", "config.yaml")
	cfg := &config.Config{DefaultDir: "secrets"}

	got := resolveNewPath("prod", cfgPath, cfg)
	want := filepath.Join(dir, "secrets", "prod.ward")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
