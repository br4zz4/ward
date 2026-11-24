//go:build e2e

package ai_mode_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

func TestGet_refusal_guides_three_exits(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "strict")

	out, stderr, code := testutil.Run(t, bin, treeFixture(), "get", "app:main.name")

	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
	clean := testutil.StripANSI(out + stderr)
	for _, want := range []string{"ward exec", "ward check", "ward set", "--from"} {
		if !testutil.Contains(clean, want) {
			t.Errorf("refusal must mention %q, got: %q", want, clean)
		}
	}
	if testutil.Contains(clean, "ward get") {
		t.Errorf("refusal must not suggest ward get, got: %q", clean)
	}
}

func TestFileExtract_refusal_does_not_suggest_ward_get(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "strict")
	dir := t.TempDir()
	if testutil.RunCmd(t, "cp", "-r", fix("file", "basic")+"/.", dir) != 0 {
		t.Fatalf("copy fixture failed")
	}
	src := filepath.Join(dir, "service-account.json")
	if err := os.WriteFile(src, []byte(`{"type":"service_account"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := testutil.Run(t, bin, dir, "file", "add", src, "app"); code != 0 {
		t.Fatalf("file add failed")
	}

	out, stderr, code := testutil.Run(t, bin, dir, "file", "extract", "service-account.json", t.TempDir())

	if code == 0 {
		t.Fatalf("expected refusal, got exit 0")
	}
	clean := testutil.StripANSI(out + stderr)
	if testutil.Contains(clean, "ward get") {
		t.Errorf("refusal must not suggest ward get, got: %q", clean)
	}
}

func TestRaw_refusal_does_not_suggest_ward_get(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "strict")

	out, stderr, code := testutil.Run(t, bin, testutil.FixtureDir("raw", "basic"), "raw", ".ward/vaults/app/main.plain.ward")

	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
	clean := testutil.StripANSI(out + stderr)
	if testutil.Contains(clean, "ward get") {
		t.Errorf("refusal must not suggest ward get, got: %q", clean)
	}
}

func TestExport_refusal_does_not_suggest_ward_get(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "strict")
	dir := testutil.FixtureDir("export", "basic")

	out, stderr, code := testutil.Run(t, bin, dir, "export", ".ward/vaults/app/main.plain.ward")

	if code != 3 {
		t.Fatalf("expected exit 3, got %d", code)
	}
	clean := testutil.StripANSI(out + stderr)
	if testutil.Contains(clean, "ward get") {
		t.Errorf("refusal must not suggest ward get, got: %q", clean)
	}
}
