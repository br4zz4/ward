package cmd

import (
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

func TestRedactSnippet(t *testing.T) {
	pin := secrets.Origin{Key: "pin", Value: "1234", Leaf: true, Snippet: "pin: 1234 # code"}
	token := secrets.Origin{Key: "token", Value: "tok_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdefXa9f2", Leaf: true, Snippet: `token: "tok_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdefXa9f2"`}
	fileSecret := secrets.Origin{Key: "cred_json", Value: "token=abc123; note: x", Leaf: true, Snippet: "token=abc123; note: x"}
	mapOrigin := secrets.Origin{Key: "db", Leaf: false, Snippet: "db:"}
	cases := []struct {
		name   string
		mode   string
		origin secrets.Origin
		want   string
	}{
		{"outside AI mode keeps the raw line", "0", pin, "pin: 1234 # code"},
		{"strict hides a short value", "strict", pin, "pin: <sensitive>"},
		{"masked length comes from the parsed value", "masked", pin, "pin: … (4 chars)"},
		{"masked long value", "masked", token, "token: tok_…a9f2 (51 chars)"},
		{"strict file secret", "strict", fileSecret, "cred_json: <sensitive>"},
		{"masked file secret never prints the raw line", "masked", fileSecret, "cred_json: to… x (21 chars)"},
		{"map origin shows only the key", "strict", mapOrigin, "db:"},
		{"empty snippet stays empty", "strict", secrets.Origin{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			t.Setenv("WARD_AI_MODE", tc.mode)

			// act
			got := redactSnippet(tc.origin)

			// assert
			if got != tc.want {
				t.Errorf("redactSnippet = %q, want %q", got, tc.want)
			}
		})
	}
}
