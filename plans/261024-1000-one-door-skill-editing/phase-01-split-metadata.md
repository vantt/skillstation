# Phase 1: Split metadata into skill.yaml and .meta/hub.yaml

## Context

`.meta/skill.yaml` mixes what the person edits (routing, runtime, labels) with what the hub decides
(status, trust, sources). Drafts therefore cannot carry routing, and `.meta/` cannot be copied safely.

## Requirements

- New file `skill.yaml` beside `SKILL.md`: `schema_version`, `routing.*`, `topics`, `technologies`,
  `domain`, `aliases`, `runtime`, `labels`. Keys keep their current names and shapes.
- `.meta/skill.yaml` becomes `.meta/hub.yaml`: `schema_version`, `status`, `quality`, `sources`,
  `provenance`, `history`, `created_at`, `updated_at`.
- `id`, `collection`, `collection_id`, `name`, `description` are no longer stored; readers take id and
  collection from the path and name/description from `SKILL.md` frontmatter.
- Content trust digest: every skill file except `skill.yaml`, plus the `runtime` block of `skill.yaml`
  (same meaning as today, new location).
- The hub writes `created_at` on the first approval that brings a skill in, and `updated_at` on every
  approved change. `status`, `maintain validate` and `doctor` offer to record a skill made or changed by
  hand in the hub folder and write the dates then.
- Upstream updates never write or delete `skill.yaml`. An upstream that ships its own top-level
  `skill.yaml` stops the add/update with a clear message.
- `skill edit`, `skill_update_preview/confirm` and the web update route write `skill.yaml` (routing,
  runtime, labels) and `SKILL.md` frontmatter (name, description); they never write the dropped fields.
- Layout migration 5 → 6: move fields, delete dropped fields only when they equal their source; on any
  disagreement stop and report each skill with both values, and change nothing.

## Files

- `schemas/skill-metadata.schema.json` (split into skill and hub schemas)
- `internal/canonical/skill.go` (validation of both files), `internal/workspace/workspace.go`
  (`IsHubMeta`, skill folder rules)
- `internal/skillruntime/trust.go` (digest)
- `internal/catalog/*` (read routing/labels/runtime from `skill.yaml`, name/description from SKILL.md)
- `internal/skill/lifecycle.go`, `internal/app/skill_lifecycle.go`, `internal/delivery/web/routes_skill_write.go`
- `internal/app/upstream_*.go`, `internal/app/skill_add.go`, `internal/app/source_import.go`
- `internal/migration/migration.go` (5 → 6)

## Steps

1. Define both schemas and the validator; keep reading the old layout behind the migration gate.
2. Move catalog, trust, lifecycle and add/import writers to the new files.
3. Write the migration with a dry-run diff, the stop-on-disagreement rule, and fixtures from the user's
   hub shapes (`test-audit` with `sources`, a skill with `distill.yaml`, one with labels/topics).
4. Hub-written dates and the "record hand-made skill" offer.

## Validation

- Migration fixture: every field lands in the right file; `distill.yaml` untouched; disagreement case
  changes nothing and names the skill.
- Trust: changing routing or labels in `skill.yaml` keeps a third-party skill approved; changing runtime
  or SKILL.md does not.
- Full `go test ./...`; `maintain validate` on a migrated copy of the user's hub.

## Risk / rollback

Layout change touches every reader. Migration is one Git commit in the hub; rollback is reverting it with
the previous binary. Ship the migration behind `doctor --fix` / `init` like earlier layout moves.

## Implementation notes (2026-10-24)

- `internal/skillmeta` is the only code that knows the files: `Read` merges skill.yaml and .meta/hub.yaml
  into the pre-split document (id from the folder) so existing readers keep working; `Split`/`Files`
  take it apart with comments kept per key.
- `mutation.PlanMutation` is the one place that stores metadata (`internal/mutation/skill_metadata.go`):
  a change to the old path `.meta/skill.yaml` on a layout-6 hub becomes the two files, and every skill a
  write touches gets `updated_at` (and `created_at` when new) in .meta/hub.yaml. A write only to other
  `.meta/` files (distill.yaml) changes no date. Skills migrated from layout 5 have no `created_at`.
- `skill.yaml` is part of a checkout/draft tree but is not served to agents and not hashed by content
  trust (`workspace.IsSkillMetadata`); serving stays stable when routing changes.
- Commit on layout 6 takes skill.yaml from the folder; a folder without one keeps the hub's (checkouts
  made before the migration).
- `skill edit --name` different from the id is refused on layout 6. The web editor's Name field must go
  (phase 5).
- `doctor --fix` records a skill folder made by hand in the hub (SKILL.md, no metadata) as a draft;
  `maintain validate` points to it.
- Upstream updates never compare or write skill.yaml; an upstream that ships one is refused.
- Still to do in phase 6: split `schemas/skill-metadata.schema.json`; preview path lists of add/import
  still name `.meta/skill.yaml` before the mutation converts it.
