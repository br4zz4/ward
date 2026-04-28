package cmd

import (
	"fmt"
	"os"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:               "get <dot.path>",
		Short:             "Return the merged value at a dot-path",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Fprintf(os.Stderr, "\n  %s✗ missing dot-path%s\n\n", clrLightRed+clrBold, clrReset)
				fmt.Fprintf(os.Stderr, "  usage: %sward get <dot.path>%s\n\n", clrCyan, clrReset)
				fmt.Fprintf(os.Stderr, "  example: %sward get project.staging.secret_key%s\n\n", clrGray, clrReset)
				os.Exit(1)
			}

			dotPath := args[0]

			enforceVaultStructure()
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			result, err := eng.MergeScoped(dotPath)
			if err != nil {
				fatal(err)
			}

			node, err := eng.GetAtPath(result, dotPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "ward:", err)
				os.Exit(1)
			}

			if node.Children == nil {
				// leaf — print raw value
				fmt.Println(node.Value)
				return
			}
			// subtree — print tree
			printTree(&secrets.Node{Children: map[string]*secrets.Node{lastSegment(dotPath): node}}, 0)
		},
	}

	return c
}
