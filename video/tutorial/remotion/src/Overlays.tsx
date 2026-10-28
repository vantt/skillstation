import { AbsoluteFill, interpolate, useCurrentFrame, useVideoConfig } from "remotion";
import { c, display, frames, mono, timeline } from "./data";
import { Pop, usePop } from "./theme";

// Layout of frames t1-t5 (video/style/frames.html): a 16:9 recording window on the left, a column for result cards on
// the right, the step title above, subtitles and the chapter bar below.
export const WINDOW = { left: 90, top: 150, width: 1307, height: 735 };
export const COLUMN = { left: 1440, width: 390 };

export const machineColor = (machine?: "hub" | "agent") => (machine === "hub" ? c.green : machine === "agent" ? c.yellow : c.cream);

// "Step 2 · Create your hub" becomes a STEP 2 pill and a title; a title without a step number stands alone.
export const StepTitle = ({ text }: { text: string }) => {
	const m = text.match(/^Step (\d+) · (.+)$/);
	const p = usePop();
	return (
		<div style={{ position: "absolute", left: 90, top: 56, display: "flex", alignItems: "center", gap: 22, opacity: Math.min(1, p * 2.5), transform: `translateX(${(1 - p) * -40}px)` }}>
			{m ? (
				<span
					style={{
						background: c.green, color: c.ink, border: `4px solid ${c.cream}`, borderRadius: 999, boxShadow: `6px 6px 0 ${c.shadow}`,
						padding: "6px 26px", font: `800 34px ${display}`, letterSpacing: "0.02em",
					}}
				>
					STEP {m[1]}
				</span>
			) : null}
			<span style={{ font: `800 64px ${display}`, letterSpacing: "-0.03em", textShadow: `5px 5px 0 ${c.ink}`, color: c.cream }}>{m ? m[2] : text}</span>
		</div>
	);
};

// Which computer a recording belongs to: hub green, agent yellow.
export const MachineLabel = ({ machine }: { machine: "hub" | "agent" }) => (
	<Pop style={{ position: "absolute", right: 90, top: 62, transformOrigin: "100% 50%" }}>
		<div
			style={{
				padding: "8px 26px", borderRadius: 999, border: `4px solid ${c.cream}`, background: machineColor(machine), color: c.ink,
				font: `800 30px ${display}`, boxShadow: `6px 6px 0 ${c.shadow}`,
			}}
		>
			{machine === "hub" ? "Hub computer" : "Agent computer"}
		</div>
	</Pop>
);

// A recording in a sticker window; its frame takes the colour of the computer it was filmed on.
export const Window = ({ machine, children }: { machine?: "hub" | "agent"; children: React.ReactNode }) => (
	<div
		style={{
			position: "absolute", ...WINDOW, overflow: "hidden", borderRadius: 22, background: c.ink, boxSizing: "content-box",
			border: `4px solid ${machineColor(machine)}`, boxShadow: `12px 12px 0 ${c.shadow}`, marginLeft: -4, marginTop: -4,
		}}
	>
		{children}
	</div>
);

// A result card that pops in beside the window and leaves after a few seconds.
export const Callout = ({ text, seconds }: { text: string; seconds: number }) => {
	const frame = useCurrentFrame();
	const pop = usePop();
	const out = interpolate(frame, [frames(seconds) - 8, frames(seconds)], [1, 0], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
	return (
		<div
			style={{
				position: "absolute", left: COLUMN.left, width: COLUMN.width, top: WINDOW.top + 250, boxSizing: "border-box",
				padding: "16px 24px", borderRadius: 22, background: c.green, color: c.ink, border: `4px solid ${c.cream}`,
				boxShadow: `8px 8px 0 ${c.shadow}`, font: `800 38px/1.1 ${display}`, letterSpacing: "-0.01em",
				opacity: Math.min(1, pop * 2.5) * out, transform: `rotate(-2deg) scale(${0.6 + 0.4 * pop})`,
			}}
		>
			✓ {text}
		</div>
	);
};

// The narration, burned in above the chapter bar. Times come from timeline.json, relative to the whole video.
export const Subtitles = () => {
	const frame = useCurrentFrame();
	const cue = timeline.cues.find((cu) => frame >= frames(cu.start) && frame < frames(cu.end));
	if (!cue) return null;
	return (
		<AbsoluteFill style={{ justifyContent: "flex-end", alignItems: "center", paddingBottom: 118 }}>
			<span
				style={{
					display: "inline-block", maxWidth: 1600, background: "rgba(20,12,56,.88)", borderRadius: 14, padding: "8px 26px",
					font: `600 38px/1.25 ${display}`, color: c.cream, textAlign: "center",
				}}
			>
				{cue.text}
			</span>
		</AbsoluteFill>
	);
};

export const CHAPTERS = [
	{ label: "Install", scene: 3 },
	{ label: "Create hub", scene: 4 },
	{ label: "Connect", scene: 5 },
	{ label: "Ask agent", scene: 6 },
	{ label: "First skill", scene: 7 },
	{ label: "Web editor", scene: 8 },
	{ label: "Other computer", scene: 9 },
	{ label: "Tips", scene: 10 },
];

// Eight cells along the bottom: done, current, to come. Hidden during the opening scenes.
export const Progress = () => {
	const { fps } = useVideoConfig();
	const frame = useCurrentFrame();
	const t = frame / fps;
	const current = timeline.scenes.filter((s) => s.start <= t).map((s) => s.scene).pop() ?? 0;
	const now = CHAPTERS.findIndex((ch) => ch.scene === current);
	if (now < 0) return null;
	return (
		<div style={{ position: "absolute", left: 90, right: 90, bottom: 44, display: "flex", gap: 12 }}>
			{CHAPTERS.map((ch, i) => {
				const state = i < now ? "done" : i === now ? "now" : "next";
				return (
					<div
						key={ch.label}
						style={{
							flex: 1, textAlign: "center", font: `700 20px ${mono}`, padding: "10px 6px", borderRadius: 14, boxSizing: "border-box",
							border: `3px solid ${state === "now" ? c.cream : state === "done" ? "rgba(240,236,226,.6)" : "rgba(240,236,226,.3)"}`,
							background: state === "now" ? c.green : state === "done" ? c.bg3 : "transparent",
							color: state === "now" ? c.ink : state === "done" ? c.cream : c.muted,
							boxShadow: state === "now" ? `5px 5px 0 ${c.shadow}` : undefined,
						}}
					>
						{ch.label}
					</div>
				);
			})}
		</div>
	);
};
