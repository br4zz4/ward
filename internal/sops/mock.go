package sops

import "os"

// MockDecryptor reads the file as plain YAML without any decryption.
// Used in tests to avoid requiring a real age key.
type MockDecryptor struct{}

func (MockDecryptor) Decrypt(path string) ([]byte, error) {
	return os.ReadFile(path)
}
