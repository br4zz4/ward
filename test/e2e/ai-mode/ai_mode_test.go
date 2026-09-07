//go:build e2e

package ai_mode_test

import (
	"os"
	"path/filepath"
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

// fix returns a fixture dir from a sibling command that carries the same
// structure: a plain (unencrypted) vault named `app` with a `main` group.
func fix(cmd, name string) string { return testutil.FixtureDir(cmd, name) }

// treeFixture has: app.main.name = myapp, app.main.db.host = localhost, ...
func treeFixture() string { return fix("tree", "basic") }

// --- ward get --ai-mode refuses ---

func TestGet_ai_mode_refuses(t *testing.T) {
	out, stderr, code := testutil.Run(t, bin, treeFixture(), "get", "app:main.name", "--ai-mode")
	if code == 0 {
		t.Fatalf("expected refusal (non-zero exit), got 0")
	}
	clean := testutil.StripANSI(out+stderr)
	if !testutil.Contains(clean, "AI mode") {
		t.Errorf("expected AI mode refusal message, got: %q", clean)
	}
	if testutil.Contains(clean, "myapp") {
		t.Errorf("secret value leaked in AI mode: %q", clean)
	}
}

// --- ward get with WARD_AI_MODE=1 refuses ---

func TestGet_env_var_ai_mode_refuses(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "1")
	out, stderr, code := testutil.Run(t, bin, treeFixture(), "get", "app:main.name")
	if code == 0 {
		t.Fatalf("expected refusal (non-zero exit), got 0")
	}
	clean := testutil.StripANSI(out+stderr)
	if !testutil.Contains(clean, "AI mode") {
		t.Errorf("expected AI mode refusal message, got: %q", clean)
	}
	if testutil.Contains(clean, "myapp") {
		t.Errorf("secret value leaked in AI mode: %q", clean)
	}
}

// --- ward tree --ai-mode shows paths but no values ---

func TestTree_ai_mode_shows_paths_hides_values(t *testing.T) {
	out, _, code := testutil.Run(t, bin, treeFixture(), "tree", "--ai-mode")
	if code != 0 {
		t.Fatalf("tree --ai-mode should exit 0, got %d", code)
	}
	clean := testutil.StripANSI(out)
	if !testutil.Contains(clean, "app.main.name") && !testutil.Contains(clean, "name") {
		t.Errorf("expected keys visible in AI mode tree, got: %q", clean)
	}
	if testutil.Contains(clean, "myapp") || testutil.Contains(clean, "localhost") {
		t.Errorf("secret values leaked in AI mode tree: %q", clean)
	}
	if !testutil.Contains(clean, "<sensitive>") {
		t.Errorf("expected <sensitive> placeholders, got: %q", clean)
	}
}

// --- ward secrets --ai-mode shows KEY=<sensitive> ---

func TestSecrets_ai_mode_hides_values(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("secrets", "basic"), "secrets", "--ai-mode")
	if code != 0 {
		t.Fatalf("secrets --ai-mode should exit 0, got %d", code)
	}
	clean := testutil.StripANSI(out)
	if !testutil.Contains(clean, "secret_key") {
		t.Errorf("expected key visible in AI mode secrets, got: %q", clean)
	}
	if testutil.Contains(clean, "abc123") {
		t.Errorf("secret value leaked in AI mode secrets: %q", clean)
	}
	if !testutil.Contains(clean, "<sensitive>") {
		t.Errorf("expected <sensitive> placeholders, got: %q", clean)
	}
}

// --- ward file extract --ai-mode refuses ---

func TestFileExtract_ai_mode_refuses(t *testing.T) {
	// arrange: fixture with a real file-secret stored
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

	// act: extract with --ai-mode
	out, stderr, code := testutil.Run(t, bin, dir, "file", "extract", "service-account.json", t.TempDir(), "--ai-mode")
	if code == 0 {
		t.Fatalf("expected refusal (non-zero exit), got 0")
	}
	clean := testutil.StripANSI(out+stderr)
	if !testutil.Contains(clean, "AI mode") {
		t.Errorf("expected AI mode refusal message, got: %q", clean)
	}
	if testutil.Contains(clean, "service_account") {
		t.Errorf("secret content leaked in AI mode: %q", clean)
	}
}

// --- without the flag, behaviour is unchanged ---

func TestGet_without_flag_shows_value(t *testing.T) {
	out, _, code := testutil.Run(t, bin, treeFixture(), "get", "app:main.name")
	if code != 0 {
		t.Fatalf("get exit %d", code)
	}
	if !testutil.Contains(out, "myapp") {
		t.Errorf("expected value without AI mode, got: %q", out)
	}
}