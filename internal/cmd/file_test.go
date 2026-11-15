package cmd

import (
	"path/filepath"
	"testing"

	"github.com/br4zz4/ward/internal/config"
)

func TestSplitVaultArg_vault_only(t *testing.T) {
	// arrange
	arg := "app"

	// act
	vault, subDir := splitVaultArg(arg)

	// assert
	if vault != "app" {
		t.Errorf("expected vault=app, got %q", vault)
	}
	if subDir != "" {
		t.Errorf("expected subDir empty, got %q", subDir)
	}
}

func TestSplitVaultArg_vault_with_subdir(t *testing.T) {
	// arrange
	arg := "app:main"

	// act
	vault, subDir := splitVaultArg(arg)

	// assert
	if vault != "app" {
		t.Errorf("expected vault=app, got %q", vault)
	}
	if subDir != "main" {
		t.Errorf("expected subDir=main, got %q", subDir)
	}
}

func TestSplitVaultArg_vault_with_nested_subdir(t *testing.T) {
	// arrange
	arg := "app:secrets.prod"

	// act
	vault, subDir := splitVaultArg(arg)

	// assert
	if vault != "app" {
		t.Errorf("expected vault=app, got %q", vault)
	}
	if subDir != "secrets.prod" {
		t.Errorf("expected subDir=secrets.prod, got %q", subDir)
	}
}

func TestSplitVaultArg_subvault_cases(t *testing.T) {
	cases := []struct{ in, wantVault, wantSub string }{
		{"myfish::restricted:certs", "myfish::restricted", "certs"},
		{"myfish::restricted", "myfish::restricted", ""},
		{"app:main", "app", "main"},
		{"app", "app", ""},
	}
	for _, c := range cases {
		// act
		vault, sub := splitVaultArg(c.in)
		// assert
		if vault != c.wantVault || sub != c.wantSub {
			t.Errorf("splitVaultArg(%q) = (%q,%q), want (%q,%q)", c.in, vault, sub, c.wantVault, c.wantSub)
		}
	}
}

func TestResolveVaultDir_by_canonical_name(t *testing.T) {
	// arrange
	cfg := &config.Config{Vaults: []config.Source{
		{Name: "myfish::default", Path: ".ward/vaults/myfish::default"},
		{Name: "myfish::restricted", Path: ".ward/vaults/myfish::restricted"},
	}}
	cfgPath := filepath.Join("/proj", ".ward", "config.yaml")
	cases := map[string]string{
		"myfish":             "/proj/.ward/vaults/myfish::default",
		"myfish::default":    "/proj/.ward/vaults/myfish::default",
		"myfish::restricted": "/proj/.ward/vaults/myfish::restricted",
		"myfish::open":       "",
	}
	for arg, want := range cases {
		// act
		got := resolveVaultDir(cfg, arg, cfgPath)
		// assert
		if got != want {
			t.Errorf("resolveVaultDir(%q) = %q, want %q", arg, got, want)
		}
	}
}
