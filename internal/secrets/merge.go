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

// Conflict holds a single key conflict between two or more origins.
type Conflict struct {
	Key     string
	Sources []Origin
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
		// Per-conflict resolution hints with dot-path examples
		leafKey := LeafKey(c.Key)
		scopePath := parentKey(c.Key) // e.g. company.sectors.one.staging
		fmt.Fprintf(&sb, "\n  %sto resolve:%s\n", colorBold, colorReset)
		fmt.Fprintf(&sb, "    %s1.%s keep %s%s%s in only one of the files above\n",
			colorGray, colorReset, colorYellow, leafKey, colorReset)
		grandparent := parentKey(scopePath)
		if grandparent == scopePath {
			// at root level — the only shared ancestor would be a new base vault
			fmt.Fprintf(&sb, "    %s2.%s move it to a shared base vault that all conflicting files include\n",
				colorGray, colorReset)
		} else {
			movedPath := grandparent + "." + leafKey // e.g. company.sectors.one.database_url
			fmt.Fprintf(&sb, "    %s2.%s hoist it one level — define %s%s%s in a shared ancestor file\n",
				colorGray, colorReset, colorYellow, movedPath, colorReset)
		}
		// Only show scope hint when the path has depth > 1 (scoping to a leaf's parent is not useful)
		if scopePath != c.Key && grandparent != scopePath {
			fmt.Fprintf(&sb, "    %s3.%s scope your command to skip the conflict:\n",
				colorGray, colorReset)
			fmt.Fprintf(&sb, "         %sward exec %s -- <cmd>%s\n",
				colorCyan, scopePath, colorReset)
			fmt.Fprintf(&sb, "         %sward envs %s%s\n",
				colorCyan, scopePath, colorReset)
		}
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "  %s→ read more:%s https://github.com/oporpino/ward/blob/main/docs/conflicts.md\n",
		colorGray, colorReset)
	return sb.String()
}

// LeafKey returns the last segment of a dot-path.
func LeafKey(dotPath string) string {
	if i := strings.LastIndex(dotPath, "."); i >= 0 {
		return dotPath[i+1:]
	}
	return dotPath
}

// parentKey returns the dot-path one level above the leaf.
func parentKey(dotPath string) string {
	if i := strings.LastIndex(dotPath, "."); i >= 0 {
		return dotPath[:i]
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
					appendConflict(conflicts, dotPath, existing.Origin, originFor(file, dotPath, lines, rawLines, spec))
					continue
				}
				dst[k] = &Node{Children: map[string]*Node{}}
			}
			mergeInto(dst[k].Children, val, file, lines, rawLines, mode, dotPath, spec, conflicts)

		default:
			existing, ok := dst[k]
			if ok && mode == config.MergeModeError && existing.Origin.Specificity == spec {
				appendConflict(conflicts, dotPath, existing.Origin, originFor(file, dotPath, lines, rawLines, spec))
				continue
			}
			overrides := ok // replacing an existing value from a less-specific file
			dst[k] = &Node{Value: val, Origin: originFor(file, dotPath, lines, rawLines, spec), Overrides: overrides}
		}
	}
}

// appendConflict adds newOrigin to an existing conflict for dotPath, or creates a new one.
func appendConflict(conflicts *[]Conflict, dotPath string, existingOrigin, newOrigin Origin) {
	for i, c := range *conflicts {
		if c.Key == dotPath {
			(*conflicts)[i].Sources = append((*conflicts)[i].Sources, newOrigin)
			return
		}
	}
	*conflicts = append(*conflicts, Conflict{
		Key:     dotPath,
		Sources: []Origin{existingOrigin, newOrigin},
	})
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
