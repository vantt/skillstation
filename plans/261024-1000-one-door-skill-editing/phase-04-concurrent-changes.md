# Phase 4: Changes from elsewhere while a draft is open

## Requirements

- Every door that changes a skill on the hub (CLI `skill edit`, upstream update, web update/changes,
  MCP `skill_update_*`, submission review) checks the draft registry. When X has an open draft, the
  preview says: "X has a draft in <project> since <date>; this change will have to be merged into it
  when that draft is committed." Approval stays possible.
- The curator, asked in P to change X's routing while P has a draft of X, edits the draft's `skill.yaml`
  instead of the hub, and says the change becomes official at the draft's commit.
- P's commit preview, when the official version moved since the draft began, names each change in
  between (who/where, when, which files). Files only one side changed merge silently; files both
  changed are asked about once.
- Nothing writes into a draft by itself. `status` in P and the draft-aware `skill_get` (phase 3) tell
  the person the official version moved and offer `skill pull`.

## Files

- `internal/app/skill_lifecycle.go`, `upstream_update.go`, `submission_review.go`, `skill_commit*.go`
- `internal/delivery/web/routes_skill_write.go`, `routes_skill_files.go`
- `internal/delivery/mcpserver/insight_tools.go`
- `internal/systemskills/curator/SKILL.md` (draft rule for routing requests)

## Validation

- Each door shows the warning with a draft open and not without one.
- Commit after a web routing change and an upstream content change reports both and asks only about
  files both sides changed.

## Implementation notes (2026-10-25)

- `openDraftWarnings` (`internal/app/draft_registry.go`) adds an `open_draft` warning to skill edit,
  resource edit, web file changes, routing suggestions (through skill edit), upstream update, submission
  review and another project's commit; the checkout's own commit is not warned. Status changes and
  content approvals do not warn (they touch only `.meta/hub.yaml`).
- The checkout's commit preview lists `hub_changes` (commits since the marker's base commit, with
  files) in the merge case; project `status` offers `skill pull` when the hub moved.
- The curator writes routing into the checkout's skill.yaml when the skill is checked out here.
