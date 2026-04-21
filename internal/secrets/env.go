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
// then collects all leaves from the tree at that point (including inherited ones).
// Leaves not present in the anchor's container data are marked as Overrides=true (inherited).
func collectEnvEntriesFromData(nodes map[string]*Node, anchor map[string]interface{}, out map[string]EnvEntry) {
	depth := anchorMapDepth(anchor)
	container, containerAnchor := descendToContainerWithAnchor(nodes, anchor, depth)
	collectEnvEntriesWithAnchorScope(container, containerAnchor, "", out)
}

// anchorMapDepth returns the depth of the deepest map chain in anchor data.
func anchorMapDepth(anchor map[string]interface{}) int {
	max := 0
	for _, v := range anchor {
		if m, ok := v.(map[string]interface{}); ok {
			d := 1 + anchorMapDepth(m)
			if d > max {
				max = d
			}
		}
	}
	return max
}

// descendToContainerWithAnchor descends through the tree following the anchor
// structure until it reaches the deepest map level — the container whose
// children are the actual secret values. It stops one level above the leaves
// of the anchor's deepest branch.
func descendToContainerWithAnchor(nodes map[string]*Node, anchor map[string]interface{}, depth int) (map[string]*Node, map[string]interface{}) {
	// Find the single map-valued key at this level to descend into.
	// If there are zero or multiple map keys we've reached the container level.
	var mapKey string
	mapCount := 0
	for k, v := range anchor {
		if _, ok := v.(map[string]interface{}); ok {
			mapKey = k
			mapCount++
		}
	}
	if mapCount != 1 {
		return nodes, anchor
	}
	child, ok := nodes[mapKey]
	if !ok || child.Children == nil {
		return nodes, anchor
	}
	return descendToContainerWithAnchor(child.Children, anchor[mapKey].(map[string]interface{}), depth-1)
}

// collectEnvEntriesWithAnchorScope collects all leaves from nodes.
// Keys not present in anchorScope are marked Overrides=true (inherited from ancestor).
func collectEnvEntriesWithAnchorScope(nodes map[string]*Node, anchorScope map[string]interface{}, prefix string, out map[string]EnvEntry) {
	for k, node := range nodes {
		key := k
		if prefix != "" {
			key = prefix + "_" + k
		}
		inAnchor := anchorScope != nil
		if inAnchor {
			_, inAnchor = anchorScope[k]
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
			e := EnvEntry{Value: fmt.Sprintf("%v", node.Value), Origin: node.Origin, Overrides: node.Overrides}
			if !inAnchor {
				e.Overrides = true // inherited from ancestor, not defined in anchor
			}
			out[strings.ToUpper(key)] = e
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
