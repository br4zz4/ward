# Import/Export File Secrets

> TLDR: Add `ward import` and `ward export` commands to store entire files as single secrets in a vault, with a dedicated `.ward` naming convention that preserves the original file extension.

**Status:** completed
**Created:** 2026-07-10
**Owner:** @oporpino

---

## Context

Some secrets are entire files — Google service account JSONs, certificates, XML configs. Storing them manually as a YAML key inside an existing `.ward` file is awkward: the content is large, hard to read, and mixes poorly with scalar secrets. A dedicated import/export flow gives these file-secrets a first-class representation.

## Objectives

- Store file contents as encrypted secrets with a clear naming convention (`name.ext.ward`)
- Expose them as env vars following the existing case rules
- Restore the original file from the vault with a single command

## Changes

### Naming convention

A file-secret is a `.ward` file whose name has the form `<original-name>.<original-ext>.ward`:

```
service-account.json.ward
credentials.xml.ward
config.yaml.ward
```

The YAML key inside the file is the original filename sanitized to snake_case (dots and hyphens → underscores):

```yaml
# service-account.json.ward
service_account_json: '<file contents as string>'
```

### Env var name

Follows the existing ward case rules — same snake_case derivation as the key name:

```
service_account_json   (default)
SERVICE_ACCOUNT_JSON   (with upcase active)
```

### `ward file import <file> <vault>`

> Note: implemented as `ward file import/export` (subcommands of `file`) to avoid collision with the existing `ward import`/`ward export` commands, which handle raw YAML encryption of `.ward` files.

### `ward import <file> [vault]` *(original design, superseded)*

- Reads `<file>` from disk
- Derives the key name: filename (without path) → replace `.` and `-` with `_`
- Creates `<vault-dir>/<original-filename>.ward` with the single key encrypted
- If `[vault]` is omitted, uses `default_dir` from config (same as `ward new`)
- Errors if the target `.ward` file already exists (no silent overwrite)

### `ward export <original-filename> [dest]`

- Resolves which `.ward` file corresponds to `<original-filename>` (looks for `<original-filename>.ward` across all vaults)
- Decrypts and reads the single key value
- Writes the content to `[dest]/<original-filename>` (defaults to CWD)
- Errors if the file already exists at destination (no silent overwrite)

### Files to create/modify

- `internal/cmd/import.go` — new `ward import` command
- `internal/cmd/export.go` — new `ward export` command
- `internal/cmd/root.go` — register both commands
- `internal/secrets/filekey.go` — key derivation logic (filename → snake_case key)
- `test/e2e/import/` — e2e tests for import
- `test/e2e/export/` — e2e tests for export

## Implementation Plan

Ordered execution steps following the TDD cycle. Each step maps to one commit.

1. **test:** key derivation from filename — files: `internal/secrets/filekey_test.go`
2. **test:** `ward import` happy path and error cases (file not found, target exists) — files: `test/e2e/import/`
3. **test:** `ward export` happy path and error cases (secret not found, dest exists) — files: `test/e2e/export/`
4. **feat:** implement key derivation (`filekey.go`) and `ward import` command — files: `internal/secrets/filekey.go`, `internal/cmd/import.go`, `internal/cmd/root.go`
5. **feat:** implement `ward export` command — files: `internal/cmd/export.go`, `internal/cmd/root.go`
6. **refactor:** extract shared filename resolution logic, apply DRY/KISS, move unit tests to proper files — files: `internal/secrets/filekey.go`, `internal/cmd/import.go`, `internal/cmd/export.go`
7. **perf:** *(skip — no database or algorithmic bottleneck expected)*

Every phase requires at least one commit. Each step must leave the test suite green before moving to the next.

### Phase philosophy and constraints

**Phase 1 — Make it Tested (`test:` commit)**
Write all tests before touching any production code. Confirm RED for the right reason.
- Constraint: no production code in this phase.

**Phase 2 — Make it Work (`feat:` commit)**
Minimum code to turn every test GREEN. Duplication and naive implementations are acceptable.
- Constraint: no refactoring, no optimization.

**Phase 3 — Make it Better (`refactor:` commit)**
Remove if/else chains, extract methods, apply DRY/SOLID/KISS, move unit tests to their proper files without changing content.
- Constraint: tests must stay green. No new behavior.

**Phase 4 — Make it Faster (`perf:` commit)**
No measured bottleneck expected — skip this phase.

## How to verify

```bash
# import
echo '{"type":"service_account"}' > /tmp/service-account.json
ward import /tmp/service-account.json
# → creates .ward/vault/service-account.json.ward

# env var
ward envs
# → service_account_json={"type":"service_account"}

# export
ward export service-account.json /tmp/out/
# → /tmp/out/service-account.json contains the original content
```

## Documentation

- `docs/configuration.md` — document the `import`/`export` commands and the file-secret naming convention
