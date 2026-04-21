package cmd_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var testBin string

func TestMain(m *testing.M) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	bin := filepath.Join(os.TempDir(), "ward-test")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/ward")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("build failed: " + err.Error() + "\n" + string(out))
	}
	testBin = bin
	code := m.Run()
	os.Remove(bin)
	os.Exit(code)
}

// buildBin returns the pre-built ward binary path (built once in TestMain).
func buildBin(t *testing.T) string {
	t.Helper()
	return testBin
}

// run executes ward with the given args from the test/fixtures/plain directory.
func run(t *testing.T, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "..", "..", "test", "fixtures", "plain")

	cmd := exec.Command(bin, args...)
	cmd.Dir = testdata

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), exitErr.ExitCode()
		}
	}
	return outBuf.String(), errBuf.String(), 0
}

func TestCmd_get_leaf(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin, "get", "company.name")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "acme") {
		t.Errorf("expected acme, got: %q", out)
	}
}

func TestCmd_get_staging_database_url(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"get", "company.sectors.one.staging.database_url",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "staging") {
		t.Errorf("expected staging url, got: %q", out)
	}
}

func TestCmd_view_subtree(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"view", "company.sectors.one.staging",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "database_url") {
		t.Errorf("expected database_url in output, got: %q", out)
	}
	if !strings.Contains(out, "staging.ward") {
		t.Errorf("expected file origin in output, got: %q", out)
	}
}

func TestCmd_exec_injects_env_vars(t *testing.T) {
	bin := buildBin(t)
	// Use --prefixed to avoid flat-name collisions across sectors
	out, _, code := run(t, bin,
		"exec", "--prefixed",
		"--", "env",
	)
	if code != 0 {
		t.Fatalf("exit %d\nstdout: %s", code, out)
	}
	if !strings.Contains(out, "DATABASE_URL=") {
		t.Errorf("expected DATABASE_URL injected, got: %q", out)
	}
}

func TestCmd_exec_no_production_leakage(t *testing.T) {
	bin := buildBin(t)
	// With --prefixed, staging and production vars have distinct names
	out, _, code := run(t, bin,
		"exec", "--prefixed",
		"--", "env",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, line := range strings.Split(out, "\n") {
		// With prefixed mode, staging vars are COMPANY_SECTORS_ONE_STAGING_*
		// production vars are COMPANY_SECTORS_ONE_PRODUCTION_* — no leakage
		if strings.Contains(line, "STAGING") && strings.Contains(line, "production") {
			t.Errorf("production value in staging var: %s", line)
		}
	}
}

func TestCmd_get_ancestor_value(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"get", "company.name",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "acme") {
		t.Errorf("expected ancestor value acme, got: %q", out)
	}
}

func TestCmd_get_sector_two_staging(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"get", "company.sectors.two.staging.database_url",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "sector2") {
		t.Errorf("expected sector2 url, got: %q", out)
	}
}

func TestCmd_sector_one_does_not_leak_into_sector_two(t *testing.T) {
	bin := buildBin(t)
	// With --prefixed, sector vars have full path names — no collision possible
	out, _, code := run(t, bin,
		"exec", "--prefixed",
		"--", "env",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, line := range strings.Split(out, "\n") {
		// sector one staging values should not appear under sector two keys
		if strings.Contains(line, "SECTORS_TWO") && strings.Contains(line, "sector 1") {
			t.Errorf("sector one value leaked into sector two var: %s", line)
		}
	}
}

func TestCmd_infra_available(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"get", "infra.region",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "us-east-1") {
		t.Errorf("expected us-east-1, got: %q", out)
	}
}

func TestCmd_exec_envs_equivalence(t *testing.T) {
	bin := buildBin(t)

	execOut, _, code := run(t, bin, "exec", "--prefixed", "--", "./print-envs.sh")
	if code != 0 {
		t.Fatalf("exec exit %d\nstdout: %s", code, execOut)
	}

	envsOut, _, code := run(t, bin, "envs", "--prefixed")
	if code != 0 {
		t.Fatalf("envs exit %d\nstdout: %s", code, envsOut)
	}

	execVars := map[string]string{}
	for _, line := range strings.Split(execOut, "\n") {
		if idx := strings.IndexByte(line, '='); idx > 0 {
			k := line[:idx]
			if strings.ToUpper(k) == k && strings.ContainsAny(k, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
				execVars[k] = line[idx+1:]
			}
		}
	}

	ansiStrip := func(s string) string {
		var out []byte
		inEsc := false
		for i := 0; i < len(s); i++ {
			if s[i] == '\033' {
				inEsc = true
			}
			if inEsc {
				if s[i] == 'm' {
					inEsc = false
				}
				continue
			}
			out = append(out, s[i])
		}
		return string(out)
	}

	envsVars := map[string]string{}
	for _, line := range strings.Split(envsOut, "\n") {
		clean := strings.TrimSpace(ansiStrip(line))
		if idx := strings.Index(clean, "  =  "); idx > 0 {
			k := strings.TrimSpace(clean[:idx])
			rest := strings.TrimSpace(clean[idx+5:])
			// strip trailing file:line origin (separated by multiple spaces)
			if i := strings.Index(rest, "  "); i > 0 {
				rest = strings.TrimSpace(rest[:i])
			}
			if rest == "" {
				continue
			}
			envsVars[k] = rest
		}
	}

	if len(envsVars) == 0 {
		t.Fatal("ward envs returned no vars")
	}

	for k, v := range envsVars {
		got, ok := execVars[k]
		if !ok {
			t.Errorf("key %s from ward envs not found in exec environment", k)
			continue
		}
		if got != v {
			t.Errorf("key %s: envs=%q exec=%q", k, v, got)
		}
	}
}

func TestCmd_init_creates_files(t *testing.T) {
	bin := buildBin(t)
	dir := t.TempDir()

	cmd := exec.Command(bin, "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %s\n%s", err, out)
	}

	if _, err := os.Stat(filepath.Join(dir, ".ward", "config.yaml")); err != nil {
		t.Error(".ward/config.yaml not created")
	}
	if _, err := os.Stat(filepath.Join(dir, ".ward", "vault", "secrets.ward")); err != nil {
		t.Error(".ward/vault/secrets.ward not created")
	}
}
