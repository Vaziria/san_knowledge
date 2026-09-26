import { spawn, type ChildProcessByStdio } from 'node:child_process';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import { createInterface, type Interface } from 'node:readline';
import type { Readable } from 'node:stream';
import type { Connect, Logger, Plugin } from 'vite';
import { Meeting, type Chat, type SavedMeeting } from './meeting.ts';
import type { ChatStatus, Numbered, TestChat } from './src/story/lake_meeting/events.ts';

// The YouTube live chat for the lake meeting story (src/story/lake_meeting).
// Browsers can't read YouTube's chat themselves, so the dev server does, with
// san_youtube (golang/packages/san_youtube, its -json output), and keeps the
// meeting (meeting.ts): who is there, and what each message makes them do.
// Every page that draws the meeting gets the same events, as the Vite event
// 'meeting:event', and a page that opens (or reloads) first asks where the
// meeting stands. Only `npm run dev` has it, like the OSC and stream bridges.
//
// - GET /__chat/meeting: the meeting now (MeetingState).
// - GET /__chat/status: reading the chat (ChatStatus), also sent on every
//   change as the Vite event 'chat:status'.
// - POST /__chat/channel { channel }: reads that channel's live chat (an
//   @handle, a channel or a video), or stops with null. A channel that isn't
//   live yet is tried again every 30 s, so the Story tab can connect before
//   going live.
// - POST /__chat/say { name, text, as }: a made-up message (TestChat), to try
//   the meeting without YouTube.
//
// san_youtube is golang/packages/san_youtube/bin/san_youtube(.exe), or
// SAN_YOUTUBE. It reads chat the way the chat window on youtube.com does, so
// it needs no API key.
//
// A dev server restart (vite.config.ts or a file it imports changed, this one
// and meeting.ts among them) carries on: Vite makes a new plugin from the new
// code before the old one closes, and the old one hands it the meeting (into
// a Meeting of the new code) and the running san_youtube, so no comment is
// missed and the viewers' animals stay.

const RETRY_MS = 30_000; // after the stream wasn't live, ended or the network failed
const SLOW_RETRY_MS = 120_000; // after YouTube refused (rate limited, chat off, members only)
const TICK_MS = 5_000; // how often the meeting checks for quiet viewers
const POLL = '1s'; // how often san_youtube asks YouTube for new chat
const MAX_POST = 16 * 1024;

// What the plugin before a restart hands the new one. HANDOVER changes when
// its shape does; a plugin that doesn't know the shape refuses it.
const HANDOVER = 1;
interface Handover {
  version: number;
  meeting: SavedMeeting;
  reader: Reading;
  tests: number;
}

// The newest plugin's hand to take a handover, in this process.
const LATEST = Symbol.for('animation:chat-bridge');
const latest = globalThis as { [LATEST]?: (handover: Handover) => boolean };

export function chatBridge(): Plugin {
  let close: ((reason: 'restart' | 'close') => void) | null = null;
  return {
    name: 'chat-bridge',
    apply: 'serve',
    configureServer(server) {
      const { logger } = server.config;
      const emit = (seq: number, event: Numbered['event']) => server.hot.send('meeting:event', { seq, event } satisfies Numbered);
      let meeting = new Meeting(emit);
      const reader = new ChatReader(sanYoutube(server.config.root), logger, (chat) => meeting.chat(chat), (status) =>
        server.hot.send('chat:status', status),
      );
      const tick = setInterval(() => meeting.tick(), TICK_MS);
      let tests = 0;

      const adopt = (handover: Handover): boolean => {
        if (handover.version !== HANDOVER) return false;
        meeting = new Meeting(emit, Date.now, Math.random, handover.meeting);
        reader.adopt(handover.reader);
        tests = handover.tests;
        if (handover.reader.child) logger.info('chat: still reading, through the restart', { timestamp: true });
        return true;
      };
      latest[LATEST] = adopt;
      close = (reason) => {
        clearInterval(tick);
        const next = latest[LATEST];
        if (reason === 'restart' && next && next !== adopt) {
          const handover: Handover = { version: HANDOVER, meeting: meeting.save(), reader: reader.release(), tests };
          if (!next(handover)) handover.reader.child?.kill();
        } else {
          reader.read(null);
        }
      };

      server.middlewares.use('/__chat', (req, res) => {
        const answer = (status: number, body: unknown = '') => {
          res.statusCode = status;
          if (typeof body === 'string') {
            res.setHeader('Content-Type', 'text/plain; charset=utf-8');
            res.end(body && `${body}\n`);
          } else {
            res.setHeader('Content-Type', 'application/json');
            res.setHeader('Cache-Control', 'no-store');
            res.end(JSON.stringify(body));
          }
        };
        if (req.method === 'GET' && req.url === '/meeting') return answer(200, meeting.state());
        if (req.method === 'GET' && req.url === '/status') return answer(200, reader.status);
        if (req.method !== 'POST') return answer(404);
        void readJson(req).then((body) => {
          if (body === undefined) return answer(400, 'send JSON');
          if (req.url === '/channel') {
            const channel = (body as { channel?: unknown }).channel;
            if (channel !== null && (typeof channel !== 'string' || !channel.trim())) return answer(400, 'channel is an @handle, a channel or a video, or null to stop');
            reader.read(channel === null ? null : channel.trim());
            return answer(200, reader.status);
          }
          if (req.url === '/say') {
            const test = testChat(body, ++tests);
            if (typeof test === 'string') return answer(400, test);
            meeting.chat(test);
            return answer(204);
          }
          answer(404);
        });
      });

    },
    // A restart hands the meeting and the reader to the new plugin; a close
    // stops reading.
    closeServer({ reason }) {
      close?.(reason);
    },
  };
}

// Where san_youtube is: SAN_YOUTUBE, or the one built in this repository.
function sanYoutube(root: string): string {
  if (process.env.SAN_YOUTUBE) return process.env.SAN_YOUTUBE;
  return join(root, '..', '..', 'golang', 'packages', 'san_youtube', 'bin', process.platform === 'win32' ? 'san_youtube.exe' : 'san_youtube');
}

type SanYoutube = ChildProcessByStdio<null, Readable, Readable>;

// A reader's san_youtube and where it stands, for the reader that takes over
// after a restart: the process, its output read line by line, and when a
// reader that waits tries again.
interface Reading {
  status: ChatStatus;
  child: SanYoutube | null;
  lines: { out: Interface; err: Interface } | null;
  last: string;
  retryAt: number; // ms (Date.now), or 0
}

// Reads one channel's live chat with san_youtube, trying again while it
// isn't live, and says how it goes.
class ChatReader {
  status: ChatStatus = { channel: null, state: 'off', message: '', video: '' };
  private child: SanYoutube | null = null;
  private lines: Reading['lines'] = null;
  private retry: NodeJS.Timeout | null = null;
  private retryAt = 0;
  private last = ''; // san_youtube's last word on stderr
  private readonly program: string;
  private readonly logger: Logger;
  private readonly onChat: (chat: Chat) => void;
  private readonly onStatus: (status: ChatStatus) => void;

  constructor(program: string, logger: Logger, onChat: (chat: Chat) => void, onStatus: (status: ChatStatus) => void) {
    this.program = program;
    this.logger = logger;
    this.onChat = onChat;
    this.onStatus = onStatus;
  }

  read(channel: string | null): void {
    this.stopChild();
    if (channel === null) return this.set({ channel: null, state: 'off', message: '', video: '' });
    this.set({ channel, state: 'connecting', message: '', video: '' });
    this.start(channel);
  }

  private start(channel: string): void {
    if (!existsSync(this.program)) {
      return this.set({ state: 'error', message: `san_youtube is not at ${this.program}: build it (golang/packages/san_youtube/build.ps1) or set SAN_YOUTUBE.` });
    }
    this.last = '';
    // -new: only what is said from now on; -lang id: Super Chats as "Rp 20.000";
    // -poll 1s: a comment comes within about a second, where YouTube's own
    // pace is every 10 s or so (san_youtube slows to it for 10 minutes after
    // YouTube answers "too many requests").
    const child = spawn(this.program, ['-json', '-new', '-lang', 'id', '-poll', POLL, channel], { stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true });
    this.follow(child, { out: createInterface({ input: child.stdout }), err: createInterface({ input: child.stderr }) }, channel);
  }

  // Hands san_youtube over, still reading, to the reader after a restart;
  // this one then does nothing more.
  release(): Reading {
    if (this.retry) clearTimeout(this.retry);
    this.retry = null;
    const reading = { status: this.status, child: this.child, lines: this.lines, last: this.last, retryAt: this.retryAt };
    this.child = null;
    this.lines = null;
    return reading;
  }

  // Goes on from the reader before a restart.
  adopt({ status, child, lines, last, retryAt }: Reading): void {
    this.stopChild();
    this.last = last;
    this.set(status);
    const channel = status.channel;
    if (!channel) return;
    if (child && lines) {
      this.follow(child, lines, channel);
    } else if (retryAt) {
      this.later(channel, retryAt);
    }
  }

  // Takes what a san_youtube reading `channel` says: chat on stdout, how it
  // goes on stderr, and its end.
  private follow(child: SanYoutube, lines: NonNullable<Reading['lines']>, channel: string): void {
    this.child = child;
    this.lines = lines;
    lines.out.on('line', (line) => {
      if (this.child !== child) return; // handed over, or stopped
      const chat = fromSanYoutube(line);
      if (chat) this.onChat(chat);
    });
    lines.err.on('line', (line) => {
      if (this.child !== child) return;
      const reading = /^== reading the live chat of (\S+)/.exec(line);
      if (reading) {
        this.logger.info(`chat: reading ${reading[1]}`, { timestamp: true });
        this.set({ state: 'reading', message: '', video: reading[1] });
      } else if (line.startsWith('san_youtube:')) {
        this.last = line.replace(/^san_youtube:\s*/, '');
      }
    });
    child.on('error', (error) => {
      if (this.child === child) this.last = error.message;
    });
    child.on('close', () => {
      if (this.child !== child) return; // stopped on purpose, or handed over
      this.child = null;
      this.lines = null;
      const said = this.last || 'the chat reader stopped';
      // A channel or video that doesn't exist won't start to; the rest may.
      if (/channel not found|not a YouTube video or channel/.test(said)) return this.set({ state: 'error', message: said });
      const wait = /rate limited|chat is disabled|members only|video not found/.test(said) ? SLOW_RETRY_MS : RETRY_MS;
      this.logger.info(`chat: ${said}; trying again in ${wait / 1000} s`, { timestamp: true });
      this.set({ state: 'waiting', message: `${said}. Trying again in ${wait / 1000} s.`, video: '' });
      this.later(channel, Date.now() + wait);
    });
  }

  // Tries the channel again at `at` (Date.now ms).
  private later(channel: string, at: number): void {
    this.retryAt = at;
    this.retry = setTimeout(
      () => {
        this.retry = null;
        this.retryAt = 0;
        if (this.status.channel === channel) this.start(channel);
      },
      Math.max(0, at - Date.now()),
    );
  }

  private stopChild(): void {
    if (this.retry) clearTimeout(this.retry);
    this.retry = null;
    this.retryAt = 0;
    const child = this.child;
    this.child = null;
    this.lines = null;
    child?.kill();
  }

  private set(change: Partial<ChatStatus>): void {
    this.status = { ...this.status, ...change };
    this.onStatus(this.status);
  }
}

// One line of san_youtube -json, cut down to what the meeting uses; null for
// kinds it doesn't use (a gift received, a released held message).
function fromSanYoutube(line: string): Chat | null {
  let json: Record<string, unknown>;
  try {
    json = JSON.parse(line) as Record<string, unknown>;
  } catch {
    return null;
  }
  const author = (json.author ?? {}) as Record<string, unknown>;
  const amount = json.amount as Record<string, unknown> | undefined;
  const type = json.type;
  if (
    type !== 'text' &&
    type !== 'super_chat' &&
    type !== 'super_sticker' &&
    type !== 'membership' &&
    type !== 'gift_purchase' &&
    type !== 'deletion' &&
    type !== 'author_deletion'
  ) {
    return null;
  }
  return {
    type,
    id: String(json.id ?? ''),
    author: { name: String(author.name ?? ''), channel: String(author.channel_id ?? ''), owner: author.owner === true },
    text: String(json.text ?? ''),
    amount: amount && { value: Number(amount.value ?? 0), currency: String(amount.currency ?? ''), text: String(amount.text ?? '') },
    milestone: json.milestone === true,
    count: Number(json.count ?? 0),
    target: json.target_id === undefined ? undefined : String(json.target_id),
  };
}

// A test message from the Story tab, as the meeting takes chat, or what is
// wrong with it.
function testChat(body: unknown, n: number): Chat | string {
  const { name, text, as } = (body ?? {}) as Partial<TestChat>;
  if (typeof name !== 'string' || !name.trim()) return 'name is the viewer shown in the chat';
  if (typeof text !== 'string') return 'text is what they write';
  if (as !== 'viewer' && as !== 'owner' && as !== 'super_chat' && as !== 'member') return 'as is viewer, owner, super_chat or member';
  const who = name.trim();
  const author = { name: who, channel: `test:${who.toLowerCase()}`, owner: as === 'owner' };
  const id = `test-${n}`;
  if (as === 'super_chat') return { type: 'super_chat', id, author, text, amount: { value: 200000, currency: 'IDR', text: 'Rp 200.000' } };
  if (as === 'member') return { type: 'membership', id, author, text };
  return { type: 'text', id, author, text };
}

// A POSTed JSON body, or undefined when it isn't one.
function readJson(req: Connect.IncomingMessage): Promise<unknown> {
  return new Promise((resolve) => {
    const chunks: Buffer[] = [];
    let size = 0;
    req.on('data', (chunk: Buffer) => {
      size += chunk.length;
      if (size <= MAX_POST) chunks.push(chunk);
    });
    req.on('end', () => {
      if (size > MAX_POST) return resolve(undefined);
      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString('utf8')));
      } catch {
        resolve(undefined);
      }
    });
    req.on('error', () => resolve(undefined));
  });
}
