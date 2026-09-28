import * as THREE from 'three';
import type { ForestAnimalOptions } from './ForestAnimal';
import { FourLegged, legRings, type FourLeggedBuild, type Posture } from './FourLegged';
import { blend, ear, eye, loft, patchy, spike, tone, tubeRings, type Palette, type Sculpt } from './parts';

// The forest lake's wild boar (WildBoar.md), as its reference sheet draws
// it: a heavy boar of flat facets, brown, with a hump at its shoulders, a
// crest of dark bristles along its spine, a big wedge of a head ending in a
// flat snout disc with two nostrils, curved white tusks, small dark eyes,
// pointed ears, short thick legs darkening to black at the hooves, and a
// thin tail with a tuft. Its colours are the sheet's four variations.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground
// under its middle. Traced from the sheet's side and front views in their
// pixels (U m each): 85 cm at the shoulder, as a grown boar is.

const U = 0.85 / 188; // m per pixel of the sheet's side view: 188 px from the ground to its hump

function P(z: number, y: number, x = 0): THREE.Vector3 {
  return new THREE.Vector3(x * U, y * U, z * U);
}

// A point of the head, traced from the side view and then shortened a
// fifth toward the back of the head, as the sheet's front and 3/4 views
// show its head shorter than its side view does.
function PH(z: number, y: number, x = 0): THREE.Vector3 {
  return P(72 + (z - 72) * 0.82, y, x);
}

// The sheet's four colour variations, picked from its pictures.
const VARIATIONS: Record<string, Palette> = {
  brown: { coat: 0xa97058, dark: 0x6e4a3c, legs: 0x3a2c2a, mane: 0x5e4a42, snout: 0x4a3a3c, nostril: 0x1e1719, tusk: 0xf1ebe0, hoof: 0x231d1f, inner: 0x3a2a28, iris: 0x1c1818, pupil: 0x0e0c0c, shine: 0xffffff, mouth: 0x2a1e1e },
  dark: { coat: 0x7d5e53, dark: 0x574543, legs: 0x2c2426, mane: 0x3e3230, snout: 0x3a3032, nostril: 0x161213, tusk: 0xefe8dc, hoof: 0x1c1718, inner: 0x2c2224, iris: 0x1c1818, pupil: 0x0e0c0c, shine: 0xffffff, mouth: 0x221a1a },
  gray: { coat: 0xa5877c, dark: 0x86695f, legs: 0x5a4a46, mane: 0x6e5a54, snout: 0x5a4a4c, nostril: 0x1e1719, tusk: 0xf3eee6, hoof: 0x2a2426, inner: 0x4a3c3c, iris: 0x1c1818, pupil: 0x0e0c0c, shine: 0xffffff, mouth: 0x2a1e1e },
  snow: { coat: 0xf0dcd0, dark: 0xdccabf, legs: 0xc8b4aa, mane: 0xe2d0c6, snout: 0xc9a8a4, nostril: 0x5a4444, tusk: 0xfffaf2, hoof: 0x6a5a58, inner: 0xd0b0aa, iris: 0x1c1818, pupil: 0x0e0c0c, shine: 0xffffff, mouth: 0x5a4444 },
};

const UPRIGHT = new THREE.Vector3(0, 0, 1);

// A boar's cloven hoof under a foot's joint: two dark toes on the ground,
// short and broad.
function hoof(s: Sculpt, at: THREE.Vector3, side: number): void {
  const x = new THREE.Vector3(side, 0, 0);
  for (const toe of [-1, 1]) {
    const base = at.clone().addScaledVector(x, toe * 4 * U);
    const rings = [
      { y: 2, z: 0, w: 4.6, d: 7.5 },
      { y: -6, z: 2, w: 5, d: 8.5 },
      { y: -12.8, z: 3.5, w: 5.2, d: 9 },
    ].map((r) => ({ at: new THREE.Vector3(base.x, at.y + r.y * U, at.z + r.z * U), w: r.w * U, up: r.d * U, down: r.d * U, normal: new THREE.Vector3(0, -1, 0.12) }));
    loft(s, rings, { sides: 6, side: x, mirror: side < 0, start: 'open', end: 'flat', paint: () => tone('hoof') });
  }
}

const BUILD: FourLeggedBuild = {
  variations: VARIATIONS,
  pelvis: P(-92, 128),
  chest: P(42, 118),
  neck: P(64, 130),
  head: P(78, 142),
  body(s: Sculpt) {
    // A barrel, deepest and highest at the shoulders.
    const slices = [
      { z: -140, y: 145, w: 14, up: 10, down: 14 },
      { z: -126, y: 138, w: 34, up: 25, down: 40 },
      { z: -100, y: 128, w: 44, up: 36, down: 53 },
      { z: -60, y: 124, w: 46, up: 48, down: 50 },
      { z: -20, y: 124, w: 50, up: 54, down: 56 },
      { z: 20, y: 125, w: 54, up: 62, down: 60 },
      { z: 45, y: 124, w: 54, up: 64, down: 58 },
      { z: 65, y: 128, w: 48, up: 60, down: 46 },
      { z: 82, y: 138, w: 40, up: 48, down: 42 },
    ];
    const body = loft(
      s,
      slices.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U, normal: UPRIGHT })),
      {
        sides: 8,
        start: 8 * U,
        end: 'flat',
        paint: (r, angle, k) => {
          const fromBelow = Math.abs(angle - Math.PI);
          if (fromBelow < 0.6) return blend('coat', 'dark', 0.45); // the dark belly
          if (fromBelow > 2.6) return patchy('dark', 'coat', r, k, 0.4, 0.4); // darker along the spine
          return patchy('coat', 'dark', r, k, 0.4, 0.35);
        },
      },
    );
    // The crest: bristles in a row along the spine, tallest over the neck
    // and shoulders, leaning back.
    const crest: [z: number, height: number][] = [
      [74, 22],
      [60, 27],
      [46, 26],
      [30, 24],
      [14, 20],
      [-2, 17],
      [-18, 13],
      [-34, 10],
      [-50, 7],
    ];
    for (const [z, height] of crest) {
      // On the spine at z: the top of the slice there.
      const i = slices.findIndex((sl) => sl.z > z);
      const a = slices[Math.max(0, i - 1)];
      const b = slices[Math.max(0, i)];
      const f = b.z === a.z ? 0 : (z - a.z) / (b.z - a.z);
      const top = a.y + a.up + (b.y + b.up - a.y - a.up) * f;
      for (const x of [-4, 4]) {
        spike(s, P(z + 7, top - 4, x - 3), P(z - 7, top - 4, x + 3), P(z - 12, top + height, x * 1.6), new THREE.Vector3(x * U, 0, 0), tone('mane'), tone('dark', 0.8));
      }
    }
    void body;
  },
  buildHead(s: Sculpt, shiny: Sculpt) {
    // A big wedge sloping down to the snout's flat disc.
    const rings = [
      { z: 72, y: 146, w: 38, up: 46, down: 44 },
      { z: 90, y: 144, w: 41, up: 42, down: 44 },
      { z: 106, y: 134, w: 35, up: 32, down: 38 },
      { z: 122, y: 121, w: 28, up: 23, down: 30 },
      { z: 138, y: 108, w: 22, up: 17, down: 21 },
      { z: 152, y: 99, w: 19.5, up: 13, down: 14 },
      { z: 160, y: 96, w: 20.5, up: 13.5, down: 14.5 },
    ];
    const head = loft(
      s,
      rings.map((r) => ({ at: PH(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U })),
      {
        sides: 8,
        turn: Math.PI / 8,
        start: 'flat',
        end: 'flat',
        paint: (r, angle, k) => {
          if (r === rings.length - 1) return tone('snout'); // the disc
          const top = Math.min(angle, 2 * Math.PI - angle);
          if (r >= 5) return blend('snout', 'coat', 0.35);
          if (top > 2.4) return blend('coat', 'dark', 0.5); // under the jaw
          if (top < 0.6 && r >= 2) return blend('coat', 'dark', 0.2);
          return patchy('coat', 'dark', r, k, 0.35, 0.35);
        },
      },
    );
    // Nostrils: two dark slots in the disc.
    const disc = rings[rings.length - 1];
    for (const side of [-1, 1]) {
      const c = PH(disc.z + 0.3, disc.y - 1, side * 7.5);
      const a = c.clone().add(P(0, 6, 0));
      const b = c.clone().add(P(0, -5, side * 3.2));
      const d = c.clone().add(P(0, -5, -side * 3.2));
      s.toward(a, b, d, new THREE.Vector3(0, 0, 1), tone('nostril'));
    }
    // Tusks: the big lower ones curving out and up from the snout's sides,
    // and small upper ones.
    for (const side of [-1, 1]) {
      loft(s, tubeRings([PH(132, 88, side * 18), PH(130, 99, side * 28), PH(126, 111, side * 35), PH(120, 122, side * 38)], [5.5, 4.6, 3.2, 1.5].map((r) => r * U)), {
        sides: 5,
        start: 'flat',
        end: 4 * U,
        paint: (r) => tone('tusk', r >= 2 ? 1.02 : 0.96),
      });
      loft(s, tubeRings([PH(148, 92, side * 17), PH(149, 98, side * 21), PH(150, 104, side * 24)], [2.8, 2, 1].map((r) => r * U)), {
        sides: 5,
        start: 'flat',
        end: 2 * U,
        paint: () => tone('tusk'),
      });
    }
    // Small dark eyes, set on the sides of the head.
    for (const side of [-1, 1]) {
      const look = new THREE.Vector3(side * 0.6, 0.2, 0.78).normalize();
      const at = s.onto(PH(200, 146, side * 22), new THREE.Vector3(0, 0, -1)).addScaledVector(look, -0.5 * U);
      eye(shiny, at, look, new THREE.Vector3(0, 1, 0), 3.4 * U, { lid: tone('dark', 0.8), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 7, 0.6, 0.45, 1.5, side * -0.2);
    }
    // Pointed ears standing up and out, dark inside.
    for (const side of [-1, 1]) {
      ear(s, PH(80, 184, side * 23), PH(85, 216, side * 36), new THREE.Vector3(side, 0, -0.3), new THREE.Vector3(side * 0.35, 0.1, 1), 24 * U, 7 * U, 0.28, patchy('coat', 'dark', 1, side), tone('inner'));
    }
    void head;
  },
  tail: {
    joints: [P(-130, 146), P(-142, 126), P(-149, 102)],
    tip: P(-151, 80),
    radii: [4, 3.5, 3].map((r) => r * U),
    build(s: Sculpt) {
      // Thin, hanging, ending in a dark tuft.
      const path = [P(-128, 148), P(-136, 140), P(-143, 124), P(-148, 104), P(-150, 90)];
      loft(s, tubeRings(path, [4, 3.6, 3, 2.6, 2.2].map((r) => r * U)), { sides: 5, start: 'flat', end: 'open', paint: () => tone('coat', 0.9) });
      for (let k = 0; k < 5; k++) {
        const angle = (k / 5) * Math.PI * 2;
        const x = Math.cos(angle) * 3;
        const z = Math.sin(angle) * 3;
        spike(s, P(-150 + z, 94, x), P(-150 - z, 94, -x), P(-151 + z * 1.6, 76, x * 1.6), P(0, 0, 0).set(x * U * 0.3, 0, z * U * 0.3), tone('mane'), tone('mane', 0.8));
      }
    },
  },
  front: {
    hip: P(45, 112, 30),
    knee: P(40, 72, 30),
    ankle: P(50, 38, 30),
    paw: P(52, 14, 30),
    bend: -1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.4, w: 22 * U, d: 30 * U },
        { t: 0, w: 24 * U, d: 30 * U },
        { t: 0.5, w: 20 * U, d: 24 * U },
        { t: 1, w: 14 * U, d: 15 * U, back: 17 * U },
        { t: 1.5, w: 11 * U, d: 12 * U },
        { t: 2, w: 10 * U, d: 10 * U },
        { t: 2.5, w: 9 * U, d: 9 * U },
        { t: 3, w: 9.5 * U, d: 9.5 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        // Brown above, darkening down to black-brown shins.
        paint: (r) => (r <= 1 ? patchy('coat', 'dark', r, 5, 0.4) : r === 2 ? blend('dark', 'legs', 0.3) : tone('legs')),
      });
      hoof(s, joints[3], side);
    },
  },
  hind: {
    hip: P(-92, 128, 30),
    knee: P(-80, 78, 30),
    ankle: P(-122, 40, 30),
    paw: P(-118, 14, 30),
    bend: 1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.3, w: 28 * U, d: 40 * U },
        { t: 0, w: 30 * U, d: 42 * U },
        { t: 0.5, w: 24 * U, d: 32 * U, dz: 3 * U },
        { t: 1, w: 15 * U, d: 17 * U },
        { t: 1.5, w: 11 * U, d: 13 * U, back: 15 * U },
        { t: 2, w: 9.5 * U, d: 10 * U, back: 12 * U },
        { t: 2.5, w: 9 * U, d: 9 * U },
        { t: 3, w: 9.5 * U, d: 9.5 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        paint: (r) => (r <= 2 ? patchy('coat', 'dark', r, 6, 0.4) : r === 3 ? blend('dark', 'legs', 0.4) : tone('legs')),
      });
      hoof(s, joints[3], side);
    },
  },
  walk: {
    stride: 0.36,
    cadence: 1.3,
    duty: 0.65,
    offsets: [0.25, 0.75, 0, 0.5],
    lift: [0.05, 0.05],
    fold: [0.9, 0.7],
    sink: 0.03,
    bob: 0.01,
    bounces: 2,
    rock: 0.02,
    flex: 0.015,
    nod: 0.05,
    lag: 0.1,
  },
  run: {
    stride: 0.5,
    cadence: 2.6,
    duty: 0.36,
    offsets: [0.56, 0.66, 0.02, 0.12],
    lift: [0.09, 0.08],
    fold: [1.3, 1.0],
    sink: 0.06,
    bob: 0.03,
    bounces: 1,
    rock: 0.07,
    flex: 0.08,
    nod: 0.06,
    lag: 0.72,
  },
};

export class WildBoar extends FourLegged {
  // The sheet's colour variations, the default (brown) first.
  static readonly COLORS = Object.keys(VARIATIONS);

  constructor(options: ForestAnimalOptions = {}) {
    super(BUILD, options);
  }

  // Lowers its head, tusks forward, and charges forward at a gallop for a
  // few seconds, then slows and stands. Named as in its sheet's poses.
  Charge(): void {
    this.setMode('charge', 'run');
  }

  // Lowers its snout to the ground and roots about for food, until told
  // otherwise. Named as in its sheet's poses ("Eat / Forage").
  Eat(): void {
    this.setMode('eat', 'stand');
  }

  protected pose(goal: Posture): void {
    super.pose(goal);
    const t = this.modeTime;
    if (this.mode === 'charge') {
      goal.neck = 0.25;
      goal.head = 0.3;
      goal.tail = 0.5;
      goal.sink = 0.03;
      if (t > 3) this.Idle();
    }
    if (this.mode === 'eat') {
      // Snout down among the leaves, rooting back and forth.
      goal.front = 0.07;
      goal.neck = 0.55;
      goal.head = 0.62 + 0.07 * Math.sin(t * 5.5);
      goal.headTurn = 0.12 * Math.sin(t * 1.3);
      goal.tailWag = 0.25;
    }
  }
}
