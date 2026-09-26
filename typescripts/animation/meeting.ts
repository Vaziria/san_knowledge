import { COMMANDS, HOST, KINDS, type Command, type Kind, type Member, type MeetingEvent, type MeetingState } from './src/story/lake_meeting/events.ts';

// Who is at the lake meeting (src/story/lake_meeting/lake_meeting.md), kept by
// the dev server so every page shows the same animals (chat-bridge.ts). It
// turns chat messages into what the pages show (events.ts):
//
// - A viewer's first message brings their animal, of a random kind; !cat,
//   !wolf and the other kinds switch it. Each message is a comment for it to
//   say, or a command (!jump, !walk, !run, !stop, !flap).
// - The channel owner's messages go to the host, the Bear.
// - At most LIMIT viewers' animals: a new one past that destroys the one that
//   joined first (FIFO). One whose viewer is quiet for QUIET_MS walks away.
// - Bots of random kinds fill the meeting up to BOTS animals. A viewer who
//   joins takes a bot's place, and a bot comes back when a viewer leaves.
// - A Super Chat, a Super Sticker, a new member or gifted memberships make
//   the fish leap, higher for a bigger Super Chat.
// - A deleted message is taken back; a viewer a moderator removes leaves at
//   once.
//
// It is plain logic: the time and the random numbers are passed in, so a
// test can run it without a server.

export const LIMIT = 12; // viewers' animals at once, not counting the host
export const BOTS = 7; // bots fill the meeting up to this many animals
export const QUIET_MS = 10 * 60 * 1000;
const LONGEST = 300; // characters of a comment; the rest is cut off

// A chat message, cut down from what san_youtube prints (-json), or made up
// for a test.
export interface Chat {
  type: 'text' | 'super_chat' | 'super_sticker' | 'membership' | 'gift_purchase' | 'deletion' | 'author_deletion';
  id: string; // the message's own id; a deletion's is empty
  author: { name: string; channel: string; owner: boolean };
  text: string;
  amount?: { value: number; currency: string; text: string };
  milestone?: boolean; // a membership: months, not a new member
  count?: number; // gifted memberships
  target?: string; // a deletion's message
}

export interface Seat extends Member {
  last: number; // ms: when the viewer last wrote
}

// All of a meeting, for a new Meeting to go on from: the dev server
// restarting after a change to this file carries the meeting over into the
// new code (chat-bridge.ts).
export interface SavedMeeting {
  seq: number;
  seats: Seat[];
  bots: number;
}

export class Meeting {
  private seq = 0;
  private seats: Seat[] = []; // in the order they joined, bots and viewers
  private bots = 0; // made so far, for their ids
  private readonly emit: (seq: number, event: MeetingEvent) => void;
  private readonly now: () => number;
  private readonly random: () => number;

  constructor(
    emit: (seq: number, event: MeetingEvent) => void,
    now: () => number = Date.now,
    random: () => number = Math.random,
    saved?: SavedMeeting,
  ) {
    this.emit = emit;
    this.now = now;
    this.random = random;
    if (saved) {
      this.seq = saved.seq;
      this.seats = saved.seats.map((seat) => ({ ...seat }));
      this.bots = saved.bots;
      this.fill(true); // this code may want more bots
    } else {
      this.fill(false); // the bots a page finds when it opens
    }
  }

  state(): MeetingState {
    return { seq: this.seq, members: this.seats.map(({ id, name, kind, bot }) => ({ id, name, kind, bot })) };
  }

  save(): SavedMeeting {
    return { seq: this.seq, seats: this.seats.map((seat) => ({ ...seat })), bots: this.bots };
  }

  chat(message: Chat): void {
    const { author } = message;
    switch (message.type) {
      case 'text':
        return this.text(message);
      case 'super_chat':
      case 'super_sticker': {
        const id = this.speaker(message, null);
        const what = message.amount?.text || (message.type === 'super_chat' ? 'Super Chat' : 'Super Sticker');
        this.send({ type: 'support', id, name: author.name, what, height: leapFor(message.amount) });
        if (message.type === 'super_chat' && message.text) this.say(id, author.name, message.text, message.id);
        return;
      }
      case 'membership': {
        const id = this.speaker(message, null);
        this.send({ type: 'support', id, name: author.name, what: message.milestone ? 'member milestone' : 'new member', height: message.milestone ? 0.45 : 0.6 });
        if (message.text) this.say(id, author.name, message.text, message.id);
        return;
      }
      case 'gift_purchase': {
        const id = this.speaker(message, null);
        this.send({ type: 'support', id, name: author.name, what: 'gifted memberships', height: 0.8 });
        return;
      }
      case 'deletion':
        if (message.target) this.send({ type: 'delete', message: message.target });
        return;
      case 'author_deletion':
        return this.remove(author.channel);
    }
  }

  // Moves the meeting on: viewers quiet for too long walk away, and bots
  // come back in their place. Call it now and then (every few seconds).
  tick(): void {
    const now = this.now();
    for (const seat of [...this.seats]) {
      if (!seat.bot && now - seat.last > QUIET_MS) this.leave(seat, 'walk');
    }
    this.fill(true);
  }

  private text(message: Chat): void {
    const words = message.text.trim();
    const [first = '', ...rest] = words.split(/\s+/);
    const word = first.startsWith('!') ? first.slice(1).toLowerCase() : '';
    const kind = (KINDS as readonly string[]).includes(word) ? (word as Kind) : null;
    const command = (COMMANDS as readonly string[]).includes(word) ? (word as Command) : null;
    const id = this.speaker(message, kind);
    if (kind) {
      if (id !== HOST) this.switch(id, kind);
      if (rest.length) this.say(id, message.author.name, rest.join(' '), message.id);
      return;
    }
    if (command) {
      const seat = this.seats.find((s) => s.id === id);
      const kindNow = id === HOST ? 'bear' : seat?.kind;
      if (command !== 'flap' || kindNow === 'penguin') this.send({ type: 'command', id, command });
      if (rest.length) this.say(id, message.author.name, rest.join(' '), message.id);
      return;
    }
    this.say(id, message.author.name, words, message.id);
  }

  // Who says a message: the host for the channel owner, otherwise the
  // viewer's animal, which joins first if it isn't there (as `kind`, or a
  // random one). Its viewer is no longer quiet.
  private speaker(message: Chat, kind: Kind | null): string {
    const { author } = message;
    if (author.owner) return HOST;
    const id = author.channel || `name:${author.name}`;
    const seat = this.seats.find((s) => s.id === id);
    if (seat) {
      seat.last = this.now();
      seat.name = author.name || seat.name;
      return id;
    }
    this.join({ id, name: author.name, kind: kind ?? this.randomKind(), bot: false });
    return id;
  }

  // A viewer's animal joins: it takes a bot's place, or the oldest viewer's
  // when the meeting is full.
  private join(member: Member): void {
    const bot = this.seats.find((s) => s.bot);
    if (bot) this.leave(bot, 'walk');
    const viewers = this.seats.filter((s) => !s.bot);
    if (viewers.length >= LIMIT) this.leave(viewers[0], 'destroy');
    this.seats.push({ ...member, last: this.now() });
    this.send({ type: 'join', member });
  }

  private switch(id: string, kind: Kind): void {
    const seat = this.seats.find((s) => s.id === id);
    if (!seat || seat.kind === kind) return;
    seat.kind = kind;
    this.send({ type: 'switch', id, kind });
  }

  private say(id: string, name: string, text: string, message: string): void {
    const clean = text.replace(/\s+/g, ' ').trim().slice(0, LONGEST);
    if (clean) this.send({ type: 'say', id, name, text: clean, message });
  }

  private leave(seat: Seat, how: 'walk' | 'destroy' | 'remove'): void {
    this.seats = this.seats.filter((s) => s !== seat);
    this.send({ type: 'leave', id: seat.id, how });
  }

  // A viewer a moderator removed: gone at once, with what they wrote.
  private remove(channel: string): void {
    const seat = this.seats.find((s) => s.id === channel && !s.bot);
    if (!seat) return;
    this.leave(seat, 'remove');
    this.fill(true);
  }

  // Bots of random kinds, up to BOTS animals; they walk in, except the ones
  // there from the start.
  private fill(walk: boolean): void {
    while (this.seats.length < BOTS) {
      const member: Member = { id: `bot-${++this.bots}`, name: '', kind: this.randomKind(), bot: true };
      this.seats.push({ ...member, last: this.now() });
      if (walk) this.send({ type: 'join', member });
    }
  }

  private randomKind(): Kind {
    return KINDS[Math.min(KINDS.length - 1, Math.floor(this.random() * KINDS.length))];
  }

  private send(event: MeetingEvent): void {
    this.emit(++this.seq, event);
  }
}

// How high the fish leaps for a Super Chat or a Super Sticker: by YouTube's
// tiers in US dollars, from 30 cm for the smallest to 1.5 m for 100 and
// more. The rates only pick a tier, so rough ones do; an unknown currency
// counts as dollars.
const PER_DOLLAR: Record<string, number> = {
  USD: 1,
  IDR: 16500,
  EUR: 0.92,
  GBP: 0.78,
  JPY: 150,
  KRW: 1380,
  INR: 84,
  MYR: 4.4,
  SGD: 1.34,
  PHP: 57,
  THB: 34,
  VND: 25000,
  TWD: 32,
  HKD: 7.8,
  AUD: 1.52,
  CAD: 1.37,
  BRL: 5.6,
  MXN: 18.5,
};
const TIERS: [dollars: number, height: number][] = [
  [100, 1.5],
  [50, 1.25],
  [20, 1],
  [10, 0.8],
  [5, 0.6],
  [2, 0.45],
  [0, 0.3],
];

export function leapFor(amount: Chat['amount']): number {
  if (!amount || !(amount.value > 0)) return TIERS[TIERS.length - 1][1];
  const dollars = amount.value / (PER_DOLLAR[amount.currency] ?? 1);
  return TIERS.find(([least]) => dollars >= least)![1];
}
