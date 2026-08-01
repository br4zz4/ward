# Mandatory Encryption, Per-Vault Key Warnings, and `.plain.ward` Files

> TLDR: Every `.ward` must be encrypted (no more silent plaintext fallback). A missing key for a vault becomes a per-vault warning while other vaults still render; no key anywhere with encrypted files present is a fatal error. Introduce `.plain.ward` — a structured YAML file that is always plaintext, never encrypted, treated as a single extension.

**Status:** draft
**Created:** 2026-08-01
**Owner:** @oporpino

---

## Context

Today, when no key resolves for a file, ward silently uses `MockDecryptor`, which reads the file as plain text. If the file is actually an age-armored blob, the armor is fed to the YAML parser and produces garbage values with no clear error. There is also no first-class way to keep non-secret, versioned config alongside secrets.

This change makes encryption the default and only mode for `.ward` files, surfaces missing keys as actionable warnings/errors instead of silent corruption, and adds a `.plain.ward` file type for structured-but-public content.

## Objectives

- Every `*.ward` file must be an age-armored blob; a plaintext `.ward` is an error.
- Remove the silent `MockDecryptor` plaintext fallback from the normal read path.
- `.plain.ward` is a new file type: structured YAML, always plaintext, never encrypted.
- `.plain.ward` is treated as a single extension: the location/dot-path strips `.plain.ward` whole (`config.plain.ward` → root `config`), so `.plain` never appears in the tree or env vars.
- A vault whose encrypted files cannot be decrypted (no key) is skipped with a `stderr` warning; other vaults still render. Its `.plain.ward` files are still read.
- If no key resolves anywhere and at least one encrypted file exists, commands fail with a clear error (suggest `set WARD_KEY` / provide `.ward/<vault>.key`, not `ward init`).
- `.plain.ward` obeys the same vault-structure/location validation as `.ward`.

## File type matrix

| Name                         | Type              | Encrypted?       | Parsing                    |
|------------------------------|-------------------|------------------|----------------------------|
| `main.ward`, `config.ward`   | structured        | yes (required)   | YAML → tree                |
| `sa.json.ward`, `x.key.ward` | raw/file-secret   | yes (required)   | raw content → single value |
| `config.plain.ward`          | structured plain  | no (forbidden)   | YAML → tree                |

`.plain.ward` is never treated as a raw/file-secret even though its inner name contains a dot.

## Changes

### File classification (`internal/secrets/filekey.go`)
- Add `IsPlainFile(path string) bool` — true when basename ends with `.plain.ward`.
- `OriginalFilename` returns `("", false)` for `.plain.ward` files (never a raw file-secret).
- Add a helper to strip the ward suffix for location purposes: `.plain.ward` → strip whole suffix; `.ward` → strip `.ward`.

### Loader (`internal/secrets/loader.go`)
- `.plain.ward` → always parsed as structured YAML, regardless of inner dots; never raw.
- Keep raw/file-secret handling for encrypted `x.ext.ward`.

### Decryptor detection (`internal/age/age.go` + `internal/sops`)
- Add detection of the age armor header (`-----BEGIN AGE ENCRYPTED FILE-----`).
- Reading a `.plain.ward` that is actually an age blob → error ("plain file must not be encrypted").
- Reading a normal `.ward` that is NOT an age blob (plaintext on disk) → error ("file is not encrypted; encrypt it or rename to .plain.ward").

### Key resolution / decryptor build (`internal/cmd/helpers.go`)
- Remove `MockDecryptor` from the normal read path for encrypted files.
- Track, per vault, whether a key resolved. Expose this so the engine can skip + warn.
- `.plain.ward` files bypass decryption entirely (read as-is).

### Engine merge (`internal/ward/engine.go`)
- During load/merge: for a vault with encrypted files but no resolvable key → skip those files, collect a warning `⚠ missing key for vault <X> — N file(s) skipped`.
- Still load that vault's `.plain.ward` files.
- If NO key resolves anywhere AND ≥1 encrypted file exists → fatal error: `no encryption key found — set WARD_KEY or provide .ward/<vault>.key`.
- If nothing encrypted exists (only `.plain.ward`) → succeed with no key.
- Warnings go to `stderr`; merged tree (stdout) stays valid.

### Read commands wired to warnings/errors
- `get`, `view`/`tree`, `envs`, `export`, `inspect`, `exec`, `raw`.

### Structure validation (`internal/cmd/validate.go`)
- `expectedFileDotPath`: strip `.plain.ward` whole (not just `.ward`) so `app/config.plain.ward` expects root `app.config`.
- `leadingKeyPath` / validation applies to `.plain.ward` the same as `.ward`; a mislocated `.plain.ward` produces the same structure-violation error.

### Discovery cleanup (`internal/secrets/discover.go`)
- Remove the dead `filepath.Glob` call and `_ = matches` (WalkDir already covers it).

## Breaking change

Projects that today keep `.ward` files in plaintext (relying on the MockDecryptor fallback) will break: those files must either be encrypted or renamed to `.plain.ward`. Documented, no auto-migration.

## How to verify

```bash
# .plain.ward read without any key, merges at the stripped dot-path
ward view          # config.plain.ward → app.config.* (no ".plain" segment)

# Vault missing a key → warning on stderr, other vaults still shown, exit 0
ward view 2>warn.txt; grep "missing key for vault" warn.txt

# No key anywhere + an encrypted file present → fatal error
env -u WARD_KEY ward get  # → "no encryption key found ..."

# A plaintext .ward (not .plain) → error
ward view   # → "file is not encrypted; encrypt it or rename to .plain.ward"

# An encrypted .plain.ward → error
ward view   # → "plain file must not be encrypted"

go test ./...
go test -tags e2e ./test/e2e/...
```

## Documentation

- `docs/configuration.md` — document `.plain.ward`, mandatory encryption, missing-key warning/error behavior.
- `README.md` — note the file types and the breaking change.
- `internal/mcp/server.go` (`wardDocs`) — mention `.plain.ward`.
