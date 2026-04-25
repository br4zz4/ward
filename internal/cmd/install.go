package cmd

import (
	"bufio"
	"encoding/json"
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
)

var pluginFiles = []struct {
	remote string
	local  string
}{
	{".claude-plugin/plugin.json", ".claude-plugin/plugin.json"},
	{"CLAUDE.md", "CLAUDE.md"},
	{"skills/ward:workspace/SKILL.md", "skills/ward:workspace/SKILL.md"},
}

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
			baseDir := promptInstallDir()
			pluginDir := filepath.Join(baseDir, "plugins", "ward")
			installPlugin(pluginDir)
			registerMarketplace(baseDir)
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

func installPlugin(pluginDir string) {
	for _, f := range pluginFiles {
		url := pluginBaseURL + "/" + f.remote
		dest := filepath.Join(pluginDir, f.local)
		if err := downloadFile(url, dest); err != nil {
			fatal(fmt.Errorf("failed to download %s: %w", f.remote, err))
		}
		fmt.Printf("  %s✓%s %s\n", clrGreen, clrReset, dest)
	}
}

func registerMarketplace(baseDir string) {
	settingsPath := filepath.Join(baseDir, "settings.json")

	var settings map[string]any
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = map[string]any{}
	}

	marketplaces, _ := settings["extraKnownMarketplaces"].(map[string]any)
	if marketplaces == nil {
		marketplaces = map[string]any{}
	}

	pluginsDir := filepath.Join(baseDir, "plugins")
	marketplaces["br4zz4"] = map[string]any{
		"source": "directory",
		"path":   pluginsDir,
	}
	settings["extraKnownMarketplaces"] = marketplaces

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fatal(fmt.Errorf("could not marshal settings: %w", err))
	}
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		fatal(fmt.Errorf("could not write settings: %w", err))
	}

	fmt.Printf("  %s✓%s marketplace registered\n", clrGreen, clrReset)
	fmt.Printf("\n  %sward Claude plugin installed.%s\n", clrBold, clrReset)
	fmt.Printf("  %sRun /plugin install ward@br4zz4 in Claude Code to activate.%s\n\n", clrGray, clrReset)
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
