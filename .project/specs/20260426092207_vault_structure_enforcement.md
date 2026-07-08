# Vault Structure Enforcement

> TLDR: Enforce that .ward files live inside their vault's folder, rename vault dirs to plural+named, add `ward vault` subcommands, and update all commands to validate structure.

**Status:** completed
**Created:** 2026-04-26
**Owner:** @oporpino

---

## Context

Currently `.ward/vault/` is a single flat directory with no enforced structure. Nothing prevents files from living outside their vault, names are singular and unnamed, and there's no way to add a second vault via CLI. This change enforces a consistent layout: every vault has a name and a path, and every `.ward` file must live inside its vault's directory tree. Commands that read secrets must refuse to run if structural violations exist.

## Objectives

- Vaults are always named; the default vault uses the project name
- Internal vault layout moves from `.ward/vault/` to `.ward/vaults/<name>/`
- `ward vault` becomes a subcommand group replacing `ward vaults`
- `ward new` requires `<vault> <path>` and validates the vault exists
- Every `.ward` file is validated to belong to its vault's directory
- Commands that consume secrets block on structural errors
- All fixtures and tests are updated to the new layout

## Changes

### Config (`internal/config/config.go`)
- Add `Name` field to `Source` struct: `Name string \`yaml:"name"\``
- Update `Load` to reject two vaults with the same name (return error)
- Keep backward-compat: if `name` is absent on load, derive from last path segment

### Init (`internal/cmd/init.go`)
- Change template: `path: .ward/vault` → `name: <projectName>\n    path: .ward/vaults/<projectName>`
- Create `.ward/vaults/<projectName>/` instead of `.ward/vault/`
- Initial stub file: `.ward/vaults/<projectName>/secrets.ward`
- Update printed file row from `.ward/vault/` to `.ward/vaults/<projectName>/`

### New command (`internal/cmd/new.go`)
- Change signature: `ward new <vault> <path>` (`ExactArgs(2)`)
- Without args → cobra shows help (remove custom no-args handling; rely on `Args: cobra.ExactArgs(2)`)
- Validate that `<vault>` exists in config; error if not found
- Resolve file path relative to the named vault's directory
- `resolveNewPath` receives vault name + path arg; resolves inside that vault's dir
- `newFileStub` uses vault name as root key (first segment) instead of project name

### Vault subcommand (`internal/cmd/vault.go`) — new file replacing `vaults.go`
- `ward vault list` — lists configured vaults with name + path (replaces `ward vaults`)
- `ward vault add <name> <path>` — registers a new vault in config
  - Without args → cobra shows help (`ExactArgs(2)`)
  - Errors if name already exists
  - Errors if another vault with the same path already exists
  - Appends `{name, path}` to `vaults` in config and saves
- Remove `internal/cmd/vaults.go`

### Structure validation (`internal/cmd/helpers.go` or new `internal/cmd/validate.go`)
- `validateVaultStructure(cfg, cfgPath) []string` — returns list of violations
- For each `.ward` file discovered under each vault path: verify the file is inside that vault's directory
- For each `.ward` file: verify first YAML key matches the vault name
- `mustValidateStructure(cfg, cfgPath)` — calls above, prints errors and `os.Exit(1)` if any

### Commands updated to call `mustValidateStructure` before processing
- `ward exec` (`internal/cmd/exec.go`)
- `ward inspect` (`internal/cmd/inspect.go`) — also reports violations in output
- `ward view` (`internal/cmd/view.go`)
- `ward envs` (`internal/cmd/envs.go`)
- `ward get` (`internal/cmd/get.go`)

### Completion (`internal/cmd/complete.go`)
- Update vault-name completion to read `cfg.Vaults` names
- Update `ward new` completion to suggest vault name as first arg, then path

### Fixtures — all `.ward/config.yaml` files under `test/`
- Update `path: .ward/vault` → `name: <projectName>\n    path: .ward/vaults/<projectName>`
- Rename vault directories from `.ward/vault/` to `.ward/vaults/<projectName>/`
- Update `.ward` file content: root key changes from project name to vault name (same in most cases, but must be consistent)
- Add fixtures for `ward vault add` and `ward new <vault> <path>` in `test/e2e/vault/` and `test/e2e/new/`

### Tests
- `test/e2e/new/new_test.go` — update for new 2-arg signature
- `test/e2e/init/init_test.go` — update expected paths and config
- Add `test/e2e/vault/vault_test.go` — covers `vault list`, `vault add`, duplicate name error, duplicate path error
- Add structure-violation test cases to `test/e2e/inspect/` and `test/e2e/exec/`

## How to verify

```bash
# Init creates new layout
ward init
# → .ward/vaults/<projectName>/secrets.ward exists
# → config has name + path

# Add a second vault
ward vault add shared .commons/ward/vaults/shared
# → config updated, no duplicate allowed

# List vaults
ward vault list

# Create a file in a specific vault
ward new shared services/api
# → .commons/ward/vaults/shared/services/api.ward created

# Structure violation: move a file outside its vault → exec/view/envs/get must refuse
ward exec -- env

# Run tests
go test ./...
```

## Documentation

No documentation changes needed.
