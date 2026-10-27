---
phase: 5
title: "Docs and contracts"
status: completed
priority: P3
effort: "2h"
dependencies: [3, 4]
---

# Phase 5: Docs and contracts

## Goal

User and contract docs describe the new facets, flags, API parameters and the `labels` field.

## Files to Create / Modify

- Modify: `docs/contracts/web-api.md` — `GET /api/v1/skills` parameters and new response fields; update-preview
  request fields.
- Modify: `docs/command-reference.md` — `skill list` and `skill edit` flags.
- Modify: `docs/curating-skills.md` — short section "Organising skills": origin meaning (the four values and
  precedence), labels vs topics (labels never affect routing, topics do), usage source and its limits (hub
  loads only, 30-day window).
- Modify: `docs/web-ui.md` — Skills screen filters and sort, detail editors.
- Modify: `docs/design/07-storage-and-mutation-model.md` or `01-system-architecture.md` (whichever describes
  skill metadata fields; read first) — one line on `labels`.
- Check: `system-skills/curator/SKILL.md` mentions metadata fields? If it lists them, add `labels` and run the
  mirror sync described in AGENTS.md; mind the known-digests release rule.

## Tasks & Steps

1. Read each target before editing; update the smallest owning section.
2. Verify every flag, parameter and field name against the code from phases 1-4.

## Verification

- Links resolve; names match `skillhub skill list --help` and `skillhub skill edit --help` output.
- `make check` (the system-skill mirror tests if curator changed).
