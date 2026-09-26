import { spawn, type ChildProcessByStdio } from 'node:child_process';
import { createReadStream, createWriteStream, existsSync, mkdirSync, readdirSync, statSync } from 'node:fs';
import { rename, rm } from 'node:fs/promises';
import type { ServerResponse } from 'node:http';
import { basename, extname, join } from 'node:path';
import type { Readable, Writable } from 'node:stream';
import type { Connect, Logger, Plugin } from 'vite';

// The scene's sound, for the panel's Audio tab. The sources are the files in
// the audio folder (typescripts/animation/audio, git-ignored, or AUDIO_DIR),
// played one or all in turn, and the input devices of this machine (a
// microphone; Sonic Pi through Stereo Mix or a virtual cable such as
// VB-CABLE). This plugin lists and serves them, and takes new files:
//
// - GET /__audio/files: JSON { files: [{ name, size }] }, by name.
// - GET /__audio/files/<name>: the file, with ranges, for the page to play.
// - POST /__audio/files?name=<name>: adds a file (its bytes), answered 201;
//   400 for a name that isn't a plain audio file's, 409 when it is taken.
// - GET /__audio/devices: JSON { devices: [names] }, as ffmpeg sees them.
//
// While a stream runs, AudioFeed (below) makes its sound: stream-bridge.ts
// gives ffmpeg's audio input to it, and the panel can change the source, the
// loop and the volume without the stream stopping.

export type AudioSource =
  | { kind: 'none' }
  | { kind: 'file'; name: string } // one file from the audio folder
  | { kind: 'files' } // every file there, by name, in turn
  | { kind: 'device'; name: string }; // an input device of this machine

export interface AudioChoice {
  source: AudioSource;
  loop: boolean; // the file, or the list, starts again at its end
  volume: number; // 0..1
}

export const SILENCE: AudioChoice = { source: { kind: 'none' }, loop: true, volume: 1 };

const EXTENSIONS: Record<string, string> = {
  '.mp3': 'audio/mpeg',
  '.m4a': 'audio/mp4',
  '.aac': 'audio/aac',
  '.ogg': 'audio/ogg',
  '.oga': 'audio/ogg',
  '.opus': 'audio/ogg',
  '.wav': 'audio/wav',
  '.flac': 'audio/flac',
  '.webm': 'audio/webm',
};
const MAX_UPLOAD = 300 * 1024 * 1024; // bytes
const DEVICES_FRESH = 10_000; // ms a device list is used before ffmpeg is asked again

export function audioFolder(root: string): string {
  return process.env.AUDIO_DIR ?? join(root, 'audio');
}

export function audioBridge(ffmpeg = process.env.FFMPEG_PATH ?? 'ffmpeg'): Plugin {
  return {
    name: 'audio-bridge',
    apply: 'serve',
    configureServer(server) {
      const folder = audioFolder(server.config.root);
      server.middlewares.use('/__audio', (req, res) => route(req, res, folder, ffmpeg, server.config.logger));
    },
  };
}

// The audio files in the folder, by name.
export function audioFiles(folder: string): { name: string; size: number }[] {
  if (!existsSync(folder)) return [];
  return readdirSync(folder)
    .filter((name) => Object.hasOwn(EXTENSIONS, extname(name).toLowerCase()))
    .map((name) => ({ name, size: statSync(join(folder, name)).size }))
    .sort((a, b) => a.name.localeCompare(b.name));
}

// A plain audio file's name (no folders), or null.
function fileName(name: string): string | null {
  if (!name || name !== basename(name) || name.startsWith('.') || /[\\/:*?"<>|\u0000-\u001f]/.test(name)) return null;
  return Object.hasOwn(EXTENSIONS, extname(name).toLowerCase()) ? name : null;
}

let devicesSeen: { at: number; names: Promise<string[]> } | null = null;

// This machine's audio input devices, as ffmpeg opens them: DirectShow on
// Windows, AVFoundation on macOS, PulseAudio (or PipeWire's) on Linux.
export function audioDevices(ffmpeg: string): Promise<string[]> {
  if (devicesSeen && Date.now() - devicesSeen.at < DEVICES_FRESH) return devicesSeen.names;
  const names = listDevices(ffmpeg);
  devicesSeen = { at: Date.now(), names };
  return names;
}

function listDevices(ffmpeg: string): Promise<string[]> {
  const [command, args, read]: [string, string[], (text: string) => string[]] =
    process.platform === 'win32'
      ? [ffmpeg, ['-hide_banner', '-list_devices', 'true', '-f', 'dshow', '-i', 'dummy'], (text) => [...text.matchAll(/"([^"]+)" \(audio\)/g)].map((m) => m[1])]
      : process.platform === 'darwin'
        ? [
            ffmpeg,
            ['-hide_banner', '-f', 'avfoundation', '-list_devices', 'true', '-i', ''],
            (text) => [...(text.split(/audio devices:/i)[1] ?? '').matchAll(/\] \[\d+\] (.+)/g)].map((m) => m[1].trim()),
          ]
        : ['pactl', ['list', 'short', 'sources'], (text) => text.split('\n').map((line) => line.split('\t')[1]).filter(Boolean)];
  return new Promise((resolve) => {
    const child = spawn(command, args, { stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true });
    let text = '';
    child.stdout.on('data', (data: Buffer) => (text += data.toString('utf8')));
    child.stderr.on('data', (data: Buffer) => (text += data.toString('utf8')));
    child.on('error', () => resolve([]));
    child.on('exit', () => resolve([...new Set(read(text))]));
  });
}

// ffmpeg's options to read a device.
function deviceInput(name: string): string[] {
  if (process.platform === 'win32') return ['-f', 'dshow', '-audio_buffer_size', '50', '-i', `audio=${name}`];
  if (process.platform === 'darwin') return ['-f', 'avfoundation', '-i', `:${name}`];
  return ['-f', 'pulse', '-i', name];
}

// A choice from the panel, or why it can't be used.
export async function readAudioChoice(body: unknown, folder: string, ffmpeg: string): Promise<AudioChoice | string> {
  const { source, loop, volume } = (body ?? {}) as Record<string, unknown>;
  const { kind, name } = (source ?? {}) as Record<string, unknown>;
  if (typeof loop !== 'boolean') return 'loop is true or false.';
  if (typeof volume !== 'number' || !(volume >= 0 && volume <= 1)) return 'volume is from 0 to 1.';
  switch (kind) {
    case 'none':
    case 'files':
      return { source: { kind }, loop, volume };
    case 'file': {
      const file = typeof name === 'string' ? fileName(name) : null;
      if (!file || !existsSync(join(folder, file))) return `No audio file "${String(name)}" in the audio folder.`;
      return { source: { kind, name: file }, loop, volume };
    }
    case 'device': {
      if (typeof name !== 'string' || !(await audioDevices(ffmpeg)).includes(name)) return `No input device "${String(name)}" here.`;
      return { source: { kind, name }, loop, volume };
    }
    default:
      return 'source is { kind: none | file | files | device }.';
  }
}

const RATE = 44100; // samples a second
const FRAME = 4; // bytes a sample: two channels of 16 bits
const TICK = 20; // ms between writes
const AHEAD = RATE * FRAME; // bytes of a file decoded ahead: a second
const AHEAD_LOW = AHEAD / 4; // read on from the file below this
const DEVICE_AHEAD = (RATE * FRAME * 3) / 10; // a device's sound kept at most: 0.3 s, the rest dropped
const CATCH_UP = RATE * FRAME; // bytes written at most in one tick, after a stall
const SHORTEST = (RATE * FRAME) / 4; // bytes of sound a file needs to loop: a quarter second
const BACKLOG = 20 * RATE * FRAME; // bytes of sound kept for an ffmpeg that doesn't read yet: 20 s

// The stream's sound: 44.1 kHz stereo 16-bit PCM, written into ffmpeg's audio
// input at the pace of its own clock, from the chosen source, or silence
// where the source has nothing (none, between files, after the end). The
// source is decoded by a second ffmpeg: a file ahead by up to a second (it
// waits while that is full), a device as it comes. A file that ends starts
// again (loop), or the next one plays, so a loop has no gap. Changing the
// source cuts over at once; the volume and the loop change as they are set.
export class AudioFeed {
  private out: Writable | null = null; // ffmpeg's audio input
  private clock: NodeJS.Timeout | null = null;
  private started = 0; // ms, when writing began
  private written = 0; // bytes since then, silence included
  private decoder: ChildProcessByStdio<null, Readable, Readable> | null = null;
  private pieces: Buffer[] = []; // decoded, not yet written
  private held = 0; // bytes in pieces
  private next = 0; // the list's next file, for "files"
  private choice: AudioChoice = SILENCE;
  playing = ''; // what plays now: a file's name or a device's
  problem = ''; // why the source doesn't play

  constructor(
    private readonly ffmpeg: string,
    private readonly folder: string,
    private readonly logger: Logger,
  ) {}

  get current(): AudioChoice {
    return this.choice;
  }

  // Plays a choice (already checked, readAudioChoice). A new source starts at
  // once; the same source with another loop or volume plays on.
  set(choice: AudioChoice): void {
    const before = this.choice;
    this.choice = choice;
    if (JSON.stringify(before.source) === JSON.stringify(choice.source) && this.decoder) return;
    this.next = 0;
    this.play();
  }

  // Writes into ffmpeg's audio input from now on: a new ffmpeg, whose sound
  // starts with its first picture.
  attach(out: Writable): void {
    this.out = out;
    out.on('error', () => {}); // ffmpeg gone; its exit says why
    this.started = performance.now();
    this.written = 0;
    this.clock ??= setInterval(() => this.tick(), TICK);
  }

  // Stops writing into ffmpeg; with `end`, its sound ends too, so a stopping
  // ffmpeg can finish (with no end, it would wait for more).
  detach(end = false): void {
    if (end) this.out?.end();
    this.out = null;
  }

  // Stops the source and the clock; the stream is over.
  stop(): void {
    if (this.clock) clearInterval(this.clock);
    this.clock = null;
    this.out = null;
    this.quit();
    this.playing = '';
  }

  // Starts decoding the chosen source.
  private play(): void {
    this.quit();
    this.problem = '';
    const { source } = this.choice;
    let input: string[];
    let name: string;
    if (source.kind === 'none') {
      this.playing = '';
      return;
    } else if (source.kind === 'device') {
      input = deviceInput(source.name);
      name = source.name;
    } else {
      const files = audioFiles(this.folder).map((f) => f.name);
      const file = source.kind === 'file' ? source.name : files[this.next % Math.max(1, files.length)];
      if (!file || !files.includes(file)) {
        this.playing = '';
        this.problem = files.length ? `${file} is not in the audio folder any more.` : 'The audio folder has no files.';
        return;
      }
      input = ['-i', join(this.folder, file)];
      name = file;
    }
    const decoder = spawn(this.ffmpeg, ['-hide_banner', '-loglevel', 'error', ...input, '-vn', '-f', 's16le', '-ar', String(RATE), '-ac', '2', 'pipe:1'], {
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
    });
    this.decoder = decoder;
    this.playing = name;
    const device = source.kind === 'device';
    let produced = 0; // bytes
    decoder.stdout.on('data', (data: Buffer) => {
      if (this.decoder !== decoder) return;
      produced += data.length;
      this.pieces.push(data);
      this.held += data.length;
      if (device) this.drop(DEVICE_AHEAD);
      else if (this.held >= AHEAD) decoder.stdout.pause(); // ffmpeg waits, and the file with it
    });
    let words = '';
    decoder.stderr.on('data', (data: Buffer) => (words += data.toString('utf8')));
    decoder.on('error', (error) => {
      if (this.decoder === decoder) this.problem = `ffmpeg could not start: ${error.message}`;
    });
    decoder.on('exit', (code) => {
      if (this.decoder !== decoder) return;
      this.decoder = null;
      const why = words.trim().split(/\r?\n/).at(-1) ?? '';
      if (code !== 0) {
        this.playing = '';
        this.problem = why || `ffmpeg stopped (code ${code}).`;
        this.logger.warn(`audio: ${name}: ${this.problem}`, { timestamp: true });
        return;
      }
      // The end of a file: again, the next, or silence. A file with next to
      // no sound in it would start again and again at once, so it doesn't.
      if (produced < SHORTEST && this.choice.source.kind === 'file') {
        this.playing = '';
        this.problem = `${name} is too short to loop.`;
        return;
      }
      const all = audioFiles(this.folder).length;
      if (this.choice.source.kind === 'files') this.next += 1;
      const more = this.choice.source.kind === 'files' && this.next < all;
      if (this.choice.loop || more) {
        if (this.choice.source.kind === 'files') this.next %= Math.max(1, all);
        this.play();
      } else {
        this.playing = '';
      }
    });
  }

  private quit(): void {
    const decoder = this.decoder;
    this.decoder = null;
    decoder?.kill();
    this.pieces = [];
    this.held = 0;
  }

  // Keeps at most `most` bytes, the newest, dropping whole samples from the
  // front so the rest stays in step.
  private drop(most: number): void {
    while (this.held > most && this.pieces.length) {
      const first = this.pieces[0];
      const cut = Math.ceil((this.held - most) / FRAME) * FRAME;
      if (cut >= first.length) {
        this.pieces.shift();
        this.held -= first.length;
      } else {
        this.pieces[0] = first.subarray(cut);
        this.held -= cut;
      }
    }
  }

  // Writes what is due by the clock: the source's sound, then silence.
  private tick(): void {
    const due = Math.floor(((performance.now() - this.started) / 1000) * RATE) * FRAME;
    let length = due - this.written;
    if (length <= 0) return;
    if (length > CATCH_UP) {
      // The process was busy for a while: what was missed is skipped.
      this.written = due - CATCH_UP;
      length = CATCH_UP;
    }
    const sound = Buffer.alloc(length); // silence where the source has nothing
    // Only whole samples: half of one would put every sample after it out of
    // step, which sounds like noise.
    const wanted = Math.min(length, this.held - (this.held % FRAME));
    let at = 0;
    while (at < wanted) {
      const piece = this.pieces[0];
      const take = Math.min(piece.length, wanted - at);
      piece.copy(sound, at, 0, take);
      at += take;
      this.held -= take;
      if (take === piece.length) this.pieces.shift();
      else this.pieces[0] = piece.subarray(take);
    }
    if (this.decoder && this.held < AHEAD_LOW) this.decoder.stdout.resume();
    const gain = this.choice.volume;
    if (gain < 1) {
      for (let i = 0; i < at; i += 2) sound.writeInt16LE(Math.round(sound.readInt16LE(i) * gain), i);
    }
    this.written += length;
    // Until ffmpeg reads the sound (first it looks into the pictures' first
    // seconds) it waits for it, all of it, so that the sound starts where the
    // pictures start: sound dropped here would put it behind them, and ffmpeg
    // would hold the pictures back to wait for it. Only a stalled ffmpeg gets
    // more than BACKLOG, and then the rest is dropped.
    if (this.out && this.out.writableLength < BACKLOG) this.out.write(sound);
  }
}

function route(req: Connect.IncomingMessage, res: ServerResponse, folder: string, ffmpeg: string, logger: Logger): void {
  const answer = (status: number, text = '') => {
    res.statusCode = status;
    res.setHeader('Content-Type', 'text/plain; charset=utf-8');
    res.end(text && `${text}\n`);
  };
  const json = (value: unknown) => {
    res.setHeader('Content-Type', 'application/json');
    res.setHeader('Cache-Control', 'no-store');
    res.end(JSON.stringify(value));
  };
  const url = new URL(req.url ?? '/', 'http://localhost');
  let path: string;
  try {
    path = decodeURIComponent(url.pathname);
  } catch {
    return answer(400, 'A broken path.');
  }

  if (req.method === 'GET' && path === '/files') return json({ files: audioFiles(folder) });
  if (req.method === 'GET' && path === '/devices') {
    void audioDevices(ffmpeg).then((devices) => json({ devices }));
    return;
  }
  if (req.method === 'GET' && path.startsWith('/files/')) {
    const name = fileName(path.slice('/files/'.length));
    if (!name || !existsSync(join(folder, name))) return answer(404, 'No such audio file.');
    return sendFile(req, res, join(folder, name), EXTENSIONS[extname(name).toLowerCase()]);
  }
  if (req.method === 'POST' && path === '/files') {
    const name = fileName(url.searchParams.get('name') ?? '');
    if (!name) return answer(400, `The name must be a plain file name ending in ${Object.keys(EXTENSIONS).join(', ')}.`);
    const target = join(folder, name);
    if (existsSync(target)) return answer(409, `${name} is in the audio folder already.`);
    // Written beside it first, so a cut-off upload never looks like a file.
    mkdirSync(folder, { recursive: true });
    const partial = `${target}.part`;
    const file = createWriteStream(partial);
    let size = 0;
    let failed = false;
    const fail = (status: number, text: string) => {
      if (failed) return;
      failed = true;
      req.unpipe(file);
      file.destroy();
      void rm(partial, { force: true });
      answer(status, text);
    };
    req.on('data', (piece: Buffer) => {
      size += piece.length;
      if (size > MAX_UPLOAD) fail(413, `An audio file is at most ${MAX_UPLOAD / 1024 / 1024} MB.`);
    });
    req.on('close', () => {
      if (!req.complete) fail(400, 'The upload was cut off.');
    });
    file.on('error', (error) => fail(500, error.message));
    file.on('finish', () => {
      if (failed) return;
      rename(partial, target).then(
        () => {
          logger.info(`audio: added ${name}`, { timestamp: true });
          answer(201);
        },
        (error: Error) => fail(500, error.message),
      );
    });
    req.pipe(file);
    return;
  }
  answer(req.method === 'GET' || req.method === 'POST' ? 404 : 405, 'GET /files, /files/<name> or /devices; POST /files?name=');
}

// A file, whole or the range asked for, so the page's player can seek.
function sendFile(req: Connect.IncomingMessage, res: ServerResponse, path: string, type: string): void {
  const size = statSync(path).size;
  res.setHeader('Content-Type', type);
  res.setHeader('Accept-Ranges', 'bytes');
  res.setHeader('Cache-Control', 'no-cache');
  const range = /^bytes=(\d*)-(\d*)$/.exec(req.headers.range ?? '');
  if (range && (range[1] || range[2])) {
    const start = range[1] ? Number(range[1]) : Math.max(0, size - Number(range[2]));
    const end = range[1] && range[2] ? Math.min(size - 1, Number(range[2])) : size - 1;
    if (start > end || start >= size) {
      res.statusCode = 416;
      res.setHeader('Content-Range', `bytes */${size}`);
      res.end();
      return;
    }
    res.statusCode = 206;
    res.setHeader('Content-Range', `bytes ${start}-${end}/${size}`);
    res.setHeader('Content-Length', String(end - start + 1));
    createReadStream(path, { start, end }).pipe(res);
    return;
  }
  res.setHeader('Content-Length', String(size));
  createReadStream(path).pipe(res);
}
