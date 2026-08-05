package secrets

import (
	"sort"
	"strings"
)

// Scope is a parsed exec/envs/tree argument: an optional vault qualifier plus a
// secret-path within it. An empty Vault means "overlay across every vault".
type Scope struct {
	Vault      string
	SecretPath string
}

// ParseScope splits s into a vault qualifier and a secret-path. The qualifier is
// the text before the first colon, but only when that colon precedes the first
// dot — a colon appearing inside the dotted path is part of the secret-path. A
// leading colon means an empty vault (overlay), same as no colon at all.
func ParseScope(s string) Scope {
	colon := strings.IndexByte(s, ':')
	if colon < 0 {
		return Scope{SecretPath: s}
	}
	dot := strings.IndexByte(s, '.')
	if dot >= 0 && dot < colon {
		return Scope{SecretPath: s}
	}
	return Scope{Vault: s[:colon], SecretPath: s[colon+1:]}
}

// ScopedRoot is a subtree selected by a scope: the dot-path where it sits in the
// merged tree and the node rooted there.
type ScopedRoot struct {
	DotPath string
	Node    *Node
}

// ResolveScopes turns one or more scopes into the set of subtree roots to flatten.
// An empty scope (no vault, no secret-path) selects the whole tree. A qualified
// scope selects exactly one vault's subtree (error if vault or path is unknown).
// An unqualified scope with a secret-path overlays: it selects that secret-path
// under every top-key that has it at depth 1 (error if none do). Multiple scopes
// union their roots, de-duplicated by dot-path.
func ResolveScopes(tree map[string]*Node, scopes []Scope) ([]ScopedRoot, error) {
	if len(scopes) == 0 {
		return []ScopedRoot{{DotPath: "", Node: &Node{Children: tree}}}, nil
	}
	seen := map[string]bool{}
	var out []ScopedRoot
	add := func(dotPath string, n *Node) {
		if seen[dotPath] {
			return
		}
		seen[dotPath] = true
		out = append(out, ScopedRoot{DotPath: dotPath, Node: n})
	}

	for _, sc := range scopes {
		if sc.Vault == "" && sc.SecretPath == "" {
			add("", &Node{Children: tree})
			continue
		}
		if sc.Vault != "" {
			full := sc.Vault
			if sc.SecretPath != "" {
				full = sc.Vault + "." + sc.SecretPath
			}
			node, err := Lookup(tree, full)
			if err != nil {
				return nil, err
			}
			add(full, node)
			continue
		}
		hit := false
		var tried []string
		for _, top := range sortedNodeKeys(tree) {
			tried = append(tried, top)
			full := top + "." + sc.SecretPath
			node, err := Lookup(tree, full)
			if err != nil {
				continue
			}
			add(full, node)
			hit = true
		}
		if !hit {
			return nil, &KeyNotFoundError{
				DotPath:   sc.SecretPath,
				AtPath:    "",
				Available: tried,
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DotPath < out[j].DotPath })
	return out, nil
}
