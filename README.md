# ward

Hierarchical secrets manager.

`ward` organises secrets the way your infrastructure is already organised — in layers. A root file defines shared config. Environment files add or override specifics. There is no duplication, no syncing, no drift.

```
.ward/vault/
  secrets.ward                      ← shared: name, region, base config
  staging.ward                      ← staging: database_url, redis_url
  production.ward                   ← production: database_url, redis_url
```

```sh
ward exec myapp.environments.staging -- your-app
# DATABASE_URL=postgres://staging.acme.internal/app
# NAME=sector 1 override
# REDIS_URL=redis://staging.acme.internal:6379
```

---

## How it works

Each `.ward` file is an encrypted YAML document. `ward` discovers all files under your configured vaults, determines which are ancestors of your target, merges them from least to most specific, and exposes the result as env vars.

**File types:** ward recognises three kinds of `.ward` files: encrypted structured files (`secrets.ward`), encrypted raw file-secrets (`sa.json.ward`), and plaintext structured files (`config.plain.ward`). `.ward` files must be encrypted — use `.plain.ward` for public, plaintext config.

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

## Requirements

- macOS 12 (Monterey) or later
- Linux (amd64 or arm64)

## Installation

**macOS (Homebrew)**

```sh
brew tap br4zz4/tap
brew install --cask ward
```

Shell completions are installed automatically.

**Debian / Ubuntu**

```sh
curl -sL $(curl -s https://api.github.com/repos/br4zz4/ward/releases/latest \
  | grep browser_download_url | grep linux_amd64.deb | cut -d'"' -f4) -o ward.deb
sudo dpkg -i ward.deb
```

**Alpine Linux**

```sh
curl -sL $(curl -s https://api.github.com/repos/br4zz4/ward/releases/latest \
  | grep browser_download_url | grep linux_amd64.apk | cut -d'"' -f4) -o ward.apk
apk add --allow-untrusted ward.apk
```

**asdf**

```sh
asdf plugin add ward https://github.com/br4zz4/asdf-ward
asdf install ward latest
asdf global ward latest
```

**Go**

```sh
go install github.com/br4zz4/ward/cmd/ward@latest
```

**From source**

```sh
git clone https://github.com/br4zz4/ward
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

# Show the merged tree with origins
ward tree

# Show the env vars that would be injected
ward secrets myapp.environments.staging

# Inject and run
ward exec myapp.environments.staging -- env | grep DATABASE
```

---

## Commands

### `ward init`

Initialise ward in the current directory. Creates `.ward/config.yaml`, generates a `.ward/<vault>.key` encryption key (named after the vault, e.g. `.ward/myapp.key`), and creates an initial `.ward/vaults/<vault>/secrets.ward`.

Prints the `WARD_KEY` token to copy to CI or a secrets manager.

### `ward new <name>`

Create a new encrypted `.ward` file and open it in `$EDITOR`.

- Bare name: `ward new staging` → `.ward/vault/staging.ward`
- Slash path: `ward new infra/prod` → `infra/prod.ward` (relative to CWD)
- Dot-slash: `ward new ./.commons/vault/ruby/staging` → `.commons/vault/ruby/staging.ward`

If the file is outside the existing vaults, it is automatically added to `.ward/config.yaml`.

### `ward edit [file]`

Decrypt a `.ward` file, open it in `$EDITOR`, re-encrypt on save. Defaults to the first file in the default vault.

### `ward secrets [scope...] [--prefixed]`

Print the env vars that would be injected by `exec`.
(Formerly `ward envs`, which still works as a deprecated alias but prints a
warning.)

A **scope** is `[vault:]secret-path`. It can be passed positionally, via
`-s/--scope`, or via `--vault <vault> --secret <secret-path>`. A qualified
scope (`commons:infra.staging`) restricts the read to one vault; an unqualified
scope (`infra.staging`) overlays that secret-path across every vault that has
it. A plain dot never identifies a vault — use the `vault:` prefix. Multiple
scopes are unioned.

```sh
# Without a scope — flat leaf names, all vaults merged
ward secrets
# DATABASE_URL  = postgres://staging.acme.internal/app
# REDIS_URL     = redis://staging.acme.internal:6379

# With a scope — scoped to that secret-path, names relative to its level
ward secrets myapp.environments.staging
# NAME          = sector 1 override
# DATABASE_URL  = postgres://staging.acme.internal/app

# Qualified to a single vault, or unioning several scopes
ward secrets commons:infra.staging
ward secrets commons:infra.staging trgclub:infra.staging

# Full path names with --prefixed
ward secrets --prefixed
# MYAPP_DATABASE_URL  = postgres://staging.acme.internal/app
# MYAPP_REDIS_URL     = redis://staging.acme.internal:6379
```

### `ward exec [scope...] -- <command>`

Merge secrets and inject as env vars, then run a command.

A **scope** is `[vault:]secret-path`, passed positionally, via `-s/--scope`, or
via `--vault/--secret`. A qualified scope targets one vault; an unqualified one
overlays the secret-path across all vaults that have it. Multiple scopes are
unioned.

```sh
ward exec myapp.environments.staging -- rails server
ward exec myapp.environments.staging -- env | grep DATABASE

# Overlay commons.infra.staging + trgclub.infra.staging
ward exec infra.staging -- deploy

# Restrict to a single vault
ward exec commons:infra.staging -- deploy

# Union of several scopes
ward exec commons:infra.staging trgclub:infra.staging -- deploy
```

### `ward tree [scope...]`

Print the merged tree with source file and line for each value.
(Formerly `ward view`, which still works but is deprecated.)

Accepts a **scope** (`[vault:]secret-path`) positionally, via `-s/--scope`, or
via `--vault/--secret`. A qualified scope shows one vault; an unqualified one
overlays the secret-path across all vaults that have it.

```sh
ward tree myapp.environments.staging
ward tree commons:infra.staging
```

```
myapp:
  name: acme                                                ← .ward/vault/secrets.ward:2
  staging:
    database_url: postgres://staging.acme.internal/app     ← .ward/vault/staging.ward:4
    redis_url:    redis://staging.acme.internal:6379        ← .ward/vault/staging.ward:5

● active  ● overrides
```

### `ward get <scope>`

Print the merged value at a **scope** (`[vault:]secret-path`), passed
positionally, via `-s/--scope`, or via `--vault/--secret`.

`get` returns a single value. With a `vault:` qualifier it reads only that
vault. Without one, it does a single lookup across all vaults: if the
secret-path exists in exactly one vault it is returned; if it exists in more
than one, `ward` reports an ambiguity error (qualify it to disambiguate). A
plain dot never identifies a vault.

```sh
ward get myapp.staging.database_url
# postgres://staging.acme.internal/app

# Qualified to a single vault
ward get commons:infra.staging.database_url
```

### `ward set <scope> <value>`

Set a single secret at a **scope** (`[vault:]secret-path`), passed positionally,
via `-s/--scope`, or via `--vault/--secret`.

The vault is now qualified with a `:` (`vault:file.key`) or with
`--vault/--secret` — **a plain dot no longer identifies the vault**. This is a
compat break: `ward set myapp.staging.database_url` treats
`myapp.staging.database_url` as a literal secret-path, not "vault myapp". To
write to a specific vault, qualify it as `ward set myapp:staging.database_url`
(or `ward set --vault myapp --secret staging.database_url`).

```sh
ward set myapp:staging.database_url postgres://...
ward set --vault myapp --secret staging.database_url postgres://...
```

- Updates the value when the path already lives in exactly one file.
- Creates a new `.ward` file (named after the path) when no file defines it,
  printing a notice.
- Aborts if the path is defined in more than one file (cannot know which to set).
- If the write leaves an env var colliding with a different dot-path, it still
  succeeds and prints a non-blocking warning.

### `ward unset <scope>`

Remove a single secret at a **scope** (`[vault:]secret-path`), passed
positionally, via `-s/--scope`, or via `--vault/--secret`. As with `set`, a
plain dot does not identify the vault — qualify it with `:` when needed.

```sh
ward unset myapp:staging.database_url
```

- Errors with `key not found` when the path does not exist.
- Aborts on the same multi-file ambiguity as `set`.
- Keeps the file's scaffold structure even when it removes the last secret.

### `ward config`

Open `.ward/config.yaml` in `$EDITOR`.

---

## Configuration

`.ward/config.yaml` is created by `ward init`:

```yaml
encryption:
  key_file: .ward/myapp.key  # encryption key file (gitignored); or use key_env

vaults:
  - path: ./.ward/vault      # directories to discover .ward files in
```

### encryption

| Field | Description |
|---|---|
| `key_file` | Path to the encryption key file. Gitignore this. |
| `key_env` | Name of an env var holding the encryption key. Takes precedence over `key_file`. |

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

Vault `path` fields support shell expansion — `$VAR`, `${VAR}`, and `$(cmd)` are expanded at load time using `sh`, making configs portable across machines:

```yaml
vaults:
  - name: myproject
    path: .ward/vaults/myproject
  - name: commons
    path: $COMMONS_DIR/.ward/vaults/commons
```

Each vault can use its own key by adding an `encryption` block:

```yaml
vaults:
  - name: myapp
    path: .ward/vaults/myapp
  - name: commons
    path: ../.commons/ward/vaults/commons
    encryption:
      key_file: .ward/commons.key
```

When no `encryption` block is set on a vault, ward resolves its key in order:

1. `WARD_KEY_<NAME>` env var (e.g. `WARD_KEY_COMMONS`) — works with single or multiple vaults
2. `.ward/<name>.key` file — auto-detected when present
3. Global `WARD_KEY` / `encryption` config

### default_dir

Where `ward new <bare-name>` places new files. Defaults to `.ward/vault`.

```yaml
default_dir: secrets
```

### WARD_KEY

`ward init` prints a `WARD_KEY=ward-<base64>` token. Set it in CI instead of mounting the key file:

```sh
export WARD_KEY=ward-AAAA...
ward exec myapp.environments.staging -- deploy
```

Use `WARD_KEY_<NAME>` for single or multiple vaults — no config needed:

```sh
# single vault
WARD_KEY_MYAPP=ward-xxx ward exec myapp.staging -- deploy

# multiple vaults
WARD_KEY_MYAPP=ward-xxx WARD_KEY_COMMONS=ward-yyy ward exec myapp.staging -- deploy
```

---

## Env var naming

| Scenario | Env var |
|---|---|
| No scope, no `--prefixed` | Flat leaf name: `DATABASE_URL` |
| No scope, `--prefixed` | Full secret-path: `MYAPP_STAGING_DATABASE_URL` |
| Unqualified scope (`infra.staging`) | Scoped to that path, flat leaf name: `DATABASE_URL` |
| Qualified scope (`commons:infra.staging`) | One vault, scoped to that path, flat leaf name: `DATABASE_URL` |
| Qualified scope, `--prefixed` | One vault, full secret-path: `INFRA_STAGING_DATABASE_URL` |

---

## Architecture

See [docs/architecture.md](docs/architecture.md) for a deep dive into the merge engine, ancestry detection, conflict resolution, and design decisions.

---

## License

MIT
