# Configuration

## .ward/config.yaml

Created by `ward init`. Can also be passed explicitly with `-c`.

```yaml
encryption:
  engine: age+armor          # age+armor (default) | sops+age
  key_file: .ward.key        # path to age key file (gitignored)
  key_env: WARD_KEY          # or: name of env var holding the age key

merge: merge                 # merge | override | error

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
| `engine` | Encryption backend. `age+armor` (default) or `sops+age`. |
| `key_file` | Path to a file containing the age private key. Gitignore this. |
| `key_env` | Name of an environment variable holding the age private key. Takes precedence over `key_file`. |

**age+armor** (default): the entire `.ward` file is encrypted as an opaque ASCII-armored blob using `filippo.io/age`. No external binaries required.

**sops+age**: YAML keys remain visible, only values are encrypted (ENC[AES256_GCM,...] format). Uses the `getsops/sops` Go library — no `sops` binary required. Compatible with files previously created by the `sops` CLI.

If both `key_env` and `key_file` are set, `key_env` takes precedence.

---

### merge

Controls what happens when multiple files define the same key at the same ancestry level.

| Value | Behaviour |
|---|---|
| `merge` | Deep merge. Leaf files override ancestor values. Peer conflicts are errors. Default. |
| `override` | Last (most specific) file always wins silently. |
| `error` | Any overlapping key is an error, regardless of ancestry. |

---

### vaults

A list of directories to discover `.ward` files in. Each vault is walked recursively.

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

Generates a fresh age key at `.ward.key`, adds it to `.gitignore`, creates `.ward/config.yaml` and an initial `.ward/vault/secrets.ward`. Prints a `WARD_KEY` token for use in CI.

### WARD_KEY token

`ward init` prints a portable token:

```
WARD_KEY=ward-<base64url-encoded-key>
```

Set it in CI instead of mounting a key file:

```sh
export WARD_KEY=ward-AAAA...
ward exec .ward/vault/staging.ward -- deploy
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
