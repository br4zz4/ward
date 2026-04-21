# Architecture

This document explains the internals of `ward` — how it discovers files, determines ancestry, merges secrets, and exposes them as env vars.

---

## Overview

```
ward exec staging.ward -- app

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
     │   Env vars  │  flatten to KEY=value, scope to anchor container
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

### File anchor loading

`secrets.FilterByAnchor(anchor ParsedFile, all []ParsedFile) []ParsedFile`

Returns all files that are structurally compatible ancestors of the anchor **and** have a strictly smaller map depth. This guards against sibling files being misidentified as ancestors.

### Directory anchor loading

Handled in `internal/ward/engine.go: orderForDirAnchor`. For each file in the dir, finds its ancestors from the global file list using the same `IsAncestorOf + mapDepth` check, deduplicates, then trims ancestors to scope.

### TrimToScope

`secrets.TrimToScope(ancestor ParsedFile, dirFiles []ParsedFile) ParsedFile`

When a directory anchor is used, ancestors are trimmed so that only branches relevant to the dir's files are retained. This prevents sibling data from leaking:

```
secrets.ward has:              dir anchor = sectors/two
  sectors:
    one:                       ← pruned (not in any file under two/)
      name: sector 1
  name: acme                   ← kept (leaf at company level)
```

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

### Flat (no anchor, no `--prefixed`)

`secrets.ToFlatEnvEntries(tree map[string]*Node) map[string]EnvEntry`

Walks all leaf nodes and uses only the leaf key name, uppercased. No structural prefix.

```
myapp.staging.database_url → DATABASE_URL
```

### Full path (no anchor, `--prefixed`)

`secrets.ToEnvEntries(tree map[string]*Node) map[string]EnvEntry`

Walks all leaf nodes. Key = uppercased dot-path with `.` replaced by `_`.

```
myapp.staging.database_url → MYAPP_STAGING_DATABASE_URL
```

### Anchor-relative (with anchor)

`secrets.ToEnvEntriesFromAnchor(tree map[string]*Node, anchorData map[string]interface{}) map[string]EnvEntry`

Descends through the tree following the anchor's structure until the anchor's container level, then collects all leaves using the remaining path as the key prefix.

```
anchor = staging.ward (defines myapp.staging.*)
container level = myapp
exposed keys: STAGING_DATABASE_URL, STAGING_REDIS_URL
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

- `Merge(anchorPath)` — load, order, merge
- `MergeForView(anchorPath)` — like Merge but always produces a full tree even when conflicts exist (for `list`/`show`)
- `Inspect(anchorPath)` — conflict-only check
- `EnvVars(result, prefixed)` — resolve env var names
- `Encrypt/Decrypt` — passthrough to the configured adapter

### ward new

`internal/cmd/new.go`

`resolveNewPath` maps user input to a file path:
- Absolute path → use as-is
- Contains `/` → use as-is relative to CWD, add `.ward` if missing
- Bare name → place in `default_dir` (default: `.ward/vault`)

`maybeAddSource` adds the new file's directory to `vaults` in the config if it is not already covered by an existing vault. The path stored in the config is always relative to the project root (parent of `.ward/`). Paths outside the project root get `../` prefixes as needed.

### Colour coding in list

`printTreeWithOrigin` renders leaves as:
- **Green** — origin inside anchor scope, key not in any ancestor
- **Orange** — overrides an ancestor value
- **Light blue** — inherited from outside anchor scope (shown in `list` only)
