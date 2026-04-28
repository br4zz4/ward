package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
)

// validateVaultStructure checks every .ward file in every vault:
//  1. The file must be physically inside the vault's directory.
//  2. The YAML key path at the top of the file must exactly match the path
//     derived from the vault name + subdirectories + file stem.
//     e.g. vault "app", file "secrets/test.ward" → must start with app.secrets.test
//
// Returns a list of human-readable violation strings (empty = clean).
func validateVaultStructure(cfg *config.Config, cfgPath string) []string {
	projectRoot, _ := filepath.Abs(filepath.Dir(filepath.Dir(cfgPath)))
	var violations []string

	for _, vault := range cfg.Vaults {
		vaultAbs, err := filepath.Abs(filepath.Join(projectRoot, vault.Path))
		if err != nil {
			violations = append(violations, fmt.Sprintf("vault %q: cannot resolve path %q: %v", vault.Name, vault.Path, err))
			continue
		}

		info, err := os.Stat(vaultAbs)
		if err != nil || !info.IsDir() {
			continue // vault dir not yet created — skip silently
		}

		_ = filepath.WalkDir(vaultAbs, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".ward" {
				return nil
			}

			// Check 1: file must be inside the vault dir
			rel, relErr := filepath.Rel(vaultAbs, path)
			if relErr != nil || strings.HasPrefix(rel, "..") {
				violations = append(violations, fmt.Sprintf(
					"file %q is outside vault %q (%s)", path, vault.Name, vault.Path,
				))
				return nil
			}

			// Check 2: YAML key path must match vault name + subdir segments + file stem
			expected := expectedFileDotPath(vault.Name, vaultAbs, path)
			expectedSegments := strings.Split(expected, ".")

			actual, keyErr := leadingKeyPath(path, len(expectedSegments))
			if keyErr != nil {
				// encrypted or unreadable — skip silently
				return nil
			}
			if actual != expected {
				violations = append(violations, fmt.Sprintf(
					"file %q: key path %q does not match expected %q",
					path, actual, expected,
				))
			}
			return nil
		})
	}
	return violations
}

// enforceVaultStructure loads the config and calls mustValidateStructure.
// Call this from commands that must block on structural violations (exec, inspect, view, envs, get).
// Do NOT call from edit, new, vault — those must remain usable to fix violations.
func enforceVaultStructure() {
	cfgPath, err := resolvedConfigFile()
	if err != nil {
		return // no project — newEngine() will handle the error
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return // config error — newEngine() will handle it
	}
	mustValidateStructure(cfg, cfgPath)
}

// mustValidateStructure runs validateVaultStructure and exits with a styled error if violations are found.
func mustValidateStructure(cfg *config.Config, cfgPath string) {
	violations := validateVaultStructure(cfg, cfgPath)
	if len(violations) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\n  %s✗ vault structure violations%s\n\n", clrLightRed+clrBold, clrReset)
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  %s•%s %s\n", clrLightRed, clrReset, v)
	}
	fmt.Fprintf(os.Stderr, "\n  %suse %sward edit <file>%s to fix the key path%s\n\n", clrGray, clrCyan, clrGray, clrReset)
	os.Exit(1)
}

// expectedFileDotPath builds the dot-path the file's root key should follow.
// e.g. vault "app", file ".ward/vaults/app/secrets/test.ward" → "app.secrets.test"
func expectedFileDotPath(vaultName, vaultAbs, filePath string) string {
	rel, err := filepath.Rel(vaultAbs, filePath)
	if err != nil {
		return vaultName
	}
	rel = strings.TrimSuffix(rel, ".ward")
	parts := strings.Split(rel, string(filepath.Separator))
	segments := append([]string{vaultName}, parts...)
	return strings.Join(segments, ".")
}

// leadingKeyPath reads a plain .ward file and returns the dot-path formed by
// following the first key at each YAML mapping level, up to depth levels deep.
// Returns an error for encrypted files (caller should skip silently).
//
// Example — depth 3, file content:
//
//	app:
//	  secrets:
//	    test:
//	      secret_1: value
//
// → "app.secrets.test"
func leadingKeyPath(path string, depth int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if bytes.HasPrefix(data, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		return "", fmt.Errorf("encrypted")
	}
	// Skip SOPS-encrypted YAML files (contain ENC[...] values and a sops: key).
	if bytes.Contains(data, []byte("ENC[")) {
		return "", fmt.Errorf("encrypted")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Kind == 0 {
		return "", fmt.Errorf("not valid YAML")
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return "", fmt.Errorf("empty document")
	}

	var segments []string
	node := doc.Content[0]
	for i := 0; i < depth; i++ {
		if node.Kind != yaml.MappingNode || len(node.Content) < 2 {
			break
		}
		key := node.Content[0].Value
		segments = append(segments, key)
		node = node.Content[1] // value of first key
	}
	if len(segments) == 0 {
		return "", fmt.Errorf("no keys found")
	}
	return strings.Join(segments, "."), nil
}

// DiscoverForVault returns all .ward files under the given vault path.
func DiscoverForVault(vaultPath string) ([]string, error) {
	return secrets.Discover([]string{vaultPath})
}
