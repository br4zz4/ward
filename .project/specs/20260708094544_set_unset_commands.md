# Set and Unset Commands

> TLDR: Add `ward set <dot.path> <value>` and `ward unset <dot.path>` to write and remove individual secrets by full dot-path, aborting with a clear error when the path is ambiguous across files.

**Status:** in_progress
**Created:** 2026-07-08
**Owner:** @oporpino

---

## Context

Today the only way to change a secret is `ward edit`/`ward new` (opens `$EDITOR`) or `ward override` (replaces a whole file from stdin). There is no scriptable, single-key mutation. We want two focused commands to set and unset one secret by its full dot-path, mirroring how `get` reads a value.

The dot-path always begins with the vault name (first segment), which is how ward maps a path to a `.ward` file. When the same dot-path is defined in more than one file, the merge is already broken (a Type-1 conflict) — `envs`, `exec`, `view` all abort. So `set`/`unset` must refuse to operate on an ambiguous path rather than guess or perpetuate the broken state.

A Type-2 conflict (env-var collision — two different dot-paths whose last segment maps to the same env var, e.g. `app.staging.token` and `app.prod.token` → `TOKEN`) is different: the full dot-path is unique, so `set`/`unset` always knows which file to touch. It does not block the operation. Consistent with `edit`/`new`/`override` (which also let you create such collisions), `set`/`unset` performs the write and, if the result leaves the touched env var in a Type-2 collision, prints a non-blocking warning.

## Objectives

- `ward set <dot.path> <value>` — create or update a single secret at a full dot-path.
- `ward unset <dot.path>` — remove a single secret at a full dot-path.
- Reuse the existing error style (colors, sources, resolution hints) for all failures.
- Never touch `config.yaml`: operate only within existing vaults.

## Changes

### `internal/cmd/set.go` (new)
`ward set <dot.path> <value>`:
1. Load config; resolve the vault from the first path segment.
   - Vault not found → **error**: vault does not exist, suggest creating it first (do not edit `config.yaml`).
2. Discover files and run a merge to detect Type-1 conflicts on the target path.
   - Target dot-path defined in 2+ files → **abort** with a clear error listing every source file+line (same shape as `ConflictError`): "cannot know which file to set".
3. Resolve the target file:
   - If the path already resolves to exactly one existing file → decrypt it, set/create the leaf in its YAML tree, re-encrypt.
   - If the path maps to no existing file → derive the file path with the `new` rule (`vault/subdirs/stem.ward`), create it, then write the leaf. **On success, print a notice that a new file was created and why.**
4. Write the value as a string leaf at the nested path (creating intermediate maps as needed).
5. Confirmation message on stderr, matching `override` style.
6. If the written env var now collides with another (Type-2), print a **non-blocking warning** listing the colliding dot-paths and the `--prefixed`/scope hint. The write still succeeds.

### `internal/cmd/unset.go` (new)
`ward unset <dot.path>`:
1. Same vault resolution and Type-1 conflict abort as `set`.
2. Locate the single file defining the path (leaf **or** group).
   - Path not found in any file → **error**: `key not found: <dot.path>` (same as `get`).
   - Path points to a **group** (a map with children), not a leaf → **error**: `<dot.path> is a group, not a leaf` and remove nothing. `unset` only ever removes a single leaf secret, never a whole branch.
3. Remove the leaf from the file's YAML tree, keeping the surrounding scaffold maps (`vault.[subdirs].stem`) in place — do **not** prune them. Pruning would drop key levels the vault-structure validator requires, so the file must keep its scaffold even when it holds no secrets.
4. Re-encrypt and write back; confirmation message.

`unset` does not warn about Type-2 collisions: removing a leaf can only reduce collisions, never create one.

### Implementation note: dot-path depth

The vault-structure rule pins every file's leading keys to `vault.[subdirs].stem`, so a leaf secret always lives at `vault.[subdirs].stem.leafname` — at least **three** segments. `set` rejects paths with fewer than three segments up front, and derives a new file's path from the middle segments (dropping the vault and the leaf).

### `internal/cmd/root.go`
Register both commands.

### Shared helpers
Reuse `newEngine`, `resolveNewPath`/`newFileStub` logic (from `new.go`), `splitPath`, and the encryption engine. Extract small helpers if `set` and `unset` share file-resolution/leaf-mutation code.

### Tests
- `internal/cmd/set_test.go`, `internal/cmd/unset_test.go` — unit tests (TDD).
- E2E fixtures under `test/e2e/set/` and `test/e2e/unset/` following the existing `vaults/<name>/*.ward` layout.
- Cover: create new key (+new file notice), update existing key, unset existing key, unset missing key (error), unset group path (error, nothing removed), Type-2 collision warning (set writes + warns), vault-not-found (set/unset), empty-file-after-unset keeps structure.
- **Type-1 conflict (same dot-path in 2+ files)** is covered by a unit test only. It is not reachable via e2e in a structurally valid project: the vault-structure rule pins each file to a unique `vault.stem` prefix, so two files cannot define the same leaf path, and `enforceVaultStructure()` (which runs first) would abort with a structure error before the Type-1 abort could fire.

## How to verify

```bash
# update existing
ward set app.db.host localhost && ward get app.db.host   # -> localhost

# create new key in a new file (prints "created" notice)
ward set app.new.token secret && ward get app.new.token  # -> secret

# ambiguous path -> aborts, lists sources
ward set app.db.token x                                   # -> conflict error, no write

# unset
ward unset app.db.host && ward get app.db.host            # -> key not found
ward unset app.missing                                     # -> error: key not found
```

## Documentation

- Update `docs/` command reference (if a command list exists) to include `set` and `unset`.
- Add a learning in `docs/learnings/` if the Type-1-conflict-abort behavior proves non-obvious.
