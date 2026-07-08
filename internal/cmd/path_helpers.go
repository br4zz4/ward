package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
)

// firstSegment returns the first dot-path segment (the vault name).
func firstSegment(dotPath string) string {
	if i := strings.Index(dotPath, "."); i >= 0 {
		return dotPath[:i]
	}
	return dotPath
}

// requireLeafDepth exits when the dot-path is too shallow to name a leaf secret.
// The vault-structure rule pins each file to vault.[subdirs].stem, so a leaf
// always lives at vault.[subdirs].stem.leafname — at least three segments.
func requireLeafDepth(dotPath string) {
	if strings.Count(dotPath, ".") < 2 {
		fatal(fmt.Errorf("dot-path %q is too shallow — use vault.file.key (at least three segments)", dotPath))
	}
}

// fileStemPath derives the file path (relative to the vault dir, no extension)
// from a full leaf dot-path, honouring the vault structure rule
// vault.[subdirs].stem.leafname. It drops the vault (first) and the leaf (last)
// segments; the remaining segments form the file path.
//
// e.g. "app.services.api.token" → "services/api"
//
//	"app.staging.token"       → "staging"
func fileStemPath(dotPath string) string {
	parts := strings.Split(dotPath, ".")
	fileSegments := parts[1 : len(parts)-1]
	return strings.Join(fileSegments, string(filepath.Separator))
}
