package cmd

import (
	"bufio"
	"encoding/json"
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
	pluginBaseURL   = "https://raw.githubusercontent.com/br4zz4/ai/main/providers/claude/plugins/ward"
	pluginName      = "br4zz4:ward"
	marketplaceName = "br4zz4"
)

var pluginFiles = []string{
	".claude-plugin/plugin.json",
	"CLAUDE.md",
	"skills/workspace/SKILL.md",
}

func NewInstallCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:   "install",
		Short: "Install integrations and plugins.",
	}
	parent.AddCommand(newInstallClaudePluginCmd())
	return parent
}

func NewUninstallCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall integrations and plugins.",
	}
	parent.AddCommand(newUninstallClaudePluginCmd())
	return parent
}

func newInstallClaudePluginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "claude-plugin",
		Short: "Install the ward Claude Code plugin.",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			claudeDir, scope := promptInstallTarget()
			pluginDir := filepath.Join(claudeDir, "plugins", pluginName)
			downloadPluginFiles(pluginDir)
			registerMarketplace(claudeDir)
			installPlugin(claudeDir, scope)
		},
	}
}

func newUninstallClaudePluginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "claude-plugin",
		Short: "Uninstall the ward Claude Code plugin.",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			claudeDir, _ := promptInstallTarget()
			uninstallPlugin(claudeDir)
		},
	}
}

func promptInstallTarget() (claudeDir, scope string) {
	fmt.Printf("\n  %sward Claude plugin%s\n\n", clrBold, clrReset)
	fmt.Printf("  Where is it installed?\n")
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

func downloadPluginFiles(pluginDir string) {
	for _, f := range pluginFiles {
		url := pluginBaseURL + "/" + f
		dest := filepath.Join(pluginDir, f)
		if err := downloadFile(url, dest); err != nil {
			fatal(fmt.Errorf("failed to download %s: %w", f, err))
		}
		fmt.Printf("  %s✓%s %s\n", clrGreen, clrReset, dest)
	}
}

func registerMarketplace(claudeDir string) {
	settingsPath := filepath.Join(claudeDir, "settings.json")
	pluginsDir := filepath.Join(claudeDir, "plugins")

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil || settings == nil {
		settings = map[string]any{}
	}

	marketplaces, _ := settings["extraKnownMarketplaces"].(map[string]any)
	if marketplaces == nil {
		marketplaces = map[string]any{}
	}
	marketplaces[marketplaceName] = map[string]any{
		"source": map[string]any{
			"source": "directory",
			"path":   pluginsDir,
		},
		"autoUpdate": true,
	}
	settings["extraKnownMarketplaces"] = marketplaces

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fatal(fmt.Errorf("could not marshal settings: %w", err))
	}
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		fatal(fmt.Errorf("could not write settings: %w", err))
	}
	fmt.Printf("  %s✓%s marketplace %s registered\n", clrGreen, clrReset, marketplaceName)
}

func isPluginInstalled() bool {
	ref := pluginName + "@" + marketplaceName
	out, err := exec.Command("claude", "plugin", "list").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), ref)
}

func installPlugin(claudeDir, scope string) {
	ref := pluginName + "@" + marketplaceName

	if isPluginInstalled() {
		out, err := exec.Command("claude", "plugin", "update", ref).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s!%s plugin update: %s\n", clrLightRed, clrReset, strings.TrimSpace(string(out)))
			return
		}
		fmt.Printf("  %s✓%s %s updated\n", clrGreen, clrReset, ref)
	} else {
		out, err := exec.Command("claude", "plugin", "install", "--scope", scope, ref).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s!%s plugin install: %s\n", clrLightRed, clrReset, strings.TrimSpace(string(out)))
			fmt.Fprintf(os.Stderr, "  %sRun manually: /plugin install %s%s\n\n", clrGray, ref, clrReset)
			return
		}
		fmt.Printf("  %s✓%s %s installed\n", clrGreen, clrReset, ref)
	}

	fmt.Printf("\n  %sward Claude plugin ready.%s\n\n", clrBold, clrReset)
}

func uninstallPlugin(claudeDir string) {
	ref := pluginName + "@" + marketplaceName

	out, err := exec.Command("claude", "plugin", "uninstall", ref).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s!%s plugin uninstall: %s\n", clrLightRed, clrReset, strings.TrimSpace(string(out)))
	} else {
		fmt.Printf("  %s✓%s %s uninstalled\n", clrGreen, clrReset, ref)
	}

	pluginDir := filepath.Join(claudeDir, "plugins", pluginName)
	if err := os.RemoveAll(pluginDir); err != nil {
		fmt.Fprintf(os.Stderr, "  %s!%s remove plugin dir: %v\n", clrLightRed, clrReset, err)
	} else {
		fmt.Printf("  %s✓%s plugin files removed\n", clrGreen, clrReset)
	}

	cacheDir := filepath.Join(claudeDir, "plugins", "cache", marketplaceName)
	if err := os.RemoveAll(cacheDir); err != nil {
		fmt.Fprintf(os.Stderr, "  %s!%s remove cache dir: %v\n", clrLightRed, clrReset, err)
	} else {
		fmt.Printf("  %s✓%s cache removed\n", clrGreen, clrReset)
	}

	deregisterMarketplace(claudeDir)

	fmt.Printf("\n  %sward Claude plugin removed.%s\n\n", clrBold, clrReset)
}

func deregisterMarketplace(claudeDir string) {
	settingsPath := filepath.Join(claudeDir, "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return
	}

	if marketplaces, ok := settings["extraKnownMarketplaces"].(map[string]any); ok {
		delete(marketplaces, marketplaceName)
	}
	if enabled, ok := settings["enabledPlugins"].(map[string]any); ok {
		delete(enabled, pluginName+"@"+marketplaceName)
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(settingsPath, out, 0o644)
	fmt.Printf("  %s✓%s marketplace %s removed\n", clrGreen, clrReset, marketplaceName)
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
