// Build a local dev copy on the user's PATH as `ward-dev`, so it can be tested
// side by side with a released `ward` (e.g. one installed via brew) without
// clobbering it. Run with: go generate ./cmd/ward
//
//go:generate sh -c "go build -ldflags \"-X main.version=dev-$(TZ=America/Sao_Paulo date +%Y%m%d-%H%M%S)\" -o \"$HOME/.local/bin/ward-dev\" . && echo 'installed ward-dev -> ~/.local/bin/ward-dev'"

//go:debug x509usefallback=1

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
	var dirPath string
	var mcpMode bool

	root := &cobra.Command{
		Use:     "ward",
		Short:   "Hierarchical secrets manager.",
		Long:    "Hierarchical secrets manager.",
		Version: version,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if err := cmd.ApplyDirFlag(dirPath); err != nil {
				fmt.Fprintln(os.Stderr, "ward:", err)
				os.Exit(1)
			}
			cmd.SetConfigFile(configPath)
		},
	}

	root.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file (default: auto-detect .ward/config.yaml)")
	root.PersistentFlags().StringVarP(&dirPath, "dir", "d", "", "project directory (default: current directory)")
	// --mcp is intercepted before command dispatch (see the os.Args scan above);
	// declaring it here makes it visible in the Flags section of --help.
	root.PersistentFlags().BoolVar(&mcpMode, "mcp", false, "start in MCP server mode (for AI integrations)")

	// Keep commands in registration order (not alphabetical) so each group
	// reads in the order the commands are added below.
	cobra.EnableCommandSorting = false

	// Commands are organised into task-based groups so --help reads by intent:
	// run with secrets, manage individual secrets, manage vault files, and setup.
	const (
		groupRun    = "run"
		groupSecret = "secrets"
		groupVault  = "vaults"
		groupSetup  = "setup"
	)
	root.AddGroup(
		&cobra.Group{ID: groupRun, Title: "Run:"},
		&cobra.Group{ID: groupSecret, Title: "Manage secrets:"},
		&cobra.Group{ID: groupVault, Title: "Manage vaults:"},
		&cobra.Group{ID: groupSetup, Title: "Setup:"},
	)

	// group assigns a GroupID to each command and returns them in order.
	group := func(id string, cmds ...*cobra.Command) []*cobra.Command {
		for _, c := range cmds {
			c.GroupID = id
		}
		return cmds
	}

	var all []*cobra.Command
	all = append(all, group(groupRun,
		cmd.NewExecCmd(),
		cmd.NewEnvsCmd(),
	)...)
	all = append(all, group(groupSecret,
		cmd.NewGetCmd(),
		cmd.NewSetCmd(),
		cmd.NewUnsetCmd(),
		cmd.NewTreeCmd(),
	)...)
	all = append(all, group(groupVault,
		cmd.NewInitCmd(),
		cmd.NewNewCmd(),
		cmd.NewEditCmd(),
		cmd.NewImportCmd(),
		cmd.NewExportCmd(),
		cmd.NewFileCmd(),
		cmd.NewRawCmd(),
		cmd.NewVaultCmd(),
		cmd.NewInspectCmd(),
		cmd.NewRotateKeyCmd(),
	)...)
	all = append(all, group(groupSetup,
		cmd.NewConfigCmd(),
		cmd.NewInstallCmd(),
		cmd.NewUninstallCmd(),
	)...)

	// Deprecated aliases — hidden from help, still functional.
	all = append(all, cmd.NewViewCmd(), cmd.NewOverrideCmd())

	root.AddCommand(all...)

	root.CompletionOptions.DisableDefaultCmd = false

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
