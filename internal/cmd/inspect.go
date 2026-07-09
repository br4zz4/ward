package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "inspect [dot.path]",
		Short:             "Detect conflicts and env var collisions across all files",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			dotPath := ""
			if len(args) == 1 {
				dotPath = args[0]
			}

			enforceVaultStructure()
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			err = eng.InspectScoped(dotPath)
			if err == nil {
				fmt.Printf("%s✓%s no conflicts found\n", clrGreen, clrReset)
				return
			}
			reportInspectError(stampEnvCommand(err, "inspect"))
		},
	}
}

// reportInspectError prints a conflict (Type-1) or env var collision (Type-2)
// using its own styled message, then exits non-zero. Any other error is fatal.
func reportInspectError(err error) {
	switch err.(type) {
	case *secrets.ConflictError, *secrets.EnvConflictError:
		lines := strings.SplitN(err.Error(), "\n", 2)
		fmt.Fprintf(os.Stderr, "%s%s%s\n", clrLightRed, lines[0], clrReset)
		if len(lines) > 1 {
			fmt.Fprintln(os.Stderr, lines[1])
		}
		os.Exit(1)
	default:
		fatal(err)
	}
}
