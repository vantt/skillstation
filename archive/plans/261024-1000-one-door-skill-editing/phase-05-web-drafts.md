# Phase 5: Web: open drafts and discard

## Requirements

- A **Drafts** view (and a line on each skill's detail page) lists open drafts: skill, project, started,
  files changed since the base, whether the official version moved since.
- **Discard** on a draft shows what will be lost before confirming: files changed and not approved, and
  files a commit would leave out (for example `.env`) by name. Confirm deletes the draft folder, the
  Claude Code pointer and the hub record. Nothing on the hub changes.
- A record whose project folder is gone shows "project no longer has this draft" with a remove button.
- Discard is a person's action in the web UI; MCP gets no discard tool.

## Files

- `internal/delivery/web/routes_drafts.go` (new: list, discard preview, discard confirm)
- `web/src/screens/drafts/*` (new), `web/src/screens/skill-detail/*`, `web/src/api/types.ts`

## Validation

- Route tests: preview lists changed and left-out files; confirm removes exactly the draft folder and
  pointer; a pointer replaced by a full copy (two copies) is shown, not deleted silently.
- Browser check of the Drafts screen and discard flow.

## Risk

The web server deletes files in a project folder. Only paths recorded in the registry, inside that
project's `.agents/skills/<id>/` and `.claude/skills/<id>/`, may be removed; symlinks are refused.

## Implementation notes (2026-10-25)

- Screen named **Checkouts** (`/checkouts`); API `GET /api/v1/checkouts`,
  `POST /api/v1/checkouts/discard/preview`, `POST /api/v1/checkouts/discard` (digest-pinned).
- The editor's Name field is gone; the skill page shows open checkouts.
- Browser check done with Playwright on a demo hub (list and discard dialog).
