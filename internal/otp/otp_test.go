package otp

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// fixed times for deterministic tests.
var (
	t59 = time.Unix(59, 0)             // counter 1 at 30s period (RFC 6238)
	tlr = time.Unix(1600000000, 0)     // arbitrary instant
)

// known URI + bare secret for the same base32 secret
const (
	testURI  = "otpauth://totp/Example:alice@google.com?secret=JBSWY3DPEHPK3PXP&issuer=Example"
	testBare = "JBSWY3DPEHPK3PXP"
	// RFC 6238 appendix test secret: base32 of ASCII "12345678901234567890"
	rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
)

func TestIsOTP_otpauth_uri(t *testing.T) {
	got := IsOTP(testURI)
	if !got {
		t.Errorf("IsOTP(%q) = false, want true", testURI)
	}
}

func TestIsOTP_bare_base32_16(t *testing.T) {
	if !IsOTP(testBare) {
		t.Errorf("IsOTP(%q) = false, want true", testBare)
	}
}

func TestIsOTP_bare_base32_32(t *testing.T) {
	if !IsOTP(rfcSecret) {
		t.Errorf("IsOTP(%q) = false, want true", rfcSecret)
	}
}

func TestIsOTP_bare_base32_64(t *testing.T) {
	val := rfcSecret + rfcSecret
	if !IsOTP(val) {
		t.Errorf("IsOTP(64-char) = false, want true")
	}
}

func TestIsOTP_invalid_chars(t *testing.T) {
	cases := []string{
		"JBSWY3DPEHPK3PX1",   // '1' not in [A-Z2-7]
		"JBSWY3DPEHPK3PX0",   // '0' not in [A-Z2-7]
		"jBSWY3DPEHPK3PXP",   // lowercase not in [A-Z2-7]
		"JBSWY-DPEHPK3PXP",   // hyphen not in [A-Z2-7]
		"otpauthxJBSWY3DPA",  // no prefix, mixed
	}
	for _, val := range cases {
		if IsOTP(val) {
			t.Errorf("IsOTP(%q) = true, want false", val)
		}
	}
}

func TestIsOTP_wrong_length(t *testing.T) {
	cases := []string{
		"JBSWY3DP",         // 8 chars
		"JBSWY3DPEHPK3P",   // 14 chars
		"JBSWY3DPEHPK3PXPP", // 17 chars
		"JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP", // 48 chars
	}
	for _, val := range cases {
		if IsOTP(val) {
			t.Errorf("IsOTP(%q) = true, want false", val)
		}
	}
}

func TestIsOTP_empty(t *testing.T) {
	if IsOTP("") {
		t.Error("IsOTP(\"\") = true, want false")
	}
}

func TestGenerateCode_bare_rfc6238_vector(t *testing.T) {
	got, err := GenerateCode(rfcSecret, t59)
	if err != nil {
		t.Fatalf("GenerateCode() unexpected error: %v", err)
	}
	// RFC 6238 SHA1 counter=1 (t=59) yields 94287082; 6-digit = last six = 287082
	if got != "287082" {
		t.Errorf("GenerateCode(rfcSecret, t=59) = %q, want %q", got, "287082")
	}
}

func TestGenerateCode_uri_six_digits(t *testing.T) {
	got, err := GenerateCode(testURI, tlr)
	if err != nil {
		t.Fatalf("GenerateCode(uri) unexpected error: %v", err)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(got) {
		t.Errorf("GenerateCode(uri) = %q, want 6-digit code", got)
	}
}

func TestGenerateCode_bare_six_digits(t *testing.T) {
	got, err := GenerateCode(testBare, tlr)
	if err != nil {
		t.Fatalf("GenerateCode(bare) unexpected error: %v", err)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(got) {
		t.Errorf("GenerateCode(bare) = %q, want 6-digit code", got)
	}
}

func TestGenerateCode_uri_matches_bare_defaults(t *testing.T) {
	// URI with no algorithm/digits/period params must behave like the bare
	// secret with SHA1/6/30 defaults.
	uriCode, err := GenerateCode(testURI, t59)
	if err != nil {
		t.Fatalf("GenerateCode(uri) unexpected error: %v", err)
	}
	bareCode, err := GenerateCode(testBare, t59)
	if err != nil {
		t.Fatalf("GenerateCode(bare) unexpected error: %v", err)
	}
	if uriCode != bareCode {
		t.Errorf("uri code %q != bare code %q at same instant", uriCode, bareCode)
	}
}

func TestGenerateCode_malformed_uri(t *testing.T) {
	val := "otpauth://totp/Test?secret=INVALID!!!"
	if _, err := GenerateCode(val, tlr); err == nil {
		t.Errorf("GenerateCode(%q) expected error for malformed URI, got nil", val)
	}
}

func TestGenerateCode_invalid_base32(t *testing.T) {
	val := "JBSWY3DPEHPK3P1" // '1' is not valid base32
	if _, err := GenerateCode(val, tlr); err == nil {
		t.Errorf("GenerateCode(%q) expected error for invalid base32, got nil", val)
	}
}

func TestVerboseInfo_uri(t *testing.T) {
	got := VerboseInfo(testURI, t59)
	wantLines := []string{
		"Issuer: Example",
		"Account: alice@google.com",
		"Algorithm: SHA1",
		"Digits: 6",
		"Period: 30s",
		"Remaining: 1s",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Errorf("VerboseInfo(uri) missing %q:\n%s", line, got)
		}
	}
}

func TestVerboseInfo_bare_defaults(t *testing.T) {
	got := VerboseInfo(testBare, t59)
	if strings.Contains(got, "Issuer:") || strings.Contains(got, "Account:") {
		t.Errorf("VerboseInfo(bare) should omit Issuer/Account:\n%s", got)
	}
	wantLines := []string{
		"Algorithm: SHA1 (default)",
		"Digits: 6 (default)",
		"Period: 30s (default)",
		"Remaining: 1s",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Errorf("VerboseInfo(bare) missing %q:\n%s", line, got)
		}
	}
}