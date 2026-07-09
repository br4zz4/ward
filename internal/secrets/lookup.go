package secrets

import (
	"fmt"
	"sort"
	"strings"
)

// KeyNotFoundError describes a dot-path that could not be resolved in a merged
// tree, naming where the walk broke and which keys were available there.
type KeyNotFoundError struct {
	DotPath   string   // the full path the user asked for
	AtPath    string   // the resolved prefix where the walk broke ("" = top level)
	Available []string // keys available at that level, sorted
}

func (e *KeyNotFoundError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "key not found: %s", e.DotPath)
	where := "at top level"
	if e.AtPath != "" {
		where = fmt.Sprintf("under %s", e.AtPath)
	}
	if len(e.Available) == 0 {
		fmt.Fprintf(&sb, "\n  nothing available %s", where)
		return sb.String()
	}
	fmt.Fprintf(&sb, "\n  available %s: %s", where, strings.Join(e.Available, ", "))
	return sb.String()
}

// Lookup navigates tree by dot-path and returns the node found there, or a
// *KeyNotFoundError that reports the level where the path broke and the keys
// available at that level.
func Lookup(tree map[string]*Node, dotPath string) (*Node, error) {
	parts := strings.Split(dotPath, ".")
	current := &Node{Children: tree}
	for i, part := range parts {
		if current.Children == nil {
			return nil, notFoundAt(dotPath, parts, i, nil)
		}
		next, ok := current.Children[part]
		if !ok {
			return nil, notFoundAt(dotPath, parts, i, current.Children)
		}
		current = next
	}
	return current, nil
}

// notFoundAt builds a KeyNotFoundError for a break at segment index i, listing
// the keys available in the map at that level (level may be nil → a leaf blocked
// the path, so nothing is available).
func notFoundAt(dotPath string, parts []string, i int, level map[string]*Node) *KeyNotFoundError {
	return &KeyNotFoundError{
		DotPath:   dotPath,
		AtPath:    strings.Join(parts[:i], "."),
		Available: sortedNodeKeys(level),
	}
}

func sortedNodeKeys(m map[string]*Node) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
