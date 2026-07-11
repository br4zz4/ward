# Tree: truncate long values to fix source annotation alignment

> TLDR: Values longer than a terminal-width threshold break `ward tree` alignment because `maxLen` is computed from the full value, pushing the `←` annotation hundreds of spaces to the right for every other line.

**Status:** proposed
**Created:** 2026-07-10
**Owner:** @oporpino

---

## Context

`ward tree` aligns the `← file:line` annotation by computing `maxLen` — the longest visible text among all leaf lines — and padding every shorter line to match. When a value is very long (e.g. a Firebase credentials JSON blob, a PEM key, a long URL), `maxLen` becomes hundreds of characters, forcing the annotation on every other line far off-screen to the right.

Root cause: `helpers.go:collectListLines` embeds the full value in `listLine.text`. `printTreeWithOrigin` then uses `visibleLen(l.text)` to compute `maxLen` with no cap.

## Objectives

- Cap the visible length used for `maxLen` calculation at a sensible column (e.g. 120).
- Truncate the displayed value in the tree with `…` when it exceeds the cap, so the line never causes misalignment.
- Keep the origin annotation (`← file:line`) correctly aligned regardless of value length.
- Do not change the raw value — only the display in `ward tree` / `ward view`.

## Changes

- `internal/cmd/helpers.go`
  - Add a `truncateValue(s string, max int) string` helper that cuts at `max` visible chars and appends `…`.
  - In `collectListLines`, apply `truncateValue` to `child.Value` (and `child.Value` in the winner branch) when building `listLine.text`. Constant cap: `120` chars.
  - In `printTreeWithOrigin`, cap `maxLen` at `120` as a safety net even if a line slips through.
- `internal/cmd/helpers_test.go` (or `internal/cmd/tree_test.go`)
  - Unit tests for `truncateValue`: empty, short, exact, over limit, value with ANSI codes.
- No fixture or e2e changes needed — the fix is pure display logic.

## How to verify

1. Create a secrets file with a value longer than 120 chars (e.g. a JSON blob or base64 string).
2. Run `ward tree` — the `←` annotation must appear within a reasonable column for every line, not pushed off-screen.
3. The truncated line shows `…` at the cut point.
4. Short values are unaffected.
5. Unit tests pass: `go test ./internal/cmd/`.

## Documentation

No documentation changes needed.
