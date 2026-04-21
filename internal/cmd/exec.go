package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/sops"
	"github.com/spf13/cobra"
)

func NewExecCmd() *cobra.Command {
	c := &cobra.Command{
		Use:                "exec [anchor.ward] -- <cmd> [args...]",
		Short:              "Merge secrets and inject as env vars, then run a command",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		Run: func(_ *cobra.Command, args []string) {
			// Parse: optional anchor before "--", then command after "--"
			anchorPath, cmdArgs := parseExecArgs(args)

			if len(cmdArgs) == 0 {
				fmt.Fprintln(os.Stderr, "ward: exec requires a command after --")
				os.Exit(1)
			}

			cfg, err := loadConfig()
			if err != nil {
				fatal(err)
			}

			tree, err := loadAndMerge(cfg, anchorPath)
			if err != nil {
				fatal(err)
			}

			var envVars map[string]string
			if anchorPath != "" {
				info, _ := os.Stat(anchorPath)
				dec := sops.MockDecryptor{}
				if info != nil && info.IsDir() {
					dirFiles, err := secrets.Discover([]string{anchorPath})
					if err == nil && len(dirFiles) > 0 {
						ref, err := secrets.Load(dirFiles[0], dec)
						if err == nil {
							envVars = secrets.ToEnvVarsFromAnchor(tree, ref.Data)
						}
					}
				} else {
					anchor, err := secrets.Load(anchorPath, dec)
					if err == nil {
						envVars = secrets.ToEnvVarsFromAnchor(tree, anchor.Data)
					}
				}
			}
			if envVars == nil {
				envVars = secrets.ToEnvVars(tree)
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

	return c
}

// parseExecArgs splits [anchor.ward] -- <cmd> [args...]
// Returns anchor path (empty if not provided) and command args.
func parseExecArgs(args []string) (anchor string, cmdArgs []string) {
	for i, a := range args {
		if a == "--" {
			if i > 0 && strings.HasSuffix(args[0], ".ward") {
				anchor = args[0]
			}
			cmdArgs = args[i+1:]
			return
		}
	}
	// No "--" found — treat all as command
	cmdArgs = args
	return
}

// mergeEnv returns the current environment with ward env vars appended/overriding.
func mergeEnv(current []string, ward map[string]string) []string {
	wardKeys := make(map[string]bool, len(ward))
	for k := range ward {
		wardKeys[k] = true
	}

	// Keep existing env vars that ward doesn't override
	result := make([]string, 0, len(current)+len(ward))
	for _, e := range current {
		parts := strings.SplitN(e, "=", 2)
		if !wardKeys[parts[0]] {
			result = append(result, e)
		}
	}
	for k, v := range ward {
		result = append(result, k+"="+v)
	}
	return result
}
