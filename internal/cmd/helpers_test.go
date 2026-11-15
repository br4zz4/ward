package cmd

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/br4zz4/ward/internal/config"
)

func TestTruncateValue_empty(t *testing.T) {
	// arrange + act
	got := truncateValue("", 10)

	// assert
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestTruncateValue_short_unchanged(t *testing.T) {
	// arrange
	val := "abc123"

	// act
	got := truncateValue(val, 10)

	// assert
	if got != val {
		t.Errorf("expected %q unchanged, got %q", val, got)
	}
}

func TestTruncateValue_exact_limit_unchanged(t *testing.T) {
	// arrange
	val := "1234567890" // exactly 10 chars

	// act
	got := truncateValue(val, 10)

	// assert
	if got != val {
		t.Errorf("expected %q unchanged at exact limit, got %q", val, got)
	}
}

func TestTruncateValue_over_limit_truncated(t *testing.T) {
	// arrange
	val := "12345678901" // 11 chars, over limit of 10

	// act
	got := truncateValue(val, 10)

	// assert
	if got != "1234567890…" {
		t.Errorf("expected truncated with ellipsis, got %q", got)
	}
}

func TestTruncateValue_long_value_truncated_at_120(t *testing.T) {
	// arrange: 200 ASCII chars
	val := ""
	for i := 0; i < 200; i++ {
		val += "x"
	}

	// act
	got := truncateValue(val, 120)

	// assert: 120 x's + ellipsis rune = 121 runes
	if len([]rune(got)) != 121 {
		t.Errorf("expected 121 runes (120 + ellipsis), got %d: %q", len([]rune(got)), got)
	}
	if got[len(got)-3:] != "…" {
		t.Errorf("expected ellipsis at end, got %q", got)
	}
}

func TestTruncateValue_visible_len_after_truncation(t *testing.T) {
	// arrange: value longer than cap — visibleLen counts bytes, "…" is 3 bytes
	val := "abcdefghij_extra" // 16 chars, cap = 10

	// act
	got := truncateValue(val, 10)

	// assert: 10 ASCII bytes + 3 bytes for "…" = 13 visible bytes
	vl := visibleLen(got)
	if vl != 13 {
		t.Errorf("expected visible len 13, got %d: %q", vl, got)
	}
}

func TestWardKeyEnvName(t *testing.T) {
	cases := map[string]string{
		"myapp":         "WARD_KEY_MYAPP",
		"messenger-api": "WARD_KEY_MESSENGER_API",
		"my.vault":      "WARD_KEY_MY_VAULT",
		"a-b.c":         "WARD_KEY_A_B_C",
	}
	for name, want := range cases {
		if got := wardKeyEnvName(name); got != want {
			t.Errorf("wardKeyEnvName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestWardKeyEnvName_namespace_separator(t *testing.T) {
	// act
	got := wardKeyEnvName("myfish::restricted")

	// assert
	if got != "WARD_KEY_MYFISH_RESTRICTED" {
		t.Errorf("got %q", got)
	}
}

func subvaultToken(raw string) string {
	return "ward-" + base64.URLEncoding.EncodeToString([]byte(raw))
}

func clearKeyEnvs(t *testing.T) {
	t.Helper()
	for _, k := range []string{"WARD_KEY", "WARD_KEY_MYFISH", "WARD_KEY_MYFISH_DEFAULT", "WARD_KEY_MYFISH_OPEN", "WARD_KEY_MYFISH_RESTRICTED"} {
		t.Setenv(k, "")
	}
}

func assertResolvedKey(t *testing.T, v config.Source, want string) {
	t.Helper()
	path, err := resolveKeyFileForVault(v)
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", v.Name, err)
	}
	if path == "" {
		t.Fatalf("%s: expected a resolved key file", v.Name)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s: key content = %q, want %q", v.Name, got, want)
	}
}

func TestResolveKeyFileForVault_default_with_default_env_only(t *testing.T) {
	for _, name := range []string{"myfish::default", "myfish"} {
		// arrange
		clearKeyEnvs(t)
		t.Setenv("WARD_KEY_MYFISH_DEFAULT", subvaultToken("AGE-SECRET-KEY-1AAA"))
		// act + assert
		assertResolvedKey(t, config.Source{Name: name}, "AGE-SECRET-KEY-1AAA")
	}
}

func TestResolveKeyFileForVault_default_with_plain_env_only(t *testing.T) {
	for _, name := range []string{"myfish::default", "myfish"} {
		// arrange
		clearKeyEnvs(t)
		t.Setenv("WARD_KEY_MYFISH", subvaultToken("AGE-SECRET-KEY-1AAA"))
		// act + assert
		assertResolvedKey(t, config.Source{Name: name}, "AGE-SECRET-KEY-1AAA")
	}
}

func TestResolveKeyFileForVault_default_with_both_equal_envs(t *testing.T) {
	for _, name := range []string{"myfish::default", "myfish"} {
		// arrange
		clearKeyEnvs(t)
		tok := subvaultToken("AGE-SECRET-KEY-1AAA")
		t.Setenv("WARD_KEY_MYFISH", tok)
		t.Setenv("WARD_KEY_MYFISH_DEFAULT", tok)
		// act + assert
		assertResolvedKey(t, config.Source{Name: name}, "AGE-SECRET-KEY-1AAA")
	}
}

func TestResolveKeyFileForVault_default_with_divergent_envs_errors(t *testing.T) {
	for _, name := range []string{"myfish::default", "myfish"} {
		// arrange
		clearKeyEnvs(t)
		t.Setenv("WARD_KEY_MYFISH", subvaultToken("AGE-SECRET-KEY-1AAA"))
		t.Setenv("WARD_KEY_MYFISH_DEFAULT", subvaultToken("AGE-SECRET-KEY-1BBB"))
		// act
		_, err := resolveKeyFileForVault(config.Source{Name: name})
		// assert
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
		for _, want := range []string{"WARD_KEY_MYFISH", "WARD_KEY_MYFISH_DEFAULT"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error %q missing %q", name, err.Error(), want)
			}
		}
	}
}

func TestResolveKeyFileForVault_open_not_unlocked_by_default_env(t *testing.T) {
	// arrange
	clearKeyEnvs(t)
	t.Setenv("WARD_KEY_MYFISH", subvaultToken("AGE-SECRET-KEY-1AAA"))
	// act
	path, err := resolveKeyFileForVault(config.Source{Name: "myfish::open"})
	// assert
	var ku *errKeyUnavailable
	if err != nil && !errors.As(err, &ku) {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected no key for myfish::open, got %q", path)
	}
}

func TestResolveKeyFileForVault_restricted_env(t *testing.T) {
	// arrange
	clearKeyEnvs(t)
	t.Setenv("WARD_KEY_MYFISH_RESTRICTED", subvaultToken("AGE-SECRET-KEY-1RRR"))
	// act + assert
	assertResolvedKey(t, config.Source{Name: "myfish::restricted"}, "AGE-SECRET-KEY-1RRR")
}

func TestBuildVaultDecryptors_indexes_by_canonical_name(t *testing.T) {
	// arrange
	clearKeyEnvs(t)
	t.Setenv("WARD_KEY_MYFISH_DEFAULT", subvaultToken("AGE-SECRET-KEY-1AAA"))
	t.Setenv("WARD_KEY_MYFISH_RESTRICTED", subvaultToken("AGE-SECRET-KEY-1RRR"))
	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults: []config.Source{
			{Name: "myfish::default", Path: ".ward/vaults/myfish::default"},
			{Name: "myfish::restricted", Path: ".ward/vaults/myfish::restricted"},
		},
	}
	// act
	decs, err := BuildVaultDecryptors(cfg)
	// assert
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := decs["myfish"]; !ok {
		t.Errorf("expected key %q, got %v", "myfish", decs)
	}
	if _, ok := decs["myfish::default"]; ok {
		t.Errorf("did not expect declared name as key: %v", decs)
	}
	if _, ok := decs["myfish::restricted"]; !ok {
		t.Errorf("expected key myfish::restricted, got %v", decs)
	}
}
