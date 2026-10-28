import { Composition } from "remotion";
import { Intro, introSchema } from "./Intro";
import { FPS, timeline } from "./data";

export const Root = () => (
	<Composition
		id="Intro"
		component={Intro}
		durationInFrames={Math.ceil(timeline.length * FPS)}
		fps={FPS}
		width={1920}
		height={1080}
		defaultProps={introSchema}
	/>
);
