# Configuration

## .ward/config.yaml

Created by `ward init`. Can also be passed explicitly with `-c`.

```yaml
encryption:
  key_file: .ward/myapp.key  # path to encryption key file (gitignored)
  key_env: WARD_KEY          # or: name of env var holding the encryption key

on_conflict: error           # error (default) | override

default_dir: .ward/vault     # where ward new <name> places files

vaults:
  - path: ./.ward/vault
  - path: ./infra/secrets    # multiple vaults supported
  - path: ../.commons/ward   # paths outside project root are fine
```

---

### encryption

| Field | Description |
|---|---|
| `key_file` | Path to the encryption key file. Gitignore this. |
| `key_env` | Name of an environment variable holding the encryption key. Takes precedence over `key_file`. |

If both `key_env` and `key_file` are set, `key_env` takes precedence.

---

### on_conflict

Controls what happens when multiple files define the same key at the same level (peer files, not ancestor/descendant).

| Value | Behaviour |
|---|---|
| `error` | Peer conflicts are errors. Default. |
| `override` | Last vault in config wins silently. |

The CLI flags `--on-conflict=error` and `--on-conflict=override` on `exec` and `envs` take precedence over this setting.

---

### vaults

A list of directories to discover `.ward` files in. Each vault is walked recursively. All vaults are always merged — the merge scope is controlled by dot-path arguments, not by specifying individual files.

`sources:` is accepted as a legacy alias — it is automatically migrated to `vaults:` on load.

**vault `path` field supports shell expansion** — `$VAR`, `${VAR}`, and `$(cmd)` are expanded at load time using `sh`. This makes configs portable across machines:

```yaml
vaults:
  - name: myproject
    path: .ward/vaults/myproject
  - name: commons
    path: $COMMONS_DIR/.ward/vaults/commons
```

Each vault can also declare its own `encryption` block to use a separate key:

```yaml
vaults:
  - name: myapp
    path: .ward/vaults/myapp
    encryption:
      key_file: .ward/myapp.key   # vault-specific key file
  - name: commons
    path: ../.commons/ward/vaults/commons
    encryption:
      key_env: WARD_KEY_COMMONS   # vault-specific env var
```

When a vault has no `encryption` block, ward applies the following resolution order:

1. `WARD_KEY_<NAME>` env var (e.g. `WARD_KEY_COMMONS`; `::`, `-` and `.` in the name become `_`, so `myfish::restricted` → `WARD_KEY_MYFISH_RESTRICTED`) — always checked first, works with single or multiple vaults. The default vault also accepts `WARD_KEY_<NAME>_DEFAULT` (see [Sub-vaults](#sub-vaults))
2. `.ward/<name>.key` file — auto-detected when present (e.g. `.ward/commons.key`, `.ward/myfish::restricted.key`)
3. Global `WARD_KEY` env var — fallback for all vaults
4. Global `encryption.key_env` or `encryption.key_file` from the config

The vault `encryption` fields follow the same rules as the global `encryption` block:

| Field | Description |
|---|---|
| `key_file` | Path to the vault's key file. Gitignore this. |
| `key_env` | Name of an env var holding this vault's key. Takes precedence over `key_file`. |

**CI usage — single vault:**

```sh
WARD_KEY_MYAPP=ward-xxx ward envs
```

**CI usage — multiple vaults:**

```sh
WARD_KEY_MYAPP=ward-xxx WARD_KEY_COMMONS=ward-yyy ward envs
```

---

### default_dir

Where `ward new <bare-name>` creates new files. Defaults to `.ward/vault`. Path is relative to the project root (parent of `.ward/`).

```yaml
default_dir: secrets
```

---

## Sub-vaults

A project often needs one vault open to the team (general secrets, dev, staging) and another only a few people can decrypt (production). A **sub-vault** is a separate vault named `<namespace>::<name>`, with its own directory and its own key, so the project name stays in the scope and in the key env var.

```yaml
# .ward/config.yaml
vaults:
  - name: myfish::default            # same vault as "myfish"
    path: .ward/vaults/myfish::default
  - name: myfish::restricted
    path: .ward/vaults/myfish::restricted
  - name: commons                    # external vault, unchanged
    path: $COMMONS_DIR/.ward/vaults/commons
```

```
.ward/
  config.yaml
  myfish::default.key                # gitignored
  myfish::restricted.key             # gitignored
  vaults/
    myfish::default/secrets.ward
    myfish::restricted/secrets.ward
```

The default vault holds `dev` and `staging`; the restricted one holds `production`. The root YAML key of each file is the vault name (`myfish:` is accepted as well as `myfish::default:` in the default vault):

```yaml
# .ward/vaults/myfish::default/secrets.ward
myfish::default:
  secrets:
    dev:
      DATABASE_URL: postgres://localhost/myfish_dev
    staging:
      DATABASE_URL: postgres://staging.myfish.internal/app

# .ward/vaults/myfish::restricted/secrets.ward
myfish::restricted:
  secrets:
    production:
      DATABASE_URL: postgres://prod.myfish.internal/app
```

```sh
ward get myfish:secrets.staging.DATABASE_URL              # default vault
ward get myfish::default:secrets.dev.DATABASE_URL         # same vault, long form
ward get myfish::restricted:secrets.production.DATABASE_URL
ward exec myfish:secrets.staging -- deploy                # never touches restricted
ward exec myfish::restricted:secrets.production -- deploy
```

### Rules

- A vault name is `<namespace>` or `<namespace>::<name>`: one `::` level only. `a::b::c`, `::b`, `a::` and a lone `:` in the name are rejected when the config loads.
- `myfish` and `myfish::default` are the same vault; declaring both is an error. Any other sub-vault (`myfish::open`, `myfish::restricted`) is a regular vault reached only by its full name — `myfish:` never includes it. The name after `::` is yours to choose; ward gives it no meaning (restriction comes only from who holds the key).
- Without a qualifier, reads overlay every vault that has the path (as before). A teammate without the restricted key sees that vault skipped with a `⚠ missing key` warning.
- `--prefixed` turns `::` into `_`: `myfish_restricted_secrets_production_DATABASE_URL`.

### Keys

| Vault | Env vars | Key files |
|---|---|---|
| `myfish` / `myfish::default` | `WARD_KEY_MYFISH`, `WARD_KEY_MYFISH_DEFAULT` | `.ward/myfish.key`, `.ward/myfish::default.key` |
| `myfish::restricted` | `WARD_KEY_MYFISH_RESTRICTED` | `.ward/myfish::restricted.key` |
| `myfish::open` | `WARD_KEY_MYFISH_OPEN` | `.ward/myfish::open.key` |

If both env vars (or both key files) of the default vault are present with different contents, the command fails naming both. Two vaults whose names derive the same env var (`myfish-restricted` and `myfish::restricted`) are rejected at load. Only the primary env var counts for this check: a legacy config with `app` and `app-default` still loads (the `_DEFAULT` alias of `app` is not reserved).

CI for production:

```sh
WARD_KEY_MYFISH_RESTRICTED=ward-xxx ward exec myfish::restricted:secrets.production -- deploy
```

### Creating and rotating

```sh
ward vault add myfish::restricted          # path defaults to .ward/vaults/myfish::restricted
```

Generates `.ward/myfish::restricted.key` (gitignored), creates the directory with an encrypted `secrets.ward` rooted at `myfish::restricted:`, registers the vault and prints `WARD_KEY_MYFISH_RESTRICTED=ward-…`. When the path already holds `.ward` files the vault is only registered.

```sh
ward rotate-key myfish::restricted
```

Rotates only that vault: re-encrypts its files, backs the old key up to `.ward/myfish::restricted.key.bkp-<timestamp>` (gitignored) and prints the new `WARD_KEY_MYFISH_RESTRICTED=ward-…` for CI. The key must live in a key file; a vault whose key exists only in an env var fails with `rotate-key requires a key file`. Without a vault name, `rotate-key` rotates the project's single key and fails, listing the vaults, when the project has several distinct keys.

### Caveats

- `::` in directory and key file names is valid on macOS and Linux, not on Windows.
- `rsync` and `scp` read `host::module/` as daemon syntax: pass these paths as `./.ward/vaults/myfish::restricted/`.
- The Finder shows `:` as `/`.
- Layouts created before sub-vaults are not migrated automatically.

---

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
- This holds whether the key source is missing, declared per vault (`key_env` /
  `key_file`), or declared globally: an unavailable key never aborts a read command,
  it only skips the vaults that depend on it. Vaults carrying their own working key
  still load. The warning names the env var or file to provide.
- If **no** key resolves anywhere and at least one encrypted file exists, the command
  fails: `no encryption key found — …`, listing every locked vault and how to unlock it.
- Commands that **write** (`set`, `edit`, `new`, `import`, `file`) fail instead of
  skipping, so an encrypted file is never replaced by plaintext — including in a
  project with no encryption configured at all, where the plain-write fallback stops
  at an already-encrypted target.
- An **unqualified** write is refused while any vault is skipped: the skipped vault
  might define the same secret-path, so ward cannot tell which vault you meant.
  Qualify it (`ward set <vault>:<path> <value>`) or provide the key.

---

## Key management

### ward init

```sh
ward init
```

Creates the default sub-vault `<project>::default`: a fresh encryption key at `.ward/<project>::default.key`, an initial `.ward/vaults/<project>::default/secrets.ward` rooted at `<project>::default:`, and `.ward/config.yaml`. The key is added to `.gitignore`. Prints the `WARD_KEY_<PROJECT>_DEFAULT` token for use in CI (`WARD_KEY_<PROJECT>` works too).

Projects created before sub-vaults (`name: myapp`, `.ward/vaults/myapp/`, `.ward/myapp.key`) keep working unchanged — `myapp` already is the default sub-vault. The bare `.ward/.key` and `.ward.key` paths are still honoured as fallbacks when present.

### WARD_KEY token

`ward init` prints a portable token:

```
WARD_KEY=ward-<base64url-encoded-key>
```

Set it in CI instead of mounting a key file:

```sh
export WARD_KEY=ward-AAAA...
ward exec qwert.environments.staging -- deploy
```

### Gitignore

`ward init` adds the generated `.ward/<vault>.key` to `.gitignore` automatically. If managing manually:

```gitignore
.ward/*.key
```

---

## File secrets

Files (JSON, YAML, XML, PEM, etc.) can be stored as a single encrypted secret using `ward file add`. The entire file content is treated as an opaque blob — it is never parsed.

### Naming convention

A file secret is a `.ward` file with a double extension:

```
service-account.json.ward   ← file secret
secrets.ward                ← regular secret file
```

The leaf key is derived from the original filename by replacing dots and hyphens with underscores:

```
service-account.json  →  service_account_json
credentials.yaml      →  credentials_yaml
```

The dot-path in the tree is determined by where the file sits in the vault directory:

```
.ward/vaults/app/service-account.json.ward          →  app.service_account_json
.ward/vaults/app/credentials/service-account.json.ward  →  app.credentials.service_account_json
```

### ward file add

```sh
ward file add <file> <vault>[.subdir]
```

Stores the file as a single encrypted secret. Creates the subdir if it does not exist. Errors if the target already exists.

```sh
ward file add service-account.json app
ward file add service-account.json app.credentials
```

### ward file extract

```sh
ward file extract <filename> [dest]
```

Restores the original file to disk. `dest` defaults to the current directory. Searches all vaults. Errors if the destination file already exists.

```sh
ward file extract service-account.json
ward file extract service-account.json /tmp/restored
```

### As env var

A file secret is exposed as a single env var whose value is the full file content:

```sh
SERVICE_ACCOUNT_JSON='{"type":"service_account","project_id":"..."}'
```

If `--upcase` is active, the name is uppercased. The name follows the same leaf-key derivation as the dot-path.

---

## OTP Code Generation

ward detects TOTP secrets automatically and generates their current 6-digit code
on the fly, so you don't need a separate authenticator app. Two value formats
are supported:

| Format | Example |
|---|---|
| `otpauth://` URI | `otpauth://totp/Example:alice@google.com?secret=JBSWY3DPEHPK3PXP&issuer=Example` |
| Bare base32 secret | `JBSWY3DPEHPK3PXP` (16, 32 or 64 chars, `[A-Z2-7]`) |

`ward get`, `ward secrets` and `ward exec` show or inject the generated code by
default. `ward tree` and `ward export` always show the original value.

```sh
ward get vault:myapp.otp          # → 287082 (6-digit code)
ward get vault:myapp.otp --raw    # → otpauth://totp/Example:... (original)
ward secrets                      # → OTP values appear as codes
ward exec -- ./publish.sh         # → OTP values injected as codes
```

### --raw

`--raw` is a global flag that skips OTP generation and shows the original stored
value. Useful for scripts that need the URI/secret itself.

```sh
ward --raw get vault:myapp.otp
ward --raw secrets
ward --raw exec -- ./publish.sh
```

### -v / --verbose

`-v` (or `--verbose`) is a global flag that prints OTP metadata — issuer,
account, algorithm, digits, period and seconds remaining in the current cycle —
to stderr before the code. `ward secrets` ignores it (its table view can't fit
multi-line metadata).

```sh
ward get vault:myapp.otp -v
Issuer: Example
Account: alice@google.com
Algorithm: SHA1
Digits: 6
Period: 30s
Remaining: 1s
287082
```

For a bare secret, issuer/account are omitted and the defaults are annotated:

```
Algorithm: SHA1 (default)
Digits: 6 (default)
Period: 30s (default)
Remaining: 1s
```

### Exec caveat

The code injected by `ward exec` is **static**: it is generated once when the
command starts and is not refreshed. TOTP codes expire after their period
(30s by default), so `exec` should only be used for short-lived commands.

### Defaults for bare secrets

A bare base32 secret (no URI) uses SHA1, 6 digits and a 30-second period — the
Google Authenticator defaults. An `otpauth://` URI overrides these with its own
`algorithm`, `digits` and `period` parameters when present.

---

## AI mode

Ward never prints secret values to an AI agent. AI mode is active when any of
these holds:

| Trigger | Notes |
|---|---|
| `--ai-mode` flag | Explicit, per invocation. |
| `WARD_AI_MODE` | `1`/`true` on (level from the config); `strict`/`masked` on, forcing that level; `0`/`false` off, including auto-detection; empty or unset falls back to auto-detection; any other value fails closed: on, strict, with a warning on stderr. |
| Agent in the parent process chain | Auto-detection: ward walks its parent pids and matches known agent binaries (`opencode`, `claude`, `codex`, `cursor-agent`, ...). Skipped when stdout is a terminal, so a human shell is never blocked — even one inherited from an agent. |
| MCP server | `ward --mcp` runs every subprocess with `WARD_AI_MODE=1`; the level comes from the project config. |

Use `WARD_AI_MODE=0` to opt out of auto-detection in a shell that sits inside an
agent's environment. A human shell never turns AI mode on by itself: without an
agent in the parent chain there is no auto-detection, and with stdout on a
terminal it is skipped.

### Levels

AI mode has two levels, chosen with the top-level `ai_mode` key of
`.ward/config.yaml` (or `WARD_AI_MODE=strict|masked` for one invocation). The
default is **masked**; an invalid `ai_mode` fails closed as `strict` with a
warning.

```yaml
ai_mode: strict   # or masked (default)
```

| Level | A single-line value shows as |
|---|---|
| `strict` | `<sensitive>` |
| `masked` | first and last `k` characters and the length: `tok_…a9f2 (51 chars)` |

At the masked level `k = min(4, length / 8)`, so at most 25% of the value is
revealed; a value under 8 characters shows only its length (`… (7 chars)`);
multi-line values (PEM, JSON) stay `<sensitive>`; a TOTP secret shows its masked
seed, never the generated code.

| Command | strict | masked |
|---|---|---|
| `tree`, `secrets` | `<sensitive>` | masked value |
| `get` | refused (exit 3) | prints the masked value |
| `get --json`, `secrets --json`, `file extract` | refused | refused |
| `raw`, `edit`, `new`, `export <file>` (stdout) | refused (exit 3) | refused (exit 3) |
| `vault add`, `rotate-key`, `init` | key written to its file, not printed | key written to its file, not printed |
| `export <file> <out>` (to disk) | allowed | allowed |
| `check <scope> <value-source>` | allowed (`match`/`differs`) | allowed |
| `set <dst> --from <src>` | allowed | allowed |

Every refusal exits with status 3 (the `exec` env-dump guard keeps status 1) and tells the agent what it may do instead:
use the secret (`ward exec`), check it against a value held elsewhere
(`ward check`) or copy it (`ward set <dst> --from <src>`). The MCP server runs its subprocesses with
`WARD_AI_MODE=1`, so the level comes from the project config; `ward_get` returns
a `masked` field at the masked level.

The level protects against accidental exposure, not against an agent that sets
`WARD_AI_MODE=0` or edits the config.

## CLI flags

All commands accept:

| Flag | Description |
|---|---|
| `-c, --config <path>` | Path to config file. Default: `.ward/config.yaml`. |
| `--ai-mode` | Suppress secret values (AI agent mode). |

`secrets` and `get` also accept:

| Flag | Description |
|---|---|
| `--json` | Machine-readable output. `secrets --json` prints a JSON object (env var name → value, untruncated); `get --json` prints `{"path","value"}` and refuses a subtree. Refused in AI mode. |

`exec` and `envs` also accept:

| Flag | Description |
|---|---|
| `--on-conflict=error\|override` | Override `on_conflict` from config for this invocation. |
| `--prefixed` | Use full dot-path as env var name instead of flat leaf name. |

## Machine-readable output (`--json`)

`secrets` and `get` accept `--json` for apps and scripts that consume secrets programmatically (e.g. `ward-rails`). It changes the output format only — value semantics still follow the global flags: `--raw` disables OTP code generation, `--prefixed` changes the key names.

**`ward --raw secrets --json [scope...]`** — a JSON object keyed by env var name, values untruncated:

```json
{
  "DATABASE_URL": "postgres://staging.acme.internal/app",
  "REDIS_URL": "redis://staging.acme.internal:6379"
}
```

With `--prefixed`, keys are the full paths (`MYAPP_SECRETS_MAIN_CODE_STAGING_DATABASE_URL`). Multiple scopes union exactly like the table view.

**`ward --raw get --json <scope>`** — a single leaf:

```json
{
  "path": "myapp.secrets.main.code.staging.DATABASE_URL",
  "value": "postgres://staging.acme.internal/app"
}
```

A subtree is refused: `--json` requires a leaf secret.

Rules:

- **AI mode is incompatible.** With `--ai-mode` or `WARD_AI_MODE=1`, `--json` exits non-zero without emitting anything — values are never exposed to agents, and a machine-readable format would defeat that guarantee.
- **stdout is JSON or empty.** On any failure (missing key, conflict, bad scope, subtree) the exit status is non-zero, the explanation goes to stderr, and stdout stays empty — so consumers never parse a partial payload.
- **No truncation.** The table view truncates long values to fit the terminal; JSON never does (multi-line file secrets survive intact).

