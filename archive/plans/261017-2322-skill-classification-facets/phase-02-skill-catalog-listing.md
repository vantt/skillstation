---
phase: 2
title: "Skill catalog listing: facets, filter, search, sort"
status: completed
priority: P2
effort: "1d"
dependencies: [1]
---

# Phase 2: Skill catalog listing: facets, filter, search, sort

## Goal

One application-level listing owns enrichment, filtering, search and sorting, so the CLI, web API and MCP
return the same rows for the same query.

## Design

- `SkillListEntry` (`internal/app/skill_list.go`) gains, all `omitempty` except where noted:
  `Description`, `Origin` (always set), `Topics`, `Technologies`, `Labels`, `AddedAt`, `ChangedAt`
  (RFC3339 date strings), `Uses30d int64` (always set), `LastUsed` (YYYY-MM-DD).
- `SkillListResult` gains `UsageAvailable bool` (true when any `load:entrypoint` rollup row exists in the
  180-day window) so callers can tell "no data" from "zero uses".
- New `SkillListQuery{State, Origin, Topic, Label, Search, Sort string}` and
  `func (SkillService) ListSkillCatalog(ctx, path string, query SkillListQuery) (SkillListResult, error)`.
  Keep `ListSkills` as is: `UsageService` and others use it as a cheap id/name lookup.
- Enrichment sources, each read once per call:
  - metadata: `SELECT skills.id,..., canonical_entities.content_json FROM skills JOIN canonical_entities`
    (same join as `internal/resolver/sqlite_catalog.go:123`), decoding topics, technologies (top level, then
    `routing.*` fallback), labels, sources and provenance;
  - learning links: `canonical_entities` rows with `kind='skill_source_link'` and role `learning-source` or
    `inspiration`;
  - upstream status: `ListSkillUpstream` (already used by `ListSkillsWithUpstream`); on error, origin falls back
    to `adopted` for upstream skills;
  - dates: one `git log --format=%x00%cI --name-only -- skills/` pass over the hub, mapping each changed path to
    its skill folder; first commit date → `AddedAt`, last → `ChangedAt`. Missing Git or uncommitted skill →
    empty dates, no error;
  - usage: `TelemetryService.Open(root)` then `recorder.Rollups(ctx, today-180d, today)`; sum `load:entrypoint`
    over the last 30 days into `Uses30d`, max day with a positive count into `LastUsed`. A telemetry store error
    leaves usage empty and `UsageAvailable=false`; it never fails the listing.
- Origin derivation, a small pure function with a table test:
  1. has upstream (reuse `contentTrustDocument.provenance()` + `skillruntime.IsThirdParty`): `forked` when
     upstream status is `modified` or `diverged`, otherwise `adopted`;
  2. else any learning source (metadata `sources[].roles` contains `learning`, or a learning link): `learned`;
  3. else `authored`.
  Check during implementation what `skill_source_link` role `origin` means; if it denotes an upstream, treat it
  as step 1.
- Filters (all optional, combined with AND): `State` (existing validation), `Origin` (one of the four, else
  invalid-request), `Topic` (case-insensitive equality against topics ∪ technologies), `Label`
  (case-insensitive equality), `Search` (case-insensitive substring over id, name, description, topics,
  technologies, labels).
- Sort: `name` (default; id as tie-breaker), `added` and `changed` (newest first, empty last), `uses`
  (descending, then `last_used` descending, then id), `last-used` (most recent first, empty last). Unknown sort →
  invalid-request listing the allowed values.
- Keep every new function under 120 lines and 6 arguments: split into `loadSkillFacets`, `skillGitDates`,
  `skillUsage`, `deriveOrigin`, `filterSkills`, `sortSkills`.

## Files to Create / Modify

- Modify: `internal/app/skill_list.go` — types, `ListSkillCatalog`, filter, sort.
- Create: `internal/app/skill_list_facets.go` — metadata/link decoding, origin derivation, Git dates, usage.
- Modify: `internal/delivery/mcpserver/skill_tools.go` — `skill_list` calls `ListSkillCatalog` with
  `SkillListQuery{State: input.State}`; paging unchanged. Re-pin any tool-output digest test that covers it.
- Create/extend tests in `internal/app` (use the `test-audit` skill).

## Tasks & Steps

1. Add types and `deriveOrigin` with a table test covering the four outcomes and the upstream-wins precedence.
2. Implement metadata and link loading from the catalog.
3. Implement the Git dates pass; reuse the package's `gitOutput`/`gitLines` helpers.
4. Implement usage from rollups.
5. Implement filter and sort; one table test at `ListSkillCatalog` level over a small fixture hub (authored,
   learned, adopted skills, one with labels/topics, seeded rollups) covering each filter, search and each sort.
6. Switch `ListSkillsWithUpstream` callers (web) and MCP `skill_list` to the new function; delete
   `ListSkillsWithUpstream` if nothing else uses it.

## Verification

- `go test ./internal/app/ -run 'SkillList|Origin' ./internal/delivery/mcpserver/...`
- `make test-perf` if a listing budget exists; otherwise time `skill list` on a hub with a few hundred commits.
- `make check`

## Risks

- Git scan on long histories. If it is slow, do not limit it with `--since`, because that loses `AddedAt`.
  Cache the result in memory, keyed by the `HEAD` commit, for the web server's lifetime instead.
