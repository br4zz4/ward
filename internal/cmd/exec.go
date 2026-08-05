package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func NewExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "exec [--prefixed] [dot.path] -- <cmd> [args...]",
		Short:              "Merge secrets and inject as env vars, then run a command",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		ValidArgsFunction:  completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			scopes, cmdArgs, prefixed := parseExecArgs(args)

			if len(cmdArgs) == 0 {
				fmt.Fprintln(os.Stderr, "ward: exec requires a command after --")
				os.Exit(1)
			}

			enforceVaultStructure()
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			firstScope := ""
			if len(scopes) > 0 {
				firstScope = scopes[0]
			}
			result, err := eng.MergeScoped(firstScope)
			if err != nil {
				fatal(err)
			}
			printEngineWarnings(eng)

			entries, err := eng.EnvVarsForScopes(result, prefixed, scopes)
			if err != nil {
				fatal(stampEnvCommand(err, "exec"))
			}
			envVars := make(map[string]string, len(entries))
			for k, en := range entries {
				envVars[k] = en.Value
			}

			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			cmd.Env = mergeEnv(os.Environ(), envVars)

			if dir := OriginalDir(); dir != "" {
				cmd.Dir = dir
			}

			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				fatal(err)
			}
		},
	}
}

// parseExecArgs parses: [--prefixed] [dot.path...] -- <cmd> [args...]
func parseExecArgs(args []string) (scopes []string, cmdArgs []string, prefixed bool) {
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--prefixed" {
			prefixed = true
			continue
		}
		rest = append(rest, a)
	}
	for i, a := range rest {
		if a == "--" {
			scopes = rest[:i]
			cmdArgs = rest[i+1:]
			return
		}
	}
	cmdArgs = rest
	return
}

// lastSegment returns the last dot-separated segment of a path.
func lastSegment(dotPath string) string {
	if i := strings.LastIndexByte(dotPath, '.'); i >= 0 {
		return dotPath[i+1:]
	}
	return dotPath
}

// mergeEnv returns the process environment with ward vars appended/overriding.
func mergeEnv(current []string, wardVars map[string]string) []string {
	wardKeys := make(map[string]bool, len(wardVars))
	for k := range wardVars {
		wardKeys[k] = true
	}
	result := make([]string, 0, len(current)+len(wardVars))
	for _, e := range current {
		if k, _, ok := strings.Cut(e, "="); ok && !wardKeys[k] {
			result = append(result, e)
		}
	}
	for k, v := range wardVars {
		result = append(result, k+"="+v)
	}
	return result
}
