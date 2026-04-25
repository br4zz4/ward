package cmd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	pluginBaseURL = "https://raw.githubusercontent.com/br4zz4/ai/main/providers/claude/plugins/ward"
	pluginFiles   = "CLAUDE.md"
	skillFile     = "skills/ward_debug.md"
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
			dir := promptInstallDir()
			installPlugin(dir)
		},
	}
}

func promptInstallDir() string {
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
		return filepath.Join(cwd, ".claude")
	case "2", "global", "g":
		home, err := os.UserHomeDir()
		if err != nil {
			fatal(fmt.Errorf("could not determine home directory: %w", err))
		}
		return filepath.Join(home, ".claude")
	default:
		fmt.Fprintf(os.Stderr, "\n  %sinvalid choice — enter 1 or 2%s\n", clrLightRed, clrReset)
		os.Exit(1)
		return ""
	}
}

func installPlugin(baseDir string) {
	files := map[string]string{
		pluginFiles: filepath.Join(baseDir, "CLAUDE.md"),
		skillFile:   filepath.Join(baseDir, skillFile),
	}

	for remote, local := range files {
		url := pluginBaseURL + "/" + remote
		if err := downloadFile(url, local); err != nil {
			fatal(fmt.Errorf("failed to download %s: %w", remote, err))
		}
		fmt.Printf("  %s✓%s %s\n", clrGreen, clrReset, local)
	}

	fmt.Printf("\n  %sward Claude plugin installed.%s\n", clrBold, clrReset)
	fmt.Printf("  %sRestart Claude Code to load the new context.%s\n\n", clrGray, clrReset)
}

func downloadFile(url, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("could not create directory: %w", err)
	}

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}

	return nil
}
