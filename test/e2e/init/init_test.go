//go:build e2e

package init_test

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestInit_creates_ward_dir(t *testing.T) {
	// arrange
	dir := t.TempDir()

	// act
	_, _, code := testutil.Run(t, bin, dir, "init")

	// assert
	if code != 0 {
		t.Fatalf("init exit %d", code)
	}
	dirName := filepath.Base(dir)
	for _, path := range []string{
		filepath.Join(dir, ".ward"),
		filepath.Join(dir, ".ward", "config.yaml"),
		filepath.Join(dir, ".ward", "vaults", dirName+"::default"),
		filepath.Join(dir, ".ward", dirName+"::default.key"),
	} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after init", path)
		}
	}
}

func TestInit_creates_secrets_file(t *testing.T) {
	// arrange
	dir := t.TempDir()

	// act
	_, _, code := testutil.Run(t, bin, dir, "init")

	// assert
	if code != 0 {
		t.Fatalf("init exit %d", code)
	}
	dirName := filepath.Base(dir)
	vault := filepath.Join(dir, ".ward", "vaults", dirName+"::default")
	entries, err := os.ReadDir(vault)
	if err != nil {
		t.Fatalf("reading vault dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one .ward file in vault after init")
	}
}

func TestInit_fails_if_already_initialized(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.Run(t, bin, dir, "init")

	// act
	_, _, code := testutil.Run(t, bin, dir, "init")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit when init called twice")
	}
}

func TestInit_get_works_after_init(t *testing.T) {
	// arrange
	dir := t.TempDir()

	// act
	_, _, code := testutil.Run(t, bin, dir, "init")

	// assert
	if code != 0 {
		t.Fatalf("init exit %d", code)
	}
	// After init, inspect should pass (clean state)
	_, _, code = testutil.Run(t, bin, dir, "inspect")
	if code != 0 {
		t.Fatalf("inspect should pass after init, got %d", code)
	}
}

func newMyfish(t *testing.T) string {
	t.Helper()
	for _, k := range []string{"WARD_KEY", "WARD_KEY_MYFISH", "WARD_KEY_MYFISH_DEFAULT"} {
		t.Setenv(k, "")
	}
	dir := filepath.Join(t.TempDir(), "myfish")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInit_config_declares_default_subvault(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	// act
	_, stderr, code := testutil.Run(t, bin, dir, "init")
	// assert
	if code != 0 {
		t.Fatalf("init exit %d: %s", code, stderr)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, ".ward", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"name: myfish::default", "path: .ward/vaults/myfish::default"} {
		if !strings.Contains(string(cfg), want) {
			t.Errorf("config missing %q:\n%s", want, cfg)
		}
	}
}

func TestInit_layout_key_and_gitignore(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	// act
	_, _, code := testutil.Run(t, bin, dir, "init")
	// assert
	if code != 0 {
		t.Fatalf("init exit %d", code)
	}
	for _, path := range []string{
		filepath.Join(dir, ".ward", "myfish::default.key"),
		filepath.Join(dir, ".ward", "vaults", "myfish::default"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s: %v", path, err)
		}
	}
	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(gi), ".ward/myfish::default.key") {
		t.Errorf("expected key in .gitignore, got %q", gi)
	}
}

func TestInit_stub_root_is_default_subvault(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	testutil.Run(t, bin, dir, "init")
	// act
	out, stderr, code := testutil.Run(t, bin, dir, "secrets", "--json")
	// assert
	if code != 0 {
		t.Fatalf("secrets exit %d: %s", code, stderr)
	}
	clean := testutil.StripANSI(out)
	for _, want := range []string{"secret_1", "secret_2"} {
		if !strings.Contains(clean, want) {
			t.Errorf("expected %s in %q", want, clean)
		}
	}
	if strings.Contains(testutil.StripANSI(stderr), "violation") {
		t.Errorf("unexpected violations: %q", stderr)
	}
	_, vErr, vCode := testutil.Run(t, bin, dir, "inspect")
	if vCode != 0 {
		t.Errorf("inspect exit %d: %s", vCode, vErr)
	}
}

func TestInit_prints_default_env_token_and_alias(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	// act
	out, _, code := testutil.Run(t, bin, dir, "init")
	// assert
	if code != 0 {
		t.Fatalf("init exit %d", code)
	}
	clean := testutil.StripANSI(out)
	if !strings.Contains(clean, "WARD_KEY_MYFISH_DEFAULT=ward-") {
		t.Errorf("expected WARD_KEY_MYFISH_DEFAULT token, got %q", clean)
	}
	if !strings.Contains(clean, "WARD_KEY_MYFISH") {
		t.Errorf("expected alias mention, got %q", clean)
	}
}

func TestInit_scope_by_dir_name_still_works(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	testutil.Run(t, bin, dir, "init")
	// act
	a, _, c1 := testutil.Run(t, bin, dir, "get", "myfish:secret_1")
	b, _, c2 := testutil.Run(t, bin, dir, "get", "myfish::default:secret_1")
	// assert
	if c1 != 0 || c2 != 0 {
		t.Fatalf("exit codes %d %d", c1, c2)
	}
	if strings.TrimSpace(a) != strings.TrimSpace(b) {
		t.Errorf("aliases differ: %q vs %q", a, b)
	}
}
