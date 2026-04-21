package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/oporpino/ward/internal/config"
	"github.com/oporpino/ward/internal/ward"
	"github.com/spf13/cobra"
)

func NewExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "exec [--prefixed] [--on-conflict=error|override] [dot.path] -- <cmd> [args...]",
		Short:              "Merge secrets and inject as env vars, then run a command",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		ValidArgsFunction:  completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			dotPath, cmdArgs, prefixed, onConflict := parseExecArgs(args)

			if len(cmdArgs) == 0 {
				fmt.Fprintln(os.Stderr, "ward: exec requires a command after --")
				os.Exit(1)
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			result, err := eng.MergeWithConflict(config.OnConflict(onConflict), dotPath)
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

// resolveEnvVars scopes the result to dotPath (if given) and returns env vars.
func resolveEnvVars(eng *ward.Engine, result *ward.MergeResult, dotPath string, prefixed bool) (map[string]string, error) {
	if dotPath == "" {
		return eng.EnvVarsMap(result, prefixed)
	}
	node, err := eng.GetAtPath(result, dotPath)
	if err != nil {
		return nil, err
	}
	if node.Children == nil {
		key := strings.ToUpper(lastSegment(dotPath))
		return map[string]string{key: fmt.Sprintf("%v", node.Value)}, nil
	}
	scoped := &ward.MergeResult{Tree: node.Children}
	return eng.EnvVarsMap(scoped, prefixed)
}

// parseExecArgs parses: [--prefixed] [--on-conflict=X] [dot.path] -- <cmd> [args...]
func parseExecArgs(args []string) (dotPath string, cmdArgs []string, prefixed bool, onConflict string) {
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--prefixed" {
			prefixed = true
			continue
		}
		if strings.HasPrefix(a, "--on-conflict=") {
			onConflict = strings.TrimPrefix(a, "--on-conflict=")
			continue
		}
		if a == "--on-conflict" && i+1 < len(args) {
			i++
			onConflict = args[i]
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
	// No "--" found — everything is cmd args
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
