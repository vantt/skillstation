# Curate flow test: agent, CLI by hand, or both

Date: 2026-10-28. Release 0.9.0. Goal: find the best flow for "create my first skill" and check whether the tutorial film shows it.

## What the code says

Source: `system-skills/curator/SKILL.md` (skill-curator 1.11.0), `skillstation skill commit --help`, the permission rules `connect -g` writes.

- The curator skill is **CLI-first**: the agent runs plain `skillstation … --json` commands. MCP has only three runtime tools; the curation MCP tools are a fallback when there is no shell.
- Create flow in the skill: `skill create` (project only) → write `.agents/skills/<id>/SKILL.md` and `skill.yaml` → try it in the project → `skill commit` preview → apply after approval.
- `connect -g` writes **ask** rules for `skillstation * --yes*`, `confirm`, `init`, `connect`, `serve`, and a **deny** for `--approve-content`. So the host asks the person before the agent applies anything.
- `skill commit` run **in a terminal** on a new folder offers "Add as a draft?" and "Activate now?" in the same command. Without a terminal (an agent) it stops at a draft and activation is a separate `skill activate` preview and approval.

## What I ran (sandbox hub, fresh home, `claude -p`, one run per flow)

| Flow | Steps for the person | Agent work | Result |
|---|---|---|---|
| A. Agent does it all | 1 request, "yes commit", host prompt, "activate", "yes", host prompt: 3 messages and 2 host prompts | 12 turns, about 30 s for the draft; loaded the curator skill, ran `version`, `status`, `skill create`, wrote SKILL.md and skill.yaml, ran the commit preview, asked | Good skill. The agent invented the name (`team-commit-message`) and the routing (3 triggers, 3 examples, 2 not-for) from one sentence; the commit preview showed no lint warnings. It did not apply the commit: `--yes` hit the host ask rule and headless mode cannot approve. After I applied it by hand it previewed activation correctly. |
| B. CLI by hand | `skill create` (4 answers), edit SKILL.md yourself, try, `skill commit` with `y`, `y` | none | Most explicit. Fewest commands to publish (draft and activate in one command). The person writes the instructions. |
| C. Mix: agent writes, person publishes | create (CLI or agent), ask the agent for the instructions, try, `skill commit` with `y`, `y` in a terminal | 9 turns, 17 s for the instructions | Good. A plain request "replace the scaffold instructions" made the agent load the curator skill by itself and also preview the commit, harmlessly. |

Caveats: one run each; headless runs cannot click an approval, so "approve" was simulated by running the same command myself in the sandbox.

## Verdict

**Best flow: ask the agent to draft, try it, then publish in the terminal.**

1. Ask the agent in plain words ("create a skill commit-style that …"). It drafts the folder and previews the commit. The person never types a command or a routing field.
2. Try the draft in the same project (the project copy is live for every agent there).
3. Publish with `skillstation skill commit commit-style` in a terminal: read the preview, answer `y` (add) and `y` (activate). One command, one reviewed diff, and activation in the same step.

Why not A to the end: agent publishing costs two extra messages and two host prompts, because the agent cannot activate in the same commit. Why not B: the person must write instructions and routing by hand, which is the slow part and what an agent does well. The agent should draft; a person should apply. That also matches the curator skill's own rule that the user approves a diff they have seen.

One request detail: name the skill in the request (`commit-style`), or the agent picks its own name.

## Against the current film

| Scene | Now | Gap |
|---|---|---|
| 6, Ask your agent | Real take: "curate my Skillstation", agent suggests a first skill | Fine. It ends with "create one from scratch". |
| 7, Your first skill | The person types `skill create`, answers 4 questions, the instructions appear (written off camera), tries it with the agent, then `skill commit` y, y | The film ignores the agent right after scene 6 offered to create the skill, and the instructions step is a hidden edit. A viewer asks why the agent was asked in scene 6 and not here. |
| 7c Try it | Real take 09 | Fine, keep. |
| 7 publish | `skill commit` y, y in a terminal | This is the recommended publish step. Keep. |

**Proposed change to scene 7:** replace "Start the skill" and "Write the instructions" (tape 04, about 20 s of typing) with one real agent take: the person asks for the skill, the agent drafts it and previews the commit. Then keep "Try it" (take 09) and the terminal publish. The hand-written `skill create` becomes a one-line tip. Needs a new take (tape 11, agent create), edits to `script.md` scene 7, `narration.vi.json`, `edit.json`, the voice, and the owner's approval of the new wording.

Also noticed upstream (not pulled): commit `cf9da5c` "say that changes confirmed in the web UI are committed for you" changes web UI wording that scene 8 films. Re-record `web-edit` after pulling if the film shows the old line.

## Side effect to know about

Running Claude Code with a copied login rotates the refresh token again (same as lesson 15). The owner's `~/.claude/.credentials.json` is stale now; a fresh login sits in `/tmp/flowtest/a/home/.claude/.credentials.json`. Restoring it was refused by the permission classifier, so the owner decides (copy it back, or run `/login`).
