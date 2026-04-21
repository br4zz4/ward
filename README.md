# ward

Hierarchical secrets management using SOPS+age.

`ward` organises secrets the way your infrastructure is already organised — in layers. A root file defines shared config. Environment files add or override specifics. There is no duplication, no syncing, no drift.

```
secrets/
  company.ward                      ← shared: name, region, base config
  company/sectors/one/
    staging.ward                    ← staging: database_url, redis_url
    production.ward                 ← production: database_url, redis_url
```

```sh
ward exec company/sectors/one/staging.ward -- your-app
# DATABASE_URL=postgres://staging.acme.internal/app
# NAME=sector 1 override
# REDIS_URL=redis://staging.acme.internal:6379
```

---

## How it works

Each `.ward` file is an encrypted YAML document. `ward` discovers all files under your configured sources, determines which are ancestors of your target, merges them from least to most specific, and exposes the result as env vars.

**Ancestry is determined by content structure, not file path.** A file is an ancestor if its map-branch structure is compatible with the target's — meaning it covers the same root key and doesn't declare conflicting branches.

**Leaf files override ancestors.** If `company.ward` sets `name: acme` and `staging.ward` sets `name: acme staging`, the merged result is `acme staging`, tracked back to `staging.ward`.

**Same-level conflicts are errors.** If two files at the same specificity level define the same key, `ward` refuses to merge and tells you exactly where each definition lives.

```
found 2 conflicts — cannot merge:

conflict: cannot merge key "database_url" — defined in multiple files at the same level:
  → secrets/company/sectors/one/conflict_a.ward:5
    database_url: postgres://conflict-a.internal/app
  → secrets/company/sectors/one/conflict_b.ward:5
    database_url: postgres://conflict-b.internal/app

  to resolve:
    1. remove the key from one of the files
    2. move it to a common ancestor if shared across environments
```

---

## Installation

**macOS (Homebrew)**

```sh
brew tap oporpino/tap
brew install --cask ward
```

Shell completion after install:

```sh
# zsh
ward completion zsh > $(brew --prefix)/share/zsh/site-functions/_ward

# bash
ward completion bash > $(brew --prefix)/etc/bash_completion.d/ward

# fish
ward completion fish > ~/.config/fish/completions/ward.fish
```

**Debian / Ubuntu (APT)**

```sh
curl -s https://packagecloud.io/install/repositories/oporpino/ward/script.deb.sh | sudo bash
sudo apt install ward
```

**Alpine Linux (APK)**

```sh
apk add ward
```

**Go**

```sh
go install github.com/oporpino/ward/cmd/ward@latest
```

**From source**

```sh
git clone https://github.com/oporpino/ward
cd ward
go build -o ~/.local/bin/ward ./cmd/ward
```

---

## Quick start

```sh
# Initialise a new project
ward init

# Edit your first secrets file (decrypts, opens $EDITOR, re-encrypts)
ward edit secrets/company.ward

# List all secrets with origins
ward list

# Inspect a specific environment
ward list secrets/company/sectors/one/staging.ward

# Show the env vars that would be injected
ward show secrets/company/sectors/one/staging.ward

# Inject and run
ward exec secrets/company/sectors/one/staging.ward -- env | grep DATABASE
```

---

## Commands

### `ward get <dot.path>`

Print the merged value at a dot-path.

```sh
ward get company.sectors.one.staging.database_url
# postgres://staging.acme.internal/app
```

### `ward list [anchor]`

Print the merged tree with source file and line for each value.

```sh
ward list secrets/company/sectors/one/staging.ward
```

```
company:
  name: acme                                                ← secrets/company.ward:2
  sectors:
    one:
      name: sector 1 override                               ← secrets/.../staging.ward:4
      staging:
        database_url: postgres://staging.acme.internal/app  ← secrets/.../staging.ward:6
        redis_url:    redis://staging.acme.internal:6379    ← secrets/.../staging.ward:7

● active  ● overrides
```

### `ward show [anchor] [--prefixed]`

Print the env vars that would be injected by `exec`. Without an anchor, shows all vars with full path names. With an anchor, shows relative names scoped to the anchor's level.

```sh
ward show secrets/company/sectors/one/staging.ward
# NAME          = sector 1 override
# DATABASE_URL  = postgres://staging.acme.internal/app
# REDIS_URL     = redis://staging.acme.internal:6379

ward show --prefixed
# COMPANY_NAME                              = acme
# COMPANY_SECTORS_ONE_STAGING_DATABASE_URL  = postgres://staging.acme.internal/app
```

### `ward exec [anchor] -- <command>`

Merge secrets and inject as env vars, then run a command.

```sh
ward exec secrets/company/sectors/one/staging.ward -- rails server
ward exec secrets/company/sectors/one/staging.ward -- env | grep DATABASE
```

### `ward init`

Generate a `ward.yaml` config and a starter `secrets.ward` file.

### `ward edit <file>`

Decrypt a `.ward` file, open it in `$EDITOR`, re-encrypt on save.

---

## Configuration

`ward.yaml` lives at the project root:

```yaml
encryption:
  engine: sops+age
  key_env: WARD_AGE_KEY      # env var holding the age private key
  key_file: .ward.key        # or a key file (gitignored)

merge: merge                 # merge | override | error

sources:
  - path: ./secrets
```

Pass a different config file with `-c`:

```sh
ward -c config/ward.yaml show staging.ward
```

### Merge modes

| Mode | Behaviour |
|---|---|
| `merge` | Deep merge. Leaf files override ancestor values. Same-level conflicts are errors. |
| `override` | Last (most specific) file always wins. No conflict errors. |
| `error` | Any overlapping key between any two files is an error. |

---

## File structure

`.ward` files are standard YAML documents encrypted with SOPS+age. Plain YAML is supported during development via `MockDecryptor` (no key required when running tests).

```yaml
# secrets/company.ward
company:
  name: acme
  region: us-east-1

# secrets/company/sectors/one/staging.ward
company:
  sectors:
    one:
      name: sector 1 override
      staging:
        database_url: postgres://staging.acme.internal/app
        redis_url: redis://staging.acme.internal:6379
```

### Directory anchors

Pass a directory instead of a file to merge all `.ward` files inside it. Conflicts between siblings in the directory are always errors — ambiguous by definition.

```sh
ward show secrets/company/sectors/one
# → error if staging.ward and production.ward both define the same key

ward show secrets/company/sectors/two
# → ok if two/staging.ward is the only file there
```

---

## Env var naming

Leaf keys are uppercased. Dots and nested map levels become `_`.

With a file anchor, the common structural prefix is stripped:

| Dot-path | Anchor | Env var |
|---|---|---|
| `company.sectors.one.staging.database_url` | `staging.ward` | `DATABASE_URL` |
| `company.sectors.one.staging.database_url` | `one/` (dir) | `STAGING_DATABASE_URL` |
| `company.sectors.one.staging.database_url` | none / `--prefixed` | `COMPANY_SECTORS_ONE_STAGING_DATABASE_URL` |

---

## Architecture

See [docs/architecture.md](docs/architecture.md) for a deep dive into the merge engine, ancestry detection, conflict resolution, and design decisions.

---

## License

MIT
