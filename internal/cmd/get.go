package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/ward"
	"github.com/spf13/cobra"
)

func NewGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:               "get [scope]",
		Short:             "Return the merged value at a scope",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeDotPaths,
	}
	sf := registerScopeFlags(c)
	c.Run = func(_ *cobra.Command, args []string) {
		sc, err := resolveScopeArg(sf, args)
		if err != nil {
			fatal(err)
		}
		if sc.Vault == "" && sc.SecretPath == "" {
			fmt.Fprintf(os.Stderr, "\n  %s✗ missing scope%s\n\n", clrLightRed+clrBold, clrReset)
			fmt.Fprintf(os.Stderr, "  usage: %sward get <scope>%s\n\n", clrCyan, clrReset)
			fmt.Fprintf(os.Stderr, "  example: %sward get vault1:group.key1%s\n\n", clrGray, clrReset)
			os.Exit(1)
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
		printEngineWarnings(eng)

		dotPath, node := resolveGetTarget(eng, result, sc)

		if node.Children == nil {
			// leaf — print raw value
			fmt.Println(node.Value)
			return
		}
		// subtree — print tree
		printTree(&secrets.Node{Children: map[string]*secrets.Node{lastSegment(dotPath): node}}, 0)
	}
	return c
}

// resolveGetTarget returns the dot-path and node a get resolves to. A qualified
// scope resolves its vault directly; an unqualified one searches for the
// secret-path under every top-key, requiring exactly one hit (erroring on
// ambiguity across vaults or when nothing matches).
func resolveGetTarget(eng *ward.Engine, result *ward.MergeResult, sc secrets.Scope) (string, *secrets.Node) {
	if sc.Vault != "" {
		node, err := eng.GetAtPath(result, sc.TreePath())
		if err != nil {
			fmt.Fprintln(os.Stderr, "ward:", err)
			os.Exit(1)
		}
		return sc.TreePath(), node
	}

	type hit struct {
		dotPath string
		node    *secrets.Node
	}
	var hits []hit
	for top := range result.Tree {
		full := top + "." + sc.SecretPath
		if node, err := eng.GetAtPath(result, full); err == nil {
			hits = append(hits, hit{dotPath: full, node: node})
		}
	}

	switch len(hits) {
	case 0:
		fmt.Fprintf(os.Stderr, "ward: key not found: %s\n", sc.SecretPath)
		os.Exit(1)
	case 1:
		return hits[0].dotPath, hits[0].node
	default:
		vaults := make([]string, len(hits))
		for i, h := range hits {
			vaults[i] = firstSegment(h.dotPath)
		}
		fmt.Fprintf(os.Stderr, "ward: %s is defined in multiple vaults (%s) — qualify it as <vault>:%s\n", sc.SecretPath, strings.Join(vaults, ", "), sc.SecretPath)
		os.Exit(1)
	}
	return "", nil
}
