import type { MeetingEvent, MeetingRules, MeetingState, Member, Numbered } from '../lake_meeting/events';

// What the dev server tells the forest lake meeting (forest_lake_meeting.md):
// the same events as the lake meeting's (lake_meeting/events.ts), with the
// forest lake's animals, as the Vite event 'forest-meeting:event', and the
// meeting as it stands at GET /__chat/forest-meeting. The dev server keeps
// both meetings from the one chat (meeting.ts, chat-bridge.ts). Shared by
// both sides, so it imports only types.

// The kinds a viewer's animal can be, the forest lake's own animals
// (figures/ForestLake/animals); the host is the explorer.
export const KINDS = ['wolf', 'deer', 'fox', 'boar', 'rabbit', 'squirrel', 'duck', 'otter', 'frog'] as const;
export type Kind = (typeof KINDS)[number];

// What a viewer can have their animal do: !jump, !walk, !run and !stop, as
// at the lake, and each kind's own tricks (TRICKS).
export const COMMANDS = ['jump', 'walk', 'run', 'stop', 'howl', 'flap', 'sit', 'look', 'eat', 'stand', 'wave'] as const;
export type Command = (typeof COMMANDS)[number];

// Every kind's commands, and the tricks of its own, from its sheet's poses:
// the wolf howls, the duck flaps its wings, the fox sits or looks up at the
// sky, the deer looks back, the boar and the rabbit eat, the otter stands
// up, and the explorer (the host) waves.
const EVERYONE: readonly Command[] = ['jump', 'walk', 'run', 'stop'];
export const TRICKS: Readonly<Record<string, readonly Command[]>> = {
  wolf: ['howl'],
  duck: ['flap'],
  fox: ['sit', 'look'],
  deer: ['look'],
  boar: ['eat'],
  rabbit: ['eat'],
  otter: ['stand'],
  explorer: ['wave'],
};

export const RULES: MeetingRules<Kind, Command> = {
  kinds: KINDS,
  commands: COMMANDS,
  host: 'explorer',
  can: (command, kind) => EVERYONE.includes(command) || (TRICKS[kind]?.includes(command) ?? false),
};

export type ForestMember = Member<Kind>;
export type ForestEvent = MeetingEvent<Kind, Command>;
export type ForestNumbered = Numbered<Kind, Command>;
export type ForestState = MeetingState<Kind>;
