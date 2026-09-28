import type { Environment, Preview, Season } from './previews';
import { forestLakeMeeting } from './story/forest_lake_meeting/forest_lake_meeting';
import { lakeMeeting } from './story/lake_meeting/lake_meeting';
import { pianoPlay } from './story/piano_play/piano_play';
import type { Theme } from './theme';

// A story plays a scene in its own environment: its figures, moved by a
// script or by outside input such as OSC or a live chat. The panel's Story
// menu or ?story=<name> picks one; while it plays, the Figure and Environment
// menus rest. Each story lives in src/story/<name>/, next to its spec,
// <name>.md. It builds its environment itself (one from the environments
// table, or one made for it, such as the lake with a wider clearing). One
// that plays in an environment with seasons (the forest lake meeting) gives
// it in each season, and the Season setting picks one.
export interface Story {
  environment: (theme: Theme) => Environment;
  // Its environment in a season: the same function for the same season
  // every time, so the stage builds it again only when the season changes.
  environmentIn?(season: Season): (theme: Theme) => Environment;
  create: (theme: Theme, environment: Environment) => Preview;
}

export const stories: Record<string, Story> = {
  piano_play: pianoPlay,
  lake_meeting: lakeMeeting,
  forest_lake_meeting: forestLakeMeeting,
};
