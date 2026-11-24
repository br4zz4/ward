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
