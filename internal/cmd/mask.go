package cmd

import (
	"fmt"
	"strings"
)

const (
	maskMinRunes    = 8
	maskMaxEdge     = 4
	maskRevealDiv   = 8
	maskPlaceholder = "<sensitive>"
)

// maskValue renders a secret for AI mode masked level: the first and last k
// runes and the length, with k = min(4, n/8), so at most 25% of the value is
// revealed. Values shorter than 8 runes reveal nothing; multi-line values are
// fully hidden because their edges say nothing about the value.
func maskValue(v string) string {
	if strings.Contains(v, "\n") {
		return maskPlaceholder
	}
	runes := []rune(v)
	n := len(runes)
	if n < maskMinRunes {
		return fmt.Sprintf("… (%d chars)", n)
	}
	k := n / maskRevealDiv
	if k > maskMaxEdge {
		k = maskMaxEdge
	}
	return fmt.Sprintf("%s…%s (%d chars)", string(runes[:k]), string(runes[n-k:]), n)
}
