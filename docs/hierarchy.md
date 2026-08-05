# Hierarchy

`ward` determines the relationship between files by inspecting their **YAML content structure**, not their file paths. File paths are convention — you can organise them however makes sense for your project.

---

## What makes a file an ancestor

A file A is an ancestor of file B if:

1. They share at least one root key (e.g. both start with `company:`).
2. Every map branch in A that also exists in B is structurally compatible — meaning the sub-maps have the same shape or A is shallower.
3. Branches in A that do not exist in B are irrelevant, not conflicting.

### Example

```yaml
# company.ward
company:
  name: acme
  sectors:
    one:
      name: sector 1
```

```yaml
# company/sectors/one/staging.ward
company:
  sectors:
    one:
      name: sector 1 override
      staging:
        database_url: postgres://staging.acme.internal/app
```

`company.ward` is an ancestor of `staging.ward` because:
- Both share root key `company` ✓
- `company.ward` has `sectors.one` as a map branch — `staging.ward` also has `sectors.one` ✓
- `company.ward` has `sectors.one.name` as a leaf — leaves don't affect ancestry ✓

---

## What makes two files siblings (not ancestors)

```yaml
# staging.ward
company:
  sectors:
    one:
      staging:
        database_url: postgres://staging

# production.ward
company:
  sectors:
    one:
      production:
        database_url: postgres://production
```

`staging.ward` and `production.ward` are **siblings** — same depth, same structural path up to `sectors.one`, different branches below it. Neither is an ancestor of the other.

If both define the same key at the same level, it is a conflict.

---

## Specificity

When multiple files could be ancestors of a target, they are ordered by **specificity** — the total number of dot-paths in their content. Files with fewer dot-paths (less content) are merged first; files with more dot-paths (more specific) are merged last and take precedence.

```
company.ward         specificity = 5  (company, company.name, company.sectors,
                                       company.sectors.one, company.sectors.one.name)
staging.ward         specificity = 6  (company, company.sectors, company.sectors.one,
                                       company.sectors.one.name, company.sectors.one.staging,
                                       company.sectors.one.staging.database_url, ...)
```

---

## Scope

A **scope** argument tells the path commands (`get`, `set`, `unset`, `exec`,
`envs`/`secrets`, `tree`, `inspect`) which secrets to operate on. The grammar
is:

```
scope = [vault:]secret-path
```

### Three equivalent ways to pass a scope

The same scope can be provided in any of three forms, in any path command:

1. **Positional** — `ward get commons:infra.KEY`
2. **Flag** — `ward get -s commons:infra.KEY` (or `--scope`)
3. **Split** — `ward get --vault commons --secret infra.KEY`

The `-s/--scope` flag and the `--vault/--secret` pair are mutually exclusive
with the inline positional form; pick one.

### Qualified vs unqualified

A scope with a `vault:` prefix is **qualified** and **strict** — it addresses
exactly one vault:

```sh
ward get commons:infra.staging.DATABASE_URL   # only the commons vault
```

A scope with **no** `vault:` prefix has no qualifier. Its behaviour depends on
the command:

- **Multi-value reads** (`exec`, `envs`/`secrets`, `tree`) — **overlay**: the
  same secret-path under *every* vault that has it (at depth 1 below the vault)
  is unioned together.
- **`get`** (single value) — **single lookup**: if the secret-path exists in
  exactly one vault it is returned; if it exists in more than one vault, `ward`
  reports an ambiguity error.

A leading colon (`:infra.staging`) is exactly the same as writing no prefix
(`infra.staging`) — it is optional sugar.

### Universal rule: a plain dot is never a vault

**A pure dot-path never identifies a vault.** `commons.infra.staging` is a
literal secret-path, *not* "vault commons, secret-path infra.staging". To
address a vault you must use the `vault:` prefix (or `--vault`).

> **Compat break:** previously the first segment of a dot-path was treated as
> the vault. That is no longer the case. Rewrite `commons.infra.staging` as
> `commons:infra.staging` when you mean the commons vault.

This rule is universal — it applies to `get`, `set`, `unset`, `exec`,
`envs` (now `secrets`), `tree`, and `inspect` alike.

### Overlay example (two vaults)

Given two vaults, `commons` and `trgclub`, both defining an `infra.staging`
subtree:

```sh
ward exec infra.staging -- deploy
```

This is an unqualified multi-value read, so it **overlays** both vaults: the
leaves of `commons.infra.staging` and `trgclub.infra.staging` are unioned and
exposed together. To restrict to a single vault, qualify it:

```sh
ward exec commons:infra.staging -- deploy   # only commons
```

### Multiple scopes

`exec` and `secrets` accept several scopes at once; the result is their union:

```sh
ward exec commons:infra.staging trgclub:infra.staging -- deploy
```

TAB completion is available for scope arguments in all path commands.

---

## Without a scope

```sh
ward exec -- app
ward secrets
```

All vaults are merged and all leaves are exposed. Conflicts between any two files at the same specificity level are errors. Use this for projects with a single unambiguous hierarchy.

---

## Recommended file layout

There is no required structure. These layouts both work:

```
# By environment
secrets/
  base.ward
  staging.ward
  production.ward

# By service/sector
secrets/
  company.ward
  company/
    api/
      staging.ward
      production.ward
    workers/
      staging.ward
      production.ward
```

The second layout makes the hierarchy explicit in the directory tree, which helps when inspecting with `ward tree`.
