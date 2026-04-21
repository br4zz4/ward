package cmd

import (
	"fmt"

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

			// Build conflict key set for highlighting.
			conflictKeys := map[string]bool{}
			if result.ConflictErr != nil {
				for _, c := range result.ConflictErr.Conflicts {
					conflictKeys[secrets.LeafKey(c.Key)] = true
				}
			}

			if len(args) == 1 {
				node, err := eng.GetAtPath(result, args[0])
				if err != nil {
					fatal(err)
				}
				fmt.Println(args[0])
				printTreeWithOrigin(node, 1, conflictKeys)
			} else {
				printTreeWithOrigin(&secrets.Node{Children: result.Tree}, 0, conflictKeys)
			}
		},
	}
}
