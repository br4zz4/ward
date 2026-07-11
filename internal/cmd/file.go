package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Import and export files as secrets",
	}
	cmd.AddCommand(newFileImportCmd(), newFileExportCmd())
	return cmd
}

func newFileImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file> <vault>",
		Short: "Store a file as a single encrypted secret",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			srcPath, vaultName := args[0], args[1]

			content, err := os.ReadFile(srcPath)
			if err != nil {
				fatal(fmt.Errorf("reading %s: %w", srcPath, err))
			}

			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(fmt.Errorf("no ward project found — run `ward init` first"))
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}

			vaultPath := vaultDir(cfg, vaultName, cfgPath)
			if vaultPath == "" {
				fatal(fmt.Errorf("vault %q not found — use `ward vault list` to see available vaults", vaultName))
			}

			wardPath := filepath.Join(vaultPath, secrets.WardFilename(srcPath))
			if _, err := os.Stat(wardPath); err == nil {
				fatal(fmt.Errorf("%s already exists — remove it first to re-import", wardPath))
			}

			key := secrets.FileKey(srcPath)
			yaml := fmt.Sprintf("%s: %s\n", key, yamlQuote(string(content)))

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			if err := os.MkdirAll(filepath.Dir(wardPath), 0755); err != nil {
				fatal(fmt.Errorf("creating directory: %w", err))
			}

			if err := eng.Encrypt(wardPath, []byte(yaml)); err != nil {
				fatal(fmt.Errorf("encrypting %s: %w", wardPath, err))
			}

			fmt.Fprintf(os.Stderr, "ward: saved %s\n", wardPath)
		},
	}
}

func newFileExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <filename> [dest]",
		Short: "Restore a file secret to disk",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(_ *cobra.Command, args []string) {
			originalName := filepath.Base(args[0])

			destDir := "."
			if len(args) == 2 {
				destDir = args[1]
			}

			destPath := filepath.Join(destDir, originalName)
			if _, err := os.Stat(destPath); err == nil {
				fatal(fmt.Errorf("%s already exists — remove it first to re-export", destPath))
			}

			wardFile, err := findFileSecret(originalName)
			if err != nil {
				fatal(err)
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			plain, err := eng.Decrypt(wardFile)
			if err != nil {
				fatal(fmt.Errorf("decrypting %s: %w", wardFile, err))
			}

			content, err := extractFileContent(plain, originalName)
			if err != nil {
				fatal(err)
			}

			if err := os.MkdirAll(destDir, 0755); err != nil {
				fatal(fmt.Errorf("creating directory: %w", err))
			}

			if err := os.WriteFile(destPath, []byte(content), 0600); err != nil {
				fatal(fmt.Errorf("writing %s: %w", destPath, err))
			}

			fmt.Fprintf(os.Stderr, "ward: exported %s → %s\n", wardFile, destPath)
		},
	}
}

// vaultDir resolves the absolute path of a vault directory by name.
func vaultDir(cfg *config.Config, vaultName, cfgPath string) string {
	projectRoot := filepath.Dir(filepath.Dir(cfgPath))
	for _, src := range cfg.Vaults {
		if src.Name == vaultName {
			return filepath.Join(projectRoot, src.Path)
		}
	}
	return ""
}

// findFileSecret searches all vaults for a .ward file matching originalName.
func findFileSecret(originalName string) (string, error) {
	cfgPath, err := resolvedConfigFile()
	if err != nil {
		return "", fmt.Errorf("no ward project found")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return "", err
	}

	wardName := secrets.WardFilename(originalName)
	projectRoot := filepath.Dir(filepath.Dir(cfgPath))

	for _, src := range cfg.Vaults {
		candidate := filepath.Join(projectRoot, src.Path, wardName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("%s: file secret not found in any vault", originalName)
}

// extractFileContent parses the single-key YAML produced by file import and
// returns the scalar value for the given originalName.
func extractFileContent(plain []byte, originalName string) (string, error) {
	key := secrets.FileKey(originalName)
	prefix := key + ": "
	lines := splitLines(string(plain))
	for _, line := range lines {
		if len(line) > len(prefix) && line[:len(prefix)] == prefix {
			return yamlUnquote(line[len(prefix):]), nil
		}
	}
	return "", fmt.Errorf("key %q not found in secret", key)
}

// splitLines splits s into lines, trimming trailing newline.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// yamlQuote wraps s in single quotes, escaping inner single quotes.
func yamlQuote(s string) string {
	escaped := ""
	for _, r := range s {
		if r == '\'' {
			escaped += "''"
		} else {
			escaped += string(r)
		}
	}
	return "'" + escaped + "'"
}

// yamlUnquote strips surrounding single or double quotes from a YAML scalar.
func yamlUnquote(s string) string {
	s = trimSpace(s)
	if len(s) >= 2 {
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			inner := s[1 : len(s)-1]
			// unescape '' → '
			result := ""
			i := 0
			for i < len(inner) {
				if i+1 < len(inner) && inner[i] == '\'' && inner[i+1] == '\'' {
					result += "'"
					i += 2
				} else {
					result += string(inner[i])
					i++
				}
			}
			return result
		}
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
