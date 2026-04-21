package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/oporpino/ward/internal/config"
	"github.com/spf13/cobra"
)

func NewNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new encrypted .ward file and open it in $EDITOR",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(fmt.Errorf("no ward project found — run `ward init` first"))
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}

			path := resolveNewPath(args[0], cfgPath, cfg)

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

			// Build example content using the directory name as root key
			stub := newFileStub(path, cfgPath)
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

			if err := maybeAddSource(cfgPath, path); err != nil {
				fatal(err)
			}
		},
	}
}

// resolveNewPath turns the user's input into a full file path.
//
// Rules:
//  1. Already has .ward extension and contains a path separator → use as-is (relative to CWD)
//  2. Otherwise → resolve inside the default_dir (or .ward/vault/ as fallback)
//     e.g. "staging" → "<default_dir>/staging.ward"
//     e.g. "staging/secrets.ward" → "<default_dir>/staging/secrets.ward"
func resolveNewPath(arg, cfgPath string, cfg *config.Config) string {
	// Absolute path: use as-is
	if filepath.IsAbs(arg) {
		return arg
	}

	defaultDir := cfg.DefaultDir
	if defaultDir == "" {
		defaultDir = ".ward/vault"
	}

	// Any path with a slash → use as-is relative to CWD (just add .ward if missing)
	if strings.ContainsRune(arg, '/') {
		return strings.TrimSuffix(arg, ".ward") + ".ward"
	}

	// Strip .ward suffix if present before joining
	name := strings.TrimSuffix(arg, ".ward")

	// Resolve default_dir relative to the project root (parent of .ward/)
	projectRoot := filepath.Dir(filepath.Dir(cfgPath))
	base := filepath.Join(projectRoot, defaultDir)

	return filepath.Join(base, name+".ward")
}

// newFileStub returns a YAML stub with the directory name as root key and
// placeholder secrets, matching the structure the user expects.
func newFileStub(filePath, cfgPath string) string {
	stem := strings.TrimSuffix(filepath.Base(filePath), ".ward")
	if stem == "" || stem == "." {
		stem = filepath.Base(filepath.Dir(filePath))
	}
	_ = cfgPath
	return fmt.Sprintf("%s:\n  secret_1: <your content>\n  secret_2: <your content>\n", stem)
}

// maybeAddSource appends the directory of newFile to the sources list in
// the config file if it is not already covered by an existing source.
func maybeAddSource(cfgPath, newFile string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil // best-effort
	}

	newDir, err := filepath.Abs(filepath.Dir(newFile))
	if err != nil {
		return nil
	}

	projectRoot := filepath.Dir(filepath.Dir(cfgPath))

	for _, src := range cfg.Vaults {
		srcAbs, err := filepath.Abs(filepath.Join(projectRoot, src.Path))
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(srcAbs, newDir)
		if err != nil {
			continue
		}
		if rel == "." || !strings.HasPrefix(rel, "..") {
			return nil // already covered
		}
	}

	rel, err := filepath.Rel(projectRoot, newDir)
	if err != nil {
		rel = newDir
	}
	sourcePath := filepath.Join(".", rel)

	cfg.Vaults = append(cfg.Vaults, config.Source{Path: sourcePath})
	if err := config.Save(cfgPath, cfg); err != nil {
		return fmt.Errorf("updating %s: %w", cfgPath, err)
	}
	fmt.Printf("  %s+%s added %s%s%s to vaults\n",
		clrGreen, clrReset, clrCyan, sourcePath, clrReset)
	return nil
}
