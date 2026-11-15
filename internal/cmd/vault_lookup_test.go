package cmd

import (
	"strings"
	"testing"

	"github.com/br4zz4/ward/internal/config"
)

func TestVaultNotFoundMessage_names_the_vault(t *testing.T) {
	// act
	got := vaultNotFoundMessage("onion-backend")

	// assert
	if !strings.Contains(got, "onion-backend") {
		t.Fatalf("expected the vault name in the message, got %q", got)
	}
}

func TestVaultNotFoundMessage_does_not_mention_dot_path(t *testing.T) {
	// act — the vault is named by `vault:`, --vault or its own argument;
	// a plain dot-path never identifies a vault.
	got := vaultNotFoundMessage("onion-backend")

	// assert
	if strings.Contains(got, "dot-path") {
		t.Fatalf("message should not describe the vault as a dot-path segment, got %q", got)
	}
}

func TestVaultNotFoundMessage_suggests_recovery(t *testing.T) {
	// act
	got := vaultNotFoundMessage("onion-backend")

	// assert
	if !strings.Contains(got, "ward vault list") {
		t.Fatalf("expected a hint to list vaults, got %q", got)
	}
	if !strings.Contains(got, "ward vault add onion-backend") {
		t.Fatalf("expected a hint to add the missing vault, got %q", got)
	}
}

func lookupCfg(names ...string) *config.Config {
	cfg := &config.Config{}
	for _, n := range names {
		cfg.Vaults = append(cfg.Vaults, config.Source{Name: n, Path: ".ward/vaults/" + n})
	}
	return cfg
}

func TestFindVault_default_alias_for_declared_default(t *testing.T) {
	for _, declared := range []string{"myfish::default", "myfish"} {
		// arrange
		cfg := lookupCfg(declared)
		for _, arg := range []string{"myfish", "myfish::default"} {
			// act
			got := findVault(cfg, arg)
			// assert
			if got == nil || got.Name != declared {
				t.Errorf("declared %q, arg %q: got %v", declared, arg, got)
			}
		}
	}
}

func TestFindVault_parent_does_not_reach_other_subvault(t *testing.T) {
	// arrange
	cfg := lookupCfg("myfish::open")
	// act
	got := findVault(cfg, "myfish")
	// assert
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestFindVault_finds_restricted_by_full_name(t *testing.T) {
	// arrange
	cfg := lookupCfg("myfish::default", "myfish::restricted")
	// act
	got := findVault(cfg, "myfish::restricted")
	// assert
	if got == nil || got.Name != "myfish::restricted" {
		t.Errorf("got %v", got)
	}
}
