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

## Dot-path scoping

A **dot-path** argument tells `ward exec`, `ward envs`, and `ward view` to scope the merge to a specific subtree of the merged data.

```sh
ward exec company.sectors.one.staging -- deploy
```

All vaults are merged as usual. The dot-path then selects the subtree at `company.sectors.one.staging` and exposes only its leaves as env vars, using flat leaf names (`DATABASE_URL`, not `COMPANY_SECTORS_ONE_STAGING_DATABASE_URL`).

TAB completion is available for dot-path arguments in all three commands.

---

## Without a dot-path

```sh
ward exec -- app
ward envs
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

The second layout makes the hierarchy explicit in the directory tree, which helps when inspecting with `ward view`.
