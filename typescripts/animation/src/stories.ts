import type { Environment, Preview } from './previews';
import { pianoPlay } from './story/piano_play/piano_play';
import type { Theme } from './theme';

// A story plays a scene in its own environment: its figures, moved by a
// script or by outside input such as OSC. The panel's Story menu or
// ?story=<name> picks one; while it plays, the Figure and Environment menus
// rest. Each story lives in src/story/<name>/, next to its spec, <name>.md.
export interface Story {
  environment: string; // a key of environments
  create: (theme: Theme, environment: Environment) => Preview;
}

export const stories: Record<string, Story> = {
  piano_play: pianoPlay,
};
