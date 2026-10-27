import type { ReactElement } from "react";
import { AbsoluteFill, Audio, OffthreadVideo, Sequence, interpolate, staticFile, useCurrentFrame } from "remotion";
import { Background, Mark, Pill, Pop, Tile, c, display, headline, mono, sticker, usePop } from "./theme";

const fps = 30;
const sec = (s: number) => Math.round(s * fps);
// Shot lengths follow plans/261027-0830-video-redo/film-1-intro-script.md
export const SHOT_FRAMES = {
	hook: sec(5),
	chore: 240,
	compare: 270,
	hub: sec(5),
	agent: 251,
	cta: sec(6),
	total: sec(5) + 240 + 270 + sec(5) + 251 + sec(6),
};

const REPOS: [string, string, string][] = [
	["~/api-gateway", "v1.3", c.green],
	["~/billing", "v0.9", c.pink],
	["~/web-app", "v1.1", c.yellow],
	["~/infra", "v0.8", c.pink],
	["~/mobile", "v1.2", c.yellow],
	["~/search", "v0.9", c.pink],
	["~/auth", "v1.0", c.pink],
	["~/docs", "v1.1", c.yellow],
	["~/etl", "v0.7", c.pink],
];

const Hook = () => (
	<Background>
		<div style={{ position: "absolute", left: 66, top: 190 }}>
			<Pop>
				<div style={{ ...headline(170), color: c.cream }}>One skill.</div>
			</Pop>
			<Pop delay={14}>
				<div style={{ ...headline(170), color: c.pink }}>Nine copies.</div>
			</Pop>
		</div>
		<Pop delay={20} style={{ position: "absolute", left: 66, top: 560, width: 948 }}>
			<div style={{ fontSize: 54, fontWeight: 700, color: c.muted, lineHeight: 1.2 }}>
				<span style={{ fontFamily: mono, color: c.cream }}>commit-style</span> in 9 projects.
			</div>
		</Pop>
		<div style={{ position: "absolute", left: 66, top: 760, width: 948, display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 24 }}>
			{REPOS.map(([repo, v, col], i) => (
				<Pop key={repo} delay={22 + i * 3} style={{ transform: undefined }}>
					<div style={{ transform: `rotate(${[-2.5, 2, -1][i % 3]}deg)` }}>
						<Tile repo={repo} version={v} color={col} />
					</div>
				</Pop>
			))}
		</div>
		<Pop delay={64} style={{ position: "absolute", left: 66, top: 1400, width: 948 }}>
			<div style={{ fontSize: 66, fontWeight: 700, color: c.muted, lineHeight: 1.15 }}>2 hours a week to keep them synced?</div>
			<div style={{ marginTop: 18, fontFamily: mono, fontSize: 30, color: c.muted }}>(example)</div>
		</Pop>
	</Background>
);

const STEPS = ["Copy to project A", "Copy to project B", "Copy to the laptop", "Fix it in one place", "Forget the others"];

const Chore = () => {
	const frame = useCurrentFrame();
	const meter = interpolate(frame, [10, 150], [20, 90], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
	const stale = Math.round((meter / 100) * 9);
	return (
		<Background>
			<Pop style={{ position: "absolute", left: 66, top: 150 }}>
				<div style={{ ...headline(120), color: c.cream }}>Every fix is</div>
				<div style={{ ...headline(120), color: c.pink }}>five steps.</div>
			</Pop>
			<div style={{ position: "absolute", left: 66, top: 520, width: 948 }}>
				{STEPS.map((s, i) => {
					const last = i === STEPS.length - 1;
					const col = last ? c.pink : c.green;
					return (
						<Pop key={s} delay={10 + i * 20} style={{ marginBottom: 30 }}>
							<div style={{ ...sticker, display: "flex", alignItems: "center", gap: 28, padding: "22px 30px", borderColor: last ? c.pink : c.cream }}>
								<div style={{ width: 64, height: 64, borderRadius: 32, background: col, color: c.ink, fontWeight: 800, fontSize: 44, display: "flex", alignItems: "center", justifyContent: "center", fontFamily: mono }}>
									{last ? "✗" : "✓"}
								</div>
								<div style={{ fontSize: 54, fontWeight: 700, color: last ? c.pink : c.cream }}>{s}</div>
							</div>
						</Pop>
					);
				})}
			</div>
			<div style={{ position: "absolute", left: 66, top: 1330, width: 948 }}>
				<div style={{ fontFamily: mono, fontSize: 36, color: c.muted, display: "flex", justifyContent: "space-between" }}>
					<span>Out of sync</span>
					<span style={{ color: c.pink, fontWeight: 700 }}>{Math.round(meter)}%</span>
				</div>
				<div style={{ ...sticker, marginTop: 16, height: 56, borderRadius: 28, boxShadow: "none", overflow: "hidden" }}>
					<div style={{ width: `${meter}%`, height: "100%", background: c.pink }} />
				</div>
				<div style={{ marginTop: 40, display: "flex", gap: 16 }}>
					{Array.from({ length: 9 }, (_, i) => (
						<div key={i} style={{ flex: 1, height: 64, borderRadius: 14, border: `3px solid ${c.cream}`, background: i < stale ? c.pink : c.green, boxShadow: `5px 5px 0 ${c.shadow}` }} />
					))}
				</div>
			</div>
		</Background>
	);
};

const Panel = ({ top, title, color, lines, total, delay }: { top: number; title: string; color: string; lines: string[]; total: string; delay: number }) => (
	<div style={{ position: "absolute", left: 66, top, width: 948, height: 760 }}>
		<Pop delay={delay} style={{ height: "100%" }}>
			<div style={{ ...sticker, borderColor: color, height: "100%", padding: "40px 48px", boxSizing: "border-box" }}>
				<div style={{ ...headline(84), color }}>{title}</div>
				<div style={{ marginTop: 36 }}>
					{lines.map((l, i) => (
						<Pop key={l} delay={delay + 14 + i * 38} style={{ marginBottom: 20 }}>
							<div style={{ fontSize: 50, fontWeight: 700, color: c.cream, display: "flex", gap: 20 }}>
								<span style={{ color }}>{color === c.pink ? "•" : "✓"}</span>
								{l}
							</div>
						</Pop>
					))}
				</div>
				<Pop delay={delay + 150} style={{ position: "absolute", left: 48, bottom: 36 }}>
					<Pill color={color} size={52}>
						{total}
					</Pill>
					<span style={{ marginLeft: 20, fontFamily: mono, fontSize: 30, color: c.muted }}>(example)</span>
				</Pop>
			</div>
		</Pop>
	</div>
);

const Compare = () => (
	<Background>
		<Panel top={110} title="Copy by hand" color={c.pink} delay={0} total="~2 h/week" lines={["Copy into each project", "Fix one, miss others", "Redo on a new laptop"]} />
		<Panel top={910} title="With Skillstation" color={c.green} delay={12} total="~5 min/week" lines={["Edit once in the hub", "Agents ask the hub", "Nothing to copy"]} />
	</Background>
);

const Hub = () => {
	const frame = useCurrentFrame();
	const draw = interpolate(frame, [10, 45], [1, 0], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
	const agents = [
		{ y: 1330, label: "agent A" },
		{ y: 1480, label: "agent B" },
		{ y: 1630, label: "agent C" },
	];
	return (
		<Background>
			<Pop style={{ position: "absolute", left: 66, top: 180 }}>
				<div style={{ ...headline(190), color: c.cream }}>One hub.</div>
			</Pop>
			<Pop delay={14} style={{ position: "absolute", left: 66, top: 380 }}>
				<div style={{ ...headline(190), color: c.green, textShadow: `7px 7px 0 ${c.ink}` }}>Any agent.</div>
			</Pop>
			<svg width={1080} height={1920} style={{ position: "absolute", inset: 0 }}>
				{agents.map((a, i) => (
					<path key={a.label} d={`M 240 ${a.y} C 380 ${a.y}, 400 960, 540 960`} stroke={c.green} strokeWidth={8} fill="none" strokeLinecap="round" strokeDasharray="1 22" strokeDashoffset={draw * 600 + i * 20} />
				))}
			</svg>
			<Pop delay={6} style={{ position: "absolute", left: 540 - 210, top: 790 }}>
				<Mark size={420} />
			</Pop>
			<Pop delay={16} style={{ position: "absolute", left: 0, width: 1080, top: 1230, textAlign: "center" }}>
				<div style={{ ...headline(76), color: c.cream, position: "relative", left: 200 }}>your hub</div>
			</Pop>
			{agents.map((a, i) => (
				<Pop key={a.label} delay={22 + i * 6} style={{ position: "absolute", left: 66, top: a.y - 40, width: 240 }}>
					<div style={{ background: c.bg3, border: `3px solid rgba(240,236,226,.4)`, borderRadius: 999, padding: "10px 20px", fontFamily: mono, fontSize: 28, whiteSpace: "nowrap" }}>● {a.label}</div>
				</Pop>
			))}
		</Background>
	);
};

// Shot 5: a real Claude Code run (video/demos/tapes/07-intro-agent.tape), cut into three parts and sped up.
// The Checked-by box is placed for this take; measure it again after every new recording.
const TAKE = "recordings/tapes/07-intro-agent.mp4";
const VIDEO_W = 948;
const VIDEO_H = Math.round((1180 * VIDEO_W) / 1000);
const PARTS = [
	{ from: 75, rate: 1.7, frames: 62, step: "1", text: "You ask for your skill.", sub: "" },
	{ from: 180, rate: 4.1, frames: 84, step: "2", text: "The agent asks the hub.", sub: "skill_resolve  →  skill_get" },
	{ from: 525, rate: 1, frames: 105, step: "3", text: "It follows the latest version.", sub: "" },
];
export const AGENT_FRAMES = PARTS.reduce((n, p) => n + p.frames, 0);

const Caption = ({ step, text, sub }: { step: string; text: string; sub: string }) => (
	<Pop style={{ position: "absolute", left: 66, top: 1440, width: 948 }}>
		<div style={{ display: "flex", gap: 28, alignItems: "flex-start" }}>
			<div style={{ width: 96, height: 96, flex: "none", borderRadius: 48, background: c.green, color: c.ink, fontWeight: 800, fontSize: 64, display: "flex", alignItems: "center", justifyContent: "center", boxShadow: `6px 6px 0 ${c.shadow}` }}>{step}</div>
			<div>
				<div style={{ ...headline(84), color: c.cream, lineHeight: 1.02 }}>{text}</div>
				{sub ? <div style={{ marginTop: 22 }}><Pill color={c.green} size={46}>{sub}</Pill></div> : null}
			</div>
		</div>
	</Pop>
);

const CheckedBox = () => {
	const p = usePop(35);
	return (
		<div style={{ position: "absolute", left: 116, top: 910, width: 500, height: 52, border: `4px solid ${c.green}`, borderRadius: 14, opacity: Math.min(1, p * 2), transform: `scale(${0.8 + 0.2 * p})`, boxShadow: `0 0 0 6px rgba(159,240,127,.25)` }} />
	);
};

const Agent = () => {
	let start = 0;
	return (
		<Background>
			<Pop style={{ position: "absolute", left: 66, top: 120 }}>
				<Pill color={c.yellow} size={32}>
					real Claude Code run, sped up
				</Pill>
			</Pop>
			<div style={{ position: "absolute", left: 66, top: 230, width: VIDEO_W, height: VIDEO_H, ...sticker, borderRadius: 26, overflow: "hidden" }} />
			{PARTS.map((part, i) => {
				const from = start;
				start += part.frames;
				return (
					<Sequence key={part.step} from={from} durationInFrames={part.frames}>
						<div style={{ position: "absolute", left: 66, top: 230, width: VIDEO_W, height: VIDEO_H, overflow: "hidden", borderRadius: 24, background: c.ink }}>
							<OffthreadVideo src={staticFile(TAKE)} startFrom={part.from} playbackRate={part.rate} muted style={{ width: VIDEO_W, height: VIDEO_H }} />
						</div>
						<Caption step={part.step} text={part.text} sub={part.sub} />
						{i === 2 ? <CheckedBox /> : null}
					</Sequence>
				);
			})}
		</Background>
	);
};

const Cta = () => {
	const p = usePop();
	return (
		<Background>
			<div style={{ position: "absolute", left: 0, width: 1080, top: 220, display: "flex", flexDirection: "column", alignItems: "center", transform: `scale(${0.6 + 0.4 * p})`, opacity: Math.min(1, p * 2) }}>
				<Mark size={300} />
				<div style={{ ...headline(120), marginTop: 20 }}>skillstation</div>
			</div>
			<Pop delay={12} style={{ position: "absolute", left: 66, top: 800, width: 948 }}>
				<div style={{ ...headline(110), color: c.cream }}>Your skills in one place.</div>
			</Pop>
			<Pop delay={24} style={{ position: "absolute", left: 66, top: 1090, width: 948 }}>
				<div style={{ ...headline(110), color: c.green, textShadow: `6px 6px 0 ${c.ink}` }}>Pulled by any agent, on demand.</div>
			</Pop>
			<Pop delay={44} style={{ position: "absolute", left: 66, top: 1560, width: 948 }}>
				<div style={{ ...sticker, padding: "30px 34px", fontFamily: mono, fontSize: 31, lineHeight: 1.5 }}>
					<span style={{ color: c.green }}>$ </span>curl -fsSL \<br />
					&nbsp;&nbsp;https://github.com/vantt/skillstation/<br />
					&nbsp;&nbsp;releases/latest/download/install.sh | sh
				</div>
			</Pop>
		</Background>
	);
};

// Music: "You're Somewhere in Between" by Michael Ramir C., Mixkit Stock Music Free License (see README.md).
// The first 15 s are a quiet intro, so the cut starts at 0:15. Click and whoosh are made with ffmpeg.
const MUSIC = "audio/youre-somewhere-in-between.mp3";
const MUSIC_START = 15 * fps;

export const Sound = () => {
	const frame = useCurrentFrame();
	const cuts: number[] = [];
	let at = 0;
	for (const k of ["hook", "chore", "compare", "hub", "agent"] as const) {
		at += SHOT_FRAMES[k];
		cuts.push(at);
	}
	const choreStart = SHOT_FRAMES.hook;
	const ticks = [0, 1, 2, 3].map((i) => choreStart + 10 + i * 20 + 4);
	const boxAt = cuts[3] + PARTS[0].frames + PARTS[1].frames + 35;
	const fade = interpolate(frame, [0, 12, SHOT_FRAMES.total - 54, SHOT_FRAMES.total], [0, 1, 1, 0], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
	return (
		<>
			<Audio src={staticFile(MUSIC)} startFrom={MUSIC_START} volume={0.75 * fade} />
			{cuts.map((f) => (
				<Sequence key={`w${f}`} from={f - 4} durationInFrames={40}>
					<Audio src={staticFile("audio/whoosh.wav")} volume={0.3} />
				</Sequence>
			))}
			{[...ticks, boxAt].map((f) => (
				<Sequence key={`c${f}`} from={f} durationInFrames={10}>
					<Audio src={staticFile("audio/click.wav")} volume={0.5} />
				</Sequence>
			))}
		</>
	);
};

export const Intro = () => {
	const order: [keyof typeof SHOT_FRAMES, () => ReactElement][] = [
		["hook", Hook],
		["chore", Chore],
		["compare", Compare],
		["hub", Hub],
		["agent", Agent],
		["cta", Cta],
	];
	let from = 0;
	return (
		<AbsoluteFill>
			<Sound />
			{order.map(([k, Shot]) => {
				const start = from;
				from += SHOT_FRAMES[k];
				return (
					<Sequence key={k} from={start} durationInFrames={SHOT_FRAMES[k]}>
						<Shot />
					</Sequence>
				);
			})}
		</AbsoluteFill>
	);
};
