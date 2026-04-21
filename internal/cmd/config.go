package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Open the nearest ward.yaml in $EDITOR",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			path, err := findWardYAML()
			if err != nil {
				fatal(err)
			}
			if err := openEditor(path); err != nil {
				fatal(err)
			}
		},
	}
}

// findWardYAML walks up the directory tree looking for ward.yaml.
func findWardYAML() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "ward.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("ward.yaml not found in current directory or any parent")
}
