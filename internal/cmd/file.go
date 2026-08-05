package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file",
		Short: "Import and export files as secrets",
	}
	cmd.AddCommand(newFileAddCmd(), newFileExtractCmd())
	return cmd
}

func newFileAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <file> <vault>[:subdir]",
		Short: "Add a file as a single encrypted secret",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			srcPath, vaultArg := args[0], args[1]

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

			vaultBase, subDir := splitVaultArg(vaultArg)
			vaultPath := resolveVaultDir(cfg, vaultBase, cfgPath)
			if vaultPath == "" {
				fatal(fmt.Errorf("vault %q not found — use `ward vault list` to see available vaults", vaultBase))
			}
			targetDir := vaultPath
			if subDir != "" {
				targetDir = filepath.Join(vaultPath, filepath.FromSlash(strings.ReplaceAll(subDir, ".", "/")))
			}

			wardPath := filepath.Join(targetDir, secrets.WardFilename(srcPath))
			if _, err := os.Stat(wardPath); err == nil {
				fatal(fmt.Errorf("%s already exists — remove it first to re-import", wardPath))
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			if err := os.MkdirAll(filepath.Dir(wardPath), 0755); err != nil {
				fatal(fmt.Errorf("creating directory: %w", err))
			}

			if err := eng.Encrypt(wardPath, content); err != nil {
				fatal(fmt.Errorf("encrypting %s: %w", wardPath, err))
			}

			fmt.Fprintf(os.Stderr, "ward: saved %s\n", wardPath)
		},
	}
}

func newFileExtractCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "extract <filename> [dest]",
		Short: "Extract a file secret to disk",
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

			content, err := eng.Decrypt(wardFile)
			if err != nil {
				fatal(fmt.Errorf("decrypting %s: %w", wardFile, err))
			}

			if err := os.MkdirAll(destDir, 0755); err != nil {
				fatal(fmt.Errorf("creating directory: %w", err))
			}

			if err := os.WriteFile(destPath, content, 0600); err != nil {
				fatal(fmt.Errorf("writing %s: %w", destPath, err))
			}

			fmt.Fprintf(os.Stderr, "ward: exported %s → %s\n", wardFile, destPath)
		},
	}
}

// splitVaultArg splits "app:main" into ("app", "main") and "app" into ("app", "").
func splitVaultArg(arg string) (vault, subDir string) {
	parts := strings.SplitN(arg, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return arg, ""
}

func resolveVaultDir(cfg *config.Config, vaultName, cfgPath string) string {
	projectRoot := filepath.Dir(filepath.Dir(cfgPath))
	for _, src := range cfg.Vaults {
		if src.Name == vaultName {
			return filepath.Join(projectRoot, src.Path)
		}
	}
	return ""
}

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

