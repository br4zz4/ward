package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMaskValue(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"51 runes reveal 4+4", "tok_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdefXa9f2", "tok_…a9f2 (51 chars)"},
		{"16 runes reveal 2+2", "abcdefghijklmnop", "ab…op (16 chars)"},
		{"10 runes reveal 1+1", "abcdefghij", "a…j (10 chars)"},
		{"8 runes reveal 1+1", "abcdefgh", "a…h (8 chars)"},
		{"7 runes reveal nothing", "abcdefg", "… (7 chars)"},
		{"1 rune reveals nothing", "x", "… (1 chars)"},
		{"0 runes", "", "… (0 chars)"},
		{"32 runes reveal 4+4", "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP", "JBSW…3PXP (32 chars)"},
		{"accents are cut by rune", "áéíóúàèìòùâêîôûãõç", "áé…õç (18 chars)"},
		{"emoji are cut by rune", "😀😁😂🤣😃😄😅😆😉😊", "😀…😊 (10 chars)"},
		{"multi-line is sensitive", "-----BEGIN KEY-----\nabc\n-----END KEY-----", "<sensitive>"},
		{"single trailing newline is sensitive", "abcdefghijklmnop\n", "<sensitive>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// act
			got := maskValue(tc.in)

			// assert
			if got != tc.want {
				t.Errorf("maskValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("result is not valid utf8: %q", got)
			}
		})
	}
}

func TestMaskValue_reveals_at_most_a_quarter(t *testing.T) {
	for n := 0; n <= 80; n++ {
		// arrange
		in := strings.Repeat("x", n)

		// act
		got := maskValue(in)

		// assert
		revealed := strings.Count(strings.SplitN(got, "…", 2)[0], "x")
		if rest := strings.SplitN(got, "…", 2); len(rest) == 2 {
			revealed += strings.Count(strings.SplitN(rest[1], " (", 2)[0], "x")
		}
		if revealed*4 > n {
			t.Errorf("n=%d revealed %d runes (> 25%%): %q", n, revealed, got)
		}
	}
}
