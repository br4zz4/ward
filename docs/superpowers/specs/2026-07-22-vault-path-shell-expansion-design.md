# Vault Path Shell Expansion

**Date:** 2026-07-22
**Status:** approved

## Problem

Projects that share a commons vault across machines need to reference it via an
environment variable (e.g. `$COMMONS_DIR`). Ward currently treats vault `path`
fields as literals, so `$COMMONS_DIR/.ward/vaults/commons` fails with a stat
error.

## Goal

Expand shell expressions in vault `path` fields at config load time, with
identical behavior to the user's shell — supporting `$VAR`, `${VAR}`, and
`$(cmd)` substitution.

## Approach

Delegate expansion to `sh -c "echo -n <path>"` and capture stdout as the
resolved path.

This gives exact shell semantics for free:
- `$VAR` and `${VAR}` — environment variable expansion
- `$(cmd)` — command substitution (e.g. `$(pwd)`, `$(cat .vault-path)`)
- Undefined variable → empty string → stat fails with a clear path error

## Implementation

### Location

`internal/config/config.go`, function `Load`, after `yaml.Unmarshal` and before
duplicate-name/path checks.

### Logic

```go
func expandPath(path string) (string, error) {
    if !strings.Contains(path, "$") {
        return path, nil
    }
    out, err := exec.Command("sh", "-c", "echo -n "+shellescape(path)).Output()
    if err != nil {
        return "", fmt.Errorf("expanding path %q: %w", path, err)
    }
    return string(out), nil
}
```

- Only forks when path contains `$` (fast path for the common case)
- Uses `echo -n` to avoid a trailing newline in stdout
- The path must be shell-escaped before interpolation to handle spaces and
  special characters safely
- Error from `sh` (e.g. bad command substitution) propagates as a config load
  error

### Shell escaping

Use single-quote wrapping: `'<path>'`, with internal single quotes escaped as
`'\''`. This is safe for all printable characters and prevents double-expansion.

Alternatively: pass the path as an argument via `sh -c 'echo -n "$1"' -- <path>`
to avoid any escaping concern entirely. This is the preferred implementation.

### Applied fields

Only `vault.Path`. Other fields (`key_file`, `key_env`, `default_dir`) are out
of scope for now.

### Duplicate-path check

The dedup check runs after expansion, so two vaults that expand to the same
path are still caught.

## Error handling

| Situation | Behavior |
|---|---|
| `$VAR` not defined | Expands to empty string; stat fails with path-not-found |
| `$(bad-cmd)` fails | `sh` exits non-zero; ward returns config load error with original path |
| `sh` not available | `exec.Command` fails; ward returns config load error |

## Tests

- Unit: path without `$` — no expansion, no fork
- Unit: `$VAR` expands correctly when env var is set
- Unit: `${VAR}` expands correctly
- Unit: `$(echo /tmp)` expands to `/tmp`
- Unit: undefined `$VAR` expands to empty string
- Unit: `$(false)` returns error

## Out of scope

- Expansion in `key_file`, `key_env`, or `default_dir`
- Windows support (`cmd.exe` semantics differ; deferred)
- Caching expansion results across multiple `Load` calls
