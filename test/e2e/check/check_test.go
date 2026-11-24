//go:build e2e

package check_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

var bin string

const stored = "tok_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdefXa9f2"

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

func writeVault(t *testing.T, dir, name, body string) {
	t.Helper()
	vault := filepath.Join(dir, ".ward", "vaults", name)
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "main.plain.ward"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeConfig(t *testing.T, dir string, vaults ...string) {
	t.Helper()
	cfg := "vaults:\n"
	for _, v := range vaults {
		cfg += "  - name: " + v + "\n    path: .ward/vaults/" + v + "\n"
	}
	if err := os.MkdirAll(filepath.Join(dir, ".ward"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".ward", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeConfig(t, dir, "app")
	writeVault(t, dir, "app", "app:\n  main:\n    token: "+stored+"\n    at: \"@x\"\n    group:\n      inner: nested\n")
	return dir
}

func valueFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "v")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func assertNoLeak(t *testing.T, out, stderr string, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		if strings.Contains(out, s) || strings.Contains(stderr, s) {
			t.Errorf("value %q leaked: stdout=%q stderr=%q", s, out, stderr)
		}
	}
}

func assertError(t *testing.T, out, stderr string, code int, hints ...string) {
	t.Helper()
	if code != 2 {
		t.Errorf("expected exit 2, got %d (stderr %q)", code, stderr)
	}
	if out != "" {
		t.Errorf("stdout must be empty on error, got %q", out)
	}
	clean := testutil.StripANSI(stderr)
	if strings.TrimSpace(clean) == "" {
		t.Fatalf("stderr must explain the error")
	}
	for _, h := range hints {
		if !strings.Contains(strings.ToLower(clean), strings.ToLower(h)) {
			t.Errorf("stderr should mention %q, got %q", h, clean)
		}
	}
	assertNoLeak(t, out, stderr, stored)
}

func TestCheck_match_via_file(t *testing.T) {
	// arrange
	dir := project(t)
	f := valueFile(t, stored+"\n")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.token", "@"+f)

	// assert
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if strings.TrimSpace(out) != "match" {
		t.Errorf("expected match, got %q", out)
	}
	assertNoLeak(t, out, stderr, stored)
}

func TestCheck_differs_via_stdin(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	out, stderr, code := testutil.RunWithStdinFull(t, bin, dir, "outro", "check", "app:main.token", "-")

	// assert
	if code != 1 {
		t.Fatalf("expected exit 1, got %d (stderr %q)", code, stderr)
	}
	if strings.TrimSpace(out) != "differs" {
		t.Errorf("expected differs, got %q", out)
	}
	assertNoLeak(t, out, stderr, stored, "outro")
}

func TestCheck_match_via_stdin_strips_trailing_newline(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	out, stderr, code := testutil.RunWithStdinFull(t, bin, dir, stored+"\n", "check", "app:main.token", "-")

	// assert
	if code != 0 || strings.TrimSpace(out) != "match" {
		t.Errorf("expected match/0, got %q/%d (stderr %q)", out, code, stderr)
	}
}

func TestCheck_literal_argument(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	out, _, code := testutil.Run(t, bin, dir, "check", "app:main.token", stored)
	out2, _, code2 := testutil.Run(t, bin, dir, "check", "app:main.token", "wrong")

	// assert
	if code != 0 || strings.TrimSpace(out) != "match" {
		t.Errorf("expected match/0, got %q/%d", out, code)
	}
	if code2 != 1 || strings.TrimSpace(out2) != "differs" {
		t.Errorf("expected differs/1, got %q/%d", out2, code2)
	}
}

func TestCheck_literal_flag_treats_at_value_as_literal(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "--literal", "app:main.at", "@x")

	// assert
	if code != 0 || strings.TrimSpace(out) != "match" {
		t.Errorf("expected match/0, got %q/%d (stderr %q)", out, code, stderr)
	}
}

func TestCheck_double_at_escapes_literal(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.at", "@@x")

	// assert
	if code != 0 || strings.TrimSpace(out) != "match" {
		t.Errorf("expected match/0 (@@x is the literal @x), got %q/%d (stderr %q)", out, code, stderr)
	}
}

func TestCheck_scope_via_vault_and_secret_flags(t *testing.T) {
	// arrange
	dir := project(t)
	f := valueFile(t, stored)

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "--vault", "app", "--secret", "main.token", "@"+f)

	// assert
	if code != 0 || strings.TrimSpace(out) != "match" {
		t.Errorf("expected match/0, got %q/%d (stderr %q)", out, code, stderr)
	}
}

func TestCheck_dir_flag_with_relative_file(t *testing.T) {
	// arrange
	proj := project(t)
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "v"), []byte(stored+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// act
	out, stderr, code := testutil.Run(t, bin, work, "--dir", proj, "check", "app:main.token", "@v")

	// assert
	if code != 0 || strings.TrimSpace(out) != "match" {
		t.Errorf("expected match/0, got %q/%d (stderr %q)", out, code, stderr)
	}
}

func TestCheck_works_in_ai_mode_levels(t *testing.T) {
	for _, mode := range []string{"strict", "masked"} {
		t.Run(mode, func(t *testing.T) {
			// arrange
			t.Setenv("WARD_AI_MODE", mode)
			dir := project(t)
			f := valueFile(t, stored+"\n")

			// act
			out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.token", "@"+f)

			// assert
			if code != 0 || strings.TrimSpace(out) != "match" {
				t.Errorf("expected match/0, got %q/%d (stderr %q)", out, code, stderr)
			}
			assertNoLeak(t, out, stderr, stored)
		})
	}
}

func TestCheck_differs_in_strict_does_not_leak(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "strict")
	dir := project(t)
	f := valueFile(t, "nope\n")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.token", "@"+f)

	// assert
	if code != 1 || strings.TrimSpace(out) != "differs" {
		t.Errorf("expected differs/1, got %q/%d (stderr %q)", out, code, stderr)
	}
	assertNoLeak(t, out, stderr, stored)
}

func TestCheck_missing_scope_is_error(t *testing.T) {
	// arrange
	dir := project(t)
	f := valueFile(t, "x")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:nao.existe", "@"+f)

	// assert
	assertError(t, out, stderr, code)
}

func TestCheck_subtree_is_error(t *testing.T) {
	// arrange
	dir := project(t)
	f := valueFile(t, "x")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.group", "@"+f)

	// assert
	assertError(t, out, stderr, code)
}

func TestCheck_missing_value_file_is_error(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.token", "@/tmp/ward-check-nao-existe-xyz")

	// assert
	assertError(t, out, stderr, code)
}

func TestCheck_empty_value_is_error(t *testing.T) {
	// arrange
	dir := project(t)
	f := valueFile(t, "\n")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.token", "@"+f)

	// assert
	assertError(t, out, stderr, code)
}

func TestCheck_ambiguous_scope_across_vaults_is_error(t *testing.T) {
	// arrange
	dir := t.TempDir()
	writeConfig(t, dir, "vault-a", "vault-b")
	writeVault(t, dir, "vault-a", "vault-a:\n  main:\n    key: one\n")
	writeVault(t, dir, "vault-b", "vault-b:\n  main:\n    key: two\n")
	f := valueFile(t, "one")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "main.key", "@"+f)

	// assert
	assertError(t, out, stderr, code, "vault-a", "vault-b")
}

func TestCheck_unqualified_unique_scope_matches(t *testing.T) {
	// arrange
	dir := t.TempDir()
	writeConfig(t, dir, "vault-a", "vault-b")
	writeVault(t, dir, "vault-a", "vault-a:\n  main:\n    only_a: one\n")
	writeVault(t, dir, "vault-b", "vault-b:\n  main:\n    only_b: two\n")
	f := valueFile(t, "two\n")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "main.only_b", "@"+f)

	// assert
	if code != 0 || strings.TrimSpace(out) != "match" {
		t.Errorf("expected match/0, got %q/%d (stderr %q)", out, code, stderr)
	}
}

func TestCheck_outside_project_is_error(t *testing.T) {
	// arrange
	dir := t.TempDir()
	f := valueFile(t, "x")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.token", "@"+f)

	// assert
	assertError(t, out, stderr, code)
}
