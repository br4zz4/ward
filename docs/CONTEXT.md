# ward

ward stores encrypted secrets in vaults inside a project and resolves them by scope.

## Language

### Vaults

**Vault**:
A directory of `.ward` files encrypted with one key, identified by a unique name.
_Avoid_: source, store

**Namespace**:
The part of a vault name before `::` (e.g. `myfish` in `myfish::restricted`); groups the sub-vaults of one project.
_Avoid_: project vault, parent vault, group

**Sub-vault**:
A vault whose name is `<namespace>::<name>`; it has its own directory and its own key, and is a separate vault from its siblings. The name after `::` is free (`restricted`, `prod`, ...) and carries no meaning to ward.
_Avoid_: child vault, nested vault, tier

**Default sub-vault**:
The sub-vault named `default`; the bare namespace refers to it, so `myfish` and `myfish::default` are the same vault. No other sub-vault can take its place.
_Avoid_: main vault, root vault

**Canonical vault name**:
The short form ward displays and indexes a vault by: the bare namespace for the default sub-vault (`myfish`), the full name otherwise (`myfish::restricted`).
_Avoid_: display name, alias

### Addressing

**Scope**:
The `[vault:]secret-path` argument that selects which secrets a command operates on; the vault qualifier may be a sub-vault name (`myfish::restricted:secrets.production`).
_Avoid_: dot-path (when a vault qualifier is meant)
