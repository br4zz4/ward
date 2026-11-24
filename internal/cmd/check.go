package cmd

import (
	"crypto/subtle"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	exitCheckMatch   = 0
	exitCheckDiffers = 1
	exitCheckError   = 2
)

// NewCheckCmd builds `ward check`: it tells whether a value held outside ward
// equals the stored secret, without printing either value.
func NewCheckCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "check [scope] <value|@file|->",
		Short: "Check whether a value equals the stored secret (match or differs)",
		Long: `Check whether a value equals the secret stored at a scope, without printing either.

The value is taken from the same sources as 'ward set':
  <value>      the argument itself
  @<path>      the contents of a file
  -            standard input (must be piped)
  @@<value>    the literal "@<value>"

Prints "match" (exit 0) or "differs" (exit 1). Any error (missing scope, a
subtree, an unreadable value source, not a ward project) exits 2 with the reason
on stderr and nothing on stdout. Works in AI mode: it reveals only equality.`,
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
	c.Flags().BoolVar(&literal, "literal", false, `compare the value as-is; do not read @file or - (stdin)`)
	c.Run = func(_ *cobra.Command, args []string) {
		errorExitCode = exitCheckError
		scopePositional, rawValue := splitCheckArgs(sf, args)
		value, _, err := resolveSetValue(rawValue, literal, os.Stdin, term.IsTerminal(int(os.Stdin.Fd())))
		if err != nil {
			fatal(err)
		}

		sc, err := resolveScopeArg(sf, scopePositional)
		if err != nil {
			fatal(err)
		}
		if sc.Vault == "" && sc.SecretPath == "" {
			fatal(fmt.Errorf("missing scope (usage: ward check <scope> <value|@file|->)"))
		}
		if sc.Vault != "" {
			src, err := vaultNamed(sc.Vault)
			if err != nil {
				fatal(err)
			}
			if src == nil {
				fatalVaultNotFound(sc.Vault)
			}
		}

		enforceVaultStructure()
		eng, err := newEngine()
		if err != nil {
			fatal(err)
		}
		result, err := eng.MergeScoped(sc.TreePath())
		if err != nil {
			fatal(err)
		}
		_, stored, err := lookupLeaf(eng, result, sc)
		if err != nil {
			fatal(err)
		}

		if subtle.ConstantTimeCompare([]byte(stored), []byte(value)) == 1 {
			fmt.Println("match")
			os.Exit(exitCheckMatch)
		}
		fmt.Println("differs")
		os.Exit(exitCheckDiffers)
	}
	return c
}

func splitCheckArgs(sf *scopeFlags, args []string) (scopePositional []string, value string) {
	usedFlags := sf.scope != "" || sf.vault != "" || sf.secret != ""
	if usedFlags {
		if len(args) != 1 {
			fatal(fmt.Errorf("check with a scope flag takes exactly one argument: the value"))
		}
		return nil, args[0]
	}
	if len(args) != 2 {
		fatal(fmt.Errorf("usage: check <scope> <value|@file|-> (e.g. check vault1:group.key1 @token.txt)"))
	}
	return []string{args[0]}, args[1]
}
