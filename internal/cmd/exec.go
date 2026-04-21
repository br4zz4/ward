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
		Use:                "exec [--prefixed] [anchor.ward] -- <cmd> [args...]",
		Short:              "Merge secrets and inject as env vars, then run a command",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		Run: func(_ *cobra.Command, args []string) {
			anchorPath, cmdArgs, prefixed := parseExecArgs(args)

			if len(cmdArgs) == 0 {
				fmt.Fprintln(os.Stderr, "ward: exec requires a command after --")
				os.Exit(1)
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			result, err := eng.Merge(anchorPath)
			if err != nil {
				fatal(err)
			}
			envVars, err := eng.EnvVarsMap(result, prefixed)
			if err != nil {
				fatal(err)
			}

			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			cmd.Env = mergeEnv(os.Environ(), envVars)

			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				fatal(err)
			}
		},
	}
}

// parseExecArgs splits [--prefixed] [anchor] -- <cmd> [args...]
func parseExecArgs(args []string) (anchor string, cmdArgs []string, prefixed bool) {
	rest := make([]string, len(args))
	copy(rest, args)

	for i, a := range rest {
		if a == "--" {
			break
		}
		if a == "--prefixed" {
			prefixed = true
			rest = append(rest[:i], rest[i+1:]...)
			break
		}
	}

	for i, a := range rest {
		if a == "--" {
			if i > 0 {
				anchor = rest[0]
			}
			cmdArgs = rest[i+1:]
			return
		}
	}
	cmdArgs = rest
	return
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
