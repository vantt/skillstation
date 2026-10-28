# Redo the two skillstation films with the P3 look

Status: **film 1 (intro) done and approved 2026-10-27; film 2 (tutorial) cut approved by the owner 2026-10-28** (`video/tutorial/output/full.mp4`, 1920×1080, 4:14, -16.5 LUFS, not published). Open items are under "Still open".

The owner disliked the old films' graphics, colour and style, not the mascot or the storytelling. So the visual look
is redone first; script and recordings are reused where they still match the product.

## Outcome and acceptance

- Film 1, intro: a viewer understands what Skillstation does and wants it. Short, before/after (one skill copied into
  many repos and drifting, versus one hub where `skill_resolve` returns the latest). No install steps.
- Film 2, tutorial: install, create hub, connect an agent, first skill, agent pulls it. A viewer can follow along.
- Both use the P3 look in [video/style/](../../video/style/README.md) once the owner approves the frames.
- Non-goals: new mascot, new product features.

References copied from vlog: [ref/](ref/) (look samples, the A1 mdview write-up and its 41 lessons, the handoff).
Lessons for this work go in [lessons.md](lessons.md).

## Inventory of the old work

| Asset | Verdict | Why |
|---|---|---|
| `video/tutorial/script.md`, `narration.vi.json`, `edit.json` | Keep | Storyboard approved; owner did not fault the story. Scenes 6 and 10 already revised for `skill-curator`. Retime `edit.json` after re-recording. |
| `video/demos/` (sandbox, happy-path, tapes 01–06, web `record.mjs`) | Keep, update | Best part. Still call `skillhub`, repo `skillhub-mcp` and 0.7.2; product is now `skillstation` 0.8.0 (installed: 0.8.0). Uncommitted edits are a partial rename. Retheme VHS and CSS to P3. |
| `video/recordings/*` mp4/webm | Drop and re-record | Old name and 0.7.2; gitignored build output; colours do not match P3. |
| `video/tutorial/audio/scene-*.wav`, `timeline.json` | Keep only if narration text is unchanged | edge-tts, English. Text says "Skill Hub" and scene 6/10 changed, so regenerate (cheap, `voice.mjs`). |
| `video/tutorial/remotion/` | Keep structure, restyle | Reuse composition and callout logic; replace backgrounds, frames, type, colours with P3. |
| `video/intro/` (HyperFrames theme, mascot, props, watermark, palettes) | Drop the look; keep `script.json` ideas | This is the look the owner rejected. 185 MB, mostly generated. The intro will be built in Remotion with the tutorial's tooling. |
| Tutorial music "Mellow Cafe Vibe" (CC0) | Re-audition | Owner disliked a similar pick in A1; compare candidates side by side. |

## Decisions (owner, 2026-10-27)

- P3 style frames approved.
- Film 1 intro: 9:16, about 60 s, on-screen text plus music, English. Script: [film-1-intro-script.md](film-1-intro-script.md).
- Film 2 tutorial: 16:9, English voice-over, EN and VI subtitles.
- Done: `video/demos/` renamed to Skillstation and `happy-path.sh` passes 14/14 on v0.8.0 (release binary).
  `video/tutorial/script.md`, `narration.vi.json`, `edit.json` renamed.
- Scene 6 needs `skill-curator`; v0.8.0 still ships `system-curator`, so scene 6 waits for the next release.

## Film 1 build

- Intro script **approved** by the owner (2026-10-27), draft 2 of [film-1-intro-script.md](film-1-intro-script.md).
- New project [video/intro-p3/](../../video/intro-p3/): Remotion, 1080×1920, 30 fps, 60 s, P3 look. Draft:
  `video/intro-p3/out/draft.mp4` (half scale, no music yet, not in Git). The old `video/intro/` is untracked and left
  as it is until the owner says to delete it.
- Shots 1–4 and 6 are built. Shot 5 uses the real take (see below).
- Shot 5 is a **real Claude Code run** (owner: record it for real), logged in with the owner's own account through a fresh `CLAUDE_CONFIG_DIR` that only links the credentials file. Setup: `video/demos/agent-take.sh`, tape `video/demos/tapes/07-intro-agent.tape`.
- Not done: music, transition sounds, the 9:16 safe-zone check.

## Film 2 build (2026-10-28)

- Owner decisions: script approved; frames t1–t5 approved; progress bar gets an "Ask agent" cell; scenes 1–3 reordered as problem, idea, install; scene 7 changed to "agent drafts, person tries, person publishes in a terminal" after the flow test in [curate-flow-test.md](curate-flow-test.md); music "City In The Rearview" (Holizna, CC0) from [music.md](music.md).
- Built on v0.9.0: tapes 01–06 and 09–12, web scenes, 41 narration sentences (`voice.mjs`), `edit.json` retimed, Remotion project restyled to P3 in [video/tutorial/remotion/](../../video/tutorial/remotion/), full render normalised to -16 LUFS.
- Lessons 12–22 in [lessons.md](lessons.md).

## Still open

1. Hide the "Auto mode is now default" notice at the start of takes 10 and 11; the "Draft? yes" callout is about a second late.
2. Short cut `short.mp4` and a GIF preview for film 2, and a 16:9 render of film 1 for the README: not asked for yet, owner decides.
3. The old `video/intro/` project is untracked, holds personal paths and old names; delete or keep, owner decides.
4. Nothing is published or uploaded; `output/youtube.md` is a draft. Check the Remotion licence (remotion.dev/license) before publishing.
