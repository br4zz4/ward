package secrets

import (
	"fmt"
	"os"
	"path/filepath"
)

// Discover returns all *.ward files found recursively under each source path.
// Order within each source is deterministic (lexicographic).
func Discover(sources []string) ([]string, error) {
	var files []string
	for _, src := range sources {
		info, err := os.Stat(src)
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", src, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("source %q is not a directory", src)
		}

		matches, err := filepath.Glob(filepath.Join(src, "**", "*.ward"))
		if err != nil {
			return nil, err
		}
		// filepath.Glob does not recurse — walk manually
		err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && filepath.Ext(path) == ".ward" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking source %q: %w", src, err)
		}
		_ = matches // WalkDir supersedes Glob
	}
	return files, nil
}
