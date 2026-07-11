# Import/Export File Secrets

> TLDR: Add `ward import` and `ward export` commands to store entire files as single secrets in a vault, with a dedicated `.ward` naming convention that preserves the original file extension.

**Status:** proposed
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

### `ward import <file> [vault]`

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
- `test/e2e/import/` — e2e tests for import
- `test/e2e/export/` — e2e tests for export

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
