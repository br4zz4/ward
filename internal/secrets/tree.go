package secrets

import "strings"

// NodeKind classifies what lives at a dot-path within a decrypted YAML tree.
type NodeKind int

const (
	// KindAbsent means nothing exists at the path.
	KindAbsent NodeKind = iota
	// KindLeaf means the path holds a scalar value.
	KindLeaf
	// KindGroup means the path holds a nested map (has children).
	KindGroup
)

// Tree is a mutable view over a decrypted YAML document. It owns all dot-path
// traversal so callers never re-implement the "walk maps segment by segment"
// loop. The zero value is not usable; build one with NewTree.
type Tree struct {
	root map[string]interface{}
}

// NewTree wraps an existing decoded YAML map. A nil map is treated as empty.
func NewTree(root map[string]interface{}) *Tree {
	if root == nil {
		root = map[string]interface{}{}
	}
	return &Tree{root: root}
}

// Root returns the underlying map so it can be re-marshalled.
func (t *Tree) Root() map[string]interface{} { return t.root }

// Kind reports what lives at the dot-path.
func (t *Tree) Kind(dotPath string) NodeKind {
	parent, key, ok := t.walkToParent(dotPath, false)
	if !ok {
		return KindAbsent
	}
	v, exists := parent[key]
	if !exists {
		return KindAbsent
	}
	if isGroup(v) {
		return KindGroup
	}
	return KindLeaf
}

// Set writes value at the dot-path, creating intermediate groups as needed.
func (t *Tree) Set(dotPath, value string) {
	parent, key, _ := t.walkToParent(dotPath, true)
	parent[key] = value
}

// UnsetOutcome is the result of removing a leaf.
type UnsetOutcome int

const (
	// UnsetAbsent means there was nothing to remove.
	UnsetAbsent UnsetOutcome = iota
	// UnsetGroup means the path is a group; nothing is removed.
	UnsetGroup
	// UnsetDone means a leaf was removed.
	UnsetDone
)

// unsetForKind translates what lives at a path into the removal outcome.
var unsetForKind = map[NodeKind]UnsetOutcome{
	KindAbsent: UnsetAbsent,
	KindGroup:  UnsetGroup,
	KindLeaf:   UnsetDone,
}

// Unset removes the leaf at the dot-path, leaving surrounding scaffold groups in
// place. It never removes a whole branch: a group path yields UnsetGroup.
func (t *Tree) Unset(dotPath string) UnsetOutcome {
	outcome := unsetForKind[t.Kind(dotPath)]
	if outcome == UnsetDone {
		parent, key, _ := t.walkToParent(dotPath, false)
		delete(parent, key)
	}
	return outcome
}

// walkToParent descends to the map that directly holds the final segment,
// returning that map and the final key. When create is true, missing
// intermediate groups are created (and any scalar in the way is replaced by a
// group). When create is false and the path breaks, ok is false.
func (t *Tree) walkToParent(dotPath string, create bool) (parent map[string]interface{}, key string, ok bool) {
	parts := strings.Split(dotPath, ".")
	current := t.root
	for _, seg := range parts[:len(parts)-1] {
		next, isMap := current[seg].(map[string]interface{})
		if !isMap {
			if !create {
				return nil, "", false
			}
			next = map[string]interface{}{}
			current[seg] = next
		}
		current = next
	}
	return current, parts[len(parts)-1], true
}

// isGroup reports whether v is a nested map (a group), not a scalar leaf.
func isGroup(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}
