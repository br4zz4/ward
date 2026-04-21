package secrets

// dotPaths returns all dot-paths present in a nested map.
func dotPaths(data map[string]interface{}, prefix string) []string {
	var paths []string
	for k, v := range data {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		paths = append(paths, full)
		if nested, ok := v.(map[string]interface{}); ok {
			paths = append(paths, dotPaths(nested, full)...)
		}
	}
	return paths
}

// isAncestorOf returns true if every map-valued key in candidate exists in anchor
// recursively. Leaf-valued keys in candidate are ignored — an ancestor may declare
// attributes that the anchor doesn't have. What matters is that the anchor covers
// all structural branches the candidate declares.
//
// Example: candidate has company.sectors.one.name (leaf) → ancestor of staging.ward
// Example: candidate has company.production (map) → NOT ancestor of staging.ward
// IsAncestorOf is the exported version of isAncestorOf.
func IsAncestorOf(candidate, anchor ParsedFile) bool {
	return isAncestorOf(candidate, anchor)
}

func isAncestorOf(candidate, anchor ParsedFile) bool {
	return structurallyCompatible(anchor.Data, candidate.Data, true)
}

// structurallyCompatible returns true if the candidate (src) is compatible with the
// anchor (dst) as an ancestor. Rules:
//   - At root: must share at least one common key.
//   - For each map-valued key in src that also exists in dst: recurse to verify
//     compatibility. Branches in src that don't exist in dst are allowed (candidate
//     may cover other environments/sectors the anchor doesn't know about).
//   - dst may not have a leaf where src has a map at the same key (type conflict).
func structurallyCompatible(dst, src map[string]interface{}, atRoot bool) bool {
	sharedAny := false
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			// candidate has a key anchor doesn't — fine, it covers other things
			continue
		}
		sharedAny = true
		srcMap, srcIsMap := sv.(map[string]interface{})
		dstMap, dstIsMap := dv.(map[string]interface{})
		if srcIsMap && !dstIsMap {
			return false // anchor has leaf where candidate has map — incompatible
		}
		if srcIsMap && dstIsMap {
			if !structurallyCompatible(dstMap, srcMap, false) {
				return false
			}
		}
		// src leaf, dst anything — fine
	}

	if atRoot {
		return sharedAny
	}
	return true
}

// FilterByAnchor returns files that share at least one root key with the anchor,
// plus the anchor itself, ordered from least specific to most specific.
// A file is only included as an ancestor if it is strictly less deep than the anchor
// (its map-branch depth is shallower than the anchor's deepest map branch).
func FilterByAnchor(anchor ParsedFile, all []ParsedFile) []ParsedFile {
	anchorDepth := mapDepth(anchor.Data)
	var result []ParsedFile
	for _, f := range all {
		if f.File == anchor.File {
			continue
		}
		if mapDepth(f.Data) < anchorDepth && isAncestorOf(f, anchor) {
			result = append(result, f)
		}
	}
	sortBySpecificity(result)
	result = append(result, anchor)
	return result
}

// MapDepth returns the maximum depth of nested maps in data.
func MapDepth(data map[string]interface{}) int {
	return mapDepth(data)
}

// TrimToScope removes from ancestor's data any map branches that do not exist
// in any of the dir files. This prevents sibling-sector data from leaking into
// an unrelated dir anchor scope.
// Example: company.ward has sectors.one and sectors.two — when used as ancestor
// of a dir anchor for sectors/two, sectors.one should be stripped.
func TrimToScope(ancestor ParsedFile, dirFiles []ParsedFile) ParsedFile {
	trimmed := trimMapToScope(ancestor.Data, dirFiles)
	return ParsedFile{
		File:     ancestor.File,
		Data:     trimmed,
		Lines:    ancestor.Lines,
		RawLines: ancestor.RawLines,
	}
}

// trimMapToScope recursively removes map-valued keys from src that are siblings
// of dir-file keys at the same level (i.e. same parent exists in dir files but
// different child key). Leaves are always preserved for Overrides detection.
// Example: company.ward has sectors.one.name; dir is two → drop sectors.one map
// but keep any leaves at the sectors level (none in this case).
func trimMapToScope(src map[string]interface{}, dirFiles []ParsedFile) map[string]interface{} {
	out := map[string]interface{}{}

	// Collect which map keys exist in dir files at this level
	dirMapKeys := map[string]bool{}
	hasDirMapKeys := false
	for _, df := range dirFiles {
		for k, v := range df.Data {
			if _, isMap := v.(map[string]interface{}); isMap {
				dirMapKeys[k] = true
				hasDirMapKeys = true
			}
		}
	}

	for k, v := range src {
		m, isMap := v.(map[string]interface{})
		if !isMap {
			// Leaf — always keep
			out[k] = v
			continue
		}
		// Map branch: if dir files have map keys at this level, only keep
		// branches that match a dir file key (prune unrelated siblings).
		if hasDirMapKeys && !dirMapKeys[k] {
			continue // prune sibling branch (e.g. sectors.one when dir has sectors.two)
		}
		// Descend with narrowed dir files
		var subDirFiles []ParsedFile
		for _, df := range dirFiles {
			if dv, ok := df.Data[k]; ok {
				if dm, ok := dv.(map[string]interface{}); ok {
					subDirFiles = append(subDirFiles, ParsedFile{
						File: df.File, Data: dm,
						Lines: df.Lines, RawLines: df.RawLines,
					})
				}
			}
		}
		out[k] = trimMapToScope(m, subDirFiles)
	}
	return out
}

func mapDepth(data map[string]interface{}) int {
	max := 0
	for _, v := range data {
		if nested, ok := v.(map[string]interface{}); ok {
			d := 1 + mapDepth(nested)
			if d > max {
				max = d
			}
		}
	}
	return max
}

// SortBySpecificity sorts files from least specific to most specific.
// Specificity = total number of dot-paths across all keys (more = more specific).
func SortBySpecificity(files []ParsedFile) []ParsedFile {
	out := make([]ParsedFile, len(files))
	copy(out, files)
	sortBySpecificity(out)
	return out
}

func sortBySpecificity(files []ParsedFile) {
	for i := 1; i < len(files); i++ {
		for j := i; j > 0 && specificity(files[j]) < specificity(files[j-1]); j-- {
			files[j], files[j-1] = files[j-1], files[j]
		}
	}
}

func specificity(f ParsedFile) int {
	return len(dotPaths(f.Data, ""))
}
