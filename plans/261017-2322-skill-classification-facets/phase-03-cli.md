---
phase: 3
title: "CLI: list flags, columns and edit flags"
status: completed
priority: P2
effort: "4h"
dependencies: [1, 2]
---

# Phase 3: CLI: list flags, columns and edit flags

## Goal

`skillhub skill list` filters, searches and sorts by the new facets, and `skillhub skill edit` sets labels,
topics and technologies.

## Files to Create / Modify

- Modify: `internal/delivery/cli/skill.go` — flag parsing for `--origin`, `--topic`, `--label`, `--search`,
  `--sort` on `list`; `--label`, `--topic`, `--technology` (repeatable) on `edit`; reject list-only flags on
  other subcommands the way `--state` is rejected (line 493). `runSkillList` (line 985) calls
  `ListSkillCatalog`.
- Modify: `internal/delivery/cli/skill_help.go` — `list` and `edit` usage text.
- Modify: CLI tests (`internal/delivery/cli/ux_behavior_test.go` or the existing skill list test).

## Tasks & Steps

1. List table columns: `ID`, `STATE`, `ORIGIN`, `USES 30D`, `LABELS` (comma-joined), `NAME`. Keep
   `COLLECTION` out of the default table; it stays in `--json`.
2. When `--sort uses` or `--sort last-used` is requested and `UsageAvailable` is false, print one line to stderr:
   "No usage data yet: usage counts skills loaded through the hub in the last 30 days."
3. Edit flags: repeated `--label` builds the list; a single `--label ""` clears labels (same rule for topic and
   technology). Pass them as `*[]string` in `skill.UpdateInput`. The preview/confirm and `--yes`/`--no-commit`
   behaviour is unchanged.
4. Help text examples: `skillhub skill list --origin learned --sort uses`,
   `skillhub skill edit pdf --label work --label favorite`.
5. Tests (use `test-audit`): one list test asserting flags reach the query and an invalid `--origin` exits with
   the invalid-request error; one edit test for `--label` set and clear. Do not re-test filter logic owned by
   phase 2.

## Verification

- `go test ./internal/delivery/cli/...`
- Manual: `go run ./cmd/skillhub skill list --workspace ~/projects/skillhub-repo --sort uses`
- `make check`
