package cmd

import (
	"strings"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

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

// completeDotPaths provides shell completion for `ward get` by listing all
// dot-path keys available in the merged tree.
func completeDotPaths(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	eng, err := newEngine()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	result, err := eng.Merge()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	all := collectDotPaths(result.Tree, "")
	var matches []string
	for _, p := range all {
		if toComplete == "" || strings.HasPrefix(p, toComplete) {
			matches = append(matches, p)
		}
	}
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
