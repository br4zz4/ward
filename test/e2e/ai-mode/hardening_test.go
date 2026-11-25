//go:build e2e

package ai_mode_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

const hardeningSecret = "hunter2-plaintext-value"

type aiCase struct {
	name string
	env  string
	flag []string
}

var aiCases = []aiCase{
	{"strict", "strict", nil},
	{"masked", "masked", nil},
	{"flag", "0", []string{"--ai-mode"}},
}

func humanProject(t *testing.T) string {
	t.Helper()
	t.Setenv("WARD_AI_MODE", "0")
	dir := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatalf("init exit %d: %s", code, stderr)
	}
	if _, stderr, code := testutil.Run(t, bin, dir, "set", "proj:main.token", hardeningSecret); code != 0 {
		t.Fatalf("set exit %d: %s", code, stderr)
	}
	return dir
}

func markerEditor(t *testing.T) (marker string) {
	t.Helper()
	d := t.TempDir()
	marker = filepath.Join(d, "editor-ran")
	script := filepath.Join(d, "editor.sh")
	body := "#!/bin/sh\ncat \"$1\" 2>/dev/null\ntouch '" + marker + "'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", script)
	return marker
}

func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(filepath.Join(dir, ".ward"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		out[p] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertSameTree(t *testing.T, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Errorf("files under .ward changed: %d before, %d after", len(before), len(after))
	}
	for p, v := range after {
		old, ok := before[p]
		if !ok {
			t.Errorf("unexpected new file %s", p)
		} else if old != v {
			t.Errorf("file %s was altered", p)
		}
	}
}

func assertNoKeyLeak(t *testing.T, out, stderr string) {
	t.Helper()
	all := testutil.StripANSI(out + stderr)
	if strings.Contains(all, "ward-") {
		t.Errorf("key token (ward-...) leaked: %q", all)
	}
	if strings.Contains(all, "WARD_KEY_") && strings.Contains(all, "=") {
		for _, line := range strings.Split(all, "\n") {
			if strings.Contains(line, "WARD_KEY_") && strings.Contains(line, "=") {
				t.Errorf("WARD_KEY_...= line leaked: %q", line)
			}
		}
	}
}

func TestEdit_ai_mode_refuses_without_running_editor(t *testing.T) {
	for _, c := range aiCases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			dir := humanProject(t)
			file := filepath.Join(dir, ".ward", "vaults", "proj::default", "main.ward")
			marker := markerEditor(t)
			before := snapshot(t, dir)
			t.Setenv("WARD_AI_MODE", c.env)

			// act
			args := append([]string{"edit", file}, c.flag...)
			out, stderr, code := testutil.Run(t, bin, dir, args...)

			// assert
			if code != 3 {
				t.Errorf("expected exit 3, got %d (stderr %q)", code, stderr)
			}
			if _, err := os.Stat(marker); err == nil {
				t.Errorf("EDITOR must not run in AI mode")
			}
			if strings.Contains(out+stderr, hardeningSecret) {
				t.Errorf("plaintext leaked: %q", out+stderr)
			}
			assertSameTree(t, before, snapshot(t, dir))
		})
	}
}

func TestNew_ai_mode_refuses_without_running_editor(t *testing.T) {
	for _, c := range aiCases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			dir := humanProject(t)
			marker := markerEditor(t)
			before := snapshot(t, dir)
			t.Setenv("WARD_AI_MODE", c.env)

			// act
			args := append([]string{"new", "proj", "extra"}, c.flag...)
			out, stderr, code := testutil.RunWithStdinFull(t, bin, dir, "y\n", args...)

			// assert
			if code != 3 {
				t.Errorf("expected exit 3, got %d (stderr %q)", code, stderr)
			}
			if _, err := os.Stat(marker); err == nil {
				t.Errorf("EDITOR must not run in AI mode")
			}
			if strings.Contains(out+stderr, hardeningSecret) {
				t.Errorf("plaintext leaked: %q", out+stderr)
			}
			assertSameTree(t, before, snapshot(t, dir))
		})
	}
}

func TestVaultAdd_ai_mode_does_not_print_key(t *testing.T) {
	for _, c := range aiCases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			dir := humanProject(t)
			t.Setenv("WARD_AI_MODE", c.env)

			// act
			args := append([]string{"vault", "add", "other"}, c.flag...)
			out, stderr, code := testutil.Run(t, bin, dir, args...)

			// assert
			if code != 0 {
				t.Fatalf("vault add exit %d: %s", code, stderr)
			}
			assertNoKeyLeak(t, out, stderr)
			key, err := os.ReadFile(filepath.Join(dir, ".ward", "other.key"))
			if err != nil || len(key) == 0 {
				t.Fatalf("expected key file .ward/other.key written: %v", err)
			}
			if !strings.Contains(testutil.StripANSI(out+stderr), "other.key") {
				t.Errorf("output must say where the key was stored, got %q", out+stderr)
			}
		})
	}
}

func TestVaultAdd_human_mode_still_prints_key(t *testing.T) {
	// arrange
	dir := humanProject(t)

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "vault", "add", "other")

	// assert
	if code != 0 {
		t.Fatalf("vault add exit %d: %s", code, stderr)
	}
	if !strings.Contains(testutil.StripANSI(out), "WARD_KEY_OTHER=ward-") {
		t.Errorf("expected key line in human mode, got %q", out)
	}
}

func TestRotateKey_ai_mode_does_not_print_key(t *testing.T) {
	for _, c := range aiCases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			dir := humanProject(t)
			keyPath := filepath.Join(dir, ".ward", "proj::default.key")
			original, err := os.ReadFile(keyPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("WARD_AI_MODE", c.env)

			// act
			args := append([]string{"rotate-key", "proj"}, c.flag...)
			out, stderr, code := testutil.Run(t, bin, dir, args...)

			// assert
			if code != 0 {
				t.Fatalf("rotate-key exit %d: %s", code, stderr)
			}
			assertNoKeyLeak(t, out, stderr)
			rotated, err := os.ReadFile(keyPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(rotated) == string(original) {
				t.Errorf("key file was not rotated")
			}
			if !strings.Contains(testutil.StripANSI(out+stderr), "proj::default.key") {
				t.Errorf("output must say where the key was stored, got %q", out+stderr)
			}
		})
	}
}

func TestRotateKey_human_mode_still_prints_key(t *testing.T) {
	// arrange
	dir := humanProject(t)

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "rotate-key", "proj")

	// assert
	if code != 0 {
		t.Fatalf("rotate-key exit %d: %s", code, stderr)
	}
	if !strings.Contains(testutil.StripANSI(out+stderr), "WARD_KEY_PROJ=ward-") {
		t.Errorf("expected key line in human mode, got %q", out+stderr)
	}
}

func TestInit_ai_mode_does_not_print_key(t *testing.T) {
	for _, c := range aiCases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			t.Setenv("WARD_AI_MODE", c.env)
			dir := filepath.Join(t.TempDir(), "proj")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}

			// act
			args := append([]string{"init"}, c.flag...)
			out, stderr, code := testutil.Run(t, bin, dir, args...)

			// assert
			if code != 0 {
				t.Fatalf("init exit %d: %s", code, stderr)
			}
			assertNoKeyLeak(t, out, stderr)
			key, err := os.ReadFile(filepath.Join(dir, ".ward", "proj::default.key"))
			if err != nil || len(key) == 0 {
				t.Fatalf("expected key file written: %v", err)
			}
			if !strings.Contains(testutil.StripANSI(out+stderr), "proj::default.key") {
				t.Errorf("output must say where the key was stored, got %q", out+stderr)
			}
		})
	}
}
