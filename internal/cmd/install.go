package cmd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	pluginBaseURL = "https://raw.githubusercontent.com/br4zz4/ai/main/providers/claude/plugins/ward"
)

func NewInstallCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:   "install",
		Short: "Install integrations and plugins.",
	}
	parent.AddCommand(newInstallClaudePluginCmd())
	return parent
}

func newInstallClaudePluginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "claude-plugin",
		Short: "Install the ward Claude Code plugin.",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			claudeDir, scope := promptInstallTarget()
			runInstallScript(claudeDir, scope)
		},
	}
}

func promptInstallTarget() (claudeDir, scope string) {
	fmt.Printf("\n  %sInstall ward Claude plugin%s\n\n", clrBold, clrReset)
	fmt.Printf("  Where do you want to install?\n")
	fmt.Printf("    %s1)%s project  %s(.claude/ in current directory)%s\n", clrCyan, clrReset, clrGray, clrReset)
	fmt.Printf("    %s2)%s global   %s(~/.claude/)%s\n", clrCyan, clrReset, clrGray, clrReset)
	fmt.Printf("\n  > ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())

	switch answer {
	case "1", "project", "p":
		cwd, err := os.Getwd()
		if err != nil {
			fatal(fmt.Errorf("could not determine current directory: %w", err))
		}
		return filepath.Join(cwd, ".claude"), "project"
	case "2", "global", "g":
		home, err := os.UserHomeDir()
		if err != nil {
			fatal(fmt.Errorf("could not determine home directory: %w", err))
		}
		return filepath.Join(home, ".claude"), "user"
	default:
		fmt.Fprintf(os.Stderr, "\n  %sinvalid choice — enter 1 or 2%s\n", clrLightRed, clrReset)
		os.Exit(1)
		return "", ""
	}
}

func runInstallScript(claudeDir, scope string) {
	tmp, err := os.CreateTemp("", "ward-install-*.sh")
	if err != nil {
		fatal(fmt.Errorf("could not create temp file: %w", err))
	}
	defer os.Remove(tmp.Name())

	if err := downloadTo(pluginBaseURL+"/install.sh", tmp); err != nil {
		fatal(fmt.Errorf("could not download install script: %w", err))
	}
	tmp.Close()

	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		fatal(fmt.Errorf("could not chmod install script: %w", err))
	}

	cmd := exec.Command("bash", tmp.Name(), claudeDir, scope)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal(fmt.Errorf("install script failed: %w", err))
	}
}

func downloadTo(url string, f *os.File) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}
	return nil
}
