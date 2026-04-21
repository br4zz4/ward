package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oporpino/ward/internal/config"
	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/ward"
	"github.com/spf13/cobra"
)

func NewEnvsCmd() *cobra.Command {
	var prefixed bool
	var onConflict string

	c := &cobra.Command{
		Use:               "envs [--prefixed] [--on-conflict=error|override] [dot.path]",
		Short:             "Show the env vars that would be injected by exec",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			dotPath := ""
			if len(args) == 1 {
				dotPath = args[0]
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			result, err := eng.MergeWithConflict(config.OnConflict(onConflict), dotPath)
			if err != nil {
				fatal(err)
			}

			entries, err := resolveEnvEntries(eng, result, dotPath, prefixed)
			if err != nil {
				fatal(err)
			}

			printEnvEntries(entries)
		},
	}

	c.Flags().BoolVar(&prefixed, "prefixed", false, "use full path env var names")
	c.Flags().StringVar(&onConflict, "on-conflict", "", "conflict mode: error (default) | override")
	return c
}

// resolveEnvEntries scopes the result to dotPath (if given) and returns env entries.
func resolveEnvEntries(eng *ward.Engine, result *ward.MergeResult, dotPath string, prefixed bool) (map[string]secrets.EnvEntry, error) {
	if dotPath == "" {
		return eng.EnvVars(result, prefixed)
	}
	node, err := eng.GetAtPath(result, dotPath)
	if err != nil {
		return nil, err
	}
	if node.Children == nil {
		key := strings.ToUpper(lastSegment(dotPath))
		return map[string]secrets.EnvEntry{
			key: {Value: fmt.Sprintf("%v", node.Value)},
		}, nil
	}
	scoped := &ward.MergeResult{Tree: node.Children}
	return eng.EnvVars(scoped, prefixed)
}

// printEnvEntries renders env entries with colour-coded keys and aligned values.
func printEnvEntries(entries map[string]secrets.EnvEntry) {
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

	// Compute column widths: key and value.
	maxKey := 0
	maxVal := 0
	for _, k := range keys {
		if len(k) > maxKey {
			maxKey = len(k)
		}
		if v := fmt.Sprintf("%v", entries[k].Value); len(v) > maxVal {
			maxVal = len(v)
		}
	}

	for _, k := range keys {
		e := entries[k]
		keyPad := strings.Repeat(" ", maxKey-len(k))
		valStr := fmt.Sprintf("%v", e.Value)
		valPad := strings.Repeat(" ", maxVal-len(valStr))
		color := clrGreen
		if e.Overrides {
			color = clrOrange
		}
		origin := ""
		if e.Origin.File != "" {
			if e.Origin.Line > 0 {
				origin = fmt.Sprintf("  %s%s%s:%s%d%s", clrCyan, e.Origin.File, clrReset, clrMagentaSoft, e.Origin.Line, clrReset)
			} else {
				origin = fmt.Sprintf("  %s%s%s", clrCyan, e.Origin.File, clrReset)
			}
		}
		fmt.Printf("%s%s%s%s  =  %s%s%s%s%s\n",
			color, k, clrReset, keyPad, clrGrayLight, valStr, clrReset, valPad, origin)
	}

	fmt.Printf("\n%s%s●%s active  %s●%s overrides%s\n",
		clrGray, clrGreen, clrGray, clrOrange, clrGray, clrReset)
}
