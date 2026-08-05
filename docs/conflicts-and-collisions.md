# Conflicts and Collisions

Ward distinguishes two different error conditions when merging secrets.
Understanding the difference tells you exactly what needs to change to fix it.

---

## Conflict

A **conflict** occurs when two or more vault files define the **same leaf dot-path**.

```yaml
# vault-a/app.ward
app:
  secret_key: key-from-a   # dot-path: app.secret_key

# vault-b/app.ward
app:
  secret_key: key-from-b   # dot-path: app.secret_key  ← same path, different file
```

Ward cannot decide which value is authoritative. The merge is blocked.

**How to resolve:**

1. **Remove** `secret_key` from one of the files — let a single vault own it.
2. **Move** it to a shared base vault that both sources include, if the value
   should be the same across both.

There is no automatic "last file wins" mode. Conflicts require an explicit
decision.

**Scope behaviour:** a scope argument now genuinely *restricts* the set of
leaves a command sees — it is not merely a tie-breaker for collisions. Leaves
outside the requested scope are not part of the operation at all, so conflicts
that live outside it cannot block a scoped read.

Because of this, a cross-environment conflict such as
`commons:infra.production.DATABASE_URL` versus
`commons:infra.staging.DATABASE_URL` no longer aborts a read scoped to
`infra.staging` — the `production` leaf is simply out of scope.

```sh
# infra.production.DATABASE_URL conflicts, but the read is scoped to staging — it succeeds
ward secrets commons:infra.staging
```

A **genuine conflict inside the same scope** — the same leaf defined by two
vaults under the same secret-path — still blocks. Resolve it with `--prefixed`
(see [Collision](#collision) below) so the vaults' env var names stay distinct.

---

## Collision

A **collision** occurs when two leaf nodes have the **same key name** but live
under **different, unrelated dot-paths**, causing them to produce the same
environment variable name when flattened.

```yaml
# staging.ward
app:
  staging:
    token: staging-token   # flattens to TOKEN

# production.ward
app:
  production:
    token: prod-token      # also flattens to TOKEN  ← collision
```

Neither dot-path is an ancestor of the other (`app.staging` ≠ `app.production`),
so ward cannot determine which `TOKEN` to inject.

**How to resolve:**

1. **Use `--prefixed`** — env var names include the full dot-path, guaranteeing
   uniqueness:
   ```sh
   ward exec --prefixed -- deploy.sh
   # injects APP_STAGING_TOKEN and APP_PRODUCTION_TOKEN
   ```

2. **Narrow the scope** — a scope argument restricts the set of leaves, so
   picking a scope under which only one of the colliding leaves lives resolves
   the collision:
   ```sh
   ward secrets app.staging        # TOKEN=staging-token, plus all in-scope vars
   ward secrets app.production     # TOKEN=prod-token, plus all in-scope vars
   ```
   The scope resolves the collision only when exactly one entry falls under it.
   If both entries are still in scope (e.g. `ward secrets app`), the collision
   is still reported.

**Note:** collisions are detected at the env-var layer, conflicts at the merge
layer. `ward inspect` reports **both** — run it (optionally scoped to a dot-path,
e.g. `ward inspect <scope>`) as a single pre-flight check. `ward inspect --prefixed`
checks as if full-path env var names were used, so it passes when `--prefixed`
would resolve every collision — a quick way to confirm that fix. `ward tree` also
shows collisions inline in the tree.

---

## Shadow (not an error)

A related but non-error case is **shadowing**: a leaf at a deeper dot-path
silently overrides a shallower leaf with the same key name when they share
a common ancestor.

```yaml
# app.ward — single file, two depths
app:
  log_level: info          # dot-path: app.log_level

  config:
    log_level: debug       # dot-path: app.config.log_level
```

`app.config.log_level` is a *descendant* of `app`, and both have the leaf
name `log_level`. The deeper one wins when flattening to env vars:

```sh
ward envs   # → LOG_LEVEL=debug
```

`ward tree` marks the shallower value as `(overridden)` in orange so you
can see it is present but not active.

---

## Quick reference

| Condition  | Trigger | Layer | `ward inspect` | Resolution |
|------------|---------|-------|----------------|------------|
| **Conflict**   | Same leaf dot-path in ≥2 files | Merge | ✓ reports it | Remove from one file, or move to shared vault |
| **Collision**  | Same leaf name, unrelated secret-paths | Env vars | ✓ reports it | Use `--prefixed` or narrow the scope |
| **Shadow**     | Same leaf name, one path is ancestor of the other | Env vars | ✗ silent | No action needed — deeper wins intentionally |
