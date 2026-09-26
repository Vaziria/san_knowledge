// What the dev server tells the lake meeting (lake_meeting.md): the Vite
// event 'meeting:event', each with a sequence number, and the meeting as it
// is when a page starts (GET /__chat/meeting). The dev server keeps the
// meeting (meeting.ts, chat-bridge.ts), so every page that draws it (the
// panel's page, the stream's hidden browser, one reloaded after a code
// change) shows the same animals. Shared by both sides, so it imports
// nothing.

// The kinds a viewer's animal can be; the Bear is the host's.
export const KINDS = ['cat', 'wolf', 'deer', 'bird', 'fox', 'snake', 'penguin', 'frog', 'lion'] as const;
export type Kind = (typeof KINDS)[number];

// What a viewer can have their animal do: !jump, !walk, !run, !stop, and
// !flap for a penguin.
export const COMMANDS = ['jump', 'walk', 'run', 'stop', 'flap'] as const;
export type Command = (typeof COMMANDS)[number];

export const HOST = 'host'; // the Bear, the channel owner's

export interface Member {
  id: string; // the viewer's channel, or bot-<n>
  name: string; // as the chat shows it; empty for a bot
  kind: Kind;
  bot: boolean;
}

export type MeetingEvent =
  // An animal walks in: a viewer's first message, or a bot filling in.
  | { type: 'join'; member: Member }
  // It walks away (quiet for 10 minutes, or a bot making room), is destroyed
  // (the oldest, when the meeting is full), or is removed by a moderator.
  | { type: 'leave'; id: string; how: 'walk' | 'destroy' | 'remove' }
  // A viewer picked another kind (!fox).
  | { type: 'switch'; id: string; kind: Kind }
  // A comment, for its animal to say. `message` is its chat id, for deleting.
  | { type: 'say'; id: string; name: string; text: string; message: string }
  | { type: 'command'; id: string; command: Command }
  // A Super Chat, a Super Sticker, a new member or gifted memberships: the
  // fish leaps `height` m out of the water. `what` says what it was for.
  | { type: 'support'; id: string; name: string; what: string; height: number }
  // A message was deleted: out of its bubble, or its turn.
  | { type: 'delete'; message: string };

export interface Numbered {
  seq: number;
  event: MeetingEvent;
}

// The meeting now: every animal but the host, in the order they joined, and
// the number of the last event, so a page applies only later ones.
export interface MeetingState {
  seq: number;
  members: Member[];
}

// Where reading the chat stands, for the Story tab.
export interface ChatStatus {
  channel: string | null; // what was asked for: @handle, a channel or a video
  state: 'off' | 'connecting' | 'reading' | 'waiting' | 'error';
  message: string; // what went wrong, or what it waits for
  video: string; // the live stream it reads, once found
}

// A made-up chat message, from the Story tab, for trying the meeting without
// YouTube (POST /__chat/say).
export interface TestChat {
  name: string;
  text: string;
  as: 'viewer' | 'owner' | 'super_chat' | 'member';
}
