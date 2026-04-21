# ward

Hierarchical secrets management with zero external dependencies.

`ward` organises secrets the way your infrastructure is already organised — in layers. A root file defines shared config. Environment files add or override specifics. There is no duplication, no syncing, no drift.

```
.ward/vault/
  secrets.ward                      ← shared: name, region, base config
  staging.ward                      ← staging: database_url, redis_url
  production.ward                   ← production: database_url, redis_url
```

```sh
ward exec staging.ward -- your-app
# DATABASE_URL=postgres://staging.acme.internal/app
# NAME=sector 1 override
# REDIS_URL=redis://staging.acme.internal:6379
```

---

## How it works

Each `.ward` file is an encrypted YAML document. `ward` discovers all files under your configured vaults, determines which are ancestors of your target, merges them from least to most specific, and exposes the result as env vars.

**Ancestry is determined by content structure, not file path.** A file is an ancestor if its map-branch structure is compatible with the target's — meaning it covers the same root key and doesn't declare conflicting branches.

**Leaf files override ancestors.** If `secrets.ward` sets `name: acme` and `staging.ward` sets `name: acme staging`, the merged result is `acme staging`, tracked back to `staging.ward`.

**Same-level conflicts are errors.** If two files at the same specificity level define the same key, `ward` refuses to merge and tells you exactly where each definition lives.

```
found 2 conflicts — cannot merge:

conflict: cannot merge key "database_url" — defined in multiple files at the same level:
  → .ward/vault/conflict_a.ward:5
    database_url: postgres://conflict-a.internal/app
  → .ward/vault/conflict_b.ward:5
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

Shell completions are installed automatically.

**Debian / Ubuntu (APT)**

```sh
curl -s https://packagecloud.io/install/repositories/oporpino/ward/script.deb.sh | sudo bash
sudo apt install ward
```

**Alpine Linux (APK)**

```sh
curl -s https://packagecloud.io/install/repositories/oporpino/ward/script.alpine.sh | sudo bash
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

# Edit the default secrets file
ward edit

# Create a new secrets file
ward new staging

# Create a file in a specific path
ward new ./.commons/ward/vaults/ruby/staging

# List all secrets with origins
ward list

# Inspect a specific environment
ward list .ward/vault/staging.ward

# Show the env vars that would be injected
ward envs .ward/vault/staging.ward

# Inject and run
ward exec .ward/vault/staging.ward -- env | grep DATABASE
```

---

## Commands

### `ward init`

Initialise ward in the current directory. Creates `.ward/config.yaml`, generates a `.ward.key` age key, and creates an initial `.ward/vault/secrets.ward`.

Prints the `WARD_KEY` token to copy to CI or a secrets manager.

### `ward new <name>`

Create a new encrypted `.ward` file and open it in `$EDITOR`.

- Bare name: `ward new staging` → `.ward/vault/staging.ward`
- Slash path: `ward new infra/prod` → `infra/prod.ward` (relative to CWD)
- Dot-slash: `ward new ./.commons/vault/ruby/staging` → `.commons/vault/ruby/staging.ward`

If the file is outside the existing vaults, it is automatically added to `.ward/config.yaml`.

### `ward edit [file]`

Decrypt a `.ward` file, open it in `$EDITOR`, re-encrypt on save. Defaults to the first file in the default vault.

### `ward envs [anchor] [--prefixed]`

Print the env vars that would be injected by `exec`.

```sh
# Without anchor — flat leaf names, all vaults merged
ward envs
# DATABASE_URL  = postgres://staging.acme.internal/app
# REDIS_URL     = redis://staging.acme.internal:6379

# With anchor — names relative to the anchor's container level
ward envs .ward/vault/staging.ward
# NAME          = sector 1 override
# DATABASE_URL  = postgres://staging.acme.internal/app

# Full path names with --prefixed
ward envs --prefixed
# MYAPP_DATABASE_URL  = postgres://staging.acme.internal/app
# MYAPP_REDIS_URL     = redis://staging.acme.internal:6379
```

### `ward exec [anchor] -- <command>`

Merge secrets and inject as env vars, then run a command.

```sh
ward exec .ward/vault/staging.ward -- rails server
ward exec .ward/vault/staging.ward -- env | grep DATABASE
```

### `ward list [anchor]`

Print the merged tree with source file and line for each value.

```sh
ward list .ward/vault/staging.ward
```

```
myapp:
  name: acme                                                ← .ward/vault/secrets.ward:2
  staging:
    database_url: postgres://staging.acme.internal/app     ← .ward/vault/staging.ward:4
    redis_url:    redis://staging.acme.internal:6379        ← .ward/vault/staging.ward:5

● active  ● overrides
```

### `ward get <dot.path>`

Print the merged value at a dot-path.

```sh
ward get myapp.staging.database_url
# postgres://staging.acme.internal/app
```

### `ward config`

Open `.ward/config.yaml` in `$EDITOR`.

---

## Configuration

`.ward/config.yaml` is created by `ward init`:

```yaml
encryption:
  key_file: .ward.key        # age key file (gitignored); or use key_env

vaults:
  - path: ./.ward/vault      # directories to discover .ward files in
```

### encryption

| Field | Description |
|---|---|
| `engine` | `age+armor` (default) or `sops+age` (legacy). |
| `key_file` | Path to the age private key file. Gitignore this. |
| `key_env` | Name of an env var holding the age private key. Takes precedence over `key_file`. |

`age+armor` is the default engine — no external tools required. The entire file is encrypted as an opaque armored blob.

`sops+age` is available for projects that previously used SOPS. It requires the SOPS Go library (bundled — no binary needed).

### merge

Controls what happens when multiple files define the same key at the same ancestry level.

| Value | Behaviour |
|---|---|
| `merge` | Deep merge. Leaf files override ancestor values. Peer conflicts are errors. Default. |
| `override` | Last (most specific) file always wins silently. |
| `error` | Any overlapping key is an error, regardless of ancestry. |

### vaults

A list of directories to discover `.ward` files in. Each vault is walked recursively.

```yaml
vaults:
  - path: ./.ward/vault
  - path: ./infra/secrets
  - path: ../.commons/ward/vaults/ruby   # outside project root is fine
```

`sources:` is accepted as a legacy alias for `vaults:`.

### default_dir

Where `ward new <bare-name>` places new files. Defaults to `.ward/vault`.

```yaml
default_dir: secrets
```

### WARD_KEY

`ward init` prints a `WARD_KEY=ward-<base64>` token. Set it in CI instead of mounting the key file:

```sh
export WARD_KEY=ward-AAAA...
ward exec .ward/vault/staging.ward -- deploy
```

---

## Env var naming

| Scenario | Env var |
|---|---|
| No anchor, no `--prefixed` | Flat leaf name: `DATABASE_URL` |
| No anchor, `--prefixed` | Full dot-path: `MYAPP_STAGING_DATABASE_URL` |
| File anchor | Relative to anchor's container: `DATABASE_URL` |
| Dir anchor | Relative to dir level: `STAGING_DATABASE_URL` |

---

## Architecture

See [docs/architecture.md](docs/architecture.md) for a deep dive into the merge engine, ancestry detection, conflict resolution, and design decisions.

---

## License

MIT
