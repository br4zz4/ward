//go:build e2e

package set_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wardage "github.com/br4zz4/ward/internal/age"
	"github.com/br4zz4/ward/test/e2e/testutil"
)

const fromSecret = "s3cr3t-value-ABC123"
const fromUnique = "unique-value-XYZ789"

const fromAppContent = `app:
  main:
    k: ` + fromSecret + `
    uniq: ` + fromUnique + `
    grp:
      a: x
      b: y
`

const fromOtherContent = `other:
  main:
    k: old-value
    grp2:
      a: "1"
      b: "2"
`

func writeEncrypted(t *testing.T, vaultDir, keyPath, content string) string {
	t.Helper()
	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := wardage.GenerateKey(keyPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	enc := wardage.AgeArmorDecryptor{KeyFile: keyPath}
	if err := enc.Encrypt(filepath.Join(vaultDir, "main.ward"), []byte(content)); err != nil {
		t.Fatal(err)
	}
	return "ward-" + base64.URLEncoding.EncodeToString(raw)
}

func fromProject(t *testing.T) string {
	t.Helper()
	clearKeyEnv(t)
	t.Setenv("WARD_KEY_OTHER", "")
	dir := t.TempDir()
	keys := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ward"), 0o755); err != nil {
		t.Fatal(err)
	}
	appTok := writeEncrypted(t, filepath.Join(dir, ".ward", "vaults", "app"), filepath.Join(keys, "app"), fromAppContent)
	otherTok := writeEncrypted(t, filepath.Join(dir, ".ward", "vaults", "other"), filepath.Join(keys, "other"), fromOtherContent)
	cfg := "vaults:\n  - name: app\n    path: .ward/vaults/app\n  - name: other\n    path: .ward/vaults/other\n"
	if err := os.WriteFile(filepath.Join(dir, ".ward", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WARD_KEY_APP", appTok)
	t.Setenv("WARD_KEY_OTHER", otherTok)
	return dir
}

func TestSetFrom_copies_between_vaults_with_different_keys(t *testing.T) {
	// arrange
	dir := fromProject(t)

	// act
	stdout, stderr, code := testutil.Run(t, bin, dir, "set", "other:main.k", "--from", "app:main.k")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := strings.TrimSuffix(getValue(t, dir, "other:main.k"), "\n"); got != fromSecret {
		t.Errorf("expected destination to hold source value, got %q", got)
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "(from secret app:main.k)") {
		t.Errorf("expected (from secret app:main.k), got %q", stderr)
	}
	if testutil.Contains(stdout+stderr, fromSecret) {
		t.Errorf("output must never contain the value: %q %q", stdout, stderr)
	}
	if got := strings.TrimSuffix(getValue(t, dir, "app:main.k"), "\n"); got != fromSecret {
		t.Errorf("source must stay intact, got %q", got)
	}
}

func TestSetFrom_creates_new_destination_key(t *testing.T) {
	// arrange
	dir := fromProject(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "other:main.fresh", "--from", "app:main.k")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := strings.TrimSuffix(getValue(t, dir, "other:main.fresh"), "\n"); got != fromSecret {
		t.Errorf("got %q", got)
	}
	if got := getValue(t, dir, "other:main.k"); got != "old-value\n" {
		t.Errorf("sibling must be preserved, got %q", got)
	}
}

func TestSetFrom_copies_within_same_vault(t *testing.T) {
	// arrange
	dir := fromProject(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:main.copy", "--from", "app:main.k")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := strings.TrimSuffix(getValue(t, dir, "app:main.copy"), "\n"); got != fromSecret {
		t.Errorf("got %q", got)
	}
}

func TestSetFrom_unqualified_source_with_single_occurrence(t *testing.T) {
	// arrange
	dir := fromProject(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "other:main.k", "--from", "main.uniq")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := strings.TrimSuffix(getValue(t, dir, "other:main.k"), "\n"); got != fromUnique {
		t.Errorf("got %q", got)
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "(from secret app:main.uniq)") {
		t.Errorf("expected qualified source in output, got %q", stderr)
	}
}

func TestSetFrom_works_under_ai_modes_without_revealing_value(t *testing.T) {
	for _, mode := range []string{"strict", "masked"} {
		t.Run(mode, func(t *testing.T) {
			// arrange
			dir := fromProject(t)
			t.Setenv("WARD_AI_MODE", mode)

			// act
			stdout, stderr, code := testutil.Run(t, bin, dir, "set", "other:main.k", "--from", "app:main.k")

			// assert
			t.Setenv("WARD_AI_MODE", "0")
			if code != 0 {
				t.Fatalf("set exit %d: %q", code, stderr)
			}
			if testutil.Contains(stdout+stderr, fromSecret) {
				t.Errorf("output must never contain the value: %q %q", stdout, stderr)
			}
			if got := strings.TrimSuffix(getValue(t, dir, "other:main.k"), "\n"); got != fromSecret {
				t.Errorf("got %q", got)
			}
		})
	}
}

func TestSetFrom_invalid_invocations_fail_and_write_nothing(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"positional value", []string{"set", "other:main.k", "valor", "--from", "app:main.k"}, ""},
		{"literal", []string{"set", "--literal", "other:main.k", "--from", "app:main.k"}, ""},
		{"source subtree", []string{"set", "other:main.k", "--from", "app:main.grp"}, ""},
		{"source missing", []string{"set", "other:main.k", "--from", "app:main.nao.existe"}, ""},
		{"source equals destination", []string{"set", "app:main.k", "--from", "app:main.k"}, "source and destination are the same"},
		{"destination is group", []string{"set", "other:main.grp2", "--from", "app:main.k"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			dir := fromProject(t)
			before := snapshot(t, dir)

			// act
			stdout, stderr, code := testutil.Run(t, bin, dir, c.args...)

			// assert
			if code == 0 {
				t.Fatalf("expected non-zero exit, stderr: %q", stderr)
			}
			if c.want != "" && !testutil.Contains(testutil.StripANSI(stderr), c.want) {
				t.Errorf("expected %q in stderr, got %q", c.want, stderr)
			}
			if testutil.Contains(stdout+stderr, fromSecret) {
				t.Errorf("output must never contain the value")
			}
			assertUntouched(t, before, snapshot(t, dir))
		})
	}
}
