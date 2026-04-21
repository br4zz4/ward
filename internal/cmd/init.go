package cmd

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const wardYAMLTemplate = `encryption:
  engine: sops+age
  key_file: .ward.key

merge: merge

sources:
  - path: ./.secrets
`

const wardFileTemplate = `myapp:
  database_url: "postgres://localhost/myapp"
  redis_url: "redis://localhost:6379"
`

func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize ward: generate age key, ward.yaml, and an encrypted secrets file",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			// 1. Generate age key
			pubKey, err := generateAgeKey(".ward.key")
			if err != nil {
				fatal(err)
			}

			// 2. Create ward.yaml
			if err := writeIfAbsent("ward.yaml", wardYAMLTemplate); err != nil {
				fatal(err)
			}

			// 3. Add .ward.key to .gitignore
			if err := ensureGitignore(".ward.key"); err != nil {
				fatal(err)
			}

			// 4. Create .secrets/ and encrypt the initial .ward file
			if err := os.MkdirAll(".secrets", 0755); err != nil {
				fatal(err)
			}
			if err := encryptIfAbsent(".secrets/.ward", wardFileTemplate, ".ward.key", pubKey); err != nil {
				fatal(err)
			}

			// 5. Print WARD_KEY token for CI
			token, err := encodeWardKey(".ward.key")
			if err == nil {
				fmt.Printf("\n%s  ward is ready%s\n\n", clrGreen+clrBold, clrReset)
				fmt.Printf("  %s.ward.key%s     age key — %skeep private, never commit%s\n", clrCyan, clrReset, clrOrange, clrReset)
				fmt.Printf("  %sward.yaml%s      config — commit this\n", clrCyan, clrReset)
				fmt.Printf("  %s.secrets/%s      encrypted secrets — safe to commit\n\n", clrCyan, clrReset)
				fmt.Printf("  %sWARD_KEY%s=%s%s%s\n", clrYellow, clrReset, clrGray, token, clrReset)
				fmt.Printf("  %s↑ copy this to CI / secrets manager%s\n\n", clrGray, clrReset)
				fmt.Printf("  %snext:%s edit your first secrets file\n\n", clrGray, clrReset)
				fmt.Printf("    %sward edit .secrets/.ward%s\n\n", clrBold, clrReset)
			}
		},
	}
}

// generateAgeKey runs age-keygen and writes the key to path.
// Returns the public key. If the file already exists, reads the public key from it.
func generateAgeKey(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("ward: %s already exists, skipping key generation\n", path)
		return readAgePublicKey(path)
	}
	cmd := exec.Command("age-keygen", "-o", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("age-keygen: %w\n%s", err, stderr.String())
	}
	fmt.Printf("ward: generated %s\n", path)
	return readAgePublicKey(path)
}

// readAgePublicKey reads the public key comment from an age key file.
func readAgePublicKey(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# public key: ") {
			return strings.TrimPrefix(line, "# public key: "), nil
		}
	}
	return "", fmt.Errorf("public key not found in %s", path)
}

// ensureGitignore adds entry to .gitignore if not already present.
func ensureGitignore(entry string) error {
	const path = ".gitignore"
	data, _ := os.ReadFile(path)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil // already present
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer f.Close()
	prefix := ""
	if len(data) > 0 && data[len(data)-1] != '\n' {
		prefix = "\n"
	}
	_, err = fmt.Fprintf(f, "%s%s\n", prefix, entry)
	fmt.Printf("ward: added %s to %s\n", entry, path)
	return err
}

// encryptIfAbsent creates path by encrypting content with sops+age if it doesn't exist.
func encryptIfAbsent(path, content, keyFile, pubKey string) error {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("ward: %s already exists, skipping\n", path)
		return nil
	}
	cmd := exec.Command("sops", "encrypt",
		"--age", pubKey,
		"--input-type", "yaml",
		"--output-type", "yaml",
		"/dev/stdin",
	)
	cmd.Stdin = strings.NewReader(content)
	cmd.Env = append(os.Environ(), "SOPS_AGE_KEY_FILE="+keyFile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sops encrypt %s: %w\n%s", path, err, stderr.String())
	}
	if err := os.WriteFile(path, stdout.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("ward: created %s (encrypted)\n", path)
	return nil
}

// encodeWardKey reads a .ward.key file and returns a portable ward-<base64url> token.
func encodeWardKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "ward-" + base64.URLEncoding.EncodeToString(data), nil
}

// decodeWardKey decodes a ward-<base64url> token into age key file contents.
func decodeWardKey(token string) ([]byte, error) {
	token = strings.TrimPrefix(token, "ward-")
	return base64.URLEncoding.DecodeString(token)
}

func writeIfAbsent(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("ward: %s already exists, skipping\n", path)
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("ward: created %s\n", path)
	return nil
}
