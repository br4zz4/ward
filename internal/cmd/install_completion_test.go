package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newCompletionRoot() *cobra.Command {
	root := &cobra.Command{Use: "ward"}
	root.AddCommand(&cobra.Command{Use: "get", Run: func(*cobra.Command, []string) {}})
	return root
}

func newCompletionRootWithInstall() (*cobra.Command, *bytes.Buffer) {
	root := newCompletionRoot()
	root.AddCommand(newInstallCompletionCmd())
	var out bytes.Buffer
	root.SetOut(&out)
	return root, &out
}

func TestResolveShell_from_env(t *testing.T) {
	tests := []struct {
		name  string
		shell string
		want  string
	}{
		{"zsh", "/bin/zsh", "zsh"},
		{"bash", "/usr/bin/bash", "bash"},
		{"fish", "/usr/local/bin/fish", "fish"},
		{"unknown falls back to zsh", "/bin/tcsh", "zsh"},
		{"empty falls back to zsh", "", "zsh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SHELL", tt.shell)
			got, err := resolveShell(nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveShell(nil) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveShell_explicit_arg_wins_over_env(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	got, err := resolveShell([]string{"bash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "bash" {
		t.Errorf("resolveShell = %q, want bash", got)
	}
}

func TestResolveShell_unsupported_arg_errors(t *testing.T) {
	if _, err := resolveShell([]string{"tcsh"}); err == nil {
		t.Fatal("expected error for unsupported shell")
	}
}

func TestCompletionFileName(t *testing.T) {
	tests := map[string]string{
		"zsh":  "_ward",
		"bash": "ward",
		"fish": "ward.fish",
	}
	for shell, want := range tests {
		t.Run(shell, func(t *testing.T) {
			if got := completionFileName(shell); got != want {
				t.Errorf("completionFileName(%q) = %q, want %q", shell, got, want)
			}
		})
	}
}

func TestCompletionTarget_explicit_output(t *testing.T) {
	tests := map[string]string{
		"zsh":  "_ward",
		"bash": "ward",
		"fish": "ward.fish",
	}
	for shell, file := range tests {
		t.Run(shell, func(t *testing.T) {
			dir := t.TempDir()
			got, err := completionTarget(shell, dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := filepath.Join(dir, file); got != want {
				t.Errorf("completionTarget = %q, want %q", got, want)
			}
		})
	}
}

func TestCompletionTarget_explicit_output_expands_home(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := completionTarget("zsh", "~/foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(home, "foo", "_ward"); got != want {
		t.Errorf("completionTarget = %q, want %q", got, want)
	}
}

func TestCompletionTarget_bash_uses_xdg_data_home(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	got, err := completionTarget("bash", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(tmp, "bash-completion", "completions", "ward"); got != want {
		t.Errorf("completionTarget = %q, want %q", got, want)
	}
}

func TestCompletionTarget_fish_uses_xdg_config_home(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	got, err := completionTarget("fish", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(tmp, "fish", "completions", "ward.fish"); got != want {
		t.Errorf("completionTarget = %q, want %q", got, want)
	}
}

func TestCompletionTarget_bash_without_xdg_uses_home(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", home)
	got, err := completionTarget("bash", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(home, ".local/share", "bash-completion", "completions", "ward"); got != want {
		t.Errorf("completionTarget = %q, want %q", got, want)
	}
}

func TestWriteCompletion_contains_shell_markers(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{"zsh", "compdef _ward ward"},
		{"bash", "bash completion"},
		{"fish", "fish completion"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeCompletion(newCompletionRoot(), tt.shell, &buf); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(buf.String(), tt.want) {
				t.Errorf("output missing %q", tt.want)
			}
		})
	}
}

func TestWriteCompletion_invalid_shell_errors(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCompletion(newCompletionRoot(), "tcsh", &buf); err == nil {
		t.Fatal("expected error for invalid shell")
	}
}

func TestWriteCompletionFile_creates_dirs_and_matches_stdout(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		t.Run(shell, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a", "b", completionFileName(shell))
			var want bytes.Buffer
			if err := writeCompletion(newCompletionRoot(), shell, &want); err != nil {
				t.Fatalf("writeCompletion: %v", err)
			}
			if err := writeCompletionFile(newCompletionRoot(), shell, path); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if !bytes.Equal(got, want.Bytes()) {
				t.Error("file content differs from writeCompletion output")
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if perm := info.Mode().Perm(); perm != 0o644 {
				t.Errorf("perm = %o, want 644", perm)
			}
		})
	}
}

func TestWriteCompletionFile_parent_is_file_errors(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeCompletionFile(newCompletionRoot(), "zsh", filepath.Join(blocker, "sub", "_ward"))
	if err == nil {
		t.Fatal("expected error when parent is a file")
	}
}

func TestInstallCompletionCmd_writes_file_to_output(t *testing.T) {
	tmp := t.TempDir()
	root, out := newCompletionRootWithInstall()
	root.SetArgs([]string{"completion", "zsh", "--output", tmp})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "_ward")); err != nil {
		t.Errorf("expected _ward to exist: %v", err)
	}
	if !strings.Contains(out.String(), "✓") {
		t.Errorf("output missing ✓: %q", out.String())
	}
}

func TestInstallCompletionCmd_short_output_flag(t *testing.T) {
	tmp := t.TempDir()
	root, _ := newCompletionRootWithInstall()
	root.SetArgs([]string{"completion", "fish", "-o", tmp})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "ward.fish")); err != nil {
		t.Errorf("expected ward.fish to exist: %v", err)
	}
}

func TestInstallCompletionCmd_stdout_does_not_create_file(t *testing.T) {
	tmp := t.TempDir()
	root, out := newCompletionRootWithInstall()
	root.SetArgs([]string{"completion", "zsh", "--output", tmp, "--stdout"})
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "_ward")); !os.IsNotExist(err) {
		t.Errorf("expected no file, stat err = %v", err)
	}
	if !strings.Contains(out.String(), "#compdef ward") {
		t.Errorf("output missing #compdef ward: %q", out.String())
	}
}

func TestInstallCompletionCmd_valid_args(t *testing.T) {
	c := newInstallCompletionCmd()
	for _, shell := range []string{"zsh", "bash", "fish"} {
		found := false
		for _, v := range c.ValidArgs {
			if v == shell {
				found = true
			}
		}
		if !found {
			t.Errorf("ValidArgs missing %q", shell)
		}
	}
}

func TestIsStandardCompletionDir(t *testing.T) {
	tests := []struct {
		dir  string
		want bool
	}{
		{"/opt/homebrew/share/zsh/site-functions", true},
		{"/usr/share/zsh/vendor-completions", true},
		{"/Users/x/.oh-my-zsh/plugins/git", false},
		{"/Users/x/.oh-my-zsh/custom/completions", false},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			if got := isStandardCompletionDir(tt.dir); got != tt.want {
				t.Errorf("isStandardCompletionDir(%q) = %v, want %v", tt.dir, got, tt.want)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := []struct {
		in   string
		want string
	}{
		{"~", home},
		{"~/x", filepath.Join(home, "x")},
		{"/abs", "/abs"},
		{"rel", "rel"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := expandHome(tt.in); got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
