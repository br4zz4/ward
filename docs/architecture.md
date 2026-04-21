# Architecture

This document explains the internals of `ward` — how it discovers files, determines ancestry, merges secrets, and exposes them as env vars.

---

## Overview

```
ward exec staging.ward -- app

     ┌─────────────┐
     │  Discover   │  glob *.ward under sources
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

Walks each source directory recursively and collects all `*.ward` files. File path has no semantic meaning — it is used only for display and deduplication.

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

The `Decryptor` interface allows swapping real SOPS decryption for a `MockDecryptor` that reads plain YAML — used in all tests.

---

## Hierarchy

### Ancestry detection

`secrets.IsAncestorOf(candidate, anchor ParsedFile) bool`

A file is an ancestor if its map-branch structure is **structurally compatible** with the anchor. Compatibility means:

1. They share at least one root key.
2. For every map-valued key in the candidate that also exists in the anchor, the sub-maps are recursively compatible.
3. Branches in the candidate that do not exist in the anchor are **allowed** — a single file may cover multiple environments or sectors; the ones not relevant to the anchor are simply irrelevant, not conflicting.

This is intentionally looser than "every branch of the candidate exists in the anchor." The strict version would fail for real-world cases where `company.ward` defines both `sectors.one` (leaf attributes) and `sectors.two` implicitly — they share the `company.sectors` branch but diverge below it.

### File anchor loading

`secrets.FilterByAnchor(anchor ParsedFile, all []ParsedFile) []ParsedFile`

Returns all files that are structurally compatible ancestors of the anchor **and** have a strictly smaller map depth. This guards against sibling files (same depth, different branch) being misidentified as ancestors.

### Directory anchor loading

Handled in `internal/cmd/helpers.go`. For each file in the dir, finds its ancestors from the global file list using the same `IsAncestorOf + mapDepth` check, deduplicates, then trims ancestors to scope.

### TrimToScope

`secrets.TrimToScope(ancestor ParsedFile, dirFiles []ParsedFile) ParsedFile`

When a directory anchor is used, ancestors are trimmed so that only branches relevant to the dir's files are retained. This prevents sibling data from leaking:

```
company.ward has:              dir anchor = sectors/two
  sectors:
    one:                       ← pruned (not in any file under two/)
      name: sector 1
    (two is not in company.ward, but sectors is shared)
  name: acme                   ← kept (leaf at company level)
```

The trim walks the ancestor's map, pruning any map branch that does not appear as a map key in any of the dir files at the same path level. Leaves are always preserved — they may share a key name with leaves in the dir files, which is important for `Overrides` detection.

### Specificity

`specificity(f ParsedFile) int` = `len(dotPaths(f.Data, ""))`

The total number of dot-paths (including intermediate nodes) in a file's data. Files are sorted ascending by specificity before merging — least specific (ancestors) first, most specific (leaves) last.

---

## Merge engine

`secrets.Merge(files []ParsedFile, mode MergeMode) (map[string]*Node, error)`

Files are processed left-to-right. For each key in each file:

- **Map key**: if a node exists and has children, recurse. If a node exists but is a leaf, it's a type conflict.
- **Leaf key**: if no node exists, create one. If a node exists:
  - `MergeModeDeep`: overwrite if incoming has higher specificity (ancestor → leaf override). Flag `Overrides = true` on the new node.
  - `MergeModeError`: raise a conflict if the existing node has the **same specificity** as the incoming file. Different specificity = legitimate ancestor override.
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
    Specificity int  // specificity of the source file
}
```

### Conflict detection

Conflicts are accumulated across the full merge (not fail-fast). All conflicts are returned in a single `ConflictError` so the user can fix everything at once:

```go
type ConflictError struct {
    Conflicts []conflict
}

type conflict struct {
    Key     string
    Sources [2]Origin
}
```

The error message is rendered with ANSI colours and includes file paths, line numbers, and source snippets.

---

## Env var generation

### Full path (no anchor)

`secrets.ToEnvVars(tree map[string]*Node) map[string]string`

Walks all leaf nodes. Key = uppercased dot-path with `.` replaced by `_`.

```
company.sectors.one.staging.database_url → COMPANY_SECTORS_ONE_STAGING_DATABASE_URL
```

### Anchor-relative (with anchor)

`secrets.ToEnvEntriesFromAnchor(tree map[string]*Node, anchorData map[string]interface{}) map[string]EnvEntry`

Descends through the tree following the anchor's structure until it reaches the anchor's **container level** — one level above the anchor's deepest content. Then collects all leaves from that node, using only the remaining path as the key prefix.

```
anchor = staging.ward (defines company.sectors.one.staging.*)
container level = company.sectors.one
exposed keys:
  name          (from company.sectors.one.name)
  staging_*     (from company.sectors.one.staging.*)
```

The descent stops when `anchorMapDepth` reaches 1 — meaning the next level contains the anchor's actual content. This ensures that structural prefixes (`company → sectors → one`) are stripped from the env var names.

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

### loadAndMerge

`internal/cmd/helpers.go: loadAndMerge(cfg *Config, anchorPath string) (map[string]*Node, error)`

Central orchestration function used by all commands. Handles three cases:

1. **No anchor** — load all source files, sort by specificity, merge with `cfg.Merge` mode.
2. **File anchor** — filter to ancestors via `FilterByAnchor`, merge with `cfg.Merge` mode.
3. **Dir anchor** — discover files in dir, find and trim ancestors, merge with `MergeModeError` (same-level conflicts in a dir are always ambiguous).

### Colour coding in list/show

`printTreeWithOrigin` collects two sets before rendering:

- `ancestorKeys`: leaf key names whose origin file is outside the anchor scope
- `isFromAnchorScope`: whether an origin file is inside the anchor path

A leaf is rendered:
- **Green** — origin inside anchor scope AND key does not appear in any ancestor
- **Orange** — origin inside anchor scope but key also appears in an ancestor (potential confusion), OR `Overrides = true`
- **Light blue** — origin outside anchor scope (inherited context, shown in `list` only)

---

## Encryption

The `Decryptor` interface is the only boundary between the merge engine and encryption:

```go
type Decryptor interface {
    Decrypt(path string) ([]byte, error)
}
```

`MockDecryptor` reads the file as-is (plain YAML). `SopsDecryptor` (not yet implemented) will call SOPS with the age key from `WARD_AGE_KEY` or the key file configured in `ward.yaml`.

This design means the entire merge engine, hierarchy detection, conflict reporting, and env var generation can be tested without any encryption infrastructure.
