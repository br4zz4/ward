package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// execWouldLeak reports whether the command tokens would print injected
// secrets to stdout — the obvious patterns an AI agent could use to exfiltrate
// values that exist only in the child process environment. In AI mode these
// commands are refused so the values never reach the agent's context.
//
// The check only covers the obvious tools (env, printenv, echo $VAR, export,
// declare, set, cat /proc/*/environ). Interpreters with obfuscation are out of
// scope: the real enforcement is that the secret value never reaches the agent
// in the first place.
func execWouldLeak(args []string) bool {
	if len(args) == 0 {
		return false
	}
	bin := filepath.Base(args[0])
	switch bin {
	case "env", "printenv", "export", "declare", "set", "envs":
		return true
	case "cat":
		for _, a := range args[1:] {
			if strings.Contains(a, "/environ") {
				return true
			}
		}
		return false
	case "echo", "printf":
		for _, a := range args[1:] {
			if strings.Contains(a, "$") {
				return true
			}
		}
		return false
	default:
		// sh -c 'echo $VAR' / bash -c 'printenv' — the shell wrapper form.
		for i := 1; i < len(args); i++ {
			a := args[i]
			if i+1 < len(args) && (a == "-c" || a == "-lc" || a == "-c ") {
				script := args[i+1]
				if containsLeakPattern(script) {
					return true
				}
			}
		}
		return false
	}
}

// containsLeakPattern reports whether a shell script string contains an obvious
// env-leak idiom: echoing a variable or dumping the environment.
func containsLeakPattern(script string) bool {
	lower := strings.ToLower(script)
	if strings.Contains(lower, "printenv") || strings.Contains(lower, "environ") || strings.Contains(lower, "export ") || strings.Contains(lower, "declare") {
		return true
	}
	// echo $X or echo ${X}
	if strings.Contains(lower, "echo") && strings.Contains(script, "$") {
		return true
	}
	return false
}

// aiModeExecRefusal prints the guard message for a blocked exec in AI mode and
// exits non-zero, explaining why and how to access the value instead.
func aiModeExecRefusal() {
	fmt.Fprintf(os.Stderr,
		"\n  %s✗ AI mode — this command would expose secrets in context%s\n\n"+
			"  %s→%s %senv%s, %secho $VAR%s, %sexport%s, %scat /proc/*/environ%s and similar\n"+
			"    dump the injected secrets into the transcript. Use them only in a shell\n"+
			"    you control, never through an AI agent.\n\n",
		clrLightRed+clrBold, clrReset,
		clrGray, clrReset, clrBold, clrReset, clrBold, clrReset, clrBold, clrReset, clrBold, clrReset)
	os.Exit(1)
}