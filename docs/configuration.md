# Configuration

## ward.yaml

`ward.yaml` must exist in the directory where you run `ward`, or be passed explicitly with `-c`.

```yaml
encryption:
  engine: sops+age
  key_env: WARD_AGE_KEY      # name of the env var holding the age private key
  key_file: .ward.key        # path to a key file (alternative to key_env)

merge: merge                 # merge | override | error

sources:
  - path: ./secrets
  - path: ./infra/secrets    # multiple source directories are supported
```

### encryption

| Field | Description |
|---|---|
| `engine` | Encryption backend. Only `sops+age` is supported. |
| `key_env` | Name of the environment variable holding the age private key. |
| `key_file` | Path to a file containing the age private key. Gitignore this. |

If both `key_env` and `key_file` are set, `key_env` takes precedence.

### merge

Controls what happens when multiple files define the same key at the same ancestry level.

| Value | Behaviour |
|---|---|
| `merge` | Deep merge. Leaf files override ancestor values. Peer conflicts are errors. Default. |
| `override` | Last (most specific) file always wins silently. |
| `error` | Any overlapping key is an error, regardless of ancestry. |

### sources

A list of directories to discover `.ward` files in. Each source is walked recursively.

---

## Key management

### Generating a key

```sh
age-keygen -o .ward.key
```

The public key is printed to stdout. Use it to encrypt your `.ward` files with SOPS.

### SOPS configuration

```yaml
# .sops.yaml
creation_rules:
  - path_regex: \.ward$
    age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### Gitignore

```gitignore
.ward.key
*.age
```

---

## CLI flags

All commands accept:

| Flag | Description |
|---|---|
| `-c, --config <path>` | Path to `ward.yaml`. Default: `ward.yaml` in current directory. |
