package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewVersionCmd prints the ward version. It mirrors the output of the cobra
// root's --version flag ("ward version <version>") as an explicit subcommand so
// scripts can call `ward version` without flags.
func NewVersionCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the ward version",
		Args:  cobra.NoArgs,
		Run: func(c *cobra.Command, _ []string) {
			fmt.Fprintf(c.OutOrStdout(), "ward version %s\n", version)
		},
	}
}