package sops

import (
	"bytes"
	"fmt"
	"os/exec"
)

// SopsDecryptor decrypts files using the sops CLI with an age key file.
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
