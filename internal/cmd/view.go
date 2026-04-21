package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/oporpino/ward/internal/config"
	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/sops"
	"github.com/spf13/cobra"
)

func NewViewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "view [anchor.ward|dot.path]",
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
				if strings.HasSuffix(arg, ".ward") {
					anchorPath = arg
				} else if _, serr := os.Stat(arg); serr == nil {
					anchorPath = arg
				} else {
					dotPath = arg
				}
			}

			// Detect conflicts without blocking — collect conflicting leaf key names
			conflictKeys := map[string]bool{}
			dec := sops.MockDecryptor{}
			paths, _ := secrets.Discover(sourcePaths(cfg))
			files, _ := secrets.LoadAll(paths, dec)
			ordered := buildOrderedFiles(cfg, anchorPath, files)
			if _, cerr := secrets.Merge(ordered, config.MergeModeError); cerr != nil {
				if ce, ok := cerr.(*secrets.ConflictError); ok {
					for _, c := range ce.Conflicts {
						conflictKeys[secrets.LeafKey(c.Key)] = true
					}
				}
			}

			// Merge with override so tree is always complete
			tree, err := loadAndMergeWithMode(cfg, anchorPath, files, config.MergeModeOverride)
			if err != nil {
				fatal(err)
			}

			if dotPath != "" {
				node, err := getAtPath(tree, dotPath)
				if err != nil {
					fatal(err)
				}
				fmt.Println(dotPath)
				printTreeWithOrigin(node, 1, anchorPath, conflictKeys)
			} else {
				printTreeWithOrigin(&secrets.Node{Children: tree}, 0, anchorPath, conflictKeys)
			}
		},
	}

	return c
}
