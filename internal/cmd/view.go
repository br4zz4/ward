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

			// Detect env var collisions to highlight affected leafs.
			var envCollisions map[string]bool // dot-path → true
			if len(args) == 0 {
				_, envErr := eng.EnvVars(result, false)
				if envErr != nil {
					var ece *secrets.EnvConflictError
					if errors.As(envErr, &ece) {
						envCollisions = make(map[string]bool, len(ece.Conflicts)*2)
						for _, c := range ece.Conflicts {
							envCollisions[c.DotPaths[0]] = true
							envCollisions[c.DotPaths[1]] = true
						}
					}
				}
			}

			if len(args) == 1 {
				node, err := eng.GetAtPath(result, args[0])
				if err != nil {
					fatal(err)
				}
				fmt.Println(args[0])
				printTreeWithOrigin(node, 1, conflicts, args[0], envCollisions)
			} else {
				printTreeWithOrigin(&secrets.Node{Children: result.Tree}, 0, conflicts, "", envCollisions)
			}

			if len(envCollisions) > 0 {
				fmt.Fprintf(os.Stderr, "\n%s⚠ env var collisions detected%s — use %s--prefixed%s or a dot-path to resolve\n",
					clrYellow+clrBold, clrReset, clrCyan, clrReset)
			}
		},
	}
}
