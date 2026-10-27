# Phase 3: Serve the draft to its project

## Context

An agent in a project reads skills two ways: from the native skill folder (sees the draft) and through
`skill_resolve`/`skill_get` (sees only the official version). Trying a draft is therefore only half true,
and draft routing cannot be tried at all.

Rule for this phase: **the MCP server shows an agent a draft exactly when that agent's native skill
folder shows it.** Same project, same view, through both channels.

## How the server knows the project (verified 2026-10-24, see plan.md Evidence)

| Host | MCP roots | Server working directory today | Without the pinned `cwd` |
|---|---|---|---|
| Claude Code 2.1.310 | yes, the launch folder | launch folder (no `cwd` registered) | same |
| Codex 0.162.1 | no (answers empty) | the hub (`cwd` pinned by `connect`) | launch folder |
| Gemini CLI 0.65.0 | not offered by `mcp list` | the hub (`cwd` pinned by `connect`) | launch folder |

The pinned `cwd` is the only blocker, and the server never uses it (it reads the hub from
`--workspace`). Rejected alternatives:

- `--project <root>` in the registration: breaks for user-scope registration (one entry for every
  project) and points every worktree or copied config at the original folder.
- A project field the agent sends in the request: depends on the model sending the right path every
  time, and lets a request name any folder.
- `PWD`: Codex does not pass it.

## Requirements

- `connect` stops writing `cwd` for Codex and Gemini (project and user scope). A registration that still
  pins the hub is reported by `doctor` and by the draft's checkout output for that host: "Codex in this
  project still gets the official version of X; run `skillstation connect` to let it try the draft."
- Project detection at each request, never cached at start-up (a draft may open or close mid-session):
  1. the MCP roots the client reports, if it supports roots;
  2. otherwise the process working directory, unless it is inside the hub.
  Walk up from that folder; the project is the deepest open-draft project root in the draft registry
  (phase 2) that contains it. No match means no draft: today's behavior, unchanged.
- The draft is served only while its folder still exists with its checkout marker (same check as
  `status`), so a draft deleted by hand is never served from the registry alone.
- For a project with a draft of X: resolve ranks X with the draft's `skill.yaml` (routing, topics,
  technologies) and SKILL.md description; `skill_get` returns the draft's files and `local.path` points at
  the draft folder. Every other skill and every other project is unaffected.
- A draft of a skill the hub does not have yet (fresh `skill create`) is resolvable in its project.
- The response says it served a draft, from which project folder, so agents and telemetry can tell.
- The HTTP server (`skillstation serve`) and `mcp proxy` to a remote hub keep serving the official
  version; remote drafts stay out of scope.

## Consequences to accept

- An agent started outside the project (for example in `~`) gets the official version. Its native skill
  folder would not see the draft either.
- A git worktree of the project does not see a draft opened in the main checkout. Draft folders are
  gitignored, so the worktree's native folder lacks it too.

## Files

- `internal/hostintegration/integration.go`, `runtime_access.go`, `toml_update.go`, `preview.go` (drop
  `cwd`; detect old pinned registrations), `doctor` checks
- `internal/delivery/mcpserver/*` (project detection per request; resolve/get handlers)
- `internal/resolver/*`, `internal/app/resolver.go` (overlay one skill's routing for one request)
- Response schemas for the "served a draft" field

## Validation

- Subprocess tests with the real binary started from a project subfolder: with roots (Claude Code shape),
  and without roots by working directory (Codex/Gemini shape): the draft is routed and served; a second
  project on the same hub, a worktree, and a server started inside the hub get the official version.
- Old registration with pinned `cwd`: official version, and `doctor` names the fix.
- Manual check with the installed Claude Code and Codex in a real project with a draft. Gemini is not
  installed here; check it before calling the phase done, or record it as unverified.
- Telemetry marks draft serves; the case journal does not learn routing from them.

## Risk

- Per-request routing overlay must not rebuild the catalog; keep it an in-memory override of one skill.
  Scoring reads only the skill record (`scoreSkill`, `internal/resolver/resolver.go:224`), so swapping the
  record scores the draft as it would score after commit. Two traps:
  - the response cache key (`resolver.cacheKey`) must include the draft (skill, project, draft digest),
    or one project gets another project's cached answer;
  - FTS only recalls candidates and the draft's new routing is not in the index: run the same rule
    recall on the draft record and score the draft's text against the query directly. Never force the
    draft into the candidate list, or the trial differs from what happens after commit.
- Verified with CLI Claude Code and Codex only. A Gemini session and IDE/desktop builds are unverified.
- Dropping `cwd` changes every connected project's config once; `connect` shows it in its preview as
  usual.

## Implementation notes (2026-10-25)

- Detection uses only the process working directory of the stdio server (`Server.projectDir`). MCP
  roots were dropped: on protocol 2026-07-28 a server may not send requests while serving one, and
  roots are deprecated (SEP-2577). The HTTP server (`remote`) never serves drafts.
- `skill_resolve`: the project's drafts route through `draftCatalog` with a private one-entry cache;
  a draft recommendation carries `draft: <folder>` (schema updated) and the draft's content digest.
- `skill_get`: returns the draft's files, `status: "draft in a project"`, `local.path` = draft folder,
  and the skill's own state/config folders.
- Telemetry names `draft_skill_ids`; a resolution whose primary is a draft records no case.
- Verified end to end (scratch hub, real binary): Claude Code 2.1.310 and Codex 0.162.1 started from
  a project subfolder got the draft; Claude Code from another folder got `no_skill`.
- The runtime profile stayed within its size budget by keeping `draft` a plain folder string.
