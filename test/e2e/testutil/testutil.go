//go:build e2e

// Package testutil provides shared helpers for e2e tests.
package testutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ProjectRoot returns the absolute path to the repository root.
func ProjectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	// file is .../test/e2e/testutil/testutil.go — go up 3 levels
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// FixtureDir returns the absolute path to test/e2e/<cmd>/fixtures/<name>.
func FixtureDir(cmd, name string) string {
	return filepath.Join(ProjectRoot(), "test", "e2e", cmd, "fixtures", name)
}

// Run executes the ward binary with the given working directory and args.
// Returns stdout, stderr, and exit code.
// Agent markers (OPENCODE_*, CLAUDE_*, ...) are stripped from the child
// environment so tests simulate a human terminal — unless the caller sets
// WARD_AI_MODE=1 explicitly (which the AI-mode tests do).
func Run(t *testing.T, bin, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	c := exec.Command(bin, args...)
	c.Dir = dir
	c.Stdout = &outBuf
	c.Stderr = &errBuf
	c.Env = stripAgentEnv(os.Environ())
	err := c.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	return
}

// stripAgentEnv removes agent-runtime markers from an environment slice so the
// ward child process does NOT auto-detect an AI agent (simulating a human).
// WARD_AI_MODE is preserved so AI-mode tests can force it on explicitly.
func stripAgentEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		k, _, _ := strings.Cut(e, "=")
		if isAgentMarker(k) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// isAgentMarker reports whether an env var name identifies an AI agent runtime.
// Kept in sync with the agent markers ward auto-detects (internal/cmd/ai_mode.go).
func isAgentMarker(k string) bool {
	if k == "OPENCODE" || strings.HasPrefix(k, "OPENCODE_") {
		return true
	}
	if strings.HasPrefix(k, "OPENCLAW_") {
		return true
	}
	if strings.HasPrefix(k, "CLAUDE_") || k == "DEFAULT_CLAUDE_MODEL" {
		return true
	}
	return false
}

// BuildBin builds the ward binary and returns its path.
// The binary is written to a temp file; the caller must remove it when done.
func BuildBin() (string, error) {
	bin, err := os.CreateTemp("", "ward-e2e-*")
	if err != nil {
		return "", err
	}
	bin.Close()
	path := bin.Name()
	cmd := exec.Command("go", "build", "-o", path, "./cmd/ward")
	cmd.Dir = ProjectRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("build ward: %w\n%s", err, out)
	}
	return path, nil
}

// Contains reports whether substr is in s.
func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// RunWithStdin executes the ward binary with stdin piped from input.
// Returns the exit code.
func RunWithStdin(t *testing.T, bin, dir, input string, args ...string) int {
	t.Helper()
	c := exec.Command(bin, args...)
	c.Dir = dir
	c.Stdin = strings.NewReader(input)
	c.Stdout = nil
	c.Stderr = nil
	c.Env = stripAgentEnv(os.Environ())
	err := c.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

// RunWithStdinFull executes the ward binary with stdin piped from input,
// capturing stdout, stderr and the exit code.
func RunWithStdinFull(t *testing.T, bin, dir, input string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	c := exec.Command(bin, args...)
	c.Dir = dir
	c.Stdin = strings.NewReader(input)
	c.Stdout = &outBuf
	c.Stderr = &errBuf
	c.Env = stripAgentEnv(os.Environ())
	err := c.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	return
}

// RunCmd runs an arbitrary OS command and returns exit code.
func RunCmd(t *testing.T, name string, args ...string) int {
	t.Helper()
	c := exec.Command(name, args...)
	c.Env = stripAgentEnv(os.Environ())
	err := c.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

// StripANSI removes ANSI escape codes from s.
func StripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		out.WriteByte(s[i])
	}
	return out.String()
}
