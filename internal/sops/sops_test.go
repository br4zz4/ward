package sops_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brazza-tech/ward/internal/sops"
)

// testdataDir returns the path to test/fixtures/sops-age relative to this file.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file location")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "test", "fixtures", "sops-age")
}

func TestSopsDecryptor_decrypt_existing_file(t *testing.T) {
	dir := testdataDir(t)
	keyFile := filepath.Join(dir, ".ward.key")
	if _, err := os.Stat(keyFile); err != nil {
		t.Skip("testdata key not available")
	}
	wardFile := filepath.Join(dir, "secrets", "company.ward")

	dec := sops.SopsDecryptor{KeyFile: keyFile}
	got, err := dec.Decrypt(wardFile)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	content := string(got)
	if !strings.Contains(content, "company:") {
		t.Errorf("expected 'company:' in decrypted output, got:\n%s", content)
	}
	if strings.Contains(content, "ENC[") {
		t.Errorf("expected no ENC[] tokens in decrypted output, got:\n%s", content)
	}
	if strings.Contains(content, "sops:") {
		t.Errorf("expected no sops metadata in decrypted output, got:\n%s", content)
	}
}
