# Style frames: P3 look for both films

Still frames for the new look, copied from the vlog repo (2026-10-27). **Not yet approved by the owner.** Palette:
deep purple background, logo green as the main colour, pink as the second. Green = one source, the latest version;
pink = an old copy. Fonts: Bricolage Grotesque 800 and JetBrains Mono. Thick cream outlines, solid sticker shadows.

| Frame | Idea |
|---|---|
| ![palette](s0-palette.png) | Palette, fonts, rules |
| ![hook](s1-hook.png) | Hook: one skill, nine copies, versions drift |
| ![hub](s2-hub.png) | Middle: one hub, every agent gets the same skill |
| ![cta](s3-cta.png) | End: logo, tagline, install command |
| ![t1](t1-terminal.png) | Tutorial: step title, machine label, highlight box, ✓ callout, subtitle, chapter progress |
| ![t2](t2-notes.png) | Tutorial: lower-third note, "expected warning" card, tip card |
| ![t3](t3-two-machines.png) | Tutorial: two computers (hub green, agent yellow), invite line |
| ![t4](t4-web.png) | Tutorial: web UI frame, button highlight, result callout |
| ![t5](t5-command-card.png) | Tutorial: command card for `serve web` (never film the link) |

Frames `t1`–`t5` are for the tutorial (film 2), **not yet approved**: terminal step with highlight, callout and
chapter progress bar with an "Ask agent" step (`t1`), notes and expected-warning card (`t2`), two computers with machine labels (`t3`), web UI
frame (`t4`), command card (`t5`). Hub = green label, agent = yellow label.

The terminal in the frames is HTML built to show the look. In the films the agent is recorded for real, then themed
with these colours. Names, hosts and versions in the frames are invented. The install command in `s3` drops
`https://` to fit; check it before use.

Rebuild (`frames.html` has one `<section class="frame" id="...">` of 1920×1080 per frame):

```bash
node video/style/render.mjs video/style/frames.html video/style
```

## Feedback

| Round | Date | Frame | Feedback | Handling |
|---|---|---|---|---|
