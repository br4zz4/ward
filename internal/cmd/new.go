package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/br4zz4/ward/internal/config"
	"github.com/spf13/cobra"
)

func NewNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "new <vault> <path>",
		Short:             "Create a new encrypted .ward file and open it in $EDITOR",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeVaultNames,
		Run: func(_ *cobra.Command, args []string) {
			vaultName := args[0]
			pathArg := args[1]

			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(fmt.Errorf("no ward project found — run `ward init` first"))
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}

			// find vault by name
			var vaultSrc *config.Source
			for i := range cfg.Vaults {
				if cfg.Vaults[i].Name == vaultName {
					vaultSrc = &cfg.Vaults[i]
					break
				}
			}
			if vaultSrc == nil {
				fatal(fmt.Errorf("vault %q not found — use `ward vault list` to see available vaults", vaultName))
			}

			path := resolveNewPath(pathArg, vaultSrc.Path, cfgPath)

			// Colored confirmation
			fmt.Printf("\n  %s→%s creating %s%s%s\n\n", clrGray, clrReset, clrCyan, path, clrReset)
			fmt.Printf("  continue? [y/N] ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				fmt.Printf("  %saborted%s\n\n", clrGray, clrReset)
				return
			}
			fmt.Println()

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

			// Build example content using the vault name as root key
			stub := newFileStub(vaultName, path, cfgPath)
			if err := eng.Encrypt(path, []byte(stub)); err != nil {
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
		},
	}
}

// resolveNewPath turns the user's input into a full file path inside the vault.
//
// arg is resolved inside vaultPath (relative to project root).
// projectRoot is derived from cfgPath (parent of .ward/).
func resolveNewPath(arg, vaultPath, cfgPath string) string {
	name := strings.TrimSuffix(arg, ".ward")
	projectRoot := filepath.Dir(filepath.Dir(cfgPath))
	return filepath.Join(projectRoot, vaultPath, name+".ward")
}

// newFileStub returns a YAML stub whose keys reflect the vault hierarchy.
//
// vaultName is used as the root key. Sub-directories within the vault become
// intermediate keys, and the file stem is the leaf key.
//
// Examples (vaultName = "myapp"):
//
//	file ".ward/vaults/myapp/staging.ward"
//	  → myapp:\n  staging:\n    secret_1: …
//
//	file ".ward/vaults/myapp/services/api.ward"
//	  → myapp:\n  services:\n    api:\n      secret_1: …
func newFileStub(vaultName, filePath, cfgPath string) string {
	fileAbs, err := filepath.Abs(filePath)
	if err != nil {
		fileAbs = filePath
	}

	stem := strings.TrimSuffix(filepath.Base(fileAbs), ".ward")
	projectRoot, _ := filepath.Abs(filepath.Dir(filepath.Dir(cfgPath)))

	cfg, cfgErr := config.Load(cfgPath)

	var segments []string

	if cfgErr == nil {
		for _, src := range cfg.Vaults {
			if src.Name != vaultName {
				continue
			}
			vaultAbs, err := filepath.Abs(filepath.Join(projectRoot, src.Path))
			if err != nil {
				continue
			}
			// Check if file is inside this vault
			rel, err := filepath.Rel(vaultAbs, filepath.Dir(fileAbs))
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}

			var subParts []string
			if rel != "." {
				subParts = strings.Split(rel, string(filepath.Separator))
			}

			segments = append([]string{vaultName}, subParts...)
			segments = append(segments, stem)
			break
		}
	}

	// Fallback: vault name + stem
	if len(segments) == 0 {
		segments = []string{vaultName, stem}
	}

	return buildNestedYAML(segments)
}

// buildNestedYAML turns ["a","b","c"] into:
//
//	a:
//	  b:
//	    c:
//	      secret_1: <your content>
//	      secret_2: <your content>
func buildNestedYAML(segments []string) string {
	var sb strings.Builder
	for i, seg := range segments {
		indent := strings.Repeat("  ", i)
		sb.WriteString(indent + seg + ":\n")
	}
	leaf := strings.Repeat("  ", len(segments))
	sb.WriteString(leaf + "secret_1: <your content>\n")
	sb.WriteString(leaf + "secret_2: <your content>\n")
	return sb.String()
}

