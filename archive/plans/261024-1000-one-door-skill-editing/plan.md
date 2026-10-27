---
title: "One door for changing a skill: skill.yaml beside SKILL.md, drafts the hub knows about"
description: "Split skill metadata into a person-editable skill.yaml and a hub-owned .meta/hub.yaml, make a project's draft the version its agents use, and send every change through one preview-then-approve door."
status: completed
priority: P1
effort: 6d
branch: main
tags: [feature, backend, frontend, cli, mcp, migration]
blockedBy: []
blocks: []
created: 2026-10-24
---

# One door for changing a skill

## Outcome

A person curating from a project connected to a hub on the same machine sees one skill and one door.
Everything they may change about a skill is in its folder (`SKILL.md`, scripts, `skill.yaml`); a draft in
their project is what every agent in that project uses; every change reaches the official version only
through a preview the person approves. Nothing a person edits is silently ignored, and nothing the hub owns
can be lost by a commit.

## Decisions (user, 2026-10-23 and 2026-10-24)

- Scope: the curator works in a project connected to a hub on the same machine. A hub on another machine
  and the hub folder itself are out of scope; their behavior must not regress.
- One door: every change (content, routing, runtime, labels) goes through preview then approval. No
  change is applied without it, and none needs a second step after it.
- `skill.yaml` sits beside `SKILL.md` in the hub and in a draft. It holds what the person edits:
  `routing` (all routing keys), `topics`, `technologies`, `domain`, `aliases`, `runtime`, `labels`,
  `schema_version`. Labels travel with the skill.
- `.meta/hub.yaml` holds what the hub decides: `status`, `quality` (content trust), `sources`,
  `provenance`, `history`, `created_at`, `updated_at`, `schema_version`. The hub writes the dates
  itself; a skill made by hand in the hub folder gets them when the next `status`, `maintain validate` or
  `doctor` records it. This replaces the earlier "dates from Git history" decision.
- Dropped: `id` and `collection` (the folder path), `name` and `description` (`SKILL.md` frontmatter
  only). Collection-by-folder is accepted for now; organizing collections is an open item.
- `.meta/` never enters a draft. A commit never syncs `.meta/` as files.
- Routing is never read from `SKILL.md` sections ("When to use", "Examples"); `skill create` writes
  `skill.yaml` instead.
- 2a: while project P has a draft of skill X, every agent in P gets the draft, through the native skill
  folder and through `skill_resolve`/`skill_get`, and the draft's `skill.yaml` routes X for P.
- 3a: a routing change asked for in P while P has a draft of X goes into the draft's `skill.yaml`.
  Committing `skill.yaml` alone is a later feature.
- 4a: approving a draft's commit closes the draft (folder, Claude Code pointer, hub record), except when
  the draft still holds files the commit leaves out (secrets, build output): then it stays and the person
  is told why.
- 5a+5b: a change to X approved elsewhere while P has a draft warns at that door ("X has a draft in P
  since …"), and P's commit says what changed meanwhile, merging files only one side changed and asking
  only about files both changed. Nothing writes into P's draft by itself; `status` and agents in P are
  told the official version moved and offered a pull.
- A person can discard a draft from the web UI.

## Evidence (verified 2026-10-23/24)

- `skill_resolve`/`skill_get` serve only the hub version; no serving code knows checkouts
  (`internal/delivery/mcpserver/skill_checkout_serving_test.go:50-65`). The resolve request carries no
  project (`schemas/skill-resolve-request-v1.schema.json`). `connect` pins the server's working directory to
  the hub for Codex and Gemini (`integration.go:357`, `toml_update.go:28`); the server never reads it.
- Probe of what each host tells a stdio MCP server, started from a project subfolder (2026-10-24):
  Claude Code 2.1.310 starts it in the launch folder and answers MCP roots with that folder; Codex 0.162.1
  has no roots, passes no `PWD`, and starts it in the registered `cwd`, else the launch folder; Gemini CLI
  0.65.0 (`mcp list` only) has no roots in its test client and starts it in the registered `cwd`, else the
  launch folder. Without the pinned `cwd`, all three start the server in the launch folder.
- Content trust hashes every skill file except `.meta/` plus the `runtime` block
  (`internal/skillruntime/trust.go:50-83`).
- Commit keeps hub routing for an existing skill and only suggests from SKILL.md sections for a new one
  (`internal/app/skill_commit.go:365-386`, `internal/skill/routing_suggest.go`).
- After a commit the project folder is kept and resynced (`internal/app/skill_commit_apply.go:256-283`);
  `writeFolderTree` removes files the final tree lacks.
- The canonical validator forces `id` to equal the folder (`internal/canonical/skill.go:155`) and treats
  `name`/`description` in `.meta/skill.yaml` as optional (`skill.go:157`).
- The user's hub is at layout schema 5; its `.meta/` folders hold `skill.yaml` and `distill.yaml`.
- `skill_update_preview/confirm` (MCP, curation profile) and `routes_skill_write.go` (web) edit routing
  directly on the hub; a connected project only gets the `runtime` MCP profile.

## Phases

| # | Phase | Depends on |
|---|---|---|
| 1 | [Split metadata into skill.yaml and .meta/hub.yaml](phase-01-split-metadata.md) | — |
| 2 | [Drafts the hub knows about](phase-02-draft-registry-and-lifecycle.md) | 1 |
| 3 | [Serve the draft to its project](phase-03-draft-aware-serving.md) | 2 |
| 4 | [Changes from elsewhere while a draft is open](phase-04-concurrent-changes.md) | 2 |
| 5 | [Web: open drafts and discard](phase-05-web-drafts.md) | 2 |
| 6 | [Docs, curator skill, contracts](phase-06-docs-and-curator.md) | 1–5 |

## Acceptance criteria

- A person can change a skill's routing, runtime or labels by editing `skill.yaml` in a draft, try it in
  that project with any agent, and make it official with one approval.
- Editing `skill.yaml` of a third-party skill needs no content re-approval unless `runtime` changed; the
  preview says which applies before approval.
- No commit, pull, checkout or discard ever writes, deletes or reads `.meta/` files of the hub as part of
  a draft. A test proves `.meta/distill.yaml` survives every draft operation.
- `skill_resolve` in a project with a draft of X routes X with the draft's `skill.yaml` and `skill_get`
  returns the draft's files; in any other project both return the official version.
- After an approved commit the draft is gone unless left-out files remain, and the person is told which.
- A change to X approved from the web, another project or the terminal while a draft is open warns at
  that door and is reported at the draft's commit.
- The web UI lists open drafts and discards one after showing what will be lost.
- Migrating a schema-5 hub moves fields without loss and stops, asking, wherever a dropped field
  disagrees with its source (folder name, SKILL.md frontmatter).
- A hub on another machine and the hub folder itself keep working (existing tests pass).

## Open items (not in this plan)

- Organizing collections beyond "the folder is the collection".
- Committing `skill.yaml` alone while content work in the draft is unfinished.

## Status (2026-10-27)

All six phases are implemented and committed on `main` (`513344e`, `3be42a8`, `73b8104`, `5776294`, `48f5990`, `aedff26`, `af93a53`; the evaluation fixture was moved to the split layout in `eb29dca`). Checked against code, tests and git history on 2026-10-27; `make check` and CI were green for the repository at the latest commit, but the web end-to-end suite was not re-run for this review.

- Naming: people and agents see a **checkout**; "draft" remains the hub lifecycle state. The web screen is "Checkouts", and the registry is `runtime/checkouts.json`.
- Phase 1 split metadata: `skill.yaml` plus `.meta/hub.yaml`, layout 6 migration. Phase 2 registry and lifecycle. Phase 3 draft-aware serving. Phase 4 concurrent-change warnings. Phase 5 web checkouts. Phase 6 docs, curator, schemas.
- This plan moved the checkout folder to `<project>/.agents/skills/<id>/` with a pointer in `.claude/skills`, which supersedes criterion 9 of `261014-1740-curation-ux-redesign`.
- Not verified: Gemini CLI sessions and IDE or desktop agent builds (phase 3), as the plan already said.
