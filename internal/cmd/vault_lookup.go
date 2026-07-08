package cmd

import (
	"fmt"
	"os"

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

// fatalVaultNotFound prints a styled error and exits when the vault (first
// dot-path segment) does not exist. set/unset never create vaults.
func fatalVaultNotFound(vaultName string) {
	fmt.Fprintf(os.Stderr, "\n  %s✗ vault %q not found%s\n\n", clrLightRed+clrBold, vaultName, clrReset)
	fmt.Fprintf(os.Stderr, "  the first segment of the dot-path must be an existing vault.\n")
	fmt.Fprintf(os.Stderr, "  %s→%s see vaults with  %sward vault list%s\n", clrGray, clrReset, clrCyan, clrReset)
	fmt.Fprintf(os.Stderr, "  %s→%s add one with     %sward vault add %s <path>%s\n\n", clrGray, clrReset, clrCyan, vaultName, clrReset)
	os.Exit(1)
}
