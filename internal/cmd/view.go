package cmd

import (
	"github.com/spf13/cobra"
)

// NewViewCmd is the deprecated alias of `tree`. It still works but is hidden
// from help and prints a deprecation notice after running.
func NewViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "view [dot.path]",
		Short:             "Deprecated: use 'tree'",
		Args:              cobra.MaximumNArgs(1),
		Hidden:            true,
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			runTree(args)
			warnDeprecated("view", "tree")
		},
	}
}
