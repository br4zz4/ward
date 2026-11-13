//go:build e2e

package json_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

var bin string

func TestMain(m *testing.M) {
	b, err := testutil.BuildBin()
	if err != nil {
		panic(err)
	}
	bin = b
	code := m.Run()
	os.Remove(b)
	os.Exit(code)
}

func wardProject(t *testing.T, dir string) string {
	t.Helper()
	vault := filepath.Base(dir)
	if _, stderr, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatalf("ward init failed in %s: %s", dir, stderr)
	}
	return vault
}

func setSecret(t *testing.T, dir, scope, value string) {
	t.Helper()
	if _, stderr, code := testutil.Run(t, bin, dir, "set", scope, value); code != 0 {
		t.Fatalf("ward set %q failed: %s", scope, stderr)
	}
}

func parseSecretsTable(t *testing.T, out string) map[string]string {
	t.Helper()
	values := map[string]string{}
	for _, line := range strings.Split(testutil.StripANSI(out), "\n") {
		if strings.TrimSpace(line) == "" || strings.Contains(line, "●") {
			continue
		}
		parts := strings.SplitN(line, "  =  ", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		rest := strings.TrimSpace(parts[1])
		value := strings.SplitN(rest+"  ", "  ", 2)[0]
		values[key] = strings.TrimRight(value, " ")
	}
	return values
}

func parseJSON(t *testing.T, out string) map[string]string {
	t.Helper()
	parsed := map[string]string{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("expected valid JSON object, got error %v\nstdout: %s", err, out)
	}
	return parsed
}

func TestSecretsJSON_matches_plain_output(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vault := wardProject(t, dir)
	setSecret(t, dir, vault+":secrets.main.code.staging.RESEND_API_KEY", "fake-value-123")
	setSecret(t, dir, vault+":secrets.main.code.staging.DATABASE_URL", "postgres://fake")

	// act
	plainOut, _, plainCode := testutil.Run(t, bin, dir, "secrets", "secrets.main.code.staging")
	jsonOut, _, jsonCode := testutil.Run(t, bin, dir, "secrets", "--json", "secrets.main.code.staging")

	// assert
	if plainCode != 0 {
		t.Fatalf("secrets exit %d", plainCode)
	}
	if jsonCode != 0 {
		t.Fatalf("secrets --json exit %d\nstdout: %s", jsonCode, jsonOut)
	}
	plain := parseSecretsTable(t, plainOut)
	encoded := parseJSON(t, jsonOut)
	if !reflect.DeepEqual(encoded, plain) {
		t.Errorf("json values differ from plain output\njson:  %v\nplain: %v", encoded, plain)
	}
}

func TestSecretsJSON_preserves_multiline_value(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vault := wardProject(t, dir)
	multiline := "first-line\nsecond-line\nthird-line"
	setSecret(t, dir, vault+":secrets.main.code.staging.PRIVATE_KEY", multiline)

	// act
	jsonOut, _, code := testutil.Run(t, bin, dir, "secrets", "--json", "secrets.main.code.staging")

	// assert
	if code != 0 {
		t.Fatalf("secrets --json exit %d\nstdout: %s", code, jsonOut)
	}
	encoded := parseJSON(t, jsonOut)
	if got := encoded["PRIVATE_KEY"]; got != multiline {
		t.Errorf("expected multiline value preserved, got %q", got)
	}
}

func TestSecretsJSON_prefixed_uses_full_paths(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vault := wardProject(t, dir)
	setSecret(t, dir, vault+":secrets.main.code.staging.RESEND_API_KEY", "fake-value-123")

	// act
	plainOut, _, _ := testutil.Run(t, bin, dir, "secrets", "--prefixed", "secrets.main.code.staging")
	jsonOut, _, code := testutil.Run(t, bin, dir, "secrets", "--json", "--prefixed", "secrets.main.code.staging")

	// assert
	if code != 0 {
		t.Fatalf("secrets --json --prefixed exit %d\nstdout: %s", code, jsonOut)
	}
	plain := parseSecretsTable(t, plainOut)
	encoded := parseJSON(t, jsonOut)
	if !reflect.DeepEqual(encoded, plain) {
		t.Errorf("prefixed json values differ from plain output\njson:  %v\nplain: %v", encoded, plain)
	}
	expectedKey := vault + "_secrets_main_code_staging_RESEND_API_KEY"
	if _, ok := encoded[expectedKey]; !ok {
		t.Errorf("expected full-path key %q, got %v", expectedKey, encoded)
	}
}

func TestGetJSON_emits_path_and_value(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vault := wardProject(t, dir)
	setSecret(t, dir, vault+":secrets.main.code.staging.RESEND_API_KEY", "fake-value-123")
	scope := vault + ":secrets.main.code.staging.RESEND_API_KEY"

	// act
	jsonOut, _, code := testutil.Run(t, bin, dir, "get", "--json", scope)

	// assert
	if code != 0 {
		t.Fatalf("get --json exit %d\nstdout: %s", code, jsonOut)
	}
	encoded := parseJSON(t, jsonOut)
	if len(encoded) != 2 {
		t.Fatalf("expected exactly path and value, got %v", encoded)
	}
	if encoded["value"] != "fake-value-123" {
		t.Errorf("expected value fake-value-123, got %q", encoded["value"])
	}
	if encoded["path"] != vault+".secrets.main.code.staging.RESEND_API_KEY" {
		t.Errorf("expected dotted path, got %q", encoded["path"])
	}
}

func TestGetJSON_subtree_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vault := wardProject(t, dir)
	setSecret(t, dir, vault+":secrets.main.code.staging.RESEND_API_KEY", "fake-value-123")

	// act
	_, _, code := testutil.Run(t, bin, dir, "get", "--json", vault+":secrets.main.code.staging")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for a subtree with --json")
	}
}

func TestSecretsJSON_refused_in_ai_mode(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vault := wardProject(t, dir)
	setSecret(t, dir, vault+":secrets.main.code.staging.RESEND_API_KEY", "fake-value-123")
	t.Setenv("WARD_AI_MODE", "1")

	// act
	stdout, _, code := testutil.Run(t, bin, dir, "secrets", "--json", "secrets.main.code.staging")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for --json in AI mode")
	}
	if stdout != "" {
		t.Errorf("expected empty stdout in AI mode, got %q", stdout)
	}
}

func TestGetJSON_missing_key_fails_without_stdout(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vault := wardProject(t, dir)
	setSecret(t, dir, vault+":secrets.main.code.staging.RESEND_API_KEY", "fake-value-123")

	// act
	stdout, _, code := testutil.Run(t, bin, dir, "get", "--json", vault+":secrets.main.code.staging.NOPE")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for a missing key")
	}
	if stdout != "" {
		t.Errorf("expected empty stdout on failure, got %q", stdout)
	}
}

func TestGetJSON_raw_returns_original_otpauth_value(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vault := wardProject(t, dir)
	otp := "otpauth://totp/Ward:test?secret=JBSWY3DPEHPK3PXP&issuer=Ward"
	setSecret(t, dir, vault+":secrets.main.code.staging.TOTP", otp)

	// act
	jsonOut, stderr, code := testutil.Run(t, bin, dir, "--raw", "get", "--json", vault+":secrets.main.code.staging.TOTP")

	// assert
	if code != 0 {
		t.Fatalf("get --json --raw exit %d\nstdout: %s\nstderr: %s", code, jsonOut, stderr)
	}
	encoded := parseJSON(t, jsonOut)
	if encoded["value"] != otp {
		t.Errorf("expected original otpauth value %q, got %q", otp, encoded["value"])
	}
}
