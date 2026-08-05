package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewSetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:               "set [scope] <value>",
		Short:             "Set a single secret at a scope",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeDotPaths,
	}
	sf := registerScopeFlags(c)
	c.Run = func(_ *cobra.Command, args []string) {
		scopePositional, value := splitSetArgs(sf, args)

		sc, err := resolveScopeArg(sf, scopePositional)
		if err != nil {
			fatal(err)
		}

		dotPath := sc.TreePath()
		requireLeafDepth(dotPath)

		ed := newSecretEditor()
		vault := ed.vaultForScope(sc)

		// A leaf lives in exactly one file. More than one → Type-1 conflict.
		targets := secrets.FilesMatching(ed.files, dotPath, secrets.IsLeaf)
		ed.abortOnAmbiguity(dotPath, targets)

		if setPathIsGroup(ed.files, dotPath) {
			fatal(groupPathError(ed.files, dotPath))
		}

		targetPath, created := resolveSetTarget(targets, dotPath, vault.Path, ed.cfgPath)

		tree := secrets.NewTree(nil)
		if _, err := os.Stat(targetPath); err == nil {
			tree = ed.load(targetPath)
		}
		tree.Set(dotPath, value)
		ed.save(targetPath, tree)

		reportSet(sc.FullPath(), targetPath, created)
		if warn := envCollisionWarning(ed.eng, dotPath); warn != "" {
			fmt.Fprint(os.Stderr, warn)
		}
	}
	return c
}

// splitSetArgs separates the positional scope (if any) from the value argument.
// When a scope flag was used the value is the sole positional; otherwise the
// scope is the first positional and the value the second.
func splitSetArgs(sf *scopeFlags, args []string) (scopePositional []string, value string) {
	usedFlags := sf.scope != "" || sf.vault != "" || sf.secret != ""
	if usedFlags {
		if len(args) != 1 {
			fatal(fmt.Errorf("set with a scope flag takes exactly one argument: the value"))
		}
		return nil, args[0]
	}
	if len(args) != 2 {
		fatal(fmt.Errorf("usage: set <scope> <value> (e.g. set commons:infra.KEY value)"))
	}
	return []string{args[0]}, args[1]
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

// setPathIsGroup reports whether dotPath resolves to a group (has children) in any loaded file.
func setPathIsGroup(files []secrets.ParsedFile, dotPath string) bool {
	for _, pf := range files {
		if secrets.NewTree(pf.Data).Kind(dotPath) == secrets.KindGroup {
			return true
		}
	}
	return false
}

// groupPathError builds the error message listing the child keys that would be lost.
func groupPathError(files []secrets.ParsedFile, dotPath string) error {
	var children []string
	seen := map[string]bool{}
	for _, pf := range files {
		node := secrets.NewTree(pf.Data)
		if node.Kind(dotPath) != secrets.KindGroup {
			continue
		}
		for _, child := range node.Children(dotPath) {
			if !seen[child] {
				seen[child] = true
				children = append(children, child)
			}
		}
	}
	sort.Strings(children)
	return fmt.Errorf("%s is a group — setting it would overwrite and lose its keys: %s",
		dotPath, strings.Join(children, ", "))
}

// resolveTargetFiles returns the files whose data defines the exact leaf dot-path.
func resolveTargetFiles(files []secrets.ParsedFile, dotPath string) []string {
	return secrets.FilesMatching(files, dotPath, secrets.IsLeaf)
}
