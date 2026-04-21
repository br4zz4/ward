package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/oporpino/ward/internal/config"
	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/sops"
)

var configFile = "ward.yaml"

func SetConfigFile(path string) {
	if path != "" {
		configFile = path
	}
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(configFile)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", configFile, err)
	}
	return cfg, nil
}

func sourcePaths(cfg *config.Config) []string {
	paths := make([]string, len(cfg.Sources))
	for i, s := range cfg.Sources {
		paths[i] = s.Path
	}
	return paths
}

// loadAndMerge discovers, loads, and merges all .ward files.
// If anchorPath is non-empty, only files sharing a root key with the anchor are loaded.
func loadAndMerge(cfg *config.Config, anchorPath string) (map[string]*secrets.Node, error) {
	dec := sops.MockDecryptor{} // TODO: replace with SopsDecryptor

	paths, err := secrets.Discover(sourcePaths(cfg))
	if err != nil {
		return nil, err
	}

	files, err := secrets.LoadAll(paths, dec)
	if err != nil {
		return nil, err
	}

	return loadAndMergeWithMode(cfg, anchorPath, files, cfg.Merge)
}

// loadAndMergeWithMode merges the given pre-loaded files with an explicit merge mode.
// Used by view to run two passes: one for conflict detection, one for display.
func loadAndMergeWithMode(cfg *config.Config, anchorPath string, files []secrets.ParsedFile, mode config.MergeMode) (map[string]*secrets.Node, error) {
	ordered := buildOrderedFiles(cfg, anchorPath, files)
	if ordered == nil {
		return nil, fmt.Errorf("anchor not found: %s", anchorPath)
	}

	// Dir anchor always enforces error mode for same-level conflicts
	if anchorPath != "" {
		if info, err := os.Stat(anchorPath); err == nil && info.IsDir() && mode != config.MergeModeOverride {
			mode = config.MergeModeError
		}
	}

	return secrets.Merge(ordered, mode)
}

// buildOrderedFiles returns the sorted slice of ParsedFiles to merge for the given anchor.
// Returns nil if the anchor path doesn't exist.
func buildOrderedFiles(cfg *config.Config, anchorPath string, files []secrets.ParsedFile) []secrets.ParsedFile {
	if anchorPath == "" {
		return secrets.SortBySpecificity(files)
	}

	info, err := os.Stat(anchorPath)
	if err != nil {
		return nil
	}

	if info.IsDir() {
		dirFiles, err := secrets.Discover([]string{anchorPath})
		if err != nil {
			return nil
		}
		dirSet := make(map[string]bool, len(dirFiles))
		for _, p := range dirFiles {
			dirSet[p] = true
		}
		var dirParsed []secrets.ParsedFile
		for _, f := range files {
			if dirSet[f.File] {
				dirParsed = append(dirParsed, f)
			}
		}
		var ancestors []secrets.ParsedFile
		for _, df := range dirParsed {
			for _, f := range files {
				if !dirSet[f.File] && secrets.IsAncestorOf(f, df) && secrets.MapDepth(f.Data) < secrets.MapDepth(df.Data) {
					ancestors = append(ancestors, f)
				}
			}
		}
		seen := map[string]bool{}
		var uniqueAncestors []secrets.ParsedFile
		for _, f := range ancestors {
			if !seen[f.File] {
				seen[f.File] = true
				uniqueAncestors = append(uniqueAncestors, secrets.TrimToScope(f, dirParsed))
			}
		}
		return secrets.SortBySpecificity(append(uniqueAncestors, dirParsed...))
	}

	// File anchor
	var anchor secrets.ParsedFile
	for _, f := range files {
		if f.File == anchorPath {
			anchor = f
			break
		}
	}
	if anchor.File == "" {
		return nil
	}
	return secrets.FilterByAnchor(anchor, files)
}

// getAtPath navigates a merged tree by dot-path and returns the subtree node.
func getAtPath(tree map[string]*secrets.Node, dotPath string) (*secrets.Node, error) {
	parts := strings.Split(dotPath, ".")
	current := &secrets.Node{Children: tree}
	for _, part := range parts {
		if current.Children == nil {
			return nil, fmt.Errorf("key not found: %s", dotPath)
		}
		next, ok := current.Children[part]
		if !ok {
			return nil, fmt.Errorf("key not found: %s", dotPath)
		}
		current = next
	}
	return current, nil
}

// printTree prints the merged tree as YAML-like output to stdout.
func printTree(node *secrets.Node, indent int) {
	prefix := strings.Repeat("  ", indent)
	if node.Children != nil {
		for k, child := range node.Children {
			if child.Children != nil {
				fmt.Printf("%s%s:\n", prefix, k)
				printTree(child, indent+1)
			} else {
				fmt.Printf("%s%s: %v\n", prefix, k, child.Value)
			}
		}
	} else {
		fmt.Printf("%s%v\n", prefix, node.Value)
	}
}

// listLine holds a single rendered line for aligned origin display.
type listLine struct {
	text     string // "  key: value"
	origin   string // "file:line"
	conflict bool   // true if this key is in conflict
}

// printTreeWithOrigin prints the merged tree with origins aligned in a column.
const (
	clrReset     = "\033[0m"
	clrBold      = "\033[1m"
	clrGray      = "\033[90m"
	clrCyan      = "\033[36m"
	clrYellow    = "\033[33m"
	clrLightRed  = "\033[91m"
	clrLightBlue = "\033[94m"
	clrGreen     = "\033[32m"
)

func printTreeWithOrigin(node *secrets.Node, indent int, anchorPath string, conflictKeys map[string]bool) {
	// Collect all leaf key names that come from outside the anchor scope (ancestors)
	ancestorKeys := map[string]bool{}
	collectAncestorKeys(node, anchorPath, ancestorKeys)

	var lines []listLine
	collectListLines(node, indent, anchorPath, ancestorKeys, conflictKeys, &lines)

	// Find max visible text width for alignment (strip ANSI codes)
	maxLen := 0
	for _, l := range lines {
		if l.origin != "" && visibleLen(l.text) > maxLen {
			maxLen = visibleLen(l.text)
		}
	}

	hasConflicts := len(conflictKeys) > 0

	for _, l := range lines {
		if l.origin != "" {
			padding := strings.Repeat(" ", maxLen-visibleLen(l.text)+2)
			arrowColor := clrYellow
			if l.conflict {
				arrowColor = clrLightRed
			}
			fmt.Printf("%s%s%s←%s %s\n", l.text, padding, arrowColor, clrReset, l.origin)
		} else {
			fmt.Println(l.text)
		}
	}

	if hasConflicts {
		fmt.Printf("\n%s%s●%s active  %s●%s overrides  %s●%s conflict%s\n",
			clrGray, clrGreen, clrGray,
			"\033[38;5;208m", clrGray,
			clrLightRed, clrGray,
			clrReset,
		)
	} else {
		fmt.Printf("\n%s%s●%s active  %s●%s overrides%s\n",
			clrGray, clrGreen, clrGray,
			"\033[38;5;208m", clrGray,
			clrReset,
		)
	}
}

// visibleLen returns the length of s ignoring ANSI escape sequences.
func visibleLen(s string) int {
	n := 0
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEsc = true
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

// collectAncestorKeys recursively collects all leaf key names whose origin is outside the anchor scope.
func collectAncestorKeys(node *secrets.Node, anchorPath string, out map[string]bool) {
	if node.Children == nil {
		return
	}
	for k, child := range node.Children {
		if child.Children != nil {
			collectAncestorKeys(child, anchorPath, out)
		} else if !isFromAnchorScope(child.Origin.File, anchorPath) {
			out[k] = true
		}
	}
}

func collectListLines(node *secrets.Node, indent int, anchorPath string, ancestorKeys map[string]bool, conflictKeys map[string]bool, lines *[]listLine) {
	if node.Children == nil {
		return
	}
	prefix := strings.Repeat("  ", indent)

	// Collect keys, sort: leaves first (alphabetical), then maps (alphabetical)
	var leafKeys, mapKeys []string
	for k, child := range node.Children {
		if child.Children != nil {
			mapKeys = append(mapKeys, k)
		} else {
			leafKeys = append(leafKeys, k)
		}
	}
	sort.Strings(leafKeys)
	sort.Strings(mapKeys)

	for _, k := range leafKeys {
		child := node.Children[k]
		origin := ""
		// Red = conflict; green = new key in anchor; orange = overrides ancestor; light blue = inherited
		var keyColor string
		if conflictKeys[k] {
			keyColor = clrLightRed
		} else if !isFromAnchorScope(child.Origin.File, anchorPath) && anchorPath != "" {
			keyColor = clrLightBlue // inherited from ancestor
		} else if child.Overrides || ancestorKeys[k] {
			keyColor = "\033[38;5;208m" // orange = key also appears in ancestor
		} else {
			keyColor = clrGreen // new key, only in anchor
		}
		if child.Origin.File != "" {
			if child.Origin.Line > 0 {
				origin = fmt.Sprintf("%s%s%s:%s%d%s", clrCyan, child.Origin.File, clrReset, clrLightRed, child.Origin.Line, clrReset)
			} else {
				origin = fmt.Sprintf("%s%s%s", clrCyan, child.Origin.File, clrReset)
			}
		}
		*lines = append(*lines, listLine{
			text:     fmt.Sprintf("%s%s%s:%s %s%v%s", prefix, keyColor, k, clrReset, clrGray, child.Value, clrReset),
			origin:   origin,
			conflict: conflictKeys[k],
		})
	}

	for _, k := range mapKeys {
		child := node.Children[k]
		*lines = append(*lines, listLine{
			text: fmt.Sprintf("%s%s%s%s:", prefix, clrBold, k, clrReset),
		})
		collectListLines(child, indent+1, anchorPath, ancestorKeys, conflictKeys, lines)
	}
}

// isFromAnchorScope returns true if the origin file is within the anchor's scope.
// For a dir anchor: the origin file must be inside the dir.
// For a file anchor: the origin file must be exactly the anchor file.
func isFromAnchorScope(originFile, anchorPath string) bool {
	if originFile == "" || anchorPath == "" {
		return false
	}
	// Normalize: add trailing slash for dir comparison
	info, err := os.Stat(anchorPath)
	if err != nil {
		return originFile == anchorPath
	}
	if info.IsDir() {
		dir := anchorPath
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		return strings.HasPrefix(originFile, dir)
	}
	return originFile == anchorPath
}

// lastEnvSegment returns the last underscore-separated segment of an env var name (lowercased).
// E.g. "STAGING_DATABASE_URL" → "database_url", "NAME" → "name".
func lastEnvSegment(envKey string) string {
	lower := strings.ToLower(envKey)
	// Find the first underscore that separates a known prefix from the leaf
	// We use the origin key directly — just return the full lowercased key as fallback
	// and also try stripping common single-segment prefixes.
	return lower
}

// fatal prints an error to stderr and exits.
func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ward:", err)
	os.Exit(1)
}
