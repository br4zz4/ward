// Build a local dev copy on the user's PATH as `ward-dev`, so it can be tested
// side by side with a released `ward` (e.g. one installed via brew) without
// clobbering it. Run with: go generate ./cmd/ward
//
//go:generate sh -c "go build -o \"$HOME/.local/bin/ward-dev\" . && echo 'installed ward-dev -> ~/.local/bin/ward-dev'"
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
	var mcpMode bool

	root := &cobra.Command{
		Use:     "ward",
		Short:   "Hierarchical secrets manager.",
		Long:    "Hierarchical secrets manager.",
		Version: version,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			cmd.SetConfigFile(configPath)
		},
	}

	root.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file (default: auto-detect .ward/config.yaml)")
	// --mcp is intercepted before command dispatch (see the os.Args scan above);
	// declaring it here makes it visible in the Flags section of --help.
	root.PersistentFlags().BoolVar(&mcpMode, "mcp", false, "start in MCP server mode (for AI integrations)")

	// Keep commands in registration order (not alphabetical) so the primary
	// group reads init → new → tree → envs → exec.
	cobra.EnableCommandSorting = false

	// Primary commands are the day-to-day entry points; the rest are grouped
	// under "Additional Commands" so --help leads with what matters most.
	const primaryGroup = "primary"
	root.AddGroup(&cobra.Group{ID: primaryGroup, Title: "Primary Commands:"})

	initCmd := cmd.NewInitCmd()
	newCmd := cmd.NewNewCmd()
	tree := cmd.NewTreeCmd()
	envs := cmd.NewEnvsCmd()
	exec := cmd.NewExecCmd()
	for _, c := range []*cobra.Command{initCmd, newCmd, tree, envs, exec} {
		c.GroupID = primaryGroup
	}

	root.AddCommand(
		// Primary, in intended order.
		initCmd,
		newCmd,
		tree,
		envs,
		exec,
		// Everything else.
		cmd.NewInstallCmd(),
		cmd.NewUninstallCmd(),
		cmd.NewGetCmd(),
		cmd.NewInspectCmd(),
		cmd.NewEditCmd(),
		cmd.NewConfigCmd(),
		cmd.NewRawCmd(),
		cmd.NewExportCmd(),
		cmd.NewImportCmd(),
		cmd.NewSetCmd(),
		cmd.NewUnsetCmd(),
		cmd.NewVaultCmd(),
		// Deprecated aliases — hidden from help, still functional.
		cmd.NewViewCmd(),
		cmd.NewOverrideCmd(),
	)

	root.CompletionOptions.DisableDefaultCmd = false

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
