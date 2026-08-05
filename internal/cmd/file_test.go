package cmd

import (
	"testing"
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
