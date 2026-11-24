package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewSetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "set [scope] <value|@file|-> | set [scope] --from <source-scope>",
		Short: "Set a single secret at a scope",
		Long: `Set a single secret at a scope.

The value is taken from:
  <value>      the argument itself
  @<path>      the contents of a file (relative paths resolve against the directory ward was run from)
  -            standard input (must be piped)
  @@<value>    the literal "@<value>"

Content read from a file or stdin loses one trailing newline; it must be
non-empty, valid UTF-8 and at most 1 MiB. Use --literal to store the argument
exactly as given (including values starting with "@" or equal to "-").

With --from <source-scope> the value is copied from another stored secret (a
leaf, possibly in another vault) without being printed; it cannot be combined
with a value or --literal.`,
		Args: cobra.ArbitraryArgs,
		ValidArgsFunction: func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return completeDotPaths(c, args, toComplete)
		},
	}
	sf := registerScopeFlags(c)
	var literal bool
	c.Flags().BoolVar(&literal, "literal", false, `store the value as-is; do not read @file or - (stdin)`)
	var from string
	c.Flags().StringVar(&from, "from", "", "copy the stored value of another secret (a leaf) instead of taking a value")
	c.Run = func(_ *cobra.Command, args []string) {
		var scopePositional []string
		var value, source string
		if from != "" {
			if literal {
				fatal(fmt.Errorf("--from cannot be combined with --literal"))
			}
			scopePositional = splitSetFromArgs(sf, args)
		} else {
			var rawValue string
			var err error
			scopePositional, rawValue = splitSetArgs(sf, args)
			value, source, err = resolveSetValue(rawValue, literal, os.Stdin, term.IsTerminal(int(os.Stdin.Fd())))
			if err != nil {
				fatal(err)
			}
		}

		sc, err := resolveScopeArg(sf, scopePositional)
		if err != nil {
			fatal(err)
		}

		dotPath := sc.TreePath()
		requireLeafDepth(dotPath)

		ed := newSecretEditor()
		vault := ed.vaultForScope(sc)

		if from != "" {
			destFull := dotPath
			if sc.Vault == "" {
				destFull = vault.CanonicalName() + "." + dotPath
			}
			value, source = copySourceValue(ed, from, destFull)
		}

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

		reportSet(sc.FullPath(), targetPath, created, source)
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
		fatal(fmt.Errorf("usage: set <scope> <value> (e.g. set vault1:group.key1 value1)"))
	}
	return []string{args[0]}, args[1]
}

// splitSetFromArgs returns the positional destination scope of `set --from`:
// the destination is the sole positional, or none when a scope flag is used.
func splitSetFromArgs(sf *scopeFlags, args []string) []string {
	usedFlags := sf.scope != "" || sf.vault != "" || sf.secret != ""
	if usedFlags {
		if len(args) != 0 {
			fatal(fmt.Errorf("set --from takes no value; the source is the --from scope"))
		}
		return nil
	}
	if len(args) != 1 {
		fatal(fmt.Errorf("usage: set <destination-scope> --from <source-scope>"))
	}
	return args
}

// copySourceValue resolves the --from source to the stored value of a single
// leaf, refusing a missing or non-leaf source and a source equal to the
// destination. It never prints the value.
func copySourceValue(ed *secretEditor, from, destFull string) (value, source string) {
	src := secrets.ParseScope(from)
	if src.Vault == "" && src.SecretPath == "" {
		fatal(fmt.Errorf("--from needs a source scope"))
	}
	if src.Vault != "" && findVault(ed.cfg, src.Vault) == nil {
		fatalVaultNotFound(src.Vault)
	}
	result, err := ed.eng.MergeScoped(src.TreePath())
	if err != nil {
		fatal(err)
	}
	srcPath, value, err := lookupLeaf(ed.eng, result, src)
	if err != nil {
		fatal(err)
	}
	if srcPath == destFull {
		fatal(fmt.Errorf("source and destination are the same: %s", src.FullPath()))
	}
	vaultName, secretPath, _ := strings.Cut(srcPath, ".")
	return value, "secret " + vaultName + ":" + secretPath
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
func reportSet(dotPath, targetPath string, created bool, source string) {
	origin := ""
	if source != "" {
		origin = fmt.Sprintf(" %s(from %s)%s", clrGray, source, clrReset)
	}
	fmt.Fprintf(os.Stderr, "  %s✓%s set %s%s%s%s\n", clrGreen, clrReset, clrBold, dotPath, clrReset, origin)
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
