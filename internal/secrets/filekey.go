package secrets

import (
	"path/filepath"
	"strings"
)

// FileKey derives the YAML key name from a filename.
// Dots and hyphens in the basename (without path) are replaced with underscores.
// Example: "service-account.json" → "service_account_json"
func FileKey(filename string) string {
	base := filepath.Base(filename)
	r := strings.NewReplacer(".", "_", "-", "_")
	return r.Replace(base)
}

// WardFilename returns the .ward filename for a given original filename.
// Example: "service-account.json" → "service-account.json.ward"
func WardFilename(filename string) string {
	base := filepath.Base(filename)
	return base + ".ward"
}

// OriginalFilename extracts the original filename from a .ward file that was
// created via file import (i.e. has the form "<name>.<ext>.ward").
// Returns the original filename and true, or "", false if it is a plain .ward file.
func OriginalFilename(wardFile string) (string, bool) {
	base := filepath.Base(wardFile)
	without := strings.TrimSuffix(base, ".ward")
	if without == base {
		return "", false
	}
	// A plain .ward file has no remaining extension after stripping .ward.
	// A file-secret has at least one more extension (e.g. ".json").
	if !strings.Contains(without, ".") {
		return "", false
	}
	return without, true
}
