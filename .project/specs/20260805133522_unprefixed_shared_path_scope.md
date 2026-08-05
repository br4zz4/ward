# Vault-Qualified Scope Path & Shared Overlay

**Date:** 2026-08-05
**Status:** proposed

## Problem

Teams want generic infra secrets to live once in a shared `commons` vault and be
consumed by every app without duplication. Each vault namespaces its values under
its own top-key (`commons.infra.staging.*`, `trgclub.infra.staging.*`), and ward
already **enforces** that a vault's files start with the vault name (see
`validateVaultStructure` — a file in vault `commons` must begin with `commons.…`,
or `exec`/`envs`/`get`/`tree` refuse to run). So top-key == vault name is a
guaranteed invariant, not a convention.

Despite that, the scope path passed to `exec`/`envs`/`tree` does **not** scope the
merged tree. It is only a collision tie-breaker (`preferPrefix`). Flat env
resolution (`ToFlatEnvEntries`) always walks the **entire** merged tree and emits
every leaf by its bare key name. Two consequences:

1. **Vault prefix is porous.** `ward exec trgclub.infra.staging` already injects
   `commons.*` leaves — not by design, but because the whole tree is flattened.
   A value the caller did not ask for leaks in.

2. **Cross-env collisions abort the command.** When `commons` defines the same leaf
   under two environments (`commons.infra.staging.TF_VAR_aws_region` and
   `commons.infra.production.TF_VAR_aws_region`), `ward envs infra.staging` fails
   with an env-var collision and injects **nothing** — even though `production` is
   irrelevant to a `staging` lookup. Verified empirically; this is the exact
   "commons value not injected" symptom in
   `WARD_FEATURE_REQUEST_shared_path_merge.md`. The report's own diagnosis
   ("different prefix, merge never unifies") is incorrect — merge unifies fine; an
   unrelated cross-env collision aborts flattening.

The request proposed three vault-config mechanisms (overlay role, path include,
strip-prefix merge mode). All add config surface to solve a problem the merge
engine does not have. The real gap is that the scope path never restricts the leaf
set, and there is no syntax to say "this vault, explicitly" vs "any vault, overlay".

## Goal

Introduce a **vault-qualified scope path** and make it actually scope the tree
before flattening.

Syntax: `[vault:]secret-path`. The optional `vault:` qualifier (colon-separated)
targets a single vault; its absence (or a leading `:`) means "match under every
vault (overlay)". Terminology used throughout: **scope** = `<vault>:<secret-path>`;
**secret-path** = the part after the colon; **vault** = the part before it.

- **Qualified** — `commons:infra.staging` → strict. Selects only the subtree under
  `commons`'s namespace at `infra.staging`. No leaves from other vaults.

- **Unqualified** — `infra.staging` → shared overlay. Selects `infra.staging`
  under **every** vault top-key that has it, unioned. This is how a caller pulls
  `commons.infra.staging` **and** `trgclub.infra.staging` together, while ignoring
  `commons.infra.production`. Ancestor levels inherit as today (a shallower
  `commons.infra` leaf still applies). Real leaf collisions across vaults still
  error (guarantee unchanged).

As an explicit alternative, allow **multiple scope paths** in one call
(`ward exec commons:infra.staging trgclub:infra.staging`) — the union of each
scoped subtree — for callers who want precise control over the overlay set.

Everything respects each vault's key: a vault whose key is unavailable is skipped
with a warning (existing lenient load), so an unqualified overlay only unions
vaults the caller can decrypt.

### Naming

The argument is a **scope** (a.k.a. **scope-path**), replacing "dot-path" in
docs/help, since it carries an optional `vault:` qualifier and is no longer only
dots. Its grammar:

```
scope       = [ vault ":" ] secret-path
vault       = a configured vault name
secret-path = dot-separated path within a vault (e.g. infra.staging)
```

Examples:
- `commons:infra.staging` → vault `commons`, secret-path `infra.staging`
- `infra.staging` → no vault (overlay), secret-path `infra.staging`
- `:infra.staging` → same as `infra.staging` (leading colon is optional sugar for
  "no vault"; **not** an error)

## Behavior specification

Merged tree has top-keys `commons` and `trgclub`.

| Command | Selected leaves |
|---|---|
| `ward exec commons:infra.staging` | only `commons.infra.staging.*` (+ inherited `commons.infra.*`, `commons.*` ancestors). No `trgclub.*`. |
| `ward exec infra.staging` | union of `commons.infra.staging.*` and `trgclub.infra.staging.*` (+ each vault's inherited ancestors). |
| `ward exec commons:infra.staging trgclub:infra.staging` | union of both explicit subtrees. |
| `ward exec trgclub.infra.staging` | **compat break:** dot-only, first segment is no longer treated as a vault → unqualified overlay looking for `<top-key>.trgclub.infra.staging` under every vault → 0 hits → key-not-found. To target the vault, use `trgclub:infra.staging`. |
| `ward exec` (no path) | whole tree, as today. |

Collision rule (unchanged in spirit): if the **selected** leaf set has the same
bare env-var name at two unrelated paths, it errors. Because scoping removes
irrelevant subtrees (e.g. `production`), the spurious cross-env collision
disappears; only genuine same-scope collisions remain — and those should error.
Decision: **no silent "app wins" overlay** — same-leaf collision across vaults
errors, preserving ward's core guarantee.

### Qualified vs unqualified detection

Detection is **syntactic**, not heuristic. Split the scope on the first `:`:

- Has `:` with a **non-empty** vault → qualified. The vault must be a configured
  vault, else error listing known vaults. Selects `Lookup(tree, vault + "." +
  secret-path)`.
- Has `:` with an **empty** vault (`:infra.staging`) → unqualified (same as no
  colon; the leading `:` is optional sugar).
- No `:` → unqualified. For each vault top-key `T` in the tree, attempt
  `Lookup(tree, T + "." + secret-path)`; collect every hit. Union. Zero hits →
  key-not-found listing the vaults tried.

Because ward already enforces top-key == vault name, "vault top-key in the tree"
and "configured vault name" are the same set — the qualifier resolves
deterministically.

Match is **anchored at depth 1** below the vault key: unqualified `infra.staging`
matches `<vault>.infra.staging`, not a deeper `<vault>.regions.us.infra.staging`.
Keeps the union predictable.

## Approach

Add a scope-selection step between merge and flattening. Merge stays whole-tree
(unchanged); scoping picks which subtree roots feed the flattener.

Resolve a scope path `P` against tree `M`:

1. Empty `P` → whole tree (current behavior).
2. `P` has `vault:` → qualified: `Lookup(M, vault + "." + rest)` → one subtree root.
   Unknown vault → error.
3. No `:` → unqualified: for each top-key `T` of `M`, try `Lookup(M, T+"."+P)`;
   collect hits. Zero hits → key-not-found listing tried vaults.
4. Multiple paths → resolve each, union the roots.

The flattener runs over the **selected roots** (each re-based so leaves keep their
relative names), not the full tree. Collision detection runs over this reduced set.

## Implementation

### Location

- `internal/secrets/scope.go` (new) — parse `[vault:]path`; resolve one-or-more
  scope paths against a merged tree into a set of `(dotPath, *Node)` roots.
- `internal/secrets/env.go` — flatten entry point taking selected roots instead of
  the whole tree; reuse `collectLeafs`/`resolveShadow`.
- `internal/ward/engine.go` — `EnvVarsPrefer`/`MergeScoped` forward resolved scope;
  `preferPrefix` kept only for the no-scope path.
- `internal/cmd/exec.go`, `envs.go`, `tree.go` — parse one-or-more scope paths.
  `exec` has `DisableFlagParsing`; extend `parseExecArgs` to collect all pre-`--`
  non-flag tokens as scope paths.
- `internal/cmd/complete.go` — completion should suggest both `vault:` qualified
  and unqualified forms.

### Notes

- **Qualified strict scope is a behavior change:** `ward exec commons:infra.staging`
  no longer leaks `trgclub.*`, and dot-only `trgclub.infra.staging` no longer
  targets a vault. Call out in docs + changelog.
- Unqualified overlay is the shared capability the request wanted — no new config
  keys, no `shared:`, no `include:`.
- Enforcement of top-key == vault name already exists (`validateVaultStructure`);
  this feature depends on it but needs no new enforcement code. Document the
  dependency.
- `tree <scope>` should render only the selected subtree(s), matching scope
  semantics (today `tree <path>` shows the whole tree).

## Tests

This change alters observable behavior in several ways (strict qualified scope,
compat break on dot-only, overlay union, collision reduction). Tests must blind
**each** transition and assert on the exact selected leaf set — not just "runs
without error". Every row in the Behavior specification table gets at least one
test. Assert both presence of expected leaves **and** absence of leaves that must
not leak (the leak is the bug we are fixing).

### E2E (`test/e2e/exec`, `test/e2e/envs`)

Fixtures: `.plain.ward`, top-keys `commons` and `trgclub`, with a shared leaf name
across envs to exercise the collision path.

Positive selection:
- Qualified `commons:infra.staging` → commons staging leaves present; **assert**
  trgclub leaves absent.
- Unqualified `infra.staging` → union of commons + trgclub staging leaves present.
- Leading-colon `:infra.staging` → identical result to `infra.staging`.
- Multiple paths `commons:infra.staging trgclub:infra.staging` → union present.
- Inherited ancestor: a `commons.infra` (one level up) leaf appears under
  `infra.staging`.
- `--prefixed` over each of the above → full-path names, correct scope.

Regression (the reported bug):
- Unqualified `infra.staging` with `commons.infra.production` also defined →
  production leaves **absent** and **no** collision error; exec injects the
  staging set (assert exit 0 AND the env vars are actually present, since the bug
  was "exit 0 but nothing injected").

Guarantee preserved:
- Genuine collision: same leaf in `commons.infra.staging` and
  `trgclub.infra.staging`, resolved via `infra.staging` → errors with the
  collision message; exec injects nothing (assert non-zero + message).
- Same collision but `--prefixed` → resolves (distinct full-path names).

Errors / compat break:
- Qualified unknown vault `nope:infra.staging` → error listing known vaults.
- Unqualified path matching zero vaults → key-not-found listing tried vaults.
- Dot-only `trgclub.infra.staging` → treated unqualified → key-not-found; error
  text should hint the `vault:` form (compat-break UX).
- Empty/`--`-only exec still errors "requires a command after --".

### Unit (`internal/secrets`)

- Scope parser: `vault:secret-path` split on first colon; no-colon → unqualified;
  colon-after-first-dot must NOT be read as a qualifier; leading `:secret-path`
  (empty vault) → unqualified (equal to no colon); empty string → whole tree.
- Resolver: qualified hit; qualified unknown vault; unqualified multi-hit;
  unqualified single-hit; unqualified no-hit; depth-1 anchoring (a deeper
  `<vault>.x.infra.staging` must NOT match `infra.staging`); multi-path union
  dedups overlapping roots; re-basing keeps leaf relative names.
- Flatten-over-roots: leaf set equals union of selected roots only (no full-tree
  leakage); collision detection sees only the reduced set.

### Determinism

Union and multi-path order must not affect output (map iteration is unordered) —
tests assert on sorted/normalized leaf sets, and a genuine-collision test must be
stable regardless of which vault is visited first.

## Documentation

- `docs/hierarchy.md` — scope path syntax `[vault:]path`; qualified (strict) vs
  unqualified (overlay); depth-1 match; note the top-key==vault-name invariant it
  relies on.
- `docs/conflicts-and-collisions.md` — scoping reduces the leaf set, so cross-env
  collisions no longer abort a scoped lookup; genuine same-scope collisions error.
- `README.md`:
  - Replace "dot-path" wording with "scope path"; document `vault:` qualifier,
    unqualified overlay, and multi-path `exec`/`envs`.
  - Document the compat break (dot-only no longer targets a vault).
  - Add asdf install instructions (bundled task below).

## Out of scope

- New vault-config keys (`shared:`, `include:`) — request Options A/B not pursued.
- Silent "app wins over commons" overlay for same-leaf collisions — decided
  against; real collisions error.
- New enforcement of top-key == vault name — already exists.
- Changing merge or ancestry semantics — merge stays whole-tree; only selection
  changes.
- Windows path/shell concerns.

## Bundled: README asdf install

Add an asdf installation section to `README.md` next to Homebrew / Debian / Alpine
/ Go. Docs-only, no code change, requested together with this feature.
