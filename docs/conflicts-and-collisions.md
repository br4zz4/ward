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

**Scope behaviour:** if you provide a dot-path argument to `ward envs` or
`ward exec`, conflicts *outside* that path are silently resolved (last writer
wins) so your scoped command can proceed. Conflicts *inside* or *above* the
requested path still block.

```sh
# app.secret_key conflicts, but app.db.host does not — scoped command succeeds
ward envs app.db
```

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

2. **Provide a dot-path hint** — tells ward which branch to prefer when
   resolving the collision. All other env vars from the full tree are still
   included:
   ```sh
   ward envs app.staging        # TOKEN=staging-token, plus all other vars
   ward envs app.production     # TOKEN=prod-token, plus all other vars
   ```
   The hint only resolves collisions where exactly one entry matches the
   prefix. If both entries are under the hint (e.g. `ward envs app`), the
   collision is still reported.

**Note:** collisions are detected at the env-var layer, not at the merge layer.
`ward inspect` reports *conflicts* only. Use `ward envs` or `ward view` to
surface collisions.

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

`ward view` marks the shallower value as `(overridden)` in orange so you
can see it is present but not active.

---

## Quick reference

| Condition  | Trigger | Layer | `ward inspect` | Resolution |
|------------|---------|-------|----------------|------------|
| **Conflict**   | Same leaf dot-path in ≥2 files | Merge | ✓ reports it | Remove from one file, or move to shared vault |
| **Collision**  | Same leaf name, unrelated dot-paths | Env vars | ✗ silent | Use `--prefixed` or a dot-path hint |
| **Shadow**     | Same leaf name, one path is ancestor of the other | Env vars | ✗ silent | No action needed — deeper wins intentionally |
