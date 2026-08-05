//go:build e2e

package secrets_test

import (
	"os"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

var bin string

func TestMain(m *testing.M) {
	b, err := testutil.BuildBin()
	if err != nil {
		panic(err)
	}
	bin = b
	code := m.Run()
	os.Remove(b)
	os.Exit(code)
}

func fix(name string) string { return testutil.FixtureDir("secrets", name) }

func TestSecrets_lists_key(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "secrets")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	clean := testutil.StripANSI(out)
	if !testutil.Contains(clean, "secret_key") {
		t.Errorf("expected secret_key in output, got: %q", out)
	}
	if !testutil.Contains(clean, "abc123") {
		t.Errorf("expected value abc123, got: %q", out)
	}
}

func TestEnvs_alias_still_works_and_warns(t *testing.T) {
	out, stderr, code := testutil.Run(t, bin, fix("basic"), "envs")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(out), "secret_key") {
		t.Errorf("expected secret_key in output, got: %q", out)
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "deprecated") {
		t.Errorf("expected deprecation notice on stderr, got: %q", stderr)
	}
}
