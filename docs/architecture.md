# Architecture

This document explains the internals of `ward` — how it discovers files, determines ancestry, merges secrets, and exposes them as env vars.

---

## Overview

```
ward exec myapp.environments.staging -- app

     ┌─────────────┐
     │  Discover   │  glob *.ward under vaults
     └──────┬──────┘
            │  []string (paths)
     ┌──────▼──────┐
     │    Load     │  decrypt + parse YAML, track line numbers
     └──────┬──────┘
            │  []ParsedFile
     ┌──────▼──────┐
     │  Hierarchy  │  determine ancestors, sort by specificity
     └──────┬──────┘
            │  []ParsedFile (ordered)
     ┌──────▼──────┐
     │    Merge    │  deep merge, conflict detection, origin tracking
     └──────┬──────┘
            │  map[string]*Node
     ┌──────▼──────┐
     │   Env vars  │  flatten to KEY=value, scoped by dot-path prefix
     └──────┬──────┘
            │  map[string]EnvEntry
     ┌──────▼──────┐
     │    Exec     │  inject into process environment
     └─────────────┘
```

---

## Discover

`secrets.Discover(sources []string) ([]string, error)`

Walks each vault directory recursively and collects all `*.ward` files. File path has no semantic meaning — it is used only for display and deduplication.

---

## Load

`secrets.Load(path string, dec Decryptor) (ParsedFile, error)`

Decrypts the file using the `Decryptor` interface, then parses it as YAML using `gopkg.in/yaml.v3`'s `yaml.Node` API. The `yaml.Node` walk preserves line numbers for every scalar value, populating a `LineMap`:

```go
type LineMap map[string]int  // dot-path → line number

type ParsedFile struct {
    File     string
    Data     map[string]interface{}
    Lines    LineMap
    RawLines []string  // source lines for snippet display in conflict errors
}
```

The `Decryptor` interface allows swapping real encryption for a plain-YAML passthrough used in tests.

---

## Encryption adapters

Two adapters implement the `Decryptor` (and `Encryptor`) interfaces:

### AgeArmorDecryptor (`age+armor`, default)

`internal/age.AgeArmorDecryptor{KeyFile: ".ward.key"}`

- Encrypts the entire file as an opaque ASCII-armored blob using `filippo.io/age`.
- Decrypt: reads the armor envelope, decrypts with the age identity from `KeyFile`.
- Plain YAML passthrough: if the file does not start with `-----BEGIN AGE ENCRYPTED FILE-----`, it is returned as-is (supports unencrypted test fixtures).
- No external binaries required.

### SopsDecryptor (`sops+age`)

`internal/sops.SopsDecryptor{KeyFile: ".ward.key"}`

- YAML keys remain visible; only values are encrypted as `ENC[AES256_GCM,...]` tokens.
- Uses `github.com/getsops/sops/v3` Go library — no `sops` binary required.
- Compatible with files previously created by the `sops` CLI.
- Sets `SOPS_AGE_KEY_FILE` env var before calling the library.

The active adapter is selected by the `engine` field in `.ward/config.yaml`. The CLI resolves it in `internal/cmd/helpers.go: decryptorFor()`.

---

## Hierarchy

### Ancestry detection

`secrets.IsAncestorOf(candidate, anchor ParsedFile) bool`

A file is an ancestor if its map-branch structure is **structurally compatible** with the anchor. Compatibility means:

1. They share at least one root key.
2. For every map-valued key in the candidate that also exists in the anchor, the sub-maps are recursively compatible.
3. Branches in the candidate that do not exist in the anchor are **allowed** — a single file may cover multiple environments; irrelevant branches are not conflicting.

### Loading all vaults

`engine.MergeScoped(scopePrefix string)` loads all `.ward` files from all configured vaults, resolves ancestry order (least to most specific), and merges them. The `scopePrefix` dot-path is used only during conflict detection and env var resolution — not to filter which files are loaded.

### Specificity

`specificity(f ParsedFile) int` = `len(dotPaths(f.Data, ""))`

The total number of dot-paths (including intermediate nodes) in a file's data. Files are sorted ascending by specificity before merging — least specific (ancestors) first, most specific (leaves) last.

---

## Merge engine

`secrets.Merge(files []ParsedFile, mode MergeMode) (map[string]*Node, error)`

Files are processed left-to-right. For each key in each file:

- **Map key**: if a node exists and has children, recurse. If a node exists but is a leaf, it's a type conflict.
- **Leaf key**: if no node exists, create one. If a node exists:
  - `MergeModeDeep`: overwrite if incoming has higher specificity. Flag `Overrides = true` on the new node.
  - `MergeModeError`: raise a conflict if the existing node has the **same specificity** as the incoming file.
  - `MergeModeOverride`: always overwrite, no conflict detection.

### Node

```go
type Node struct {
    Value     interface{}
    Origin    Origin
    Overrides bool              // replaced a value from a less-specific file
    Children  map[string]*Node  // nil for leaf nodes
}

type Origin struct {
    File        string
    Line        int
    Snippet     string
    Specificity int
}
```

### Conflict detection

Conflicts are accumulated across the full merge (not fail-fast). All conflicts are returned in a single `ConflictError` so the user can fix everything at once.

---

## Env var generation

### Flat (no dot-path, no `--prefixed`)

`secrets.ToFlatEnvEntries(tree map[string]*Node) map[string]EnvEntry`

Walks all leaf nodes and uses only the leaf key name, uppercased. No structural prefix.

```
myapp.staging.database_url → DATABASE_URL
```

### Full path (no dot-path, `--prefixed`)

`secrets.ToEnvEntries(tree map[string]*Node) map[string]EnvEntry`

Walks all leaf nodes. Key = uppercased dot-path with `.` replaced by `_`.

```
myapp.staging.database_url → MYAPP_STAGING_DATABASE_URL
```

### Dot-path scoped (with dot-path argument)

`engine.EnvVarsPrefer(r *MergeResult, prefixed bool, preferPrefix string) (map[string]EnvEntry, error)`

When a dot-path is provided (e.g. `myapp.environments.staging`), the full merged tree is used but env var names are resolved using flat leaf names. The `preferPrefix` is used to break ties when two leaves would produce the same env var name — the one under the given dot-path wins.

```
dot-path = myapp.environments.staging
exposed: DATABASE_URL, REDIS_URL (flat leaf names)
```

### EnvEntry

```go
type EnvEntry struct {
    Value     string
    Origin    Origin
    Overrides bool
}
```

`Overrides` propagates from the underlying `Node`. The CLI uses it to colour-code output: green = active (new key), orange = overrides an ancestor value.

---

## CLI layer

### Engine

`internal/ward.Engine` is the central orchestration object. It holds the config and decryptor and exposes:

- `Merge()` — load, order, merge all vaults
- `MergeScoped(dotPath)` — like Merge but scopes conflict detection to a dot-path prefix
- `MergeForView()` — always produces a full tree even when conflicts exist (for `view`)
- `Inspect()` — conflict-only check
- `EnvVars(result, prefixed)` — resolve env var names
- `Encrypt/Decrypt` — passthrough to the configured adapter

### ward new

`internal/cmd/new.go`

`resolveNewPath` maps user input to a file path:
- Absolute path → use as-is
- Contains `/` → use as-is relative to CWD, add `.ward` if missing
- Bare name → place in `default_dir` (default: `.ward/vault`)

`maybeAddSource` adds the new file's directory to `vaults` in the config if it is not already covered by an existing vault. The path stored in the config is always relative to the project root (parent of `.ward/`). Paths outside the project root get `../` prefixes as needed.

### Colour coding in view

`printTreeWithOrigin` renders leaves as:
- **Green** — active key, not overriding any ancestor value
- **Orange** — overrides an ancestor value
- **Light blue** — inherited from outside the given dot-path scope (shown in `view` only)
