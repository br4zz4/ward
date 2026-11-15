//go:build e2e

package vault_test

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

func fix(name string) string { return testutil.FixtureDir("vault", name) }

func TestVaultList_shows_configured_vaults(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)

	// act
	stdout, _, code := testutil.Run(t, bin, dir, "vault", "list")

	// assert
	if code != 0 {
		t.Fatalf("vault list exit %d", code)
	}
	if !strings.Contains(stdout, "myapp") {
		t.Errorf("expected vault name 'myapp' in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, ".ward/vaults/myapp") {
		t.Errorf("expected vault path '.ward/vaults/myapp' in output, got: %s", stdout)
	}
}

func TestVaultAdd_registers_new_vault(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)

	// act
	stdout, _, code := testutil.Run(t, bin, dir, "vault", "add", "shared", ".commons/ward/vaults/shared")

	// assert
	if code != 0 {
		t.Fatalf("vault add exit %d, output: %s", code, stdout)
	}
	// verify it shows up in list
	out, _, _ := testutil.Run(t, bin, dir, "vault", "list")
	if !strings.Contains(out, "shared") {
		t.Errorf("expected 'shared' in vault list after add, got: %s", out)
	}
}

func TestVaultAdd_rejects_duplicate_name(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)

	// act
	_, _, code := testutil.Run(t, bin, dir, "vault", "add", "myapp", ".ward/vaults/other")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for duplicate vault name")
	}
}

func TestVaultAdd_rejects_duplicate_path(t *testing.T) {
	// arrange
	dir := t.TempDir()
	testutil.RunCmd(t, "cp", "-r", fix("basic")+"/.", dir)

	// act
	_, _, code := testutil.Run(t, bin, dir, "vault", "add", "other", ".ward/vaults/myapp")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for duplicate vault path")
	}
}

func newMyfish(t *testing.T) string {
	t.Helper()
	for _, k := range []string{"WARD_KEY", "WARD_KEY_MYFISH", "WARD_KEY_MYFISH_DEFAULT", "WARD_KEY_MYFISH_RESTRICTED"} {
		t.Setenv(k, "")
	}
	dir := filepath.Join(t.TempDir(), "myfish")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	return dir
}

func TestVaultAdd_subvault_generates_key_folder_stub_and_token(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	// act
	out, stderr, code := testutil.Run(t, bin, dir, "vault", "add", "myfish::restricted")
	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(testutil.StripANSI(out), "WARD_KEY_MYFISH_RESTRICTED=ward-") {
		t.Errorf("expected token in output, got %q", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".ward", "myfish::restricted.key")); err != nil {
		t.Errorf("key file: %v", err)
	}
	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(gi), ".ward/myfish::restricted.key") {
		t.Errorf("key not in .gitignore: %q", gi)
	}
	stub, err := os.ReadFile(filepath.Join(dir, ".ward", "vaults", "myfish::restricted", "secrets.ward"))
	if err != nil {
		t.Fatalf("stub: %v", err)
	}
	if !strings.Contains(string(stub), "BEGIN AGE ENCRYPTED FILE") {
		t.Errorf("stub not encrypted: %q", stub)
	}
	cfg, _ := os.ReadFile(filepath.Join(dir, ".ward", "config.yaml"))
	for _, want := range []string{"name: myfish::restricted", "path: .ward/vaults/myfish::restricted"} {
		if !strings.Contains(string(cfg), want) {
			t.Errorf("config missing %q:\n%s", want, cfg)
		}
	}
}

func TestVaultAdd_subvault_stub_root_and_usable(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	testutil.Run(t, bin, dir, "vault", "add", "myfish::restricted")
	// act
	_, setErr, setCode := testutil.Run(t, bin, dir, "set", "myfish::restricted:secrets.production.DATABASE_URL", "postgres://prod")
	out, getErr, getCode := testutil.Run(t, bin, dir, "get", "myfish::restricted:secrets.production.DATABASE_URL")
	stubOut, _, stubCode := testutil.Run(t, bin, dir, "get", "myfish::restricted:secret_1")
	// assert
	if setCode != 0 {
		t.Fatalf("set exit %d: %s", setCode, setErr)
	}
	if getCode != 0 || !strings.Contains(out, "postgres://prod") {
		t.Errorf("get: code=%d out=%q err=%q", getCode, out, getErr)
	}
	if stubCode != 0 || stubOut == "" {
		t.Errorf("stub root must be myfish::restricted: code=%d out=%q", stubCode, stubOut)
	}
}

func TestVaultAdd_existing_path_only_registers(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	existing := filepath.Join(dir, "shared", ".ward", "vaults", "commons")
	if err := os.MkdirAll(existing, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(existing, "main.ward")
	if err := os.WriteFile(marker, []byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n-----END AGE ENCRYPTED FILE-----\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// act
	_, stderr, code := testutil.Run(t, bin, dir, "vault", "add", "commons", "shared/.ward/vaults/commons")
	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".ward", "commons.key")); err == nil {
		t.Error("no key must be generated for an existing vault")
	}
	entries, _ := os.ReadDir(existing)
	if len(entries) != 1 {
		t.Errorf("no stub must be created, got %v", entries)
	}
	cfg, _ := os.ReadFile(filepath.Join(dir, ".ward", "config.yaml"))
	if !strings.Contains(string(cfg), "name: commons") {
		t.Errorf("vault not registered:\n%s", cfg)
	}
}

func TestVaultAdd_rejects_invalid_names(t *testing.T) {
	for _, name := range []string{"a::b::c", "::b", "a::", "a:b"} {
		// arrange
		dir := newMyfish(t)
		// act
		_, stderr, code := testutil.Run(t, bin, dir, "vault", "add", name)
		// assert
		if code == 0 {
			t.Errorf("%q: expected failure", name)
		}
		if !strings.Contains(stderr, "invalid vault name") {
			t.Errorf("%q: stderr %q missing 'invalid vault name'", name, stderr)
		}
	}
}

func TestVaultAdd_rejects_canonical_duplicate(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	// act
	_, stderr, code := testutil.Run(t, bin, dir, "vault", "add", "myfish")
	// assert
	if code == 0 {
		t.Fatal("myfish duplicates myfish::default")
	}
	if !strings.Contains(stderr, "already exists") && !strings.Contains(stderr, "duplicate") {
		t.Errorf("stderr %q should report the duplicate vault", stderr)
	}
}

func TestVaultAdd_rejects_colliding_envs(t *testing.T) {
	// arrange
	dir := newMyfish(t)
	if _, _, code := testutil.Run(t, bin, dir, "vault", "add", "myfish-restricted"); code != 0 {
		t.Fatal("first add failed")
	}
	// act
	_, stderr, code := testutil.Run(t, bin, dir, "vault", "add", "myfish::restricted")
	// assert
	if code == 0 {
		t.Fatal("expected failure for colliding envs")
	}
	if !strings.Contains(stderr, "WARD_KEY_MYFISH_RESTRICTED") {
		t.Errorf("stderr %q missing env name", stderr)
	}
}
