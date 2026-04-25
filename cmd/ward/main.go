package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/br4zz4/ward/internal/cmd"
)

var version = "dev"

func main() {
	var configPath string

	root := &cobra.Command{
		Use:     "ward",
		Short:   "Hierarchical secrets manager.",
		Version: version,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			cmd.SetConfigFile(configPath)
		},
	}

	root.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file (default: auto-detect .ward/config.yaml)")

	root.AddCommand(
		cmd.NewInstallCmd(),
		cmd.NewUninstallCmd(),
		cmd.NewGetCmd(),
		cmd.NewViewCmd(),
		cmd.NewEnvsCmd(),
		cmd.NewInspectCmd(),
		cmd.NewExecCmd(),
		cmd.NewInitCmd(),
		cmd.NewEditCmd(),
		cmd.NewNewCmd(),
		cmd.NewConfigCmd(),
		cmd.NewRawCmd(),
		cmd.NewExportCmd(),
		cmd.NewOverrideCmd(),
		cmd.NewVaultsCmd(),
	)

	root.CompletionOptions.DisableDefaultCmd = false

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
