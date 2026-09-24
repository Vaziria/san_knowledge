import { spawn, type ChildProcess, type ChildProcessWithoutNullStreams } from 'node:child_process';
import { existsSync, mkdtempSync } from 'node:fs';
import { rm } from 'node:fs/promises';
import type { ServerResponse } from 'node:http';
import { tmpdir } from 'node:os';
import { basename, delimiter, join } from 'node:path';
import type { Writable } from 'node:stream';
import type { Connect, Logger, Plugin, ViteDevServer } from 'vite';

// Live streaming the preview to YouTube (or any RTMP server). Browsers can't
// speak RTMP, so a page records the scene's canvas (src/stream.ts) and posts
// the recording here in short pieces, and this plugin pipes them into ffmpeg,
// which scales the picture to 720p or 1080p at 30 frames a second, encodes it
// as H.264 with a keyframe every 2 s, adds a silent stereo track (YouTube
// wants audio) and pushes it to the RTMP server as FLV. One stream at a time.
//
// Who draws the stream, as the Stream tab's Render option says:
// - the page (web + stream): the panel's own page records what it shows.
// - the dev server (stream only): this plugin starts a hidden (headless)
//   Chrome or Edge, which opens the page as the stream's renderer
//   (?renderer=1280x720 and the panel's settings: no panel, drawn at the
//   stream's size; src/streamRenderer.ts) and records it. The panel steers it
//   through /follow. It streams until /stop, whether or not the panel's page
//   stays open.
// With `gpu`, ffmpeg encodes on the graphics card (the first of
// GPU_ENCODERS that works here) and the hidden browser draws on it; without,
// both run on the CPU (x264, and SwiftShader for the drawing).
//
// - POST /__stream/start, JSON { url, key, quality, drawnBy, gpu, scene }:
//   starts ffmpeg, and the hidden browser for drawnBy "server", which draws
//   `scene` (the panel's settings as a query string). Answered 202; 400 with
//   the reason for a bad URL, key or quality, 409 while a stream runs. The URL
//   must be rtmp:// or rtmps:// (YouTube's is rtmp://a.rtmp.youtube.com/live2),
//   so a request can never make ffmpeg write a file here; the key goes after
//   it, as YouTube's stream URL + key.
// - POST /__stream/recording, JSON { id, graphics }: a page starts recording,
//   drawn on `graphics` (its WebGL renderer). A new recording from the hidden
//   browser (a code change reloaded its page) starts ffmpeg again, since one
//   recording can't continue another: YouTube sees the stream reconnect.
//   Answered 204, or 409.
// - POST /__stream/chunk, the next piece of the recording
//   (application/octet-stream, header X-Recording: its id), answered 204 once
//   ffmpeg has taken it, or 409 when no stream takes it.
// - POST /__stream/follow, JSON { scene } or { behaviour: { name, args } } or
//   { restart: true }: passed on to the hidden browser as the Vite event
//   'stream:follow'. 204, or 409 when the dev server draws no stream.
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
// test's --remote-debugging-port). If no piece comes for a while (the page
// closed or reloaded, the hidden browser stuck), the stream ends by itself.

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
  kbps: number; // the bitrate going out
  seconds: number; // of stream sent
}

// A change for the hidden browser: the panel's settings, a behaviour run, or
// its demo restarted. src/stream.ts has the same shape.
export interface Follow {
  scene?: string;
  behaviour?: { name: string; args: string[] };
  restart?: true;
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
}

const FPS = 30;
const KEYFRAME = 2 * FPS; // frames from one keyframe to the next; YouTube asks for one at least every 4 s
const AUDIO_KBPS = 128;
const MAX_JSON = 16 * 1024; // bytes
const MAX_CHUNK = 16 * 1024 * 1024; // bytes; a piece is a quarter second of video
const MAX_PREVIEW = 2 * 1024 * 1024; // bytes of JPEG
const MAX_LOG = 4 * 1024; // bytes
const MAX_SCENE = 2048; // characters of query string
const STALL = 15; // s without a piece from the page before the stream ends
const STALL_HIDDEN = 40; // the same for the hidden browser, which also needs time to start and to reload
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

// ffmpeg's options for the picture: scaled to fit the frame (bars where the
// drawing's shape differs), in the encoder's pixels, at a steady 30 frames a
// second (a picture that comes late is sent again, and counted in
// dup_frames), with a keyframe every 2 s.
function videoOptions(encoder: Encoder, width: number, height: number, kbps: number): string[] {
  const filters = [
    `scale=${width}:${height}:force_original_aspect_ratio=decrease`,
    `pad=${width}:${height}:(ow-iw)/2:(oh-ih)/2`,
    `format=${encoder.format}`,
    ...(encoder.upload ? ['hwupload'] : []),
  ];
  return [
    ['-vf', filters.join(','), '-r', String(FPS), '-fps_mode', 'cfr'],
    ['-c:v', encoder.name, ...encoder.options],
    ['-b:v', `${kbps}k`, '-maxrate', `${kbps}k`, '-bufsize', `${2 * kbps}k`, '-g', String(KEYFRAME)],
  ].flat();
}

export function streamBridge(ffmpeg = process.env.FFMPEG_PATH ?? 'ffmpeg'): Plugin {
  return {
    name: 'stream-bridge',
    apply: 'serve',
    configureServer(server) {
      const stream = new Stream(ffmpeg, server);
      server.middlewares.use('/__stream', (req, res) => route(req, res, stream));
      server.httpServer?.once('close', () => stream.stop(true));
    },
  };
}

// The one stream: its ffmpeg, and its hidden browser when the dev server
// draws it.
class Stream {
  private active = false; // from start until the stream has ended
  private process: ChildProcessWithoutNullStreams | null = null; // ffmpeg
  private args: string[] = []; // ffmpeg's, to start it again
  private browser: HiddenBrowser | null = null;
  private secret = ''; // the key, to mask
  private status: StreamStatus = idle('');
  private recording: string | null = null; // the id of the recording ffmpeg takes
  private readonly retired = new Set<string>(); // recordings that ended; late pieces of theirs are dropped
  private lastPiece = 0; // ms, when a piece last came
  private watchdog: NodeJS.Timeout | null = null;
  private lastWords: string[] = [];
  private earlier = 0; // s streamed before ffmpeg last started again
  private report = new Map<string, string>(); // ffmpeg's progress report being read
  private marks: { at: number; fresh: number }[] = []; // for the frame rate
  private gpuSearch: Promise<Encoder | null> | null = null;
  private readonly logger: Logger;
  preview: Buffer | null = null; // the hidden browser's latest picture

  constructor(
    private readonly ffmpeg: string,
    private readonly server: ViteDevServer,
  ) {
    this.logger = server.config.logger;
  }

  get running(): boolean {
    return this.active;
  }

  // The dev server draws the stream in its hidden browser.
  get drawnHere(): boolean {
    return this.active && this.status.drawnBy === 'server' && this.status.state !== 'stopping';
  }

  get current(): StreamStatus {
    return this.status;
  }

  // Starts ffmpeg pushing the recording to the URL and key, already checked
  // (readStart()), and for drawnBy "server" the hidden browser that draws it.
  // How it goes shows in `current`.
  async start({ url, key, quality, drawnBy, gpu, scene }: StartRequest): Promise<void> {
    const { width, height, kbps } = QUALITIES[quality];
    this.active = true;
    this.secret = key;
    this.recording = null;
    this.retired.clear();
    this.preview = null;
    this.earlier = 0;
    this.status = { ...idle(''), state: 'starting', drawnBy, gpu, scene: drawnBy === 'server' ? scene : '' };
    this.lastPiece = Date.now();

    const encoder = (gpu && (await this.gpuEncoder())) || X264;
    if (!this.active || this.status.state !== 'starting') return; // stopped while looking for an encoder
    this.status.encoder = encoder.name;
    const target = `${url.replace(/\/+$/, '')}/${key}`;
    this.args = [
      // Only errors: a live recording's timestamps make routine warnings that
      // would flood the terminal. Progress comes as key=value lines on stdout.
      ['-hide_banner', '-loglevel', 'error', '-nostats', '-progress', 'pipe:1'],
      encoder.device ?? [],
      // The recording, as it comes (WebM is Matroska).
      ['-f', 'matroska', '-thread_queue_size', '1024', '-i', 'pipe:0'],
      // Silence, since YouTube wants an audio track.
      ['-f', 'lavfi', '-i', 'anullsrc=channel_layout=stereo:sample_rate=44100'],
      ['-map', '0:v:0', '-map', '1:a:0'],
      videoOptions(encoder, width, height, kbps),
      ['-c:a', 'aac', '-b:a', `${AUDIO_KBPS}k`, '-ar', '44100'],
      ['-shortest', '-f', 'flv', target],
    ].flat();
    const drawer = drawnBy === 'server' ? 'the dev server' : 'the page';
    this.logger.info(`stream: starting ${width}x${height} to ${this.mask(target)}, drawn by ${drawer}, encoded with ${encoder.name}`, {
      timestamp: true,
    });
    this.startFfmpeg();
    this.watchdog = setInterval(() => this.checkPieces(), 1000);
    if (drawnBy === 'server') this.openBrowser(width, height, gpu, scene);
  }

  // A page starts recording. Only one recording goes to ffmpeg; a new one from
  // the hidden browser, whose page a code change reloaded, starts ffmpeg again.
  announce(id: string, graphics: string): void {
    if (!this.process || this.status.state === 'stopping') throw new Error('No stream is running.');
    if (id === this.recording) return;
    if (this.retired.has(id)) throw new Error('That recording has ended.');
    if (this.recording) {
      if (this.status.drawnBy === 'page') throw new Error('Another page is streaming already. Stop it first.');
      this.retired.add(this.recording);
      this.logger.info('stream: the hidden browser loaded the page again; reconnecting', { timestamp: true });
      this.restartFfmpeg();
    }
    this.recording = id;
    this.lastPiece = Date.now();
    this.status.graphics = graphics;
  }

  // Hands a piece of the recording to ffmpeg, once it can take it.
  async write(piece: Buffer, id: string): Promise<void> {
    const child = this.process;
    if (!child || this.status.state === 'stopping') throw new Error('No stream is running.');
    if (id !== this.recording) throw new Error(this.retired.has(id) ? 'That recording has ended.' : 'Announce the recording first.');
    this.lastPiece = Date.now();
    if (!child.stdin.write(piece)) await drained(child.stdin);
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

  // Ends the stream: ffmpeg finishes what it has, or is killed at once
  // (`now`) or after STOP_WAIT. The hidden browser closes at once.
  stop(now = false): void {
    if (!this.active) return;
    this.closeBrowser();
    const child = this.process;
    if (!child) return this.finish('Stopped.', false); // still looking for an encoder
    this.status = { ...this.status, state: 'stopping' };
    child.stdin.end();
    const kill = () => {
      if (this.process === child) child.kill('SIGKILL');
    };
    if (now) kill();
    else setTimeout(kill, STOP_WAIT * 1000).unref();
  }

  // Text with the stream key hidden.
  mask(text: string): string {
    return this.secret ? text.split(this.secret).join('••••') : text;
  }

  private startFfmpeg(): void {
    this.lastWords = [];
    this.report.clear();
    this.marks = [];
    const child = spawn(this.ffmpeg, this.args, { stdio: 'pipe', windowsHide: true });
    this.process = child;

    child.on('error', (error) => {
      if (this.process !== child) return;
      this.process = null;
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
      if (this.process !== child) return;
      pending += data.toString('utf8');
      const lines = pending.split(/\r?\n/);
      pending = lines.pop() ?? '';
      for (const line of lines) this.progress(line);
    });
    let words = '';
    child.stderr.on('data', (data: Buffer) => {
      if (this.process !== child) return;
      words += data.toString('utf8');
      const lines = words.split(/\r?\n/);
      words = lines.pop() ?? '';
      for (const line of lines.map((l) => this.mask(l.trim())).filter(Boolean)) {
        // A recording cut off as the stream ends (the hidden browser closes
        // at once) is how it ends, not an error.
        if (this.status.state === 'stopping' && /ended prematurely/i.test(line)) continue;
        this.lastWords = [...this.lastWords, line].slice(-ERROR_LINES);
        this.logger.error(`stream: ffmpeg: ${line}`, { timestamp: true });
      }
    });
    child.on('exit', (code, signal) => {
      if (this.process !== child) return; // replaced, or ended on purpose
      this.process = null;
      const stopping = this.status.state === 'stopping';
      if (code === 0 || stopping) this.finish(stopping ? 'Stopped.' : 'The stream ended.', false);
      else this.finish(this.lastWords.at(-1) ?? `ffmpeg stopped (${signal ?? `code ${code}`}).`, true);
    });
  }

  // ffmpeg anew, for a new recording: the old one is killed, and the stream
  // reconnects.
  private restartFfmpeg(): void {
    const old = this.process;
    this.process = null; // so its exit is not taken for the stream's end
    old?.kill('SIGKILL');
    this.earlier = this.status.seconds;
    this.status = { ...this.status, state: 'starting', fps: 0, kbps: 0 };
    this.startFfmpeg();
  }

  // Ends the stream, and says how it ended.
  private finish(message: string, failed: boolean): void {
    if (!this.active) return;
    this.active = false;
    if (this.watchdog) clearInterval(this.watchdog);
    this.watchdog = null;
    const child = this.process;
    this.process = null;
    child?.kill('SIGKILL');
    this.closeBrowser();
    this.recording = null;
    this.status = { ...this.status, state: failed ? 'error' : 'off', message, fps: 0 };
    const log = failed ? this.logger.error : this.logger.info;
    log.call(this.logger, `stream: ${message}`, { timestamp: true });
  }

  // Ends the stream if the pictures stopped coming.
  private checkPieces(): void {
    if (!this.active || this.status.state === 'stopping') return;
    const hidden = this.status.drawnBy === 'server';
    const limit = hidden ? STALL_HIDDEN : STALL;
    if (Date.now() - this.lastPiece <= limit * 1000) return;
    if (hidden) {
      this.finish(`The hidden browser sent no picture for ${limit} s.`, true);
    } else {
      this.logger.warn(`stream: nothing from the page for ${limit} s, ending the stream`, { timestamp: true });
      this.stop();
    }
  }

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
    status.seconds = this.earlier + Math.max(0, Math.round(number('out_time_us') / 1e6));
    // New pictures a second over the last reports: the frames ffmpeg sent
    // again to keep 30 a second, because drawing fell behind, don't count.
    const now = Date.now();
    const fresh = status.frames - number('dup_frames');
    this.marks = [...this.marks, { at: now, fresh }].slice(-FPS_REPORTS);
    const first = this.marks[0];
    // (Over a short time ffmpeg can send more than 30 a second, catching up.)
    if (now > first.at) status.fps = Math.min(FPS, Math.max(0, (fresh - first.fresh) / ((now - first.at) / 1000)));
    if (status.frames > 0 && status.state === 'starting') {
      status.state = 'live';
      this.logger.info('stream: live', { timestamp: true });
    }
    this.report.clear();
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
    videoOptions(encoder, 640, 360, 1000),
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
    kbps: 0,
    seconds: 0,
  };
}

// A start request, or why it can't be served.
function readStart(body: unknown): StartRequest | string {
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
  const { scene, behaviour, restart } = (body ?? {}) as Record<string, unknown>;
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
  if (Object.keys(change).length === 0) return 'Send scene, behaviour or restart.';
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
  if (req.method !== 'POST') return answer(405, 'POST to /start, /recording, /chunk, /follow, /preview, /log or /stop; GET /status or /preview');

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
        const request = readStart(json());
        if (typeof request === 'string') return answer(400, request);
        stream.start(request).then(
          () => answer(202),
          (error: Error) => answer(500, error.message),
        );
        return;
      }
      case '/recording': {
        const { id, graphics } = (json() ?? {}) as Record<string, unknown>;
        if (typeof id !== 'string' || !/^[\w-]{1,64}$/.test(id) || typeof graphics !== 'string') {
          return answer(400, 'Send JSON: { id, graphics }.');
        }
        try {
          stream.announce(id, graphics.slice(0, 200));
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
