---
title: Curate and send skills from a project, not only on the hub machine
status: draft
---

# Curate and send skills from a project, not only on the hub machine

## Outcome

Skills are born and fixed in projects, so the person works there and sends the
result to the hub when it is good. The transport already exists for a hub on
another machine (`skill checkout`, `skill commit`, `skill pull`, a review queue
on the hub). What is missing is the guidance, the early checks and the feedback.
The curator stops saying "curation runs on the hub machine" to a request to send
or fix a skill.

## Decisions (owner, 2026-11-03)

- The person packages their own skill, in their own way. The curator never
  restructures it; it starts when the person says it is ready.
- Before sending, the curator helps with metadata and routing (name,
  description, `skill.yaml`: triggers, examples, not-for) and shows each change
  as a diff the person approves. It does not edit the body of `SKILL.md` unless
  asked.
- Fixing a skill: the curator follows the person. Whether the person edits the
  files or asks the agent to, is theirs to choose; no default.
- Starting point of a fix is a sentence in the conversation ("the skill we just
  used is not right"), not a command. The curator maps it to the skill the agent
  just resolved, asks, and then checks it out (a checkout writes to the project,
  so it is never silent). Also offered after a failed `skill_feedback`, or when
  the person names the skill. Loading the curator for this needs the MCP
  instructions line and the curator description to name the case.
- First question of a fix: keep it for this project only, or send it to the hub.
- Only a person on the hub approves content and records an audit. A scan
  run in a project is advice for the author, never evidence for the hub.
- Scan and routing check at the author's side (owner, 2026-11-04): the person
  who made a skill runs both where it was made, and the curator runs them
  itself when it sends the skill, without asking. A finding of `high` or
  `critical` holds the send and the person gets the fix on the spot; sending
  anyway needs their explicit word. The hub is always the last gate and scans
  again, so a client result is advice, never evidence; recording an audit stays
  hub-only.
- The scan is a command of the binary (`skillstation skill scan <folder>`), not
  a served skill: every connected machine has the binary, so there is no
  distribution path, no reserved id and no Node requirement to solve. The
  routing check is read-only `POST /mcp/lint` on the hub, since a collision
  with other skills can only be seen with the hub's catalog.
- Rule patterns are Go regular expressions (RE2). The one shipped rule that used
  a lookahead was rewritten in a form both engines read; the hub of the owner
  holds no custom rules.

## Non-goals

- No curation tool, canonical write, approval or activation from a project.
- No change to who may approve third-party content.

## Phases

| # | Phase | Changes the protocol | Status |
|---|---|---|---|
| A | Curator: prepare-to-send flow, fix flow with its entry point, keep-or-send question, offer after failed feedback; MCP instructions and curator description name the "a skill was wrong" case | No | done, except trying the activation on a real host |
| B | Preview checks: id already on the hub, `SKILL.md` frontmatter valid, hub moved past the checkout base, a reminder in `status` for a checkout left open | No | done |
| G | `skillstation skill scan <folder>` in Go (`internal/skillscan`), the same findings as the script's scan; the curator runs it on every send | No | done; `audit.mjs` still carries its own scan (below) |
| E | Read-only `POST /mcp/lint` and `skill lint <folder>` from a project with a remote hub | Yes (`docs/contracts/error-codes.md`, `docs/design/remote-hub.md`) | done |
| D | `skill edit` from a project with a remote hub (checkout to a temp folder, change, send) | No | open |
| C | Note to the reviewer with a submission; the sender can see the state of a `SUB-n` | Yes | needs owner decision |

Order: A and B, then G and E, then D, then C. Phases E and C change `docs/contracts`.

## Acceptance criteria

- In a project connected to a remote hub, a request to send, update or fix a skill
  is carried out up to the submission, and the curator names what the reviewer
  does on the hub.
- Sending a folder whose id the hub already has is shown as a replacement
  before anything is sent.
- A fix can start from "the skill we just used is not right" without the person
  naming a command or an id.
- A checkout that was left open shows up in `status`.
- `make check` passes; every released bundled file changed has its shipped digest
  in `internal/systemskills/known-digests.go`.

## Open points to verify while building

- Whether the routing in a folder's `skill.yaml` reaches the reviewer and is
  applied on approval.
- What `/mcp/bundle` returns for an id the hub has never seen.
- Whether a project's checkout is served through `skill_resolve` when the hub is
  remote (the proxy shows no draft handling); Claude Code reads the checkout
  through its pointer either way.
- What installing `skill-audit` as a served skill does on hosts without the
  skills extension.
- Whether anything records the skills used recently, to offer when the person
  opens a new session and says "the last skill".

## Risk and rollback

Phases A, B and D are additive and reversible by reverting the commit. A change
to a bundled skill file needs its shipped digest added in the same commit.

## Findings while building A and B

- Serving `skill-audit` would have needed the bundled-skill serving path
  generalized to a multi-file skill with scripts. Replaced by the `skill scan`
  command (G), which every connected machine already has.
- `audit.mjs` and `internal/skillscan` both implement the scan. They read one
  `rules/rules.txt` and `TestFolderScanFindsWhatTheScriptFinds` plus
  `TestRulesDigestMatchesTheScript` hold them equal, including the rules digest
  recorded audits keep. Open: make `audit.mjs` call `skillstation skill scan
  --json` and drop its engine (its tests then need a built binary), and delete
  the parity tests with it.
- The scan's JSON carries `rules_digest` in the form recorded audits already
  use, so the switch above invalidates no recorded audit.
- A folder with no `skill.yaml` is linted with empty routing, so the check says
  what is missing instead of failing.
- The MCP instructions have a hard allowance of 1900 bytes
  (`runtimeInstructionsAllowance`). The curator line was rewritten to fit; it no
  longer lists the verbs (add, edit, commit, pull, routing, repair), which the
  curator description still carries.
- Changing the curator description changes the Claude Code wrapper, so the
  shipped render's digest was added to `internal/hostintegration/known-wrappers.go`.
- `TestReadEndpointsGolden` in `internal/delivery/web` fails on a clean `main`
  (its golden data holds a date); it is not touched by this work.
- The check against the hub asks `/mcp/bundle` for headers only. An id that is a
  draft or waits for review answers 403 and counts as present; a skill the hub
  does not serve answers 404 and counts as new, so a draft with the same id is
  not seen.

## Activation trial (2026-11-04, Claude Code headless against a scratch hub)

- With a hub on another machine, `connect --hub` installs no skill-curator, and
  the hub does not serve it to Claude Code (`skill_get skill-curator` answers
  `not_found`). The agent then never loads the curator: asked to push a folder it
  explored the CLI by itself and sent a preview with no scan, no metadata review
  and no routing check. The send flow written in the curator does not reach a
  Claude Code agent of a remote-hub project.
- With the curator installed in the project (as a local `connect` writes it):
  "I made a skill in the standup-notes folder, push it to the hub" and the
  Vietnamese "skill standup-notes nay nen gui len hub" loaded it, ran the preview
  and the scan itself, read the files, noticed the missing `skill.yaml` and
  offered routing before sending. "that skill's instructions are unclear, can we
  change them?" loaded it and proposed checkout then diff. "cai skill do lam sai
  roi, sua lai giup toi" asked what was wrong first, then loaded it on the answer
  and ran `skill checkout`. "skill vua roi chua on" offered the curator. "the skill
  we just used got it wrong" only asked what went wrong, with no offer to fix the
  skill.
- Seen and fixed: `skill commit standup-notes` (a folder named by its bare name)
  was refused and cost the agent a second call; a bare name that is a folder with
  a `SKILL.md`, in a project that holds nothing under that id, is now that folder.
- Seen, not fixed: the agent skipped the keep-for-project-or-send question when the
  person had already said what to change; it tried `--diff` on a remote commit.

## Decision: the curator is written on `connect --hub` (2026-11-04)

Serving the curator only through the hub was tried first: `skill_get
skill-curator` returned the bundled file, and an explicit request loaded it, but
agents did not load it from natural phrases, because `skill_resolve` never
recommends a reserved system id and agents call it only some of the time. That
half was removed again.

Chosen instead, as the owner decided: a remote `connect` writes the curator like a
local one (shared file in `.agents/skills/`, wrapper for Claude Code), and the
generated folders stay out of Git.

- The curator's instructions run this machine's own commands (`skill scan`,
  `skill lint`), so the file must come from this machine's binary, not from the
  hub, which may run another version. It also works with no hub reachable.
- `SystemSkill.ClientSide` marks such a skill; `planMode.installs` is the one
  place that decides whether a plan writes a skill's files (local: all; remote:
  client-side ones; removal: none). `distill-lab` and `skill-audit` stay hub-only.
- `connect` (local hub in a project, and remote) adds the two folders to the
  project's `.gitignore` when the project is in a Git work tree, once, and says
  so. Each machine writes them, so a copy in Git would go stale for everyone
  else. Not done for the hub folder (it has its own ignore), user scope, or a
  folder outside Git.
- Doctor, for a project that uses a remote hub: `remote_skills` reports missing
  or older skill files and `doctor --fix` writes exactly those files (no token is
  involved), while `remote_reconnect` stays the person's. For a project in Git:
  `client_skill_tracked` prints the `git rm --cached` command, and
  `client_skill_not_ignored` is fixed by adding the `.gitignore` lines. A local
  hub's client group gets the same Git findings.
- Trial after the change (project connected with `connect --hub`, no other setup):
  "I wrote a skill in the folder standup-notes. Please send it to the hub.", its
  Vietnamese version and "skill release-notes vua roi viet sai dinh dang, sua lai
  giup toi" all loaded the curator by themselves, ran `skill commit` and `skill
  scan`, read the files, and (for the fix) went to `skill checkout`.

Not covered: Claude Code permission or skill settings that hide a project skill;
doctor does not read them.

## Decision: split the curator (2026-11-04)

The curator mixed two jobs: administering a hub (what the web UI mostly does
now) and sending, checking and fixing a skill from a project (what an agent
needs in a project, with any hub). Writing a skill's content is left to each
agent host's own skill creator.

- `skillstation-send` (`system-skills/send/SKILL.md`) holds the send flow, the
  fix flow and routing improvement of one skill. It is `ClientSide`: written in
  every project and user scope for a local or a remote hub, never in the hub
  folder. It is not served over MCP; hosts get the files.
- `skill-curator` keeps the hub's administration and is `HubOnly`, written in the
  hub folder only. `RemoveFromProjects` makes a connection or `doctor --fix`
  remove the copies earlier releases wrote in projects, when Skillstation wrote
  them (a file the person edited stays and is reported).
- `planMode.installs` is the one place that decides, per skill and folder,
  whether a plan writes a skill's files; `plannedSkill` carries that decision to
  the apply step.
- The MCP instructions name `skillstation-send` for sending and fixing, and the
  curator for the hub's owner.
- Trial (Claude Code headless, project connected to a hub on another machine):
  "send it to the hub" in English and Vietnamese, "check then send", and "skill
  release-notes got the format wrong, fix it" (3 of 3 after the description
  named that case first) loaded `skillstation-send`; "write me a skill for PDF"
  loaded nothing.
- A dev build's copy of the old curator is not a shipped digest, so a project
  connected with such a build keeps it and `connect` says it left the file;
  copies written by a released version are removed.

Not covered: Claude Code permission or skill settings that hide a project skill.

## Still to verify

- Try the curator's send flow on a real host end to end: scan holds a send with
  a `high` finding, the fix, the second scan, the routing check, the send.
- Whether a person on the hub would rather see the client's scan result next to
  the submission (needs phase C).

## Phrases to try the activation on a real host

Say each after an agent has used a skill, in a new conversation each time, and
note whether the agent offers to fix it, loads the curator, or carries on:

- "skill vừa rồi chưa ổn, nó bỏ qua bước kiểm tra"
- "cái skill đó làm sai rồi, sửa lại giúp tôi"
- "the skill we just used got it wrong"
- "that skill's instructions are unclear, can we change them?"
- "skill này nên gửi lên hub"
- "I made a skill in this folder, push it to the hub"
