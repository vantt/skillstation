import type { ReactElement } from "react";
import { AbsoluteFill, OffthreadVideo, Sequence, interpolate, staticFile, useCurrentFrame } from "remotion";
import { SHOT_FRAMES, Sound } from "./Intro";
import { Background, Mark, Pill, Pop, Tile, c, headline, mono, sticker, usePop } from "./theme";

// 16:9 cut of the intro for the GitHub README: same story and timing as Intro.tsx, no sound.
const W = 1920;

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
		<div style={{ position: "absolute", left: 110, top: 250 }}>
			<Pop>
				<div style={{ ...headline(132), color: c.cream }}>One skill.</div>
			</Pop>
			<Pop delay={14}>
				<div style={{ ...headline(132), color: c.pink }}>Nine copies.</div>
			</Pop>
		</div>
		<Pop delay={20} style={{ position: "absolute", left: 110, top: 600, width: 800 }}>
			<div style={{ fontSize: 56, fontWeight: 700, color: c.muted, lineHeight: 1.2 }}>
				<span style={{ fontFamily: mono, color: c.cream }}>commit-style</span> in 9 projects.
			</div>
		</Pop>
		<div style={{ position: "absolute", left: 940, top: 240, width: 880, display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 20 }}>
			{REPOS.map(([repo, v, col], i) => (
				<Pop key={repo} delay={22 + i * 3}>
					<div style={{ transform: `rotate(${[-2.5, 2, -1][i % 3]}deg)` }}>
						<Tile repo={repo} version={v} color={col} />
					</div>
				</Pop>
			))}
		</div>
		<Pop delay={64} style={{ position: "absolute", left: 110, top: 770, width: 760 }}>
			<div style={{ fontSize: 56, fontWeight: 700, color: c.muted, lineHeight: 1.15 }}>2 hours a week to keep them synced?</div>
			<div style={{ marginTop: 14, fontFamily: mono, fontSize: 28, color: c.muted }}>(example)</div>
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
			<Pop style={{ position: "absolute", left: 110, top: 70 }}>
				<div style={{ ...headline(110), color: c.cream, display: "inline" }}>Every fix is </div>
				<div style={{ ...headline(110), color: c.pink, display: "inline" }}>five steps.</div>
			</Pop>
			<div style={{ position: "absolute", left: 110, top: 250, width: 880 }}>
				{STEPS.map((s, i) => {
					const last = i === STEPS.length - 1;
					const col = last ? c.pink : c.green;
					return (
						<Pop key={s} delay={10 + i * 20} style={{ marginBottom: 22 }}>
							<div style={{ ...sticker, display: "flex", alignItems: "center", gap: 26, padding: "18px 28px", borderColor: last ? c.pink : c.cream }}>
								<div style={{ width: 62, height: 62, flex: "none", borderRadius: 31, background: col, color: c.ink, fontWeight: 800, fontSize: 42, display: "flex", alignItems: "center", justifyContent: "center", fontFamily: mono }}>
									{last ? "✗" : "✓"}
								</div>
								<div style={{ fontSize: 50, fontWeight: 700, color: last ? c.pink : c.cream }}>{s}</div>
							</div>
						</Pop>
					);
				})}
			</div>
			<div style={{ position: "absolute", left: 1100, top: 250, width: 720 }}>
				<div style={{ fontFamily: mono, fontSize: 38, color: c.muted }}>Out of sync</div>
				<div style={{ ...headline(230), color: c.pink, marginTop: 6 }}>{Math.round(meter)}%</div>
				<div style={{ ...sticker, marginTop: 26, height: 52, borderRadius: 26, boxShadow: "none", overflow: "hidden" }}>
					<div style={{ width: `${meter}%`, height: "100%", background: c.pink }} />
				</div>
				<div style={{ marginTop: 40, display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 16 }}>
					{Array.from({ length: 9 }, (_, i) => (
						<div key={i} style={{ height: 78, borderRadius: 16, border: `3px solid ${c.cream}`, background: i < stale ? c.pink : c.green, boxShadow: `6px 6px 0 ${c.shadow}` }} />
					))}
				</div>
			</div>
		</Background>
	);
};

const Panel = ({ left, title, color, lines, total, delay }: { left: number; title: string; color: string; lines: string[]; total: string; delay: number }) => (
	<div style={{ position: "absolute", left, top: 110, width: 840, height: 860 }}>
		<Pop delay={delay} style={{ height: "100%" }}>
			<div style={{ ...sticker, borderColor: color, height: "100%", padding: "44px 52px", boxSizing: "border-box", position: "relative" }}>
				<div style={{ ...headline(92), color }}>{title}</div>
				<div style={{ marginTop: 44 }}>
					{lines.map((l, i) => (
						<Pop key={l} delay={delay + 14 + i * 38} style={{ marginBottom: 26 }}>
							<div style={{ fontSize: 54, fontWeight: 700, color: c.cream, display: "flex", gap: 22 }}>
								<span style={{ color }}>{color === c.pink ? "•" : "✓"}</span>
								{l}
							</div>
						</Pop>
					))}
				</div>
				<Pop delay={delay + 150} style={{ position: "absolute", left: 52, bottom: 44 }}>
					<Pill color={color} size={60}>
						{total}
					</Pill>
					<span style={{ marginLeft: 22, fontFamily: mono, fontSize: 32, color: c.muted }}>(example)</span>
				</Pop>
			</div>
		</Pop>
	</div>
);

const Compare = () => (
	<Background>
		<Panel left={90} title="Copy by hand" color={c.pink} delay={0} total="~2 h/week" lines={["Copy into each project", "Fix one, miss others", "Redo on a new laptop"]} />
		<Panel left={990} title="With Skillstation" color={c.green} delay={12} total="~5 min/week" lines={["Edit once in the hub", "Agents ask the hub", "Nothing to copy"]} />
	</Background>
);

const Hub = () => {
	const frame = useCurrentFrame();
	const draw = interpolate(frame, [10, 45], [1, 0], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
	const ys = [190, 340, 490];
	const labels = ["agent A", "agent B", "agent C"];
	return (
		<Background>
			<svg width={W} height={1080} style={{ position: "absolute", inset: 0 }}>
				{ys.flatMap((y, i) => [
					<path key={`l${i}`} d={`M 380 ${y + 30} C 600 ${y + 30}, 620 340, 790 340`} stroke={c.green} strokeWidth={7} fill="none" strokeLinecap="round" strokeDasharray="1 20" strokeDashoffset={draw * 600 + i * 20} />,
					<path key={`r${i}`} d={`M 1540 ${y + 30} C 1320 ${y + 30}, 1300 340, 1130 340`} stroke={c.green} strokeWidth={7} fill="none" strokeLinecap="round" strokeDasharray="1 20" strokeDashoffset={draw * 600 + i * 20} />,
				])}
			</svg>
			<Pop delay={6} style={{ position: "absolute", left: 960 - 180, top: 150 }}>
				<Mark size={360} />
			</Pop>
			<Pop delay={16} style={{ position: "absolute", left: 0, width: W, top: 520, textAlign: "center" }}>
				<div style={{ ...headline(64), color: c.cream }}>your hub</div>
			</Pop>
			{ys.map((y, i) => (
				<Pop key={`a${i}`} delay={22 + i * 6} style={{ position: "absolute", left: 110, top: y }}>
					<div style={{ background: c.bg3, border: `3px solid rgba(240,236,226,.4)`, borderRadius: 999, padding: "10px 26px", fontFamily: mono, fontSize: 32, whiteSpace: "nowrap" }}>● {labels[i]}</div>
				</Pop>
			))}
			{ys.map((y, i) => (
				<Pop key={`b${i}`} delay={34 + i * 6} style={{ position: "absolute", left: 1540, top: y }}>
					<div style={{ background: c.bg3, border: `3px solid rgba(240,236,226,.4)`, borderRadius: 999, padding: "10px 26px", fontFamily: mono, fontSize: 32, whiteSpace: "nowrap" }}>● {labels[i].replace("agent", "agent")}</div>
				</Pop>
			))}
			<Pop style={{ position: "absolute", left: 0, width: W, top: 740, textAlign: "center" }}>
				<span style={{ ...headline(170), color: c.cream }}>One hub. </span>
				<span style={{ ...headline(170), color: c.green, textShadow: `7px 7px 0 ${c.ink}` }}>Any agent.</span>
			</Pop>
		</Background>
	);
};

// Shot 5: the real take, landscape (video/demos/tapes/08-intro-agent-wide.tape), cut in three parts and sped up.
// The Checked-by box is measured on this take; measure it again after every new recording.
const TAKE = "recordings/tapes/08-intro-agent-wide.mp4";
const VW = 1200;
const VH = 960;
const WIN_LEFT = 60;
const WIN_TOP = 60;
const PARTS = [
	{ from: 90, rate: 3.0, frames: 64, step: "1", text: "You ask for your skill.", sub: "" },
	{ from: 282, rate: 4.07, frames: 84, step: "2", text: "The agent asks the hub.", sub: "skill_resolve → skill_get" },
	{ from: 624, rate: 1, frames: 105, step: "3", text: "It follows the latest version.", sub: "" },
];
export const AGENT_FRAMES_WIDE = PARTS.reduce((n, p) => n + p.frames, 0);

const Caption = ({ step, text, sub }: { step: string; text: string; sub: string }) => (
	<Pop style={{ position: "absolute", left: 1330, top: 380, width: 530 }}>
		<div style={{ width: 100, height: 100, borderRadius: 50, background: c.green, color: c.ink, fontWeight: 800, fontSize: 68, display: "flex", alignItems: "center", justifyContent: "center", boxShadow: `6px 6px 0 ${c.shadow}` }}>{step}</div>
		<div style={{ ...headline(76), color: c.cream, lineHeight: 1.02, marginTop: 28 }}>{text}</div>
		{sub ? (
			<div style={{ marginTop: 26 }}>
				<Pill color={c.green} size={34}>
					{sub}
				</Pill>
			</div>
		) : null}
	</Pop>
);

const CheckedBox = () => {
	const p = usePop(35);
	return (
		<div style={{ position: "absolute", left: WIN_LEFT + 60 - 12, top: WIN_TOP + 622 - 25, width: 470, height: 50, border: `4px solid ${c.green}`, borderRadius: 12, opacity: Math.min(1, p * 2), transform: `scale(${0.8 + 0.2 * p})`, boxShadow: `0 0 0 6px rgba(159,240,127,.25)` }} />
	);
};

const Agent = () => {
	let start = 0;
	return (
		<Background>
			<Pop style={{ position: "absolute", left: 1330, top: 110 }}>
				<Pill color={c.yellow} size={28}>
					real Claude Code run, sped up
				</Pill>
			</Pop>
			<div style={{ position: "absolute", left: WIN_LEFT, top: WIN_TOP, width: VW, height: VH, ...sticker, borderRadius: 26 }} />
			{PARTS.map((part, i) => {
				const from = start;
				start += part.frames;
				return (
					<Sequence key={part.step} from={from} durationInFrames={part.frames}>
						<div style={{ position: "absolute", left: WIN_LEFT, top: WIN_TOP, width: VW, height: VH, overflow: "hidden", borderRadius: 24, background: c.ink }}>
							<OffthreadVideo src={staticFile(TAKE)} startFrom={part.from} playbackRate={part.rate} muted style={{ width: VW, height: VH }} />
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
			<div style={{ position: "absolute", left: 110, top: 120, display: "flex", alignItems: "center", gap: 36, transform: `scale(${0.6 + 0.4 * p})`, transformOrigin: "left center", opacity: Math.min(1, p * 2) }}>
				<Mark size={220} />
				<div style={{ ...headline(170) }}>skillstation</div>
			</div>
			<Pop delay={20} style={{ position: "absolute", left: 110, top: 400, width: 1700 }}>
				<div style={{ ...headline(120), color: c.cream }}>Your skills in one place.</div>
			</Pop>
			<Pop delay={30} style={{ position: "absolute", left: 110, top: 530, width: 1700 }}>
				<div style={{ ...headline(100), color: c.green, textShadow: `6px 6px 0 ${c.ink}` }}>Pulled by any agent, on demand.</div>
			</Pop>
			<Pop delay={50} style={{ position: "absolute", left: 110, top: 790, width: 1700 }}>
				<div style={{ ...sticker, padding: "34px 40px", fontFamily: mono, fontSize: 28, whiteSpace: "nowrap" }}>
					<span style={{ color: c.green }}>$ </span>curl -fsSL https://github.com/vantt/skillstation/releases/latest/download/install.sh | sh
				</div>
			</Pop>
		</Background>
	);
};

export const IntroWide = ({ withSound = false }: { withSound?: boolean }) => {
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
			{withSound ? <Sound /> : null}
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
