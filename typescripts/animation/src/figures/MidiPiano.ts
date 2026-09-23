import * as THREE from 'three';
import { RoundedBoxGeometry } from 'three/addons/geometries/RoundedBoxGeometry.js';
import { defaultTheme, surface, type Theme } from '../theme';

// A cute two-octave MIDI keyboard, C4 to B5: 14 white keys, 10 sharps,
// 3 knobs behind them and an empty panel on the left. Everything is
// rounded, and every colour comes from the theme. Units are meters, the keys
// face +z and run along x, and the origin is on the floor under the center.

const LOWEST = 60; // C4
const HIGHEST = 83; // B5
const WHITE_COUNT = 14;

const WHITE_WIDTH = 0.03; // wider than a real key (23.5 mm) for a chunky toy look
const WHITE_LENGTH = 0.15;
const WHITE_HEIGHT = 0.02;
const BLACK_WIDTH = 0.0175;
const BLACK_LENGTH = 0.095;
const BLACK_HEIGHT = 0.014;
const KEY_GAP = 0.0015;
const KEY_PRESS_ANGLE = 0.07;
const KEY_SPEED = 25;

const KNOB_COUNT = 3;
const KNOB_SPACING = 0.05;
const KNOB_RADIUS = 0.014;
const KNOB_HEIGHT = 0.012;

const FOOT = 0.008; // the body sits this high on its feet
const LEFT_PANEL = 0.1;
const RIGHT_PANEL = 0.02;
const BACK_PANEL = 0.06;
const BODY_WIDTH = LEFT_PANEL + WHITE_COUNT * WHITE_WIDTH + RIGHT_PANEL;
const BODY_DEPTH = WHITE_LENGTH + BACK_PANEL + 0.005;
const BODY_HEIGHT = 0.045;
const KEYBED_HEIGHT = BODY_HEIGHT - 0.015;
const BODY_RADIUS = 0.006;
const KEY_BACK = BODY_DEPTH / 2 - 0.005 - WHITE_LENGTH;

const BLACK_NOTES = new Set([1, 3, 6, 8, 10]);

const NOTE_LETTERS: Record<string, number> = { C: 0, D: 2, E: 4, F: 5, G: 7, A: 9, B: 11 };

// Chord suffix -> semitones above the root.
const CHORD_INTERVALS: Record<string, number[]> = {
  '': [0, 4, 7], // major
  m: [0, 3, 7],
  dim: [0, 3, 6],
  aug: [0, 4, 8],
  sus2: [0, 2, 7],
  sus4: [0, 5, 7],
  '7': [0, 4, 7, 10],
  maj7: [0, 4, 7, 11],
  m7: [0, 3, 7, 10],
};

// Semitones above C of a letter with an optional # or b: "C#" -> 1, "Cb" -> -1.
function semitones(letter: string, accidental: string): number {
  return NOTE_LETTERS[letter] + (accidental === '#' ? 1 : accidental === 'b' ? -1 : 0);
}

// MIDI note of a note name. With an octave it is scientific pitch ("A4" = 69,
// "C5" = 72), moved by whole octaves into lowest..highest when the keyboard
// doesn't have that octave ("A2" and "A6" both give A4 or A5). Without an
// octave ("E", "F#") it is in the octave starting at `lowest`. Throws on a
// name it can't read.
export function noteNumber(note: string, lowest = LOWEST, highest = HIGHEST): number {
  const match = /^([A-G])([#b]?)(-?\d)?$/.exec(note.trim());
  if (!match) throw new Error(`unknown note "${note}"`);
  const offset = semitones(match[1], match[2]);
  if (match[3] === undefined) return lowest + ((offset + 12) % 12);
  let midi = (Number(match[3]) + 1) * 12 + offset;
  while (midi < lowest) midi += 12;
  while (midi > highest) midi -= 12;
  return midi;
}

// MIDI notes of a chord name such as "A", "C#m", "Bb7" or "Fmaj7", with the
// root in the octave starting at `lowest`. Throws on a name it can't read.
export function chordNotes(chord: string, lowest = LOWEST): number[] {
  const match = /^([A-G])([#b]?)(.*)$/.exec(chord.trim());
  const intervals = match ? CHORD_INTERVALS[match[3]] : undefined;
  if (!match || !intervals) throw new Error(`unknown chord "${chord}"`);
  const root = lowest + ((semitones(match[1], match[2]) + 12) % 12);
  return intervals.map((i) => root + i);
}

function createMaterials(theme: Theme) {
  const c = theme.colors;
  return {
    body: surface(theme, c.body),
    whiteKey: surface(theme, c.light),
    blackKey: surface(theme, c.dark),
    foot: surface(theme, c.trim),
    marker: surface(theme, c.marker),
    knob: surface(theme, c.accent),
  };
}

function mesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

// A rounded box whose bottom sits on the feet.
function roundedBox(w: number, h: number, d: number, mat: THREE.Material, x: number, z: number): THREE.Mesh {
  const radius = Math.min(BODY_RADIUS, w / 2, h / 2, d / 2) * 0.99;
  const m = mesh(new RoundedBoxGeometry(w, h, d, 4, radius), mat);
  m.position.set(x, FOOT + h / 2, z);
  return m;
}

export interface MidiPianoOptions {
  theme?: Theme;
}

export class MidiPiano extends THREE.Group {
  static readonly LOWEST = LOWEST;
  static readonly HIGHEST = HIGHEST;
  static readonly KNOB_COUNT = KNOB_COUNT;

  private readonly m: ReturnType<typeof createMaterials>;
  private readonly keys: THREE.Object3D[] = [];
  private readonly keyTargets: number[] = [];
  private readonly knobs: THREE.Object3D[] = [];
  private chord: number[] = [];
  private note = -1;

  constructor(options: MidiPianoOptions = {}) {
    super();
    this.name = 'midi-piano';
    this.m = createMaterials(options.theme ?? defaultTheme);
    this.buildBody();
    this.buildFeet();
    this.buildKeys();
    this.buildKnobs();
  }

  isBlack(midi: number): boolean {
    return BLACK_NOTES.has(midi % 12);
  }

  // Notes outside C4..B5 are ignored.
  setKeyPressed(midi: number, pressed: boolean): void {
    const i = midi - LOWEST;
    if (i < 0 || i >= this.keys.length) return;
    this.keyTargets[i] = pressed ? KEY_PRESS_ANGLE : 0;
  }

  releaseAll(): void {
    this.keyTargets.fill(0);
    this.chord = [];
    this.note = -1;
  }

  // The MIDI note of the key a part belongs to (the key or its mesh), or null
  // for any other part, such as the body; for playing it by clicking. Not in
  // the spec, MidiPiano.md.
  keyAt(part: THREE.Object3D): number | null {
    for (let object: THREE.Object3D | null = part; object && object !== this; object = object.parent) {
      const i = this.keys.indexOf(object);
      if (i >= 0) return LOWEST + i;
    }
    return null;
  }

  // Presses the keys of a chord ("A" = A major; also m, dim, aug, sus2, sus4,
  // 7, maj7, m7) with its root in the C4 octave, and releases the chord held
  // before. A key the current PlayNote note holds stays down. Returns the
  // notes pressed. Named as in the spec, MidiPiano.md.
  PlayChord(chord: string): number[] {
    const notes = chordNotes(chord);
    for (const n of this.chord) if (n !== this.note) this.setKeyPressed(n, false);
    for (const n of notes) this.setKeyPressed(n, true);
    this.chord = notes;
    return notes;
  }

  // Presses one note ("E", "F#", or with an octave, "A4" = MIDI 69) and
  // releases the note played before; it is held together with the current
  // chord, and a key the chord holds stays down. Without an octave the note is
  // in the C4 octave; an octave the keyboard lacks moves to one it has.
  // Returns the MIDI note. Named as in the spec, MidiPiano.md.
  PlayNote(note: string): number {
    const midi = noteNumber(note);
    if (!this.chord.includes(this.note)) this.setKeyPressed(this.note, false);
    this.setKeyPressed(midi, true);
    this.note = midi;
    return midi;
  }

  // value 0..1, turning from about 7 o'clock to 5 o'clock.
  setKnob(index: number, value: number): void {
    const knob = this.knobs[index];
    if (!knob) return;
    knob.rotation.y = 0.75 * Math.PI * (1 - 2 * THREE.MathUtils.clamp(value, 0, 1));
  }

  // Eases keys toward their pressed/released angle; call once per frame.
  update(delta: number): void {
    const t = 1 - Math.exp(-KEY_SPEED * delta);
    this.keys.forEach((key, i) => {
      key.rotation.x += (this.keyTargets[i] - key.rotation.x) * t;
    });
  }

  private buildBody(): void {
    const half = BODY_WIDTH / 2;
    const backDepth = KEY_BACK + BODY_DEPTH / 2;

    // Low keybed under the keys, taller panels behind and beside them.
    this.add(roundedBox(BODY_WIDTH, KEYBED_HEIGHT, BODY_DEPTH, this.m.body, 0, 0));
    this.add(roundedBox(BODY_WIDTH, BODY_HEIGHT, backDepth, this.m.body, 0, -BODY_DEPTH / 2 + backDepth / 2));
    this.add(roundedBox(LEFT_PANEL, BODY_HEIGHT, BODY_DEPTH, this.m.body, -half + LEFT_PANEL / 2, 0));
    this.add(roundedBox(RIGHT_PANEL, BODY_HEIGHT, BODY_DEPTH, this.m.body, half - RIGHT_PANEL / 2, 0));
  }

  private buildFeet(): void {
    const geo = new THREE.SphereGeometry(0.012, 24, 12);
    geo.scale(1, 0.5, 1);
    const x = BODY_WIDTH / 2 - 0.03;
    const z = BODY_DEPTH / 2 - 0.03;
    for (const [sx, sz] of [[-1, -1], [1, -1], [-1, 1], [1, 1]]) {
      const foot = mesh(geo, this.m.foot);
      foot.position.set(sx * x, 0.006, sz * z);
      this.add(foot);
    }
  }

  private buildKeys(): void {
    // Pivot at the back of each key so rotation.x > 0 dips the front.
    const whiteGeo = new RoundedBoxGeometry(WHITE_WIDTH - KEY_GAP, WHITE_HEIGHT, WHITE_LENGTH, 3, 0.0025);
    whiteGeo.translate(0, WHITE_HEIGHT / 2, WHITE_LENGTH / 2);
    const blackGeo = new RoundedBoxGeometry(BLACK_WIDTH, BLACK_HEIGHT, BLACK_LENGTH, 3, 0.002);
    blackGeo.translate(0, BLACK_HEIGHT / 2, BLACK_LENGTH / 2);

    const x0 = -BODY_WIDTH / 2 + LEFT_PANEL + WHITE_WIDTH / 2;
    const y = FOOT + KEYBED_HEIGHT;
    let whiteIndex = 0;
    for (let midi = LOWEST; midi <= HIGHEST; midi++) {
      const pivot = new THREE.Group();
      if (this.isBlack(midi)) {
        pivot.position.set(x0 + (whiteIndex - 0.5) * WHITE_WIDTH, y + WHITE_HEIGHT - 0.004, KEY_BACK);
        pivot.add(mesh(blackGeo, this.m.blackKey));
      } else {
        pivot.position.set(x0 + whiteIndex * WHITE_WIDTH, y, KEY_BACK);
        pivot.add(mesh(whiteGeo, this.m.whiteKey));
        whiteIndex++;
      }
      pivot.name = `key-${midi}`;
      this.keys.push(pivot);
      this.keyTargets.push(0);
      this.add(pivot);
    }
  }

  private buildKnobs(): void {
    const baseGeo = new THREE.CylinderGeometry(KNOB_RADIUS, KNOB_RADIUS, KNOB_HEIGHT, 32);
    baseGeo.translate(0, KNOB_HEIGHT / 2, 0);
    const domeGeo = new THREE.SphereGeometry(KNOB_RADIUS, 32, 12, 0, Math.PI * 2, 0, Math.PI / 2);
    domeGeo.scale(1, 0.3, 1);
    domeGeo.translate(0, KNOB_HEIGHT, 0);
    const dotGeo = new THREE.SphereGeometry(0.0025, 12, 8);

    const z = (-BODY_DEPTH / 2 + KEY_BACK) / 2;
    for (let i = 0; i < KNOB_COUNT; i++) {
      const knob = new THREE.Group();
      knob.add(mesh(baseGeo, this.m.knob));
      knob.add(mesh(domeGeo, this.m.knob));
      // Marker dot on the dome pointing at the knob's value.
      const dot = mesh(dotGeo, this.m.marker);
      dot.position.set(0, KNOB_HEIGHT + 0.0034, -0.008);
      knob.add(dot);
      knob.position.set((i - (KNOB_COUNT - 1) / 2) * KNOB_SPACING, FOOT + BODY_HEIGHT, z);
      this.knobs.push(knob);
      this.add(knob);
      this.setKnob(i, 0.5);
    }
  }
}
