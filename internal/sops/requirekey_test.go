package sops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireKeyDecryptor_errors_on_encrypted_ward(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "main.ward")
	os.WriteFile(p, []byte("-----BEGIN AGE ENCRYPTED FILE-----\nabc\n"), 0644)
	_, err := RequireKeyDecryptor{}.Decrypt(p)
	if err == nil {
		t.Fatal("expected error when no key for an encrypted .ward")
	}
	if !strings.Contains(err.Error(), "no encryption key") {
		t.Errorf("expected 'no encryption key' error, got %v", err)
	}
}

func TestRequireKeyDecryptor_reads_plain_ward(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	os.WriteFile(p, []byte("app:\n  x: \"1\"\n"), 0644)
	got, err := RequireKeyDecryptor{}.Decrypt(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(got), "app:") {
		t.Errorf("expected plaintext passthrough, got %q", got)
	}
}

// Regression: a file named exactly "plain.ward" must NOT be treated as a plain
// file — it should yield a NoKeyError, not silently read as plaintext.
func TestRequireKeyDecryptor_exact_plain_ward_is_not_plain(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "plain.ward")
	os.WriteFile(p, []byte("-----BEGIN AGE ENCRYPTED FILE-----\nabc\n"), 0644)
	_, err := RequireKeyDecryptor{}.Decrypt(p)
	if err == nil {
		t.Fatal("expected NoKeyError for plain.ward, got nil")
	}
	if !IsNoKeyError(err) {
		t.Errorf("expected IsNoKeyError to be true, got error: %v", err)
	}
}
