import type { CSSProperties, ReactNode } from "react";
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from "remotion";
import { loadFont as loadDisplay } from "@remotion/google-fonts/BricolageGrotesque";
import { loadFont as loadMono } from "@remotion/google-fonts/JetBrainsMono";

// P3 look: video/style/frames.html. Green = one source, latest; pink = an old copy.
export const c = {
	bg: "#23175a",
	bg3: "#3d2c94",
	green: "#9ff07f",
	brand: "#84b577",
	pink: "#ff6fc8",
	yellow: "#ffd166",
	cream: "#f0ece2",
	muted: "#b6addf",
	ink: "#140c38",
	shadow: "rgba(10,5,30,.65)",
};

export const display = loadDisplay("normal", { weights: ["700", "800"] }).fontFamily;
export const mono = loadMono("normal", { weights: ["400", "700"] }).fontFamily;

export const Background = ({ children }: { children: ReactNode }) => (
	<AbsoluteFill style={{ background: c.bg, color: c.cream, fontFamily: display }}>
		<AbsoluteFill
			style={{
				backgroundImage: "radial-gradient(rgba(240,236,226,.10) 1.6px, transparent 1.8px)",
				backgroundSize: "36px 36px",
			}}
		/>
		{children}
	</AbsoluteFill>
);

/** 0 to 1 with a small overshoot: elements pop in like stickers. */
export const usePop = (delay = 0) => {
	const frame = useCurrentFrame();
	const { fps } = useVideoConfig();
	return spring({ frame: frame - delay, fps, config: { damping: 12, stiffness: 170 } });
};

export const Pop = ({ delay = 0, children, style }: { delay?: number; children: ReactNode; style?: CSSProperties }) => {
	const p = usePop(delay);
	const o = interpolate(p, [0, 0.35], [0, 1], { extrapolateRight: "clamp" });
	return <div style={{ opacity: o, transform: `scale(${0.6 + 0.4 * p})`, ...style }}>{children}</div>;
};

export const sticker: CSSProperties = {
	background: c.ink,
	border: `4px solid ${c.cream}`,
	borderRadius: 26,
	boxShadow: `12px 12px 0 ${c.shadow}`,
};

export const Pill = ({ color, children, size = 34 }: { color: string; children: ReactNode; size?: number }) => (
	<span
		style={{
			display: "inline-block",
			padding: "2px 18px",
			borderRadius: 999,
			background: color,
			color: c.ink,
			fontFamily: mono,
			fontWeight: 700,
			fontSize: size,
		}}
	>
		{children}
	</span>
);

export const Tile = ({ repo, version, color }: { repo: string; version: string; color: string }) => (
	<div style={{ ...sticker, borderRadius: 20, border: `3px solid ${c.cream}`, boxShadow: `8px 8px 0 ${c.shadow}`, padding: "20px 24px", fontFamily: mono, fontSize: 28 }}>
		<div style={{ color: c.muted, fontSize: 24 }}>{repo}</div>
		<div style={{ marginTop: 6 }}>commit-style/</div>
		<div style={{ marginTop: 12 }}>
			<Pill color={color} size={26}>
				{version}
			</Pill>
		</div>
	</div>
);

export const Mark = ({ size }: { size: number }) => (
	<svg width={size} height={size} viewBox="0 0 32 32">
		<path fill="#84b577" d="M16 1L31 16L16 31L1 16Z" />
		<path fill="#b3d6a9" d="M16 1L16 13L1 16Z" />
		<path fill="#9bc78f" d="M16 1L31 16L16 13Z" />
		<path fill="#84b577" d="M1 16L16 13L16 31Z" />
		<path fill="#6f9f62" d="M16 13L31 16L16 31Z" />
	</svg>
);

export const headline = (size: number): CSSProperties => ({
	fontWeight: 800,
	fontSize: size,
	letterSpacing: "-0.03em",
	lineHeight: 0.95,
});
