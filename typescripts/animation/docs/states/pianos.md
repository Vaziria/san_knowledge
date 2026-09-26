# Pianos state

Updated 2026-09-25 by task 17 (the first state record), from the code as it is today. The piano and OSC code hasn't changed since commit ca1bf63 (2026-09-24).

## What works

- **MIDI keyboard** ([MidiPiano.ts](../../src/figures/MidiPiano.ts), spec [MidiPiano.md](../../src/figures/MidiPiano.md)): a cute two-octave keyboard, C4–B5 (MIDI 60–83), with 14 white and 10 black keys, three knobs and an empty left panel. Everything is rounded and every colour comes from the theme. See [rules.md, Keyboards and pianos](../../../../docs/3d_modelling/rules.md#keyboards-and-pianos) and [Cute style](../../../../docs/3d_modelling/rules.md#cute-style).
  - `PlayChord(chord)` presses a chord rooted in the C4 octave and lets go of the chord before. It takes major, `m`, `dim`, `aug`, `sus2`, `sus4`, `7`, `maj7` and `m7`, with `#` or `b` on the root.
  - `PlayNote(note)` presses one note (`E`, `F#`, or `A4`) and lets go of the note before. The note is held together with the chord. An octave the keyboard lacks is moved onto it.
  - A name either one can't read throws an error.
  - Beyond the spec: `setKeyPressed(midi, pressed)` (notes outside C4–B5 are ignored), `releaseAll()`, `setKnob(index, 0..1)`, `keyAt(part)` for playing by pointer, and `update(delta)`, which eases the keys toward their targets.
- **In the preview** (`midi-piano` in [previews.ts](../../src/previews.ts), on the floor): a demo loops C G Am F with a four-note melody over each chord and swings the knobs. The Behaviours tab has `PlayChord` and `PlayNote` buttons, and a bad name shows its error there. The penguin and every animal can `Hold` the keyboard.
- **Played from outside:** the story in [piano_play.md](piano_play.md) shows the keyboard on the grass and plays what OSC sends (`/piano/chord`, `/piano/note` with a name or a MIDI number, `/piano/release`). A click or touch on a key holds it down.
- **OSC input** ([rules.md, Stories](../../../../docs/3d_modelling/rules.md#stories) rules 3–8):
  - [osc-bridge.ts](../../osc-bridge.ts) is a Vite plugin that runs under `npm run dev` only. It listens for UDP on 127.0.0.1 and ::1, port 57121 (`OSC_PORT` changes it). It also takes one packet at a time through `POST /__osc`, answered 204, or 400 with the reason, so OSC works through ngrok or a VS Code tunnel. It reads OSC 1.0 messages and bundles, and ignores a bundle's time tag. Each message reaches the page as the Vite event `osc:message`, and `onOsc()` in [osc.ts](../../src/osc.ts) hands it to a listener. The bridge opens its port only once the dev server listens, so a config restart doesn't find the port taken.
  - [osc-relay.mjs](../../osc-relay.mjs) is served at `/__osc/relay.mjs`. On Sonic Pi's computer it takes UDP on port 57121 and posts each packet to `/__osc`, one request at a time, bundling packets that arrive meanwhile. [examples/sonicpi.rb](../../examples/sonicpi.rb) sends its chords and melody this way.
  - [osc-send.ts](../../osc-send.ts): `npm run osc -- /piano/chord Am` sends one message from the dev server's machine. `npm run osc:demo` plays C G Am F with a melody until Ctrl+C, then sends `/piano/release`.
- **Grand piano** ([Piano.ts](../../src/figures/Piano.ts), draft spec [Piano.md](../../src/figures/Piano.md)): 88 keys (A0–C8), a 1.5 m case with its lid on a prop stick, three legs, three pedals and a music stand. It has `setKeyPressed`, `releaseAll` and `update`. It is in no preview, story or panel list, so it can only be used from code.

## How it was last checked

- 2026-09-25 23:04: both runs of `npm run typecheck` pass. The dev-server run covers `osc-bridge.ts` and `osc-send.ts` (task 17).
- Before commit ca1bf63 (2026-09-24), undated ([rules.md, Stories](../../../../docs/3d_modelling/rules.md#stories) rules 6 and 9):
  - `piano_play` was checked with real OSC packets to 127.0.0.1 and ::1, spied on with `onOsc`. The console warnings were read and a screenshot showed the pressed keys.
  - Keys were pressed with DevTools mouse and touch events.
  - After the story stopped, a bad message gave no warning.
- The relay was used with Sonic Pi through ngrok ([examples/sonicpi.rb](../../examples/sonicpi.rb)).
- The user accepted the keyboard's rounding (Cute style rule 1).
- No check of the grand piano is recorded. No task in `tasks/done/` touched the pianos.

## Known issues and left for the user

- The grand piano can't be seen from the panel, because it is in no preview. Piano.md says so.
- Clicking keys plays them only in the `piano_play` story. The `midi-piano` figure preview has no `press()`, so there a drag on a key turns the view.
- The knobs turn only in the preview's demo. No behaviour button or OSC address sets them.
- OSC reaches only a page served by `npm run dev`; a built page gets none. UDP doesn't go through ngrok or VS Code tunnels, so Sonic Pi on another computer needs the relay.
- The grand piano's black keys are 12.5 mm wide, but rules.md Keyboards and pianos rule 3 gives 13.7 mm for a real one. The MIDI keyboard's 17.5 mm is 13.7 mm scaled to its 30 mm white keys.
- rules.md Moving parts rule 3 names `setPitchBend(-1..1)` as an example API, but neither piano has one.
- rules.md Checking the result rule 1 says the dev-server check covers `vite.config.ts`, `osc-bridge.ts` and `stream-bridge.ts`. `tsconfig.node.json` also checks `osc-send.ts`, the audio and chat bridges, and `meeting.ts`.

## Open questions

- [MidiPiano.md](../../src/figures/MidiPiano.md) names only `PlayChord(chord string)` (example "A") and `PlayNote(note string)` (no example). Claude chose the rest, and it is in the code's comments for the user to put in the spec ([rules.md, Figure specs](../../../../docs/3d_modelling/rules.md#figure-specs) rule 5):
  - the chord names it takes;
  - the root in the C4 octave;
  - a note held together with the chord;
  - the extra methods.
- [Piano.md](../../src/figures/Piano.md) is a draft written from the code, and the user hasn't confirmed it. Should the grand piano come back into the preview?
- Task 16, the part preview, is in progress. It gives the pianos no parts, as the task allows.
- No queued task touches the pianos.

## History

- 2026-09-24, commit 278ad67: the new animals' Hold menu offers the MIDI keyboard too.
- 2026-09-24, commit ca1bf63: this commit added:
  - the grand piano and the MIDI keyboard, with their specs;
  - the `midi-piano` preview, with its demo and buttons, and the penguin holding the keyboard;
  - the OSC bridge (UDP and `POST /__osc`), the relay and `osc-send.ts`;
  - the `piano_play` story.
