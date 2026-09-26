import { spawn, type ChildProcess, type ChildProcessByStdio } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { existsSync, mkdtempSync } from 'node:fs';
import { rm } from 'node:fs/promises';
import type { ServerResponse } from 'node:http';
import { tmpdir } from 'node:os';
import { basename, delimiter, join } from 'node:path';
import type { Readable, Writable } from 'node:stream';
import type { Connect, Logger, Plugin, ViteDevServer } from 'vite';
import { AudioFeed, audioFolder, readAudioChoice, SILENCE, type AudioChoice } from './audio-bridge.ts';

// Live streaming the preview to YouTube (or any RTMP server). Browsers can't
// speak RTMP, so a page records the scene's canvas (src/stream.ts) and posts
// the recording here in short pieces. Each recording goes into a decoder, an
// ffmpeg of its own, which fits its pictures to 720p or 1080p at 30 a second,
// raw. This plugin hands them on at a steady 30 a second to the encoder, one
// ffmpeg for the whole stream, which encodes them as H.264 with a keyframe
// every 2 s, adds the sound the Audio tab picks (AudioFeed in
// audio-bridge.ts; silence by default, and YouTube wants a sound track either
// way) and pushes it to the RTMP server as FLV. While no new picture comes (a
// page reloading after a code change, its code broken, its tab hidden) the
// last one goes again, so YouTube's connection stays open and the next
// recording carries on where the last one stopped. One stream at a time.
//
// Who draws the stream, as the Stream tab's Render option says:
// - the page (web + stream): the panel's own page records what it shows. The
//   start's answer names the stream's session, which the page keeps
//   (sessionStorage) and names again when it records after a reload, so a
//   reload carries on and another page can't take the stream over. A page
//   gone for PAGE_GONE s (a closed tab) ends the stream.
// - the dev server (stream only): this plugin starts a hidden (headless)
//   Chrome or Edge, which opens the page as the stream's renderer
//   (?renderer=1280x720 and the panel's settings: no panel, drawn at the
//   stream's size; src/streamRenderer.ts) and records it. The panel steers it
//   through /follow. It streams until /stop, whether or not the panel's page
//   stays open, and waits for the hidden page as long as the hidden browser
//   runs: a broken save freezes the picture until the fix.
// With `gpu`, ffmpeg encodes on the graphics card (the first of
// GPU_ENCODERS that works here) and the hidden browser draws on it; without,
// both run on the CPU (x264, and SwiftShader for the drawing).
//
// A dev server restart (vite.config.ts or a file it imports changed) doesn't
// end a stream either: the stream keeps the code it started with, and the
// new plugin passes /__stream on to it until it ends (Carried).
//
// - POST /__stream/start, JSON { url, key, quality, drawnBy, gpu, scene,
//   audio }: starts the stream, and the hidden browser for drawnBy "server",
//   which draws `scene` (the panel's settings as a query string), with the
//   sound `audio` says (a source it can't play leaves silence and says why
//   in the status). Answered 202 with JSON { session }; 400 with
//   the reason for a bad URL, key or quality, 409 while a stream runs. The URL
//   must be rtmp:// or rtmps:// (YouTube's is rtmp://a.rtmp.youtube.com/live2),
//   so a request can never make ffmpeg write a file here; the key goes after
//   it, as YouTube's stream URL + key. The encoder connects with the first
//   pictures.
// - POST /__stream/recording, JSON { id, graphics, session? }: a page starts
//   recording, drawn on `graphics` (its WebGL renderer). A new recording takes
//   over from the one before when the hidden browser sends it, or a page
//   naming the session. Answered 204, or 409.
// - POST /__stream/chunk, the next piece of the recording
//   (application/octet-stream, header X-Recording: its id), answered 204 once
//   its decoder has taken it, or 409 when no stream takes it.
// - POST /__stream/follow, JSON { scene } or { behaviour: { name, args } } or
//   { restart: true } or { walk: [moves held] }: passed on to the hidden
//   browser as the Vite event 'stream:follow'. 204, or 409 when the dev
//   server draws no stream.
// - POST /__stream/audio, JSON { source, loop, volume }: the sound from now
//   on, while the stream goes on. 204; 400 for a source that isn't here, 409
//   when no stream runs.
// - POST /__stream/preview, a JPEG: the hidden browser's latest picture, which
//   GET /__stream/preview returns (404 before the first), for the panel.
// - POST /__stream/log, text: an error in the hidden browser's page, for this
//   terminal.
// - POST /__stream/stop: ends the stream; ffmpeg finishes what it has and
//   closes the connection. Answered 204.
// - GET /__stream/status: JSON StreamStatus.
//
// The stream key is a secret: it never goes in a log or a message (ffmpeg's
// words have it masked). ffmpeg must be installed (winget install ffmpeg), or
// FFMPEG_PATH set to it. The hidden browser is Chrome, Edge or Chromium where
// they install, or STREAM_BROWSER; STREAM_BROWSER_ARGS adds flags to it (a
// test's --remote-debugging-port).

export type StreamState = 'off' | 'starting' | 'live' | 'stopping' | 'error';
export type DrawnBy = 'page' | 'server'; // the panel's page, or the hidden browser here

export interface StreamStatus {
  state: StreamState;
  message: string; // what went wrong, or how it ended
  drawnBy: DrawnBy;
  gpu: boolean; // the graphics card was asked for
  graphics: string; // what draws the pictures: the recording page's WebGL renderer
  encoder: string; // ffmpeg's video encoder
  scene: string; // what the hidden browser draws: the panel's settings as a query string
  frames: number; // sent so far
  fps: number; // new pictures a second, not counting the repeats that keep it at 30
  frozen: number; // s the last picture has been sent again for, as no new one came; 0 while they come
  kbps: number; // the bitrate going out
  seconds: number; // of stream sent
  audio: { choice: AudioChoice; playing: string; problem: string } | null; // its sound, while it streams
}

// A change for the hidden browser: the panel's settings, a behaviour run,
// its demo restarted, or what is held down to walk the camera.
// src/stream.ts has the same shape.
export interface Follow {
  scene?: string;
  behaviour?: { name: string; args: string[] };
  restart?: true;
  walk?: string[];
}

export const QUALITIES = {
  '720p': { width: 1280, height: 720, kbps: 3000 },
  '1080p': { width: 1920, height: 1080, kbps: 6000 },
} as const;
export type Quality = keyof typeof QUALITIES;

interface StartRequest {
  url: string;
  key: string;
  quality: Quality;
  drawnBy: DrawnBy;
  gpu: boolean;
  scene: string;
  audio: AudioChoice | string; // or why the one asked for can't play
}

const FPS = 30;
const KEYFRAME = 2 * FPS; // frames from one keyframe to the next; YouTube asks for one at least every 4 s
const AUDIO_KBPS = 128;
const MAX_JSON = 16 * 1024; // bytes
const MAX_CHUNK = 16 * 1024 * 1024; // bytes; a piece is a quarter second of video
const MAX_PREVIEW = 2 * 1024 * 1024; // bytes of JPEG
const MAX_LOG = 4 * 1024; // bytes
const MAX_SCENE = 2048; // characters of query string
const FIRST = 15; // s a page gets to send its first pictures after the start
const FIRST_HIDDEN = 40; // the same for the hidden browser, which has to start first
const PAGE_GONE = 60; // s without a piece from the page (web + stream) before the stream ends: a closed tab
const FROZEN = 2; // s without a new picture before the status says the last one goes again
const FROZEN_SAY = 5; // s of it before the terminal says so
const BUFFER = 12; // pictures held before they go on (0.4 s), smoothing the pieces' arrival
const MOST = 30; // pictures held at most; past that the oldest are dropped, down to BUFFER
const CATCH_UP = FPS; // pictures sent at most at once, after this process was busy
const BACKLOG = 8; // pictures waiting for the encoder at most; one that can't keep up misses the rest
const PUMP_MS = 10; // how often the pictures due go to the encoder
const STOP_WAIT = 10; // s ffmpeg gets to finish after the stream ends, before it is killed
const ERROR_LINES = 6; // of ffmpeg's last words kept for a message
const PROBE_WAIT = 15; // s an encoder gets to show it works
const FPS_REPORTS = 4; // of ffmpeg's progress reports (two a second) the frame rate is measured over

// How ffmpeg encodes the picture.
interface Encoder {
  name: string; // ffmpeg's
  device?: string[]; // options before the inputs that open the graphics card
  format: 'yuv420p' | 'nv12'; // the pixels it takes
  upload?: boolean; // the pictures go up to the graphics card first (hwupload)
  options: string[]; // its own, besides the bitrate and the keyframes
}

// On the CPU.
const X264: Encoder = {
  name: 'libx264',
  format: 'yuv420p',
  options: ['-preset', 'veryfast', '-tune', 'zerolatency', '-keyint_min', String(KEYFRAME), '-sc_threshold', '0'],
};

// On the graphics card, best first; the first that works with this ffmpeg on
// this machine is used (Stream.gpuEncoder). NVIDIA's own (NVENC) needs a
// driver as new as the ffmpeg build; Media Foundation (Windows) and Vulkan
// reach the same encoder through older drivers too.
const GPU_ENCODERS: Encoder[] = [
  { name: 'h264_nvenc', format: 'yuv420p', options: ['-preset', 'p4', '-tune', 'll', '-rc', 'cbr', '-profile:v', 'high'] },
  { name: 'h264_qsv', format: 'nv12', options: ['-preset', 'veryfast', '-profile:v', 'high'] },
  { name: 'h264_amf', format: 'nv12', options: ['-usage', 'lowlatency', '-rc', 'cbr', '-profile:v', 'high'] },
  { name: 'h264_videotoolbox', format: 'nv12', options: ['-realtime', '1', '-profile:v', 'high'] },
  // Its profile takes a number: 100 is High.
  {
    name: 'h264_mf',
    format: 'nv12',
    options: ['-hw_encoding', '1', '-scenario', 'live_streaming', '-rate_control', 'cbr', '-profile:v', '100'],
  },
  {
    name: 'h264_vulkan',
    device: ['-init_hw_device', 'vulkan=gpu', '-filter_hw_device', 'gpu'],
    format: 'nv12',
    upload: true,
    options: ['-rc_mode', 'cbr', '-profile:v', 'high'],
  },
  {
    name: 'h264_vaapi',
    device: ['-vaapi_device', '/dev/dri/renderD128'],
    format: 'nv12',
    upload: true,
    options: ['-rc_mode', 'CBR', '-profile:v', 'high'],
  },
];

// The filters that fit a picture to the frame: scaled (bars where the
// drawing's shape differs), in the encoder's pixels.
function fitting(width: number, height: number, format: Encoder['format']): string[] {
  return [`scale=${width}:${height}:force_original_aspect_ratio=decrease`, `pad=${width}:${height}:(ow-iw)/2:(oh-ih)/2`, `format=${format}`];
}

// ffmpeg's options to encode fitted pictures: the encoder's own, the bitrate,
// and a keyframe every 2 s.
function encoding(encoder: Encoder, kbps: number): string[] {
  return [
    ['-c:v', encoder.name, ...encoder.options],
    ['-b:v', `${kbps}k`, '-maxrate', `${kbps}k`, '-bufsize', `${2 * kbps}k`, '-g', String(KEYFRAME)],
  ].flat();
}

// A stream that runs through a dev server restart. Vite loads this file again
// and makes a new plugin, in the same process; the stream goes on with the
// code it started with, and the new plugin passes its requests on to it.
interface Carried {
  running(): boolean;
  route(req: Connect.IncomingMessage, res: ServerResponse): void;
  stop(now: boolean): void;
}

const CARRIED = Symbol.for('animation:stream-bridge');
const carried = globalThis as { [CARRIED]?: Carried };

export function streamBridge(ffmpeg = process.env.FFMPEG_PATH ?? 'ffmpeg'): Plugin {
  let stream: Stream | null = null;
  return {
    name: 'stream-bridge',
    apply: 'serve',
    configureServer(server) {
      const own = new Stream(ffmpeg, server);
      stream = own;
      server.middlewares.use('/__stream', (req, res) => {
        const earlier = carried[CARRIED];
        if (earlier?.running()) return earlier.route(req, res);
        delete carried[CARRIED];
        route(req, res, own);
      });
    },
    // A restart carries a running stream over to the new plugin; a close
    // ends it.
    closeServer({ reason }) {
      const own = stream;
      if (!own) return;
      if (reason === 'restart') {
        if (own.running) carried[CARRIED] = { running: () => own.running, route: (req, res) => route(req, res, own), stop: (now) => own.stop(now) };
        return;
      }
      carried[CARRIED]?.stop(true);
      delete carried[CARRIED];
      own.stop(true);
    },
  };
}

// An ffmpeg of the stream's: the encoder (pictures on stdin, the sound on fd
// 3) or a recording's decoder (the recording on stdin, pictures on stdout,
// its progress on fd 3).
type Ffmpeg = ChildProcessByStdio<Writable, Readable, Readable>;

// The one stream: its encoder, the current recording's decoder, the pictures
// between them, its sound, and its hidden browser when the dev server draws
// it.
class Stream {
  private active = false; // from start until the stream has ended
  private encoder: Ffmpeg | null = null; // from the first pictures to the end
  private decoder: Ffmpeg | null = null; // the current recording's
  private feed: AudioFeed | null = null; // the sound, while it streams
  private args: string[] = []; // the encoder's
  private browser: HiddenBrowser | null = null;
  private secret = ''; // the key, to mask
  private session = ''; // names the stream to the page that started it
  private status: StreamStatus = idle('');
  private recording: string | null = null; // the id of the recording being decoded
  private readonly retired = new Set<string>(); // recordings that ended; late pieces of theirs are dropped
  private started = 0; // ms, when the stream started
  private lastPiece = 0; // ms, when a piece last came
  private lastPicture = 0; // ms, when the decoder last gave a picture
  private watchdog: NodeJS.Timeout | null = null;
  private lastWords: string[] = [];
  private report = new Map<string, string>(); // the encoder's progress report being read
  private marks: { at: number; fresh: number }[] = []; // for the frame rate, from the decoder's reports
  private gpuSearch: Promise<Encoder | null> | null = null;
  private readonly logger: Logger;
  private readonly folder: string; // of audio files
  // The pictures, raw in the encoder's pixels: made whole from the decoder's
  // output, held a moment, and sent at 30 a second.
  private width = 0;
  private height = 0;
  private format: Encoder['format'] = 'yuv420p';
  private size = 0; // bytes a picture
  private filling: Buffer | null = null; // the picture the decoder is writing
  private filled = 0; // bytes of it so far
  private held: Buffer[] = []; // whole, not yet sent
  private spare: Buffer[] = []; // dropped before they were sent, to fill again
  private last: Buffer | null = null; // sent last, and again while no new one comes
  private flowing = false; // held pictures go out; false until BUFFER are held, at the start and for a new recording
  private owed = 0; // turns the last picture filled because the next came late; that many late ones are skipped
  private pump: NodeJS.Timeout | null = null;
  private pumpStart = 0; // ms (performance.now), when the encoder started
  private sent = 0; // pictures since then
  preview: Buffer | null = null; // the hidden browser's latest picture

  constructor(
    private readonly ffmpeg: string,
    private readonly server: ViteDevServer,
  ) {
    this.logger = server.config.logger;
    this.folder = audioFolder(server.config.root);
  }

  get running(): boolean {
    return this.active;
  }

  // The dev server draws the stream in its hidden browser.
  get drawnHere(): boolean {
    return this.active && this.status.drawnBy === 'server' && this.status.state !== 'stopping';
  }

  get current(): StreamStatus {
    const feed = this.feed;
    return { ...this.status, audio: feed ? { choice: feed.current, playing: feed.playing, problem: feed.problem } : null };
  }

  // A sound from the panel, or why it can't be played here.
  checkAudio(body: unknown): Promise<AudioChoice | string> {
    return readAudioChoice(body, this.folder, this.ffmpeg);
  }

  // The sound from now on; the stream goes on.
  setAudio(choice: AudioChoice): void {
    if (!this.feed || this.status.state === 'stopping') throw new Error('No stream is running.');
    this.feed.set(choice);
  }

  // Starts a stream to the URL and key, already checked (readStart()), and
  // for drawnBy "server" the hidden browser that draws it; the encoder starts
  // with the first pictures. How it goes shows in `current`. Resolves to the
  // stream's session.
  async start({ url, key, quality, drawnBy, gpu, scene, audio }: StartRequest): Promise<string> {
    const { width, height, kbps } = QUALITIES[quality];
    this.active = true;
    this.secret = key;
    this.session = randomUUID();
    this.recording = null;
    this.retired.clear();
    this.preview = null;
    this.status = { ...idle(''), state: 'starting', drawnBy, gpu, scene: drawnBy === 'server' ? scene : '' };
    this.started = this.lastPiece = this.lastPicture = Date.now();
    // The sound gets ready now, and plays from the first picture.
    this.feed = new AudioFeed(this.ffmpeg, this.folder, this.logger);
    this.feed.set(typeof audio === 'string' ? SILENCE : audio);
    if (typeof audio === 'string') this.feed.problem = audio;
    const session = this.session;

    const encoder = (gpu && (await this.gpuEncoder())) || X264;
    if (!this.active || this.status.state !== 'starting') return session; // stopped while looking for an encoder
    this.status.encoder = encoder.name;
    this.width = width;
    this.height = height;
    this.format = encoder.format;
    this.size = (width * height * 3) / 2; // yuv420p and nv12 alike
    this.clearPictures();
    const target = `${url.replace(/\/+$/, '')}/${key}`;
    this.args = [
      // Only errors: a live recording's timestamps make routine warnings that
      // would flood the terminal. Progress comes as key=value lines on stdout.
      ['-hide_banner', '-loglevel', 'error', '-nostats', '-progress', 'pipe:1'],
      encoder.device ?? [],
      // The pictures, raw, already fitted, 30 a second, from this plugin.
      ['-f', 'rawvideo', '-pixel_format', encoder.format, '-video_size', `${width}x${height}`, '-framerate', String(FPS)],
      ['-thread_queue_size', '64', '-i', 'pipe:0'],
      // The sound, raw, from the feed (fd 3). Raw sound has nothing to look
      // into, and looking (5 s by default) at sound that comes in real time
      // held the stream back for as long.
      ['-thread_queue_size', '1024', '-probesize', '32', '-analyzeduration', '0'],
      ['-f', 's16le', '-ar', '44100', '-ac', '2', '-i', 'pipe:3'],
      ['-map', '0:v:0', '-map', '1:a:0'],
      encoder.upload ? ['-vf', 'hwupload'] : [],
      encoding(encoder, kbps),
      ['-c:a', 'aac', '-b:a', `${AUDIO_KBPS}k`, '-ar', '44100'],
      // No -shortest: with the sound live, it held the pictures back for 10 s
      // at the start. Stop ends both the pictures and the sound instead.
      ['-f', 'flv', target],
    ].flat();
    const drawer = drawnBy === 'server' ? 'the dev server' : 'the page';
    this.logger.info(`stream: starting ${width}x${height} to ${this.mask(target)}, drawn by ${drawer}, encoded with ${encoder.name}`, {
      timestamp: true,
    });
    this.watchdog = setInterval(() => this.check(), 1000);
    if (drawnBy === 'server') this.openBrowser(width, height, gpu, scene);
    return session;
  }

  // A page starts recording, with a decoder of its own. A new recording takes
  // over from the one before when the hidden browser sends it (its page
  // loaded again), or the page that started the stream (`session`) after a
  // reload; the stream shows the last picture meanwhile.
  announce(id: string, graphics: string, session: string): void {
    if (!this.active || this.status.state === 'stopping' || !this.size) throw new Error('No stream is running.');
    if (id === this.recording) return;
    if (this.retired.has(id)) throw new Error('That recording has ended.');
    if (this.recording) {
      if (this.status.drawnBy === 'page' && session !== this.session) throw new Error('Another page is streaming already. Stop it first.');
      this.retired.add(this.recording);
      const what = this.status.drawnBy === 'server' ? 'the hidden browser loaded the page again' : 'the page that streams was loaded again';
      this.logger.info(`stream: ${what}; the stream goes on`, { timestamp: true });
    }
    this.recording = id;
    this.lastPiece = Date.now();
    this.status.graphics = graphics;
    this.startDecoder();
  }

  // Hands a piece of the recording to its decoder, once it can take it.
  async write(piece: Buffer, id: string): Promise<void> {
    if (!this.active || this.status.state === 'stopping') throw new Error('No stream is running.');
    if (id !== this.recording) throw new Error(this.retired.has(id) ? 'That recording has ended.' : 'Announce the recording first.');
    const decoder = this.decoder;
    if (!decoder) throw new Error('That recording has ended.'); // its decoder failed
    this.lastPiece = Date.now();
    if (!decoder.stdin.write(piece)) await drained(decoder.stdin);
  }

  // Passes a change from the panel on to the hidden browser.
  follow(change: Follow): void {
    if (!this.drawnHere) throw new Error('The dev server is not drawing a stream.');
    if (change.scene !== undefined) this.status.scene = change.scene;
    this.server.hot.send('stream:follow', change);
  }

  // A line from the hidden browser's page: an error there.
  log(text: string): void {
    if (!this.drawnHere) return;
    const lines = text.split(/\r?\n/).map((line) => line.trim());
    for (const line of lines.filter(Boolean).slice(0, 10)) {
      this.logger.warn(`stream: hidden browser: ${this.mask(line)}`, { timestamp: true });
    }
  }

  // Ends the stream: the encoder finishes what it has, or is killed at once
  // (`now`) or after STOP_WAIT. The hidden browser closes at once.
  stop(now = false): void {
    if (!this.active) return;
    this.closeBrowser();
    const child = this.encoder;
    if (!child) return this.finish('Stopped.', false); // no pictures yet
    this.status = { ...this.status, state: 'stopping' };
    this.stopPump();
    this.stopDecoder();
    this.feed?.detach(true);
    child.stdin.end();
    const kill = () => {
      if (this.encoder === child) child.kill('SIGKILL');
    };
    if (now) kill();
    else setTimeout(kill, STOP_WAIT * 1000).unref();
  }

  // Text with the stream key hidden.
  mask(text: string): string {
    return this.secret ? text.split(this.secret).join('••••') : text;
  }

  // A decoder for the new recording, in place of the last one: WebM in; out,
  // its pictures fitted to the stream, in the encoder's pixels, at a steady
  // 30 a second (a picture that comes late is made again, and counted in
  // dup_frames), raw.
  private startDecoder(): void {
    this.stopDecoder();
    const args = [
      ['-hide_banner', '-loglevel', 'error', '-nostats', '-progress', 'pipe:3'],
      ['-f', 'matroska', '-thread_queue_size', '1024', '-i', 'pipe:0', '-map', '0:v:0'],
      ['-vf', fitting(this.width, this.height, this.format).join(','), '-r', String(FPS), '-fps_mode', 'cfr'],
      ['-f', 'rawvideo', 'pipe:1'],
    ].flat();
    const child = spawn(this.ffmpeg, args, { stdio: ['pipe', 'pipe', 'pipe', 'pipe'], windowsHide: true }) as Ffmpeg;
    this.decoder = child;
    // What the last one left goes, and the new one's pictures go out once
    // BUFFER are held.
    this.filled = 0;
    this.drop(this.held.length);
    this.flowing = false;
    this.marks = [];
    child.stdin.on('error', () => {}); // a write after it is gone; its exit says why
    child.on('error', (error) => {
      if (this.decoder !== child) return;
      this.decoder = null;
      this.logger.error(`stream: ffmpeg could not start: ${error.message}`, { timestamp: true });
    });
    child.stdout.on('data', (data: Buffer) => {
      if (this.decoder === child) this.take(data);
    });
    // Its progress, for the frame rate: the new pictures a second.
    const report = new Map<string, string>();
    let pending = '';
    (child.stdio[3] as Readable).on('data', (data: Buffer) => {
      if (this.decoder !== child) return;
      pending += data.toString('utf8');
      const lines = pending.split(/\r?\n/);
      pending = lines.pop() ?? '';
      for (const line of lines) this.decoderProgress(line, report);
    });
    let words = '';
    child.stderr.on('data', (data: Buffer) => {
      if (this.decoder !== child) return;
      words += data.toString('utf8');
      const lines = words.split(/\r?\n/);
      words = lines.pop() ?? '';
      for (const line of lines.map((l) => this.mask(l.trim())).filter(Boolean)) {
        this.logger.error(`stream: ffmpeg (recording): ${line}`, { timestamp: true });
      }
    });
    child.on('exit', (code, signal) => {
      if (this.decoder !== child) return; // replaced, or the stream ended
      this.decoder = null;
      this.logger.warn(`stream: the recording's decoder stopped (${signal ?? `code ${code}`}); the stream shows its last picture`, {
        timestamp: true,
      });
    });
  }

  private stopDecoder(): void {
    const decoder = this.decoder;
    this.decoder = null;
    decoder?.kill('SIGKILL');
  }

  // The decoder's output, as it comes, made into whole pictures.
  private take(data: Buffer): void {
    let at = 0;
    while (at < data.length) {
      this.filling ??= this.spare.pop() ?? Buffer.allocUnsafe(this.size);
      const copied = data.copy(this.filling, this.filled, at);
      this.filled += copied;
      at += copied;
      if (this.filled < this.size) continue;
      this.held.push(this.filling);
      this.filling = null;
      this.filled = 0;
      this.lastPicture = Date.now();
      // Too many held (the page sent a backlog, or the decoder made up for a
      // gap): the oldest go, and the stream catches up.
      if (this.held.length > MOST) {
        this.drop(this.held.length - BUFFER);
        this.owed = 0;
      }
      if (!this.encoder && this.held.length >= BUFFER) this.startEncoder();
    }
  }

  // Drops the oldest `count` held pictures, never sent, to be filled again.
  private drop(count: number): void {
    for (const picture of this.held.splice(0, count)) if (this.spare.length < BUFFER) this.spare.push(picture);
  }

  // The encoder, with the first pictures: it connects to YouTube, and stays
  // until the stream ends.
  private startEncoder(): void {
    this.lastWords = [];
    this.report.clear();
    const child = spawn(this.ffmpeg, this.args, { stdio: ['pipe', 'pipe', 'pipe', 'pipe'], windowsHide: true }) as Ffmpeg;
    this.encoder = child;
    (child.stdio[3] as Writable).on('error', () => {}); // ffmpeg gone; its exit says why

    child.on('error', (error) => {
      if (this.encoder !== child) return;
      this.encoder = null;
      const missing = (error as NodeJS.ErrnoException).code === 'ENOENT';
      this.finish(
        missing
          ? `ffmpeg was not found. Install it (winget install ffmpeg) or set FFMPEG_PATH, then restart the dev server.`
          : `ffmpeg could not start: ${error.message}`,
        true,
      );
    });
    child.stdin.on('error', () => {}); // a write after ffmpeg is gone; its exit says why

    // -progress writes key=value lines, a report at a time.
    let pending = '';
    child.stdout.on('data', (data: Buffer) => {
      if (this.encoder !== child) return;
      pending += data.toString('utf8');
      const lines = pending.split(/\r?\n/);
      pending = lines.pop() ?? '';
      for (const line of lines) this.progress(line);
    });
    let words = '';
    child.stderr.on('data', (data: Buffer) => {
      if (this.encoder !== child) return;
      words += data.toString('utf8');
      const lines = words.split(/\r?\n/);
      words = lines.pop() ?? '';
      for (const line of lines.map((l) => this.mask(l.trim())).filter(Boolean)) {
        this.lastWords = [...this.lastWords, line].slice(-ERROR_LINES);
        this.logger.error(`stream: ffmpeg: ${line}`, { timestamp: true });
      }
    });
    child.on('exit', (code, signal) => {
      if (this.encoder !== child) return; // ended on purpose
      this.encoder = null;
      const stopping = this.status.state === 'stopping';
      if (code === 0 || stopping) this.finish(stopping ? 'Stopped.' : 'The stream ended.', false);
      else this.finish(this.lastWords.at(-1) ?? `ffmpeg stopped (${signal ?? `code ${code}`}).`, true);
    });

    // The sound starts with the first picture, so they start together.
    this.feed?.attach(child.stdio[3] as Writable);
    this.pumpStart = performance.now();
    this.sent = 0;
    this.flowing = false;
    this.pump = setInterval(() => this.send(), PUMP_MS);
  }

  // Sends the encoder the pictures due by the clock: the held ones in turn,
  // BUFFER behind the decoder, or the last one again while none are held.
  // Pictures that come after their turn (it went to the last one again) are
  // skipped, so the stream stays BUFFER behind and a hitch in the drawing
  // shows no longer than it was.
  private send(): void {
    const encoder = this.encoder;
    if (!encoder) return;
    const due = Math.floor(((performance.now() - this.pumpStart) / 1000) * FPS);
    if (due - this.sent > CATCH_UP) this.sent = due - CATCH_UP; // this process was busy: the missed ones are skipped
    while (this.sent < due) {
      this.sent += 1;
      if (!this.flowing && this.held.length >= BUFFER) {
        this.flowing = true;
        this.owed = 0;
      }
      if (this.flowing && this.held.length) {
        const late = Math.min(this.owed, this.held.length - 1);
        this.drop(late);
        this.owed -= late;
        this.last = this.held.shift()!;
      } else if (this.flowing) {
        this.owed = Math.min(this.owed + 1, MOST);
      }
      if (this.last && encoder.stdin.writableLength <= BACKLOG * this.size) encoder.stdin.write(this.last);
    }
  }

  private stopPump(): void {
    if (this.pump) clearInterval(this.pump);
    this.pump = null;
  }

  private clearPictures(): void {
    this.filling = null;
    this.filled = 0;
    this.held = [];
    this.spare = [];
    this.last = null;
    this.flowing = false;
    this.owed = 0;
  }

  // Ends the stream, and says how it ended.
  private finish(message: string, failed: boolean): void {
    if (!this.active) return;
    this.active = false;
    if (this.watchdog) clearInterval(this.watchdog);
    this.watchdog = null;
    this.stopPump();
    this.stopDecoder();
    const child = this.encoder;
    this.encoder = null;
    child?.kill('SIGKILL');
    this.closeBrowser();
    this.feed?.stop();
    this.feed = null;
    this.recording = null;
    this.session = '';
    this.clearPictures();
    this.status = { ...this.status, state: failed ? 'error' : 'off', message, fps: 0, frozen: 0 };
    const log = failed ? this.logger.error : this.logger.info;
    log.call(this.logger, `stream: ${message}`, { timestamp: true });
  }

  // Once a second: the first pictures not coming, a page gone, or the
  // picture standing still.
  private check(): void {
    if (!this.active || this.status.state === 'stopping') return;
    const now = Date.now();
    const hidden = this.status.drawnBy === 'server';
    if (!this.encoder) {
      const limit = hidden ? FIRST_HIDDEN : FIRST;
      if (now - this.started > limit * 1000) this.finish(`${hidden ? 'The hidden browser' : 'The page'} sent no picture for ${limit} s.`, true);
      return;
    }
    if (!hidden && now - this.lastPiece > PAGE_GONE * 1000) {
      this.logger.warn(`stream: nothing from the page for ${PAGE_GONE} s, ending the stream`, { timestamp: true });
      return this.stop();
    }
    const still = Math.floor((now - this.lastPicture) / 1000);
    const was = this.status.frozen;
    if (still >= FROZEN) {
      this.status.frozen = still;
      this.status.fps = 0;
      if (was < FROZEN_SAY && still >= FROZEN_SAY) {
        const page = hidden ? 'the hidden page' : 'the page';
        this.logger.warn(`stream: no new picture for ${still} s; the stream shows the last one until ${page} draws again`, { timestamp: true });
      }
    } else if (was) {
      this.status.frozen = 0;
      if (was >= FROZEN_SAY) this.logger.info(`stream: new pictures again, after ${was} s`, { timestamp: true });
    }
  }

  // A line of the encoder's progress: what went out.
  private progress(line: string): void {
    const at = line.indexOf('=');
    if (at < 0) return;
    const name = line.slice(0, at).trim();
    const value = line.slice(at + 1).trim();
    if (name !== 'progress') {
      this.report.set(name, value);
      return;
    }
    // A report ends with progress=, twice a second.
    const number = (key: string) => Number.parseFloat(this.report.get(key) ?? '') || 0;
    const status = this.status;
    status.frames = number('frame');
    status.kbps = Math.round(number('bitrate'));
    status.seconds = Math.max(0, Math.round(number('out_time_us') / 1e6));
    if (status.frames > 0 && status.state === 'starting') {
      status.state = 'live';
      this.logger.info('stream: live', { timestamp: true });
    }
    this.report.clear();
  }

  // A line of the decoder's progress: the new pictures a second over the
  // last reports. The pictures it made again to keep 30 a second, because
  // drawing fell behind, don't count.
  private decoderProgress(line: string, report: Map<string, string>): void {
    const at = line.indexOf('=');
    if (at < 0) return;
    const name = line.slice(0, at).trim();
    if (name !== 'progress') {
      report.set(name, line.slice(at + 1).trim());
      return;
    }
    const number = (key: string) => Number.parseFloat(report.get(key) ?? '') || 0;
    const now = Date.now();
    const fresh = number('frame') - number('dup_frames');
    this.marks = [...this.marks, { at: now, fresh }].slice(-FPS_REPORTS);
    const first = this.marks[0];
    // (Over a short time ffmpeg can make more than 30 a second, catching up.)
    if (now > first.at && !this.status.frozen) this.status.fps = Math.min(FPS, Math.max(0, (fresh - first.fresh) / ((now - first.at) / 1000)));
    report.clear();
  }

  // The first encoder on the graphics card that works with this ffmpeg here,
  // or null; looked for once, when a stream first asks for one. Of those this
  // ffmpeg has, each that doesn't work says why in the terminal (NVENC: the
  // driver it needs).
  private gpuEncoder(): Promise<Encoder | null> {
    this.gpuSearch ??= (async () => {
      const present = await listEncoders(this.ffmpeg);
      if (present.size === 0) return null; // no ffmpeg at all; starting it says so
      for (const encoder of GPU_ENCODERS.filter(({ name }) => present.has(name))) {
        const problem = await tryEncoder(this.ffmpeg, encoder);
        if (problem === null) {
          this.logger.info(`stream: encoding on the graphics card with ${encoder.name}`, { timestamp: true });
          return encoder;
        }
        this.logger.info(`stream: ${encoder.name} doesn't work here: ${problem}`, { timestamp: true });
      }
      this.logger.warn('stream: no encoder on the graphics card works with this ffmpeg; encoding on the CPU (libx264)', {
        timestamp: true,
      });
      return null;
    })();
    return this.gpuSearch;
  }

  private openBrowser(width: number, height: number, gpu: boolean, scene: string): void {
    const path = findBrowser();
    if (!path) return this.finish('No Chrome, Edge or Chromium was found to draw the stream. Install one, or set STREAM_BROWSER.', true);
    const page = this.server.resolvedUrls?.local[0];
    if (!page) return this.finish('The dev server has no address yet.', true);
    const url = new URL(page);
    const params = new URLSearchParams(scene);
    params.set('renderer', `${width}x${height}`);
    url.search = params.toString();
    const how = gpu ? 'on the graphics card' : 'in software';
    this.logger.info(`stream: drawing in a hidden ${basename(path)}, ${how}`, { timestamp: true });
    const browser = new HiddenBrowser(path, url.href, width, height, gpu, (why) => {
      if (this.browser === browser) this.finish(why, true);
    });
    this.browser = browser;
  }

  private closeBrowser(): void {
    this.browser?.close();
    this.browser = null;
  }
}

// The hidden (headless) browser that draws the stream when the dev server
// does, in a profile of its own that is deleted when it closes. On Windows it
// also closes when the dev server's process ends however it ends: Node keeps
// its children in a job that closes with it.
class HiddenBrowser {
  private closing = false;
  private readonly child: ChildProcess;

  constructor(path: string, url: string, width: number, height: number, gpu: boolean, onExit: (why: string) => void) {
    const profile = mkdtempSync(join(tmpdir(), 'animation-stream-'));
    const flags = [
      ['--headless', `--user-data-dir=${profile}`, `--window-size=${width},${height}`],
      ['--no-first-run', '--no-default-browser-check', '--disable-extensions', '--disable-sync', '--mute-audio', '--hide-scrollbars'],
      // Drawing at full speed, although nobody looks.
      ['--disable-background-timer-throttling', '--disable-renderer-backgrounding', '--disable-backgrounding-occluded-windows'],
      // Without the graphics card: drawing in software, and the recording
      // encoded on the CPU.
      gpu ? [] : ['--disable-gpu', '--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--disable-accelerated-video-encode'],
      url.startsWith('https:') ? ['--ignore-certificate-errors'] : [], // a dev server's own certificate
      (process.env.STREAM_BROWSER_ARGS ?? '').split(/\s+/).filter(Boolean),
      [url],
    ].flat();
    this.child = spawn(path, flags, { stdio: 'ignore', windowsHide: true });
    this.child.on('error', (error) => {
      if (!this.closing) onExit(`The hidden browser could not start: ${error.message}`);
    });
    this.child.on('exit', (code, signal) => {
      // Its helpers may hold the profile's files a moment longer.
      void rm(profile, { recursive: true, force: true, maxRetries: 10, retryDelay: 300 }).catch(() => {});
      if (!this.closing) onExit(`The hidden browser closed (${signal ?? `code ${code}`}).`);
    });
  }

  close(): void {
    this.closing = true;
    this.child.kill();
  }
}

// Chrome, Edge or Chromium, where they install; STREAM_BROWSER picks one.
function findBrowser(): string | null {
  const named = process.env.STREAM_BROWSER;
  if (named) return named;
  let candidates: string[];
  if (process.platform === 'win32') {
    const roots = ['PROGRAMFILES', 'PROGRAMFILES(X86)', 'LOCALAPPDATA'].map((name) => process.env[name]).filter(Boolean) as string[];
    candidates = ['Google/Chrome/Application/chrome.exe', 'Microsoft/Edge/Application/msedge.exe'].flatMap((app) =>
      roots.map((root) => join(root, app)),
    );
  } else if (process.platform === 'darwin') {
    candidates = ['Google Chrome', 'Microsoft Edge', 'Chromium'].map((app) => `/Applications/${app}.app/Contents/MacOS/${app}`);
  } else {
    const names = ['google-chrome', 'google-chrome-stable', 'chromium', 'chromium-browser', 'microsoft-edge', 'microsoft-edge-stable'];
    const dirs = (process.env.PATH ?? '').split(delimiter).filter(Boolean);
    candidates = names.flatMap((name) => dirs.map((dir) => join(dir, name)));
  }
  return candidates.find((path) => existsSync(path)) ?? null;
}

// Why an encoder doesn't work here, or null when it does: it encodes a
// second of test picture the way it would encode the stream.
function tryEncoder(ffmpeg: string, encoder: Encoder): Promise<string | null> {
  const args = [
    ['-hide_banner', '-loglevel', 'error'],
    encoder.device ?? [],
    ['-f', 'lavfi', '-i', `testsrc2=s=640x360:r=${FPS}`, '-frames:v', String(FPS)],
    ['-vf', [...fitting(640, 360, encoder.format), ...(encoder.upload ? ['hwupload'] : [])].join(',')],
    encoding(encoder, 1000),
    ['-f', 'flv', 'pipe:1'],
  ].flat();
  return new Promise((resolve) => {
    const child = spawn(ffmpeg, args, { stdio: ['ignore', 'ignore', 'pipe'], windowsHide: true });
    let words = '';
    child.stderr.on('data', (data: Buffer) => (words += data.toString('utf8')));
    const timer = setTimeout(() => child.kill('SIGKILL'), PROBE_WAIT * 1000);
    child.on('error', (error) => {
      clearTimeout(timer);
      resolve(error.message);
    });
    child.on('exit', (code) => {
      clearTimeout(timer);
      // ffmpeg's first lines say why; the ones after them only that it stopped.
      const why = words
        .split(/\r?\n/)
        .map((line) => line.replace(/^(\[[^\]]*\]\s*)+/, '').trim())
        .filter((line) => line && !/opening encoder|sending frames|Task finished|Terminating thread|Could not open|Nothing was written/.test(line))
        .slice(0, 2)
        .join(' ');
      resolve(code === 0 ? null : why || `ffmpeg stopped (code ${code}).`);
    });
  });
}

// The names of the encoders this ffmpeg has; none when there is no ffmpeg.
function listEncoders(ffmpeg: string): Promise<Set<string>> {
  return new Promise((resolve) => {
    const child = spawn(ffmpeg, ['-hide_banner', '-encoders'], { stdio: ['ignore', 'pipe', 'ignore'], windowsHide: true });
    let text = '';
    child.stdout.on('data', (data: Buffer) => (text += data.toString('utf8')));
    child.on('error', () => resolve(new Set()));
    // " V....D libx264   libx264 H.264 / AVC …": video encoders start with V.
    child.on('exit', () => resolve(new Set([...text.matchAll(/^\s*V\S{5}\s+(\S+)/gm)].map((match) => match[1]))));
  });
}

// Waits until a stream can take more, or has closed.
function drained(stream: Writable): Promise<void> {
  return new Promise((resolve) => {
    const done = () => {
      stream.off('drain', done);
      stream.off('close', done);
      resolve();
    };
    stream.on('drain', done);
    stream.on('close', done);
  });
}

function idle(message: string): StreamStatus {
  return {
    state: 'off',
    message,
    drawnBy: 'page',
    gpu: false,
    graphics: '',
    encoder: '',
    scene: '',
    frames: 0,
    fps: 0,
    frozen: 0,
    kbps: 0,
    seconds: 0,
    audio: null,
  };
}

// A start request, or why it can't be served.
function readStart(body: unknown): Omit<StartRequest, 'audio'> | string {
  const { url, key, quality, drawnBy = 'page', gpu = false, scene = '' } = (body ?? {}) as Record<string, unknown>;
  if (typeof url !== 'string' || typeof key !== 'string' || typeof quality !== 'string') return 'Send url, key and quality.';
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return `"${url}" is not a URL.`;
  }
  if (parsed.protocol !== 'rtmp:' && parsed.protocol !== 'rtmps:') return 'The stream URL must start with rtmp:// or rtmps://.';
  if (!parsed.hostname || /\s/.test(url)) return 'The stream URL has no server in it.';
  if (!/^[A-Za-z0-9_-]{1,200}$/.test(key)) return 'The stream key is letters, digits, - and _ only, as YouTube gives it.';
  if (!Object.hasOwn(QUALITIES, quality)) return `The quality is one of ${Object.keys(QUALITIES).join(', ')}.`;
  if (drawnBy !== 'page' && drawnBy !== 'server') return 'drawnBy is page or server.';
  if (typeof gpu !== 'boolean') return 'gpu is true or false.';
  if (typeof scene !== 'string' || scene.length > MAX_SCENE) return `scene is a query string of at most ${MAX_SCENE} characters.`;
  return { url, key, quality: quality as Quality, drawnBy, gpu, scene: scene.replace(/^\?/, '') };
}

// A change for the hidden browser, or why it can't be passed on.
function readFollow(body: unknown): Follow | string {
  const { scene, behaviour, restart, walk } = (body ?? {}) as Record<string, unknown>;
  const change: Follow = {};
  if (scene !== undefined) {
    if (typeof scene !== 'string' || scene.length > MAX_SCENE) return `scene is a query string of at most ${MAX_SCENE} characters.`;
    change.scene = scene.replace(/^\?/, '');
  }
  if (behaviour !== undefined) {
    const { name, args } = (behaviour ?? {}) as Record<string, unknown>;
    const texts = Array.isArray(args) && args.length <= 20 && args.every((arg) => typeof arg === 'string' && arg.length <= 4000);
    if (typeof name !== 'string' || name.length > 100 || !texts) return 'behaviour is { name, args }, args a list of texts.';
    change.behaviour = { name, args: args as string[] };
  }
  if (restart !== undefined) {
    if (restart !== true) return 'restart is true.';
    change.restart = true;
  }
  if (walk !== undefined) {
    const moves = Array.isArray(walk) && walk.length <= 12 && walk.every((move) => typeof move === 'string' && move.length <= 20);
    if (!moves) return 'walk is a list of moves held.';
    change.walk = walk as string[];
  }
  if (Object.keys(change).length === 0) return 'Send scene, behaviour, restart or walk.';
  return change;
}

function route(req: Connect.IncomingMessage, res: ServerResponse, stream: Stream): void {
  const answer = (status: number, text = '') => {
    res.statusCode = status;
    res.setHeader('Content-Type', 'text/plain; charset=utf-8');
    res.end(text && `${text}\n`);
  };
  const path = (req.url ?? '').split('?')[0];

  if (req.method === 'GET' && path === '/status') {
    res.setHeader('Content-Type', 'application/json');
    res.setHeader('Cache-Control', 'no-store');
    res.end(JSON.stringify(stream.current));
    return;
  }
  if (req.method === 'GET' && path === '/preview') {
    if (!stream.preview) return answer(404, 'No picture yet.');
    res.setHeader('Content-Type', 'image/jpeg');
    res.setHeader('Cache-Control', 'no-store');
    res.end(stream.preview);
    return;
  }
  if (req.method !== 'POST') return answer(405, 'POST to /start, /recording, /chunk, /follow, /audio, /preview, /log or /stop; GET /status or /preview');

  const limit = { '/chunk': MAX_CHUNK, '/preview': MAX_PREVIEW, '/log': MAX_LOG }[path] ?? MAX_JSON;
  const pieces: Buffer[] = [];
  let size = 0;
  req.on('data', (piece: Buffer) => {
    size += piece.length;
    if (size <= limit) pieces.push(piece);
  });
  req.on('end', () => {
    if (size > limit) return answer(413, `at most ${limit} bytes here`);
    const body = Buffer.concat(pieces);
    const json = (): unknown => {
      try {
        return JSON.parse(body.toString('utf8'));
      } catch {
        return undefined;
      }
    };
    const refused = (error: unknown) => answer(409, (error as Error).message);
    switch (path) {
      case '/start': {
        if (stream.running) return answer(409, 'A stream is already running. Stop it first.');
        const body = json();
        const request = readStart(body);
        if (typeof request === 'string') return answer(400, request);
        const asked = ((body ?? {}) as { audio?: unknown }).audio;
        void (asked === undefined ? Promise.resolve(SILENCE) : stream.checkAudio(asked)).then((audio) => {
          // Another start may have come while the sound was checked.
          if (stream.running) return answer(409, 'A stream is already running. Stop it first.');
          stream.start({ ...request, audio }).then(
            (session) => {
              res.statusCode = 202;
              res.setHeader('Content-Type', 'application/json');
              res.end(JSON.stringify({ session }));
            },
            (error: Error) => answer(500, error.message),
          );
        });
        return;
      }
      case '/audio':
        void stream.checkAudio(json()).then((audio) => {
          if (typeof audio === 'string') return answer(400, audio);
          try {
            stream.setAudio(audio);
            answer(204);
          } catch (error) {
            refused(error);
          }
        });
        return;
      case '/recording': {
        const { id, graphics, session = '' } = (json() ?? {}) as Record<string, unknown>;
        if (typeof id !== 'string' || !/^[\w-]{1,64}$/.test(id) || typeof graphics !== 'string' || typeof session !== 'string') {
          return answer(400, 'Send JSON: { id, graphics, session }.');
        }
        try {
          stream.announce(id, graphics.slice(0, 200), session);
          return answer(204);
        } catch (error) {
          return refused(error);
        }
      }
      case '/chunk':
        stream.write(body, String(req.headers['x-recording'] ?? '')).then(() => answer(204), refused);
        return;
      case '/follow': {
        const change = readFollow(json());
        if (typeof change === 'string') return answer(400, change);
        try {
          stream.follow(change);
          return answer(204);
        } catch (error) {
          return refused(error);
        }
      }
      case '/preview':
        if (!stream.drawnHere) return answer(409, 'The dev server is not drawing a stream.');
        stream.preview = body;
        return answer(204);
      case '/log':
        stream.log(body.toString('utf8'));
        return answer(204);
      case '/stop':
        stream.stop();
        return answer(204);
      default:
        return answer(404);
    }
  });
}
