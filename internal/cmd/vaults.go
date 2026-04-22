package cmd

import (
	"fmt"
	"strings"

	"github.com/brazza-tech/ward/internal/config"
	"github.com/spf13/cobra"
)

func NewVaultsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vaults",
		Short: "List configured vault paths",
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
				fmt.Printf("  %s●%s %s%s%s\n", clrGreen, clrReset, clrGrayLight, v.Path, clrReset)
			}

			fmt.Printf("\n  %s%d vault(s) configured in %s%s\n\n",
				clrGray, len(cfg.Vaults), cfgPath, clrReset)
		},
	}
}
