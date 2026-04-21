package cmd

import (
	"fmt"
	"os"

	"github.com/oporpino/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:               "get [dot.path]",
		Short:             "Return the merged value at a dot-path",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			result, err := eng.Merge()
			if err != nil {
				fatal(err)
			}

			if len(args) == 0 {
				// No dot-path: print entire tree
				root := &secrets.Node{Children: result.Tree}
				printTree(root, 0)
				return
			}

			node, err := eng.GetAtPath(result, args[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, "ward:", err)
				os.Exit(1)
			}
			printTree(node, 0)
		},
	}

	return c
}
