# Configuration

## .ward/config.yaml

Created by `ward init`. Can also be passed explicitly with `-c`.

```yaml
encryption:
  key_file: .ward.key        # path to encryption key file (gitignored)
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

---

### default_dir

Where `ward new <bare-name>` creates new files. Defaults to `.ward/vault`. Path is relative to the project root (parent of `.ward/`).

```yaml
default_dir: secrets
```

---

## Key management

### ward init

```sh
ward init
```

Generates a fresh encryption key at `.ward.key`, adds it to `.gitignore`, creates `.ward/config.yaml` and an initial `.ward/vault/secrets.ward`. Prints a `WARD_KEY` token for use in CI.

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

`ward init` adds `.ward.key` to `.gitignore` automatically. If managing manually:

```gitignore
.ward.key
```

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
