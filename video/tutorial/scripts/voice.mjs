#!/usr/bin/env node
// Voice-over, timeline and subtitles of the tutorial film, from the narration in video/tutorial/script.md.
//
//   node video/tutorial/scripts/voice.mjs            all scenes
//   node video/tutorial/scripts/voice.mjs --list     print the sentences, numbered, and stop (for translating)
//
// Each sentence of a scene's "Narration" row becomes one edge-tts clip, so every subtitle cue is one sentence
// with its real length. video/tutorial/edit.json says when each sentence starts in its scene and which shots the scene
// plays; a scene lasts as long as its shots, or longer when the voice needs it. The clips of a scene are placed
// over silence in video/tutorial/audio/scene-NN.wav. Then:
//   video/tutorial/audio/timeline.json         scene starts and lengths in seconds, and every cue; Remotion reads it
//   video/tutorial/output/subtitles.en.srt     the narration
//   video/tutorial/output/subtitles.vi.srt     the same cues, text from video/tutorial/narration.vi.json
// Needs edge-tts, ffmpeg and ffprobe on PATH. Sends only the narration text to Microsoft's speech service.
import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const videoDir = join(dirname(fileURLToPath(import.meta.url)), "..");
const audioDir = join(videoDir, "audio");
const outDir = join(videoDir, "output");
const VOICE = process.env.VOICE ?? "en-US-AndrewNeural";
const RATE = process.env.VOICE_RATE ?? "+0%";
const GAP = 0.35; // silence between two sentences of a scene
const LEAD = 0.4; // before the first sentence of a scene
const TAIL = 0.8; // after the last sentence, before the next scene starts

// How a word is spoken, where the written form would be read wrongly. Subtitles keep the written form.
const SPOKEN = [
	[/\bskillstation\b/gi, "skill station"],
	[/\bMCP\b/g, "M C P"],
];

function scenes() {
	const md = readFileSync(join(videoDir, "script.md"), "utf8");
	const list = [];
	let current;
	for (const line of md.split("\n")) {
		const heading = line.match(/^## Scene (\d+)\. (.+?) \(/);
		if (heading) {
			current = { scene: Number(heading[1]), title: heading[2] };
			list.push(current);
			continue;
		}
		const narration = line.match(/^\| Narration \| "(.+)" \|$/);
		if (narration && current) current.text = narration[1];
	}
	for (const s of list) if (!s.text) throw new Error(`scene ${s.scene} has no Narration row`);
	return list;
}

// Sentence ends: . ! ? followed by a space and a capital letter, or the end of the text.
const sentences = (text) => text.split(/(?<=[.!?])\s+(?=[A-Z])/).map((s) => s.trim()).filter(Boolean);

const run = (cmd, args) => execFileSync(cmd, args, { stdio: ["ignore", "pipe", "pipe"] }).toString();
const seconds = (file) =>
	Number(run("ffprobe", ["-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", file]));
const pad = (n) => String(n).padStart(2, "0");

function srtTime(t) {
	const ms = Math.round(t * 1000);
	const h = Math.floor(ms / 3600000), m = Math.floor(ms / 60000) % 60, s = Math.floor(ms / 1000) % 60;
	return `${pad(h)}:${pad(m)}:${pad(s)},${String(ms % 1000).padStart(3, "0")}`;
}

function srt(cues, textOf) {
	return cues.map((c, i) => `${i + 1}\n${srtTime(c.start)} --> ${srtTime(c.end)}\n${textOf(c, i)}\n`).join("\n");
}

const all = scenes();
const edit = JSON.parse(readFileSync(join(videoDir, "edit.json"), "utf8")).scenes;
const shotSeconds = (shot) => shot.seconds ?? (shot.to - shot.from) / (shot.rate ?? 1);
if (process.argv.includes("--list")) {
	let n = 0;
	for (const s of all) for (const sentence of sentences(s.text)) console.log(`${++n}\t[${s.scene}] ${sentence}`);
	process.exit(0);
}

for (const tool of ["edge-tts", "ffmpeg", "ffprobe"]) {
	try {
		run("sh", ["-c", `command -v ${tool}`]);
	} catch {
		console.error(`voice.mjs: ${tool} not found on PATH`);
		process.exit(1);
	}
}

const work = join(audioDir, "clips");
rmSync(work, { recursive: true, force: true });
mkdirSync(work, { recursive: true });
mkdirSync(outDir, { recursive: true });

const timeline = { voice: VOICE, rate: RATE, scenes: [], cues: [] };
let clock = 0;
for (const s of all) {
	const parts = [];
	const plan = edit[s.scene];
	if (!plan) throw new Error(`edit.json has no scene ${s.scene}`);
	const said = sentences(s.text);
	if (plan.say.length !== said.length) {
		throw new Error(`edit.json scene ${s.scene}: say has ${plan.say.length} entries, the narration has ${said.length} sentences`);
	}
	const shotsLength = plan.shots.reduce((sum, shot) => sum + shotSeconds(shot), 0);
	let local = LEAD;
	const cueStart = timeline.cues.length;
	said.forEach((sentence, i) => {
		const wanted = plan.say[i];
		if (wanted != null && wanted + 0.001 < local) {
			console.warn(`scene ${s.scene} sentence ${i + 1}: wanted at ${wanted} s, starts at ${local.toFixed(2)} s (the one before is still speaking)`);
		}
		if (wanted != null) local = Math.max(local, wanted);
		let spoken = sentence;
		for (const [re, say] of SPOKEN) spoken = spoken.replace(re, say);
		const mp3 = join(work, `scene-${pad(s.scene)}-${pad(i + 1)}.mp3`);
		// The speech service is a network call; retry a few times before giving up.
		for (let attempt = 1; ; attempt++) {
			try {
				run("edge-tts", ["--voice", VOICE, `--rate=${RATE}`, "--text", spoken, "--write-media", mp3]);
				break;
			} catch (error) {
				if (attempt === 4) throw error;
				console.warn(`edge-tts failed (try ${attempt}), retrying`);
			}
		}
		const length = seconds(mp3);
		timeline.cues.push({ scene: s.scene, text: sentence, start: clock + local, end: clock + local + length });
		parts.push({ file: mp3, at: local });
		local += length + GAP;
	});
	const length = Math.max(local - GAP + TAIL, shotsLength);
	// One file per scene: each clip placed at its offset over silence, so the scene audio matches the cues.
	const wav = join(audioDir, `scene-${pad(s.scene)}.wav`);
	const inputs = parts.flatMap((p) => ["-i", p.file]);
	const delays = parts.map((p, i) => `[${i}]adelay=${Math.round(p.at * 1000)}:all=1[a${i}]`).join(";");
	const mix = parts.map((_, i) => `[a${i}]`).join("");
	run("ffmpeg", [
		"-v", "error", "-y", ...inputs,
		"-filter_complex", `${delays};${mix}amix=inputs=${parts.length}:normalize=0,apad=whole_dur=${length.toFixed(3)}`,
		"-ar", "48000", "-ac", "2", wav,
	]);
	timeline.scenes.push({
		scene: s.scene, title: s.title, start: clock, length: Number(length.toFixed(3)), shotsLength: Number(shotsLength.toFixed(3)),
		audio: `audio/scene-${pad(s.scene)}.wav`, cues: [cueStart, timeline.cues.length],
	});
	console.log(`scene ${pad(s.scene)}  ${length.toFixed(1)} s  ${s.title}`);
	clock += length;
}
timeline.length = Number(clock.toFixed(3));
for (const c of timeline.cues) (c.start = Number(c.start.toFixed(3))), (c.end = Number(c.end.toFixed(3)));
writeFileSync(join(audioDir, "timeline.json"), JSON.stringify(timeline, null, 2) + "\n");
writeFileSync(join(outDir, "subtitles.en.srt"), srt(timeline.cues, (c) => c.text));

const viFile = join(videoDir, "narration.vi.json");
if (existsSync(viFile)) {
	const vi = JSON.parse(readFileSync(viFile, "utf8"));
	if (vi.length !== timeline.cues.length) {
		console.error(`voice.mjs: narration.vi.json has ${vi.length} sentences, the narration has ${timeline.cues.length}; run --list`);
		process.exit(1);
	}
	writeFileSync(join(outDir, "subtitles.vi.srt"), srt(timeline.cues, (_, i) => vi[i]));
}
const m = Math.floor(clock / 60), sec = Math.round(clock % 60);
console.log(`total ${m}:${pad(sec)} (${timeline.cues.length} cues)`);
