package cmd

import (
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	var anchorPath string

	c := &cobra.Command{
		Use:   "get <dot.path>",
		Short: "Return the merged value at a dot-path",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			result, err := eng.Merge(anchorPath)
			if err != nil {
				fatal(err)
			}
			node, err := eng.GetAtPath(result, args[0])
			if err != nil {
				fatal(err)
			}
			printTree(node, 0)
		},
	}

	c.Flags().StringVarP(&anchorPath, "anchor", "a", "", "anchor .ward file to scope the merge")
	return c
}
