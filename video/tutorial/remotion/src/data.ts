// The cut comes from two files written elsewhere: video/tutorial/edit.json (shots and voice placement, by hand) and
// video/tutorial/audio/timeline.json (real scene starts and lengths, by video/tutorial/scripts/voice.mjs).
import editJson from "../../edit.json";
import timelineJson from "../../audio/timeline.json";

export const FPS = 30;

export type Shot = {
	src?: string;
	from?: number;
	to?: number;
	rate?: number;
	mg?: string;
	pending?: string;
	seconds?: number;
	step?: string;
	machine?: "hub" | "agent";
	callouts?: { at: number; text: string }[];
};

export type ScenePlan = { shots: Shot[]; say: (number | null)[]; step?: string };
export type Cue = { scene: number; text: string; start: number; end: number };
export type TimelineScene = { scene: number; title: string; start: number; length: number; audio: string };

export const edit = editJson.scenes as unknown as Record<string, ScenePlan>;
export const timeline = timelineJson as unknown as { length: number; scenes: TimelineScene[]; cues: Cue[] };

export const shotSeconds = (shot: Shot) => shot.seconds ?? ((shot.to ?? 0) - (shot.from ?? 0)) / (shot.rate ?? 1);
export const frames = (seconds: number) => Math.round(seconds * FPS);

export { c, display, mono } from "./theme";
