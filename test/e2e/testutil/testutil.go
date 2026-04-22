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
func Run(t *testing.T, bin, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	c := exec.Command(bin, args...)
	c.Dir = dir
	c.Stdout = &outBuf
	c.Stderr = &errBuf
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
	err := c.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

// RunCmd runs an arbitrary OS command and returns exit code.
func RunCmd(t *testing.T, name string, args ...string) int {
	t.Helper()
	c := exec.Command(name, args...)
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
