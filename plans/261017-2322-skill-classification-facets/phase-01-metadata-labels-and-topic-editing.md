---
phase: 1
title: "Metadata: labels field and topic editing"
status: completed
priority: P2
effort: "4h"
dependencies: []
---

# Phase 1: Metadata: labels field and topic editing

## Goal

Skill metadata accepts an optional `labels` array, and the skill update path can replace `labels`, `topics`
and `technologies`, without touching routing indexes for labels or content-review state.

## Files to Create / Modify

- Modify: `schemas/skill-metadata.schema.json` — add top-level
  `"labels": {"type": "array", "items": {"type": "string", "minLength": 1, "maxLength": 64}, "uniqueItems": true}`.
- Modify: `internal/canonical/skill.go` — add `labels` to the allowed key set (line 131) and to the string-list
  validation loop (line 177). Do not add it to the `routing` key set.
- Modify: `internal/skill/lifecycle.go` — `UpdateInput` gains `Labels`, `Topics`, `Technologies *[]string`
  (nil keeps, empty clears). Apply in the update function next to `input.Routing` (around line 277):
  - write the cleaned list (`cleanStrings`, trimmed, de-duplicated, order kept) to the top-level key, or delete
    the key when empty;
  - for `topics` and `technologies`, also delete `routing.topics` / `routing.technologies` so the top-level copy
    is the only one (catalog projection prefers top level, `internal/catalog/project.go:89`).
- Modify: the request digest / idempotency input of that update so the new fields take part in it (follow how
  `Routing` is included).
- Modify: `internal/delivery/mcpserver` skill update tool — confirm it does not expose the new fields (no change
  expected; only check).

## Tasks & Steps

1. Schema and validator change; keep the schema and Go validator in agreement (there is a schema embed test in
   `schemas/embed_test.go`).
2. Extend `UpdateInput` and the apply logic as above. Label normalisation: trim, drop empties, reject entries
   longer than 64 characters or containing a newline with an invalid-request error.
3. Verify by reading `internal/catalog/project.go` and `internal/app/self_resolution.go` that nothing iterates the
   whole document into `skill_fts` (labels must not be indexed). `flattenText` feeds `curation_fts`, which is
   curation search, not resolution; leave it.
4. Tests (use the `test-audit` skill):
   - extend the existing canonical validation table with a valid `labels` and an invalid one (non-list, empty item);
   - one update test: setting labels then clearing them; setting topics removes `routing.topics`;
   - one resolver test: a skill with `labels: [zzlabel]` is not returned by resolving `zzlabel`, while the
     same token in `topics` is (proves labels are not a routing signal and topics are).

## Verification

- `go test ./internal/canonical/... ./internal/skill/... ./internal/resolver/... ./schemas/...`
- `make check`

## Risks

- Older binaries reading a hub with `labels` would reject the unknown key. Accepted: the hub and its binary are
  upgraded together; note it in the release notes (phase 5).
