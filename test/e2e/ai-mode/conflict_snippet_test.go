//go:build e2e

package ai_mode_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

const conflictValue1 = "SUPERSECRETVALUE1"
const conflictValue2 = "SUPERSECRETVALUE2"

func conflictProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	vault := filepath.Join(dir, ".ward", "vaults", "app")
	if err := os.MkdirAll(filepath.Join(vault, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(dir, ".ward", "config.yaml"):    "vaults:\n  - name: app\n    path: .ward/vaults/app\n",
		filepath.Join(vault, "main.plain.ward"):       "app:\n  main:\n    db:\n      token: " + conflictValue1 + "\n    other: x\n",
		filepath.Join(vault, "main", "db.plain.ward"): "app:\n  main:\n    db:\n      token: " + conflictValue2 + "\n",
	}
	for p, body := range files {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func conflictCommands() map[string][]string {
	return map[string][]string{
		"get":        {"get", "app:main.db.token"},
		"check":      {"check", "app:main.db.token", "anything"},
		"set --from": {"set", "app:main.other", "--from", "app:main.db.token"},
		"tree":       {"tree"},
	}
}

func TestConflictSnippet_never_leaks_values_in_ai_mode(t *testing.T) {
	for _, mode := range []string{"strict", "masked"} {
		for name, args := range conflictCommands() {
			t.Run(mode+"/"+name, func(t *testing.T) {
				// arrange
				dir := conflictProject(t)
				t.Setenv("WARD_AI_MODE", mode)

				// act
				out, stderr, _ := testutil.Run(t, bin, dir, args...)

				// assert
				got := testutil.StripANSI(out + stderr)
				for _, v := range []string{conflictValue1, conflictValue2} {
					if strings.Contains(got, v) {
						t.Errorf("value %q leaked: %q", v, got)
					}
				}
			})
		}
	}
}

func TestConflictSnippet_strict_shows_sensitive_placeholder(t *testing.T) {
	// arrange
	dir := conflictProject(t)
	t.Setenv("WARD_AI_MODE", "strict")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.db.token", "anything")

	// assert
	if code == 0 {
		t.Fatalf("expected failure on conflict, got exit 0 (%q)", out)
	}
	if !strings.Contains(testutil.StripANSI(stderr), "<sensitive>") {
		t.Errorf("expected <sensitive> in conflict output, got %q", stderr)
	}
}

func TestConflictSnippet_masked_shows_masked_form(t *testing.T) {
	// arrange
	dir := conflictProject(t)
	t.Setenv("WARD_AI_MODE", "masked")

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.db.token", "anything")

	// assert
	if code == 0 {
		t.Fatal("expected failure on conflict")
	}
	if !strings.Contains(testutil.StripANSI(stderr), "(17 chars)") {
		t.Errorf("expected masked form in conflict output, got %q", stderr)
	}
}

func TestConflictSnippet_outside_ai_mode_still_shows_snippet(t *testing.T) {
	// arrange
	dir := conflictProject(t)
	t.Setenv("WARD_AI_MODE", "0")

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.db.token", "anything")

	// assert
	if code == 0 {
		t.Fatal("expected failure on conflict")
	}
	got := testutil.StripANSI(stderr)
	if !strings.Contains(got, "token: "+conflictValue1) || !strings.Contains(got, "token: "+conflictValue2) {
		t.Errorf("expected full snippets outside AI mode, got %q", got)
	}
}

func TestSecretsJSON_ai_mode_exits_3(t *testing.T) {
	for _, mode := range []string{"1", "strict", "masked"} {
		t.Run(mode, func(t *testing.T) {
			// arrange
			t.Setenv("WARD_AI_MODE", mode)

			// act
			out, _, code := testutil.Run(t, bin, treeFixture(), "secrets", "--json")

			// assert
			if code != 3 {
				t.Errorf("expected exit 3, got %d", code)
			}
			if out != "" {
				t.Errorf("expected empty stdout, got %q", out)
			}
		})
	}
}
