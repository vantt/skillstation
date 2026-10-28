# Intro film (9:16 and 16:9, about 41 s, text and music)

Remotion project for film 1. Look: [video/style](../style/README.md). Script and shot list:
[plans/261027-0830-video-redo/film-1-intro-script.md](../../plans/261027-0830-video-redo/film-1-intro-script.md).

| Path | What it is |
|---|---|
| `src/Intro.tsx` | The six shots, their timing and the sound cues. Shot lengths are in `SHOT_FRAMES`. |
| `src/theme.tsx` | P3 colours, fonts, pop-in helper, sticker, tile |
| `public/recordings` | Link to `video/recordings/`; shot 5 plays `tapes/07-intro-agent.mp4` |
| `public/audio/` | `click.wav`, `whoosh.wav` (made with ffmpeg, in Git); the music `.ogg` (not in Git) |

## Make it again

From the repository root, once: `npm ci` in this folder. Then:

```bash
video/demos/happy-path.sh
video/demos/agent-take.sh up
```

```bash
env -u CLAUDECODE -u CLAUDE_CODE_CHILD_SESSION vhs video/demos/tapes/07-intro-agent.tape
```

The two `-u` options drop the Claude variables of the shell that starts VHS. The take is
different every time: measure the green box over `Checked-by` and the cut points again (`PARTS` and `CheckedBox` in
`src/Intro.tsx`) after a new recording. Then, in this folder: `npm run draft` (half size), `npm run render` (9:16 with music) or `npm run render:wide-sound`
(16:9 with music). Both end with `scripts/normalize.mjs`, which sets the audio to -16 LUFS.

## 16:9 cut for the GitHub README

`src/IntroWide.tsx`, composition `IntroWide` (1920×1080, same 41.4 s, **no sound**; the README player starts muted).
It plays the landscape take `video/demos/tapes/08-intro-agent-wide.tape`. Render it, about 2.8 MB, under the 10 MB limit:

```bash
npm run render:wide
```

Output: `out/intro-readme.mp4`. The README shows `docs/images/demo/intro.gif` (3.5 MB, plays on its own) linked to
`docs/images/demo/intro.mp4` (a copy of that file), in `README.md` and `README.vi.md`. GitHub sanitises a `<video>` tag in
a README, so a relative video does not play inline; the GIF does, and the link opens the `.mp4` in GitHub's own player.
(A video can also play inline through a `user-attachments` URL, made by dragging the `.mp4` into the GitHub editor; free
plans allow 10 MB.) After a new render, copy the file over `intro.mp4` and make the GIF again:

```bash
ffmpeg -i docs/images/demo/intro.mp4 -vf "fps=12,scale=720:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=96:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" docs/images/demo/intro.gif
```

## Music

"You're Somewhere in Between" by Michael Ramir C., from Mixkit, under the Mixkit Stock Music Free License (not CC; it
allows commercial use and YouTube without attribution). Chosen by the owner on 2026-10-27 from the second round of
candidates ([music.md](../../plans/261027-0830-video-redo/music.md)). The first 15 s are a quiet intro, so `MUSIC_START`
in `src/Intro.tsx` skips them. Download it from the Mixkit synthpop/electropop listing
(`https://assets.mixkit.co/music/928/928.mp3`) to `public/audio/youre-somewhere-in-between.mp3`; it is not in Git,
because the licence does not allow redistributing the track on its own. Optional credit line for a description:
`Music: You're Somewhere in Between by Michael Ramir C. (Mixkit)`.
