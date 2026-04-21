package secrets

import (
	"fmt"
	"strings"

	"github.com/oporpino/ward/internal/config"
)

const (
	colorRed      = "\033[31m"
	colorLightRed = "\033[91m"
	colorYellow   = "\033[33m"
	colorCyan     = "\033[36m"
	colorGreen    = "\033[32m"
	colorGray     = "\033[90m"
	colorBold     = "\033[1m"
	colorReset    = "\033[0m"
)

// Origin tracks where a leaf value came from.
type Origin struct {
	File        string
	Line        int
	Snippet     string
	Specificity int // higher = more specific (leaf); used to detect same-level conflicts
}

// Node is either a leaf value with an origin, or a nested map.
type Node struct {
	Value     interface{}
	Origin    Origin
	Overrides bool // true when this value replaced a value from a less-specific (ancestor) file
	Children  map[string]*Node
}

// Conflict holds a single key conflict between two origins.
type Conflict struct {
	Key     string
	Sources [2]Origin
}

// ConflictError is returned when one or more keys are defined in multiple files at the same level.
type ConflictError struct {
	Conflicts []Conflict
}

func (e *ConflictError) Error() string {
	var sb strings.Builder
	n := len(e.Conflicts)
	word := "conflict"
	if n > 1 {
		word = "conflicts"
	}
	fmt.Fprintf(&sb, "%s%sfound %d %s%s — cannot merge:\n\n",
		colorBold, colorRed, n, word, colorReset,
	)
	for _, c := range e.Conflicts {
		// Dot-path on its own line, prominent
		fmt.Fprintf(&sb, "%s%s%s\n", colorBold, c.Key, colorReset)
		for _, s := range c.Sources {
			if s.Line > 0 {
				fmt.Fprintf(&sb, "  %s%s%s:%s%d%s\n",
					colorCyan, s.File, colorReset,
					colorLightRed, s.Line, colorReset,
				)
				if s.Snippet != "" {
					fmt.Fprintf(&sb, "    %s%s%s\n", colorGray, s.Snippet, colorReset)
				}
			} else {
				fmt.Fprintf(&sb, "  %s%s%s\n",
					colorCyan, s.File, colorReset,
				)
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("  to resolve:\n")
	sb.WriteString("    1. remove the key from one of the files\n")
	fmt.Fprintf(&sb, "    2. move it to a common %sancestor%s if shared across %senvironments%s\n",
		colorGreen, colorReset,
		colorCyan, colorReset,
	)
	return sb.String()
}

// LeafKey returns the last segment of a dot-path.
func LeafKey(dotPath string) string {
	if i := strings.LastIndex(dotPath, "."); i >= 0 {
		return dotPath[i+1:]
	}
	return dotPath
}

// Merge merges a sequence of ParsedFiles in order (index 0 = most ancestral, last = leaf).
// Files must be pre-sorted from least specific to most specific (SortBySpecificity).
// A conflict is only raised when two files at the same specificity level define the same key.
func Merge(files []ParsedFile, mode config.MergeMode) (map[string]*Node, error) {
	result := map[string]*Node{}
	var conflicts []Conflict

	for _, pf := range files {
		spec := specificity(pf)
		mergeInto(result, pf.Data, pf.File, pf.Lines, pf.RawLines, mode, "", spec, &conflicts)
	}

	if len(conflicts) > 0 {
		return nil, &ConflictError{Conflicts: conflicts}
	}
	return result, nil
}

func mergeInto(dst map[string]*Node, src map[string]interface{}, file string, lines LineMap, rawLines []string, mode config.MergeMode, prefix string, spec int, conflicts *[]Conflict) {
	for k, v := range src {
		dotPath := k
		if prefix != "" {
			dotPath = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			existing, ok := dst[k]
			if !ok || existing.Children == nil {
				if ok && existing.Children == nil && mode == config.MergeModeError && existing.Origin.Specificity == spec {
					*conflicts = append(*conflicts, Conflict{
						Key:     dotPath,
						Sources: [2]Origin{existing.Origin, originFor(file, dotPath, lines, rawLines, spec)},
					})
					continue
				}
				dst[k] = &Node{Children: map[string]*Node{}}
			}
			mergeInto(dst[k].Children, val, file, lines, rawLines, mode, dotPath, spec, conflicts)

		default:
			existing, ok := dst[k]
			if ok && mode == config.MergeModeError && existing.Origin.Specificity == spec {
				*conflicts = append(*conflicts, Conflict{
					Key:     dotPath,
					Sources: [2]Origin{existing.Origin, originFor(file, dotPath, lines, rawLines, spec)},
				})
				continue
			}
			overrides := ok // replacing an existing value from a less-specific file
			dst[k] = &Node{Value: val, Origin: originFor(file, dotPath, lines, rawLines, spec), Overrides: overrides}
		}
	}
}

func originFor(file, dotPath string, lines LineMap, rawLines []string, spec int) Origin {
	o := Origin{File: file, Specificity: spec}
	if lines != nil {
		if ln, ok := lines[dotPath]; ok {
			o.Line = ln
			if ln > 0 && ln <= len(rawLines) {
				o.Snippet = strings.TrimSpace(rawLines[ln-1])
			}
		}
	}
	return o
}
