//go:build e2e

package ai_mode_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

func writeWrapper(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	script := "#!/bin/sh\n\"$WARD_BIN\" \"$@\"\nexit $?\n"
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod wrapper: %v", err)
	}
	return path
}

func cleanEnv(extra ...string) []string {
	env := make([]string, 0, len(os.Environ())+len(extra))
	for _, e := range os.Environ() {
		if k, _, _ := strings.Cut(e, "="); k == "WARD_AI_MODE" {
			continue
		}
		env = append(env, e)
	}
	return append(env, extra...)
}

func runViaWrapper(t *testing.T, wrapper, dir string, extraEnv []string, args ...string) (string, string, int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	c := exec.Command(wrapper, args...)
	c.Dir = dir
	c.Stdout = &outBuf
	c.Stderr = &errBuf
	c.Env = cleanEnv(append([]string{"WARD_BIN=" + bin}, extraEnv...)...)
	code := 0
	if err := c.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run wrapper: %v", err)
		}
		code = ee.ExitCode()
	}
	return outBuf.String(), errBuf.String(), code
}

func TestGet_auto_detects_agent_ancestor(t *testing.T) {
	// arrange
	wrapper := writeWrapper(t, "opencode")

	// act
	out, stderr, code := runViaWrapper(t, wrapper, treeFixture(), nil, "get", "app:main.name")

	// assert
	if code == 0 {
		t.Fatalf("expected refusal (non-zero exit), got 0")
	}
	if out != "" {
		t.Errorf("expected empty stdout in AI mode, got: %q", testutil.StripANSI(out))
	}
	clean := testutil.StripANSI(stderr)
	if !testutil.Contains(clean, "AI mode active") {
		t.Errorf("expected AI mode refusal on stderr, got: %q", clean)
	}
	if testutil.Contains(clean, "myapp") {
		t.Errorf("secret value leaked in AI mode: %q", clean)
	}
}

func TestGet_auto_detection_disabled_by_env(t *testing.T) {
	// arrange
	wrapper := writeWrapper(t, "opencode")

	// act
	out, stderr, code := runViaWrapper(t, wrapper, treeFixture(), []string{"WARD_AI_MODE=0"}, "get", "app:main.name")

	// assert
	if code != 0 {
		t.Fatalf("expected exit 0 with WARD_AI_MODE=0, got %d (stderr: %q)", code, testutil.StripANSI(stderr))
	}
	if !testutil.Contains(testutil.StripANSI(out), "myapp") {
		t.Errorf("expected value myapp on stdout, got: %q", testutil.StripANSI(out))
	}
}
