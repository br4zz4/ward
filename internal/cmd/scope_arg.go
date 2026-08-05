package cmd

import (
	"fmt"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

// scopeFlags holds the three ways to express a scope on a path command.
type scopeFlags struct {
	scope  string
	vault  string
	secret string
}

// registerScopeFlags adds -s/--scope, --vault and --secret to c and returns the
// binding used by resolveScopeArg.
func registerScopeFlags(c *cobra.Command) *scopeFlags {
	f := &scopeFlags{}
	c.Flags().StringVarP(&f.scope, "scope", "s", "", "scope: vault:secret-path (e.g. commons:infra.KEY)")
	c.Flags().StringVar(&f.vault, "vault", "", "vault name (use with --secret)")
	c.Flags().StringVar(&f.secret, "secret", "", "secret-path within the vault (use with --vault)")
	return f
}

// resolveScopeArg turns whichever form the caller used into a secrets.Scope.
// Precedence: --vault/--secret, then -s/--scope, then the positional. The forms
// are mutually exclusive; mixing them is an error.
func resolveScopeArg(f *scopeFlags, positional []string) (secrets.Scope, error) {
	hasPositional := len(positional) > 0 && positional[0] != ""
	hasScopeFlag := f.scope != ""
	hasVaultSecret := f.vault != "" || f.secret != ""

	forms := 0
	if hasPositional {
		forms++
	}
	if hasScopeFlag {
		forms++
	}
	if hasVaultSecret {
		forms++
	}
	if forms > 1 {
		return secrets.Scope{}, fmt.Errorf("use only one of: positional scope, --scope, or --vault/--secret")
	}

	switch {
	case hasVaultSecret:
		return secrets.Scope{Vault: f.vault, SecretPath: f.secret}, nil
	case hasScopeFlag:
		return secrets.ParseScope(f.scope), nil
	case hasPositional:
		return secrets.ParseScope(positional[0]), nil
	default:
		return secrets.Scope{}, nil
	}
}
