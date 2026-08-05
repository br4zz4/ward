# Vault-Qualified Scope Path & Shared Overlay — Implementation Plan

> **Spec:** `.project/specs/20260805133522_unprefixed_shared_path_scope.md`
> **Branch:** `feat/vault-qualified-scope`

**Goal:** Make the `exec`/`envs`/`tree` scope argument actually select subtree(s)
before flattening, with a `vault:secret-path` syntax where the vault is optional
(absent = overlay across all vaults).

**Architecture:** A new pure resolver in `internal/secrets` parses a scope into
`(vault, secret-path)` and selects one-or-more subtree roots from the merged tree.
Env-var flattening runs over the selected roots instead of the whole tree. The
merge engine is untouched (still whole-tree); only selection changes. CLI commands
parse one-or-more scopes and forward them.

**Tech stack:** Go 1.26, cobra CLI, `gopkg.in/yaml.v3`. Tests: Go stdlib testing;
e2e via `test/e2e/testutil` with `//go:build e2e`.

## Global constraints

- Commit messages: one line, max 60 chars, no AI mentions, Portuguese.
- Every behavior change gets a test asserting presence AND absence of leaves.
- Top-key == vault-name is already enforced (`validateVaultStructure`); no new
  enforcement code — the resolver relies on it.
- Map iteration is unordered: assert on normalized/sorted sets; overlay must be
  order-independent.

---

## File map

- Create `internal/secrets/scope.go` — scope parse + resolve to selected roots.
- Create `internal/secrets/scope_test.go` — unit tests for parse + resolve.
- Modify `internal/secrets/env.go` — add `ToFlatEnvEntriesScoped` over selected roots.
- Modify `internal/ward/engine.go` — `EnvVarsForScopes` forwarding resolved roots.
- Modify `internal/cmd/exec.go` — parse multiple scopes; call scoped resolution.
- Modify `internal/cmd/envs.go` — same for envs.
- Modify `internal/cmd/tree.go` — support `vault:` + unqualified overlay in the path arg.
- Create e2e fixtures under `test/e2e/exec/fixtures` and `test/e2e/envs/fixtures`.
- Modify `test/e2e/exec/exec_test.go`, `test/e2e/envs/envs_test.go`.
- Modify `README.md`, `docs/hierarchy.md`, `docs/conflicts-and-collisions.md`.

---

## Task 1: Scope parser

**Files:**
- Create: `internal/secrets/scope.go`
- Test: `internal/secrets/scope_test.go`

**Interfaces:**
- Produces: `type Scope struct { Vault string; SecretPath string }`,
  `func ParseScope(s string) Scope`

`Vault` is empty for unqualified/overlay scopes. Split on the **first** colon only;
a colon appearing after a dot is not a qualifier. Leading `:` → empty vault.

- [ ] **Step 1: Write the failing test**

```go
package secrets

import "testing"

func TestParseScope(t *testing.T) {
	cases := []struct {
		in         string
		wantVault  string
		wantSecret string
	}{
		{"commons:infra.staging", "commons", "infra.staging"},
		{"infra.staging", "", "infra.staging"},
		{":infra.staging", "", "infra.staging"},
		{"", "", ""},
		{"commons:", "commons", ""},
		{"infra.staging:weird", "", "infra.staging:weird"}, // colon after dot is not a qualifier
	}
	for _, c := range cases {
		got := ParseScope(c.in)
		if got.Vault != c.wantVault || got.SecretPath != c.wantSecret {
			t.Errorf("ParseScope(%q) = {%q,%q}, want {%q,%q}",
				c.in, got.Vault, got.SecretPath, c.wantVault, c.wantSecret)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/secrets/ -run TestParseScope
```

Expected: FAIL — `undefined: ParseScope`

- [ ] **Step 3: Write minimal implementation**

```go
package secrets

import "strings"

// Scope is a parsed exec/envs/tree argument: an optional vault qualifier plus a
// secret-path within it. An empty Vault means "overlay across every vault".
type Scope struct {
	Vault      string
	SecretPath string
}

// ParseScope splits s into a vault qualifier and a secret-path. The qualifier is
// the text before the first colon, but only when that colon precedes the first
// dot — a colon appearing inside the dotted path is part of the secret-path. A
// leading colon means an empty vault (overlay), same as no colon at all.
func ParseScope(s string) Scope {
	colon := strings.IndexByte(s, ':')
	if colon < 0 {
		return Scope{SecretPath: s}
	}
	dot := strings.IndexByte(s, '.')
	if dot >= 0 && dot < colon {
		return Scope{SecretPath: s}
	}
	return Scope{Vault: s[:colon], SecretPath: s[colon+1:]}
}
```

- [ ] **Step 4: Run to verify it passes**

```bash
go test ./internal/secrets/ -run TestParseScope
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/scope.go internal/secrets/scope_test.go
git commit -m "feat: parser de scope com vault opcional"
```

---

## Task 2: Scope resolver

**Files:**
- Modify: `internal/secrets/scope.go`
- Test: `internal/secrets/scope_test.go`

**Interfaces:**
- Consumes: `Scope` (Task 1); `map[string]*Node` merged tree; `Lookup` (existing).
- Produces: `type ScopedRoot struct { DotPath string; Node *Node }`,
  `func ResolveScopes(tree map[string]*Node, scopes []Scope) ([]ScopedRoot, error)`

Rules: empty scope list OR a single scope with empty vault AND empty secret-path →
one root at "" wrapping the whole tree. Qualified → `Lookup(tree, vault+"."+secret)`,
unknown vault or missing path → error. Unqualified with a secret-path → for each
top-key `T`, try `Lookup(tree, T+"."+secret)`, collect hits; zero hits → error
listing tried vaults. Multiple scopes → union, de-duplicated by DotPath.

- [ ] **Step 1: Write the failing test**

```go
func mkTree() map[string]*Node {
	leaf := func(v string) *Node { return &Node{Value: v} }
	return map[string]*Node{
		"commons": {Children: map[string]*Node{
			"infra": {Children: map[string]*Node{
				"staging":    {Children: map[string]*Node{"A": leaf("1")}},
				"production": {Children: map[string]*Node{"A": leaf("2")}},
			}},
		}},
		"trgclub": {Children: map[string]*Node{
			"infra": {Children: map[string]*Node{
				"staging": {Children: map[string]*Node{"B": leaf("3")}},
			}},
		}},
	}
}

func rootPaths(rs []ScopedRoot) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.DotPath
	}
	sort.Strings(out)
	return out
}

func TestResolveScopes_qualified(t *testing.T) {
	rs, err := ResolveScopes(mkTree(), []Scope{{Vault: "commons", SecretPath: "infra.staging"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := rootPaths(rs); len(got) != 1 || got[0] != "commons.infra.staging" {
		t.Errorf("got %v", got)
	}
}

func TestResolveScopes_qualified_unknown_vault(t *testing.T) {
	_, err := ResolveScopes(mkTree(), []Scope{{Vault: "nope", SecretPath: "infra.staging"}})
	if err == nil {
		t.Fatal("expected error for unknown vault")
	}
}

func TestResolveScopes_unqualified_overlay(t *testing.T) {
	rs, err := ResolveScopes(mkTree(), []Scope{{SecretPath: "infra.staging"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"commons.infra.staging", "trgclub.infra.staging"}
	if got := rootPaths(rs); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestResolveScopes_unqualified_no_hit(t *testing.T) {
	_, err := ResolveScopes(mkTree(), []Scope{{SecretPath: "nope.nope"}})
	if err == nil {
		t.Fatal("expected key-not-found")
	}
}

func TestResolveScopes_depth1_only(t *testing.T) {
	// infra.staging must match <vault>.infra.staging, never a deeper occurrence.
	tree := map[string]*Node{
		"commons": {Children: map[string]*Node{
			"regions": {Children: map[string]*Node{
				"us": {Children: map[string]*Node{
					"infra": {Children: map[string]*Node{
						"staging": {Children: map[string]*Node{"X": {Value: "deep"}}},
					}},
				}},
			}},
		}},
	}
	_, err := ResolveScopes(tree, []Scope{{SecretPath: "infra.staging"}})
	if err == nil {
		t.Fatal("deep match must not resolve at depth 1")
	}
}

func TestResolveScopes_multi_union_dedup(t *testing.T) {
	rs, err := ResolveScopes(mkTree(), []Scope{
		{Vault: "commons", SecretPath: "infra.staging"},
		{Vault: "commons", SecretPath: "infra.staging"},
		{Vault: "trgclub", SecretPath: "infra.staging"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := rootPaths(rs); len(got) != 2 {
		t.Errorf("expected 2 deduped roots, got %v", got)
	}
}

func TestResolveScopes_empty_whole_tree(t *testing.T) {
	rs, err := ResolveScopes(mkTree(), []Scope{{SecretPath: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].DotPath != "" {
		t.Errorf("expected whole-tree root, got %v", rootPaths(rs))
	}
}
```

Add imports `reflect`, `sort` to the test file.

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/secrets/ -run TestResolveScopes
```

Expected: FAIL — `undefined: ResolveScopes`

- [ ] **Step 3: Write minimal implementation**

```go
import (
	"fmt"
	"sort"
	"strings"
)

// ScopedRoot is a subtree selected by a scope: the dot-path where it sits in the
// merged tree and the node rooted there.
type ScopedRoot struct {
	DotPath string
	Node    *Node
}

// ResolveScopes turns one or more scopes into the set of subtree roots to flatten.
// An empty scope (no vault, no secret-path) selects the whole tree. A qualified
// scope selects exactly one vault's subtree (error if vault or path is unknown).
// An unqualified scope with a secret-path overlays: it selects that secret-path
// under every top-key that has it at depth 1 (error if none do). Multiple scopes
// union their roots, de-duplicated by dot-path.
func ResolveScopes(tree map[string]*Node, scopes []Scope) ([]ScopedRoot, error) {
	if len(scopes) == 0 {
		return []ScopedRoot{{DotPath: "", Node: &Node{Children: tree}}}, nil
	}
	seen := map[string]bool{}
	var out []ScopedRoot
	add := func(dotPath string, n *Node) {
		if seen[dotPath] {
			return
		}
		seen[dotPath] = true
		out = append(out, ScopedRoot{DotPath: dotPath, Node: n})
	}

	for _, sc := range scopes {
		if sc.Vault == "" && sc.SecretPath == "" {
			add("", &Node{Children: tree})
			continue
		}
		if sc.Vault != "" {
			full := sc.Vault
			if sc.SecretPath != "" {
				full = sc.Vault + "." + sc.SecretPath
			}
			node, err := Lookup(tree, full)
			if err != nil {
				return nil, err
			}
			add(full, node)
			continue
		}
		// unqualified overlay: match under every top-key at depth 1
		hit := false
		var tried []string
		for _, top := range sortedNodeKeys(tree) {
			tried = append(tried, top)
			full := top + "." + sc.SecretPath
			node, err := Lookup(tree, full)
			if err != nil {
				continue
			}
			add(full, node)
			hit = true
		}
		if !hit {
			return nil, &KeyNotFoundError{
				DotPath:   sc.SecretPath,
				AtPath:    "",
				Available: tried,
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DotPath < out[j].DotPath })
	return out, nil
}

var _ = fmt.Sprintf
var _ = strings.Split
```

Remove the unused `var _` lines if the compiler is happy without them (they guard
against import churn while iterating; delete before commit).

- [ ] **Step 4: Run to verify it passes**

```bash
go test ./internal/secrets/ -run TestResolveScopes
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/scope.go internal/secrets/scope_test.go
git commit -m "feat: resolver de scope para subtrees"
```

---

## Task 3: Flatten over selected roots

**Files:**
- Modify: `internal/secrets/env.go`
- Test: `internal/secrets/env_test.go` (create if absent)

**Interfaces:**
- Consumes: `[]ScopedRoot` (Task 2), existing `collectLeafs`, `resolveShadow`.
- Produces: `func ToFlatEnvEntriesScoped(roots []ScopedRoot) (map[string]EnvEntry, error)`

Behaves like `ToFlatEnvEntries` but only over the union of the selected roots'
subtrees. Each root's leaves are collected relative to the root (bare leaf names).
Cross-root same-name at unrelated paths → `EnvConflictError` (genuine collision).
No `preferPrefix` needed: scoping already narrows.

- [ ] **Step 1: Write the failing test**

```go
package secrets

import "testing"

func TestToFlatEnvEntriesScoped_overlay(t *testing.T) {
	tree := mkTree() // from scope_test.go (same package)
	roots, _ := ResolveScopes(tree, []Scope{{SecretPath: "infra.staging"}})
	got, err := ToFlatEnvEntriesScoped(roots)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["A"]; !ok {
		t.Error("expected A from commons.infra.staging")
	}
	if _, ok := got["B"]; !ok {
		t.Error("expected B from trgclub.infra.staging")
	}
	if len(got) != 2 {
		t.Errorf("expected exactly 2 leaves (no production leak), got %v", got)
	}
}

func TestToFlatEnvEntriesScoped_collision(t *testing.T) {
	leaf := func(v string) *Node { return &Node{Value: v} }
	tree := map[string]*Node{
		"commons": {Children: map[string]*Node{"infra": {Children: map[string]*Node{
			"staging": {Children: map[string]*Node{"A": leaf("1")}}}}}},
		"trgclub": {Children: map[string]*Node{"infra": {Children: map[string]*Node{
			"staging": {Children: map[string]*Node{"A": leaf("2")}}}}}},
	}
	roots, _ := ResolveScopes(tree, []Scope{{SecretPath: "infra.staging"}})
	if _, err := ToFlatEnvEntriesScoped(roots); err == nil {
		t.Fatal("expected collision on A across two vaults")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/secrets/ -run TestToFlatEnvEntriesScoped
```

Expected: FAIL — `undefined: ToFlatEnvEntriesScoped`

- [ ] **Step 3: Write minimal implementation**

```go
// ToFlatEnvEntriesScoped flattens only the leaves under the selected roots, using
// bare leaf names. Same-name leaves at unrelated dot-paths across the selected set
// are a collision (EnvConflictError), matching ToFlatEnvEntries semantics but over
// a scope-restricted set rather than the whole tree.
func ToFlatEnvEntriesScoped(roots []ScopedRoot) (map[string]EnvEntry, error) {
	merged := map[string]*Node{}
	for _, r := range roots {
		if r.Node.Children != nil {
			for k, v := range r.Node.Children {
				merged[k] = v
			}
			continue
		}
		// a leaf root (secret-path resolved directly to a leaf): key by last segment
		merged[LeafKey(r.DotPath)] = r.Node
	}
	return ToFlatEnvEntries(merged, "")
}
```

Note: keys from different roots may clash at the top of `merged` (e.g. both roots
have an `infra` child under different vaults). Because each root is already the
`infra.staging` node itself (not the vault node), the children are the leaves —
clashes there are exactly the genuine collisions we want `ToFlatEnvEntries` to
catch. Verify with the collision test.

- [ ] **Step 4: Run to verify it passes**

```bash
go test ./internal/secrets/ -run TestToFlatEnvEntriesScoped
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/env.go internal/secrets/env_test.go
git commit -m "feat: flatten de env sobre roots do scope"
```

---

## Task 4: Engine forwarding

**Files:**
- Modify: `internal/ward/engine.go`

**Interfaces:**
- Consumes: `secrets.ResolveScopes`, `secrets.ToFlatEnvEntriesScoped`,
  `secrets.ParseScope`.
- Produces: `func (e *Engine) EnvVarsForScopes(r *MergeResult, prefixed bool, scopes []string) (map[string]secrets.EnvEntry, error)`

Parses each raw scope string, resolves against `r.Tree`, then flattens. With
`prefixed`, emit full-path names over the selected roots (reuse `ToEnvEntries`
on the union subtree; full paths keep vault prefix so no collision).

- [ ] **Step 1: Write the failing test** — covered by e2e in Tasks 6–7; add a thin
  unit if desired. Minimal unit:

```go
// internal/ward/engine_scope_test.go
package ward

import "testing"

func TestEnvVarsForScopes_smoke(t *testing.T) {
	// build a MergeResult by hand or via a fixture loader; assert overlay union.
	// (Kept minimal; full coverage is in e2e Tasks 6-7.)
	t.Skip("covered by e2e; placeholder for engine wiring")
}
```

- [ ] **Step 2: Run** — `go build ./...` must fail until the method exists.

```bash
go build ./...
```

Expected: FAIL — `EnvVarsForScopes` referenced by CLI (after Task 5) / undefined.

- [ ] **Step 3: Write minimal implementation**

```go
// EnvVarsForScopes resolves env vars restricted to the given scopes. An empty
// slice (or a single empty scope) selects the whole tree. With prefixed=true the
// names are full dot-paths over the selected roots; otherwise bare leaf names,
// with genuine cross-scope collisions surfaced as an error.
func (e *Engine) EnvVarsForScopes(r *MergeResult, prefixed bool, scopes []string) (map[string]secrets.EnvEntry, error) {
	parsed := make([]secrets.Scope, 0, len(scopes))
	for _, s := range scopes {
		if s == "" {
			continue
		}
		parsed = append(parsed, secrets.ParseScope(s))
	}
	roots, err := secrets.ResolveScopes(r.Tree, parsed)
	if err != nil {
		return nil, err
	}
	if prefixed {
		union := map[string]*secrets.Node{}
		for _, root := range roots {
			if root.DotPath == "" {
				return secrets.ToEnvEntries(r.Tree), nil
			}
			placeAtPath(union, root.DotPath, root.Node)
		}
		return secrets.ToEnvEntries(union), nil
	}
	return secrets.ToFlatEnvEntriesScoped(roots)
}

// placeAtPath rebuilds the nesting for dotPath in dst so prefixed names keep the
// full vault-qualified path.
func placeAtPath(dst map[string]*secrets.Node, dotPath string, node *secrets.Node) {
	parts := strings.Split(dotPath, ".")
	cur := dst
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = node
			return
		}
		if cur[p] == nil {
			cur[p] = &secrets.Node{Children: map[string]*secrets.Node{}}
		}
		cur = cur[p].Children
	}
}
```

- [ ] **Step 4: Run to verify it builds**

```bash
go build ./... && go test ./internal/ward/
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ward/engine.go internal/ward/engine_scope_test.go
git commit -m "feat: engine resolve env por scopes"
```

---

## Task 5: CLI multi-scope parsing (exec + envs)

**Files:**
- Modify: `internal/cmd/exec.go`, `internal/cmd/envs.go`

**Interfaces:**
- Consumes: `eng.EnvVarsForScopes` (Task 4).
- Produces: exec/envs accept zero-or-more scopes before `--` / as positional args.

For `exec`, extend `parseExecArgs` to return `[]string` scopes (all pre-`--`
non-flag tokens). For `envs`, change `MaximumNArgs(1)` to `ArbitraryArgs` and pass
all args. `MergeScoped` still receives the first scope for conflict scoping (or ""),
keeping current conflict-narrowing behavior.

- [ ] **Step 1: Write the failing test** — e2e in Tasks 6–7 cover this. Build-level
  check here: after edits, `go build ./...` succeeds and existing exec/envs e2e
  still pass for single-scope.

- [ ] **Step 2: Run**

```bash
go build ./... && go test -tags e2e ./test/e2e/exec/... ./test/e2e/envs/...
```

Expected: existing single-scope tests PASS; new behavior added below.

- [ ] **Step 3: Write minimal implementation**

`exec.go` — change `parseExecArgs` signature to `(scopes []string, cmdArgs []string, prefixed bool)`:

```go
func parseExecArgs(args []string) (scopes []string, cmdArgs []string, prefixed bool) {
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--prefixed" {
			prefixed = true
			continue
		}
		rest = append(rest, a)
	}
	for i, a := range rest {
		if a == "--" {
			scopes = rest[:i]
			cmdArgs = rest[i+1:]
			return
		}
	}
	cmdArgs = rest
	return
}
```

In the `Run`, replace the single-path resolution:

```go
scopes, cmdArgs, prefixed := parseExecArgs(args)
// ...
firstScope := ""
if len(scopes) > 0 {
	firstScope = scopes[0]
}
result, err := eng.MergeScoped(firstScope)
if err != nil {
	fatal(err)
}
printEngineWarnings(eng)

entries, err := eng.EnvVarsForScopes(result, prefixed, scopes)
if err != nil {
	fatal(stampEnvCommand(err, "exec"))
}
envVars := make(map[string]string, len(entries))
for k, e := range entries {
	envVars[k] = e.Value
}
```

`envs.go` — accept arbitrary args:

```go
Args: cobra.ArbitraryArgs,
// ...
result, err := eng.MergeScoped(firstOrEmpty(args))
// ...
entries, err := eng.EnvVarsForScopes(result, prefixed, args)
```

Add helper `firstOrEmpty` in `envs.go` (or `helpers.go`):

```go
func firstOrEmpty(a []string) string {
	if len(a) > 0 {
		return a[0]
	}
	return ""
}
```

- [ ] **Step 4: Run**

```bash
go build ./... && go test -tags e2e ./test/e2e/exec/... ./test/e2e/envs/...
```

Expected: PASS (single-scope unchanged; multi-scope exercised in Tasks 6–7).

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/exec.go internal/cmd/envs.go internal/cmd/helpers.go
git commit -m "feat: exec e envs aceitam multiplos scopes"
```

---

## Task 6: E2E — envs scope behavior

**Files:**
- Create: `test/e2e/envs/fixtures/overlay/.ward/config.yaml`
- Create: `test/e2e/envs/fixtures/overlay/.ward/vaults/commons/infra.plain.ward`
- Create: `test/e2e/envs/fixtures/overlay/.ward/vaults/trgclub/infra.plain.ward`
- Modify: `test/e2e/envs/envs_test.go`

Fixture `config.yaml`:

```yaml
vaults:
  - name: commons
    path: .ward/vaults/commons
  - name: trgclub
    path: .ward/vaults/trgclub
```

`commons/infra.plain.ward`:

```yaml
commons:
  infra:
    staging:
      TF_VAR_aws_shared_vpc_id: vpc-xxxx
      TF_VAR_aws_region: us-east-1
    production:
      TF_VAR_aws_region: us-west-2
```

`trgclub/infra.plain.ward`:

```yaml
trgclub:
  infra:
    staging:
      TF_VAR_api_url: https://trg.example
      TF_VAR_rails_master_key: rk-trg
```

- [ ] **Step 1: Write the failing tests**

```go
func fixO() string { return testutil.FixtureDir("envs", "overlay") }

func TestEnvs_qualified_only_commons(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fixO(), "envs", "commons:infra.staging")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	s := testutil.StripANSI(out)
	if !testutil.Contains(s, "TF_VAR_aws_shared_vpc_id") {
		t.Error("expected commons leaf present")
	}
	if testutil.Contains(s, "TF_VAR_api_url") {
		t.Error("trgclub leaf must NOT leak into qualified commons scope")
	}
}

func TestEnvs_unqualified_overlay_union(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fixO(), "envs", "infra.staging")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	s := testutil.StripANSI(out)
	for _, k := range []string{"TF_VAR_aws_shared_vpc_id", "TF_VAR_api_url"} {
		if !testutil.Contains(s, k) {
			t.Errorf("expected %s in overlay union", k)
		}
	}
}

func TestEnvs_leading_colon_equals_unqualified(t *testing.T) {
	a, _, _ := testutil.Run(t, bin, fixO(), "envs", ":infra.staging")
	b, _, _ := testutil.Run(t, bin, fixO(), "envs", "infra.staging")
	if testutil.StripANSI(a) != testutil.StripANSI(b) {
		t.Error(":infra.staging must equal infra.staging")
	}
}

func TestEnvs_overlay_no_cross_env_collision(t *testing.T) {
	// The bug: production leaf collides with staging and aborts. Must not happen.
	out, _, code := testutil.Run(t, bin, fixO(), "envs", "infra.staging")
	s := testutil.StripANSI(out)
	if code != 0 {
		t.Fatalf("overlay must not error on cross-env collision, exit %d: %s", code, s)
	}
	if testutil.Contains(s, "us-west-2") {
		t.Error("production value must not appear in staging overlay")
	}
}

func TestEnvs_multi_scope_union(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fixO(), "envs",
		"commons:infra.staging", "trgclub:infra.staging")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	s := testutil.StripANSI(out)
	for _, k := range []string{"TF_VAR_aws_shared_vpc_id", "TF_VAR_api_url"} {
		if !testutil.Contains(s, k) {
			t.Errorf("expected %s in multi-scope union", k)
		}
	}
}

func TestEnvs_unknown_vault_errors(t *testing.T) {
	_, _, code := testutil.Run(t, bin, fixO(), "envs", "nope:infra.staging")
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown vault")
	}
}

func TestEnvs_dot_only_no_longer_targets_vault(t *testing.T) {
	// compat break: commons.infra.staging is now unqualified → looks for
	// <top-key>.commons.infra.staging → no hit → error.
	_, _, code := testutil.Run(t, bin, fixO(), "envs", "commons.infra.staging")
	if code == 0 {
		t.Fatal("dot-only vault path must no longer resolve")
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -tags e2e ./test/e2e/envs/... -run TestEnvs_
```

Expected: new tests FAIL (behavior not wired / fixtures new) until Tasks 3–5 land.

- [ ] **Step 3: Implementation** — none; fixtures + Tasks 3–5 make these pass.

- [ ] **Step 4: Run to verify they pass**

```bash
go test -tags e2e ./test/e2e/envs/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add test/e2e/envs/
git commit -m "test: e2e de scope e overlay em envs"
```

---

## Task 7: E2E — exec scope behavior

**Files:**
- Create: `test/e2e/exec/fixtures/overlay/` mirroring Task 6's fixture.
- Modify: `test/e2e/exec/exec_test.go`

- [ ] **Step 1: Write the failing tests**

```go
func fixOX() string { return testutil.FixtureDir("exec", "overlay") }

func TestExec_overlay_injects_union(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fixOX(), "exec", "infra.staging", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	for _, kv := range []string{"TF_VAR_aws_shared_vpc_id=vpc-xxxx", "TF_VAR_api_url=https://trg.example"} {
		if !testutil.Contains(out, kv) {
			t.Errorf("expected %s injected, got: %s", kv, out)
		}
	}
	if testutil.Contains(out, "us-west-2") {
		t.Error("production value leaked into staging exec")
	}
}

func TestExec_qualified_no_cross_vault(t *testing.T) {
	out, _, code := testutil.Run(t, bin, fixOX(), "exec", "commons:infra.staging", "--", "env")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if testutil.Contains(out, "TF_VAR_api_url") {
		t.Error("trgclub leaf must not appear in qualified commons exec")
	}
}

func TestExec_genuine_collision_errors(t *testing.T) {
	// collision fixture: same leaf in both vaults' infra.staging.
	_, _, code := testutil.Run(t, bin,
		testutil.FixtureDir("exec", "overlay-collision"), "exec", "infra.staging", "--", "env")
	if code == 0 {
		t.Fatal("genuine cross-vault collision must abort exec")
	}
}

func TestExec_collision_resolves_with_prefixed(t *testing.T) {
	out, _, code := testutil.Run(t, bin,
		testutil.FixtureDir("exec", "overlay-collision"), "exec", "--prefixed", "infra.staging", "--", "env")
	if code != 0 {
		t.Fatalf("prefixed must resolve collision, exit %d: %s", code, out)
	}
}
```

Create `test/e2e/exec/fixtures/overlay-collision/` with the same `TF_VAR_aws_region`
leaf under both `commons.infra.staging` and `trgclub.infra.staging`.

- [ ] **Step 2: Run to verify they fail**

```bash
go test -tags e2e ./test/e2e/exec/... -run TestExec_
```

Expected: new tests FAIL until fixtures + wiring present.

- [ ] **Step 3: Implementation** — fixtures only; logic from Tasks 3–5.

- [ ] **Step 4: Run to verify they pass**

```bash
go test -tags e2e ./test/e2e/exec/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add test/e2e/exec/
git commit -m "test: e2e de scope e overlay em exec"
```

---

## Task 8: Tree scope + overlay

**Files:**
- Modify: `internal/cmd/tree.go`
- Create/Modify: `test/e2e/tree/fixtures/overlay/`, `test/e2e/tree/tree_test.go`

`tree <scope>` currently calls `GetAtPath(result, args[0])` (single Lookup). Extend
to parse the scope: qualified → single Lookup as today; unqualified → resolve
overlay roots and print each. Unknown vault / no-hit → the resolver's error.

- [ ] **Step 1: Write the failing test**

```go
func TestTree_qualified_shows_only_vault(t *testing.T) {
	out, _, code := testutil.Run(t, bin,
		testutil.FixtureDir("tree", "overlay"), "tree", "commons:infra.staging")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	s := testutil.StripANSI(out)
	if testutil.Contains(s, "trgclub") {
		t.Error("qualified tree must not show other vaults")
	}
}

func TestTree_unqualified_shows_overlay(t *testing.T) {
	out, _, code := testutil.Run(t, bin,
		testutil.FixtureDir("tree", "overlay"), "tree", "infra.staging")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	s := testutil.StripANSI(out)
	for _, v := range []string{"commons", "trgclub"} {
		if !testutil.Contains(s, v) {
			t.Errorf("overlay tree should include %s", v)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test -tags e2e ./test/e2e/tree/... -run TestTree_
```

Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

In `runTree`, when `len(args) == 1`, replace the single `GetAtPath` with scope
resolution:

```go
if len(args) == 1 {
	sc := secrets.ParseScope(args[0])
	roots, rerr := secrets.ResolveScopes(result.Tree, []secrets.Scope{sc})
	if rerr != nil {
		fatal(rerr)
	}
	for _, r := range roots {
		fmt.Println(r.DotPath)
		printTreeWithOrigin(r.Node, 1, conflicts, r.DotPath, envCollisions)
	}
	return
}
```

- [ ] **Step 4: Run to verify it passes**

```bash
go test -tags e2e ./test/e2e/tree/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/tree.go test/e2e/tree/
git commit -m "feat: tree suporta scope e overlay"
```

---

## Task 9: Completion

**Files:**
- Modify: `internal/cmd/complete.go`

Suggest both `vault:` qualified and unqualified secret-paths. Keep it minimal:
list vault names with trailing `:` plus the existing dot-path suggestions.

- [ ] **Step 1: Write the failing test** — completion is hard to e2e; a unit over
  the completion function asserting a `vault:` entry appears.

```go
// internal/cmd/complete_test.go (add case)
func TestComplete_offers_vault_colon(t *testing.T) {
	got := completionCandidates(testCfgWithVaults("commons", "trgclub"))
	if !containsStr(got, "commons:") || !containsStr(got, "trgclub:") {
		t.Errorf("expected vault: candidates, got %v", got)
	}
}
```

(Adapt to the actual completion helper names in `complete.go`; extract a
`completionCandidates` helper if needed to make it testable.)

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./internal/cmd/ -run TestComplete_offers_vault_colon
```

Expected: FAIL

- [ ] **Step 3: Write minimal implementation** — add `<vault>:` entries to the
  candidate list in the completion path.

- [ ] **Step 4: Run to verify it passes**

```bash
go test ./internal/cmd/ -run TestComplete_offers_vault_colon
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/complete.go internal/cmd/complete_test.go
git commit -m "feat: completion sugere vault qualificado"
```

---

## Task 10: Docs + README asdf

**Files:**
- Modify: `docs/hierarchy.md`, `docs/conflicts-and-collisions.md`, `README.md`

- [ ] **Step 1** — `docs/hierarchy.md`: add a "Scope path" section documenting the
  `[vault:]secret-path` grammar, qualified (strict) vs unqualified (overlay),
  depth-1 match, and the top-key==vault-name invariant it relies on.

- [ ] **Step 2** — `docs/conflicts-and-collisions.md`: note that scoping narrows the
  leaf set, so cross-env collisions no longer abort a scoped lookup; genuine
  same-scope collisions still error, resolvable with `--prefixed`.

- [ ] **Step 3** — `README.md`: replace "dot-path" wording with "scope"; document
  `vault:` qualifier, unqualified overlay, multi-scope; document the compat break
  (dot-only no longer targets a vault). Add the asdf install block:

````markdown
**asdf**

```sh
asdf plugin add ward https://github.com/br4zz4/asdf-ward
asdf install ward latest
asdf global ward latest
```
````

Confirm the asdf plugin repo URL before committing; if none exists yet, document
the generic `asdf` binary install path instead and note the plugin is planned.

- [ ] **Step 4: Verify docs build/lint** (markdown only — visual check).

- [ ] **Step 5: Commit**

```bash
git add docs/hierarchy.md docs/conflicts-and-collisions.md README.md
git commit -m "docs: scope path, overlay e asdf no readme"
```

---

## Final verification

```bash
go build ./...
go test ./...
go test -tags e2e ./test/e2e/...
```

Expected: all PASS. Manually reproduce the original bug scenario to confirm the fix:

```bash
# in a fixture with commons.infra.{staging,production} + trgclub.infra.staging
ward exec infra.staging -- env | grep TF_VAR   # union, no collision, production absent
```

---

## Self-review

**Spec coverage:**
- Grammar + colon rule → Task 1.
- Qualified/unqualified/overlay/multi resolution + depth-1 + errors → Task 2.
- Scope-restricted flatten + genuine collision → Task 3.
- Prefixed over scope + engine wiring → Task 4.
- exec/envs multi-scope CLI → Task 5.
- All behavior-table rows + regression + compat break → Tasks 6–7.
- tree scope/overlay → Task 8.
- completion → Task 9.
- docs (hierarchy, conflicts) + README + asdf → Task 10.

**Open items to confirm during implementation:**
- asdf plugin repo URL (Task 10) — verify or fall back to binary install docs.
- `complete.go` helper names (Task 9) — adapt test to actual structure.
- `placeAtPath` (Task 4) lives in `ward` package referencing `secrets.Node`;
  confirm it compiles with the exported field access.

**Parallelization:** Tasks 1→2→3→4→5 are a dependency chain (each consumes the
prior). Tasks 6, 7, 8 depend on 5 but are independent of each other. Task 9 and 10
are independent of everything after 5. Suggested parallel wave after Task 5:
{6, 7, 8, 9, 10}.
