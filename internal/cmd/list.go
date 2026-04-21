package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/oporpino/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "list [anchor.ward|dot.path]",
		Short: "Show the merged tree with source file and line for each value",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := loadConfig()
			if err != nil {
				fatal(err)
			}

			anchorPath := ""
			dotPath := ""

			if len(args) == 1 {
				arg := args[0]
				// treat as anchor if it ends in .ward or exists as a file/dir
				if strings.HasSuffix(arg, ".ward") {
					anchorPath = arg
				} else if _, serr := os.Stat(arg); serr == nil {
					anchorPath = arg
				} else {
					dotPath = arg
				}
			}

			tree, err := loadAndMerge(cfg, anchorPath)
			if err != nil {
				fatal(err)
			}

			if dotPath != "" {
				node, err := getAtPath(tree, dotPath)
				if err != nil {
					fatal(err)
				}
				fmt.Println(dotPath)
				printTreeWithOrigin(node, 1, anchorPath)
			} else {
				printTreeWithOrigin(&secrets.Node{Children: tree}, 0, anchorPath)
			}
		},
	}

	return c
}
