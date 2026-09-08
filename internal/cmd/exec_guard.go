package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// execWouldLeak reports whether the command tokens would print injected
// secrets to stdout — the patterns an agent could use to exfiltrate values
// that exist only in the child process environment. The guard is
// UNCONDITIONAL: these commands dump the environment or echo secret values,
// which is never what `ward exec` is for (use `ward secrets` to inspect).
//
// Covered: env/printenv/export/declare/set, echo/printf $VAR, cat /proc/*/environ,
// and compound shell scripts (sh -c 'env', sh -c 'echo $KEY', ...).
// Interpreters with obfuscation are out of scope: the real enforcement is that
// the secret value never reaches the caller in the first place.
func execWouldLeak(args []string) bool {
	if len(args) == 0 {
		return false
	}
	bin := filepath.Base(args[0])
	// Compound shell invocation: sh -c 'script' / bash -lc 'script' — inspect
	// the script by splitting it into individual commands.
	if bin == "sh" || bin == "bash" || bin == "zsh" || bin == "dash" || bin == "ksh" {
		for i := 1; i < len(args); i++ {
			if i+1 < len(args) && (args[i] == "-c" || args[i] == "-lc") {
				if scriptContainsLeak(args[i+1]) {
					return true
				}
			}
		}
		return false
	}
	return tokensWouldLeak(args)
}

// tokensWouldLeak checks a single command (argv form): binary + args.
func tokensWouldLeak(args []string) bool {
	if len(args) == 0 {
		return false
	}
	bin := filepath.Base(args[0])
	switch bin {
	case "env", "printenv", "export", "declare", "set", "envs", "typeset":
		return true
	case "cat", "less", "more", "head", "tail", "grep", "dd":
		for _, a := range args[1:] {
			if strings.Contains(a, "environ") {
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
		return false
	}
}

// scriptContainsLeak inspects a shell script string for leak idioms. The script
// is split into individual commands so `test "$X" = "y" && echo ok` (legitimate,
// does not print the value) is allowed while `echo $SECRET` and `env` are not.
func scriptContainsLeak(script string) bool {
	// Fast path: obvious dump words anywhere in the script.
	lower := strings.ToLower(script)
	for _, w := range []string{"printenv", "environ", "export ", "declare", "typeset", "envs"} {
		if strings.Contains(lower, w) {
			return true
		}
	}
	// Split on command separators (;, |, &, newline) and inspect each command.
	parts := regexp.MustCompile(`[;&|\n]`).Split(script, -1)
	for _, part := range parts {
		if tokensWouldLeak(strings.Fields(part)) {
			return true
		}
	}
	return false
}

// execRefusal prints the guard message for a blocked exec and exits non-zero,
// explaining why and how to use secrets safely instead.
func execRefusal() {
	fmt.Fprintf(os.Stderr,
		"\n  %s✗ blocked — this command would print secret values%s\n\n"+
			"  %s→%s %senv%s, %secho $VAR%s, %sexport%s, %scat /proc/*/environ%s and similar\n"+
			"    dump the injected secrets to stdout. To inspect which vars exist, use %sward secrets%s;\n"+
			"    to run a command that USES a secret without printing it, pass it inside %ssh -c%s:\n\n"+
			"      %sward exec -- sh -c '<command using $VAR>'%s\n\n",
		clrLightRed+clrBold, clrReset,
		clrGray, clrReset, clrBold, clrReset, clrBold, clrReset, clrBold, clrReset, clrBold, clrReset,
		clrBold, clrReset, clrBold, clrReset,
		clrBold, clrReset)
	os.Exit(1)
}