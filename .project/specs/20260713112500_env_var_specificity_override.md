# Env Var Specificity Override

> TLDR: When two dot-paths resolve to the same env var name, the more specific (deeper) path wins instead of raising a collision error.

**Status:** proposed
**Created:** 2026-07-13
**Owner:** @oporpino

---

## Context

Today, when two secrets at different depths in the hierarchy produce the same env var name — e.g. `app.service_account_json` and `app.other.service_account_json` — ward raises an env var collision error and refuses to continue.

The expected behavior is the same as regular secret override: a more specific path (deeper in the hierarchy) silently overrides a less specific one. This is consistent with how ward already handles vault layering — `production` overrides `base`.

## Objectives

- `app.one.any` overrides `app.any` when both produce the same env var name
- No collision error when the conflict is resolvable by specificity
- The less specific value is marked as overridden (same as existing override display)
- If two paths at the same depth produce the same env var name, it is still a collision error

## Changes

- `internal/secrets/env.go` — resolve env var collisions by depth before raising an error; deeper dot-path wins
- `internal/secrets/env_test.go` — tests for specificity resolution

## Implementation Plan

1. **test:** same-env-var at different depths resolves to deeper path, same-depth still errors — `internal/secrets/env_test.go`
2. **feat:** in `EnvVars`, when two entries produce the same env var name, pick the one with more dot-path segments; only error when depths are equal — `internal/secrets/env.go`
3. **refactor:** extract depth-comparison logic if needed — `internal/secrets/env.go`

## How to verify

```bash
# file-secrets at different depths → no collision, deeper wins
ward-dev file import sa.json app
ward-dev file import sa.json app.other
ward-dev envs
# → service_account_json = <value from app.other.service_account_json>  (overrides)

# regular secrets at different depths → same behavior
# .ward/vaults/app/secrets.ward:       app: { secrets: { database_url: "base" } }
# .ward/vaults/app/prod/secrets.ward:  app: { prod: { secrets: { database_url: "prod" } } }
ward-dev envs
# → database_url = prod  (app.prod.secrets.database_url overrides app.secrets.database_url)

# two secrets at same depth → still a collision error
# app.secrets.database_url and app.other.database_url  (both depth 3)
ward-dev envs
# → ✗ env var collision
```

## Documentation

No documentation changes needed.
