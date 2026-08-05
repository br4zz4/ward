package cmd

import (
	"github.com/spf13/cobra"
)

// NewEnvsCmd is the deprecated alias of `secrets`. It still works but is hidden
// from help and prints a deprecation notice after running.
func NewEnvsCmd() *cobra.Command {
	var prefixed bool

	c := &cobra.Command{
		Use:               "envs [--prefixed] [scope...]",
		Short:             "Deprecated: use 'secrets'",
		Hidden:            true,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			runSecrets(args, prefixed)
			warnDeprecated("envs", "secrets")
		},
	}

	c.Flags().BoolVar(&prefixed, "prefixed", false, "use full path env var names")
	return c
}
