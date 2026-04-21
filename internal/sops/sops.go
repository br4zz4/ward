package sops

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SopsDecryptor decrypts and re-encrypts files using the sops CLI with an age key file.
type SopsDecryptor struct {
	KeyFile string // path to the age key file (e.g. .ward.key)
}

func (d SopsDecryptor) Decrypt(path string) ([]byte, error) {
	cmd := exec.Command("sops", "decrypt", "--input-type", "yaml", "--output-type", "yaml", path)
	cmd.Env = append(cmd.Environ(), "SOPS_AGE_KEY_FILE="+d.KeyFile)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sops decrypt %s: %w\n%s", path, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func (d SopsDecryptor) Encrypt(path string, plaintext []byte) error {
	pubKey, err := readAgePublicKey(d.KeyFile)
	if err != nil {
		return err
	}

	cmd := exec.Command("sops", "encrypt",
		"--age", pubKey,
		"--input-type", "yaml",
		"--output-type", "yaml",
		"/dev/stdin",
	)
	cmd.Stdin = bytes.NewReader(plaintext)
	cmd.Env = append(os.Environ(), "SOPS_AGE_KEY_FILE="+d.KeyFile)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sops encrypt %s: %w\n%s", path, err, stderr.String())
	}
	return os.WriteFile(path, stdout.Bytes(), 0644)
}

func readAgePublicKey(keyFile string) (string, error) {
	f, err := os.Open(keyFile)
	if err != nil {
		return "", fmt.Errorf("reading key file %s: %w", keyFile, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# public key: ") {
			return strings.TrimPrefix(line, "# public key: "), nil
		}
	}
	return "", fmt.Errorf("public key not found in %s", keyFile)
}
