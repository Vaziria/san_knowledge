import { useEffect, useRef, useState } from 'react';
import {
  addFiles,
  audioOptions,
  loadDevices,
  loadFiles,
  setAudio,
  useAudio,
  useHere,
  useLibrary,
  type AudioSource,
} from '../audio';
import { useStreaming } from '../stream';
import { Action, Select, Slider, Toggle } from './controls';

// The Audio tab: the scene's sound (audio.ts), in this page and in the
// stream. The source is none, one file of the dev server's audio folder,
// every file there in turn, or an input device of the dev server's machine;
// Add files puts files into the folder from here. Loop plays the file, or the
// list, again from its start. Rows share the panel's three columns, like the
// Settings tab.

const SOURCES: { value: AudioSource['kind']; label: string }[] = [
  { value: 'none', label: 'none (silent)' },
  { value: 'file', label: 'one file' },
  { value: 'files', label: 'every file, in turn' },
  { value: 'device', label: 'an input device' },
];

// What the file picker takes; the dev server takes the same.
const ACCEPT = 'audio/*,.mp3,.m4a,.aac,.ogg,.oga,.opus,.wav,.flac,.webm';

export function AudioTab() {
  const source = useAudio((a) => a.source);
  const loop = useAudio((a) => a.loop);
  const volume = useAudio((a) => a.volume);
  const hereOn = useAudio((a) => a.here);
  const files = useLibrary((l) => l.files);
  const devices = useLibrary((l) => l.devices);
  const loaded = useLibrary((l) => l.loaded);
  const fromFiles = source.kind === 'file' || source.kind === 'files';

  useEffect(() => {
    void loadFiles();
    void loadDevices();
  }, []);
  // A file or device that isn't there (yet, or any more) gives way to the
  // first one that is.
  useEffect(() => {
    if (source.kind === 'file' && loaded && files.length && !files.some((f) => f.name === source.name)) {
      setAudio({ source: { kind: 'file', name: files[0].name } });
    }
    if (source.kind === 'device' && devices.length && !devices.includes(source.name)) {
      setAudio({ source: { kind: 'device', name: devices[0] } });
    }
  }, [source, files, devices, loaded]);

  const pick = (kind: string) => {
    if (kind === 'none' || kind === 'files') setAudio({ source: { kind } });
    if (kind === 'file') setAudio({ source: { kind, name: source.kind === 'file' ? source.name : (files[0]?.name ?? '') } });
    if (kind === 'device') setAudio({ source: { kind, name: devices[0] ?? '' } });
  };

  return (
    <div className="grid grid-cols-[max-content_1fr_max-content] items-center gap-2">
      <Select label="Source" value={source.kind} options={SOURCES} onChange={pick} />
      {source.kind === 'file' && files.length > 0 && (
        <Select
          label="File"
          value={source.name}
          options={files.map(({ name }) => ({ value: name, label: name }))}
          onChange={(name) => setAudio({ source: { kind: 'file', name } })}
        />
      )}
      {source.kind === 'device' && devices.length > 0 && (
        <Select
          label="Device"
          value={source.name}
          options={devices.map((name) => ({ value: name, label: name }))}
          onChange={(name) => setAudio({ source: { kind: 'device', name } })}
        />
      )}
      {fromFiles && <AddFiles />}
      {fromFiles && <Toggle label="Loop" on={loop} onChange={(on) => setAudio({ loop: on })} />}
      <Slider
        label="Volume"
        value={Math.round(volume * 100)}
        min={0}
        max={100}
        unit="%"
        onChange={(percent) => setAudio({ volume: percent / 100 })}
      />
      {fromFiles && <Toggle label="Play here" on={hereOn} onChange={(on) => setAudio({ here: on })} />}
      <Where />
      <Note source={source} files={files.length} devices={devices.length} />
    </div>
  );
}

// Puts files into the dev server's audio folder, and picks the first one
// when no file is picked yet.
function AddFiles() {
  const input = useRef<HTMLInputElement>(null);
  const [adding, setAdding] = useState(false);
  const [problem, setProblem] = useState('');
  const add = async (list: FileList | null) => {
    if (!list?.length) return;
    const chosen = [...list];
    setAdding(true);
    setProblem('');
    const why = await addFiles(chosen);
    setAdding(false);
    setProblem(why ?? '');
    const { source } = audioOptions.getState();
    if (!why && source.kind === 'file' && !source.name) setAudio({ source: { kind: 'file', name: chosen[0].name } });
    if (input.current) input.current.value = ''; // the same file can be picked again
  };
  return (
    <>
      <input ref={input} type="file" accept={ACCEPT} multiple hidden onChange={(event) => void add(event.currentTarget.files)} />
      <Action label={adding ? 'Adding…' : 'Add files…'} wide disabled={adding} onClick={() => input.current?.click()} />
      {problem && <p className="col-span-full text-xs font-medium text-destructive">{problem}</p>}
    </>
  );
}

// Where the sound is heard: here, and in the stream while one runs.
function Where() {
  const hereOn = useAudio((a) => a.here);
  const device = useAudio((a) => a.source.kind === 'device');
  const playing = useHere((h) => h.playing);
  const waiting = useHere((h) => h.waiting);
  const streamed = useStreaming((s) => s.audio);
  const live = useStreaming((s) => s.state === 'starting' || s.state === 'live');
  const line = 'col-span-full text-xs';
  return (
    <>
      {!device && hereOn && (
        <p className={line}>
          {playing ? `Playing here: ${playing}` : waiting ? 'Click anywhere on the page to hear it here: the browser waits for a click.' : 'Not playing here.'}
        </p>
      )}
      {live && streamed && (
        <p className={`${line} font-medium`}>In the stream: {streamed.choice.source.kind === 'none' ? 'silence' : streamed.playing || 'silence'}</p>
      )}
      {live && streamed?.problem && <p className={`${line} font-medium text-destructive`}>{streamed.problem}</p>}
    </>
  );
}

function Note({ source, files, devices }: { source: AudioSource; files: number; devices: number }) {
  let text: string;
  if (source.kind === 'device') {
    text =
      (devices ? '' : 'The dev server\'s machine has no input device ffmpeg can open. ') +
      "A device is the dev server machine's, and plays in the stream only. For Sonic Pi or other sound playing there, " +
      'pick a device that records it: Stereo Mix (Sound settings, Recording, if the sound card has one), or a virtual ' +
      'cable such as VB-CABLE with the sound played into it.';
  } else if (source.kind === 'none') {
    text = 'Nothing plays; the stream gets silence, which YouTube wants rather than no sound.';
  } else {
    text =
      (files ? '' : 'No files yet. ') +
      'Files are kept in typescripts/animation/audio on the dev server (not in git): add them here, or put them there. ' +
      'The stream plays them on the dev server, so changing them while live keeps the stream going.';
  }
  return <p className="col-span-full text-xs text-muted-foreground">{text}</p>;
}
