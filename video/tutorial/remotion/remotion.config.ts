import { Config } from "@remotion/cli/config";

// public/ links to video/recordings, video/tutorial/audio and docs/images/brand, so the cut reads them in place.
Config.setVideoImageFormat("jpeg");
