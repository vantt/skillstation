import { AbsoluteFill, interpolate, useCurrentFrame, useVideoConfig } from "remotion";
import { c, display, mono } from "./data";
import { Background, Mark, Pill, Pop, sticker, Tile } from "./theme";

// Graphics leave the bottom 260 px free for subtitles and the chapter bar, and the top 170 px for the step title.
const Stage = ({ children, gap = 40 }: { children: React.ReactNode; gap?: number }) => (
	<Background>
		<AbsoluteFill style={{ justifyContent: "center", alignItems: "center", flexDirection: "column", gap, paddingTop: 120, paddingBottom: 250 }}>
			{children}
		</AbsoluteFill>
	</Background>
);

const Box = ({ label, sub, color, style }: { label: string; sub?: string; color: string; style?: React.CSSProperties }) => (
	<div
		style={{
			...sticker, border: `4px solid ${color}`, padding: "24px 34px", font: `800 42px ${display}`, textAlign: "center", color: c.cream, ...style,
		}}
	>
		{label}
		{sub ? <div style={{ fontFamily: mono, fontSize: 26, fontWeight: 400, marginTop: 10, color: c.muted }}>{sub}</div> : null}
	</div>
);

const seconds = () => {
	const frame = useCurrentFrame();
	const { fps } = useVideoConfig();
	return frame / fps;
};

// Scene 1: the problem. Three copies of one skill at different versions (pink = an old copy).
const Copies = () => (
	<Stage gap={56}>
		<div style={{ display: "flex", gap: 44 }}>
			{[
				["project-a", "v3", c.green, -2.5],
				["project-b", "v2", c.yellow, 2],
				["laptop", "v1", c.pink, -1],
			].map(([name, v, color, tilt], i) => (
				<Pop key={String(name)} delay={6 + i * 6} style={{ transform: `rotate(${tilt}deg)` }}>
					<Tile repo={`${name}/.claude/skills`} version={String(v)} color={String(color)} />
				</Pop>
			))}
		</div>
		<Pop delay={60} style={{ font: `800 58px ${display}`, letterSpacing: "-0.03em", textAlign: "center" }}>
			The same skill, copied three times.
			<br />
			<span style={{ color: c.pink }}>Which one is right?</span>
		</Pop>
	</Stage>
);

// Scene 2 (a): the answer. One hub, every agent connected to it.
const Idea = () => (
	<Stage gap={64}>
		<Pop delay={6}>
			<Box label="Your hub" sub="a Git repository you own" color={c.green} style={{ fontSize: 60, padding: "30px 60px" }} />
		</Pop>
		<div style={{ display: "flex", gap: 34 }}>
			{[
				["Claude Code", 1],
				["Codex", 0.55],
				["Gemini CLI", 0.55],
				["Another computer", 1],
			].map(([name, opacity], i) => (
				<Pop key={String(name)} delay={48 + i * 6} style={{ opacity: Number(opacity) }}>
					<Box label={String(name)} color={c.green} style={{ fontSize: 34, padding: "20px 28px" }} />
				</Pop>
			))}
		</div>
		<Pop delay={80} style={{ font: `600 28px ${display}`, color: c.muted }}>
			Verified end to end with Claude Code
		</Pop>
	</Stage>
);

// Scene 2 (c): what you get.
const Cards = () => (
	<Stage gap={34}>
		{["Your library, in Git", "Review before agents get it", "Nothing to install per project"].map((card, i) => (
			<Pop key={card} delay={6 + i * 27} style={{ font: `800 66px ${display}`, letterSpacing: "-0.03em" }}>
				<span style={{ color: c.green, textShadow: `5px 5px 0 ${c.ink}` }}>✓</span> {card}
			</Pop>
		))}
	</Stage>
);

// Frame t5: a command shown as a card, never filmed.
const CommandCard = ({ command, note }: { command: string; note?: string }) => (
	<Stage gap={40}>
		<Pop>
			<div style={{ ...sticker, padding: "34px 48px", font: `700 46px ${mono}`, color: c.cream }}>
				<span style={{ color: c.pink }}>$ </span>
				{command}
			</div>
		</Pop>
		{note ? (
			<Pop delay={14}>
				<div style={{ background: c.bg3, border: `3px solid rgba(240,236,226,.85)`, borderRadius: 22, padding: "16px 30px", font: `600 32px/1.3 ${display}`, maxWidth: 1400, textAlign: "center" }}>
					{note}
				</div>
			</Pop>
		) : null}
	</Stage>
);

// Frame t3: two computers, hub green and agent yellow, joined by a dotted line.
const TwoComputers = () => {
	const frame = useCurrentFrame();
	const line = interpolate(frame, [24, 60], [0, 1], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
	const Term = ({ name, color, lines }: { name: string; color: string; lines: string[] }) => (
		<div style={{ ...sticker, border: `4px solid ${color}`, width: 640, height: 400, fontFamily: mono, overflow: "hidden" }}>
			<div style={{ padding: "14px 22px", color, fontSize: 24, borderBottom: "3px solid rgba(240,236,226,.12)" }}>{name}</div>
			<div style={{ padding: "22px 28px", fontSize: 26, lineHeight: 1.6, color: c.muted, whiteSpace: "pre" }}>{lines.join("\n")}</div>
		</div>
	);
	return (
		<Stage gap={44}>
			<div style={{ display: "flex", alignItems: "center", gap: 0 }}>
				<Pop>
					<Term name="Hub computer" color={c.green} lines={["~/skillstation-hub", "skillstation serve"]} />
				</Pop>
				<div style={{ width: 160, height: 0, borderTop: `8px dotted ${c.green}`, opacity: line, transform: `scaleX(${line})` }} />
				<Pop delay={8}>
					<Term name="Agent computer" color={c.yellow} lines={["no skills, only a token"]} />
				</Pop>
			</div>
			<Pop delay={60} style={{ font: `800 46px ${display}`, letterSpacing: "-0.02em" }}>
				Same Wi-Fi or LAN, <span style={{ color: c.green }}>or Tailscale</span>
			</Pop>
		</Stage>
	);
};

const Outro = () => {
	const t = seconds();
	if (t < 7) {
		return (
			<Stage gap={44}>
				{[
					["skillstation status", "what to do next"],
					["skillstation doctor", "what is wrong, and one fix"],
				].map(([cmd, what], i) => (
					<Pop key={cmd} delay={12 + i * 48} style={{ display: "flex", gap: 30, alignItems: "baseline" }}>
						<span style={{ fontFamily: mono, fontWeight: 700, fontSize: 56, color: c.green }}>{cmd}</span>
						<span style={{ font: `800 46px ${display}` }}>{what}</span>
					</Pop>
				))}
			</Stage>
		);
	}
	if (t < 10.7) {
		return (
			<Stage>
				<Pop>
					<div style={{ ...sticker, border: `4px solid ${c.yellow}`, padding: "30px 48px", font: `700 50px ${mono}`, color: c.cream }}>
						<span style={{ color: c.yellow }}>&gt; </span>curate my Skillstation
					</div>
				</Pop>
				<Pop delay={12} style={{ font: `800 40px ${display}`, color: c.muted }}>
					In a new chat
				</Pop>
			</Stage>
		);
	}
	return (
		<Stage gap={30}>
			<Pop style={{ display: "flex", alignItems: "center", gap: 26 }}>
				<Mark size={110} />
				<span style={{ font: `800 100px ${display}`, letterSpacing: "-0.04em" }}>Skillstation</span>
			</Pop>
			<Pop delay={9} style={{ fontFamily: mono, fontWeight: 700, fontSize: 48 }}>
				github.com/vantt/skillstation
			</Pop>
			<Pop delay={18}>
				<Pill color={c.green} size={34}>
					Tell us what works, and what does not
				</Pill>
			</Pop>
		</Stage>
	);
};

// A shot that is not filmed yet: says what goes here, so the draft keeps its timing.
export const Pending = ({ what }: { what: string }) => (
	<Stage gap={24}>
		<div style={{ fontSize: 34, color: c.green, letterSpacing: 4, fontFamily: mono }}>SHOT TO BE RECORDED</div>
		<div style={{ font: `800 52px ${display}`, maxWidth: 1500, textAlign: "center" }}>{what}</div>
	</Stage>
);

export const Graphic = ({ name }: { name: string }) => {
	switch (name) {
		case "copies": return <Copies />;
		case "idea": return <Idea />;
		case "cards": return <Cards />;
		case "serve-web-card":
			return <CommandCard command="cd ~/skillstation-hub && skillstation serve web" note="Anyone with the link can change your skills. Keep it on this machine." />;
		case "install-service-card":
			return <CommandCard command="cd ~/skillstation-hub && skillstation serve --install-service" note="Linux and macOS. On Windows, run skillstation serve in its own terminal." />;
		case "two-computers": return <TwoComputers />;
		case "outro": return <Outro />;
		default: return <Pending what={`motion graphic "${name}"`} />;
	}
};
