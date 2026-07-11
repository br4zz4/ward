package secrets

import (
	"path/filepath"
	"strings"
)

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

// OriginalFilename returns the original filename from a file-secret .ward path.
// Returns "", false for plain .ward files (e.g. "main.ward" has no inner extension).
func OriginalFilename(wardFile string) (string, bool) {
	base := filepath.Base(wardFile)
	without := strings.TrimSuffix(base, ".ward")
	if without == base || !strings.Contains(without, ".") {
		return "", false
	}
	return without, true
}
