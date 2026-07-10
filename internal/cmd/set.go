package cmd

import (
	"fmt"
	"os"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "set <dot.path> <value>",
		Short:             "Set a single secret at a full dot-path",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			dotPath, value := args[0], args[1]

			requireLeafDepth(dotPath)

			ed := newSecretEditor()
			vault := ed.vaultFor(dotPath)

			// A leaf lives in exactly one file. More than one → Type-1 conflict.
			targets := secrets.FilesMatching(ed.files, dotPath, secrets.IsLeaf)
			ed.abortOnAmbiguity(dotPath, targets)

			targetPath, created := resolveSetTarget(targets, dotPath, vault.Path, ed.cfgPath)

			tree := secrets.NewTree(nil)
			if _, err := os.Stat(targetPath); err == nil {
				tree = ed.load(targetPath)
			}
			tree.Set(dotPath, value)
			ed.save(targetPath, tree)

			reportSet(dotPath, targetPath, created)
			if warn := envCollisionWarning(ed.eng, dotPath); warn != "" {
				fmt.Fprint(os.Stderr, warn)
			}
		},
	}
}

// resolveSetTarget picks the file to write: the sole existing file that defines
// the leaf, or a newly derived path when none does (created=true).
func resolveSetTarget(targets []string, dotPath, vaultPath, cfgPath string) (path string, created bool) {
	if len(targets) == 1 {
		return targets[0], false
	}
	return resolveNewPath(fileStemPath(dotPath), vaultPath, cfgPath), true
}

// reportSet prints the success line and, for a new file, the creation notice.
func reportSet(dotPath, targetPath string, created bool) {
	fmt.Fprintf(os.Stderr, "  %s✓%s set %s%s%s\n", clrGreen, clrReset, clrBold, dotPath, clrReset)
	if created {
		fmt.Fprintf(os.Stderr, "  %s→%s a new file was created: %s%s%s (no existing file defined this path)\n",
			clrGray, clrReset, clrCyan, targetPath, clrReset)
	}
}

// setLeaf sets value at the nested dot-path, creating intermediate maps as needed.
func setLeaf(data map[string]interface{}, dotPath, value string) {
	secrets.NewTree(data).Set(dotPath, value)
}

// resolveTargetFiles returns the files whose data defines the exact leaf dot-path.
func resolveTargetFiles(files []secrets.ParsedFile, dotPath string) []string {
	return secrets.FilesMatching(files, dotPath, secrets.IsLeaf)
}
