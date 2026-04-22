package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/br4zz4/ward/internal/ward"
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
			dotPath, cmdArgs, prefixed := parseExecArgs(args)

			if len(cmdArgs) == 0 {
				fmt.Fprintln(os.Stderr, "ward: exec requires a command after --")
				os.Exit(1)
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			result, err := eng.MergeScoped(dotPath)
			if err != nil {
				fatal(err)
			}

			envVars, err := resolveEnvVars(eng, result, dotPath, prefixed)
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

// resolveEnvVars returns env vars from the full merged tree.
// dotPath is used as a preference hint to resolve env var collisions.
func resolveEnvVars(eng *ward.Engine, result *ward.MergeResult, dotPath string, prefixed bool) (map[string]string, error) {
	entries, err := eng.EnvVarsPrefer(result, prefixed, dotPath)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(entries))
	for k, e := range entries {
		out[k] = e.Value
	}
	return out, nil
}

// parseExecArgs parses: [--prefixed] [dot.path] -- <cmd> [args...]
func parseExecArgs(args []string) (dotPath string, cmdArgs []string, prefixed bool) {
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--prefixed" {
			prefixed = true
			continue
		}
		rest = append(rest, a)
	}

	for i, a := range rest {
		if a == "--" {
			if i > 0 {
				dotPath = rest[0]
			}
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
