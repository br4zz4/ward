# Mandatory Encryption, Per-Vault Key Warnings, and `.plain.ward` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every `*.ward` file mandatorily encrypted, add a plaintext `.plain.ward` file type (structured YAML, treated as one extension), turn a missing per-vault key into a warning (skip that vault, render the rest) and no-key-anywhere-with-encrypted-files into a fatal error.

**Architecture:** All read commands funnel through `Engine.load()` (`internal/ward/engine.go`), so warning/skip logic lives there. File classification (`.plain.ward` vs `.ward` vs raw file-secret) lives in `internal/secrets/filekey.go`. A new `PlainDecryptor` (`internal/sops`) reads `.plain.ward` as plaintext and rejects age-armored content; a new `RequireKeyDecryptor` replaces the silent `MockDecryptor` fallback so keyless encrypted files error instead of feeding armor to the YAML parser. Per-vault key availability is tracked at decryptor-build time so `load()` can decide skip-vs-fail.

**Tech Stack:** Go stdlib — `bytes`, `path/filepath`, `strings`. Existing `filippo.io/age` via `internal/age`.

## Global Constraints

- ward only discovers files whose extension is `.ward` (`filepath.Ext == ".ward"`). Unchanged.
- `.plain.ward` is a single extension: strip `.plain.ward` whole for location/dot-path. `config.plain.ward` → root key `config`; no `.plain` segment appears in the tree or env vars.
- `.plain.ward` is always structured YAML, never a raw file-secret, even when its inner name contains a dot.
- age armor header sentinel: `-----BEGIN AGE ENCRYPTED FILE-----` (already used by `internal/age.isArmored` and `internal/cmd/validate.go`).
- Warnings go to `stderr`; merged output (stdout) stays valid.
- No new external dependencies.
- Use the go binary at `/Users/oporpino/.asdf/shims/go` (the `go` shell alias is `git checkout` in this environment).
- Commit language: Portuguese, single line, ≤60 chars, no AI mention (project rule).

---

### Task 1: File classification helpers for `.plain.ward`

**Files:**
- Modify: `internal/secrets/filekey.go`
- Modify: `internal/secrets/filekey_test.go`

**Interfaces:**
- Produces:
  - `IsPlainFile(path string) bool` — true when basename ends with `.plain.ward`
  - `StripWardSuffix(path string) string` — basename minus `.ward`, or minus `.plain.ward` for plain files (e.g. `config.plain.ward` → `config`, `main.ward` → `main`)
  - `OriginalFilename(wardFile string) (string, bool)` — updated to return `("", false)` for `.plain.ward`

- [ ] **Step 1: Write the failing tests**

Add to `internal/secrets/filekey_test.go`:

```go
func TestIsPlainFile(t *testing.T) {
	cases := map[string]bool{
		"config.plain.ward":       true,
		"a/b/config.plain.ward":   true,
		"main.ward":               false,
		"sa.json.ward":            false,
		"notplain.ward":           false,
		"plain.ward":              false, // no name before .plain
	}
	for in, want := range cases {
		if got := IsPlainFile(in); got != want {
			t.Errorf("IsPlainFile(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestStripWardSuffix(t *testing.T) {
	cases := map[string]string{
		"config.plain.ward":     "config",
		"a/b/config.plain.ward": "config",
		"main.ward":             "main",
		"sa.json.ward":          "sa.json",
	}
	for in, want := range cases {
		if got := StripWardSuffix(in); got != want {
			t.Errorf("StripWardSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOriginalFilename_plain_is_not_file_secret(t *testing.T) {
	if _, ok := OriginalFilename("config.plain.ward"); ok {
		t.Error("expected .plain.ward to not be a file-secret")
	}
	if _, ok := OriginalFilename("sa.json.ward"); !ok {
		t.Error("expected sa.json.ward to still be a file-secret")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/secrets/ -run 'TestIsPlainFile|TestStripWardSuffix|TestOriginalFilename_plain' -v
```

Expected: compile error / `undefined: IsPlainFile` / `undefined: StripWardSuffix`.

- [ ] **Step 3: Implement the helpers**

Edit `internal/secrets/filekey.go`. Add a constant and the two new functions, and guard `OriginalFilename`:

```go
const plainSuffix = ".plain.ward"

// IsPlainFile reports whether path is a .plain.ward file (structured plaintext).
// A bare "plain.ward" (no name before .plain) is not treated as plain.
func IsPlainFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, plainSuffix) && base != plainSuffix
}

// StripWardSuffix returns the basename without its ward suffix.
// For .plain.ward files the whole ".plain.ward" is removed; otherwise ".ward".
// e.g. "config.plain.ward" -> "config", "main.ward" -> "main".
func StripWardSuffix(path string) string {
	base := filepath.Base(path)
	if IsPlainFile(path) {
		return strings.TrimSuffix(base, plainSuffix)
	}
	return strings.TrimSuffix(base, ".ward")
}
```

Then in `OriginalFilename`, add an early return for plain files at the top of the function body:

```go
func OriginalFilename(wardFile string) (string, bool) {
	if IsPlainFile(wardFile) {
		return "", false
	}
	base := filepath.Base(wardFile)
	without := strings.TrimSuffix(base, ".ward")
	if without == base || !strings.Contains(without, ".") {
		return "", false
	}
	return without, true
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/secrets/ -run 'TestIsPlainFile|TestStripWardSuffix|TestOriginalFilename' -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/filekey.go internal/secrets/filekey_test.go
git commit -m "feat: helpers de classificacao .plain.ward"
```

---

### Task 2: Loader parses `.plain.ward` as structured YAML

**Files:**
- Modify: `internal/secrets/loader.go:35`
- Modify: `internal/secrets/integration_test.go`

**Interfaces:**
- Consumes: `IsPlainFile` from Task 1
- Produces: `Load` treats `.plain.ward` as structured (skips the file-secret branch)

**Note:** `Load` already routes file-secrets via `OriginalFilename`. Since Task 1 makes `OriginalFilename` return false for `.plain.ward`, plain files already fall through to the structured YAML branch. This task adds an explicit guard for clarity and a regression test.

- [ ] **Step 1: Write the failing test**

Add to `internal/secrets/integration_test.go`:

```go
func TestLoad_plain_ward_is_structured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.plain.ward")
	content := "app:\n  config:\n    port: \"8080\"\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	pf, err := Load(path, "app", dir, sops.PlainDecryptor{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// structured: nested map, not a single raw value
	appNode, ok := pf.Data["app"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured app map, got %T", pf.Data["app"])
	}
	cfgNode, ok := appNode["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected structured config map, got %T", appNode["config"])
	}
	if cfgNode["port"] != "8080" {
		t.Errorf("expected port 8080, got %v", cfgNode["port"])
	}
}
```

(This test consumes `sops.PlainDecryptor` from Task 3. If running strictly in order, temporarily use `sops.MockDecryptor{}` here and switch to `PlainDecryptor{}` after Task 3 — but since both read plaintext, `MockDecryptor{}` also passes. Use `MockDecryptor{}` to keep this task self-contained, then Task 3 adds `PlainDecryptor`.)

Use `sops.MockDecryptor{}` in this test for now.

- [ ] **Step 2: Run test to verify it passes or fails**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/secrets/ -run TestLoad_plain_ward_is_structured -v
```

Expected: PASS already (because Task 1 made `OriginalFilename` return false for plain). If it PASSES, the guard in Step 3 is defensive; still add it.

- [ ] **Step 3: Add an explicit guard in `Load`**

In `internal/secrets/loader.go`, change the file-secret condition at line 35 from:

```go
	if orig, ok := OriginalFilename(path); ok {
```

to:

```go
	if orig, ok := OriginalFilename(path); ok && !IsPlainFile(path) {
```

- [ ] **Step 4: Run the secrets tests**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/secrets/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/loader.go internal/secrets/integration_test.go
git commit -m "feat: loader trata .plain.ward como estruturado"
```

---

### Task 3: `PlainDecryptor` — plaintext reader that rejects encrypted content

**Files:**
- Create: `internal/sops/plain.go`
- Create: `internal/sops/plain_test.go`

**Interfaces:**
- Produces: `sops.PlainDecryptor` with `Decrypt(path string) ([]byte, error)` and `Encrypt(path string, plaintext []byte) error`. `Decrypt` errors if the file is age-armored; `Encrypt` writes plaintext as-is.

- [ ] **Step 1: Write the failing test**

Create `internal/sops/plain_test.go`:

```go
package sops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlainDecryptor_reads_plaintext(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	os.WriteFile(p, []byte("app:\n  x: \"1\"\n"), 0644)
	got, err := PlainDecryptor{}.Decrypt(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(got), "app:") {
		t.Errorf("expected plaintext passthrough, got %q", got)
	}
}

func TestPlainDecryptor_rejects_encrypted(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	os.WriteFile(p, []byte("-----BEGIN AGE ENCRYPTED FILE-----\nabc\n"), 0644)
	_, err := PlainDecryptor{}.Decrypt(p)
	if err == nil {
		t.Fatal("expected error for encrypted .plain.ward")
	}
	if !strings.Contains(err.Error(), "must not be encrypted") {
		t.Errorf("expected 'must not be encrypted' error, got %v", err)
	}
}

func TestPlainDecryptor_encrypt_writes_plaintext(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	if err := (PlainDecryptor{}).Encrypt(p, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "hello" {
		t.Errorf("expected plaintext write, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/sops/ -run TestPlainDecryptor -v
```

Expected: `undefined: PlainDecryptor`.

- [ ] **Step 3: Implement `PlainDecryptor`**

Create `internal/sops/plain.go`:

```go
package sops

import (
	"bytes"
	"fmt"
	"os"
)

// PlainDecryptor reads .plain.ward files as plaintext YAML. It refuses to read
// age-armored content: a .plain.ward file must never be encrypted.
type PlainDecryptor struct{}

var ageArmorHeader = []byte("-----BEGIN AGE ENCRYPTED FILE-----")

func (PlainDecryptor) Decrypt(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if bytes.HasPrefix(bytes.TrimSpace(data), ageArmorHeader) {
		return nil, fmt.Errorf("%s is a plain file but is encrypted — a .plain.ward file must not be encrypted", path)
	}
	return data, nil
}

// Encrypt writes plaintext to path unchanged (.plain.ward is never encrypted).
func (PlainDecryptor) Encrypt(path string, plaintext []byte) error {
	return os.WriteFile(path, plaintext, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/sops/ -run TestPlainDecryptor -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sops/plain.go internal/sops/plain_test.go
git commit -m "feat: PlainDecryptor le texto puro e rejeita cifrado"
```

---

### Task 4: `RequireKeyDecryptor` — replaces silent plaintext fallback

**Files:**
- Create: `internal/sops/requirekey.go`
- Create: `internal/sops/requirekey_test.go`

**Interfaces:**
- Produces: `sops.RequireKeyDecryptor` with `Decrypt(path string) ([]byte, error)`. For `.plain.ward` files it delegates to plaintext read (rejecting armor); for any other `.ward` file it always errors "no encryption key". This is the no-key fallback that must never feed armor to the YAML parser.

- [ ] **Step 1: Write the failing test**

Create `internal/sops/requirekey_test.go`:

```go
package sops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequireKeyDecryptor_errors_on_encrypted_ward(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "main.ward")
	os.WriteFile(p, []byte("-----BEGIN AGE ENCRYPTED FILE-----\nabc\n"), 0644)
	_, err := RequireKeyDecryptor{}.Decrypt(p)
	if err == nil {
		t.Fatal("expected error when no key for an encrypted .ward")
	}
	if !strings.Contains(err.Error(), "no encryption key") {
		t.Errorf("expected 'no encryption key' error, got %v", err)
	}
}

func TestRequireKeyDecryptor_reads_plain_ward(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	os.WriteFile(p, []byte("app:\n  x: \"1\"\n"), 0644)
	got, err := RequireKeyDecryptor{}.Decrypt(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(got), "app:") {
		t.Errorf("expected plaintext passthrough, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/sops/ -run TestRequireKeyDecryptor -v
```

Expected: `undefined: RequireKeyDecryptor`.

- [ ] **Step 3: Implement `RequireKeyDecryptor`**

Create `internal/sops/requirekey.go`:

```go
package sops

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RequireKeyDecryptor is the fallback used when no encryption key is available.
// It reads .plain.ward files as plaintext (delegating to PlainDecryptor) but
// errors on any other .ward file, so encrypted content is never silently parsed
// as YAML. The error is sentinel-detectable via IsNoKeyError.
type RequireKeyDecryptor struct{}

func (RequireKeyDecryptor) Decrypt(path string) ([]byte, error) {
	if strings.HasSuffix(filepath.Base(path), ".plain.ward") {
		return PlainDecryptor{}.Decrypt(path)
	}
	return nil, &NoKeyError{Path: path}
}

// NoKeyError signals that path is encrypted but no key was available.
type NoKeyError struct{ Path string }

func (e *NoKeyError) Error() string {
	return fmt.Sprintf("%s requires an encryption key but none is configured", e.Path)
}

// IsNoKeyError reports whether err is (or wraps) a NoKeyError.
func IsNoKeyError(err error) bool {
	for err != nil {
		if _, ok := err.(*NoKeyError); ok {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
```

Note: the test asserts the message contains "no encryption key"; adjust the assertion or the message so they agree. Change `NoKeyError.Error()` to include that phrase:

```go
func (e *NoKeyError) Error() string {
	return fmt.Sprintf("%s: no encryption key configured for this file", e.Path)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/sops/ -run TestRequireKeyDecryptor -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sops/requirekey.go internal/sops/requirekey_test.go
git commit -m "feat: RequireKeyDecryptor exige chave para cifrados"
```

---

### Task 5: Wire decryptor selection to plain + require-key fallback

**Files:**
- Modify: `internal/cmd/helpers.go:97-113` (`decryptorFor`)
- Modify: `internal/cmd/helpers.go:255-275` (`decryptorForVault`)
- Modify: `internal/ward/engine.go:222-232` (`decryptorFor`)

**Interfaces:**
- Consumes: `sops.RequireKeyDecryptor`, `sops.PlainDecryptor` (Tasks 3-4), `secrets.IsPlainFile` (Task 1)
- Produces: engine picks `PlainDecryptor` for `.plain.ward` files regardless of vault key; keyless vaults get `RequireKeyDecryptor` instead of `MockDecryptor`

- [ ] **Step 1: Change the global no-key fallback**

In `internal/cmd/helpers.go`, in `decryptorFor` (around line 102-103), replace:

```go
	if keyFile == "" {
		return sops.MockDecryptor{}, nil
	}
```

with:

```go
	if keyFile == "" {
		return sops.RequireKeyDecryptor{}, nil
	}
```

- [ ] **Step 2: Make the engine choose `PlainDecryptor` per plain file**

In `internal/ward/engine.go`, update `decryptorFor` (line 222) so `.plain.ward` files always use plaintext regardless of the vault decryptor:

```go
func (e *Engine) decryptorFor(path string) sops.Decryptor {
	if secrets.IsPlainFile(path) {
		return sops.PlainDecryptor{}
	}
	if len(e.vaultDec) > 0 {
		vi := buildVaultRootIndex(e.cfg)(path)
		if vi.name != "" {
			if dec, ok := e.vaultDec[vi.name]; ok {
				return dec
			}
		}
	}
	return e.dec
}
```

Ensure `internal/ward/engine.go` imports `"github.com/br4zz4/ward/internal/secrets"` (it already does — it references `secrets.ParsedFile`).

- [ ] **Step 3: Build to verify it compiles**

```bash
/Users/oporpino/.asdf/shims/go build ./...
```

Expected: builds clean.

- [ ] **Step 4: Run unit tests**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/...
```

Expected: PASS (test fixtures using real keys still decrypt; `.plain.ward` routed to plaintext).

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/helpers.go internal/ward/engine.go
git commit -m "feat: engine usa plain/require-key por arquivo"
```

---

### Task 6: Skip keyless-encrypted files with a warning; fail when nothing decrypts

**Files:**
- Modify: `internal/secrets/loader.go` (add `LoadAllLenient`)
- Modify: `internal/ward/engine.go:264-282` (`load`)
- Modify: `internal/ward/engine.go:36-39` (surface warnings) — add package-level warning sink
- Test: `internal/ward/engine_test.go` (create if absent)

**Interfaces:**
- Consumes: `sops.IsNoKeyError` (Task 4)
- Produces:
  - `secrets.LoadAllLenient(paths, vaultFor, decFor) ([]ParsedFile, []LoadSkip, error)` where `type LoadSkip struct { Path string; Err error }`
  - `Engine.Warnings() []string` returning human-readable per-file skip warnings collected on the last `load()`

- [ ] **Step 1: Write the failing test**

Create `internal/ward/engine_test.go`:

```go
package ward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/sops"
)

// A vault with an encrypted file but no key is skipped with a warning;
// a plain file in another vault still loads.
func TestLoad_skips_keyless_encrypted_and_warns(t *testing.T) {
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	vB := filepath.Join(dir, "b")
	os.MkdirAll(vA, 0755)
	os.MkdirAll(vB, 0755)
	// encrypted file in vault a (armored, no key)
	os.WriteFile(filepath.Join(vA, "main.ward"),
		[]byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n"), 0644)
	// plain file in vault b
	os.WriteFile(filepath.Join(vB, "conf.plain.ward"),
		[]byte("b:\n  ok: \"1\"\n"), 0644)

	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults: []config.Source{
			{Name: "a", Path: vA},
			{Name: "b", Path: vB},
		},
	}
	// no vault decryptors, global fallback requires key
	eng := NewEngine(cfg, sops.RequireKeyDecryptor{})
	res, err := eng.MergeForView()
	if err != nil {
		t.Fatalf("expected no error (b is loadable), got %v", err)
	}
	if res.Tree["b"] == nil {
		t.Error("expected vault b to be present in tree")
	}
	warns := eng.Warnings()
	if len(warns) != 1 || !strings.Contains(warns[0], "a") {
		t.Errorf("expected one warning mentioning vault a, got %v", warns)
	}
}

// No key anywhere and only encrypted files → error.
func TestLoad_all_keyless_encrypted_errors(t *testing.T) {
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	os.MkdirAll(vA, 0755)
	os.WriteFile(filepath.Join(vA, "main.ward"),
		[]byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n"), 0644)

	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults:     []config.Source{{Name: "a", Path: vA}},
	}
	eng := NewEngine(cfg, sops.RequireKeyDecryptor{})
	if _, err := eng.MergeForView(); err == nil {
		t.Fatal("expected error when nothing is decryptable")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/ward/ -run 'TestLoad_skips_keyless|TestLoad_all_keyless' -v
```

Expected: `undefined: (*Engine).Warnings` / current `load` returns an error for the first case.

- [ ] **Step 3: Add `LoadAllLenient` to the loader**

In `internal/secrets/loader.go`, add after `LoadAll`:

```go
// LoadSkip records a file that could not be loaded and was skipped.
type LoadSkip struct {
	Path string
	Err  error
}

// LoadAllLenient is like LoadAll but, instead of aborting on the first file
// error, collects failures as skips and returns the successfully loaded files.
// Non-file errors (e.g. malformed YAML) are also reported as skips.
func LoadAllLenient(paths []string, vaultFor func(path string) (string, string), decFor func(path string) sops.Decryptor) ([]ParsedFile, []LoadSkip) {
	files := make([]ParsedFile, 0, len(paths))
	var skips []LoadSkip
	for _, p := range paths {
		name, root := "", ""
		if vaultFor != nil {
			name, root = vaultFor(p)
		}
		pf, err := Load(p, name, root, decFor(p))
		if err != nil {
			skips = append(skips, LoadSkip{Path: p, Err: err})
			continue
		}
		files = append(files, pf)
	}
	return files, skips
}
```

- [ ] **Step 4: Add a warnings field to `Engine` and rewrite `load`**

In `internal/ward/engine.go`, add a `warnings` field to the `Engine` struct (near line 20 where `dec`/`vaultDec` are declared):

```go
	warnings []string
```

Add a `Warnings` accessor near `SourcePaths`:

```go
// Warnings returns human-readable warnings collected during the last load
// (e.g. vaults skipped because no key was available).
func (e *Engine) Warnings() []string {
	return e.warnings
}
```

Rewrite `load()` (lines 264-282) to use the lenient loader, classify skips, and decide skip-vs-fail:

```go
func (e *Engine) load() ([]secrets.ParsedFile, error) {
	paths, err := secrets.Discover(sourcePaths(e.cfg))
	if err != nil {
		return nil, fmt.Errorf("discovering files: %w", err)
	}
	vaultFor := buildVaultRootIndex(e.cfg)
	vaultRootFor := func(path string) (string, string) {
		vi := vaultFor(path)
		return vi.name, vi.root
	}
	decFor := func(path string) sops.Decryptor {
		return e.decryptorFor(path)
	}

	files, skips := secrets.LoadAllLenient(paths, vaultRootFor, decFor)

	e.warnings = nil
	var keySkips, otherSkips []secrets.LoadSkip
	for _, s := range skips {
		if sops.IsNoKeyError(s.Err) {
			keySkips = append(keySkips, s)
		} else {
			otherSkips = append(otherSkips, s)
		}
	}

	// A non-key error (malformed YAML, plain file that is encrypted, etc.) is fatal.
	if len(otherSkips) > 0 {
		return nil, fmt.Errorf("loading %s: %w", otherSkips[0].Path, otherSkips[0].Err)
	}

	// Encrypted files with no key: if NOTHING loaded, that's fatal; otherwise warn per vault.
	if len(keySkips) > 0 {
		if len(files) == 0 {
			return nil, fmt.Errorf("no encryption key found — set WARD_KEY or provide .ward/<vault>.key")
		}
		byVault := map[string]int{}
		for _, s := range keySkips {
			byVault[vaultFor(s.Path).name]++
		}
		for name, n := range byVault {
			e.warnings = append(e.warnings, fmt.Sprintf("missing key for vault %s — %d file(s) skipped", name, n))
		}
	}

	return files, nil
}
```

Remove the now-unused `LoadAll` call site if `load` was its only caller. Check with:

```bash
grep -rn "secrets.LoadAll\b" internal/ test/
```

If `LoadAll` still has other callers (tests), leave it; otherwise it stays as public API. Do not delete it.

- [ ] **Step 5: Run the engine tests**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/ward/ -run 'TestLoad_skips_keyless|TestLoad_all_keyless' -v
```

Expected: PASS.

- [ ] **Step 6: Run all internal tests**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/secrets/loader.go internal/ward/engine.go internal/ward/engine_test.go
git commit -m "feat: pula vault sem chave com aviso, erra sem nada legivel"
```

---

### Task 7: Print warnings on stderr from read commands

**Files:**
- Modify: `internal/cmd/helpers.go` (add `printEngineWarnings`)
- Modify: `internal/cmd/view.go`, `internal/cmd/get.go`, `internal/cmd/envs.go`, `internal/cmd/export.go`, `internal/cmd/inspect.go`, `internal/cmd/exec.go`

**Interfaces:**
- Consumes: `Engine.Warnings()` (Task 6)
- Produces: `printEngineWarnings(eng *ward.Engine)` — writes each warning to stderr with the `⚠` prefix

**Note:** Warnings are populated during `load()`, which runs inside `Merge*`/`Inspect*`. Call `printEngineWarnings` after the merge/inspect call in each command, before printing stdout output.

- [ ] **Step 1: Add the helper**

In `internal/cmd/helpers.go`, add:

```go
// printEngineWarnings writes any load-time warnings (e.g. skipped vaults) to stderr.
func printEngineWarnings(eng *ward.Engine) {
	for _, w := range eng.Warnings() {
		fmt.Fprintf(os.Stderr, "  %s⚠ %s%s\n", clrYellow, w, clrReset)
	}
}
```

Confirm `ward` and `os` are imported in `helpers.go` (they are).

- [ ] **Step 2: Call it in each read command**

For each of `view.go`, `get.go`, `envs.go`, `export.go`, `inspect.go`, `exec.go`: locate the line where the engine's merge/inspect result is obtained (e.g. `res, err := eng.MergeForView()` or `eng.Merge()` / `eng.Inspect*`), and immediately after the error check, add:

```go
	printEngineWarnings(eng)
```

Use `grep` to find the exact call sites:

```bash
grep -n "eng.Merge\|eng.Inspect\|MergeForView\|MergeScoped" internal/cmd/view.go internal/cmd/get.go internal/cmd/envs.go internal/cmd/export.go internal/cmd/inspect.go internal/cmd/exec.go
```

Add `printEngineWarnings(eng)` after each result is successfully obtained (after the `if err != nil` block).

- [ ] **Step 3: Build**

```bash
/Users/oporpino/.asdf/shims/go build ./...
```

Expected: builds clean.

- [ ] **Step 4: Run cmd tests**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/cmd/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/
git commit -m "feat: exibe avisos de vault sem chave no stderr"
```

---

### Task 8: `.plain.ward` structure validation

**Files:**
- Modify: `internal/cmd/validate.go:110-121` (`expectedFileDotPath`), `:135-146` (`leadingKeyPath` encrypted-skip)
- Test: `internal/cmd/validate_test.go` (create if absent)

**Interfaces:**
- Consumes: `secrets.StripWardSuffix` (Task 1)
- Produces: `expectedFileDotPath` strips `.plain.ward` whole; `leadingKeyPath` reads `.plain.ward` as plaintext (does not skip it as "encrypted")

- [ ] **Step 1: Write the failing test**

Create `internal/cmd/validate_test.go`:

```go
package cmd

import (
	"path/filepath"
	"testing"
)

func TestExpectedFileDotPath_plain_strips_suffix(t *testing.T) {
	vaultAbs := "/proj/.ward/vaults/app"
	file := filepath.Join(vaultAbs, "config.plain.ward")
	got := expectedFileDotPath("app", vaultAbs, file)
	if got != "app.config" {
		t.Errorf("expected app.config, got %q", got)
	}
}

func TestExpectedFileDotPath_plain_with_subdir(t *testing.T) {
	vaultAbs := "/proj/.ward/vaults/app"
	file := filepath.Join(vaultAbs, "svc", "db.plain.ward")
	got := expectedFileDotPath("app", vaultAbs, file)
	if got != "app.svc.db" {
		t.Errorf("expected app.svc.db, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/cmd/ -run TestExpectedFileDotPath_plain -v
```

Expected: FAIL — current code trims only `.ward`, yielding `app.config.plain`.

- [ ] **Step 3: Update `expectedFileDotPath`**

In `internal/cmd/validate.go`, change the body of `expectedFileDotPath` (lines 112-121). Replace:

```go
	rel = strings.TrimSuffix(rel, ".ward")
	parts := strings.Split(rel, string(filepath.Separator))
```

with:

```go
	dir := filepath.Dir(rel)
	base := secrets.StripWardSuffix(rel)
	rel = base
	if dir != "." {
		rel = filepath.Join(dir, base)
	}
	parts := strings.Split(rel, string(filepath.Separator))
```

Confirm `internal/cmd/validate.go` imports `"github.com/br4zz4/ward/internal/secrets"`. If not, add it.

- [ ] **Step 4: Ensure `leadingKeyPath` reads `.plain.ward` (does not treat it as encrypted)**

`leadingKeyPath` (line 135) already reads the file and only skips when it starts with the age header. A `.plain.ward` is plaintext, so it is read normally — no change needed. Add a regression test to lock this in. Append to `validate_test.go`:

```go
import "os"

func TestLeadingKeyPath_reads_plain_ward(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.plain.ward")
	os.WriteFile(p, []byte("app:\n  config:\n    x: \"1\"\n"), 0644)
	got, err := leadingKeyPath(p, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "app.config" {
		t.Errorf("expected app.config, got %q", got)
	}
}
```

(Merge the `os` import into the existing import block rather than adding a second `import`.)

- [ ] **Step 5: Run tests to verify they pass**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/cmd/ -run 'TestExpectedFileDotPath|TestLeadingKeyPath' -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cmd/validate.go internal/cmd/validate_test.go
git commit -m "feat: valida estrutura de .plain.ward"
```

---

### Task 9: Clean up dead `filepath.Glob` in discovery

**Files:**
- Modify: `internal/secrets/discover.go:22-25,39`

**Interfaces:** none (internal cleanup)

- [ ] **Step 1: Remove the dead Glob call**

In `internal/secrets/discover.go`, delete the unused `filepath.Glob` block (lines 22-25) and the `_ = matches` line (line 39). The `WalkDir` block is the real implementation. Resulting loop body:

```go
	for _, src := range sources {
		info, err := os.Stat(src)
		if err != nil {
			return nil, fmt.Errorf("source %q: %w", src, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("source %q is not a directory", src)
		}

		err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && filepath.Ext(path) == ".ward" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking source %q: %w", src, err)
		}
	}
```

- [ ] **Step 2: Run tests**

```bash
/Users/oporpino/.asdf/shims/go test ./internal/secrets/...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/secrets/discover.go
git commit -m "refactor: remove glob morto no discover"
```

---

### Task 10: E2E coverage

**Files:**
- Create: `test/e2e/plain/plain_test.go`

**Interfaces:**
- Consumes: the CLI binary via `testutil.BuildBin` / `testutil.Run` (see `test/e2e/init/init_test.go` for the pattern)

- [ ] **Step 1: Write the e2e tests**

Create `test/e2e/plain/plain_test.go`:

```go
//go:build e2e

package plain_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

var bin string

func TestMain(m *testing.M) {
	b, err := testutil.BuildBin()
	if err != nil {
		panic(err)
	}
	bin = b
	code := m.Run()
	os.Remove(b)
	os.Exit(code)
}

// .plain.ward is read without a key and merges at the stripped dot-path.
func TestPlain_read_without_key(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	vault := filepath.Base(dir)
	vaultDir := filepath.Join(dir, ".ward", "vaults", vault)
	// public plain file
	os.WriteFile(filepath.Join(vaultDir, "config.plain.ward"),
		[]byte(vault+":\n  config:\n    port: \"8080\"\n"), 0644)

	out, _, code := testutil.Run(t, bin, dir, "get", vault+".config.port")
	if code != 0 {
		t.Fatalf("get exit %d", code)
	}
	if !testutil.Contains(out, "8080") {
		t.Errorf("expected 8080 from plain file, got %q", out)
	}
}

// An encrypted .ward with no key present → error.
func TestPlain_encrypted_without_key_errors(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	vault := filepath.Base(dir)
	// remove the key so nothing decrypts
	os.Remove(filepath.Join(dir, ".ward", vault+".key"))

	_, _, code := testutil.Run(t, bin, dir, "get", vault+".secret_1")
	if code == 0 {
		t.Fatal("expected non-zero exit when key is missing and file is encrypted")
	}
}

// A plaintext .plain.ward keeps working even with the key removed.
func TestPlain_survives_key_removal(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := testutil.Run(t, bin, dir, "init"); code != 0 {
		t.Fatal("init failed")
	}
	vault := filepath.Base(dir)
	vaultDir := filepath.Join(dir, ".ward", "vaults", vault)
	os.WriteFile(filepath.Join(vaultDir, "public.plain.ward"),
		[]byte(vault+":\n  public:\n    name: hello\n"), 0644)
	os.Remove(filepath.Join(dir, ".ward", vault+".key"))

	// encrypted secret errors, but plain value is still retrievable and a warning is printed
	out, stderr, _ := testutil.Run(t, bin, dir, "get", vault+".public.name")
	if !testutil.Contains(out, "hello") && !testutil.Contains(stderr, "hello") {
		t.Errorf("expected plain value 'hello' to be readable, stdout=%q stderr=%q", out, stderr)
	}
}
```

- [ ] **Step 2: Run the new e2e package**

```bash
/Users/oporpino/.asdf/shims/go test -tags e2e ./test/e2e/plain/... -v
```

Expected: PASS. If `TestPlain_survives_key_removal` fails because `get` on a scoped path only touches one vault (and may not trigger the encrypted-file load), adjust the assertion to use `ward view` instead of `get`, which loads all files. Prefer `view` if `get` short-circuits.

- [ ] **Step 3: Run the full e2e suite (excluding known-failing pre-existing packages)**

```bash
/Users/oporpino/.asdf/shims/go test -tags e2e ./test/e2e/init/... ./test/e2e/rotate/... ./test/e2e/get/... ./test/e2e/view/... ./test/e2e/envs/... ./test/e2e/plain/...
```

Expected: PASS for init/rotate/plain. `view` has a PRE-EXISTING failure (`TestView_multi_vault_shows_both_values`, value truncation) unrelated to this change — confirm it is the same failure as on `main` and ignore it.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/plain/
git commit -m "test: e2e para .plain.ward e chave ausente"
```

---

### Task 11: Documentation

**Files:**
- Modify: `docs/configuration.md`
- Modify: `README.md`
- Modify: `internal/mcp/server.go` (`wardDocs` const)

- [ ] **Step 1: Document in `docs/configuration.md`**

Add a section after the file-type / vault discussion:

```markdown
## File types

ward discovers every `*.ward` file under each vault. There are three kinds:

| Name | Encrypted? | Content |
|------|-----------|---------|
| `main.ward`, `config.ward` | **yes (required)** | structured YAML, merged into the tree |
| `sa.json.ward`, `token.key.ward` | yes (required) | raw file stored as a single secret |
| `config.plain.ward` | **no (never)** | structured YAML, plaintext, safe to read without a key |

Every encrypted `.ward` file must be an age-armored blob. A `.ward` file that is
plaintext on disk (and is not a `.plain.ward`) is an error — encrypt it, or rename
it to `.plain.ward` if it is intentionally public.

`.plain.ward` is treated as a single extension: `config.plain.ward` merges at the
same dot-path as `config.ward` would (`<vault>.config.…`) — the `.plain` marker
never appears in the tree or env var names.

### Missing keys

- A vault whose encrypted files cannot be decrypted (no key) is **skipped** with a
  `⚠ missing key for vault <name>` warning on stderr; the other vaults still render.
  That vault's `.plain.ward` files are still read.
- If **no** key resolves anywhere and at least one encrypted file exists, the command
  fails: `no encryption key found — set WARD_KEY or provide .ward/<vault>.key`.
```

- [ ] **Step 2: Document in `README.md`**

Add a short note in the commands/overview section listing the three file types and the breaking change: "`.ward` files must be encrypted; use `.plain.ward` for public, plaintext config."

- [ ] **Step 3: Update MCP docs**

In `internal/mcp/server.go`, in the `wardDocs` const, under "Core concepts" add a bullet:

```
- **.plain.ward**: structured plaintext file (never encrypted), read without a key
```

- [ ] **Step 4: Build and run the full test suite**

```bash
/Users/oporpino/.asdf/shims/go build ./... && /Users/oporpino/.asdf/shims/go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add docs/configuration.md README.md internal/mcp/server.go
git commit -m "docs: documenta .plain.ward e chave obrigatoria"
```

---

## Self-Review Notes

- **Spec coverage:** every spec section maps to a task — file matrix (T1/T2), mandatory encryption via require-key fallback (T4/T5), `.plain.ward` plaintext (T3/T5), skip+warn / fatal-no-key (T6/T7), structure validation (T8), discovery cleanup (T9), breaking change + docs (T11), e2e (T10).
- **Pre-existing failures:** `test/e2e/view` (`TestView_multi_vault_shows_both_values`) and `test/e2e/new` (timeout) fail on clean `main`, unrelated to this change. Do not treat them as regressions.
- **MockDecryptor retained:** it stays as a test helper (used across `internal/secrets`, `internal/cmd`, `test/integration`); only the production no-key fallback switches to `RequireKeyDecryptor`.
