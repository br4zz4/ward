//go:build e2e

package sops_test

import (
	"os"
	"testing"

	"github.com/oporpino/ward/test/e2e/testutil"
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

func fix(name string) string { return testutil.FixtureDir("sops", name) }

func TestSops_raw_decrypts_root_file(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "raw", "secrets/company.ward")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"name", "acme", "api_key"} {
		if !testutil.Contains(out, want) {
			t.Errorf("expected %q in output, got: %q", want, out)
		}
	}
}

func TestSops_raw_decrypts_nested_file(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "raw", "secrets/company/sectors/one/staging.ward")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"database_url", "postgres://staging.acme.internal/app", "api_key"} {
		if !testutil.Contains(out, want) {
			t.Errorf("expected %q in output, got: %q", want, out)
		}
	}
}

func TestSops_get_merges_hierarchy(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "get", "company.name")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "acme") {
		t.Errorf("expected 'acme', got: %q", out)
	}
}

func TestSops_get_deep_value(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fix("basic"), "get", "company.sectors.one.staging.database_url")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !testutil.Contains(out, "postgres://staging.acme.internal/app") {
		t.Errorf("expected database_url, got: %q", out)
	}
}
