package age

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// AgeArmorDecryptor encrypts and decrypts .ward files as opaque age-armored blobs.
// The entire file is the ciphertext — no YAML structure is exposed.
type AgeArmorDecryptor struct {
	KeyFile string
}

// Decrypt returns the plaintext of path. Plain (unencrypted) files pass through as-is.
func (d AgeArmorDecryptor) Decrypt(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if !isArmored(data) {
		return data, nil
	}
	identity, err := loadIdentity(d.KeyFile)
	if err != nil {
		return nil, err
	}
	r, err := age.Decrypt(armor.NewReader(bytes.NewReader(data)), identity)
	if err != nil {
		return nil, fmt.Errorf("age decrypt %s: %w", path, err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, fmt.Errorf("reading decrypted %s: %w", path, err)
	}
	return buf.Bytes(), nil
}

// Encrypt encrypts plaintext with the key in KeyFile and writes the armored result to path.
func (d AgeArmorDecryptor) Encrypt(path string, plaintext []byte) error {
	recipient, err := loadRecipient(d.KeyFile)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	aw := armor.NewWriter(&buf)
	w, err := age.Encrypt(aw, recipient)
	if err != nil {
		return fmt.Errorf("age encrypt init: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return fmt.Errorf("age encrypt write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("age encrypt close: %w", err)
	}
	if err := aw.Close(); err != nil {
		return fmt.Errorf("armor close: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// GenerateKey generates a new age key at path. No-op if the file already exists.
func GenerateKey(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return fmt.Errorf("generating age key: %w", err)
	}
	content := fmt.Sprintf(
		"# created: %s\n# public key: %s\n%s\n",
		time.Now().UTC().Format(time.RFC3339),
		id.Recipient().String(),
		id.String(),
	)
	return os.WriteFile(path, []byte(content), 0600)
}

// PublicKeyFrom reads the public key from an age key file.
func PublicKeyFrom(keyFile string) (string, error) {
	f, err := os.Open(keyFile)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", keyFile, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := scanner.Text(); strings.HasPrefix(line, "# public key: ") {
			return strings.TrimPrefix(line, "# public key: "), nil
		}
	}
	return "", fmt.Errorf("public key not found in %s", keyFile)
}

// isArmored returns true when data begins with the age armor header.
func isArmored(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte("-----BEGIN AGE ENCRYPTED FILE-----"))
}

// loadIdentity parses the age private key from keyFile.
func loadIdentity(keyFile string) (age.Identity, error) {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("reading key %s: %w", keyFile, err)
	}
	ids, err := age.ParseIdentities(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parsing identity from %s: %w", keyFile, err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no identity found in %s", keyFile)
	}
	return ids[0], nil
}

// loadRecipient derives the public key recipient from the private key in keyFile.
func loadRecipient(keyFile string) (age.Recipient, error) {
	id, err := loadIdentity(keyFile)
	if err != nil {
		return nil, err
	}
	x, ok := id.(*age.X25519Identity)
	if !ok {
		return nil, fmt.Errorf("key in %s is not an X25519 identity", keyFile)
	}
	return x.Recipient(), nil
}
