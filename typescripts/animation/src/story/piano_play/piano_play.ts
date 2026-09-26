import { MidiPiano } from '../../figures/MidiPiano';
import { onOsc, type OscMessage } from '../../osc';
import { environments } from '../../previews';
import type { Story } from '../../stories';

// Piano play (piano_play.md): the MIDI piano on the grass, playing the chords
// and notes sent to it over OSC (Open Sound Control), for example from Sonic
// Pi. The dev server receives them on UDP port 57121 (osc-bridge.ts):
//
//   /piano/chord "Am"         PlayChord, with a chord name as it takes them
//   /piano/note "E5" or 76    PlayNote, with a note name or a MIDI note number
//   /piano/release            lets go of every key
//
// A chord or note stays down until the next one replaces it. Other addresses
// are left alone; a name the piano can't read is reported in the console and
// skipped.
//
// It can also be played by hand: click or touch a key and it stays down
// until you let go, one key per finger.
export const pianoPlay: Story = {
  environment: environments.grass,
  create: (theme) => {
    const piano = new MidiPiano({ theme });
    return {
      figure: piano,
      camera: [0.3, 0.35, 0.7],
      target: [0, 0.03, 0],
      update: (_elapsed, delta) => piano.update(delta),
      dispose: onOsc((message) => play(piano, message)),
      press: (part) => {
        const key = piano.keyAt(part);
        if (key === null) return null;
        piano.setKeyPressed(key, true);
        return () => piano.setKeyPressed(key, false);
      },
    };
  },
};

function play(piano: MidiPiano, { address, args }: OscMessage): void {
  try {
    if (address === '/piano/chord') piano.PlayChord(String(args[0]));
    else if (address === '/piano/note') piano.PlayNote(typeof args[0] === 'number' ? noteName(args[0]) : String(args[0]));
    else if (address === '/piano/release') piano.releaseAll();
  } catch (error) {
    console.warn(`piano_play: ${address} ${args.join(' ')}: ${(error as Error).message}`);
  }
}

// The name of a MIDI note number, in scientific pitch: 60 is "C4", 61 "C#4".
const NOTE_NAMES = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B'];

function noteName(midi: number): string {
  const note = Math.round(midi);
  return NOTE_NAMES[((note % 12) + 12) % 12] + (Math.floor(note / 12) - 1);
}
