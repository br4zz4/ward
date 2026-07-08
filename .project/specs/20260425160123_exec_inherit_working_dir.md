# exec: inherit caller working directory

> TLDR: `ward exec` should run the command in the caller's current working directory, not the ward project root.

**Status:** proposed
**Created:** 2026-04-25
**Owner:** @oporpino

---

## Context

When `ward exec -- <cmd>` is invoked from a subdirectory (or when a tool like GitHub Actions sets `working-directory` before running the shell step), the spawned command runs in the wrong directory.

Root cause: `internal/cmd/exec.go` creates an `exec.Command` without setting `cmd.Dir`, so it inherits the Go process's working directory — which is where ward resolved its `.ward/config.yaml` (the project root), not where the caller invoked `ward exec`.

Example: GitHub Actions sets `working-directory: src/shared/messagebroker/terraform` before running `ward exec -- terraform plan`. The shell `cd`s to that path, but `ward exec` spawns `terraform` back in the repo root, where there are no `.tf` files.

## Objectives

- `ward exec -- <cmd>` runs the command in the caller's current working directory (`os.Getwd()`)
- Behaviour is consistent whether called from root or a subdirectory
- No breaking changes for existing usage

## Changes

- `internal/cmd/exec.go` — set `cmd.Dir` to `os.Getwd()` before `cmd.Run()`

```go
// before cmd.Run()
wd, err := os.Getwd()
if err != nil {
    fatal(err)
}
cmd.Dir = wd
```

- `test/e2e/exec/exec_test.go` — add test: invoke `ward exec` from a subdirectory and assert the command runs there

## How to verify

```bash
# from any subdirectory of a ward project (config is 3 levels up)
cd src/layer/component
ward exec -- pwd   # should print <project-root>/src/layer/component, not <project-root>
```

And in GitHub Actions with `working-directory: src/shared/messagebroker/terraform`:
```yaml
- name: Terraform plan
  working-directory: src/shared/messagebroker/terraform
  env:
    WARD_KEY: ${{ secrets.WARD_KEY }}
  run: ward exec -- terraform plan
```

## Documentation

No documentation changes needed.
