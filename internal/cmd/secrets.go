package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

// NewSecretsCmd shows the secrets (env vars) that would be injected by exec.
func NewSecretsCmd() *cobra.Command {
	var prefixed bool

	c := &cobra.Command{
		Use:               "secrets [--prefixed] [scope...]",
		Short:             "Show the secrets (env vars) that would be injected by exec",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeDotPaths,
		Run:               func(_ *cobra.Command, args []string) { runSecrets(args, prefixed) },
	}

	c.Flags().BoolVar(&prefixed, "prefixed", false, "use full path env var names")
	return c
}

func runSecrets(args []string, prefixed bool) {
	scopes := args
	firstScope := ""
	if len(scopes) > 0 {
		firstScope = scopes[0]
	}

	enforceVaultStructure()
	eng, err := newEngine()
	if err != nil {
		fatal(err)
	}
	result, err := eng.MergeScoped(firstScope)
	if err != nil {
		fatal(err)
	}
	printEngineWarnings(eng)

	entries, err := eng.EnvVarsForScopes(result, prefixed, scopes)
	if err != nil {
		fatal(stampEnvCommand(err, "secrets"))
	}

	printEnvEntries(entries)
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

	termWidth := terminalWidth()
	const envsOriginReserve = 55
	valMaxCols := termWidth - envsOriginReserve - 20 // leave room for key + "  =  "
	if valMaxCols < 20 {
		valMaxCols = 20
	}

	// Compute column widths: key and value.
	maxKey := 0
	maxVal := 0
	for _, k := range keys {
		if len(k) > maxKey {
			maxKey = len(k)
		}
		if v := truncateValue(fmt.Sprintf("%v", entries[k].Value), valMaxCols); len(v) > maxVal {
			maxVal = len(v)
		}
	}

	for _, k := range keys {
		e := entries[k]
		keyPad := strings.Repeat(" ", maxKey-len(k))
		valStr := truncateValue(fmt.Sprintf("%v", e.Value), valMaxCols)
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
