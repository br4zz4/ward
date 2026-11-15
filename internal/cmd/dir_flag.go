package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

var invocationDir string

// ApplyDirFlag changes the working directory to dir when non-empty.
// This makes all relative paths in the config resolve against the target project root.
func ApplyDirFlag(dir string) error {
	return applyDirFlag(dir)
}

func applyDirFlag(dir string) error {
	if wd, err := os.Getwd(); err == nil {
		invocationDir = wd
	}
	if dir == "" {
		return nil
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("--dir %s: %w", dir, err)
	}
	return nil
}

// resolveInvocationPath resolves a user-supplied path against the directory
// ward was invoked from, ignoring any later --dir change.
func resolveInvocationPath(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	base := invocationDir
	if base == "" {
		if wd, err := os.Getwd(); err == nil {
			base = wd
		}
	}
	return filepath.Join(base, p)
}
