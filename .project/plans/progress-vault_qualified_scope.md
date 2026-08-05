# Progress: Vault-Qualified Scope Path & Shared Overlay
Base SHA: 64f282c644c2c57e9d13824b3e98615dad6d22cf

Execution mode: Tasks 1-5 sequential (coupled chain, shared files).
Tasks 6-12 parallel (worktrees) after 5. Task 13 (release) controller-led.

Task 1: completa (commit 2048367, review limpo — parser trivial idêntico ao plano)
Task 2: completa (commit 5d138e7, review limpo, vet ok)
Task 3: completa (commit 4148931, review limpo — subagente usou root.DotPath como prefixo, melhoria correta sobre o brief)
Task 4: completa (engine EnvVarsForScopes, merged, suite verde)
Task 5: completa (commit c9aa673 — exec/envs multi-scope; verificado manualmente com
  /tmp/wardtest .plain.ward: overlay junta commons+trgclub, ignora production, sem colisão)

KNOWN ISSUE (pre-existing, NOT ours): all 104 e2e fixtures are plaintext files named
`.ward` (not `.plain.ward`); the mandatory-encryption change rejects them → entire
e2e suite fails "no encryption key found" on merge-base too. Blocks Task 13's
"all e2e green" precondition. Our new fixtures use .plain.ward (verified working).
Decision needed: fix legacy fixtures (rename to .plain.ward or encrypt) as part of
this branch, or separately.
Task 6: pending
Task 7: pending
Task 8: pending
Task 9: pending
Task 10: pending
Task 10: completa (docs scope universal + asdf, merged)
Task 11: completa (mcp ward_exec scope, commit ff1febf)
Task 12: completa (ward_docs explica scope, commit ff1febf)
Task 17: completa (resolver central de scope, merged)
Task 13: pending (integrate + release)
Task 14: pending (ward secrets rename)
Task 15: pending (set/unset/file: -s/--scope + --vault/--secret flags)
Task 16: pending (get/set/unset/inspect accept vault:secret-path)
Task 17: pending (central scope-arg resolver: positional | -s/--scope | --vault/--secret)
Task 18: pending (file add migrates vault.subdir -> vault:subdir)
Task 19: pending (rewrite breaking tests: set/unset unknown_vault_fails)
Task 20: pending (fix legacy e2e: rename .ward->.plain.ward in 8 safe dirs + update
  refs in export/import/override/raw; KEEP sops/ encrypted + get/missing-key as .ward)

Verified manually (binary from branch): exec overlay + exec commons:infra.staging
BOTH work correctly. get/set/secrets/inspect do NOT have scope yet.
Decision: finish 14/15/16/18 + fix e2e, then one complete release (v0.2.1 or v0.3.0).
Parallel wave now: {16, 14, 18, 20}; then 15 (after 16, shared editor.go); then 19.

Universal rule (decided mid-run): every path command takes scope 3 ways:
positional `commons:infra.KEY`, flag `-s/--scope commons:infra.KEY`, or
`--vault commons --secret infra.KEY`. Colon qualifies vault; plain dot never
identifies vault (compat break). Applies to get/set/unset/exec/envs/secrets/tree/
inspect; file add uses vault:subdir. Everything in this branch -> v0.3.0.
