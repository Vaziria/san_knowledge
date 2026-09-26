# Piano play state

Updated 2026-09-25 by task 17 (the first state record), from the code as it is today.

## What works

The MIDI piano on the grass, played by OSC (Open Sound Control), for example from Sonic Pi. Spec: [piano_play.md](../../src/story/piano_play/piano_play.md), the user's. Rules: [rules.md, Stories](../../../../docs/3d_modelling/rules.md#stories), rules 1–9. Code: [piano_play.ts](../../src/story/piano_play/piano_play.ts). The piano itself: [MidiPiano.md](../../src/figures/MidiPiano.md) and [pianos.md](pianos.md).

- **Picked in the Story tab** (spec item 2) or with `?story=piano_play`. [stories.ts](../../src/stories.ts) lists it first, after `none` and before `lake_meeting`. It plays on `environments.grass`, seen from a fixed camera above the front of the keyboard.
- **OSC messages** come through the dev server's OSC bridge (`osc-bridge.ts`: UDP port 57121 on this machine, or `POST /__osc`; the bridge is in [pianos.md](pianos.md)):
  - `/piano/chord "Am"` presses the chord (`PlayChord`), rooted in the C4 octave, and lets go of the one before.
  - `/piano/note "E5"` or a MIDI number (`76`) presses one note (`PlayNote`) along with the chord, letting go of the note before. A note outside the keyboard's two octaves (C4–B5) moves into one it has.
  - `/piano/release` lets go of every key.
  - Other addresses are left alone. A name the piano can't read is warned about in the console and skipped.
- **It stops listening in `dispose()`**, when another story or a figure replaces it.
- **Played by hand:** click or touch a key and it stays down until let go, one key per finger. Dragging anywhere else turns the view (Stories rule 9).
- **Sending OSC:** `npm run osc -- /piano/chord Am`, `npm run osc:demo`, and Sonic Pi, through `osc-relay.mjs` when the preview is reached through a tunnel ([sonicpi.rb](../../examples/sonicpi.rb); Stories rule 7).
- It makes no sound: the keys go down, and the sound is Sonic Pi's own.
- Only `npm run dev` has the OSC bridge. A built page shows the piano but gets no messages.

## How it was last checked

- 2026-09-25, task 17: the code read against the spec and rules.md, the story itself not run. `npm run typecheck` is clean at 23:10, with other sessions' unfinished work in the tree (earlier that evening it failed for a few minutes on the dock's half-built lines, task 09).
- 2026-09-24, when it was built (commit ca1bf63): Stories rules 6 and 9 say how. Real UDP packets from Node to 127.0.0.1 and ::1 on a test server's own `OSC_PORT`, the console's warnings, a screenshot of the pressed keys, and no warning once the story was off (so `dispose()` ran). Key presses sent as DevTools mouse and touch events. No results were written down beyond those rules.
- No task has checked it since. Its code changed in one line (History).

## Known issues and left for the user

- **A finger and OSC don't know of each other.** Letting go of a key by hand lifts it even while the OSC chord or note holds it, and a new chord lifts old chord keys a finger still holds (`press` in `piano_play.ts`, `PlayChord` in `MidiPiano.ts`). Seen in the code, not tried.
- **[sonicpi.rb](../../examples/sonicpi.rb) names an ngrok URL,** which changes whenever ngrok restarts.
- **Not committed:** its one-line environment change, and the `stories.ts` change that goes with it, are in the working tree only.

## Open questions

- The spec is two lines. Playing by hand, `/piano/release` and MIDI note numbers go beyond it; they are in rules.md, not in the spec.
- No task in `tasks/` touches it.

## History

- 2026-09-25, before the task queue, not committed: it names its environment's builder (`environments.grass`) instead of the key `'grass'`, since stories now build their own environment (for the lake meeting's clearing).
- 2026-09-24 commit ca1bf63: the story added from the user's spec, with the OSC bridge, playing by hand and the story picker in the panel.
