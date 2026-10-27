# Film 1: intro, script and shot list

Draft 2 for owner review (2026-10-27). Decided: **9:16 (1080×1920), about 41 s (was 60: the owner found the pace slow twice); shot 5 is 8.4 s, on-screen text plus music, English.**
Look: [P3 frames](../../video/style/README.md). Built in Remotion.

Draft 2 is built on the old film's scenes 1 and 2 ([script.json](../../video/intro/script.json)): "Copies everywhere"
(the chore, then the out-of-sync meter) and "Before and after" (copy by hand against edit once). The story is the
chore first, then the relief. No install steps; the end card carries the install line.

## What is reused from the old scenes

| Old element | Used as |
|---|---|
| Opening line "2 hours a week just to keep skill copies synced?"; the "36 copies in 9 projects" count is replaced by one named skill copied nine times | Hook, shot 1 |
| Scene 1 checklist: copy to project A, copy to project B, copy to the laptop, fix it in one place, forget the others | The chore, shot 2: each line ticks in; the last one fails in pink |
| Scene 1 meter "Out of sync", 20 → 90 | Rises while cards turn pink, shot 2 |
| Scene 2 before/after lists: *Copy into each project · Fix one, miss others · Redo on a new laptop* against *Edit once in the hub · Agents ask the hub · Nothing to copy* | The split screen, shot 4 |
| Totals "2 h/week" and "5 min/week" | Shown, but labelled "example" as in the old film (see open question 1) |

## Rules

- Text is the voice: at most 9 words per card, on screen at least 2.5 s, large enough for a phone.
- Pink = old copy or the chore, green = latest or the relief. One thing moves at a time.
- Numbers are examples, not measurements: every number carries "example" on screen (the old film's footnote).
- Claude Code is the only agent verified end to end; the hub shot says "any agent" and does not list others.
- The terminal shot is a real Claude Code run, themed to P3, sped up (the label says so). Claude Code folds MCP calls into "Called skillstation 3 times", so the tool names come from the captions; they match what the run really called (`skill_resolve`, then `skill_get`).

## Shots

| # | Time | On screen | Text card | Source |
|---|---|---|---|---|
| 1 | 0–5 | Nine repo cards fly in, each `commit-style/`, versions differ (one green, the rest pink/yellow) | **One skill. Nine copies.** *`commit-style` in 9 projects.* then *2 hours a week to keep them synced?* (example) | MG, frame `s1` vertical |
| 2 | 5–13 | A checklist ticks down one line at a time: Copy to project A ✓, Copy to project B ✓, Copy to the laptop ✓, Fix it in one place ✓, **Forget the others ✗** (pink). Beside it the "Out of sync" meter climbs 20 → 90 and the cards turn pink | The chore: every fix is five steps. | MG |
| 3 | 13–22 | Screen splits top and bottom. Top (pink): *Copy by hand*, three lines, "~2 h/week (example)". Bottom (green): *With Skillstation*, three lines, "~5 min/week (example)". Lines appear in pairs | *Fix one, miss others* turns into *Edit once in the hub*. | MG |
| 4 | 22–27 | The cards fold into one green diamond, "your hub"; dotted lines out to "any agent" | **One hub. Any agent.** | MG, frame `s2` vertical |
| 5 | 27–35.4 | A real Claude Code run, sped up, in a sticker window with three captions. (1) the prompt is typed: *Use my commit-style skill to write a commit message for my staged changes.* (2) the agent works: caption *The agent asks the hub.* with the pill `skill_resolve → skill_get`. (3) the answer ends with `Checked-by: skillstation-demo`, boxed in green; caption *It follows the latest version.* | 1 You ask for your skill. 2 The agent asks the hub. 3 It follows the latest version. | **Real take**: `video/demos/agent-take.sh` + `tapes/07-intro-agent.tape`; sped up and annotated in Remotion |
| 6 | 35.4–41.4 | Logo mark, tagline, install command | **Your skills in one place. Pulled by any agent, on demand.** `curl -fsSL https://github.com/vantt/skillstation/releases/latest/download/install.sh \| sh` | Frame `s3` vertical; check the command before use |

The install line is long for 1080 px: split it over two lines in mono, or show the short form and put the full
command in the description.

## Music

A short bright loop, 60 s, ending on a clear hit at shot 6. Audition 4–5 CC0 or CC BY tracks side by side on the same
cut and let the owner choose; record source and licence in the Remotion project. Click and pop sounds are cut with
ffmpeg (no licence issue).

## Open questions

1. The old film's brief allowed no numbers without a published way to measure them, so "36 copies / 2 h / 5 min" are
   shown as examples. Keep that, or drop the time totals and keep only the lists?
2. The meter and the totals are made-up illustration. Is that acceptable for the hook, or should the hook use the real
   story only (nine cards, versions drift)?
3. Shots 2 and 4 both show "the chore" and "the fix" lists. Should shot 2 stay as the slow, visual build-up (about 16 s),
   or be shortened to make room for the agent shot?
