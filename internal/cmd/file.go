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

			segments := fileYAMLSegments(vaultBase, subDir, srcPath)
			yamlContent, err := marshalNested(segments, string(content))
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

// fileYAMLSegments returns the YAML key path for a file-secret:
// vault + subdir parts + leaf key derived from filename.
// e.g. vault="app", subDir="credentials", file="sa.json" → ["app","credentials","service_account_json"]
func fileYAMLSegments(vaultBase, subDir, srcPath string) []string {
	segments := []string{vaultBase}
	if subDir != "" {
		segments = append(segments, strings.Split(subDir, ".")...)
	}
	return append(segments, secrets.FileKey(srcPath))
}

// marshalNested builds nested YAML where segments are map keys and value is the leaf.
// e.g. ["app","creds","key"], "val" → app:\n  creds:\n    key: val\n
func marshalNested(segments []string, value string) ([]byte, error) {
	if len(segments) < 2 {
		return nil, fmt.Errorf("segments must have at least two elements")
	}
	var build func(segs []string) interface{}
	build = func(segs []string) interface{} {
		if len(segs) == 1 {
			return map[string]string{segs[0]: value}
		}
		return map[string]interface{}{segs[0]: build(segs[1:])}
	}
	return yaml.Marshal(build(segments))
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
