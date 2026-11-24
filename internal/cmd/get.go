package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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
	var jsonOut bool
	c.Flags().BoolVar(&jsonOut, "json", false, "print the resolved value as JSON (refused in AI mode)")
	c.Run = func(c *cobra.Command, args []string) {
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

		aiMode := IsAIMode()
		if aiMode && (jsonOut || AILevel() == levelStrict) {
			aiModeRefusal(sc.FullPath())
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
		printEngineWarnings(eng)

		dotPath, node := resolveGetTarget(eng, result, sc)

		now := time.Now()
		if aiMode {
			if node.Children == nil {
				fmt.Println(aiValue(fmt.Sprintf("%v", node.Value)))
				return
			}
			printTreeWith(&secrets.Node{Children: map[string]*secrets.Node{lastSegment(dotPath): node}}, 0, aiValue)
			return
		}
		if jsonOut {
			printJSONValue(dotPath, node, now, flagRaw(c))
			return
		}
		if node.Children == nil {
			// leaf — print the OTP code when the value is a TOTP secret
			fmt.Println(otpValue(fmt.Sprintf("%v", node.Value), now, flagRaw(c), flagVerbose(c)))
			return
		}
		// subtree — print tree, applying the same OTP transform to each leaf
		printTreeWith(&secrets.Node{Children: map[string]*secrets.Node{lastSegment(dotPath): node}}, 0, func(v string) string {
			return otpValue(v, now, flagRaw(c), flagVerbose(c))
		})
	}
	return c
}

// printJSONValue renders a leaf as {"path","value"}. A subtree is refused so
// machine consumers never get an ambiguous payload.
func printJSONValue(dotPath string, node *secrets.Node, now time.Time, raw bool) {
	if node.Children != nil {
		fatal(fmt.Errorf("--json requires a leaf secret, %s is a subtree", dotPath))
	}
	enc, err := json.MarshalIndent(map[string]string{
		"path":  dotPath,
		"value": otpValue(fmt.Sprintf("%v", node.Value), now, raw, false),
	}, "", "  ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(enc))
}

// resolveGetTarget returns the dot-path and node a get resolves to, exiting on
// error. See findTarget for the resolution rules.
func resolveGetTarget(eng *ward.Engine, result *ward.MergeResult, sc secrets.Scope) (string, *secrets.Node) {
	dotPath, node, err := findTarget(eng, result, sc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ward:", err)
		os.Exit(errorExitCode)
	}
	return dotPath, node
}

// findTarget resolves a scope to a dot-path and node. A qualified scope
// resolves its vault directly; an unqualified one searches for the secret-path
// under every top-key, requiring exactly one hit (erroring on ambiguity across
// vaults or when nothing matches).
func findTarget(eng *ward.Engine, result *ward.MergeResult, sc secrets.Scope) (string, *secrets.Node, error) {
	if sc.Vault != "" {
		node, err := eng.GetAtPath(result, sc.TreePath())
		if err != nil {
			return "", nil, err
		}
		return sc.TreePath(), node, nil
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
		return "", nil, fmt.Errorf("key not found: %s", sc.SecretPath)
	case 1:
		return hits[0].dotPath, hits[0].node, nil
	}
	vaults := make([]string, len(hits))
	for i, h := range hits {
		vaults[i] = firstSegment(h.dotPath)
	}
	return "", nil, fmt.Errorf("%s is defined in multiple vaults (%s) — qualify it as <vault>:%s", sc.SecretPath, strings.Join(vaults, ", "), sc.SecretPath)
}

// lookupLeaf resolves a scope to the stored value of a single leaf secret.
func lookupLeaf(eng *ward.Engine, result *ward.MergeResult, sc secrets.Scope) (dotPath, value string, err error) {
	dotPath, node, err := findTarget(eng, result, sc)
	if err != nil {
		return "", "", err
	}
	if node.Children != nil {
		return "", "", fmt.Errorf("%s is a subtree, not a leaf secret", dotPath)
	}
	return dotPath, fmt.Sprintf("%v", node.Value), nil
}
