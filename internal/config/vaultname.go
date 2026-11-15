package config

import (
	"fmt"
	"strings"
)

const (
	// NamespaceSep separates a namespace from a sub-vault name (myfish::restricted).
	NamespaceSep = "::"
	// DefaultSubVault is the sub-vault a bare namespace refers to.
	DefaultSubVault = "default"
)

// ValidateVaultName checks a vault name: either a plain name or
// <namespace>::<sub> with exactly one level and no lone colon.
func ValidateVaultName(name string) error {
	if name == "" {
		return fmt.Errorf("invalid vault name %q: empty", name)
	}
	ns, sub, nested := strings.Cut(name, NamespaceSep)
	if nested {
		if ns == "" || sub == "" {
			return fmt.Errorf("invalid vault name %q: empty segment around %q", name, NamespaceSep)
		}
		if strings.Contains(sub, NamespaceSep) {
			return fmt.Errorf("invalid vault name %q: only one %q level is allowed", name, NamespaceSep)
		}
		if strings.Contains(ns, ":") || strings.Contains(sub, ":") {
			return fmt.Errorf("invalid vault name %q: ':' is not allowed inside a segment", name)
		}
		return nil
	}
	if strings.Contains(name, ":") {
		return fmt.Errorf("invalid vault name %q: ':' is only allowed as %q", name, NamespaceSep)
	}
	return nil
}

func splitVaultName(name string) (ns, sub string, nested bool) {
	return strings.Cut(name, NamespaceSep)
}

// isDefaultVault reports whether name denotes a default sub-vault (bare
// namespace or <ns>::default).
func isDefaultVault(name string) bool {
	_, sub, nested := splitVaultName(name)
	return !nested || sub == DefaultSubVault
}

// CanonicalVaultName returns the short form of a vault name: x::default → x.
func CanonicalVaultName(name string) string {
	ns, sub, nested := splitVaultName(name)
	if nested && sub == DefaultSubVault {
		return ns
	}
	return name
}

// KeyEnvNames lists the env vars that can carry a vault's key. The default
// sub-vault accepts both WARD_KEY_<NS> and WARD_KEY_<NS>_DEFAULT.
func KeyEnvNames(name string) []string {
	canon := CanonicalVaultName(name)
	if _, _, nested := splitVaultName(canon); nested {
		return []string{KeyEnvName(canon)}
	}
	return []string{KeyEnvName(canon), KeyEnvName(canon + NamespaceSep + DefaultSubVault)}
}

// KeyFileCandidates lists the auto-detected key file paths for a vault.
func KeyFileCandidates(name string) []string {
	canon := CanonicalVaultName(name)
	if _, _, nested := splitVaultName(canon); nested {
		return []string{".ward/" + canon + ".key"}
	}
	return []string{".ward/" + canon + ".key", ".ward/" + canon + NamespaceSep + DefaultSubVault + ".key"}
}

// CanonicalName returns the canonical form of the vault's name.
func (s Source) CanonicalName() string {
	return CanonicalVaultName(s.Name)
}
