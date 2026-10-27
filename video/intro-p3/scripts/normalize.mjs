// Bring the audio of a rendered film to -16 LUFS in place; the picture is copied, not re-encoded.
import { execFileSync } from "node:child_process";
import { renameSync } from "node:fs";

const file = process.argv[2];
if (!file) throw new Error("usage: node scripts/normalize.mjs <video.mp4>");
const tmp = file.replace(/\.mp4$/, ".norm.mp4");
execFileSync("ffmpeg", ["-loglevel", "error", "-y", "-i", file, "-c:v", "copy", "-af", "loudnorm=I=-16:TP=-1.5:LRA=11", "-c:a", "aac", "-b:a", "192k", tmp]);
renameSync(tmp, file);
console.log(`normalized ${file}`);
