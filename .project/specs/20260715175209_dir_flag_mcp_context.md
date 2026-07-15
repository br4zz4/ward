# Dir flag and MCP project context

> TLDR: Add `--dir/-d` flag to the CLI and an optional `dir` parameter to all MCP tools so agents can target any ward project without relying on the working directory.

**Status:** proposed
**Created:** 2026-07-15
**Owner:** @oporpino

---

## Context

The MCP server runs `ward` via `exec.Command` without setting `cmd.Dir`, so it always resolves the config from the process working directory — whichever directory the MCP was launched from. Agents have no way to target a different project.

`--config/-c` already exists but requires knowing the full config file path, which is verbose and forces the agent to know the internal structure (`.ward/config.yaml`). A `--dir/-d` flag is more ergonomic: pass the project root and let ward find the config automatically, the same way `FindConfigFile` already works.

The third piece is documentation: `wardDocs` in `server.go` must describe both flags so agents know they exist and when to use them.

## Objectives

- Add `--dir/-d` persistent flag to the root CLI command
- Add optional `dir` parameter to every MCP tool
- Update `wardDocs` to explain `--dir` and `--config` usage for agents working across multiple projects

## Changes

- `cmd/ward/main.go` — add `--dir/-d` persistent flag; in `PersistentPreRun`, chdir before calling `SetConfigFile`
- `internal/mcp/server.go` — add `dir` string param to all tools; in each handler, prepend `-d <dir>` to args when present; update `wardDocs`

## Implementation Plan

1. **test:** `--dir` flag resolves config from the given directory — files: `internal/cmd/helpers_test.go` or integration test
2. **feat:** add `--dir/-d` flag to `main.go`; in `PersistentPreRun` chdir to the given dir before `SetConfigFile` — files: `cmd/ward/main.go`
3. **feat:** add `dir` param to all MCP tools; prepend `-d <dir>` to args when non-empty — files: `internal/mcp/server.go`
4. **feat:** update `wardDocs` string to document `--dir`, `--config`, and when agents should use each — files: `internal/mcp/server.go`

### Phase philosophy and constraints

**Phase 1 — Make it Tested**
Write tests that invoke ward with `--dir /some/path` and confirm it loads the correct config. Confirm RED (flag not recognized yet).

**Phase 2 — Make it Work**
Add the flag and chdir logic. All tests turn GREEN. MCP `dir` param prepended as `-d` args.

**Phase 3 — Make it Better**
Review if chdir + SetConfigFile interaction is clean. No behavioral changes.

**Phase 4 — Make it Faster**
Not applicable.

## How to verify

```sh
# CLI: run from a different dir and target another ward project
cd /tmp
ward -d ~/workspace/myproject get DATABASE_URL

# MCP: call ward_get with dir param pointing to a different project
# agent should receive secrets from the correct project
```

## Documentation

No documentation changes needed.
