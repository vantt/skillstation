# Intro video: status (history)

> Since 2026-10-19 each film has its own status and feedback table:
> [film-1-intro.md](../261016-1000-intro-video/film-1-intro.md) and
> [film-2-tutorial.md](../261016-1000-intro-video/film-2-tutorial.md). Paths below are from before the move:
> `video/explainer/` is now `video/intro/`; `video/script.md`, `edit.json`, `scripts/`, `remotion/` and `output/` are
> now under `video/tutorial/`.

Last updated 2026-10-17 08:00 UTC, on `main` (fc2a8df), filmed on **v0.7.2**.

**Two films since 2026-10-18** (owner: content ok, visuals plain and without emphasis; split into an intro and a how-to):

- **Intro, 9:16, 1:25, music only:** `video/output/intro-explainer-9x16.mp4`, made with the explainer-ai-for-business-music
  skill in `video/explainer/`. Gates A, B and C all passed, each reviewed by a fresh subagent (A failed once on a
  number mismatch, B once on a scene-1 note, C once on a black 8 px strip from ffmpeg 8.1; all fixed).
- **How-to, 16:9, 4:11, voice-over:** `video/output/draft.mp4`, the Remotion cut redesigned: recordings in a window on
  a brand backdrop, step titles that open big then settle, result callouts timed to the output, a slow push-in, scene
  fades and a chapter progress bar. The two Claude Code shots are drawn illustrations, marked "Illustration".

## Done

- **Storyboard** [`video/script.md`](../../video/script.md), approved at CHECKPOINT 1.
  - The owner decided that the "agent on another computer" chapter is filmed with two sandboxes on one computer.
  - The private LAN address stays on screen.
  - The chapter times in the storyboard are the real ones from the current cut.
- **Terminal tapes** in `video/demos/tapes/`: six scenes on v0.7.2, at font 26. `render.sh` runs them in order. Each tape
  waits for the `$ ` prompt after its hidden setup, because a fresh sandbox with `SKILLHUB_BIN=release` first downloads
  the release; a fixed wait let the hidden command show on screen.
- **`happy-path.sh` passes all 14 steps on v0.7.2.** The executable-bit failure is gone, and `init` now suggests
  `connect -g`.
- **Web scenes:** both recorded again at a real 1920x1080, after a fix in `record.mjs` (see below).
- **Voice-over:** `video/scripts/voice.mjs` reads the narration from `script.md` and makes one edge-tts clip per
  sentence (`en-US-AndrewNeural`). It places each sentence at the time `video/edit.json` gives. It writes
  `audio/timeline.json`, `output/subtitles.en.srt` and `output/subtitles.vi.srt` (the Vietnamese text is in
  `video/narration.vi.json`).
- **Cut:** `video/remotion/`, Remotion 4.0.531 and React 19.3.0, pinned. It has:
  - motion graphics for the cold open, the problem, the command cards, the two computers and the outro;
  - a header band with the step title and the machine label;
  - burned-in English subtitles;
  - the music, ducked under the voice.
- **Music:** "Mellow Cafe Vibe" by John Bartmann, CC0, from Wikimedia Commons. The licence was checked through the
  Commons API.
- **`youtube.md`:** title, description, real chapters and tags. Nothing has been uploaded.
- **`video/README.md`:** rebuilding from nothing, after a release, sources and licences.

## Fixes made in the demo scripts

- **`sandbox.sh`:** `SKILLHUB_BIN=release` now works. The value is kept before the loop that unsets `SKILLHUB_*`.
  Checked: the release installs into the sandbox, and the real `~/.bashrc` and `~/.zshrc` are untouched.
- **`web/record.mjs`:** Playwright records the viewport in CSS pixels, so the 1280x720 viewport filled only the top
  left of the 1920x1080 video. The viewport is now 1920x1080 and the page is zoomed to 150 %.

## Still open

1. **The two Claude Code shots** (scene 6, "curate my Skill Hub", and scene 7c, "/commit-style") are illustrations.
   To replace them with real recordings, Claude Code needs a demo config folder (`CLAUDE_CONFIG_DIR`) that the owner
   signs in to once.
2. **Line-precise highlight boxes** from `script.md` are replaced by result callouts; boxes on exact lines would
   break whenever CLI output changes.
3. **Local tag `v0.7.1`** differs from the one on GitHub (it was moved), so `git fetch --tags` refuses it. Not
   touched; `git fetch --no-tags` works.
4. **Remotion licence:** free for individuals and companies of up to 3 people. The owner confirms before publishing.
5. **Still to make:** `short.mp4` and `preview.gif`.

## Problems to report (not fixed)

- `skill create` writes "Use when write a commit message."
- Web UI: the label "Waiting for rev..." is cut off. The licence of a submitted skill shows "not stated".
- `write_instructions` exists twice, in `happy-path.sh` and in `tapes/helpers.sh`.
- Once, during the music search, a request to the Wikimedia API carried the user name in its User-Agent. Later
  requests used a neutral User-Agent.
