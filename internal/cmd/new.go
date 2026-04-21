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

	// Paths starting with ./ or ../ or a hidden dir (e.g. ".commons/...") → explicit
	// relative path, use as-is
	if strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "../") || strings.HasPrefix(arg, ".") {
		return strings.TrimSuffix(arg, ".ward") + ".ward"
	}

	// Everything else (bare name or bare subpath like "environments/staging")
	// → place inside the default vault dir
	name := strings.TrimSuffix(arg, ".ward")

	// Resolve default_dir relative to the project root (parent of .ward/)
	projectRoot := filepath.Dir(filepath.Dir(cfgPath))
	base := filepath.Join(projectRoot, defaultDir)

	return filepath.Join(base, name+".ward")
}

// newFileStub returns a YAML stub whose keys reflect the vault hierarchy.
//
// The path segments are derived from the vault source path that contains the
// file, plus any subdirectory depth within that vault, plus the file stem.
//
// Examples (projectRoot = /app):
//
//	vault ".ward/vault",  file ".ward/vault/staging.ward"
//	  → staging:\n  secret_1: …
//
//	vault "../.commons/stacks/ruby",  file "../.commons/stacks/ruby/staging.ward"
//	  → commons:\n  stacks:\n    ruby:\n      staging:\n        secret_1: …
//
//	vault ".ward/vault",  file ".ward/vault/services/api.ward"
//	  → services:\n  api:\n    secret_1: …
func newFileStub(filePath, cfgPath string) string {
	fileAbs, err := filepath.Abs(filePath)
	if err != nil {
		fileAbs = filePath
	}

	stem := strings.TrimSuffix(filepath.Base(fileAbs), ".ward")
	projectRoot, _ := filepath.Abs(filepath.Dir(filepath.Dir(cfgPath)))
	projectName := filepath.Base(projectRoot)

	cfg, cfgErr := config.Load(cfgPath)

	var segments []string

	if cfgErr == nil {
		for _, src := range cfg.Vaults {
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

			// Is this vault inside the project root?
			vaultRelToProject, err := filepath.Rel(projectRoot, vaultAbs)
			isExternal := err != nil || strings.HasPrefix(vaultRelToProject, "..")

			if isExternal {
				// External vault: derive root from vault path segments (no leading dots)
				segments = append(vaultPathSegments(src.Path), subParts...)
			} else {
				// Internal vault: use project name as root + subpath inside vault
				segments = append([]string{projectName}, subParts...)
			}
			segments = append(segments, stem)
			break
		}
	}

	// Fallback: project name + stem
	if len(segments) == 0 {
		segments = []string{projectName, stem}
	}

	return buildNestedYAML(segments)
}

// vaultPathSegments converts a vault path like "../.commons/stacks/ruby" into
// clean segments ["commons", "stacks", "ruby"], stripping leading dots/slashes.
func vaultPathSegments(vaultPath string) []string {
	// Clean and split
	clean := filepath.Clean(vaultPath)
	parts := strings.Split(clean, string(filepath.Separator))
	var out []string
	for _, p := range parts {
		if p == "." || p == ".." || p == "" {
			continue
		}
		// Strip leading dot from hidden dirs (e.g. ".commons" → "commons")
		p = strings.TrimPrefix(p, ".")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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
