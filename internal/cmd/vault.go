package cmd

import (
	"fmt"
	"strings"

	"github.com/br4zz4/ward/internal/config"
	"github.com/spf13/cobra"
)

func NewVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage vaults",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newVaultListCmd())
	cmd.AddCommand(newVaultAddCmd())
	return cmd
}

func newVaultListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured vaults",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(err)
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}
			if len(cfg.Vaults) == 0 {
				fmt.Printf("\n  %s✗ no vaults configured%s — add a vault to %s%s%s\n\n",
					clrLightRed, clrReset, clrCyan, cfgPath, clrReset)
				return
			}
			ruler := func(label string) {
				dashes := 52 - len(label) - 1
				fmt.Printf("\n  %s%s%s%s%s\n",
					clrCyan+clrBold, label, clrReset+clrGray, strings.Repeat("─", dashes), clrReset)
			}
			ruler("VAULTS")
			for _, v := range cfg.Vaults {
				fmt.Printf("  %s●%s %s%s%s  %s%s%s\n",
					clrGreen, clrReset,
					clrCyan, v.Name, clrReset,
					clrGrayLight, v.Path, clrReset)
			}
			fmt.Printf("\n  %s%d vault(s) configured in %s%s\n\n",
				clrGray, len(cfg.Vaults), cfgPath, clrReset)
		},
	}
}

func newVaultAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <path>",
		Short: "Register a new vault",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			name, vaultPath := args[0], args[1]

			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(fmt.Errorf("no ward project found — run `ward init` first"))
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}

			for _, v := range cfg.Vaults {
				if v.Name == name {
					fatal(fmt.Errorf("vault %q already exists", name))
				}
				if strings.TrimSuffix(v.Path, "/") == strings.TrimSuffix(vaultPath, "/") {
					fatal(fmt.Errorf("path %q is already used by vault %q", vaultPath, v.Name))
				}
			}

			cfg.Vaults = append(cfg.Vaults, config.Source{Name: name, Path: vaultPath})
			if err := config.Save(cfgPath, cfg); err != nil {
				fatal(fmt.Errorf("updating %s: %w", cfgPath, err))
			}
			fmt.Printf("\n  %s✓%s vault %s%s%s added (%s%s%s)\n\n",
				clrGreen, clrReset,
				clrCyan, name, clrReset,
				clrGrayLight, vaultPath, clrReset)
		},
	}
}
