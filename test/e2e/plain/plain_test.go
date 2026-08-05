//go:build e2e

package plain_test

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

// .plain.ward is read without a key and merges at the stripped dot-path.
func TestPlain_read_without_key(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	vault := filepath.Base(dir)
	vaultDir := filepath.Join(dir, ".ward", "vaults", vault)
	// public plain file
	os.WriteFile(filepath.Join(vaultDir, "config.plain.ward"),
		[]byte(vault+":\n  config:\n    port: \"8080\"\n"), 0644)

	out, _, code := testutil.Run(t, bin, dir, "get", vault+":config.port")
	if code != 0 {
		t.Fatalf("get exit %d", code)
	}
	if !testutil.Contains(out, "8080") {
		t.Errorf("expected 8080 from plain file, got %q", out)
	}
}

// An encrypted .ward with no key present → error.
func TestPlain_encrypted_without_key_errors(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	vault := filepath.Base(dir)
	// remove the key so nothing decrypts
	os.Remove(filepath.Join(dir, ".ward", vault+".key"))

	_, _, code := testutil.Run(t, bin, dir, "get", vault+":secret_1")
	if code == 0 {
		t.Fatal("expected non-zero exit when key is missing and file is encrypted")
	}
}

// A plaintext .plain.ward keeps working even with the key removed.
func TestPlain_survives_key_removal(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	vault := filepath.Base(dir)
	vaultDir := filepath.Join(dir, ".ward", "vaults", vault)
	os.WriteFile(filepath.Join(vaultDir, "public.plain.ward"),
		[]byte(vault+":\n  public:\n    name: hello\n"), 0644)
	os.Remove(filepath.Join(dir, ".ward", vault+".key"))

	// encrypted secret errors, but plain value is still retrievable and a warning is printed
	out, stderr, _ := testutil.Run(t, bin, dir, "get", vault+":public.name")
	if !testutil.Contains(out, "hello") && !testutil.Contains(stderr, "hello") {
		t.Errorf("expected plain value 'hello' to be readable, stdout=%q stderr=%q", out, stderr)
	}
}
