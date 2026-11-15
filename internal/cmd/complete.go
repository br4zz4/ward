package cmd

import (
	"sort"
	"strings"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

// completeVaultNames provides shell completion for vault names.
func completeVaultNames(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		// second arg onwards: complete file paths within the vault
		return completeWardFiles(nil, args, toComplete)
	}
	cfgPath, err := resolvedConfigFile()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, v := range cfg.Vaults {
		if toComplete == "" || strings.HasPrefix(v.CanonicalName(), toComplete) {
			names = append(names, v.CanonicalName())
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeWardFiles provides shell completion for commands that accept a .ward file path.
// It lists all .ward files discovered in the configured vaults.
func completeWardFiles(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	eng, err := newEngine()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	files, err := secrets.Discover(eng.SourcePaths())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var results []string
	for _, f := range files {
		if toComplete == "" || strings.HasPrefix(f, toComplete) || strings.Contains(f, toComplete) {
			results = append(results, f)
		}
	}
	return results, cobra.ShellCompDirectiveNoFileComp
}

// completeDotPaths completes scope arguments in the `vault:secret-path` form.
// Before the colon it offers vault names (with a trailing colon and no space);
// after it, the secret-paths inside that vault.
func completeDotPaths(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	eng, err := newEngine()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	result, err := eng.Merge()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeScopes(result.Tree, toComplete)
}

func completeScopes(tree map[string]*secrets.Node, toComplete string) ([]string, cobra.ShellCompDirective) {
	var matches []string
	vault, _, qualified := secrets.SplitVaultQualifier(toComplete)
	if !qualified || len(toComplete) == len(vault) {
		for vault := range tree {
			if strings.HasPrefix(vault+":", toComplete) {
				matches = append(matches, vault+":")
			}
		}
		sort.Strings(matches)
		return matches, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	}
	node, ok := tree[config.CanonicalVaultName(vault)]
	if !ok || node.Children == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	for _, p := range collectDotPaths(node.Children, "") {
		if full := vault + ":" + p; strings.HasPrefix(full, toComplete) {
			matches = append(matches, full)
		}
	}
	sort.Strings(matches)
	return matches, cobra.ShellCompDirectiveNoFileComp
}

// collectDotPaths recursively collects all dot-paths from a secrets tree.
func collectDotPaths(tree map[string]*secrets.Node, prefix string) []string {
	var paths []string
	for k, node := range tree {
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		if node.Children != nil {
			// Include the intermediate path itself
			paths = append(paths, full)
			// Recurse into children
			paths = append(paths, collectDotPaths(node.Children, full)...)
		} else {
			paths = append(paths, full)
		}
	}
	return paths
}
