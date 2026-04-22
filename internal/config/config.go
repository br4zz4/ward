package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MergeMode is used internally by the merge engine.
// MergeModeOverride is only used by MergeForView to produce a complete tree for display.
type MergeMode string

const (
	MergeModeOverride MergeMode = "override" // internal: used by view only
	MergeModeError    MergeMode = "error"    // always the user-facing mode

	// DefaultConfigFile is the canonical config path for new projects.
	DefaultConfigFile = ".ward/config.yaml"
)

type Encryption struct {
	Engine  string `yaml:"engine,omitempty"`
	KeyEnv  string `yaml:"key_env,omitempty"`
	KeyFile string `yaml:"key_file,omitempty"`
}

type Source struct {
	Path string `yaml:"path"`
}

type Config struct {
	Encryption Encryption `yaml:"encryption,omitempty"`
	DefaultDir string     `yaml:"default_dir,omitempty"`
	Vaults     []Source   `yaml:"vaults"`
	Sources    []Source   `yaml:"sources,omitempty"` // legacy: migrated to Vaults on load
}

// Save writes cfg back to path in YAML.
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// FindConfigFile returns the path to the ward config file by trying candidates
// in order. Returns an error wrapping os.ErrNotExist if none are found.
func FindConfigFile() (string, error) {
	candidates := []string{
		".ward/config.yaml",
		".ward/config.yml",
		"ward.yaml",
		"ward.yml",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("reading %s: %w", DefaultConfigFile, os.ErrNotExist)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	cfg := &Config{
		Encryption: Encryption{Engine: "age+armor"},
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if cfg.Encryption.Engine == "" {
		cfg.Encryption.Engine = "age+armor"
	}
	if len(cfg.Vaults) == 0 && len(cfg.Sources) > 0 {
		cfg.Vaults = cfg.Sources
	}
	cfg.Sources = nil // avoid double-serialization

	return cfg, nil
}
