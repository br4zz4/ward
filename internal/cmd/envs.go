package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/ward"
	"github.com/spf13/cobra"
)

func NewEnvsCmd() *cobra.Command {
	var prefixed bool

	c := &cobra.Command{
		Use:   "envs [anchor.ward]",
		Short: "Show the env vars that would be injected by exec",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			anchorPath := ""
			if len(args) == 1 {
				anchorPath = args[0]
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			result, err := eng.Merge(anchorPath)
			if err != nil {
				fatal(err)
			}
			entries, err := eng.EnvVars(result, prefixed)
			if err != nil {
				fatal(err)
			}

			printEnvEntries(entries, result, anchorPath)
		},
	}

	c.Flags().BoolVar(&prefixed, "prefixed", false, "use full path env var names")
	return c
}

// printEnvEntries renders env entries with colour-coded keys and aligned values.
func printEnvEntries(entries map[string]secrets.EnvEntry, result *ward.MergeResult, anchorPath string) {
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, oj := entries[keys[i]].Overrides, entries[keys[j]].Overrides
		if oi != oj {
			return oi
		}
		return keys[i] < keys[j]
	})

	maxLen := 0
	for _, k := range keys {
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}

	// Collect leaf key names that exist in the full tree outside the anchor scope.
	// This catches keys like NAME that originate from an ancestor file but are
	// re-defined (and thus overridden) within the anchor scope.
	rawAncestorKeys := map[string]bool{}
	collectAncestorKeys(&secrets.Node{Children: result.Tree}, anchorPath, rawAncestorKeys)
	// rawAncestorKeys uses the original YAML key name (e.g. "name"); map to the
	// last segment of the env var name for comparison.
	ancestorLeafKeys := map[string]bool{}
	for k := range rawAncestorKeys {
		ancestorLeafKeys[strings.ToLower(k)] = true
	}

	for _, k := range keys {
		e := entries[k]
		padding := strings.Repeat(" ", maxLen-len(k))
		color := envEntryColor(e, k, anchorPath, ancestorLeafKeys)
		fmt.Printf("%s%s%s%s  =  %s%v%s\n",
			color, k, clrReset, padding, clrGray, e.Value, clrReset)
	}

	fmt.Printf("\n%s%s●%s active  %s●%s overrides%s\n",
		clrGray, clrGreen, clrGray, clrOrange, clrGray, clrReset)
}

func envEntryColor(e secrets.EnvEntry, key, anchorPath string, ancestorLeafKeys map[string]bool) string {
	if e.Overrides || (anchorPath != "" && !isFromAnchorScope(e.Origin.File, anchorPath)) || ancestorLeafKeys[lastSegment(key)] {
		return clrOrange
	}
	return clrGreen
}

// lastSegment returns the last underscore-separated segment of an env key (lowercased).
func lastSegment(key string) string {
	lower := strings.ToLower(key)
	if i := strings.LastIndex(lower, "_"); i >= 0 {
		return lower[i+1:]
	}
	return lower
}
