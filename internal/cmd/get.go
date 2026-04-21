package cmd

import (
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	var anchorPath string

	c := &cobra.Command{
		Use:   "get <dot.path>",
		Short: "Return the merged value at a dot-path (decrypted YAML output)",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := loadConfig()
			if err != nil {
				fatal(err)
			}

			tree, err := loadAndMerge(cfg, anchorPath)
			if err != nil {
				fatal(err)
			}

			node, err := getAtPath(tree, args[0])
			if err != nil {
				fatal(err)
			}

			printTree(node, 0)
		},
	}

	c.Flags().StringVarP(&anchorPath, "anchor", "a", "", "anchor .ward file to scope the merge")
	return c
}
