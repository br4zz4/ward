package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

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

			vaultPath := resolveVaultDir(cfg, vaultName, cfgPath)
			if vaultPath == "" {
				fatal(fmt.Errorf("vault %q not found — use `ward vault list` to see available vaults", vaultName))
			}

			wardPath := filepath.Join(vaultPath, secrets.WardFilename(srcPath))
			if _, err := os.Stat(wardPath); err == nil {
				fatal(fmt.Errorf("%s already exists — remove it first to re-import", wardPath))
			}

			yamlContent, err := yaml.Marshal(map[string]string{secrets.FileKey(srcPath): string(content)})
			if err != nil {
				fatal(fmt.Errorf("encoding secret: %w", err))
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			if err := os.MkdirAll(filepath.Dir(wardPath), 0755); err != nil {
				fatal(fmt.Errorf("creating directory: %w", err))
			}

			if err := eng.Encrypt(wardPath, yamlContent); err != nil {
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

			wardFile, err := locateFileSecret(originalName)
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

			content, err := extractFileSecret(plain, originalName)
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

// resolveVaultDir returns the absolute directory path for the named vault, or "".
func resolveVaultDir(cfg *config.Config, vaultName, cfgPath string) string {
	projectRoot := filepath.Dir(filepath.Dir(cfgPath))
	for _, src := range cfg.Vaults {
		if src.Name == vaultName {
			return filepath.Join(projectRoot, src.Path)
		}
	}
	return ""
}

// locateFileSecret searches all vaults for a .ward file matching originalName.
func locateFileSecret(originalName string) (string, error) {
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

// extractFileSecret parses the YAML produced by file import and returns the
// scalar value for the key derived from originalName.
func extractFileSecret(plain []byte, originalName string) (string, error) {
	var data map[string]string
	if err := yaml.Unmarshal(plain, &data); err != nil {
		return "", fmt.Errorf("parsing secret: %w", err)
	}
	key := secrets.FileKey(originalName)
	value, ok := data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret", key)
	}
	return value, nil
}
