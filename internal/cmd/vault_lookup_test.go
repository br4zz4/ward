package cmd

import (
	"strings"
	"testing"
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
