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
