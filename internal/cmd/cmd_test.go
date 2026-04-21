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

// buildBin compiles the ward binary once and returns its path.
func buildBin(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	bin := filepath.Join(t.TempDir(), "ward")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/ward")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}
	return bin
}

// run executes ward with the given args from the case1_unencrypted testdata directory.
func run(t *testing.T, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	testdata := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "case1_unencrypted")

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

func TestCmd_get_with_anchor_staging(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"get", "company.sectors.one.staging.database_url",
		"--anchor", "secrets/company/sectors/one/staging.ward",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "staging") {
		t.Errorf("expected staging url, got: %q", out)
	}
}

func TestCmd_get_staging_key_not_in_production(t *testing.T) {
	bin := buildBin(t)
	_, _, code := run(t, bin,
		"get", "company.sectors.one.staging",
		"--anchor", "secrets/company/sectors/one/production.ward",
	)
	if code == 0 {
		t.Error("expected non-zero exit: staging key should not exist under production anchor")
	}
}

func TestCmd_view_subtree_with_origin(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"view", "secrets/company/sectors/one/staging.ward",
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
	out, _, code := run(t, bin,
		"exec", "secrets/company/sectors/one/staging.ward",
		"--", "env",
	)
	if code != 0 {
		t.Fatalf("exit %d\nstdout: %s", code, out)
	}
	// With file anchor, exec strips the anchor's own level → DATABASE_URL not STAGING_DATABASE_URL
	if !strings.Contains(out, "DATABASE_URL=") {
		t.Errorf("expected DATABASE_URL injected, got: %q", out)
	}
	if strings.Contains(out, "STAGING_DATABASE_URL=") {
		t.Errorf("unexpected STAGING_DATABASE_URL — should be relative (DATABASE_URL), got: %q", out)
	}
}

func TestCmd_exec_no_production_leakage(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"exec", "secrets/company/sectors/one/staging.ward",
		"--", "env",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	// Ensure production vars are not present in staging exec
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "PRODUCTION_") {
			t.Errorf("production var leaked into staging exec: %s", line)
		}
	}
}

func TestCmd_get_ancestor_value(t *testing.T) {
	bin := buildBin(t)
	// company.name is defined in company.ward (ancestor) — should be visible with anchor
	out, _, code := run(t, bin,
		"get", "company.name",
		"--anchor", "secrets/company/sectors/one/staging.ward",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "acme") {
		t.Errorf("expected ancestor value acme, got: %q", out)
	}
}

func TestCmd_get_sector_two_with_anchor(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"get", "company.sectors.two.staging.database_url",
		"--anchor", "secrets/company/sectors/two/staging.ward",
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
	out, _, code := run(t, bin,
		"exec", "secrets/company/sectors/two/staging.ward",
		"--", "env",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "COMPANY_SECTORS_ONE_") {
			t.Errorf("sector one var leaked into sector two exec: %s", line)
		}
	}
}

func TestCmd_infra_excluded_from_company_anchor(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"exec", "secrets/company/sectors/one/staging.ward",
		"--", "env",
	)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	// infra.ward has a different root key — should not appear
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "INFRA_") {
			t.Errorf("infra var should not appear in company anchor exec: %s", line)
		}
	}
}

func TestCmd_infra_available_with_infra_anchor(t *testing.T) {
	bin := buildBin(t)
	out, _, code := run(t, bin,
		"get", "infra.region",
		"--anchor", "secrets/infra.ward",
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
	anchor := "secrets/company/sectors/one/staging.ward"

	// ward exec -- print-envs.sh: captures the injected env as KEY=value lines
	execOut, _, code := run(t, bin, "exec", anchor, "--", "./print-envs.sh")
	if code != 0 {
		t.Fatalf("exec exit %d\nstdout: %s", code, execOut)
	}

	// ward envs <anchor>: strip ANSI codes and parse KEY = value lines
	envsOut, _, code := run(t, bin, "envs", anchor)
	if code != 0 {
		t.Fatalf("envs exit %d\nstdout: %s", code, envsOut)
	}

	// Build set of KEY=value from exec output (only WARD-injected vars — uppercase, no PATH etc.)
	execVars := map[string]string{}
	for _, line := range strings.Split(execOut, "\n") {
		if idx := strings.IndexByte(line, '='); idx > 0 {
			k := line[:idx]
			if strings.ToUpper(k) == k && strings.ContainsAny(k, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
				execVars[k] = line[idx+1:]
			}
		}
	}

	// Parse ward envs output: strip ANSI, split on " = "
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
			v := strings.TrimSpace(clean[idx+5:])
			envsVars[k] = v
		}
	}

	if len(envsVars) == 0 {
		t.Fatal("ward envs returned no vars")
	}

	// Every key from ward envs must appear in exec output with the same value
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
	if _, err := exec.LookPath("age-keygen"); err != nil {
		t.Skip("age-keygen not in PATH")
	}
	if _, err := exec.LookPath("sops"); err != nil {
		t.Skip("sops not in PATH")
	}
	bin := buildBin(t)
	dir := t.TempDir()

	cmd := exec.Command(bin, "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %s\n%s", err, out)
	}

	if _, err := os.Stat(filepath.Join(dir, "ward.yaml")); err != nil {
		t.Error("ward.yaml not created")
	}
	if _, err := os.Stat(filepath.Join(dir, ".secrets", ".ward")); err != nil {
		t.Error(".secrets/.ward not created")
	}
}
