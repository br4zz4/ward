package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/br4zz4/ward/internal/cmd"
	"github.com/br4zz4/ward/internal/mcp"
)

var version = "dev"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--mcp" {
			if err := mcp.Serve(); err != nil {
				log.Fatal(err)
			}
			return
		}
	}

	var configPath string

	root := &cobra.Command{
		Use:     "ward",
		Short:   "Hierarchical secrets manager.",
		Long:    "Hierarchical secrets manager.\n\nRun with --mcp to start in MCP server mode (for AI integrations).",
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
