import type { Environment, Preview } from './previews';
import { lakeMeeting } from './story/lake_meeting/lake_meeting';
import { pianoPlay } from './story/piano_play/piano_play';
import type { Theme } from './theme';

// A story plays a scene in its own environment: its figures, moved by a
// script or by outside input such as OSC or a live chat. The panel's Story
// menu or ?story=<name> picks one; while it plays, the Figure and Environment
// menus rest. Each story lives in src/story/<name>/, next to its spec,
// <name>.md. It builds its environment itself (one from the environments
// table, or one made for it, such as the lake with a wider clearing).
export interface Story {
  environment: (theme: Theme) => Environment;
  create: (theme: Theme, environment: Environment) => Preview;
}

export const stories: Record<string, Story> = {
  piano_play: pianoPlay,
  lake_meeting: lakeMeeting,
};
