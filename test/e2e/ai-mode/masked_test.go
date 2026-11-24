//go:build e2e

package ai_mode_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

const token = "tok_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdefXa9f2"
const maskedToken = "tok_…a9f2 (51 chars)"
const seed = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"
const maskedSeed = "JBSW…3PXP (32 chars)"

const defaultBody = `app:
  main:
    token: ` + token + `
    totp: ` + seed + `
    pem: |
      -----BEGIN PRIVATE KEY-----
      MIIEvQIBADANBgkqhkiG9w0BAQEFAASC
      -----END PRIVATE KEY-----
    group:
      inner: tok_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdefXa9f2
`

func project(t *testing.T, aiMode, body string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := "vaults:\n  - name: app\n    path: .ward/vaults/app\n"
	if aiMode != "" {
		cfg = "ai_mode: " + aiMode + "\n" + cfg
	}
	vault := filepath.Join(dir, ".ward", "vaults", "app")
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ward", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "main.plain.ward"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func clean(out, stderr string) string { return testutil.StripANSI(out + stderr) }

func TestSecrets_default_without_ai_mode_key_is_masked(t *testing.T) {
	// arrange
	dir := project(t, "", defaultBody)
	t.Setenv("WARD_AI_MODE", "1")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "secrets")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := testutil.StripANSI(out)
	if !strings.Contains(got, maskedToken) {
		t.Errorf("expected %q in output, got: %q", maskedToken, got)
	}
	if strings.Contains(got, token) {
		t.Errorf("full value leaked: %q", got)
	}
}

func TestTree_default_without_ai_mode_key_is_masked(t *testing.T) {
	// arrange
	dir := project(t, "", defaultBody)
	t.Setenv("WARD_AI_MODE", "1")

	// act
	out, _, code := testutil.Run(t, bin, dir, "tree")

	// assert
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := testutil.StripANSI(out)
	if !strings.Contains(got, maskedToken) || strings.Contains(got, token) {
		t.Errorf("expected masked token only, got: %q", got)
	}
}

func TestFlag_ai_mode_defaults_to_masked(t *testing.T) {
	// arrange
	dir := project(t, "", defaultBody)

	// act
	out, _, code := testutil.Run(t, bin, dir, "secrets", "--ai-mode")

	// assert
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := testutil.StripANSI(out)
	if !strings.Contains(got, maskedToken) || strings.Contains(got, token) {
		t.Errorf("expected masked token only, got: %q", got)
	}
}

func TestConfig_strict_hides_values_and_refuses_get(t *testing.T) {
	// arrange
	dir := project(t, "strict", defaultBody)
	t.Setenv("WARD_AI_MODE", "1")

	// act
	treeOut, _, treeCode := testutil.Run(t, bin, dir, "tree")
	getOut, getErr, getCode := testutil.Run(t, bin, dir, "get", "app:main.token")

	// assert
	if treeCode != 0 {
		t.Fatalf("tree exit %d", treeCode)
	}
	tree := testutil.StripANSI(treeOut)
	if !strings.Contains(tree, "<sensitive>") || strings.Contains(tree, "tok_") {
		t.Errorf("expected only <sensitive> placeholders, got: %q", tree)
	}
	if getCode != 3 {
		t.Errorf("expected get exit 3, got %d", getCode)
	}
	if strings.Contains(clean(getOut, getErr), "tok_AbCd") || strings.Contains(clean(getOut, getErr), "a9f2") {
		t.Errorf("value leaked on refusal: %q", clean(getOut, getErr))
	}
}

func TestEnv_masked_overrides_config_strict(t *testing.T) {
	// arrange
	dir := project(t, "strict", defaultBody)
	t.Setenv("WARD_AI_MODE", "masked")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "get", "app:main.token")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if strings.TrimSpace(testutil.StripANSI(out)) != maskedToken {
		t.Errorf("expected stdout %q, got %q", maskedToken, out)
	}
}

func TestEnv_strict_overrides_config_masked(t *testing.T) {
	// arrange
	dir := project(t, "masked", defaultBody)
	t.Setenv("WARD_AI_MODE", "strict")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "get", "app:main.token")

	// assert
	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
	if strings.Contains(clean(out, stderr), "a9f2") {
		t.Errorf("value leaked: %q", clean(out, stderr))
	}
}

func TestEnv_true_masks_and_false_shows_clear(t *testing.T) {
	// arrange
	dir := project(t, "", defaultBody)

	// act
	t.Setenv("WARD_AI_MODE", "true")
	onOut, _, onCode := testutil.Run(t, bin, dir, "secrets")
	t.Setenv("WARD_AI_MODE", "false")
	offOut, _, offCode := testutil.Run(t, bin, dir, "secrets")

	// assert
	if onCode != 0 || offCode != 0 {
		t.Fatalf("exit codes: %d %d", onCode, offCode)
	}
	on := testutil.StripANSI(onOut)
	if !strings.Contains(on, maskedToken) || strings.Contains(on, token) {
		t.Errorf("true must mask, got: %q", on)
	}
	off := testutil.StripANSI(offOut)
	if !strings.Contains(off, "tok_AbCdEfGhIjKlMnOp") || strings.Contains(off, "(51 chars)") || strings.Contains(off, "<sensitive>") {
		t.Errorf("false must show the clear value, got: %q", off)
	}
}

func TestEnv_unknown_value_is_fail_closed(t *testing.T) {
	// arrange
	dir := project(t, "masked", defaultBody)
	t.Setenv("WARD_AI_MODE", "masekd")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "secrets")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := testutil.StripANSI(out)
	if !strings.Contains(got, "<sensitive>") || strings.Contains(got, "tok_") {
		t.Errorf("expected <sensitive> only, got: %q", got)
	}
	if !strings.Contains(stderr, `unknown WARD_AI_MODE value "masekd", using strict`) {
		t.Errorf("expected warning on stderr, got: %q", stderr)
	}
}

func TestConfig_invalid_ai_mode_is_fail_closed(t *testing.T) {
	// arrange
	dir := project(t, "loose", defaultBody)
	t.Setenv("WARD_AI_MODE", "1")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "secrets")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := testutil.StripANSI(out)
	if !strings.Contains(got, "<sensitive>") || strings.Contains(got, "tok_") {
		t.Errorf("expected <sensitive> only, got: %q", got)
	}
	if !strings.Contains(stderr, `unknown ai_mode "loose"`) {
		t.Errorf("expected warning on stderr, got: %q", stderr)
	}
}

func TestTree_masked_multiline_is_sensitive(t *testing.T) {
	// arrange
	dir := project(t, "", defaultBody)
	t.Setenv("WARD_AI_MODE", "masked")

	// act
	out, _, code := testutil.Run(t, bin, dir, "tree")

	// assert
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := testutil.StripANSI(out)
	if !strings.Contains(got, "<sensitive>") {
		t.Errorf("expected <sensitive> for the PEM value, got: %q", got)
	}
	if strings.Contains(got, "BEGIN PRIVATE KEY") || strings.Contains(got, "MIIEvQ") {
		t.Errorf("PEM content leaked: %q", got)
	}
}

func TestSecrets_masked_totp_shows_masked_seed_never_code(t *testing.T) {
	// arrange
	dir := project(t, "", "app:\n  main:\n    totp: "+seed+"\n")
	t.Setenv("WARD_AI_MODE", "masked")

	// act
	out, _, code := testutil.Run(t, bin, dir, "secrets")

	// assert
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := testutil.StripANSI(out)
	if !strings.Contains(got, maskedSeed) {
		t.Errorf("expected %q, got: %q", maskedSeed, got)
	}
	if strings.Contains(got, seed) {
		t.Errorf("seed leaked: %q", got)
	}
	if regexp.MustCompile(`(^|[^0-9])[0-9]{6}([^0-9]|$)`).MatchString(got) {
		t.Errorf("a 6-digit OTP code appeared: %q", got)
	}
}

func TestGet_masked_leaf_prints_masked_value(t *testing.T) {
	// arrange
	dir := project(t, "masked", defaultBody)
	t.Setenv("WARD_AI_MODE", "1")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "get", "app:main.token")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if strings.TrimSpace(testutil.StripANSI(out)) != maskedToken {
		t.Errorf("expected stdout %q, got %q", maskedToken, out)
	}
}

func TestGet_masked_subtree_prints_tree_with_masked_leaves(t *testing.T) {
	// arrange
	dir := project(t, "masked", defaultBody)
	t.Setenv("WARD_AI_MODE", "1")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "get", "app:main")

	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	got := testutil.StripANSI(out)
	if !strings.Contains(got, maskedToken) {
		t.Errorf("expected masked leaf in subtree, got: %q", got)
	}
	if !strings.Contains(got, "<sensitive>") {
		t.Errorf("expected multi-line leaf as <sensitive>, got: %q", got)
	}
	if strings.Contains(got, token) || strings.Contains(got, seed) || strings.Contains(got, "BEGIN PRIVATE KEY") {
		t.Errorf("clear value leaked: %q", got)
	}
}

func TestMasked_json_outputs_are_refused(t *testing.T) {
	// arrange
	dir := project(t, "masked", defaultBody)
	t.Setenv("WARD_AI_MODE", "masked")
	cases := [][]string{
		{"secrets", "--json"},
		{"get", "--json", "app:main.token"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			// act
			out, stderr, code := testutil.Run(t, bin, dir, args...)

			// assert
			if code == 0 {
				t.Fatalf("expected non-zero exit")
			}
			if out != "" {
				t.Errorf("expected empty stdout, got %q", out)
			}
			if strings.Contains(clean(out, stderr), "a9f2") {
				t.Errorf("value leaked: %q", stderr)
			}
		})
	}
}

func TestFileExtract_masked_is_refused(t *testing.T) {
	// arrange
	dir := t.TempDir()
	if testutil.RunCmd(t, "cp", "-r", fix("file", "basic")+"/.", dir) != 0 {
		t.Fatalf("copy fixture failed")
	}
	src := filepath.Join(dir, "sa.json")
	if err := os.WriteFile(src, []byte(`{"type":"service_account"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := testutil.Run(t, bin, dir, "file", "add", src, "app"); code != 0 {
		t.Fatalf("file add failed")
	}
	t.Setenv("WARD_AI_MODE", "masked")
	dest := t.TempDir()

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "file", "extract", "sa.json", dest)

	// assert
	if code == 0 {
		t.Fatalf("expected refusal (non-zero exit)")
	}
	if strings.Contains(clean(out, stderr), "service_account") {
		t.Errorf("content leaked: %q", clean(out, stderr))
	}
	if entries, _ := os.ReadDir(dest); len(entries) != 0 {
		t.Errorf("expected nothing extracted, got %d entries", len(entries))
	}
}

func TestSecrets_without_ai_mode_shows_clear_value(t *testing.T) {
	// arrange
	dir := project(t, "masked", defaultBody)
	t.Setenv("WARD_AI_MODE", "0")

	// act
	out, _, code := testutil.Run(t, bin, dir, "secrets")

	// assert
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := testutil.StripANSI(out)
	if !strings.Contains(got, "tok_AbCdEfGhIjKlMnOp") || strings.Contains(got, "(51 chars)") || strings.Contains(got, "<sensitive>") {
		t.Errorf("expected clear value outside AI mode, got: %q", got)
	}
	if strings.Contains(got, maskedToken) {
		t.Errorf("unexpected masking outside AI mode: %q", got)
	}
}
