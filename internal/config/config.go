package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type MergeMode string

const (
	MergeModeDeep     MergeMode = "merge"
	MergeModeOverride MergeMode = "override"
	MergeModeError    MergeMode = "error"
)

type Encryption struct {
	Engine  string `yaml:"engine"`
	KeyEnv  string `yaml:"key_env"`
	KeyFile string `yaml:"key_file"`
}

type Source struct {
	Path string `yaml:"path"`
}

type Config struct {
	Encryption Encryption `yaml:"encryption"`
	Merge      MergeMode  `yaml:"merge"`
	Sources    []Source   `yaml:"sources"`
}

// Save writes cfg back to path in YAML. It preserves the structure but
// does not round-trip comments or formatting from the original file.
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

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	cfg := &Config{
		Merge: MergeModeDeep,
		Encryption: Encryption{Engine: "age+armor"},
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if cfg.Merge == "" {
		cfg.Merge = MergeModeDeep
	}
	if cfg.Encryption.Engine == "" {
		cfg.Encryption.Engine = "age+armor"
	}

	return cfg, nil
}
