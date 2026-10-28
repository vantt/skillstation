import { Audio, Video } from "@remotion/media";
import { AbsoluteFill, Freeze, interpolate, Sequence, staticFile, useCurrentFrame } from "remotion";
import { Graphic, Pending } from "./Graphics";
import { Callout, MachineLabel, Progress, StepTitle, Subtitles, Window } from "./Overlays";
import { Background, c } from "./theme";
import { edit, frames, type Shot, shotSeconds, timeline } from "./data";

export const introSchema = { burnSubtitles: true };

// Background music: Holizna, "City In The Rearview", CC0, from https://opengameart.org/content/chill-beats-collection.
// The file is the track joined to itself with a 4 s crossfade (video/tutorial/README.md), so it outlasts the film.
const MUSIC = "audio/music/city-in-the-rearview.ogg";
const MUSIC_VOLUME = 0.14;
const MUSIC_UNDER_VOICE = 0.05;

// Lower the music while the narration speaks, with a short ramp, and fade it out at the very end.
const musicVolume = (frame: number) => {
	const speaking = timeline.cues.some((c) => frame >= frames(c.start) - 6 && frame < frames(c.end) + 6);
	const end = frames(timeline.length);
	const fade = interpolate(frame, [end - 60, end], [1, 0], { extrapolateLeft: "clamp", extrapolateRight: "clamp" });
	return (speaking ? MUSIC_UNDER_VOICE : MUSIC_VOLUME) * fade;
};

const ClipVideo = ({ shot }: { shot: Shot }) => (
	<Video
		src={staticFile(shot.src!)}
		trimBefore={frames(shot.from ?? 0)}
		playbackRate={shot.rate ?? 1}
		muted
		style={{ width: "100%", height: "100%", objectFit: "contain" }}
	/>
);

// A recording in a sticker window on the dotted P3 background, with its result cards in the right column.
const Clip = ({ shot, length, hold }: { shot: Shot; length: number; hold: number }) => {
	const own = length - hold;
	return (
		<>
			<Background>{null}</Background>
			<Window machine={shot.machine}>
				<Sequence durationInFrames={own} premountFor={30}>
					<ClipVideo shot={shot} />
				</Sequence>
				{hold > 0 ? (
					// The voice runs longer than the clip: keep its last frame on screen.
					<Sequence from={own} durationInFrames={hold}>
						<Freeze frame={own - 1}>
							<ClipVideo shot={shot} />
						</Freeze>
					</Sequence>
				) : null}
			</Window>
			{(shot.callouts ?? []).map((co, i, all) => {
				const next = all[i + 1]?.at ?? co.at + 3;
				const seconds = Math.min(3, next - co.at);
				return (
					<Sequence key={i} from={frames(co.at)} durationInFrames={frames(seconds)}>
						<Callout text={co.text} seconds={seconds} />
					</Sequence>
				);
			})}
		</>
	);
};

const ShotView = ({ shot, length, hold }: { shot: Shot; length: number; hold: number }) => {
	if (shot.src) return <Clip shot={shot} length={length} hold={hold} />;
	if (shot.pending) return <Pending what={shot.pending} />;
	return <Graphic name={shot.mg ?? "?"} />;
};

const Scene = ({ scene }: { scene: number }) => {
	const frame = useCurrentFrame();
	const plan = edit[scene];
	const t = timeline.scenes.find((s) => s.scene === scene)!;
	const total = frames(t.length);
	const fadeIn = interpolate(frame, [0, 10], [0, 1], { extrapolateRight: "clamp" });
	let at = 0;
	return (
		<AbsoluteFill style={{ background: c.bg, opacity: fadeIn }}>
			{plan.shots.map((shot, i) => {
				const from = at;
				const own = frames(shotSeconds(shot));
				const last = i === plan.shots.length - 1;
				const length = last ? total - from : own;
				at += own;
				const step = shot.step ?? (i === 0 ? plan.step : undefined);
				return (
					<Sequence key={i} from={from} durationInFrames={length} premountFor={30}>
						<ShotView shot={shot} length={length} hold={last ? length - own : 0} />
						{step ? <StepTitle text={step} /> : null}
						{shot.machine ? <MachineLabel machine={shot.machine} /> : null}
					</Sequence>
				);
			})}
			<Audio src={staticFile(t.audio)} />
		</AbsoluteFill>
	);
};

export const Intro = ({ burnSubtitles }: typeof introSchema) => (
	<AbsoluteFill style={{ background: c.bg }}>
		{timeline.scenes.map((s) => (
			<Sequence key={s.scene} from={frames(s.start)} durationInFrames={frames(s.length)} premountFor={30}>
				<Scene scene={s.scene} />
			</Sequence>
		))}
		<Audio src={staticFile(MUSIC)} loop volume={musicVolume} />
		{burnSubtitles ? <Subtitles /> : null}
		<Progress />
	</AbsoluteFill>
);
