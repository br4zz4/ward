package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/oporpino/ward/internal/config"
	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/sops"
	"github.com/oporpino/ward/internal/ward"
)

var configFile = "ward.yaml"

func SetConfigFile(path string) {
	if path != "" {
		configFile = path
	}
}

// newEngine loads ward.yaml and returns a ready-to-use Engine.
func newEngine() (*ward.Engine, error) {
	cfg, err := config.Load(configFile)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", configFile, err)
	}
	dec := decryptorFor(cfg)
	return ward.NewEngine(cfg, dec), nil
}

// decryptorFor returns the appropriate Decryptor based on the config.
// Falls back to MockDecryptor when no key file is configured.
func decryptorFor(cfg *config.Config) sops.Decryptor {
	if cfg.Encryption.KeyFile != "" {
		return sops.SopsDecryptor{KeyFile: cfg.Encryption.KeyFile}
	}
	return sops.MockDecryptor{}
}

// fatal prints err to stderr and exits 1.
func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ward:", err)
	os.Exit(1)
}

// --- ANSI colour constants ---------------------------------------------------

const (
	clrReset     = "\033[0m"
	clrBold      = "\033[1m"
	clrGray      = "\033[90m"
	clrCyan      = "\033[36m"
	clrYellow    = "\033[33m"
	clrLightRed  = "\033[91m"
	clrGreen     = "\033[32m"
	clrOrange    = "\033[38;5;208m"
)

// --- presentation ------------------------------------------------------------

// printTree renders a node as plain YAML-like text (used by get).
func printTree(node *secrets.Node, indent int) {
	prefix := strings.Repeat("  ", indent)
	if node.Children != nil {
		for _, k := range sortedKeys(node.Children) {
			child := node.Children[k]
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

// listLine is one rendered row for the aligned-origin display.
type listLine struct {
	text     string
	origin   string
	conflict bool
}

// printTreeWithOrigin renders the merged tree with colour-coded leaf origins.
// conflictKeys is the set of leaf key names in conflict (may be nil).
func printTreeWithOrigin(node *secrets.Node, indent int, anchorPath string, conflictKeys map[string]bool) {
	var lines []listLine
	collectListLines(node, indent, anchorPath, conflictKeys, &lines)

	maxLen := 0
	for _, l := range lines {
		if l.origin != "" && visibleLen(l.text) > maxLen {
			maxLen = visibleLen(l.text)
		}
	}

	for _, l := range lines {
		if l.origin != "" {
			padding := strings.Repeat(" ", maxLen-visibleLen(l.text)+2)
			arrow := clrYellow
			if l.conflict {
				arrow = clrLightRed
			}
			fmt.Printf("%s%s%s←%s %s\n", l.text, padding, arrow, clrReset, l.origin)
		} else {
			fmt.Println(l.text)
		}
	}

	if len(conflictKeys) > 0 {
		fmt.Printf("\n%s%s●%s active  %s●%s overrides  %s●%s conflict%s\n",
			clrGray, clrGreen, clrGray, clrOrange, clrGray, clrLightRed, clrGray, clrReset)
	} else {
		fmt.Printf("\n%s%s●%s active  %s●%s overrides%s\n",
			clrGray, clrGreen, clrGray, clrOrange, clrGray, clrReset)
	}
}

// --- tree traversal ----------------------------------------------------------

func collectListLines(node *secrets.Node, indent int, anchorPath string, conflictKeys map[string]bool, lines *[]listLine) {
	if node.Children == nil {
		return
	}
	prefix := strings.Repeat("  ", indent)

	var leafKeys, mapKeys []string
	for k, child := range node.Children {
		if child.Children != nil {
			mapKeys = append(mapKeys, k)
		} else {
			leafKeys = append(leafKeys, k)
		}
	}
	sort.Slice(leafKeys, func(i, j int) bool {
		ci, cj := node.Children[leafKeys[i]], node.Children[leafKeys[j]]
		pi, pj := leafPriority(ci, leafKeys[i], conflictKeys), leafPriority(cj, leafKeys[j], conflictKeys)
		if pi != pj {
			return pi < pj
		}
		return leafKeys[i] < leafKeys[j]
	})
	sort.Strings(mapKeys)

	for _, k := range leafKeys {
		child := node.Children[k]
		color := leafColor(child, k, conflictKeys)
		*lines = append(*lines, listLine{
			text:     fmt.Sprintf("%s%s%s:%s %s%v%s", prefix, color, k, clrReset, clrGray, child.Value, clrReset),
			origin:   formatOrigin(child.Origin),
			conflict: conflictKeys[k],
		})
	}
	for _, k := range mapKeys {
		child := node.Children[k]
		*lines = append(*lines, listLine{
			text: fmt.Sprintf("%s%s%s%s:", prefix, clrBold, k, clrReset),
		})
		collectListLines(child, indent+1, anchorPath, conflictKeys, lines)
	}
}

// leafPriority returns sort order: 0=conflict, 1=override, 2=active.
func leafPriority(child *secrets.Node, k string, conflictKeys map[string]bool) int {
	switch {
	case conflictKeys[k]:
		return 0
	case child.Overrides:
		return 1
	default:
		return 2
	}
}

func leafColor(child *secrets.Node, k string, conflictKeys map[string]bool) string {
	switch {
	case conflictKeys[k]:
		return clrLightRed
	case child.Overrides:
		return clrOrange
	default:
		return clrGreen
	}
}

func formatOrigin(o secrets.Origin) string {
	if o.File == "" {
		return ""
	}
	if o.Line > 0 {
		return fmt.Sprintf("%s%s%s:%s%d%s", clrCyan, o.File, clrReset, clrLightRed, o.Line, clrReset)
	}
	return fmt.Sprintf("%s%s%s", clrCyan, o.File, clrReset)
}

// --- utilities ---------------------------------------------------------------

// visibleLen returns the visible (non-ANSI) length of s.
func visibleLen(s string) int {
	n, inEsc := 0, false
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

// sortedKeys returns the keys of m sorted alphabetically.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
