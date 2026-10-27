---
title: "Classify, filter and sort skills by origin, topic, labels and usage"
description: "Skill lists in the CLI, web UI and MCP show where each skill came from, its topics, personal labels and how often it is used, and can be filtered, searched and sorted by them."
status: completed
priority: P2
effort: 3d
branch: main
tags: [feature, backend, frontend, api, cli]
blockedBy: []
blocks: []
created: 2026-10-17
---

# Classify, filter and sort skills by origin, topic, labels and usage

## Outcome

Today a skill has one visible classification, `collection`, which is really its storage folder
(`skills/<collection>/<skill>/`). After this plan a user can answer "which skills did I write, which did I
learn, which did I take from someone else, which do I actually use" from `skillhub skill list` and the web
**Skills** screen, and can group skills with personal labels.

Five independent facets:

| Facet | Source | Editable | Affects routing |
|---|---|---|---|
| Origin: `authored`, `learned`, `adopted`, `forked` | Derived from provenance, sources and upstream status | No (derived) | No |
| Topic | Existing `topics` and `technologies` | Yes, new CLI flags and web fields | Yes (they already feed `skill_fts.keywords`) |
| State | Existing `status` | Existing lifecycle commands | Yes (unchanged) |
| Labels | New `labels` string array in skill metadata | Yes, CLI and web only | No |
| Usage | Telemetry rollup metric `load:entrypoint` | No | No |

Collection stays as it is.

## Decisions (user, 2026-10-17)

- No manual authorship override; a wrongly derived origin is corrected with a label.
- Labels are editable from the CLI and web UI, visible to agents in `skill_list`, not editable through MCP.
  They live in the Git canonical repository, so a remote hub serves them like any other metadata.
- Topics and technologies become editable (CLI and web), knowing that this deliberately changes routing.
- Origin precedence: an upstream wins. Upstream present → `adopted`, or `forked` when upstream status is
  `modified` or `diverged`. No upstream but a learning source → `learned`. Otherwise `authored`.
- Usage counts loads (`load:entrypoint`, local and remote bundle), not first activations.
- Sorting by usage is available in the web UI and the CLI.

## Evidence (verified 2026-10-17)

- `schemas/skill-metadata.schema.json` has `topics`, `technologies`, `domain` at top level and under
  `routing`; no `labels`. `internal/canonical/skill.go:131` whitelists top-level keys.
- `internal/catalog/project.go:89-103` writes topics and technologies into `skill_fts.keywords`; the
  resolver ranks with `bm25` over `skill_fts` (`internal/resolver/sqlite_catalog.go:106`). Labels must never
  reach `skill_fts` or `indexedTerms` (`internal/app/self_resolution.go:266`).
- `internal/skillruntime/trust.go:53`: the content-review digest excludes `skill.meta.yaml`, so editing
  labels or topics does not invalidate a content approval.
- `internal/app/skill_snapshot.go:300` (`contentTrustDocument.provenance`) plus `skillruntime.IsThirdParty`
  already decide "has an upstream"; origin derivation reuses them. Learning sources are either
  `sources[].roles: learning` in metadata or `skill_source_link` entities with role `learning-source` or
  `inspiration` (`internal/catalog/input.go:545`).
- `internal/skill/lifecycle.go:294-296`: `skill edit` drops `created_at`, `updated_at` and `history`, and the
  user's hub has none of them. Dates therefore come from Git history of the skill folder, as
  `internal/app/review_queue.go:98` already does for one skill.
- `internal/telemetry/rollup.go:149` stores `load:<resource_kind>` per skill per UTC day in
  `telemetry_daily_rollups` (180-day retention); `internal/app/usage.go` reads them through
  `TelemetryService.Open` and `recorder.Rollups`.
- The user's hub (`~/projects/skillhub-repo`) has 5 skills: 3 with a GitHub origin, 1 with a learning source,
  none with topics, technologies or domain.

## Phases

| # | Phase | Status | Depends on |
|---|---|---|---|
| 1 | [Metadata: labels field and topic editing](phase-01-metadata-labels-and-topic-editing.md) | completed | – |
| 2 | [Skill catalog listing: facets, filter, search, sort](phase-02-skill-catalog-listing.md) | completed | 1 |
| 3 | [CLI: list flags, columns and edit flags](phase-03-cli.md) | completed | 1, 2 |
| 4 | [Web API and UI](phase-04-web-api-and-ui.md) | completed | 1, 2 |
| 5 | [Docs and contracts](phase-05-docs-and-contracts.md) | completed | 3, 4 |

Phases 3 and 4 touch disjoint files and can run in parallel after phase 2.

## Constraints

- Resolver output unchanged by labels. Topic edits change routing only for the skill edited.
- `skills/<collection>/` layout and collection semantics unchanged.
- Skills without `labels` stay valid; no migration.
- Editing labels or topics does not change `quality.content_reviewed_digest` validity or lifecycle state.
- AGENTS.md: `make check` and `make web-check` pass; functions at most 120 lines and 6 arguments; use the
  `test-audit` skill when writing tests; one owner test per contract.

## Non-goals

- User management or an authorship field.
- Hierarchical taxonomy, multi-valued collections, AI auto-labelling.
- Counting native host invocations that bypass the hub.
- Editing `domain` (unused, not populated; it stays readable but gets no new UI).
- Labels in the MCP edit tool.

## Acceptance criteria

1. `skill.meta.yaml` accepts `labels: [..]`; invalid shapes fail validation with a clear message.
2. `skillhub skill edit <id> --label x --label y`, `--topic`, `--technology` replace those fields; an empty
   value clears them. The web skill detail screen edits the same fields through the existing
   update-preview → confirm flow.
3. `skill list` (CLI, `GET /api/v1/skills`, MCP `skill_list`) returns per skill: `description`, `origin`,
   `topics`, `technologies`, `labels`, `added_at`, `changed_at`, `uses_30d`, `last_used`.
4. The CLI and web filter by origin, topic, label and state, search over id, name, description, topics and
   labels, and sort by name, added, changed, uses (30 days) and last used.
5. A test proves a label token does not change `skill_resolve` results.
6. When telemetry has no load rows, the web list says that usage data is not available yet instead of showing
   bare zeros, and the CLI prints a one-line note when sorting by usage.
7. Existing routing evaluation suites and `make check` / `make web-check` pass.

## Risks

- Git history scan cost on large hubs: one `git log` pass per list request. Mitigation in phase 2; verify with
  `make test-perf` if a budget covers listing.
- Uncommitted skills (`--no-commit`) have no Git dates; they sort as newest with `added_at` empty.
- Origin can be wrong for a local folder copied from someone else; accepted, labels cover it.

## Status (2026-10-27)

All five phases are done. Checked against code, tests and git history on 2026-10-27; `make check` and CI were green for the repository at the latest commit, but the web end-to-end suite was not re-run for this review.

- Labels and topic editing, listing with origin, topics, labels, dates and usage, CLI flags, web filters and sorting, docs and contracts are all present (`internal/app/skill_list*.go`, `internal/delivery/cli/skill.go`, `web/src/screens/skills/`, `docs/contracts/web-api.md`).
- Minor gap: the "labels never match in search" proof is at the index level (`catalog_test.go`), not through `skill_resolve`.
