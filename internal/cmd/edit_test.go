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

func TestScopeTargetFiles_plain_dot_never_identifies_vault(t *testing.T) {
	// act — "app.main" is a literal secret-path, NOT vault app + path main.
	// No file defines "app.main" below its vault root, so nothing matches.
	got := scopeTargetFiles(editFixtureFiles(), secrets.ParseScope("app.main"))

	// assert
	if len(got) != 0 {
		t.Fatalf("a plain dot-path must not select a vault's files, got %v", got)
	}
}

func TestScopeTargetFiles_bare_vault_name_is_not_a_scope(t *testing.T) {
	// act — unqualified "app" must not resolve through the vault root either;
	// `ward edit app` is handled as a vault name before scope resolution.
	got := scopeTargetFiles(editFixtureFiles(), secrets.ParseScope("app"))

	// assert
	if len(got) != 0 {
		t.Fatalf("bare vault name must not match as a scope, got %v", got)
	}
}

func TestScopeTargetFiles_qualified_vault_name_only(t *testing.T) {
	// act — with the colon it IS a vault qualifier, selecting the whole vault
	got := scopeTargetFiles(editFixtureFiles(), secrets.ParseScope("app:"))

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

func TestResolveVaultFile_literal_dotted_component(t *testing.T) {
	// arrange — a directory whose name legitimately contains a dot
	files := []string{
		".ward/vaults/app/services/api.v2/prod.ward",
		".ward/vaults/app/services/api/v2/prod.ward",
	}

	// act — the path as typed must win over dot-to-slash normalisation
	got := resolveVaultFile(".ward/vaults/app", files, "services/api.v2/prod")

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/services/api.v2/prod.ward" {
		t.Fatalf("expected the literal api.v2 path, got %v", got)
	}
}

func TestResolveVaultFile_literal_dotted_component_with_extension(t *testing.T) {
	// arrange
	files := []string{".ward/vaults/app/services/api.v2/prod.ward"}

	// act — only the trailing .ward is an extension; inner dots are literal
	got := resolveVaultFile(".ward/vaults/app", files, "services/api.v2/prod.ward")

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/services/api.v2/prod.ward" {
		t.Fatalf("expected the literal api.v2 path, got %v", got)
	}
}

func TestResolveVaultFile_dotted_basename(t *testing.T) {
	// arrange
	files := []string{".ward/vaults/app/api.v2.ward"}

	// act — a bare basename carrying a dot resolves literally
	got := resolveVaultFile(".ward/vaults/app", files, "api.v2")

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/api.v2.ward" {
		t.Fatalf("expected api.v2.ward, got %v", got)
	}
}

func TestResolveVaultFile_dot_notation_still_falls_back(t *testing.T) {
	// arrange — no literal "infra.production" file exists
	files := []string{".ward/vaults/app/infra/production.ward"}

	// act
	got := resolveVaultFile(".ward/vaults/app", files, "infra.production")

	// assert
	if len(got) != 1 || got[0] != ".ward/vaults/app/infra/production.ward" {
		t.Fatalf("expected the dot-notation fallback to resolve, got %v", got)
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

func TestUnresolvedScopeError_nil_when_nothing_was_skipped(t *testing.T) {
	// act — a clean load that simply had no match is not an error here
	got := unresolvedScopeError(secrets.ParseScope("app:main"), nil, false)

	// assert
	if got != nil {
		t.Fatalf("expected no error when the whole project was read, got %v", got)
	}
}

func TestUnresolvedScopeError_nil_when_the_scoped_vault_was_read(t *testing.T) {
	// act — another vault was skipped, but this scope's vault loaded fine, so
	// the skip cannot explain the miss
	got := unresolvedScopeError(secrets.ParseScope("app:nope.missing"),
		[]string{"missing key for vault locked — 1 file(s) skipped"}, true)

	// assert
	if got != nil {
		t.Fatalf("expected no error when the scoped vault was read, got %v", got)
	}
}

func TestUnresolvedScopeError_reports_the_skipped_vault(t *testing.T) {
	// arrange
	warnings := []string{"missing key for vault locked — 1 file(s) skipped"}

	// act
	got := unresolvedScopeError(secrets.ParseScope("locked:infra.production"), warnings, false)

	// assert
	if got == nil {
		t.Fatal("expected an error when files were skipped")
	}
	if !strings.Contains(got.Error(), "locked:infra.production") {
		t.Errorf("expected the scope in the message, got %q", got)
	}
	if !strings.Contains(got.Error(), "missing key for vault locked") {
		t.Errorf("expected the skip warning in the message, got %q", got)
	}
}

func TestUnresolvedScopeError_suggests_the_path_form(t *testing.T) {
	// act — the <vault> <file> form resolves without a key, so point at it
	got := unresolvedScopeError(secrets.ParseScope("locked:infra.production"),
		[]string{"missing key for vault locked — 1 file(s) skipped"}, false)

	// assert
	if got == nil || !strings.Contains(got.Error(), "ward edit locked <file>") {
		t.Fatalf("expected the vault+path form suggested, got %v", got)
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
