---
title: Move the skill-to-source links into distill.yaml
status: planned
target: v0.12.0
---

# Move the skill-to-source links into distill.yaml

## Outcome

Which sources a skill learns from is stored once, in the `sources` block of the
skill's `.meta/distill.yaml`. The hub reads and writes it through its single
door; distill-lab reads it and, outside a hub, writes it. The files
`sources/skills/LINK-<skill>--<source>.yaml` go away.

## Decisions (owner, 2026-10-31)

- Shared source facts stay in the hub: `sources/catalog/<id>.yaml` keeps trust,
  license, cadence, monitoring state and "has new commits".
- `upstream` (where the skill was copied from) stays in `.meta/hub.yaml`.
- `distill.yaml` gains `sources: [{id, repository, ref, path}]`. `cursors`,
  `coverage` and `lessons` stay as they are and are written only by the skill.
- `source watch|attach|detach` write the `sources` block through the hub's
  mutation path (locked, atomic, own commit). The script keeps the block as it
  is and never edits it in a hub.
- Outside a hub the skill writes the block itself.
- distill-lab detects the mode at the start of every run: the binary runs and
  `skillstation status --json` says `"role": "hub"`; otherwise standalone.
- In a hub "distill upstream project X" is one request: preview `source watch`
  (or `attach` when the source exists), the user agrees, the hub writes, the skill
  distills. Outside a hub the skill adds the source and distills.

## Non-goals

- No change to the source catalog, triage, capture or monitoring behavior.
- No TOML parser, no new dependency.
- No change to `upstream` or the pull and merge flows.

## Phases

1. **Schema and readers.** `sources` in the distill.yaml schema (Go
   `internal/distill`, the script's validation, `schemas/`); the script keeps
   the block byte-for-byte meaning when it rewrites the file; one Go function
   reads the sources of a skill.
2. **Hub writes.** `source watch|attach|detach` create a minimal valid
   `distill.yaml` when missing and edit the block through `PlanMutation`; every
   reader of the old link files (`source list`, skill sources lookup, learning
   source ids, doctor, catalog) reads the block; "no orphan sources" still
   holds (a source needs a skill).
3. **Migration.** A migration step moves each `LINK-*` file into the `sources`
   block of its skill and removes the file; hubs without links are untouched;
   an existing hub round trips in a fixture test; data is never dropped.
4. **distill-lab.** Mode detection section first in SKILL.md; the 'distill
   upstream project X' row; standalone writes of the block; bump the version;
   keep the narrowed self-containment test (only the 'In a hub' section may name
   `skillstation` commands); mirror and known digests as AGENTS.md says.
5. **Docs and release.** docs/design 06 and 07, command reference, curating
   docs; release notes; check the layout migration number.

## Acceptance

- A hub with N skill-source links migrates to N `sources` entries and no
  `LINK-*` file; `source list` shows the same links before and after.
- `source watch|attach|detach` leave Git clean (their own commit) and a second
  `distill format` keeps the block unchanged.
- distill-lab run in a hub and in a folder without a hub both end with lessons
  and cursors in `distill.yaml`; outside a hub the sources survive between runs.
- `make check` passes; the tag-derived digest tests pass; release smoke tests
  pass on three systems.

## Risks

- Two writers of one file: the hub owns `sources`, the skill owns the rest; a
  test rewrites the file with the script and compares the block.
- A hub that never ran distill-lab has no `distill.yaml`: the hub creates it.
- Migration on a hub with a half-applied earlier migration: run it under the
  mutation transaction and make it idempotent.

## Open questions

- Which copy of the repository address wins when the catalog record and the
  `sources` entry differ: proposal is the catalog record, with a doctor warning.
