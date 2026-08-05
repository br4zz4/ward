package cmd

import (
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

func TestResolveScopeArg_positional(t *testing.T) {
	got, err := resolveScopeArg(&scopeFlags{}, []string{"commons:infra.KEY"})
	if err != nil || got != (secrets.Scope{Vault: "commons", SecretPath: "infra.KEY"}) {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestResolveScopeArg_scopeFlag(t *testing.T) {
	got, _ := resolveScopeArg(&scopeFlags{scope: "commons:infra.KEY"}, nil)
	if got.Vault != "commons" || got.SecretPath != "infra.KEY" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveScopeArg_vaultSecretFlags(t *testing.T) {
	got, _ := resolveScopeArg(&scopeFlags{vault: "commons", secret: "infra.KEY"}, nil)
	if got.Vault != "commons" || got.SecretPath != "infra.KEY" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveScopeArg_conflict(t *testing.T) {
	if _, err := resolveScopeArg(&scopeFlags{vault: "commons", secret: "infra.KEY"}, []string{"trgclub:x"}); err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
}

func TestResolveScopeArg_empty(t *testing.T) {
	got, err := resolveScopeArg(&scopeFlags{}, nil)
	if err != nil || got != (secrets.Scope{}) {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestScope_TreePath(t *testing.T) {
	if (secrets.Scope{Vault: "commons", SecretPath: "infra.KEY"}).TreePath() != "commons.infra.KEY" {
		t.Fatal("qualified tree path wrong")
	}
	if (secrets.Scope{SecretPath: "infra.KEY"}).TreePath() != "infra.KEY" {
		t.Fatal("unqualified tree path wrong")
	}
}

func TestScope_FullPath(t *testing.T) {
	if (secrets.Scope{Vault: "commons", SecretPath: "infra.KEY"}).FullPath() != "commons:infra.KEY" {
		t.Fatal("qualified full path should use colon")
	}
	if (secrets.Scope{SecretPath: "infra.KEY"}).FullPath() != "infra.KEY" {
		t.Fatal("unqualified full path wrong")
	}
}
