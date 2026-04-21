package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/oporpino/ward/internal/cmd"
)

var version = "dev"

func main() {
	var configPath string

	root := &cobra.Command{
		Use:     "ward",
		Short:   "Hierarchical secrets management using SOPS+age",
		Version: version,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			cmd.SetConfigFile(configPath)
		},
	}

	root.PersistentFlags().StringVarP(&configPath, "config", "c", "ward.yaml", "config file")

	root.AddCommand(
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
	)

	root.CompletionOptions.DisableDefaultCmd = false

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
