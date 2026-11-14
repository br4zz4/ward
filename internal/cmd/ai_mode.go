package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// aiModeFlag tracks whether the persistent --ai-mode flag was passed. It is
// set by main.go before command dispatch.
var aiModeFlag = false

// SetAIMode stores the --ai-mode persistent flag value.
func SetAIMode(v bool) {
	aiModeFlag = v
}

// agentBinaries are process names that identify an AI agent runtime. Ward walks
// its parent chain looking for them, so it knows when an agent is the caller.
var agentBinaries = []string{
	"opencode",
	"openclaw",
	"claude",
	"codex",
	"cursor-agent",
	"aider",
}

// processInfoFn resolves a pid into its command line and parent pid. Replaced in
// tests to simulate a process tree without spawning `ps`.
var processInfoFn = processInfo

// stdoutIsTerminal reports whether stdout is a terminal. A human reading the
// output is not an AI agent, so auto-detection is skipped when it is one.
var stdoutIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// IsAIMode reports whether secret values must be suppressed from output.
// Precedence: the --ai-mode flag, then the WARD_AI_MODE env var ("1" on, any
// other value off), then auto-detection of an AI agent in the parent process
// chain — the latter skipped when stdout is a terminal. MCP tools always run in
// AI mode by passing --ai-mode explicitly.
func IsAIMode() bool {
	if aiModeFlag {
		return true
	}
	if v := os.Getenv("WARD_AI_MODE"); v != "" {
		return v == "1"
	}
	return agentAncestor() != "" && !stdoutIsTerminal()
}

// agentAncestor walks the parent chain and returns the name of the agent
// runtime that spawned ward, or "" when ward was not spawned by one.
func agentAncestor() string {
	pid := os.Getppid()
	for depth := 0; depth < 16 && pid > 1; depth++ {
		cmdline, ppid := processInfoFn(pid)
		if cmdline == "" {
			return ""
		}
		if name := agentCommandName(cmdline); name != "" {
			return name
		}
		pid = ppid
	}
	return ""
}

// processInfo returns the command line and parent pid of pid, or "" and 0 when
// the process cannot be inspected.
func processInfo(pid int) (string, int) {
	out, err := exec.Command("ps", "-ww", "-o", "ppid=", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", 0
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return "", 0
	}
	ppid, err := strconv.Atoi(fields[0])
	if err != nil {
		return "", 0
	}
	return strings.Join(fields[1:], " "), ppid
}

// agentCommandName returns the agent binary found in a command line, matching
// both a bare binary name and a path segment (node /path/opencode/bin/x.js).
func agentCommandName(cmdline string) string {
	for _, tok := range strings.Fields(cmdline) {
		base := filepath.Base(tok)
		for _, name := range agentBinaries {
			if base == name || strings.Contains(tok, "/"+name+"/") {
				return name
			}
		}
	}
	return ""
}

// aiModeReason explains why AI mode is on, for the refusal message.
func aiModeReason() string {
	if aiModeFlag {
		return "the --ai-mode flag is set"
	}
	if os.Getenv("WARD_AI_MODE") == "1" {
		return "WARD_AI_MODE=1 is set"
	}
	if name := agentAncestor(); name != "" {
		return fmt.Sprintf("ward was started by the %s agent", name)
	}
	return "secret values are hidden from AI agents"
}

// aiModeRefusal prints a styled refusal explaining that secret values are
// never exposed in AI mode. It must NOT tell the agent to run `ward get` (that
// would read the value): the agent may EXECUTE a command that uses the secret,
// but must never READ it.
func aiModeRefusal(scope string) {
	fmt.Fprintf(os.Stderr,
		"\n  %s✗ AI mode active — secret values are never exposed in AI context%s\n\n"+
			"  %s→%s you may EXECUTE commands that use the secret, but never READ it:\n\n"+
			"      %sward exec -- sh -c '<command using the env var>'%s\n\n"+
			"      (e.g. ward exec -- sh -c 'curl -H \"Authorization: Bearer $API_KEY\" https://api.example.com')\n\n"+
			"  %s%s%s\n\n",
		clrLightRed, clrReset,
		clrGray, clrReset,
		clrBold, clrReset,
		clrGray, aiModeReason(), clrReset)
	os.Exit(1)
}

// aiValue returns the display form of a secret value: the value itself in
// normal mode, or "<sensitive>" in AI mode.
func aiValue(v string) string {
	if IsAIMode() {
		return "<sensitive>"
	}
	return v
}

// printAIModeBanner explains how ward keeps secrets out of AI agent context.
// Shown by `ward init` and `ward install`.
func printAIModeBanner() {
	fmt.Fprintf(os.Stderr, "\n  %s🔐 AI agents — protect your secrets%s\n\n", clrYellow+clrBold, clrReset)
	fmt.Fprintf(os.Stderr, "  %sward%s hides secret values from AI agents when AI mode is active.\n", clrCyan, clrReset)
	fmt.Fprintf(os.Stderr, "  An agent is detected from the parent process chain, so no setup is needed\n")
	fmt.Fprintf(os.Stderr, "  for the CLI. Pin it explicitly in your agent config when you want certainty:\n\n")
	fmt.Fprintf(os.Stderr, "    1)  Set the env var %sWARD_AI_MODE=1%s for your agent:\n", clrBold, clrReset)
	fmt.Fprintf(os.Stderr, "        %sOpenCode%s  opencode.json  →  %s\"env\": { \"WARD_AI_MODE\": \"1\" }%s\n", clrCyan, clrReset, clrGray, clrReset)
	fmt.Fprintf(os.Stderr, "        %sClaude Code%s  .claude.json  →  %s\"env\": { \"WARD_AI_MODE\": \"1\" }%s\n", clrCyan, clrReset, clrGray, clrReset)
	fmt.Fprintf(os.Stderr, "\n    2)  Or always pass %s--ai-mode%s when running ward from a script\n", clrBold, clrReset)
	fmt.Fprintf(os.Stderr, "        %sward get db.password --ai-mode%s\n", clrGray, clrReset)
	fmt.Fprintf(os.Stderr, "\n  Disable auto-detection for a shell with %sWARD_AI_MODE=0%s.\n\n", clrBold, clrReset)
}
