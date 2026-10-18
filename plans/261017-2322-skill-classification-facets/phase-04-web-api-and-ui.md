---
phase: 4
title: "Web API and UI"
status: completed
priority: P2
effort: "1d"
dependencies: [1, 2]
---

# Phase 4: Web API and UI

## Goal

The web **Skills** screen filters by origin, topic and label, searches the richer fields, sorts by name, dates
and usage, and the skill detail screen edits labels, topics and technologies.

## Files to Create / Modify

- Modify: `internal/delivery/web/routes_read.go` — `handleSkills` (line 65) reads `state`, `origin`, `topic`,
  `label`, `q`, `sort` query parameters into `SkillListQuery` and calls `ListSkillCatalog`; invalid values return
  the app's invalid-request error.
- Modify: `internal/delivery/web/routes_skill_write.go` — `skillUpdatePreviewRequest` gains `labels`, `topics`,
  `technologies` (`*[]string`), passed into `skill.UpdateInput` in `handleSkillUpdatePreview` (line 183).
- Modify: `web/src/api/types.ts` — `SkillListItem` gains the phase 2 fields; `SkillListResult` gains
  `usage_available`.
- Modify: `web/src/api/queries.ts` — `useSkills` takes the query object and puts it in the request and the query
  key.
- Modify: `web/src/screens/skills/SkillsScreen.tsx` — remove client-side filtering of `q` and `collection`;
  URL params map to API params. Controls: search, State, Origin, Topic, Label, Sort, plus the existing upstream
  filter (still client-side, it is not part of the query). Topic and label options come from the values in the
  unfiltered result. Keep the collection filter only if it is still wanted; default is to keep it client-side
  so behaviour does not regress.
- Modify: the skills list rows — origin chip, label chips, "N uses (30d)" with last-used date as title text.
  When `usage_available` is false and sort is `uses`/`last-used`, show an inline note instead of zeros.
- Modify: `web/src/screens/skill-detail/OverviewTab.tsx` — labels editor (chips + add input) saved through the
  existing update preview → confirm flow; show origin.
- Modify: `web/src/screens/skill-detail/RoutingTab.tsx` — topics and technologies editors next to the other
  routing fields, saved through the same flow.
- Modify: `web/src/i18n/en.ts` (and other locale files present) — new strings.
- Modify: tests `web/src/screens/skills/skills.test.tsx` and the detail tab tests.

## Tasks & Steps

1. API parameters and update request fields (Go), with one handler test that a query parameter reaches the
   service and one that an invalid `origin` is a 400.
2. Types and query hook.
3. Skills screen controls and rows; follow the existing `app-field` / `fg-input` patterns and the design brief
   in `docs/use-cases/05-webui-design-brief.md`.
4. Detail editors for labels (Overview) and topics/technologies (Routing).
5. Tests (use `test-audit`): skills screen sends the selected filters and sort to the API and renders origin,
   labels and the no-usage note; detail editor sends `labels` in the preview request.

## Verification

- `go test ./internal/delivery/web/...`
- `make web-check`
- `make web-e2e` if an e2e scenario covers the skills list; extend it with one filter.
- Manual: `make web-dev` (ports 7421/5421; stop a stale owner instead of changing ports), open Skills, filter
  by origin, sort by uses, add a label on a skill, confirm it appears in the list.
