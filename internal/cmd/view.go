package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/oporpino/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "view [dot.path]",
		Short:             "Show the merged tree with source file and line for each value",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			result, err := eng.MergeForView()
			if err != nil {
				fatal(err)
			}

			// Build conflict map: dot-path → Conflict (for full source info).
			conflicts := map[string]secrets.Conflict{}
			if result.ConflictErr != nil {
				for _, c := range result.ConflictErr.Conflicts {
					conflicts[c.Key] = c
				}
			}

			if len(args) == 1 {
				node, err := eng.GetAtPath(result, args[0])
				if err != nil {
					fatal(err)
				}
				fmt.Println(args[0])
				printTreeWithOrigin(node, 1, conflicts, args[0])
			} else {
				printTreeWithOrigin(&secrets.Node{Children: result.Tree}, 0, conflicts, "")
			}

			// Warn about env var collisions (only when no dot-path scope is given).
			if len(args) == 0 {
				_, envErr := eng.EnvVars(result, false)
				if envErr != nil {
					var ece *secrets.EnvConflictError
					if errors.As(envErr, &ece) {
						fmt.Fprintf(os.Stderr, "\n%s⚠ env var collisions detected%s — run %sward envs%s to see details\n",
							clrYellow+clrBold, clrReset, clrCyan, clrReset)
					}
				}
			}
		},
	}
}
