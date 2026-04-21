package secrets

import (
	"fmt"
	"strings"
)

// EnvEntry holds an env var's value and the origin node it came from.
type EnvEntry struct {
	Value     string
	Origin    Origin
	Overrides bool // true when this value replaced a value from an ancestor file
}

// ToEnvVars converts all leaf nodes of a merged tree into env var pairs with full path.
// Example: company.sectors.one.staging.database_url → COMPANY_SECTORS_ONE_STAGING_DATABASE_URL
func ToEnvVars(tree map[string]*Node) map[string]string {
	entries := ToEnvEntries(tree)
	out := make(map[string]string, len(entries))
	for k, e := range entries {
		out[k] = e.Value
	}
	return out
}

// ToEnvEntries is like ToEnvVars but preserves origin information.
func ToEnvEntries(tree map[string]*Node) map[string]EnvEntry {
	result := map[string]EnvEntry{}
	collectEnvEntries(tree, "", result)
	return result
}

// ToEnvVarsFromAnchor returns env vars scoped to the anchor's container level,
// using relative names (stripping the common ancestor prefix).
// Example: anchor at company.sectors.one → NAME, STAGING_DATABASE_URL, etc.
func ToEnvVarsFromAnchor(tree map[string]*Node, anchorData map[string]interface{}) map[string]string {
	entries := ToEnvEntriesFromAnchor(tree, anchorData)
	out := make(map[string]string, len(entries))
	for k, e := range entries {
		out[k] = e.Value
	}
	return out
}

// parentDotPath returns the dot-path one level above the leaf (strips last segment).
func parentDotPath(dotPath string) string {
	if i := strings.LastIndex(dotPath, "."); i >= 0 {
		return dotPath[:i]
	}
	return dotPath
}

// EnvConflict holds a single env var name collision between two dot-paths.
type EnvConflict struct {
	EnvKey   string
	DotPaths [2]string
}

// EnvConflictError is returned when flat env var names collide across different dot-paths.
type EnvConflictError struct {
	Conflicts []EnvConflict
}

func (e *EnvConflictError) Error() string {
	var sb strings.Builder
	n := len(e.Conflicts)
	word := "collision"
	if n > 1 {
		word = "collisions"
	}
	fmt.Fprintf(&sb, "%s%sfound %d env var %s%s — use a more specific dot-path:\n\n",
		colorBold, colorRed, n, word, colorReset,
	)
	for _, c := range e.Conflicts {
		fmt.Fprintf(&sb, "%s%s%s%s\n", colorBold, colorPink, c.EnvKey, colorReset)
		fmt.Fprintf(&sb, "  defined under %s%s%s\n", colorYellow, c.DotPaths[0], colorReset)
		fmt.Fprintf(&sb, "  defined under %s%s%s\n\n", colorYellow, c.DotPaths[1], colorReset)
		fmt.Fprintf(&sb, "  %sto resolve:%s\n", colorBold, colorReset)
		fmt.Fprintf(&sb, "    %s1.%s scope to the path you need:\n", colorGray, colorReset)
		fmt.Fprintf(&sb, "         %sward exec %s -- <cmd>%s\n", colorCyan, parentDotPath(c.DotPaths[0]), colorReset)
		fmt.Fprintf(&sb, "         %sward exec %s -- <cmd>%s\n", colorCyan, parentDotPath(c.DotPaths[1]), colorReset)
		fmt.Fprintf(&sb, "    %s2.%s use %s--prefixed%s to keep full path names:\n", colorGray, colorReset, colorCyan, colorReset)
		fmt.Fprintf(&sb, "         %sward exec --prefixed -- <cmd>%s\n\n", colorCyan, colorReset)
	}
	fmt.Fprintf(&sb, "  %s→ read more:%s https://github.com/oporpino/ward/blob/main/docs/conflicts.md\n",
		colorGray, colorReset)
	return sb.String()
}

// ToFlatEnvEntries returns only the leaf values as env vars using just the leaf key name
// (uppercased), without any path prefix. Used by ward envs/exec without --prefixed.
// Returns EnvConflictError when two different dot-paths produce the same env var name.
func ToFlatEnvEntries(tree map[string]*Node) (map[string]EnvEntry, error) {
	out := map[string]EnvEntry{}
	dotPaths := map[string]string{} // envKey → first dot-path that set it
	var conflicts []EnvConflict
	collectFlatEntries(tree, "", out, dotPaths, &conflicts)
	if len(conflicts) > 0 {
		return nil, &EnvConflictError{Conflicts: conflicts}
	}
	return out, nil
}

func collectFlatEntries(nodes map[string]*Node, prefix string, out map[string]EnvEntry, dotPaths map[string]string, conflicts *[]EnvConflict) {
	for k, node := range nodes {
		dotPath := k
		if prefix != "" {
			dotPath = prefix + "." + k
		}
		if node.Children != nil {
			collectFlatEntries(node.Children, dotPath, out, dotPaths, conflicts)
		} else {
			envKey := strings.ToUpper(strings.ReplaceAll(k, "-", "_"))
			if existing, seen := dotPaths[envKey]; seen && existing != dotPath {
				// check not already recorded
				already := false
				for _, c := range *conflicts {
					if c.EnvKey == envKey {
						already = true
						break
					}
				}
				if !already {
					*conflicts = append(*conflicts, EnvConflict{
						EnvKey:   envKey,
						DotPaths: [2]string{existing, dotPath},
					})
				}
				continue
			}
			dotPaths[envKey] = dotPath
			out[envKey] = EnvEntry{Value: fmt.Sprintf("%v", node.Value), Origin: node.Origin, Overrides: node.Overrides}
		}
	}
}

// ToEnvEntriesFromAnchor is like ToEnvVarsFromAnchor but preserves origin information.
func ToEnvEntriesFromAnchor(tree map[string]*Node, anchorData map[string]interface{}) map[string]EnvEntry {
	result := map[string]EnvEntry{}
	collectEnvEntriesFromData(tree, anchorData, result)
	return result
}

// collectEnvEntriesFromData walks the tree guided by anchorData structure.
// It descends to the anchor's container level (one level above the anchor's deepest content),
// collecting leaf nodes found along the way (e.g. "name" at an intermediate level),
// then collects all leaves from the container level (including inherited ones).
// Leaves not present in the anchor's container data are marked as Overrides=true (inherited).
func collectEnvEntriesFromData(nodes map[string]*Node, anchor map[string]interface{}, out map[string]EnvEntry) {
	collectEnvEntriesDescending(nodes, anchor, "", out)
}

// collectEnvEntriesDescending walks the tree guided by anchorData, collecting leaves at every
// level it passes through. When it reaches the container level (mapCount != 1), it collects
// all remaining leaves from the full subtree at that point.
func collectEnvEntriesDescending(nodes map[string]*Node, anchor map[string]interface{}, prefix string, out map[string]EnvEntry) {
	// Collect any leaf nodes at this level (e.g. "name" alongside a "staging:" map).
	// More specific (deeper) entries overwrite less specific ones, but Overrides=true
	// is preserved if any level in the chain had an override.
	for k, node := range nodes {
		if node.Children != nil {
			continue
		}
		key := strings.ToUpper(k)
		if prefix != "" {
			key = strings.ToUpper(prefix + "_" + k)
		}
		overrides := node.Overrides
		if prev, exists := out[key]; exists && prev.Overrides {
			overrides = true
		}
		out[key] = EnvEntry{Value: fmt.Sprintf("%v", node.Value), Origin: node.Origin, Overrides: overrides}
	}

	// Find the single map key to descend into; if not exactly one, we're at the container level.
	var mapKey string
	mapCount := 0
	for k, v := range anchor {
		if _, ok := v.(map[string]interface{}); ok {
			mapKey = k
			mapCount++
		}
	}
	if mapCount != 1 {
		// At container level — collect all leaves from the full subtree of every map child.
		for k, node := range nodes {
			if node.Children == nil {
				continue // already collected above
			}
			_, inAnchor := anchor[k]
			key := k
			if prefix != "" {
				key = prefix + "_" + k
			}
			collectEnvEntriesWithAnchorScope(node.Children, func() map[string]interface{} {
				if inAnchor {
					if m, ok := anchor[k].(map[string]interface{}); ok {
						return m
					}
				}
				return nil
			}(), key, out)
		}
		return
	}

	// Descend into the single map child, stripping that key from the prefix.
	child, ok := nodes[mapKey]
	if !ok || child.Children == nil {
		return
	}
	collectEnvEntriesDescending(child.Children, anchor[mapKey].(map[string]interface{}), prefix, out)
}

// collectEnvEntriesWithAnchorScope collects all leaves from nodes using node.Overrides directly.
func collectEnvEntriesWithAnchorScope(nodes map[string]*Node, anchorScope map[string]interface{}, prefix string, out map[string]EnvEntry) {
	for k, node := range nodes {
		key := k
		if prefix != "" {
			key = prefix + "_" + k
		}
		if node.Children != nil {
			var childScope map[string]interface{}
			if anchorScope != nil {
				if sv, ok := anchorScope[k]; ok {
					childScope, _ = sv.(map[string]interface{})
				}
			}
			collectEnvEntriesWithAnchorScope(node.Children, childScope, key, out)
		} else {
			out[strings.ToUpper(key)] = EnvEntry{Value: fmt.Sprintf("%v", node.Value), Origin: node.Origin, Overrides: node.Overrides}
		}
	}
}

func collectEnvEntries(nodes map[string]*Node, prefix string, out map[string]EnvEntry) {
	for k, node := range nodes {
		key := k
		if prefix != "" {
			key = prefix + "_" + k
		}
		if node.Children != nil {
			collectEnvEntries(node.Children, key, out)
		} else {
			out[strings.ToUpper(key)] = EnvEntry{Value: fmt.Sprintf("%v", node.Value), Origin: node.Origin, Overrides: node.Overrides}
		}
	}
}

func collectEnvVars(nodes map[string]*Node, prefix string, out map[string]string) {
	for k, node := range nodes {
		key := k
		if prefix != "" {
			key = prefix + "_" + k
		}
		if node.Children != nil {
			collectEnvVars(node.Children, key, out)
		} else {
			out[strings.ToUpper(key)] = fmt.Sprintf("%v", node.Value)
		}
	}
}
