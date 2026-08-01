package sops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlainDecryptor_reads_plaintext(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	os.WriteFile(p, []byte("app:\n  x: \"1\"\n"), 0644)
	got, err := PlainDecryptor{}.Decrypt(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(got), "app:") {
		t.Errorf("expected plaintext passthrough, got %q", got)
	}
}

func TestPlainDecryptor_rejects_encrypted(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	os.WriteFile(p, []byte("-----BEGIN AGE ENCRYPTED FILE-----\nabc\n"), 0644)
	_, err := PlainDecryptor{}.Decrypt(p)
	if err == nil {
		t.Fatal("expected error for encrypted .plain.ward")
	}
	if !strings.Contains(err.Error(), "must not be encrypted") {
		t.Errorf("expected 'must not be encrypted' error, got %v", err)
	}
}

func TestPlainDecryptor_encrypt_writes_plaintext(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	if err := (PlainDecryptor{}).Encrypt(p, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "hello" {
		t.Errorf("expected plaintext write, got %q", got)
	}
}
