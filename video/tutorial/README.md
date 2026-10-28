# Film 2: tutorial

This folder holds the 16:9 how-to film. It is cut in code, so it can be made again after every release. To give
feedback, use the table in
[plans/261016-1000-intro-video/film-2-tutorial.md](../../plans/261016-1000-intro-video/film-2-tutorial.md).

| Path | What it is | In Git |
|---|---|---|
| `script.md` | Storyboard. The `Narration` row of each scene is the voice-over text. | yes |
| `edit.json` | The cut, per scene: shots, when each sentence starts, and the result callouts | yes |
| `narration.vi.json` | Vietnamese subtitles, one line per narration sentence | yes |
| `scripts/voice.mjs` | Builds the voice-over, the timeline and the subtitles | yes |
| `remotion/` | The Remotion project that renders the film | yes, without `node_modules/` |
| `audio/` | Voice clips, `timeline.json` and the music | no, rebuilt |
| `output/` | Videos, plus the `.srt` files and `youtube.md` | only `.srt` and `youtube.md` |

`remotion/public/` links to `../../recordings`, `../audio` and `docs/images/brand`, so the cut reads them in place.
`edit.json` paths such as `recordings/tapes/...` are resolved through those links.

## Rebuild everything

Install the tools first; see [../README.md](../README.md#tools). Then, from the repository root:

```bash
export SKILLSTATION_BIN=release
video/demos/happy-path.sh
video/demos/web/run.sh
video/demos/tapes/render.sh
node video/tutorial/scripts/voice.mjs
```

What each command does:

1. `happy-path.sh` runs the documented steps as a check. It stops on the first step that fails.
2. `web/run.sh` records the two web UI scenes.
3. `tapes/render.sh` records the terminal scenes. Stop any hub server of yours on port 7420 first.
4. `voice.mjs` writes the voice-over to `audio/` and the subtitles to `output/`.

Get the music once, then render:

```bash
mkdir -p video/tutorial/audio/music /tmp/chill-beats
curl -L -o /tmp/chill-beats/chill_beats.zip https://opengameart.org/sites/default/files/chill_beats.zip
python3 -I -c "import zipfile; zipfile.ZipFile('/tmp/chill-beats/chill_beats.zip').extractall('/tmp/chill-beats')"
t="/tmp/chill-beats/Chill Beats/01 HoliznaCC0 - City In The Rearview.ogg"
ffmpeg -i "$t" -i "$t" -filter_complex "[0][1]acrossfade=d=4:c1=tri:c2=tri" -c:a libvorbis -q:a 4 video/tutorial/audio/music/city-in-the-rearview.ogg
cd video/tutorial/remotion && npm ci
npm run draft
npm run render
```

`npm run draft` writes `output/draft.mp4` at 960x540. `npm run render` writes `output/full.mp4` at 1920x1080 and 30 fps, then brings its audio to -16 LUFS (`scripts/normalize.mjs`, picture copied).
To scrub through the cut, run `npm run studio`.

## After a release

1. Run the four rebuild commands again.
2. If a recording changed length, adjust `from`, `to`, `say` and `callouts` in `edit.json` for that scene. To find the
   new times, look at the tape with a frame every second:

   ```bash
   ffmpeg -i video/recordings/tapes/04-first-skill.mp4 -vf "fps=1,scale=240:-1,tile=10x5" -frames:v 1 sheet.png
   ```
3. If the narration changed in `script.md`, update `narration.vi.json` too. It must keep one line per sentence;
   `node video/tutorial/scripts/voice.mjs --list` numbers them.
4. Render the draft and watch it before the final render.
