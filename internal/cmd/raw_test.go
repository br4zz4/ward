package cmd

import (
	"strings"
	"testing"
)

func TestIndent_prefixes_two_spaces(t *testing.T) {
	// arrange
	content := []byte("app:\n  main:\n    name: svc\n")

	// act
	got := indent(content)

	// assert: every non-empty line gains a two-space prefix
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if !strings.HasPrefix(line, "  ") {
			t.Fatalf("expected two-space indent on each line, got line %q in:\n%s", line, got)
		}
	}
}

func TestIndent_drops_trailing_blank(t *testing.T) {
	// arrange
	content := []byte("a: 1\n\n\n")

	// act
	got := indent(content)

	// assert: no trailing blank lines beyond the final newline
	if strings.HasSuffix(got, "\n\n") {
		t.Fatalf("expected trailing blank lines trimmed, got:\n%q", got)
	}
	if got != "  a: 1\n" {
		t.Fatalf("expected %q, got %q", "  a: 1\n", got)
	}
}

func TestIndent_empty_content_is_empty(t *testing.T) {
	if got := indent([]byte("\n\n")); got != "" {
		t.Fatalf("expected empty string for blank content, got %q", got)
	}
}

func TestRawHeader_shows_path_and_rule(t *testing.T) {
	// arrange
	path := ".ward/vaults/app/main.ward"

	// act
	got := stripANSICmd(rawHeader(path))

	// assert: the path on the first line, a rule of matching width on the second
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 header lines, got %d: %q", len(lines), got)
	}
	if lines[0] != path {
		t.Errorf("expected path %q on line 1, got %q", path, lines[0])
	}
	if lines[1] != strings.Repeat("─", len([]rune(path))) {
		t.Errorf("expected rule matching path width, got %q", lines[1])
	}
}

// stripANSICmd removes ANSI escape codes for assertions in this package.
func stripANSICmd(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
