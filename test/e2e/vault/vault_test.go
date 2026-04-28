//go:build e2e

package vault_test

import (
	"os"
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
