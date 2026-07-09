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

	// Primary commands are the day-to-day entry points; the rest are grouped
	// under "Additional Commands" so --help leads with what matters most.
	const primaryGroup = "primary"
	root.AddGroup(&cobra.Group{ID: primaryGroup, Title: "Primary Commands:"})

	exec := cmd.NewExecCmd()
	envs := cmd.NewEnvsCmd()
	exec.GroupID = primaryGroup
	envs.GroupID = primaryGroup

	root.AddCommand(
		exec,
		envs,
		cmd.NewInstallCmd(),
		cmd.NewUninstallCmd(),
		cmd.NewGetCmd(),
		cmd.NewViewCmd(),
		cmd.NewInspectCmd(),
		cmd.NewInitCmd(),
		cmd.NewEditCmd(),
		cmd.NewNewCmd(),
		cmd.NewConfigCmd(),
		cmd.NewRawCmd(),
		cmd.NewExportCmd(),
		cmd.NewOverrideCmd(),
		cmd.NewSetCmd(),
		cmd.NewUnsetCmd(),
		cmd.NewVaultCmd(),
	)

	root.CompletionOptions.DisableDefaultCmd = false

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
