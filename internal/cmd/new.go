package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oporpino/ward/internal/config"
	"github.com/spf13/cobra"
)

func NewNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <file.ward>",
		Short: "Create a new encrypted .ward file and open it in $EDITOR",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			path := args[0]

			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				fatal(fmt.Errorf("creating directory: %w", err))
			}

			if _, err := os.Stat(path); err == nil {
				fatal(fmt.Errorf("%s already exists — use `ward edit` to modify it", path))
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			stub := []byte("{}\n")
			if err := eng.Encrypt(path, stub); err != nil {
				fatal(fmt.Errorf("creating %s: %w", path, err))
			}

			plain, err := eng.Decrypt(path)
			if err != nil {
				fatal(fmt.Errorf("decrypting %s: %w", path, err))
			}

			tmp, err := writeTempFile(path, plain)
			if err != nil {
				fatal(err)
			}
			defer os.Remove(tmp)

			if err := openEditor(tmp); err != nil {
				fatal(err)
			}

			edited, err := os.ReadFile(tmp)
			if err != nil {
				fatal(fmt.Errorf("reading temp file: %w", err))
			}

			if err := eng.Encrypt(path, edited); err != nil {
				fatal(fmt.Errorf("re-encrypting %s: %w", path, err))
			}

			if err := maybeAddSource(configFile, path); err != nil {
				fatal(err)
			}
		},
	}
}

// maybeAddSource appends the directory of newFile to the sources list in
// wardYAML if it is not already covered by an existing source.
func maybeAddSource(wardYAML, newFile string) error {
	cfg, err := config.Load(wardYAML)
	if err != nil {
		return nil // best-effort: if we can't load, skip silently
	}

	newDir, err := filepath.Abs(filepath.Dir(newFile))
	if err != nil {
		return nil
	}

	wardDir := filepath.Dir(wardYAML)

	for _, src := range cfg.Sources {
		srcAbs, err := filepath.Abs(filepath.Join(wardDir, src.Path))
		if err != nil {
			continue
		}
		// covered if newDir is srcAbs or a subdirectory of srcAbs
		rel, err := filepath.Rel(srcAbs, newDir)
		if err != nil {
			continue
		}
		if rel == "." || !strings.HasPrefix(rel, "..") {
			return nil // already covered
		}
	}

	// Compute a relative path from wardYAML's directory to newDir
	rel, err := filepath.Rel(wardDir, newDir)
	if err != nil {
		rel = newDir
	}
	sourcePath := "./" + rel

	cfg.Sources = append(cfg.Sources, config.Source{Path: sourcePath})
	if err := config.Save(wardYAML, cfg); err != nil {
		return fmt.Errorf("updating %s: %w", wardYAML, err)
	}
	fmt.Printf("ward: added %s%s%s to sources in %s%s%s\n",
		clrCyan, sourcePath, clrReset, clrCyan, wardYAML, clrReset)
	return nil
}
