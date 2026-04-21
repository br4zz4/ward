package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	wardage "github.com/oporpino/ward/internal/age"
)

// wardConfigTemplate is the minimal config written by ward init.
// merge is intentionally omitted — the default (deep merge) is used automatically.
const wardConfigTemplate = `encryption:
  key_file: .ward.key

sources:
  - path: ./.ward/vault
`

func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize ward in the current directory",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			// 1. Generate age key
			if err := wardage.GenerateKey(".ward.key"); err != nil {
				fatal(err)
			}

			// 2. Create .ward/ directory and config
			if err := os.MkdirAll(".ward", 0755); err != nil {
				fatal(fmt.Errorf("creating .ward/: %w", err))
			}
			if err := writeIfAbsent(".ward/config.yaml", wardConfigTemplate); err != nil {
				fatal(err)
			}

			// 3. Add .ward.key to .gitignore
			if err := ensureGitignore(".ward.key"); err != nil {
				fatal(err)
			}

			// 4. Create .ward/vault/ and encrypt the initial secrets file
			if err := os.MkdirAll(".ward/vault", 0755); err != nil {
				fatal(fmt.Errorf("creating .ward/vault/: %w", err))
			}
			dirName := currentDirName()
			stub := initSecretsStub(dirName)
			if err := encryptIfAbsent(".ward/vault/secrets.ward", stub, ".ward.key"); err != nil {
				fatal(err)
			}

			// 5. Print summary and WARD_KEY token
			token, err := encodeWardKey(".ward.key")
			if err == nil {
				fmt.Printf("\n  %s✓ ward is ready%s\n\n", clrGreen+clrBold, clrReset)
				fmt.Printf("  %s.ward/config.yaml%s    config — %scommit this%s\n", clrCyan, clrReset, clrGreen, clrReset)
				fmt.Printf("  %s.ward.key%s             age key — %skeep private, never commit%s\n", clrCyan, clrReset, clrOrange, clrReset)
				fmt.Printf("  %s.ward/vault/%s          encrypted secrets — %ssafe to commit%s\n", clrCyan, clrReset, clrGreen, clrReset)
				fmt.Printf("\n  %sWARD_KEY%s=%s%s%s\n", clrYellow, clrReset, clrGray, token, clrReset)
				fmt.Printf("  %s↑ copy this to CI / secrets manager%s\n", clrGray, clrReset)
				fmt.Printf("\n  %s─────────────────────────────────────%s\n\n", clrGray, clrReset)
				fmt.Printf("  %sedit secrets%s\n", clrBold, clrReset)
				fmt.Printf("    %sward edit%s\n\n", clrCyan, clrReset)
				fmt.Printf("  %screate a new secrets file%s\n", clrBold, clrReset)
				fmt.Printf("    %sward new staging%s\n\n", clrCyan, clrReset)
				fmt.Printf("  %sedit config%s\n", clrBold, clrReset)
				fmt.Printf("    %sward config%s\n\n", clrCyan, clrReset)
			}
		},
	}
}

// currentDirName returns the base name of the current working directory.
func currentDirName() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "app"
	}
	name := filepath.Base(cwd)
	if name == "." || name == "/" {
		return "app"
	}
	return name
}

// initSecretsStub returns a YAML stub using dirName as the root key.
func initSecretsStub(dirName string) string {
	return fmt.Sprintf("%s:\n  secret_1: <your content>\n  secret_2: <your content>\n", dirName)
}

// isGitRepo returns true if the current directory is inside a git repository.
func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// ensureGitignore adds entry to .gitignore if not already present and inside a git repo.
func ensureGitignore(entry string) error {
	if !isGitRepo() {
		return nil
	}
	const path = ".gitignore"
	data, _ := os.ReadFile(path)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
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
	return err
}

// encryptIfAbsent creates path by encrypting content with age+armor if it doesn't exist.
func encryptIfAbsent(path, content, keyFile string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return wardage.AgeArmorDecryptor{KeyFile: keyFile}.Encrypt(path, []byte(content))
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
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
