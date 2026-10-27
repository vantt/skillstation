import { Composition } from "remotion";
import { Intro, SHOT_FRAMES } from "./Intro";
import { IntroWide } from "./IntroWide";

export const Root = () => (
	<>
		<Composition id="Intro" component={Intro} width={1080} height={1920} fps={30} durationInFrames={SHOT_FRAMES.total} />
		<Composition id="IntroWide" component={IntroWide} width={1920} height={1080} fps={30} durationInFrames={SHOT_FRAMES.total} />
		<Composition id="IntroWideSound" component={IntroWide} defaultProps={{ withSound: true }} width={1920} height={1080} fps={30} durationInFrames={SHOT_FRAMES.total} />
	</>
);
