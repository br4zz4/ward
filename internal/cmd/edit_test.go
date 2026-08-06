package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

// editFixtureFiles mirrors a two-vault project: vault "app" split across a
// parent file and one child per environment, plus an unrelated vault "infra".
func editFixtureFiles() []secrets.ParsedFile {
	return []secrets.ParsedFile{
		{File: ".ward/vaults/app/main.ward", Data: map[string]interface{}{
			"app": map[string]interface{}{"main": map[string]interface{}{"name": "x"}},
		}},
		{File: ".ward/vaults/app/main/production.ward", Data: map[string]interface{}{
			"app": map[string]interface{}{"main": map[string]interface{}{"production": map[string]interface{}{"token": "y"}}},
		}},
		{File: ".ward/vaults/infra/dns.ward", Data: map[string]interface{}{
			"infra": map[string]interface{}{"dns": map[string]interface{}{"zone": "z"}},
		}},
	}
}

func TestScopeTargetFiles_vault_qualified(t *testing.T) {
	// act
	got := scopeTargetFiles(editFixtureFiles(), secrets.ParseScope("app:main.production"))

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/main/production.ward" {
		t.Fatalf("expected [production.ward], got %v", got)
	}
}

func TestScopeTargetFiles_unqualified_dot_path(t *testing.T) {
	// act
	got := scopeTargetFiles(editFixtureFiles(), secrets.ParseScope("main.production"))

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/main/production.ward" {
		t.Fatalf("expected [production.ward], got %v", got)
	}
}

func TestScopeTargetFiles_vault_name_only(t *testing.T) {
	// act
	got := scopeTargetFiles(editFixtureFiles(), secrets.ParseScope("app"))

	// assert
	if len(got) != 2 {
		t.Fatalf("expected both app files, got %v", got)
	}
}

func TestScopeTargetFiles_group_spanning_files(t *testing.T) {
	// act — "app:main" is defined in the parent and the production child
	got := scopeTargetFiles(editFixtureFiles(), secrets.ParseScope("app:main"))

	// assert
	if len(got) != 2 {
		t.Fatalf("expected 2 files for group scope, got %v", got)
	}
}

func TestScopeTargetFiles_absent_path(t *testing.T) {
	// act
	got := scopeTargetFiles(editFixtureFiles(), secrets.ParseScope("app:nope.nothing"))

	// assert
	if len(got) != 0 {
		t.Fatalf("expected no files, got %v", got)
	}
}

// vaultFileFixture is one vault's discovered .ward files, as Discover returns them.
func vaultFileFixture() []string {
	return []string{
		".ward/vaults/app/main.ward",
		".ward/vaults/app/infra/production.ward",
		".ward/vaults/app/infra/staging.ward",
		".ward/vaults/app/db/production.ward",
	}
}

func TestResolveVaultFile_relative_path(t *testing.T) {
	// act
	got := resolveVaultFile(".ward/vaults/app", vaultFileFixture(), "infra/production")

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/infra/production.ward" {
		t.Fatalf("expected [infra/production.ward], got %v", got)
	}
}

func TestResolveVaultFile_relative_path_with_extension(t *testing.T) {
	// act
	got := resolveVaultFile(".ward/vaults/app", vaultFileFixture(), "infra/production.ward")

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/infra/production.ward" {
		t.Fatalf("expected [infra/production.ward], got %v", got)
	}
}

func TestResolveVaultFile_dot_path(t *testing.T) {
	// act — dot notation addresses the same file as the slash form
	got := resolveVaultFile(".ward/vaults/app", vaultFileFixture(), "infra.production")

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/infra/production.ward" {
		t.Fatalf("expected [infra/production.ward], got %v", got)
	}
}

func TestResolveVaultFile_basename_unique(t *testing.T) {
	// act
	got := resolveVaultFile(".ward/vaults/app", vaultFileFixture(), "staging")

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/infra/staging.ward" {
		t.Fatalf("expected [infra/staging.ward], got %v", got)
	}
}

func TestResolveVaultFile_basename_ambiguous(t *testing.T) {
	// act — "production" exists under both infra/ and db/
	got := resolveVaultFile(".ward/vaults/app", vaultFileFixture(), "production")

	// assert
	if len(got) != 2 {
		t.Fatalf("expected 2 ambiguous matches, got %v", got)
	}
}

func TestResolveVaultFile_group_prefix(t *testing.T) {
	// act — a directory name selects every file beneath it
	got := resolveVaultFile(".ward/vaults/app", vaultFileFixture(), "infra")

	// assert
	if len(got) != 2 {
		t.Fatalf("expected both infra files, got %v", got)
	}
}

func TestResolveVaultFile_no_match(t *testing.T) {
	// act
	got := resolveVaultFile(".ward/vaults/app", vaultFileFixture(), "nope")

	// assert
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %v", got)
	}
}

func TestPromptChoice_valid_choice(t *testing.T) {
	// arrange
	var out bytes.Buffer

	// act
	got, err := promptChoice("Select a vault:", []string{"app", "infra"}, strings.NewReader("2\n"), &out)

	// assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "infra" {
		t.Fatalf("expected infra, got %q", got)
	}
	if !strings.Contains(out.String(), "Select a vault:") {
		t.Fatalf("expected label in output, got %q", out.String())
	}
}

func TestPromptChoice_preserves_given_order(t *testing.T) {
	// arrange — vaults must be offered in config order, not sorted
	var out bytes.Buffer

	// act
	got, err := promptChoice("Select a vault:", []string{"zeta", "alpha"}, strings.NewReader("1\n"), &out)

	// assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "zeta" {
		t.Fatalf("expected zeta (first as given), got %q", got)
	}
}

func TestPromptChoice_single_item_no_prompt(t *testing.T) {
	// arrange
	var out bytes.Buffer

	// act
	got, err := promptChoice("Select a vault:", []string{"only"}, strings.NewReader(""), &out)

	// assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "only" {
		t.Fatalf("expected only, got %q", got)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output, got %q", out.String())
	}
}

func TestPromptChoice_invalid_choice(t *testing.T) {
	// act
	_, err := promptChoice("Select a vault:", []string{"a", "b"}, strings.NewReader("0\n"), &bytes.Buffer{})

	// assert
	if err == nil {
		t.Fatal("expected error for out-of-range choice")
	}
}

func TestPromptChoice_empty_list(t *testing.T) {
	// act
	_, err := promptChoice("Select a vault:", nil, strings.NewReader(""), &bytes.Buffer{})

	// assert
	if err == nil {
		t.Fatal("expected error for empty list")
	}
}

func TestPickFromList_valid_choice(t *testing.T) {
	// arrange
	files := []string{"a.ward", "b.ward"}
	var out bytes.Buffer

	// act
	got, err := pickFromList(files, strings.NewReader("2\n"), &out)

	// assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "b.ward" {
		t.Fatalf("expected b.ward, got %q", got)
	}
	if !strings.Contains(out.String(), "a.ward") || !strings.Contains(out.String(), "b.ward") {
		t.Fatalf("expected both files listed, got %q", out.String())
	}
}

func TestPickFromList_invalid_choice(t *testing.T) {
	// act
	_, err := pickFromList([]string{"a.ward", "b.ward"}, strings.NewReader("9\n"), &bytes.Buffer{})

	// assert
	if err == nil {
		t.Fatal("expected error for out-of-range choice")
	}
}

func TestPickFromList_single_file_no_prompt(t *testing.T) {
	// act — a single candidate is returned without reading input
	var out bytes.Buffer
	got, err := pickFromList([]string{"only.ward"}, strings.NewReader(""), &out)

	// assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "only.ward" {
		t.Fatalf("expected only.ward, got %q", got)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output, got %q", out.String())
	}
}
