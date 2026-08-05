package secrets

import "strings"

// Scope is a parsed exec/envs/tree argument: an optional vault qualifier plus a
// secret-path within it. An empty Vault means "overlay across every vault".
type Scope struct {
	Vault      string
	SecretPath string
}

// ParseScope splits s into a vault qualifier and a secret-path. The qualifier is
// the text before the first colon, but only when that colon precedes the first
// dot — a colon appearing inside the dotted path is part of the secret-path. A
// leading colon means an empty vault (overlay), same as no colon at all.
func ParseScope(s string) Scope {
	colon := strings.IndexByte(s, ':')
	if colon < 0 {
		return Scope{SecretPath: s}
	}
	dot := strings.IndexByte(s, '.')
	if dot >= 0 && dot < colon {
		return Scope{SecretPath: s}
	}
	return Scope{Vault: s[:colon], SecretPath: s[colon+1:]}
}
