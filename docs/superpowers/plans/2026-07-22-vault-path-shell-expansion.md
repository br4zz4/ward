# Vault Path Shell Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand shell expressions (`$VAR`, `${VAR}`, `$(cmd)`) in vault `path` fields at config load time by delegating to `sh`.

**Architecture:** Add `expandPath(string) (string, error)` in `internal/config/config.go`. Call it for each vault's `Path` inside `Load`, after `yaml.Unmarshal` and before duplicate checks. Fast path skips `sh` when path has no `$`.

**Tech Stack:** Go stdlib — `os/exec`, `strings`.

## Global Constraints

- Only `vault.Path` is expanded — `key_file`, `key_env`, `default_dir` are out of scope.
- Duplicate-path dedup runs after expansion.
- No Windows support — `sh` only.
- No external dependencies.

---

### Task 1: `expandPath` — unit tests + implementation

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `expandPath(path string) (string, error)` — unexported, package-level function in `internal/config`

- [ ] **Step 1: Write failing tests**

Add to `internal/config/config_test.go`:

```go
func TestExpandPath_no_dollar_passthrough(t *testing.T) {
	got, err := expandPath("/some/static/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/some/static/path" {
		t.Errorf("expected passthrough, got %q", got)
	}
}

func TestExpandPath_dollar_var(t *testing.T) {
	t.Setenv("WARD_TEST_DIR", "/tmp/testdir")
	got, err := expandPath("$WARD_TEST_DIR/vault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/testdir/vault" {
		t.Errorf("expected /tmp/testdir/vault, got %q", got)
	}
}

func TestExpandPath_braced_var(t *testing.T) {
	t.Setenv("WARD_TEST_DIR", "/tmp/testdir")
	got, err := expandPath("${WARD_TEST_DIR}/vault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/testdir/vault" {
		t.Errorf("expected /tmp/testdir/vault, got %q", got)
	}
}

func TestExpandPath_command_substitution(t *testing.T) {
	got, err := expandPath("$(echo /tmp)/vault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/vault" {
		t.Errorf("expected /tmp/vault, got %q", got)
	}
}

func TestExpandPath_undefined_var_empty(t *testing.T) {
	os.Unsetenv("WARD_UNDEFINED_XYZ")
	got, err := expandPath("$WARD_UNDEFINED_XYZ/vault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/vault" {
		t.Errorf("expected /vault (empty var), got %q", got)
	}
}

func TestExpandPath_bad_command_returns_error(t *testing.T) {
	_, err := expandPath("$(false)/vault")
	if err == nil {
		t.Fatal("expected error from failing command substitution")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/config/... -run TestExpandPath -v
```

Expected: compile error or `undefined: expandPath`.

- [ ] **Step 3: Add `expandPath` to `internal/config/config.go`**

Add the import `"os/exec"` to the import block, then add after the existing helper functions (before or after the `Load` function — outside it):

```go
// expandPath expands shell expressions in path using sh, matching shell semantics.
// Supports $VAR, ${VAR}, and $(cmd). Returns path unchanged when it contains no $.
func expandPath(path string) (string, error) {
	if !strings.Contains(path, "$") {
		return path, nil
	}
	out, err := exec.Command("sh", "-c", `echo -n "$1"`, "--", path).Output()
	if err != nil {
		return "", fmt.Errorf("expanding vault path %q: %w", path, err)
	}
	return string(out), nil
}
```

Note: `exec.Command("sh", "-c", `echo -n "$1"`, "--", path)` passes `path` as `$1` to the shell script — no escaping needed, fully safe with spaces and special chars.

- [ ] **Step 4: Run tests to verify they pass**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/config/... -run TestExpandPath -v
```

Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add expandPath for shell expression expansion"
```

---

### Task 2: Wire `expandPath` into `Load`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Consumes: `expandPath(path string) (string, error)` from Task 1

- [ ] **Step 1: Write failing test**

Add to `internal/config/config_test.go`:

```go
func TestLoad_vault_path_expands_env_var(t *testing.T) {
	dir := t.TempDir()
	vaultDir := filepath.Join(dir, "myvault")
	if err := os.MkdirAll(vaultDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WARD_TEST_VAULT", vaultDir)

	path := writeTemp(t, `vaults:
  - name: test
    path: $WARD_TEST_VAULT
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Vaults[0].Path != vaultDir {
		t.Errorf("expected %q, got %q", vaultDir, cfg.Vaults[0].Path)
	}
}

func TestLoad_vault_path_expand_error_propagates(t *testing.T) {
	path := writeTemp(t, `vaults:
  - name: test
    path: $(false)/vault
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error from failing command substitution in vault path")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/config/... -run TestLoad_vault_path -v
```

Expected: FAIL — paths are not expanded yet.

- [ ] **Step 3: Call `expandPath` in `Load`**

In `internal/config/config.go`, inside `Load`, find the backward-compat block that derives vault names (around line 130). Add expansion immediately before it:

```go
// expand shell expressions in vault paths
for i := range cfg.Vaults {
    expanded, err := expandPath(cfg.Vaults[i].Path)
    if err != nil {
        return nil, fmt.Errorf("vault %q path: %w", cfg.Vaults[i].Name, err)
    }
    cfg.Vaults[i].Path = expanded
}

// backward-compat: derive name from path when absent
for i := range cfg.Vaults {
```

- [ ] **Step 4: Run all config tests**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/config/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Run full test suite**

```bash
/Users/oporpino/.asdf/shims/go test ./...
```

Expected: all tests PASS (or only pre-existing failures unrelated to this change).

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: expand shell expressions in vault path at load time"
```

---

### Task 3: Documentation + push

**Files:**
- Modify: `docs/configuration.md`
- Modify: `README.md`

- [ ] **Step 1: Update `docs/configuration.md`**

Find the vault `path` field description (around the vaults table) and add:

```markdown
| `path` | Directory containing `.ward` files for this vault. Supports shell expansion: `$VAR`, `${VAR}`, `$(cmd)`. |
```

Also add an example block after the vaults section:

```markdown
**Using environment variables in vault paths:**

```yaml
vaults:
  - name: myproject
    path: .ward/vaults/myproject
  - name: commons
    path: $COMMONS_DIR/.ward/vaults/commons
```
```

- [ ] **Step 2: Update `README.md`**

Find the vaults section and add the same example and note about shell expansion in `path`.

- [ ] **Step 3: Commit and push**

```bash
git add docs/configuration.md README.md
git commit -m "docs: document shell expansion in vault path"
git push origin main
```
