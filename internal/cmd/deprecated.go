package cmd

import (
	"fmt"
	"os"
)

// warnDeprecated prints a deprecation notice to stderr after a deprecated
// command has run, pointing the user at its replacement.
func warnDeprecated(old, replacement string) {
	fmt.Fprintf(os.Stderr, "\n  %s⚠ '%s' is deprecated%s — use %sward %s%s (will be removed soon)\n",
		clrYellow+clrBold, old, clrReset, clrCyan, replacement, clrReset)
}
