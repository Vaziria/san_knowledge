import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

// The scene's sound, for the panel's Audio tab: none, one file from the dev
// server's audio folder, every file there in turn, or an input device of the
// dev server's machine (a microphone; Sonic Pi through Stereo Mix or a
// virtual cable). Loop plays the file, or the list, again from its start;
// the volume goes from 0 to 1.
//
// The stream's sound is made on the dev server (audio-bridge.ts), so it is
// the same with either Render, and it changes while live: stream.ts sends the
// choice when the stream starts and whenever it changes. With "Play here"
// this page plays a file too, to hear it here; a device's sound stays on the
// dev server's machine. The choice is kept in this browser.

// What to play; audio-bridge.ts has the same shapes.
export type AudioSource =
  | { kind: 'none' }
  | { kind: 'file'; name: string }
  | { kind: 'files' }
  | { kind: 'device'; name: string };
export interface AudioChoice {
  source: AudioSource;
  loop: boolean;
  volume: number; // 0..1
}

export interface AudioOptions extends AudioChoice {
  here: boolean; // this page plays it too
}

const KEY = 'animation-audio';
const FRESH: AudioOptions = { source: { kind: 'none' }, loop: true, volume: 1, here: true };

export const audioOptions = createStore<AudioOptions>()(saved);

export function useAudio<T>(select: (options: AudioOptions) => T): T {
  return useStore(audioOptions, select);
}

export function setAudio(change: Partial<AudioOptions>): void {
  audioOptions.setState(change);
  try {
    localStorage.setItem(KEY, JSON.stringify(audioOptions.getState()));
  } catch {
    // not remembered, that's all
  }
}

// The part the stream plays.
export function audioChoice({ source, loop, volume } = audioOptions.getState()): AudioChoice {
  return { source, loop, volume };
}

function saved(): AudioOptions {
  try {
    const stored = JSON.parse(localStorage.getItem(KEY) ?? 'null') as Partial<AudioOptions> | null;
    const kind = stored?.source?.kind;
    if (!stored || !['none', 'file', 'files', 'device'].includes(kind ?? '')) return FRESH;
    return {
      source: stored.source!,
      loop: stored.loop !== false,
      volume: typeof stored.volume === 'number' ? Math.min(1, Math.max(0, stored.volume)) : 1,
      here: stored.here !== false,
    };
  } catch {
    return FRESH; // storage can be blocked, or hold something else
  }
}

// What the dev server has to play: the files in its audio folder, and the
// input devices of its machine.
export interface AudioFile {
  name: string;
  size: number; // bytes
}
interface Library {
  files: AudioFile[];
  devices: string[];
  loaded: boolean; // the files have been asked for
}

export const library = createStore<Library>()(() => ({ files: [], devices: [], loaded: false }));

export function useLibrary<T>(select: (library: Library) => T): T {
  return useStore(library, select);
}

export async function loadFiles(): Promise<void> {
  const answer = await fetch('/__audio/files', { cache: 'no-store' }).catch(() => null);
  const files = answer?.ok ? ((await answer.json()) as { files: AudioFile[] }).files : [];
  library.setState({ files, loaded: true });
}

export async function loadDevices(): Promise<void> {
  const answer = await fetch('/__audio/devices', { cache: 'no-store' }).catch(() => null);
  library.setState({ devices: answer?.ok ? ((await answer.json()) as { devices: string[] }).devices : [] });
}

// Puts files into the dev server's audio folder: why one couldn't go, or
// null.
export async function addFiles(files: readonly File[]): Promise<string | null> {
  for (const file of files) {
    const answer = await fetch(`/__audio/files?name=${encodeURIComponent(file.name)}`, { method: 'POST', body: file }).catch(
      () => null,
    );
    if (!answer) return 'Lost the dev server.';
    if (!answer.ok) return (await answer.text()).trim() || `The dev server said ${answer.status}.`;
  }
  await loadFiles();
  return null;
}

// Playing here: what plays, or that the browser waits for a click before it
// plays sound (it does after a reload).
interface Here {
  playing: string; // a file's name, or nothing
  waiting: boolean;
}

export const here = createStore<Here>()(() => ({ playing: '', waiting: false }));

export function useHere<T>(select: (here: Here) => T): T {
  return useStore(here, select);
}

let player: HTMLAudioElement | null = null;
let next = 0; // the list's file, for "files"
let lastSource = ''; // a new source starts its list from the top
let ended = ''; // the choice that has played to its end (no loop): it rests until the choice changes

// Plays the chosen file in this page while "Play here" is on (the panel's
// page; main.tsx).
export function playHere(): void {
  const audio = new Audio();
  audio.preload = 'auto';
  player = audio;
  audio.addEventListener('ended', () => {
    const { source, loop } = audioOptions.getState();
    // One file loops by itself (audio.loop); a list goes on to the next.
    if (source.kind === 'files') {
      next += 1;
      if (next >= library.getState().files.length) {
        next = 0;
        if (!loop) ended = restingKey();
      }
    } else {
      ended = restingKey();
    }
    void sync();
  });
  audioOptions.subscribe(() => void sync());
  // A click on the page lets the browser play sound.
  window.addEventListener(
    'pointerdown',
    () => {
      if (here.getState().waiting) void sync();
    },
    { capture: true },
  );
  void sync();
}

function restingKey(): string {
  const { source, loop, here: on } = audioOptions.getState();
  return JSON.stringify({ source, loop, on });
}

async function sync(): Promise<void> {
  const audio = player;
  if (!audio) return;
  const { source, loop, volume, here: on } = audioOptions.getState();
  const sourceKey = JSON.stringify(source);
  if (sourceKey !== lastSource) {
    lastSource = sourceKey;
    next = 0;
  }
  if (ended && ended !== restingKey()) ended = '';
  audio.volume = volume;
  audio.loop = source.kind === 'file' && loop;

  let file = '';
  if (on && !ended) {
    if (source.kind === 'file') file = source.name;
    if (source.kind === 'files') {
      if (!library.getState().loaded) await loadFiles();
      const files = library.getState().files;
      file = files.length ? files[next % files.length].name : '';
    }
  }
  if (!file) {
    audio.pause();
    here.setState({ playing: '', waiting: false });
    return;
  }
  if (audio.dataset.file !== file) {
    audio.src = `/__audio/files/${encodeURIComponent(file)}`;
    audio.dataset.file = file;
  }
  if (!audio.paused) return here.setState({ playing: file, waiting: false });
  try {
    await audio.play();
    here.setState({ playing: file, waiting: false });
  } catch (error) {
    // Browsers play sound only after a click on the page.
    here.setState({ playing: '', waiting: error instanceof DOMException && error.name === 'NotAllowedError' });
  }
}
