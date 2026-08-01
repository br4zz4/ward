package sops

import (
	"bytes"
	"fmt"
	"os"
)

// PlainDecryptor reads .plain.ward files as plaintext YAML. It refuses to read
// age-armored content: a .plain.ward file must never be encrypted.
type PlainDecryptor struct{}

var ageArmorHeader = []byte("-----BEGIN AGE ENCRYPTED FILE-----")

func (PlainDecryptor) Decrypt(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if bytes.HasPrefix(bytes.TrimSpace(data), ageArmorHeader) {
		return nil, fmt.Errorf("%s is a plain file but is encrypted — a .plain.ward file must not be encrypted", path)
	}
	return data, nil
}

// Encrypt writes plaintext to path unchanged (.plain.ward is never encrypted).
func (PlainDecryptor) Encrypt(path string, plaintext []byte) error {
	return os.WriteFile(path, plaintext, 0644)
}
