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

1. `WARD_KEY_<NAME>` env var (e.g. `WARD_KEY_COMMONS`) — always checked first, works with single or multiple vaults
2. `.ward/<name>.key` file — auto-detected when present (e.g. `.ward/commons.key`)
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
- This holds whether the vault declares no key at all or declares a `key_env` /
  `key_file` that is missing at runtime: an unset `key_env` never aborts a read
  command, it only skips that vault. The warning names the env var or file to provide.
- If **no** key resolves anywhere and at least one encrypted file exists, the command
  fails: `no encryption key found — …`.
- Commands that **write** to a vault (`set`, `edit`, `new`, `import`, `file`) fail
  instead of skipping, so an encrypted file is never replaced by plaintext.

---

## Key management

### ward init

```sh
ward init
```

Generates a fresh encryption key at `.ward/<vault>.key` (named after the vault, e.g. `.ward/myapp.key`), adds it to `.gitignore`, creates `.ward/config.yaml` and an initial `.ward/vaults/<vault>/secrets.ward`. Prints a `WARD_KEY` token for use in CI.

The bare `.ward/.key` and `.ward.key` paths are still honoured as fallbacks when present, so existing projects keep working.

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

## CLI flags

All commands accept:

| Flag | Description |
|---|---|
| `-c, --config <path>` | Path to config file. Default: `.ward/config.yaml`. |

`exec` and `envs` also accept:

| Flag | Description |
|---|---|
| `--on-conflict=error\|override` | Override `on_conflict` from config for this invocation. |
| `--prefixed` | Use full dot-path as env var name instead of flat leaf name. |
