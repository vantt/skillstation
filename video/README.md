# Videos

Two films, each in its own folder with its own build, outputs and feedback. The brief and both films' status and
feedback tables are in [plans/261016-1000-intro-video/](../plans/261016-1000-intro-video/plan.md).

| Film | Folder | Format | Status and feedback |
|---|---|---|---|
| 1. Intro | [`intro/`](intro/README.md) | 9:16, about 1:25, music only, HyperFrames | [film-1-intro.md](../archive/plans/261016-1000-intro-film/plan.md) |
| 2. Tutorial | [`tutorial/`](tutorial/README.md) | 16:9, about 4:10, voice-over and subtitles, Remotion | [film-2-tutorial.md](../plans/261016-1000-intro-video/film-2-tutorial.md) |

Shared by both, used by the tutorial:

| Path | What it is | In Git |
|---|---|---|
| `demos/` | Sandbox, happy path, Playwright web scenes and VHS tapes. See [demos/README.md](demos/README.md). | yes |
| `recordings/` | Terminal and web recordings that `demos/` makes | no, rebuilt |

## Tools

These tools are needed on PATH:

- `vhs`, `ttyd`, `ffmpeg` and `ffprobe`. Use **ffmpeg 9 or newer**: 8.1 blackens the last 8 pixel columns of a
  1080-wide render.
- Node 20 or newer.
- `edge-tts`, for the tutorial's voice. Install it with `uv tool install edge-tts`.

The web scenes also need `npm ci` and `npx playwright install chromium`, run once in `web/`.

The binary under test is set by `SKILLHUB_BIN`:

- `SKILLHUB_BIN=release`: the latest release, installed into the sandbox;
- `SKILLHUB_BIN=<path>`: a binary you have;
- not set: built from this repository with Go.

## Sources and licences

- **Tutorial music:** "Mellow Cafe Vibe" by John Bartmann, CC0, from
  [Wikimedia Commons](https://commons.wikimedia.org/wiki/File:John_Bartmann_-_Mellow_Cafe_Vibe.ogg).
- **Intro v2 music (`intro-p3/`):** "You're Somewhere in Between" by Michael Ramir C., from Mixkit, under the Mixkit
  Stock Music Free License (not CC; commercial use and YouTube allowed, no attribution needed; the file is not
  committed). Click and whoosh are made with ffmpeg.
- **Old intro music:** the explainer skill's bundled bed. It is AI-generated and provided by the skill for convenience; for
  paid advertising, use a track you hold a licence for.
- **Tutorial voice:** Microsoft Edge neural voice `en-US-AndrewNeural`, through `edge-tts`. Only the narration text is
  sent.
- **Logos:** `docs/images/brand/`, this project's own.
- **Fonts:** the intro uses Mali and Be Vietnam Pro (SIL OFL 1.1, bundled by the skill). The tutorial uses the system
  fonts.
- **Remotion:** free for individuals and companies of up to 3 people; larger companies need a licence. See
  [remotion.dev/license](https://www.remotion.dev/license) before you publish.
