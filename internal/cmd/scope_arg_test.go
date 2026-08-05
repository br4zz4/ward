package cmd

import (
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

func TestResolveScopeArg_positional(t *testing.T) {
	got, err := resolveScopeArg(&scopeFlags{}, []string{"vault1:group.key1"})
	if err != nil || got != (secrets.Scope{Vault: "vault1", SecretPath: "group.key1"}) {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestResolveScopeArg_scopeFlag(t *testing.T) {
	got, _ := resolveScopeArg(&scopeFlags{scope: "vault1:group.key1"}, nil)
	if got.Vault != "vault1" || got.SecretPath != "group.key1" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveScopeArg_vaultSecretFlags(t *testing.T) {
	got, _ := resolveScopeArg(&scopeFlags{vault: "vault1", secret: "group.key1"}, nil)
	if got.Vault != "vault1" || got.SecretPath != "group.key1" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveScopeArg_conflict(t *testing.T) {
	if _, err := resolveScopeArg(&scopeFlags{vault: "vault1", secret: "group.key1"}, []string{"vault2:x"}); err == nil {
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
	if (secrets.Scope{Vault: "vault1", SecretPath: "group.key1"}).TreePath() != "vault1.group.key1" {
		t.Fatal("qualified tree path wrong")
	}
	if (secrets.Scope{SecretPath: "group.key1"}).TreePath() != "group.key1" {
		t.Fatal("unqualified tree path wrong")
	}
}

func TestScope_FullPath(t *testing.T) {
	if (secrets.Scope{Vault: "vault1", SecretPath: "group.key1"}).FullPath() != "vault1:group.key1" {
		t.Fatal("qualified full path should use colon")
	}
	if (secrets.Scope{SecretPath: "group.key1"}).FullPath() != "group.key1" {
		t.Fatal("unqualified full path wrong")
	}
}
