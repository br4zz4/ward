# Preserve case in env var injection

> TLDR: Add a `preserve-case` option to ward so env var keys keep their original YAML casing, enabling use cases like Terraform's `TF_VAR_do_token` convention.

**Status:** proposed
**Created:** 2026-04-25
**Owner:** @oporpino

---

## Context

Ward currently uppercases all leaf key names when injecting env vars (`strings.ToUpper` in `internal/secrets/env.go`). This prevents usage with tools that are case-sensitive about env var names — most notably Terraform, which requires `TF_VAR_` prefix in uppercase but the variable suffix in the exact case defined in `variables.tf` (e.g. `TF_VAR_do_token`).

With the current behavior, a vault key `TF_VAR_do_token` becomes `TF_VAR_DO_TOKEN`, which Terraform ignores.

## Objectives

- Add a `--preserve-case` flag to `ward envs` and `ward exec` that keeps the original YAML key casing
- Optionally expose this as a config option in `.ward/config.yaml` (`env_case: preserve`)
- Add documentation explaining both Terraform usage patterns (uppercase vars + uppercase ward, or preserve-case)
- Add a link in README pointing to the Terraform integration doc

## Changes

### Code
- `internal/secrets/env.go` — make uppercasing conditional in `collectLeafs`, `collectEnvEntries`, `collectEnvVars`, `collectEnvEntriesWithAnchorScope`
- `internal/cmd/envs.go` — add `--preserve-case` flag
- `internal/cmd/exec.go` — add `--preserve-case` flag
- `internal/config/config.go` — add optional `env_case` config field (`upper` default, `preserve` opt-in)
- `internal/secrets/env_test.go` — add tests for preserve-case behavior

### Docs
- `docs/terraform.md` — new file documenting two integration patterns:
  1. **Uppercase vars**: define Terraform vars as `DO_TOKEN`, ward injects `DO_TOKEN` naturally
  2. **Preserve-case**: define vars as `do_token`, use `ward exec --preserve-case -- terraform ...` so `TF_VAR_do_token` is injected correctly
- `README.md` — add link to `docs/terraform.md` under integrations or usage section

## How to verify

```bash
# vault with mixed-case key
echo 'TF_VAR_do_token: mytoken' | ward override .ward/vault/secrets.ward

# without flag (current behavior)
ward envs  # shows TF_VAR_DO_TOKEN

# with flag (new behavior)
ward envs --preserve-case  # shows TF_VAR_do_token

# end-to-end with terraform
ward exec --preserve-case -- terraform plan  # terraform reads TF_VAR_do_token correctly
```

## Documentation

- `docs/terraform.md` — new integration guide
- `README.md` — link to terraform doc
