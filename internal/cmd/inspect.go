package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

// inspectResult collects all detected issues from a single inspect run.
type inspectResult struct {
	structureViolations []string
	conflictErr         *secrets.ConflictError
	envConflictErr      *secrets.EnvConflictError
}

func (r inspectResult) hasErrors() bool {
	return len(r.structureViolations) > 0 || r.conflictErr != nil || r.envConflictErr != nil
}

func NewInspectCmd() *cobra.Command {
	var prefixed bool

	c := &cobra.Command{
		Use:               "inspect [--prefixed] [dot.path]",
		Short:             "Detect conflicts and env var collisions across all files",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			dotPath := ""
			if len(args) == 1 {
				dotPath = args[0]
			}

			result := runInspectAll(dotPath, prefixed)
			printInspectResult(result, prefixed)
		},
	}

	c.Flags().BoolVar(&prefixed, "prefixed", false, "check as if env vars used full path names (no Type-2 collisions)")
	return c
}

// runInspectAll collects structure violations, Type-1 conflicts, and Type-2 env
// collisions without short-circuiting on the first error type found.
func runInspectAll(dotPath string, prefixed bool) inspectResult {
	var result inspectResult

	cfgPath, err := resolvedConfigFile()
	if err != nil {
		return result
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return result
	}
	result.structureViolations = validateVaultStructure(cfg, cfgPath)

	eng, err := newEngine()
	if err != nil {
		fatal(err)
	}

	all, err := eng.InspectAll(dotPath, prefixed)
	if err != nil {
		fatal(err)
	}
	result.conflictErr = all.ConflictErr
	if all.EnvConflictErr != nil {
		all.EnvConflictErr.Cmd = "inspect"
		result.envConflictErr = all.EnvConflictErr
	}

	return result
}

// printInspectResult renders detail blocks for each error type followed by a
// summary footer, then exits non-zero if any errors were found.
func printInspectResult(result inspectResult, prefixed bool) {
	if len(result.structureViolations) > 0 {
		fmt.Fprintf(os.Stderr, "\n  %s✗ vault structure violations%s\n\n", clrLightRed+clrBold, clrReset)
		for _, v := range result.structureViolations {
			fmt.Fprintf(os.Stderr, "  %s•%s %s\n", clrLightRed, clrReset, v)
		}
		fmt.Fprintf(os.Stderr, "\n  %suse %sward edit <file>%s to fix the key path%s\n\n", clrGray, clrCyan, clrGray, clrReset)
	}

	if result.conflictErr != nil {
		printInspectDetailBlock(result.conflictErr)
	}

	if result.envConflictErr != nil {
		printInspectDetailBlock(result.envConflictErr)
	}

	summary := formatInspectSummary(result)
	if result.hasErrors() {
		fmt.Fprint(os.Stderr, summary)
		os.Exit(1)
	}
	fmt.Print(summary)
}

// printInspectDetailBlock prints the styled detail for a ConflictError or
// EnvConflictError, matching the previous reportInspectError output format.
func printInspectDetailBlock(err error) {
	lines := strings.SplitN(err.Error(), "\n", 2)
	fmt.Fprintf(os.Stderr, "%s%s%s\n", clrLightRed, lines[0], clrReset)
	if len(lines) > 1 {
		fmt.Fprintln(os.Stderr, lines[1])
	}
}

// formatInspectSummary returns a single summary line (with ANSI colour) showing
// counts for each error category, similar to test suite footers.
func formatInspectSummary(r inspectResult) string {
	nConflicts := 0
	if r.conflictErr != nil {
		nConflicts = len(r.conflictErr.Conflicts)
	}
	nEnv := 0
	if r.envConflictErr != nil {
		nEnv = len(r.envConflictErr.Conflicts)
	}
	nStruct := len(r.structureViolations)

	conflictLabel := "conflicts"
	if nConflicts == 1 {
		conflictLabel = "conflict"
	}
	envLabel := "env collisions"
	if nEnv == 1 {
		envLabel = "env collision"
	}
	structLabel := "structure errors"
	if nStruct == 1 {
		structLabel = "structure error"
	}

	if r.hasErrors() {
		return fmt.Sprintf("  %s✗%s %s%d %s%s, %d %s, %d %s%s\n",
			clrLightRed+clrBold, clrReset,
			clrBold, nConflicts, conflictLabel, clrReset,
			nEnv, envLabel,
			nStruct, structLabel,
			clrReset,
		)
	}
	return fmt.Sprintf("  %s✓%s %s%d %s, %d %s, %d %s%s\n",
		clrGreen+clrBold, clrReset,
		clrBold, nConflicts, conflictLabel,
		nEnv, envLabel,
		nStruct, structLabel,
		clrReset,
	)
}
