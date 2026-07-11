package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
		Use:   "import <file> <vault>[.subdir]",
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

			vaultBase, subDir := splitVaultArg(vaultName)
			vaultPath := resolveVaultDir(cfg, vaultBase, cfgPath)
			if vaultPath == "" {
				fatal(fmt.Errorf("vault %q not found — use `ward vault list` to see available vaults", vaultBase))
			}
			if subDir != "" {
				vaultPath = filepath.Join(vaultPath, subDir)
			}

			wardPath := filepath.Join(vaultPath, secrets.WardFilename(srcPath))
			if _, err := os.Stat(wardPath); err == nil {
				fatal(fmt.Errorf("%s already exists — remove it first to re-import", wardPath))
			}

			key := secrets.FileKey(srcPath)
			segments := append(strings.Split(vaultBase, "."), key)
			yamlContent := nestedYAML(segments, string(content))

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

// nestedYAML builds a nested YAML string from segments, placing value at the leaf.
// nestedYAML(["app", "service_account_json"], "...") →
//
//	app:
//	  service_account_json: |
//	    ...
func nestedYAML(segments []string, value string) []byte {
	var sb strings.Builder
	for i, seg := range segments[:len(segments)-1] {
		sb.WriteString(strings.Repeat("  ", i) + seg + ":\n")
	}
	leaf := segments[len(segments)-1]
	indent := strings.Repeat("  ", len(segments)-1)
	// Use literal block scalar for multiline-safe encoding.
	sb.WriteString(indent + leaf + ": |\n")
	for _, line := range strings.Split(value, "\n") {
		sb.WriteString(indent + "  " + line + "\n")
	}
	return []byte(sb.String())
}

// splitVaultArg splits "app.main" into ("app", "main") and "app" into ("app", "").
func splitVaultArg(arg string) (vault, subDir string) {
	parts := strings.SplitN(arg, ".", 2)
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

func extractFileSecret(plain []byte, originalName string) (string, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal(plain, &data); err != nil {
		return "", fmt.Errorf("parsing secret: %w", err)
	}
	key := secrets.FileKey(originalName)
	value, err := deepFind(data, key)
	if err != nil {
		return "", fmt.Errorf("key %q not found in secret", key)
	}
	return strings.TrimRight(value, "\n"), nil
}

// deepFind recursively searches a nested map for the first leaf matching key.
func deepFind(m map[string]interface{}, key string) (string, error) {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v), nil
	}
	for _, v := range m {
		if nested, ok := v.(map[string]interface{}); ok {
			if found, err := deepFind(nested, key); err == nil {
				return found, nil
			}
		}
	}
	return "", fmt.Errorf("not found")
}
