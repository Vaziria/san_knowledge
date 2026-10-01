import { forestLakeBuild, forestLakeEnvironment, SEASONS, type Environment, type Season } from '../../previews';
import type { Stepwise } from '../../stepwise';
import type { Story } from '../../stories';
import type { Theme } from '../../theme';
import { ForestMeeting } from './ForestMeeting';

// Forest lake meeting (forest_lake_meeting.md): the lake meeting at the
// forest lake, as the user asked ("make story ForestLake Metting like lake
// meeting"). The viewers of the YouTube live stream meet there as the forest
// lake's own animals; the dev server reads the chat and keeps who is there,
// as for the lake meeting and from the same chat (chat-bridge.ts,
// meeting.ts); the Story tab picks the channel and can send made-up
// messages.
//
// - The host is the explorer, who starts on the landing and roams like the
//   others; the channel owner's messages and commands go to him (!wave).
// - A viewer's first message brings their animal, of a random kind: a wolf,
//   deer, fox, boar, rabbit, squirrel, duck, otter or frog; !wolf and the
//   other kinds switch it. Its messages are its speech bubbles, each headed
//   by the viewer's name, and !jump, !walk, !run and !stop are commands, as
//   at the lake, with each kind's own tricks (!howl, !flap, !sit, !look,
//   !eat, !stand).
// - At most 12 viewers' animals, the oldest destroyed past that, one quiet
//   for 10 minutes walking away; bots keep the meeting at seven or more.
// - The trout leaps out of the water for a Super Chat, a Super Sticker or a
//   new member, higher for a bigger Super Chat, and now and then of its own
//   accord.
// - The camera follows each comment and command in turn; when the chat is
//   quiet it goes back to the host, and after 30 s it follows one animal
//   after another (ForestMeeting.ts).
//
// It starts at the forest lake in the season the Season setting picks (the
// Environments tab, ?season=), spring by default, and the season changes
// every minute of the meeting's own time (scenarios/season_change.md):
// spring, summer, autumn, winter, then spring again. The meeting carries on
// through the change: the stage builds the next season's forest lake ahead,
// a piece at a time while it plays, and swaps it in under the animals
// (Preview.changes, Stage.ts), which stay where they are, in the new
// season's coats where their sheets have one. A season picked while it
// plays comes the same way, at once, and the next minute counts from there.
const seasons = Object.fromEntries(SEASONS.map((season) => [season, (_theme: Theme) => forestLakeEnvironment(season)])) as Record<Season, (theme: Theme) => Environment>;
const ahead = Object.fromEntries(SEASONS.map((season) => [season, (_theme: Theme) => forestLakeBuild(season)])) as Record<Season, (theme: Theme) => Stepwise<Environment>>;

export const forestLakeMeeting: Story = {
  environment: seasons.spring,
  environmentIn: (season) => seasons[season],
  create: (theme, environment) => {
    const meeting = new ForestMeeting(theme, environment);
    return {
      figure: meeting,
      camera: ForestMeeting.WIDE.camera.toArray(),
      target: ForestMeeting.WIDE.target.toArray(),
      bounds: meeting.bounds,
      followShadow: true,
      update: (_elapsed, delta) => meeting.update(delta),
      dispose: () => meeting.dispose(),
      shot: () => meeting.shot(),
      changes: {
        next: () => ahead[meeting.nextSeason],
        prepare: (environment) => meeting.prepare(environment),
        due: () => meeting.seasonDue,
        pick: (create) => {
          const season = SEASONS.find((s) => seasons[s] === create);
          if (season) meeting.pick(season);
          return season !== undefined;
        },
      },
    };
  },
};
