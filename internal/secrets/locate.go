package secrets

// FilesMatching returns the files whose decoded data satisfies match at dotPath.
// It centralises the "scan every parsed file for a dot-path" loop so callers
// only supply the acceptance rule as a predicate over the node kind.
func FilesMatching(files []ParsedFile, dotPath string, match func(NodeKind) bool) []string {
	var out []string
	for _, pf := range files {
		if match(NewTree(pf.Data).Kind(dotPath)) {
			out = append(out, pf.File)
		}
	}
	return out
}

// IsLeaf accepts only a scalar leaf. Used to locate where a secret is defined.
func IsLeaf(k NodeKind) bool { return k == KindLeaf }

// Exists accepts any present node (leaf or group).
func Exists(k NodeKind) bool { return k != KindAbsent }
