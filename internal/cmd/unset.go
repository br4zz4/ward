package cmd

import (
	"fmt"
	"os"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "unset <dot.path>",
		Short:             "Remove a single secret at a full dot-path",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			dotPath := args[0]

			ed := newSecretEditor()
			ed.vaultFor(dotPath)

			// Match leaf or group so a group path resolves to its file and can
			// report a precise error. More than one → Type-1 conflict.
			targets := secrets.FilesMatching(ed.files, dotPath, secrets.Exists)
			ed.abortOnAmbiguity(dotPath, targets)
			if len(targets) == 0 {
				fatal(ed.keyNotFound(dotPath))
			}

			targetPath := targets[0]
			tree := ed.load(targetPath)

			requireRemovable(tree.Unset(dotPath), dotPath, ed)

			ed.save(targetPath, tree)
			fmt.Fprintf(os.Stderr, "  %s✓%s unset %s%s%s\n", clrGreen, clrReset, clrBold, dotPath, clrReset)
		},
	}
}

// requireRemovable exits with the right error for a non-removal outcome, or
// returns cleanly when a leaf was removed.
func requireRemovable(outcome secrets.UnsetOutcome, dotPath string, ed *secretEditor) {
	switch outcome {
	case secrets.UnsetAbsent:
		fatal(ed.keyNotFound(dotPath))
	case secrets.UnsetGroup:
		fatal(fmt.Errorf("%s is a group, not a leaf — unset removes a single secret, not a whole branch", dotPath))
	}
}

// --- test-facing wrappers ----------------------------------------------------

// unsetResult mirrors secrets.UnsetOutcome for the cmd-level unit tests.
type unsetResult = secrets.UnsetOutcome

const (
	unsetNotFound = secrets.UnsetAbsent
	unsetIsGroup  = secrets.UnsetGroup
	unsetRemoved  = secrets.UnsetDone
)

// unsetLeaf removes a single leaf from data, keeping surrounding scaffold groups.
func unsetLeaf(data map[string]interface{}, dotPath string) unsetResult {
	return secrets.NewTree(data).Unset(dotPath)
}
