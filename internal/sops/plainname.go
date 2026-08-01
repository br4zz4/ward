package sops

import (
	"path/filepath"
	"strings"
)

// isPlainWardName reports whether path refers to a .plain.ward file.
// A file named exactly "plain.ward" (base == ".plain.ward") is NOT a plain
// file — it is treated as a regular encrypted .ward file. This mirrors the
// contract of secrets.IsPlainFile.
func isPlainWardName(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, ".plain.ward") && base != ".plain.ward"
}
