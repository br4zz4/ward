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
	// Collect any leaf nodes at this level (e.g. "name" alongside a "staging:" map)
	for k, node := range nodes {
		if node.Children != nil {
			continue
		}
		e := EnvEntry{Value: fmt.Sprintf("%v", node.Value), Origin: node.Origin, Overrides: node.Overrides}
		key := k
		if prefix != "" {
			key = prefix + "_" + k
		}
		out[strings.ToUpper(key)] = e
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
