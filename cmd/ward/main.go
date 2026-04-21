package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/oporpino/ward/internal/cmd"
)

func main() {
	var configPath string

	root := &cobra.Command{
		Use:   "ward",
		Short: "Hierarchical secrets management using SOPS+age",
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
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
