package cmd

import (
	"fmt"
	"os"
)

// aiModeFlag tracks whether the persistent --ai-mode flag was passed. It is
// set by main.go before command dispatch.
var aiModeFlag = false

// SetAIMode stores the --ai-mode persistent flag value.
func SetAIMode(v bool) {
	aiModeFlag = v
}

// IsAIMode reports whether secret values must be suppressed from output.
// True when the --ai-mode flag was passed OR the WARD_AI_MODE env var is set
// to "1". MCP tools always run in AI mode by passing --ai-mode explicitly.
func IsAIMode() bool {
	return aiModeFlag || os.Getenv("WARD_AI_MODE") == "1"
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
			"      (e.g. ward exec -- sh -c 'curl -H \"Authorization: Bearer $API_KEY\" https://api.example.com')\n\n",
		clrLightRed, clrReset,
		clrGray, clrReset,
		clrBold, clrReset)
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

// printAIModeBanner explains how to keep secrets out of AI agent context.
// Shown by `ward init` and `ward install`.
func printAIModeBanner() {
	fmt.Fprintf(os.Stderr, "\n  %s🔐 AI agents — protect your secrets%s\n\n", clrYellow+clrBold, clrReset)
	fmt.Fprintf(os.Stderr, "  %sward%s hides secret values from AI agents when AI mode is active.\n", clrCyan, clrReset)
	fmt.Fprintf(os.Stderr, "  Configure your agent so it never sees secret values in context:\n\n")
	fmt.Fprintf(os.Stderr, "    1)  Set the env var %sWARD_AI_MODE=1%s for your agent (recommended):\n", clrBold, clrReset)
	fmt.Fprintf(os.Stderr, "        %sOpenCode%s  opencode.json  →  %s\"env\": { \"WARD_AI_MODE\": \"1\" }%s\n", clrCyan, clrReset, clrGray, clrReset)
	fmt.Fprintf(os.Stderr, "        %sClaude Code%s  .claude.json  →  %s\"env\": { \"WARD_AI_MODE\": \"1\" }%s\n", clrCyan, clrReset, clrGray, clrReset)
	fmt.Fprintf(os.Stderr, "\n    2)  Or always pass %s--ai-mode%s when running ward manually:\n", clrBold, clrReset)
	fmt.Fprintf(os.Stderr, "        %sward get db.password --ai-mode%s\n", clrGray, clrReset)
	fmt.Fprintf(os.Stderr, "\n  Without this, AI agents will see secret values in plain text.\n\n")
}