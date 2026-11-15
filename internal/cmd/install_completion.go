package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	shellFish = "fish"
	shellBash = "bash"
	shellZsh  = "zsh"
)

func newInstallCompletionCmd() *cobra.Command {
	var (
		output   string
		toStdout bool
	)

	c := &cobra.Command{
		Use:       "completion [zsh|bash|fish]",
		Short:     "Install shell completion for ward",
		Long:      "Install shell completion for ward. The shell is taken from the argument, or\ndetected from $SHELL (zsh when detection fails).",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{shellZsh, shellBash, shellFish},
		Run: func(c *cobra.Command, args []string) {
			out := c.OutOrStdout()
			shell, err := resolveShell(args)
			if err != nil {
				fatal(err)
			}
			if toStdout {
				if err := writeCompletion(c.Root(), shell, out); err != nil {
					fatal(err)
				}
				return
			}
			path, err := completionTarget(shell, output)
			if err != nil {
				fatal(err)
			}
			if err := writeCompletionFile(c.Root(), shell, path); err != nil {
				fatal(err)
			}
			fmt.Fprintf(out, "  %s✓%s %s\n", clrGreen, clrReset, path)
			printCompletionHint(out, shell, path)
		},
	}

	c.Flags().StringVarP(&output, "output", "o", "", "write the completion file into this directory")
	c.Flags().BoolVar(&toStdout, "stdout", false, "print the completion script instead of writing it")
	return c
}

func resolveShell(args []string) (string, error) {
	if len(args) == 1 {
		return normalizeShell(args[0])
	}
	shell, err := normalizeShell(filepath.Base(os.Getenv("SHELL")))
	if err == nil {
		return shell, nil
	}
	return shellZsh, nil
}

func normalizeShell(name string) (string, error) {
	switch name {
	case shellZsh, shellBash, shellFish:
		return name, nil
	}
	return "", fmt.Errorf("unsupported shell %q — use zsh, bash or fish", name)
}

func completionTarget(shell, dir string) (string, error) {
	if dir != "" {
		return filepath.Join(expandHome(dir), completionFileName(shell)), nil
	}
	switch shell {
	case shellZsh:
		if fpathDir := zshFpathDir(); fpathDir != "" {
			return filepath.Join(fpathDir, "_ward"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine home directory: %w", err)
		}
		return filepath.Join(home, ".zfunc", "_ward"), nil
	case shellBash:
		return filepath.Join(xdgHome("XDG_DATA_HOME", ".local/share"), "bash-completion", "completions", "ward"), nil
	case shellFish:
		return filepath.Join(xdgHome("XDG_CONFIG_HOME", ".config"), "fish", "completions", "ward.fish"), nil
	}
	return "", fmt.Errorf("unsupported shell %q", shell)
}

func completionFileName(shell string) string {
	switch shell {
	case shellZsh:
		return "_ward"
	case shellFish:
		return "ward.fish"
	}
	return "ward"
}

func writeCompletion(root *cobra.Command, shell string, w io.Writer) error {
	switch shell {
	case shellZsh:
		return root.GenZshCompletion(w)
	case shellBash:
		return root.GenBashCompletionV2(w, true)
	case shellFish:
		return root.GenFishCompletion(w, true)
	}
	return fmt.Errorf("unsupported shell %q", shell)
}

func writeCompletionFile(root *cobra.Command, shell, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	defer f.Close()
	if err := writeCompletion(root, shell, f); err != nil {
		return fmt.Errorf("could not write %s: %w", path, err)
	}
	return nil
}

func printCompletionHint(w io.Writer, shell, path string) {
	if shell == shellZsh && strings.Contains(filepath.ToSlash(path), "/.zfunc/") {
		fmt.Fprintf(w, "\n  %sadd this to ~/.zshrc before compinit:%s\n", clrGray, clrReset)
		fmt.Fprintf(w, "    fpath=($HOME/.zfunc $fpath)\n")
		fmt.Fprintln(w)
		return
	}
	fmt.Fprintf(w, "  %srestart your shell to activate it%s\n\n", clrGray, clrReset)
}

func zshFpathDir() string {
	out, err := exec.Command(shellZsh, "-c", "print -l -- $fpath").Output()
	if err != nil {
		return ""
	}
	for _, dir := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if isStandardCompletionDir(dir) && isWritableDir(dir) {
			return dir
		}
	}
	return ""
}

func isStandardCompletionDir(dir string) bool {
	slashed := filepath.ToSlash(dir)
	for _, suffix := range []string{"/share/zsh/site-functions", "/share/zsh/vendor-completions"} {
		if strings.HasSuffix(slashed, suffix) {
			return true
		}
	}
	return false
}

func isWritableDir(dir string) bool {
	f, err := os.CreateTemp(dir, ".ward-completion-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

func xdgHome(env, fallback string) string {
	if dir := os.Getenv(env); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fallback
	}
	if fallback == "" {
		return home
	}
	return filepath.Join(home, fallback)
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}
