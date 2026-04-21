package age_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oporpino/ward/internal/age"
)

func TestAgeArmorDecryptor_plaintext_passthrough(t *testing.T) {
	dir := t.TempDir()
	plain := []byte("myapp:\n  secret: hello\n")
	path := filepath.Join(dir, "test.ward")
	if err := os.WriteFile(path, plain, 0644); err != nil {
		t.Fatal(err)
	}

	dec := age.AgeArmorDecryptor{KeyFile: "unused"}
	got, err := dec.Decrypt(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("want %q got %q", plain, got)
	}
}

func TestAgeArmorDecryptor_roundtrip(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "test.key")

	if err := age.GenerateKey(keyFile); err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	plain := []byte("myapp:\n  secret: supersecret\n  token: abc123\n")
	wardFile := filepath.Join(dir, "secrets.ward")

	enc := age.AgeArmorDecryptor{KeyFile: keyFile}
	if err := enc.Encrypt(wardFile, plain); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// encrypted file must not contain plaintext
	ciphertext, _ := os.ReadFile(wardFile)
	if strings.Contains(string(ciphertext), "supersecret") {
		t.Fatal("plaintext value found in encrypted file")
	}
	if !strings.Contains(string(ciphertext), "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Fatal("expected age armor header in encrypted file")
	}

	// decrypt and compare
	dec := age.AgeArmorDecryptor{KeyFile: keyFile}
	got, err := dec.Decrypt(wardFile)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("roundtrip mismatch:\nwant: %q\n got: %q", plain, got)
	}
}

func TestAgeArmorDecryptor_wrong_key(t *testing.T) {
	dir := t.TempDir()
	keyFile1 := filepath.Join(dir, "key1.key")
	keyFile2 := filepath.Join(dir, "key2.key")

	if err := age.GenerateKey(keyFile1); err != nil {
		t.Fatal(err)
	}
	if err := age.GenerateKey(keyFile2); err != nil {
		t.Fatal(err)
	}

	wardFile := filepath.Join(dir, "secrets.ward")
	if err := (age.AgeArmorDecryptor{KeyFile: keyFile1}).Encrypt(wardFile, []byte("secret: val")); err != nil {
		t.Fatal(err)
	}

	_, err := (age.AgeArmorDecryptor{KeyFile: keyFile2}).Decrypt(wardFile)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestGenerateKey_idempotent(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "test.key")

	if err := age.GenerateKey(keyFile); err != nil {
		t.Fatalf("first call: %v", err)
	}
	data1, _ := os.ReadFile(keyFile)

	if err := age.GenerateKey(keyFile); err != nil {
		t.Fatalf("second call: %v", err)
	}
	data2, _ := os.ReadFile(keyFile)

	if string(data1) != string(data2) {
		t.Fatal("GenerateKey must not overwrite existing key")
	}
}

func TestGenerateKey_format(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "test.key")

	if err := age.GenerateKey(keyFile); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(keyFile)
	content := string(data)

	if !strings.Contains(content, "# public key: age1") {
		t.Fatal("key file must contain '# public key: age1...' comment")
	}
	if !strings.Contains(content, "AGE-SECRET-KEY-1") {
		t.Fatal("key file must contain AGE-SECRET-KEY-1 private key")
	}

	info, _ := os.Stat(keyFile)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("key file must have 0600 permissions, got %v", info.Mode().Perm())
	}
}

func TestPublicKeyFrom(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "test.key")

	if err := age.GenerateKey(keyFile); err != nil {
		t.Fatal(err)
	}

	pubKey, err := age.PublicKeyFrom(keyFile)
	if err != nil {
		t.Fatalf("PublicKeyFrom: %v", err)
	}
	if !strings.HasPrefix(pubKey, "age1") {
		t.Fatalf("expected pubkey starting with age1, got %q", pubKey)
	}
}
