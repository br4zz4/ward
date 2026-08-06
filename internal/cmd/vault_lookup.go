package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/br4zz4/ward/internal/config"
)

// findVault returns the config source matching name, or nil.
func findVault(cfg *config.Config, name string) *config.Source {
	for i := range cfg.Vaults {
		if cfg.Vaults[i].Name == name {
			return &cfg.Vaults[i]
		}
	}
	return nil
}

// fatalVaultNotFound prints a styled error and exits when the named vault does
// not exist. Commands never create vaults implicitly.
func fatalVaultNotFound(vaultName string) {
	fmt.Fprint(os.Stderr, vaultNotFoundMessage(vaultName))
	os.Exit(1)
}

// vaultNotFoundMessage builds the styled "vault not found" text. The vault is
// always named explicitly — by a `vault:` prefix, --vault, or its own argument
// — so the message never refers to dot-path segments.
func vaultNotFoundMessage(vaultName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s✗ vault %q not found%s\n\n", clrLightRed+clrBold, vaultName, clrReset)
	fmt.Fprintf(&b, "  no vault with that name is configured.\n")
	fmt.Fprintf(&b, "  %s→%s see vaults with  %sward vault list%s\n", clrGray, clrReset, clrCyan, clrReset)
	fmt.Fprintf(&b, "  %s→%s add one with     %sward vault add %s <path>%s\n\n", clrGray, clrReset, clrCyan, vaultName, clrReset)
	return b.String()
}
