package cmd

import (
	"fmt"
	"os"
)

// ApplyDirFlag changes the working directory to dir when non-empty.
// This makes all relative paths in the config resolve against the target project root.
func ApplyDirFlag(dir string) error {
	return applyDirFlag(dir)
}

func applyDirFlag(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("--dir %s: %w", dir, err)
	}
	return nil
}
