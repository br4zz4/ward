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
	EnvKey        string
	DotPaths      [2]string
	CaseCollision bool // true when keys are the same name but different case
}

// EnvConflictError is returned when flat env var names collide across different dot-paths.
type EnvConflictError struct {
	Conflicts []EnvConflict

	// Cmd is the ward command that surfaced the collision ("exec", "envs",
	// "inspect"). It only shapes the resolution examples. The empty value is
	// treated as "exec" so an un-stamped error still reads sensibly.
	Cmd string
}

func (e *EnvConflictError) Error() string {
	var sb strings.Builder
	n := len(e.Conflicts)
	word := "collision"
	if n > 1 {
		word = "collisions"
	}
	fmt.Fprintf(&sb, "%s%sfound %d env var %s%s — one name, two scopes:\n\n",
		colorBold, colorRed, n, word, colorReset,
	)
	for _, c := range e.Conflicts {
		if c.CaseCollision {
			e.writeCaseCollision(&sb, c)
			continue
		}
		e.writeScopeCollision(&sb, c)
	}
	fmt.Fprintf(&sb, "  %s→ read more:%s https://github.com/br4zz4/ward/blob/main/docs/conflicts-and-collisions.md\n",
		colorGray, colorReset)
	return sb.String()
}

// writeScopeCollision renders one env var defined under two unrelated scopes,
// with copy-pasteable fixes tailored to the command that ran.
func (e *EnvConflictError) writeScopeCollision(sb *strings.Builder, c EnvConflict) {
	scopeA, scopeB := parentDotPath(c.DotPaths[0]), parentDotPath(c.DotPaths[1])

	fmt.Fprintf(sb, "%s✗ %s%s  %s— same env var from two scopes%s\n",
		colorBold+colorPink, c.EnvKey, colorReset, colorGray, colorReset)
	fmt.Fprintf(sb, "    %s%s%s\n", colorYellow, c.DotPaths[0], colorReset)
	fmt.Fprintf(sb, "    %s%s%s\n\n", colorYellow, c.DotPaths[1], colorReset)

	fmt.Fprintf(sb, "  %sfix — pick one:%s\n", colorBold, colorReset)
	fmt.Fprintf(sb, "    %s→ narrow the scope to the one you need:%s\n", colorGray, colorReset)
	fmt.Fprintf(sb, "        %s%s%s\n", colorCyan, e.example(scopeA), colorReset)
	fmt.Fprintf(sb, "        %s%s%s\n", colorCyan, e.example(scopeB), colorReset)
	fmt.Fprintf(sb, "    %s→ keep both, with full-path names:%s\n", colorGray, colorReset)
	fmt.Fprintf(sb, "        %s%s%s\n", colorCyan, e.prefixedExample(), colorReset)
	fmt.Fprintf(sb, "        %s(→ %s and %s)%s\n\n",
		colorGray, prefixedName(c.DotPaths[0]), prefixedName(c.DotPaths[1]), colorReset)
}

// writeCaseCollision renders the case-only collision (same name, different case).
func (e *EnvConflictError) writeCaseCollision(sb *strings.Builder, c EnvConflict) {
	fmt.Fprintf(sb, "%s✗ %s%s  %s— two names differ only in case%s\n",
		colorBold+colorPink, c.EnvKey, colorReset, colorGray, colorReset)
	fmt.Fprintf(sb, "    %s%s%s\n", colorYellow, c.DotPaths[0], colorReset)
	fmt.Fprintf(sb, "    %s%s%s\n\n", colorYellow, c.DotPaths[1], colorReset)
	fmt.Fprintf(sb, "  %sfix:%s rename one so both use consistent casing\n\n", colorBold, colorReset)
}

// commandName returns the command whose examples to show, defaulting to exec.
func (e *EnvConflictError) commandName() string {
	if e.Cmd == "" {
		return "exec"
	}
	return e.Cmd
}

// example renders a scoped invocation for the triggering command.
// exec needs a trailing "-- <cmd>"; envs/inspect take just the scope.
func (e *EnvConflictError) example(scope string) string {
	if e.commandName() == "exec" {
		return fmt.Sprintf("ward exec %s -- <cmd>", scope)
	}
	return fmt.Sprintf("ward %s %s", e.commandName(), scope)
}

// prefixedExample renders the --prefixed invocation for the triggering command.
// inspect has no --prefixed of its own, so it points the user at envs/exec.
func (e *EnvConflictError) prefixedExample() string {
	switch e.commandName() {
	case "exec":
		return "ward exec --prefixed -- <cmd>"
	case "inspect":
		return "ward envs --prefixed   (or: ward exec --prefixed -- <cmd>)"
	default:
		return fmt.Sprintf("ward %s --prefixed", e.commandName())
	}
}

// prefixedName previews the full-path env var name a dot-path becomes under
// --prefixed: the whole dot-path joined by "_", hyphens normalised, case
// preserved as written. Mirrors ToEnvEntries, which walks from the root.
// e.g. "app.staging.token" → "app_staging_token".
func prefixedName(dotPath string) string {
	return strings.ReplaceAll(strings.ReplaceAll(dotPath, ".", "_"), "-", "_")
}

// leafRef is a leaf node with its full dot-path.
type leafRef struct {
	dotPath string
	node    *Node
}

// ToFlatEnvEntries returns only the leaf values as env vars using just the leaf key name
// (uppercased), without any path prefix. Used by ward envs/exec without --prefixed.
//
// Shadow rule: when two dot-paths share the same leaf key name and one is a descendant
// of the other (e.g. app.log_level and app.config.log_level), the deeper one wins
// silently — the shallower is shadowed (dropped). This is NOT a conflict.
//
// Collision: same leaf key name at unrelated dot-paths → EnvConflictError, unless
// preferPrefix is set — then the entry whose dot-path is under preferPrefix wins,
// and all other entries for that env key are discarded. Other (non-colliding) vars
// from the full tree are still included.
func ToFlatEnvEntries(tree map[string]*Node, preferPrefix string) (map[string]EnvEntry, error) {
	byEnvKey := map[string][]leafRef{}
	collectLeafs(tree, "", byEnvKey)
	return flatEntriesFromLeafs(byEnvKey, preferPrefix)
}

// ToFlatEnvEntriesScoped flattens the leaves under the given scoped roots into env
// vars keyed by leaf name (uppercased), applying the same shadow and collision
// rules as ToFlatEnvEntries. Leaves are collected per root into a shared map so
// intermediate sub-maps sharing a name across roots do not overwrite one another.
func ToFlatEnvEntriesScoped(roots []ScopedRoot) (map[string]EnvEntry, error) {
	byEnvKey := map[string][]leafRef{}
	for _, root := range roots {
		if root.Node.Children != nil {
			collectLeafs(root.Node.Children, root.DotPath, byEnvKey)
		} else {
			key := LeafKey(root.DotPath)
			byEnvKey[key] = append(byEnvKey[key], leafRef{root.DotPath, root.Node})
		}
	}
	return flatEntriesFromLeafs(byEnvKey, "")
}

func flatEntriesFromLeafs(byEnvKey map[string][]leafRef, preferPrefix string) (map[string]EnvEntry, error) {
	out := map[string]EnvEntry{}
	var conflicts []EnvConflict

	// detect case-insensitive collisions (e.g. DATABASE_URL vs database_url)
	caseGroups := map[string][]string{}
	for k := range byEnvKey {
		lower := strings.ToLower(k)
		caseGroups[lower] = append(caseGroups[lower], k)
	}
	for _, keys := range caseGroups {
		if len(keys) < 2 {
			continue
		}
		conflicts = append(conflicts, EnvConflict{
			EnvKey:        keys[0],
			DotPaths:      [2]string{keys[0], keys[1]},
			CaseCollision: true,
		})
	}

	for envKey, entries := range byEnvKey {
		if len(entries) == 1 {
			e := entries[0]
			out[envKey] = EnvEntry{Value: fmt.Sprintf("%v", e.node.Value), Origin: e.node.Origin, Overrides: e.node.Overrides}
			continue
		}
		winner, _, isCollision := resolveShadow(entries)
		if isCollision {
			// If preferPrefix uniquely selects one entry, use it to resolve.
			if preferPrefix != "" {
				var matched []leafRef
				for _, e := range entries {
					if strings.HasPrefix(e.dotPath, preferPrefix+".") || e.dotPath == preferPrefix {
						matched = append(matched, e)
					}
				}
				if len(matched) == 1 {
					out[envKey] = EnvEntry{Value: fmt.Sprintf("%v", matched[0].node.Value), Origin: matched[0].node.Origin, Overrides: matched[0].node.Overrides}
					continue
				}
			}
			already := false
			for _, c := range conflicts {
				if c.EnvKey == envKey {
					already = true
					break
				}
			}
			if !already {
				conflicts = append(conflicts, EnvConflict{
					EnvKey:   envKey,
					DotPaths: [2]string{entries[0].dotPath, entries[1].dotPath},
				})
			}
			continue
		}
		out[envKey] = EnvEntry{Value: fmt.Sprintf("%v", winner.node.Value), Origin: winner.node.Origin, Overrides: true}
	}

	if len(conflicts) > 0 {
		return nil, &EnvConflictError{Conflicts: conflicts}
	}
	return out, nil
}

// envKey builds the env var name for a leaf. Case is always preserved as written in the YAML.
// Hyphens are converted to underscores. Nested keys are joined with "_".
func envKey(prefix, k string) string {
	safe := strings.ReplaceAll(k, "-", "_")
	if prefix == "" {
		return safe
	}
	return prefix + "_" + safe
}

// collectLeafs walks the tree and groups all leaf nodes by their leaf key name.
func collectLeafs(nodes map[string]*Node, prefix string, out map[string][]leafRef) {
	for k, node := range nodes {
		dotPath := k
		if prefix != "" {
			dotPath = prefix + "." + k
		}
		if node.Children != nil {
			collectLeafs(node.Children, dotPath, out)
		} else {
			leafKey := strings.ReplaceAll(k, "-", "_")
			out[leafKey] = append(out[leafKey], leafRef{dotPath, node})
		}
	}
}

// resolveShadow determines if one entry shadows others (ancestor relationship) or collides.
// Shadow: deepest dot-path wins when all shallower ones are proper ancestors of it.
// Collision: entries at unrelated dot-paths.
func resolveShadow(entries []leafRef) (winner leafRef, shadowed []leafRef, isCollision bool) {
	// Find the deepest entry by segment count.
	deepest := entries[0]
	for _, e := range entries[1:] {
		if strings.Count(e.dotPath, ".") > strings.Count(deepest.dotPath, ".") {
			deepest = e
		}
	}
	// All others must be proper ancestors: deepest.dotPath must be under their parent.
	for _, e := range entries {
		if e.dotPath == deepest.dotPath {
			continue
		}
		// e's parent path must be a prefix of deepest's parent path (or equal).
		eParent := parentDotPath(e.dotPath)
		deepestParent := parentDotPath(deepest.dotPath)
		if eParent == "" {
			// e is at root level — deepest is always under it
			shadowed = append(shadowed, e)
			continue
		}
		if strings.HasPrefix(deepestParent, eParent) {
			shadowed = append(shadowed, e)
		} else {
			return deepest, nil, true // unrelated — collision
		}
	}
	return deepest, shadowed, false
}

// MarkShadowed walks the tree and sets Overrides=true on any leaf that would be
// shadowed by a deeper leaf with the same key name (same as shadow rule in ToFlatEnvEntries).
// This lets view display shadowed leafs in orange.
func MarkShadowed(tree map[string]*Node) {
	byEnvKey := map[string][]leafRef{}
	collectLeafs(tree, "", byEnvKey)
	for _, entries := range byEnvKey {
		if len(entries) < 2 {
			continue
		}
		_, shadowed, isCollision := resolveShadow(entries)
		if isCollision {
			continue
		}
		for _, s := range shadowed {
			// Navigate to the node and set Overrides=true
			markNodeAt(tree, s.dotPath)
		}
	}
}

// markNodeAt sets Overrides=true on the leaf node at the given dot-path.
func markNodeAt(tree map[string]*Node, dotPath string) {
	parts := strings.Split(dotPath, ".")
	current := tree
	for i, part := range parts {
		node, ok := current[part]
		if !ok {
			return
		}
		if i == len(parts)-1 {
			node.Overrides = true
			return
		}
		if node.Children == nil {
			return
		}
		current = node.Children
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
		key := envKey(prefix, k)
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
		key := envKey(prefix, k)
		if node.Children != nil {
			var childScope map[string]interface{}
			if anchorScope != nil {
				if sv, ok := anchorScope[k]; ok {
					childScope, _ = sv.(map[string]interface{})
				}
			}
			collectEnvEntriesWithAnchorScope(node.Children, childScope, key, out)
		} else {
			out[key] = EnvEntry{Value: fmt.Sprintf("%v", node.Value), Origin: node.Origin, Overrides: node.Overrides}
		}
	}
}

func collectEnvEntries(nodes map[string]*Node, prefix string, out map[string]EnvEntry) {
	for k, node := range nodes {
		key := envKey(prefix, k)
		if node.Children != nil {
			collectEnvEntries(node.Children, key, out)
		} else {
			out[key] = EnvEntry{Value: fmt.Sprintf("%v", node.Value), Origin: node.Origin, Overrides: node.Overrides}
		}
	}
}

func collectEnvVars(nodes map[string]*Node, prefix string, out map[string]string) {
	for k, node := range nodes {
		key := envKey(prefix, k)
		if node.Children != nil {
			collectEnvVars(node.Children, key, out)
		} else {
			out[key] = fmt.Sprintf("%v", node.Value)
		}
	}
}
