import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';
import { settings, settingsFromQuery, settingsQuery } from './settings';
import type { Stage } from './Stage';
import { audioChoice, audioOptions, setAudio, type AudioChoice } from './audio';
import { walking } from './walk';

// Live streaming the scene to YouTube, for the panel's Stream tab. A page
// records the scene's canvas (only the scene: the panel is not in it) with
// MediaRecorder and posts the recording to the dev server in quarter-second
// pieces, one after another so they arrive in order; the dev server pipes
// them into ffmpeg, which pushes them to YouTube over RTMP (stream-bridge.ts).
//
// The Stream tab's Render option says where the scene is drawn:
// - web + stream: here, and this page's drawing is what streams. The canvas
//   draws only while the tab is shown: a hidden tab freezes the stream. A
//   reload (a code change) carries on: the page keeps the stream's session in
//   sessionStorage, which only this tab has, and records again as it loads;
//   the stream shows its last picture meanwhile. Closed for a minute, the
//   stream ends.
// - stream only: not here. The dev server draws it in a hidden browser that
//   opens this page as the stream's renderer (streamRenderer.ts), and this
//   page steers it: its settings, behaviours and walking go there (follow).
//   It streams until Stop, whether or not this page stays open.
// - web only: here, and nothing streams.
// The GPU switch has the dev server's side use the graphics card: ffmpeg's
// encoding, and the hidden browser's drawing. The sound is the Audio tab's
// (audio.ts): the dev server plays it into the stream (audio-bridge.ts), and
// each change while live goes there too.
//
// Streaming is an action, like the behaviours, not a setting: nothing of it
// is in the URL. Render and GPU are kept in this browser, and the Stream tab
// keeps the URL, the key and the quality. The status comes from the dev
// server once a second while a stream runs. It only works under `npm run
// dev`, which has the bridge.

// How the stream goes, as the dev server reports it; stream-bridge.ts has
// the same shapes.
export type StreamState = 'off' | 'starting' | 'live' | 'stopping' | 'error';
export type DrawnBy = 'page' | 'server'; // this page, or the dev server's hidden browser
export interface StreamStatus {
  state: StreamState;
  message: string; // what went wrong, or how it ended
  drawnBy: DrawnBy;
  gpu: boolean; // the graphics card was asked for
  graphics: string; // what draws the pictures: the recording page's WebGL renderer
  encoder: string; // ffmpeg's video encoder
  scene: string; // what the hidden browser draws: the settings as a query string
  frames: number; // sent so far
  fps: number; // new pictures a second, not counting the repeats that keep it at 30
  frozen: number; // s the last picture has been sent again for, as no new one came; 0 while they come
  kbps: number; // the bitrate going out
  seconds: number; // of stream sent
  audio: { choice: AudioChoice; playing: string; problem: string } | null; // its sound, while it streams
}

// A change for the dev server's hidden browser: the settings (as a query
// string), a behaviour run, the demo restarted, or what is held down to walk
// the camera (walk.ts).
export interface Follow {
  scene?: string;
  behaviour?: { name: string; args: string[] };
  restart?: true;
  walk?: string[];
}

// The sizes the dev server streams at (QUALITIES in stream-bridge.ts).
export const QUALITY_NAMES = ['720p', '1080p'] as const;
export type Quality = (typeof QUALITY_NAMES)[number];

// Where the scene is drawn.
export const RENDERS = [
  { value: 'both', label: 'web + stream' },
  { value: 'stream', label: 'stream only' },
  { value: 'web', label: 'web only' },
] as const;
export type Render = (typeof RENDERS)[number]['value'];

// YouTube's stream URL, as YouTube Studio shows it (Go live, Stream).
export const YOUTUBE_URL = 'rtmp://a.rtmp.youtube.com/live2';

const FPS = 30;
const PIECE_MS = 250; // how much recording goes in each post
const RECORD_BPS = 8_000_000; // the recording's bitrate to the dev server; ffmpeg encodes it again for YouTube
// What the recording is encoded as: the first the browser can do. H.264 is
// often done by the graphics card, which leaves more time for drawing.
const TYPES = ['video/webm;codecs=h264', 'video/webm;codecs=vp8', 'video/webm'];
const NO_RECORDER = 'This browser cannot record the canvas (no WebM MediaRecorder).';
const POLL_MS = 1000;
const RENDER_KEY = 'animation-stream-render';
const GPU_KEY = 'animation-stream-gpu';
const SESSION_KEY = 'animation-stream-session'; // the stream this tab draws, in its sessionStorage
const tab = () => sessionStorage; // this tab's own storage, which a reload keeps

export interface StreamOptions {
  render: Render;
  gpu: boolean;
}

export const streamOptions = createStore<StreamOptions>()(() => ({
  render: RENDERS.find((r) => r.value === stored(RENDER_KEY))?.value ?? 'both',
  gpu: stored(GPU_KEY) !== 'off',
}));

export function useStreamOptions<T>(select: (options: StreamOptions) => T): T {
  return useStore(streamOptions, select);
}

export function setRender(render: Render): void {
  streamOptions.setState({ render });
  store(RENDER_KEY, render);
}

export function setGpu(gpu: boolean): void {
  streamOptions.setState({ gpu });
  store(GPU_KEY, gpu ? 'on' : 'off');
}

export const streaming = createStore<StreamStatus>()(() => idle());

export function useStreaming<T>(select: (status: StreamStatus) => T): T {
  return useStore(streaming, select);
}

// A recording of a canvas going to the dev server.
export interface Recording {
  stop(): Promise<void>; // resolves once its last piece has gone
  cancel(): void; // stops at once; nothing more goes
}

// Records a canvas and posts it to the dev server, piece after piece, after
// saying what draws it (graphics, the WebGL renderer) and, for the page that
// started the stream, its session. onFail says why it stopped early; null
// when this browser can't record.
export function record(canvas: HTMLCanvasElement, graphics: string, onFail: (message: string) => void, session = ''): Recording | null {
  const type = recordingType();
  if (!type) {
    onFail(NO_RECORDER);
    return null;
  }
  const id = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
  const media = canvas.captureStream(FPS);
  const recorder = new MediaRecorder(media, { mimeType: type, videoBitsPerSecond: RECORD_BPS });
  let broken = false; // failed or cancelled: nothing more goes
  const cancel = () => {
    broken = true;
    recorder.ondataavailable = null;
    recorder.onstop = null;
    if (recorder.state !== 'inactive') recorder.stop();
    for (const track of media.getTracks()) track.stop();
  };
  const fail = (problem: string | null) => {
    if (!problem || broken) return;
    cancel();
    onFail(problem);
  };
  // Every post waits for the one before, so the pieces arrive in order.
  let sending: Promise<unknown> = post('/__stream/recording', { id, graphics, session }).then(fail);
  recorder.ondataavailable = (event) => {
    if (event.data.size === 0) return;
    sending = sending.then(async () => {
      if (!broken) fail(await post('/__stream/chunk', event.data, { 'X-Recording': id }));
    });
  };
  recorder.start(PIECE_MS);
  return {
    stop: () =>
      new Promise<void>((resolve) => {
        if (broken || recorder.state === 'inactive') return resolve();
        // The last piece comes before the stop.
        recorder.onstop = () => {
          for (const track of media.getTracks()) track.stop();
          void sending.then(() => resolve());
        };
        recorder.stop();
      }),
    cancel,
  };
}

let stage: Stage | null = null; // this page's, which a web + stream recording records
let recording: Recording | null = null; // this page's, while it streams what it draws
let following: Promise<unknown> = Promise.resolve(); // changes go to the hidden browser in order
let sounding: Promise<unknown> = Promise.resolve(); // and the sound's to the dev server
let poll: number | null = null;

// Sets up streaming for the panel's page (main.tsx), not for the hidden
// browser's.
export function connectStream(source: Stage): void {
  stage = source;
  // A stream that runs as this page loads shows in the panel. One the dev
  // server draws is steered from here: the panel takes on what it draws.
  void fetch('/__stream/status', { cache: 'no-store' })
    .then((answer) => (answer.ok ? (answer.json() as Promise<StreamStatus>) : null))
    .then((status) => {
      if (!status || !running(status.state)) return forget();
      streaming.setState(status);
      if (status.drawnBy === 'server') {
        setRender('stream');
        if (status.scene) settings.setState(settingsFromQuery(status.scene));
      } else {
        // This tab's stream, before a reload: it records again.
        const session = stored(SESSION_KEY, tab);
        if (session && status.state !== 'stopping' && !recording) {
          setRender('both');
          recording = recordHere(session);
        }
      }
      // The panel shows the sound that streams.
      if (status.audio) setAudio(status.audio.choice);
      watch();
    })
    .catch(() => {});
  // The settings go to the hidden browser while it draws the stream, and so
  // does what is held down to walk the camera.
  settings.subscribe((s) => follow({ scene: settingsQuery(s) }));
  walking.subscribe((w) => follow({ walk: w.held }));
  // The sound changes while live, however the stream is drawn.
  audioOptions.subscribe((now, before) => {
    if (JSON.stringify(audioChoice(now)) === JSON.stringify(audioChoice(before))) return; // only Play here
    if (!running(streaming.getState().state)) return;
    sounding = sounding.then(() => post('/__stream/audio', audioChoice(now)));
  });
}

// Starts streaming to an RTMP server (YouTube's stream URL and stream key) at
// 720p or 1080p: this page's drawing (web + stream), or the dev server's
// (stream only), with or without the graphics card. How it goes shows in
// `streaming`.
export async function startStream(url: string, key: string, quality: Quality): Promise<void> {
  const { render, gpu } = streamOptions.getState();
  if (!stage || recording || render === 'web' || running(streaming.getState().state)) return;
  const drawnBy: DrawnBy = render === 'stream' ? 'server' : 'page';
  if (drawnBy === 'page' && !recordingType()) return fail(NO_RECORDER);
  streaming.setState({ ...idle(), state: 'starting', drawnBy, gpu });

  let answer: Response;
  try {
    answer = await fetch('/__stream/start', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        url: url.trim(),
        key: key.trim(),
        quality,
        drawnBy,
        gpu,
        scene: settingsQuery(settings.getState()),
        audio: audioChoice(),
      }),
    });
  } catch {
    return fail('The dev server did not answer. Streaming needs npm run dev.');
  }
  if (answer.status === 404) return fail('This server has no stream bridge. Streaming needs npm run dev.');
  if (!answer.ok) return fail((await answer.text()).trim() || `The dev server said ${answer.status}.`);
  if (drawnBy === 'page') {
    const { session = '' } = (await answer.json().catch(() => ({}))) as { session?: string };
    store(SESSION_KEY, session, tab);
    recording = recordHere(session);
  }
  watch();
}

// Records this page's drawing for the stream; a recording that fails ends it.
function recordHere(session: string): Recording | null {
  return record(stage!.canvas, stage!.graphics, (message) => {
    fail(message);
    void post('/__stream/stop', {});
  }, session);
}

// Stops streaming: a recording here ends and its last piece is sent, then
// ffmpeg finishes and closes the connection, and the hidden browser closes.
export function stopStream(): void {
  const ending = recording;
  recording = null;
  forget();
  if (ending) void ending.stop().then(() => post('/__stream/stop', {}));
  else void post('/__stream/stop', {});
  if (streaming.getState().state !== 'off') streaming.setState({ state: 'stopping' });
  watch();
}

// Sends a change to the dev server's hidden browser, while it draws the
// stream (and this page, with "stream only", steers it).
export function follow(change: Follow): void {
  const { state, drawnBy } = streaming.getState();
  if (streamOptions.getState().render !== 'stream' || drawnBy !== 'server' || (state !== 'starting' && state !== 'live')) return;
  following = following.then(() => post('/__stream/follow', change));
}

function fail(message: string): void {
  recording?.cancel();
  recording = null;
  forget();
  streaming.setState({ state: 'error', message });
}

// This tab draws no stream (any more).
function forget(): void {
  store(SESSION_KEY, null, tab);
}

// Asks the dev server how the stream goes, once a second, until it is over.
function watch(): void {
  if (poll !== null) return;
  poll = window.setInterval(async () => {
    const answer = await fetch('/__stream/status', { cache: 'no-store' }).catch(() => null);
    if (!answer?.ok) return;
    const status = (await answer.json()) as StreamStatus;
    const mine = streaming.getState();
    // A failure here (the browser, the dev server gone) outranks the server's
    // word, and so does stopping, until the server has stopped too.
    if (mine.state === 'error' && status.state === 'off') status.state = 'error';
    if (mine.state === 'error' && !status.message) status.message = mine.message;
    if (mine.state === 'stopping' && (status.state === 'live' || status.state === 'starting')) status.state = 'stopping';
    // ffmpeg gave up (YouTube refused the key, the network is down): stop
    // recording, and say what ffmpeg said.
    if (status.state === 'error' && recording) fail(status.message);
    streaming.setState(status);
    if ((status.state === 'off' || status.state === 'error') && !recording) {
      forget();
      window.clearInterval(poll!);
      poll = null;
    }
  }, POLL_MS);
}

// Posts to the dev server: why it refused, or null.
async function post(path: string, body: unknown, headers: Record<string, string> = {}): Promise<string | null> {
  const blob = body instanceof Blob;
  const answer = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': blob ? 'application/octet-stream' : 'application/json', ...headers },
    body: blob ? body : JSON.stringify(body),
  }).catch(() => null);
  if (!answer) return 'Lost the dev server.';
  if (answer.ok) return null;
  return (await answer.text()).trim() || `The dev server said ${answer.status}.`;
}

function recordingType(): string | undefined {
  return typeof MediaRecorder === 'undefined' ? undefined : TYPES.find((t) => MediaRecorder.isTypeSupported(t));
}

function running(state: StreamState): boolean {
  return state === 'starting' || state === 'live' || state === 'stopping';
}

function idle(): StreamStatus {
  return {
    state: 'off',
    message: '',
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

export function stored(key: string, storage: () => Storage = () => localStorage): string | null {
  try {
    return storage().getItem(key);
  } catch {
    return null; // storage can be blocked
  }
}

// Keeps a value in this browser, or forgets it (null).
export function store(key: string, value: string | null, storage: () => Storage = () => localStorage): void {
  try {
    if (value === null) storage().removeItem(key);
    else storage().setItem(key, value);
  } catch {
    // not remembered, that's all
  }
}
