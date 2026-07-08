package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/ward"
	"github.com/spf13/cobra"
)

func NewSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "set <dot.path> <value>",
		Short:             "Set a single secret at a full dot-path",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			dotPath := args[0]
			value := args[1]

			enforceVaultStructure()

			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(fmt.Errorf("no ward project found — run `ward init` first"))
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}

			// A valid leaf path is vault.[subdirs].stem.leafname — at least 3 segments.
			if strings.Count(dotPath, ".") < 2 {
				fatal(fmt.Errorf("dot-path %q is too shallow — use vault.file.key (at least three segments)", dotPath))
			}

			vaultName := firstSegment(dotPath)
			vaultSrc := findVault(cfg, vaultName)
			if vaultSrc == nil {
				fatalVaultNotFound(vaultName)
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			files, err := eng.LoadFiles()
			if err != nil {
				fatal(err)
			}

			// Type-1 conflict: same dot-path defined in more than one file → abort.
			targets := resolveTargetFiles(files, dotPath)
			if len(targets) > 1 {
				fatal(fmt.Errorf("%s", ambiguousTargetError(dotPath, targets, files)))
			}

			var targetPath string
			created := false
			if len(targets) == 1 {
				targetPath = targets[0]
			} else {
				targetPath = resolveNewPath(fileStemPath(dotPath), vaultSrc.Path, cfgPath)
				created = true
			}

			// Decrypt (or start empty for a new file), mutate, re-encrypt.
			data := map[string]interface{}{}
			if !created {
				plain, err := eng.Decrypt(targetPath)
				if err != nil {
					fatal(fmt.Errorf("decrypting %s: %w", targetPath, err))
				}
				if err := yaml.Unmarshal(plain, &data); err != nil {
					fatal(fmt.Errorf("parsing %s: %w", targetPath, err))
				}
				if data == nil {
					data = map[string]interface{}{}
				}
			}

			setLeaf(data, dotPath, value)

			out, err := yaml.Marshal(data)
			if err != nil {
				fatal(fmt.Errorf("encoding YAML: %w", err))
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				fatal(fmt.Errorf("creating directory: %w", err))
			}
			if err := eng.Encrypt(targetPath, out); err != nil {
				fatal(fmt.Errorf("encrypting %s: %w", targetPath, err))
			}

			fmt.Fprintf(os.Stderr, "  %s✓%s set %s%s%s\n", clrGreen, clrReset, clrBold, dotPath, clrReset)
			if created {
				fmt.Fprintf(os.Stderr, "  %s→%s a new file was created: %s%s%s (no existing file defined this path)\n",
					clrGray, clrReset, clrCyan, targetPath, clrReset)
			}

			// Type-2 collision: env var now collides with a different dot-path → warn, do not fail.
			if warn := envCollisionWarning(eng, dotPath); warn != "" {
				fmt.Fprint(os.Stderr, warn)
			}
		},
	}
}

// firstSegment returns the first dot-path segment (the vault name).
func firstSegment(dotPath string) string {
	if i := strings.Index(dotPath, "."); i >= 0 {
		return dotPath[:i]
	}
	return dotPath
}

// fileStemPath derives the file path (relative to the vault dir, no extension)
// from a full leaf dot-path, honouring the vault structure rule
// vault.[subdirs].stem.leafname. It drops the vault (first) and the leaf (last)
// segments; the remaining segments form the file path.
//
// e.g. "app.services.api.token" → "services/api"
//
//	"app.staging.token"       → "staging"
func fileStemPath(dotPath string) string {
	parts := strings.Split(dotPath, ".")
	fileSegments := parts[1 : len(parts)-1]
	return strings.Join(fileSegments, string(filepath.Separator))
}

// findVault returns the config source matching name, or nil.
func findVault(cfg *config.Config, name string) *config.Source {
	for i := range cfg.Vaults {
		if cfg.Vaults[i].Name == name {
			return &cfg.Vaults[i]
		}
	}
	return nil
}

// resolveTargetFiles returns the files whose data defines the exact leaf dot-path.
func resolveTargetFiles(files []secrets.ParsedFile, dotPath string) []string {
	var out []string
	parts := strings.Split(dotPath, ".")
	for _, pf := range files {
		if leafExists(pf.Data, parts) {
			out = append(out, pf.File)
		}
	}
	return out
}

// resolvePathFiles returns the files whose data defines the dot-path at all,
// whether as a leaf or a group. unset uses this so a path pointing to a group
// still resolves to its file and can report a precise "group, not a leaf" error.
func resolvePathFiles(files []secrets.ParsedFile, dotPath string) []string {
	var out []string
	parts := strings.Split(dotPath, ".")
	for _, pf := range files {
		if pathExists(pf.Data, parts) {
			out = append(out, pf.File)
		}
	}
	return out
}

// leafExists reports whether data contains a scalar leaf at the given path segments.
func leafExists(data map[string]interface{}, parts []string) bool {
	current := data
	for i, p := range parts {
		v, ok := current[p]
		if !ok {
			return false
		}
		if i == len(parts)-1 {
			// Last segment must be a scalar leaf, not a map.
			_, isMap := v.(map[string]interface{})
			return !isMap
		}
		next, isMap := v.(map[string]interface{})
		if !isMap {
			return false
		}
		current = next
	}
	return false
}

// pathExists reports whether data contains the given path segments, as either a
// leaf or a group.
func pathExists(data map[string]interface{}, parts []string) bool {
	current := data
	for i, p := range parts {
		v, ok := current[p]
		if !ok {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		next, isMap := v.(map[string]interface{})
		if !isMap {
			return false
		}
		current = next
	}
	return false
}

// setLeaf sets value at the nested dot-path, creating intermediate maps as needed.
func setLeaf(data map[string]interface{}, dotPath, value string) {
	parts := strings.Split(dotPath, ".")
	current := data
	for i, p := range parts {
		if i == len(parts)-1 {
			current[p] = value
			return
		}
		next, isMap := current[p].(map[string]interface{})
		if !isMap {
			next = map[string]interface{}{}
			current[p] = next
		}
		current = next
	}
}

// unsetResult reports the outcome of unsetLeaf.
type unsetResult int

const (
	unsetNotFound unsetResult = iota // no key at the path
	unsetIsGroup                     // the path points to a group (map), not a leaf
	unsetRemoved                     // a leaf was removed
)

// unsetLeaf removes the leaf at dot-path, leaving the surrounding scaffold maps
// (vault.[subdirs].stem) in place so the file keeps a valid structure even when
// it becomes empty of secrets. It only removes a single leaf: if the path points
// to a group (a map with children) it removes nothing and reports unsetIsGroup.
func unsetLeaf(data map[string]interface{}, dotPath string) unsetResult {
	parts := strings.Split(dotPath, ".")
	current := data
	for i, p := range parts {
		if i == len(parts)-1 {
			v, ok := current[p]
			if !ok {
				return unsetNotFound
			}
			if _, isMap := v.(map[string]interface{}); isMap {
				return unsetIsGroup // a group, not a leaf — never remove a whole branch
			}
			delete(current, p)
			return unsetRemoved
		}
		next, isMap := current[p].(map[string]interface{})
		if !isMap {
			return unsetNotFound
		}
		current = next
	}
	return unsetNotFound
}

// fatalVaultNotFound prints a styled error and exits when the vault (first
// dot-path segment) does not exist. set never creates vaults.
func fatalVaultNotFound(vaultName string) {
	fmt.Fprintf(os.Stderr, "\n  %s✗ vault %q not found%s\n\n", clrLightRed+clrBold, vaultName, clrReset)
	fmt.Fprintf(os.Stderr, "  the first segment of the dot-path must be an existing vault.\n")
	fmt.Fprintf(os.Stderr, "  %s→%s see vaults with  %sward vault list%s\n", clrGray, clrReset, clrCyan, clrReset)
	fmt.Fprintf(os.Stderr, "  %s→%s add one with     %sward vault add %s <path>%s\n\n", clrGray, clrReset, clrCyan, vaultName, clrReset)
	os.Exit(1)
}

// ambiguousTargetError builds a Type-1 conflict message: the dot-path is defined
// in more than one file, so ward cannot know which file to write.
func ambiguousTargetError(dotPath string, targets []string, files []secrets.ParsedFile) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s%scannot set %s%s%s — defined in %d files:%s\n\n",
		clrBold, clrLightRed, clrReset+clrBold, dotPath, clrReset+clrBold+clrLightRed, len(targets), clrReset)
	for _, t := range targets {
		line := lineFor(files, t, dotPath)
		if line > 0 {
			fmt.Fprintf(&sb, "  %s%s%s:%s%d%s\n", clrCyan, t, clrReset, clrLightRed, line, clrReset)
		} else {
			fmt.Fprintf(&sb, "  %s%s%s\n", clrCyan, t, clrReset)
		}
	}
	fmt.Fprintf(&sb, "\n  %sto resolve:%s keep %s%s%s in only one of the files above\n",
		clrBold, clrReset, clrYellow, secrets.LeafKey(dotPath), clrReset)
	return sb.String()
}

// lineFor returns the source line of dotPath in the given file, or 0.
func lineFor(files []secrets.ParsedFile, file, dotPath string) int {
	for _, pf := range files {
		if pf.File == file {
			return pf.Lines[dotPath]
		}
	}
	return 0
}

// envCollisionWarning re-merges after a write and returns a non-blocking warning
// if the touched dot-path now leaves its env var in a Type-2 collision. Empty
// string means no warning.
func envCollisionWarning(eng *ward.Engine, dotPath string) string {
	result, err := eng.MergeForView()
	if err != nil {
		return ""
	}
	collision := envCollisionFor(result.Tree, dotPath)
	if collision == nil {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  %s⚠ warning:%s env var %s%s%s now collides\n",
		clrYellow+clrBold, clrReset, clrBold, collision.EnvKey, clrReset)
	if collision.CaseCollision {
		fmt.Fprintf(&sb, "    %s%s%s and %s%s%s differ only in case\n",
			clrYellow, collision.DotPaths[0], clrReset, clrYellow, collision.DotPaths[1], clrReset)
		fmt.Fprintf(&sb, "    %s→%s use consistent casing across vaults\n\n", clrGray, clrReset)
	} else {
		fmt.Fprintf(&sb, "    %s%s%s\n", clrYellow, collision.DotPaths[0], clrReset)
		fmt.Fprintf(&sb, "    %s%s%s\n", clrYellow, collision.DotPaths[1], clrReset)
		fmt.Fprintf(&sb, "    %s→%s use %s--prefixed%s or scope the path on %sward exec%s\n\n",
			clrGray, clrReset, clrCyan, clrReset, clrCyan, clrReset)
	}
	return sb.String()
}

// envCollisionFor returns the Type-2 env var collision affecting dotPath's leaf,
// or nil if there is none.
func envCollisionFor(tree map[string]*secrets.Node, dotPath string) *secrets.EnvConflict {
	_, err := secrets.ToFlatEnvEntries(tree, "")
	if err == nil {
		return nil
	}
	ce, ok := err.(*secrets.EnvConflictError)
	if !ok {
		return nil
	}
	leaf := strings.ReplaceAll(secrets.LeafKey(dotPath), "-", "_")
	for i := range ce.Conflicts {
		if ce.Conflicts[i].EnvKey == leaf {
			return &ce.Conflicts[i]
		}
		if ce.Conflicts[i].CaseCollision &&
			strings.EqualFold(ce.Conflicts[i].EnvKey, leaf) {
			return &ce.Conflicts[i]
		}
	}
	return nil
}
