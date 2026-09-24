import { useEffect, useState } from 'react';
import {
  QUALITY_NAMES,
  RENDERS,
  setGpu,
  setRender,
  startStream,
  stopStream,
  useStreamOptions,
  useStreaming,
  YOUTUBE_URL,
  type Quality,
  type Render,
  type StreamStatus,
} from '../stream';
import { Action, Select, TextField, Toggle } from './controls';

// The Stream tab: streams the scene live to YouTube over RTMP (stream.ts, and
// stream-bridge.ts on the dev server). YouTube Studio shows the stream URL and
// the stream key to paste here (Create, Go live, Stream). Render says where
// the scene is drawn: here and in the stream, only in the stream (drawn by
// the dev server), or only here. GPU has the dev server's side use the
// graphics card. The URL, Render and GPU are kept in this browser; the key is
// not kept anywhere, so after a reload it is pasted again. Rows share the
// panel's three columns, like the Settings tab.

const URL_KEY = 'animation-stream-url';

function savedUrl(): string {
  try {
    return localStorage.getItem(URL_KEY) || YOUTUBE_URL;
  } catch {
    return YOUTUBE_URL; // storage can be blocked
  }
}

const RENDER_OPTIONS = RENDERS.map(({ value, label }) => ({ value, label }));
const QUALITY_OPTIONS = QUALITY_NAMES.map((name) => ({ value: name, label: `${name}, 30 fps` }));

const NOTES: Record<Render, string> = {
  both:
    'Streams the scene as it is drawn here, without the panel, with silent sound. Keep this tab shown: a hidden tab ' +
    'stops drawing, and the stream freezes. GPU: ffmpeg encodes on the graphics card, or on the CPU.',
  stream:
    "The dev server draws the scene for the stream in a hidden browser, and this page doesn't: it steers the stream " +
    'with the panel (figure, story, environment, look, behaviours). It streams until Stop, with this page hidden or ' +
    'closed. GPU: drawing and encoding on the graphics card, or on the CPU (slower).',
  web: 'The scene is drawn here only; nothing is streamed.',
};

export function StreamTab() {
  const status = useStreaming((s) => s);
  const render = useStreamOptions((o) => o.render);
  const gpu = useStreamOptions((o) => o.gpu);
  const [url, setUrl] = useState(savedUrl);
  const [key, setKey] = useState('');
  const [quality, setQuality] = useState<Quality>('720p');
  const busy = status.state === 'starting' || status.state === 'live' || status.state === 'stopping';
  const off = render === 'web'; // nothing to stream

  const changeUrl = (value: string) => {
    setUrl(value);
    try {
      localStorage.setItem(URL_KEY, value);
    } catch {
      // not remembered, that's all
    }
  };
  const pickRender = (value: string) => {
    const found = RENDERS.find((r) => r.value === value);
    if (found) setRender(found.value);
  };
  const pickQuality = (value: string) => {
    const found = QUALITY_NAMES.find((name) => name === value);
    if (found) setQuality(found);
  };

  return (
    <div className="grid grid-cols-[max-content_1fr_max-content] items-center gap-2">
      <Select label="Render" value={render} options={RENDER_OPTIONS} disabled={busy} onChange={pickRender} />
      <Toggle label="GPU" on={gpu} disabled={busy || off} onChange={setGpu} />
      <TextField label="Stream URL" value={url} disabled={busy || off} onChange={changeUrl} />
      <TextField
        label="Stream key"
        secret
        value={key}
        placeholder="from YouTube Studio"
        disabled={busy || off}
        onChange={setKey}
      />
      <Select label="Quality" value={quality} options={QUALITY_OPTIONS} disabled={busy || off} onChange={pickQuality} />
      {busy ? (
        <Action
          label={status.state === 'stopping' ? 'Stopping…' : 'Stop'}
          wide
          tone="danger"
          disabled={status.state === 'stopping'}
          onClick={stopStream}
        />
      ) : (
        <Action
          label="Go live"
          wide
          tone="main"
          disabled={off || !url.trim() || !key.trim()}
          onClick={() => void startStream(url, key, quality)}
        />
      )}
      <StatusLine status={status} />
      <Machinery status={status} />
      <p className="col-span-full text-xs text-muted-foreground">{NOTES[render]}</p>
    </div>
  );
}

// How the stream goes: connecting, live with its time, frame rate and
// bitrate, or what went wrong.
function StatusLine({ status }: { status: StreamStatus }) {
  const line = 'col-span-full text-xs';
  switch (status.state) {
    case 'starting':
      return <p className={line}>Connecting…</p>;
    case 'live':
      return (
        <p className={`${line} font-medium`}>
          <span className="text-destructive">● Live</span> · {clock(status.seconds)} · {Math.round(status.fps)} fps ·{' '}
          {status.kbps} kbit/s
        </p>
      );
    case 'stopping':
      return <p className={line}>Stopping…</p>;
    case 'error':
      return <p className={`${line} font-medium text-destructive`}>{status.message}</p>;
    default:
      return <p className={`${line} text-muted-foreground`}>{status.message || 'Not streaming.'}</p>;
  }
}

// While it streams: what draws the pictures and what encodes them, so the
// GPU switch can be seen to work.
function Machinery({ status }: { status: StreamStatus }) {
  if (status.state !== 'starting' && status.state !== 'live') return null;
  const drawn = status.drawnBy === 'server' ? 'Drawn by the dev server' : 'Drawn here';
  const on = status.graphics ? ` on ${graphicsName(status.graphics)}` : '';
  let encoded = '';
  if (status.encoder === 'libx264') {
    encoded = status.gpu ? 'encoded on the CPU (libx264): no graphics-card encoder works with this ffmpeg' : 'encoded on the CPU (libx264)';
  } else if (status.encoder) {
    encoded = `encoded on the graphics card (${status.encoder})`;
  }
  return (
    <p className="col-span-full text-xs text-muted-foreground">
      {drawn}
      {on}
      {encoded && `, ${encoded}`}.
    </p>
  );
}

// "ANGLE (NVIDIA, NVIDIA GeForce RTX 3050 (0x00002584) Direct3D11 vs_5_0
// ps_5_0, D3D11)" -> "NVIDIA GeForce RTX 3050". SwiftShader draws on the CPU.
function graphicsName(raw: string): string {
  if (/swiftshader|llvmpipe|software/i.test(raw)) return 'the CPU (SwiftShader, no graphics card)';
  const inner = /^ANGLE \([^,]*, (.*), [^,]*\)$/.exec(raw)?.[1] ?? raw;
  return inner
    .replace(/ \(0x[0-9a-f]+\).*$/i, '')
    .replace(/ (Direct3D|OpenGL|Vulkan)\S*.*$/i, '')
    .replace(/^ANGLE Metal Renderer: /, '');
}

// 83 -> "1:23", 3723 -> "1:02:03".
function clock(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = String(Math.floor(seconds % 60)).padStart(2, '0');
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`;
}

const PREVIEW_MS = 1000; // how often the dev server's hidden browser sends a picture

// Where the scene would be, behind the panel, with "stream only": a picture
// of what the stream shows, new every second while the dev server draws it.
export function StreamPreview() {
  const render = useStreamOptions((o) => o.render);
  const drawing = useStreaming((s) => s.drawnBy === 'server' && (s.state === 'starting' || s.state === 'live'));
  const [tick, setTick] = useState(0);
  const [shown, setShown] = useState(false); // a picture has come
  useEffect(() => {
    if (!drawing) return;
    const timer = window.setInterval(() => setTick((n) => n + 1), PREVIEW_MS);
    return () => {
      window.clearInterval(timer);
      setShown(false);
    };
  }, [drawing]);
  if (render !== 'stream') return null;
  return (
    <div className="pointer-events-none fixed inset-0 flex items-center justify-center p-6">
      {drawing && (
        <img
          src={`/__stream/preview?${tick}`}
          alt="What the stream shows"
          className={shown ? 'size-full object-contain' : 'hidden'}
          onLoad={() => setShown(true)}
          onError={() => setShown(false)}
        />
      )}
      {!(drawing && shown) && (
        <p className="max-w-xs text-center text-sm text-neutral-400">
          {drawing
            ? 'Waiting for the first picture from the dev server…'
            : "Stream only: the scene isn't drawn here. Once you go live, the dev server draws it for the stream, and a picture of it shows here every second."}
        </p>
      )}
    </div>
  );
}
