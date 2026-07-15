package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	wardage "github.com/br4zz4/ward/internal/age"
)

// setupRotateFixture creates a minimal ward project with one encrypted .ward file
// and optionally a file secret (.json.ward). Returns the project dir and key path.
func setupRotateFixture(t *testing.T, includeFileSecret bool) (projectDir, keyPath string) {
	t.Helper()
	dir := t.TempDir()

	// generate age key
	keyPath = filepath.Join(dir, ".ward", ".key")
	if err := os.MkdirAll(filepath.Join(dir, ".ward"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := wardage.GenerateKeyForce(keyPath); err != nil {
		t.Fatal(err)
	}

	// create vault dir and encrypt a .ward file
	vaultDir := filepath.Join(dir, ".ward", "vaults", "myapp")
	if err := os.MkdirAll(vaultDir, 0755); err != nil {
		t.Fatal(err)
	}
	enc := wardage.AgeArmorDecryptor{KeyFile: keyPath}
	wardFile := filepath.Join(vaultDir, "secrets.ward")
	if err := enc.Encrypt(wardFile, []byte("myapp:\n  token: secret123\n")); err != nil {
		t.Fatal(err)
	}

	if includeFileSecret {
		jsonFile := filepath.Join(vaultDir, "service-account.json.ward")
		if err := enc.Encrypt(jsonFile, []byte(`{"type":"service_account"}`)); err != nil {
			t.Fatal(err)
		}
	}

	return dir, keyPath
}

func TestRotateKey_reencrypts_ward_files_with_new_key(t *testing.T) {
	// arrange
	projectDir, keyPath := setupRotateFixture(t, false)
	vaultDir := filepath.Join(projectDir, ".ward", "vaults", "myapp")
	wardFile := filepath.Join(vaultDir, "secrets.ward")

	originalKeyContent, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	// act
	if _, err := rotateKey(keyPath, keyPath, []string{vaultDir}); err != nil {
		t.Fatalf("rotateKey failed: %v", err)
	}

	// assert: key file has changed
	newKeyContent, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(newKeyContent) == string(originalKeyContent) {
		t.Fatal("expected key file to have changed after rotation")
	}

	// assert: .ward file decrypts correctly with new key
	enc := wardage.AgeArmorDecryptor{KeyFile: keyPath}
	plain, err := enc.Decrypt(wardFile)
	if err != nil {
		t.Fatalf("expected decryption with new key to succeed: %v", err)
	}
	if !strings.Contains(string(plain), "token: secret123") {
		t.Fatalf("expected plaintext to contain 'token: secret123', got: %s", string(plain))
	}
}

func TestRotateKey_creates_backup_of_old_key(t *testing.T) {
	// arrange
	projectDir, keyPath := setupRotateFixture(t, false)
	vaultDir := filepath.Join(projectDir, ".ward", "vaults", "myapp")
	originalKeyContent, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	wardDir := filepath.Join(projectDir, ".ward")

	// act
	if _, err := rotateKey(keyPath, keyPath, []string{vaultDir}); err != nil {
		t.Fatalf("rotateKey failed: %v", err)
	}

	// assert: a backup file exists with the old key content
	entries, err := os.ReadDir(wardDir)
	if err != nil {
		t.Fatal(err)
	}
	var bkpFile string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".key.bkp-") {
			bkpFile = filepath.Join(wardDir, e.Name())
			break
		}
	}
	if bkpFile == "" {
		t.Fatal("expected a .key.bkp-<timestamp> file to exist after rotation")
	}
	bkpContent, err := os.ReadFile(bkpFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(bkpContent) != string(originalKeyContent) {
		t.Fatal("expected backup file to contain the original key content")
	}
}

func TestRotateKey_no_ward_new_files_remain_on_success(t *testing.T) {
	// arrange
	_, keyPath := setupRotateFixture(t, false)
	vaultDir := filepath.Join(filepath.Dir(filepath.Dir(keyPath)), ".ward", "vaults", "myapp")

	// act
	if _, err := rotateKey(keyPath, keyPath, []string{vaultDir}); err != nil {
		t.Fatalf("rotateKey failed: %v", err)
	}

	// assert: no .ward.new staging files remain
	err := filepath.WalkDir(vaultDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".ward.new") {
			t.Errorf("unexpected staging file left behind: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRotateKey_handles_file_secrets(t *testing.T) {
	// arrange: vault has both a YAML .ward and a binary file secret (.json.ward)
	projectDir, keyPath := setupRotateFixture(t, true)
	vaultDir := filepath.Join(projectDir, ".ward", "vaults", "myapp")

	// act
	if _, err := rotateKey(keyPath, keyPath, []string{vaultDir}); err != nil {
		t.Fatalf("rotateKey failed: %v", err)
	}

	// assert: file secret decrypts with new key
	enc := wardage.AgeArmorDecryptor{KeyFile: keyPath}
	plain, err := enc.Decrypt(filepath.Join(vaultDir, "service-account.json.ward"))
	if err != nil {
		t.Fatalf("expected file secret to decrypt with new key: %v", err)
	}
	if !strings.Contains(string(plain), "service_account") {
		t.Fatalf("expected original content, got: %s", string(plain))
	}
}

func TestRotateKey_rollback_on_encrypt_failure(t *testing.T) {
	// arrange: set up project and capture original key + plaintext
	projectDir, keyPath := setupRotateFixture(t, false)
	vaultDir := filepath.Join(projectDir, ".ward", "vaults", "myapp")
	wardDir := filepath.Join(projectDir, ".ward")
	wardFile := filepath.Join(vaultDir, "secrets.ward")

	originalKeyContent, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	originalCiphertext, err := os.ReadFile(wardFile)
	if err != nil {
		t.Fatal(err)
	}

	// act: simulate failure by making the vault dir read-only so writes fail
	// We add a second vault dir that does not exist to trigger a discover error
	badVaultDir := filepath.Join(projectDir, "nonexistent", "vault")
	_, err = rotateKey(keyPath, keyPath, []string{vaultDir, badVaultDir})

	// assert: rotateKey returned an error
	if err == nil {
		t.Fatal("expected rotateKey to fail when a vault dir does not exist")
	}

	// assert: original key file unchanged
	currentKeyContent, readErr := os.ReadFile(keyPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(currentKeyContent) != string(originalKeyContent) {
		t.Fatal("expected key file to be unchanged after rollback")
	}

	// assert: original .ward file unchanged
	currentCiphertext, readErr := os.ReadFile(wardFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(currentCiphertext) != string(originalCiphertext) {
		t.Fatal("expected .ward file to be unchanged after rollback")
	}

	// assert: no backup file created (failure happened before commit point)
	entries, readErr := os.ReadDir(wardDir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".key.bkp-") {
			t.Errorf("expected no backup file on rollback, found: %s", e.Name())
		}
	}

	// assert: no .ward.new files remain
	_ = filepath.WalkDir(vaultDir, func(path string, d os.DirEntry, wErr error) error {
		if wErr != nil {
			return wErr
		}
		if strings.HasSuffix(path, ".ward.new") {
			t.Errorf("unexpected staging file left behind: %s", path)
		}
		return nil
	})
}

func TestRotateKey_backup_timestamp_format(t *testing.T) {
	// arrange
	projectDir, keyPath := setupRotateFixture(t, false)
	vaultDir := filepath.Join(projectDir, ".ward", "vaults", "myapp")
	wardDir := filepath.Join(projectDir, ".ward")

	before := time.Now().UTC()

	// act
	if _, err := rotateKey(keyPath, keyPath, []string{vaultDir}); err != nil {
		t.Fatalf("rotateKey failed: %v", err)
	}

	after := time.Now().UTC()

	// assert: backup name contains a timestamp parseable as YYYYMMDDHHMMSS
	entries, err := os.ReadDir(wardDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".key.bkp-") {
			continue
		}
		ts := strings.TrimPrefix(e.Name(), ".key.bkp-")
		parsed, parseErr := time.ParseInLocation("20060102150405", ts, time.UTC)
		if parseErr != nil {
			t.Fatalf("backup timestamp %q is not in YYYYMMDDHHMMSS format: %v", ts, parseErr)
		}
		if parsed.Before(before.Truncate(time.Second)) || parsed.After(after.Add(time.Second)) {
			t.Fatalf("backup timestamp %v is outside expected range [%v, %v]", parsed, before, after)
		}
		return
	}
	t.Fatal("no .key.bkp-* file found")
}
