package cmd

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/br4zz4/ward/internal/secrets"
)

func TestOtpValue_non_otp_unchanged(t *testing.T) {
	val := "abc123-my-token"
	got := otpValue(val, time.Unix(59, 0), false, false)
	if got != val {
		t.Errorf("otpValue(non-otp) = %q, want %q", got, val)
	}
}

func TestOtpValue_generates_code(t *testing.T) {
	got := otpValue("JBSWY3DPEHPK3PXP", time.Unix(59, 0), false, false)
	if !regexp.MustCompile(`^\d{6}$`).MatchString(got) {
		t.Errorf("otpValue(otp) = %q, want 6-digit code", got)
	}
}

func TestOtpValue_raw_returns_original(t *testing.T) {
	val := "JBSWY3DPEHPK3PXP"
	got := otpValue(val, time.Unix(59, 0), true, false)
	if got != val {
		t.Errorf("otpValue(raw) = %q, want original %q", got, val)
	}
}

func TestOtpValue_generation_error_falls_back(t *testing.T) {
	val := "otpauth://totp/Test?secret=INVALID!!!"
	got := otpValue(val, time.Unix(59, 0), false, false)
	if got != val {
		t.Errorf("otpValue(bad) = %q, want original fallback %q", got, val)
	}
}

func TestOtpValue_verbose_still_returns_code(t *testing.T) {
	got := otpValue("JBSWY3DPEHPK3PXP", time.Unix(59, 0), false, true)
	if !regexp.MustCompile(`^\d{6}$`).MatchString(got) {
		t.Errorf("otpValue(verbose) = %q, want 6-digit code", got)
	}
}

func TestParseExecArgs_otp_flags_stripped(t *testing.T) {
	scopes, cmdArgs, prefixed, raw, verbose := parseExecArgs([]string{"--raw", "-v", "--", "env"})
	if len(scopes) != 0 {
		t.Errorf("expected no scopes, got %v", scopes)
	}
	if len(cmdArgs) != 1 || cmdArgs[0] != "env" {
		t.Errorf("expected [env] cmdArgs, got %v", cmdArgs)
	}
	if prefixed {
		t.Error("expected prefixed=false")
	}
	if !raw {
		t.Error("expected raw=true")
	}
	if !verbose {
		t.Error("expected verbose=true")
	}
}

func TestParseExecArgs_verbose_after_exec(t *testing.T) {
	scopes, cmdArgs, _, raw, verbose := parseExecArgs([]string{"-v", "--", "env"})
	if len(scopes) != 0 {
		t.Errorf("expected no scopes, got %v", scopes)
	}
	if len(cmdArgs) != 1 || cmdArgs[0] != "env" {
		t.Errorf("expected [env] cmdArgs, got %v", cmdArgs)
	}
	if raw {
		t.Error("expected raw=false")
	}
	if !verbose {
		t.Error("expected verbose=true from -v")
	}
}

func TestParseExecArgs_command_flags_preserved_after_dashdash(t *testing.T) {
	_, cmdArgs, _, _, _ := parseExecArgs([]string{"--", "env", "-v", "--raw"})
	want := []string{"env", "-v", "--raw"}
	if len(cmdArgs) != len(want) {
		t.Fatalf("expected %v, got %v", want, cmdArgs)
	}
	for i := range want {
		if cmdArgs[i] != want[i] {
			t.Errorf("cmdArgs[%d] = %q, want %q", i, cmdArgs[i], want[i])
		}
	}
}

func TestPrintTreeWith_transforms_leaf_values(t *testing.T) {
	node := &secrets.Node{Children: map[string]*secrets.Node{
		"otp": {Value: "JBSWY3DPEHPK3PXP"},
		"grp": {Children: map[string]*secrets.Node{
			"nested": {Value: "plain"},
		}},
	}}
	out := captureStdout(func() {
		printTreeWith(node, 0, func(v string) string { return "X" + v })
	})
	if !strings.Contains(out, "otp: XJBSWY3DPEHPK3PXP") {
		t.Errorf("expected transformed leaf line, got:\n%s", out)
	}
	if !strings.Contains(out, "nested: Xplain") {
		t.Errorf("expected nested transformed leaf line, got:\n%s", out)
	}
}

func TestPrintTree_unchanged_when_no_transform(t *testing.T) {
	node := &secrets.Node{Children: map[string]*secrets.Node{
		"key": {Value: "value"},
	}}
	out := captureStdout(func() {
		printTree(node, 0)
	})
	if !strings.Contains(out, "key: value") {
		t.Errorf("expected original leaf line, got:\n%s", out)
	}
}

// captureStdout runs fn and returns everything written to stdout.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}