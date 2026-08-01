package secrets

import (
	"path/filepath"
	"strings"
)

const plainSuffix = ".plain.ward"

// FileKey converts a filename to a YAML key: "service-account.json" → "service_account_json"
func FileKey(filename string) string {
	base := filepath.Base(filename)
	r := strings.NewReplacer(".", "_", "-", "_")
	return r.Replace(base)
}

// WardFilename appends .ward to the basename: "service-account.json" → "service-account.json.ward"
func WardFilename(filename string) string {
	base := filepath.Base(filename)
	return base + ".ward"
}

// IsPlainFile reports whether path is a .plain.ward file (structured plaintext).
// A bare "plain.ward" (no name before .plain) is not treated as plain.
func IsPlainFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, plainSuffix) && base != plainSuffix
}

// StripWardSuffix returns the basename without its ward suffix.
// For .plain.ward files the whole ".plain.ward" is removed; otherwise ".ward".
// e.g. "config.plain.ward" -> "config", "main.ward" -> "main".
func StripWardSuffix(path string) string {
	base := filepath.Base(path)
	if IsPlainFile(path) {
		return strings.TrimSuffix(base, plainSuffix)
	}
	return strings.TrimSuffix(base, ".ward")
}

// OriginalFilename returns the original filename from a file-secret .ward path.
// Returns "", false for plain .ward files (e.g. "main.ward" has no inner extension).
func OriginalFilename(wardFile string) (string, bool) {
	if IsPlainFile(wardFile) {
		return "", false
	}
	base := filepath.Base(wardFile)
	without := strings.TrimSuffix(base, ".ward")
	if without == base || !strings.Contains(without, ".") {
		return "", false
	}
	return without, true
}
