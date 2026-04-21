package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func writeWardYAML(t *testing.T, dir, sources string) string {
	t.Helper()
	content := "encryption:\n  engine: sops+age\n  key_file: .ward.key\nmerge: merge\nsources:\n" + sources
	path := filepath.Join(dir, "ward.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readSources(t *testing.T, wardYAML string) []string {
	t.Helper()
	data, err := os.ReadFile(wardYAML)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Sources []struct {
			Path string `yaml:"path"`
		} `yaml:"sources"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(cfg.Sources))
	for i, s := range cfg.Sources {
		paths[i] = s.Path
	}
	return paths
}

func TestMaybeAddSource_outside_adds_source(t *testing.T) {
	dir := t.TempDir()
	wardYAML := writeWardYAML(t, dir, "  - path: ./.secrets\n")

	newFile := filepath.Join(dir, "infra", "prod.ward")
	if err := maybeAddSource(wardYAML, newFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sources := readSources(t, wardYAML)
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d: %v", len(sources), sources)
	}
	if !strings.Contains(sources[1], "infra") {
		t.Errorf("expected new source to contain 'infra', got %q", sources[1])
	}
}

func TestMaybeAddSource_inside_source_no_change(t *testing.T) {
	dir := t.TempDir()
	wardYAML := writeWardYAML(t, dir, "  - path: ./.secrets\n")

	newFile := filepath.Join(dir, ".secrets", "staging.ward")
	if err := maybeAddSource(wardYAML, newFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sources := readSources(t, wardYAML)
	if len(sources) != 1 {
		t.Errorf("expected 1 source (no change), got %d: %v", len(sources), sources)
	}
}

func TestMaybeAddSource_exact_source_dir_no_change(t *testing.T) {
	dir := t.TempDir()
	wardYAML := writeWardYAML(t, dir, "  - path: ./.secrets\n")

	newFile := filepath.Join(dir, ".secrets", "prod.ward")
	if err := maybeAddSource(wardYAML, newFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sources := readSources(t, wardYAML)
	if len(sources) != 1 {
		t.Errorf("expected no new source, got %d: %v", len(sources), sources)
	}
}

func TestMaybeAddSource_idempotent(t *testing.T) {
	dir := t.TempDir()
	wardYAML := writeWardYAML(t, dir, "  - path: ./.secrets\n")

	newFile := filepath.Join(dir, "infra", "prod.ward")
	if err := maybeAddSource(wardYAML, newFile); err != nil {
		t.Fatal(err)
	}
	if err := maybeAddSource(wardYAML, newFile); err != nil {
		t.Fatal(err)
	}

	sources := readSources(t, wardYAML)
	if len(sources) != 2 {
		t.Errorf("expected idempotent: still 2 sources, got %d: %v", len(sources), sources)
	}
}

func TestMaybeAddSource_missing_wardyaml_is_noop(t *testing.T) {
	dir := t.TempDir()
	wardYAML := filepath.Join(dir, "ward.yaml") // does not exist

	newFile := filepath.Join(dir, "infra", "prod.ward")
	if err := maybeAddSource(wardYAML, newFile); err != nil {
		t.Errorf("expected silent no-op, got error: %v", err)
	}
}
