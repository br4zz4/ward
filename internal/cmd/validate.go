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
//  2. The first YAML root key must equal the vault's name.
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

		// walk the vault directory
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

			// Check 2: first root YAML key must equal vault name
			rootKey, keyErr := firstRootKey(path)
			if keyErr != nil {
				violations = append(violations, fmt.Sprintf(
					"file %q: cannot read YAML: %v", path, keyErr,
				))
				return nil
			}
			if rootKey != "" && rootKey != vault.Name {
				violations = append(violations, fmt.Sprintf(
					"file %q: root key %q does not match vault name %q", path, rootKey, vault.Name,
				))
			}
			return nil
		})
	}
	return violations
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
	fmt.Fprintf(os.Stderr, "\n  %sfix the violations above before running this command%s\n\n", clrGray, clrReset)
	os.Exit(1)
}

// firstRootKey reads a .ward file (which may be encrypted or plain YAML) and
// returns the first root key. Ward files that are age-encrypted will return ""
// (encrypted — skip root-key validation). For plain YAML files, it parses and
// returns the first mapping key.
func firstRootKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if bytes.HasPrefix(data, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		return "", nil // encrypted — skip root-key validation
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Kind == 0 {
		return "", fmt.Errorf("not valid YAML")
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		mapping := doc.Content[0]
		if mapping.Kind == yaml.MappingNode && len(mapping.Content) > 0 {
			return mapping.Content[0].Value, nil
		}
	}
	return "", fmt.Errorf("no root key found")
}

// DiscoverForVault returns all .ward files under the given vault path.
func DiscoverForVault(vaultPath string) ([]string, error) {
	return secrets.Discover([]string{vaultPath})
}
