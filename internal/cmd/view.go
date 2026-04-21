package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/oporpino/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view [anchor.ward|dot.path]",
		Short: "Show the merged tree with source file and line for each value",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			anchorPath, dotPath := parseAnchorArg(args)

			result, err := eng.MergeForView(anchorPath)
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

			if dotPath != "" {
				node, err := eng.GetAtPath(result, dotPath)
				if err != nil {
					fatal(err)
				}
				fmt.Println(dotPath)
				printTreeWithOrigin(node, 1, anchorPath, conflictKeys)
			} else {
				printTreeWithOrigin(&secrets.Node{Children: result.Tree}, 0, anchorPath, conflictKeys)
			}
		},
	}
}

// parseAnchorArg classifies a single optional argument as either an anchor path
// (ends in .ward or exists on disk) or a dot-path expression.
func parseAnchorArg(args []string) (anchorPath, dotPath string) {
	if len(args) == 0 {
		return "", ""
	}
	arg := args[0]
	if strings.HasSuffix(arg, ".ward") {
		return arg, ""
	}
	if _, err := os.Stat(arg); err == nil {
		return arg, ""
	}
	return "", arg
}
