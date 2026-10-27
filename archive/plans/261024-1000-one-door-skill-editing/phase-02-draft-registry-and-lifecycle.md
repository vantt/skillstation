# Phase 2: Drafts the hub knows about

## Context

Today a checkout is known only to its project (marker file). 2a, 4a and 5a need the hub to know which
project holds a draft of which skill, from which base, since when.

## Requirements

- Checkout (and `skill create`) records the draft in the hub: skill id, project root, base version,
  started at. The record lives in hub runtime state, not in the canonical Git tree.
- The draft carries `skill.yaml` like any other skill file (phase 1 makes it one). `.meta/` is never
  copied.
- Commit reads routing, runtime and labels from the draft's `skill.yaml`; routing suggestions from
  SKILL.md sections are removed. `skill create` writes the form's answers into `skill.yaml`.
- The preview lists content changes, `skill.yaml` changes and their consequence (content re-approval
  needed or not) before approval.
- After an approved commit the draft closes: folder, Claude Code pointer and hub record are removed,
  and the `.gitignore` lines stay harmless. If the draft holds files the commit left out, it stays, the
  record stays, and the result names those files.
- The Claude Code pointer is rewritten whenever the draft's SKILL.md description changes through pull.
- `status` in the hub lists open drafts; a record whose project folder is gone is offered for removal.

## Files

- `internal/app/skill_checkout.go`, `skill_remote.go` (remote stays marker-only), `skill_scaffold.go`
- `internal/app/skill_commit*.go`, `skill_pull.go`, `project_skill_folder.go`, `checkout_status.go`
- new hub draft registry under `internal/app` (runtime state, file-locked like other runtime state)
- `internal/skill/routing_suggest.go` (remove SKILL.md section parsing from commit)

## Validation

- Checkout → edit `skill.yaml` → commit applies routing; a third-party skill stays approved.
- Commit closes the draft; with a `.env` left in the draft it stays and says so.
- `.meta/distill.yaml` on the hub is byte-identical after checkout, pull, commit, discard.
- Stale record (project deleted) is offered for removal by `status`.

## Risk

Closing a draft deletes project files; only files the commit just made official, plus the pointer and
marker, may be removed.

## Implementation notes (2026-10-25)

- Registry: `runtime/drafts.json` (`internal/app/draft_registry.go`), its own state lock; a refreshed
  record keeps its start date. Checkout and `skill create` record; a remote checkout does not.
- `skill create` writes the form's routing to `skill.yaml`; SKILL.md no longer repeats it in sections.
- On layout 6 a commit takes routing only from skill.yaml and the flags: nothing is suggested from
  SKILL.md; the preview shows `routing` (what the hub keeps) instead of `routing_suggestions`.
- A third-party skill needs a new approval only when its approved content changes (content digest,
  including the runtime block): `trust.content_kept` / `trust.needs_approval`.
- An applied commit closes a draft in `.agents/skills/<id>` or `.claude/skills/<id>`: removes the files
  it committed, the marker, the pointer and the record. Left-out files keep it open (`left_out`). A
  commit Git refused keeps it until the next commit finishes Git. Folders elsewhere keep the old resync.
- Pull rewrites the Claude Code pointer when SKILL.md changed and refreshes the record's base.
- `status` in the hub folder lists open drafts and records whose folder is gone.
