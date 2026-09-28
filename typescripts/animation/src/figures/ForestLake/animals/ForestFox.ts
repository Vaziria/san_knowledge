import * as THREE from 'three';
import type { ForestAnimalOptions } from './ForestAnimal';
import { FourLegged, legRings, type FourLeggedBuild, type Posture } from './FourLegged';
import { blend, ear, eye, loft, patchy, paw, spike, tone, tubeRings, type Palette, type Sculpt } from './parts';

// The forest lake's fox (ForestFox.md), as its reference sheet draws it: a
// slender red fox of flat facets, bright orange, with a white bib from its
// chin down its chest, white cheeks and muzzle sides, a white line along its
// belly, black stockings and black-backed ears, amber eyes, and a huge brush
// of a tail with a white tip. Its colours are the sheet's four variations.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground
// under its middle. Traced from the sheet's side and front views in their
// pixels (U m each): 39 cm at the shoulder, as a red fox is.

const U = 0.39 / 170; // m per pixel of the sheet's side view: 170 px from the ground to its withers

function P(z: number, y: number, x = 0): THREE.Vector3 {
  return new THREE.Vector3(x * U, y * U, z * U);
}

// The sheet's four colour variations, picked from its pictures.
const VARIATIONS: Record<string, Palette> = {
  orange: { coat: 0xf27b37, dark: 0xd2612b, cream: 0xf7ebe6, sock: 0x4d342c, back: 0x33272a, inner: 0xf3d6d0, nose: 0x2a2224, iris: 0xb7661c, pupil: 0x1a1310, shine: 0xffffff, mouth: 0x3a2626 },
  white: { coat: 0xeceaf2, dark: 0xd0d0e0, cream: 0xfdfcff, sock: 0xbdb9cc, back: 0x9f9bb2, inner: 0xf3e6ea, nose: 0x2a2d38, iris: 0xc89436, pupil: 0x1a1310, shine: 0xffffff, mouth: 0x4a3336 },
  brown: { coat: 0xb77756, dark: 0x8c5c43, cream: 0xeadacb, sock: 0x4a3428, back: 0x392a23, inner: 0xdbbaa6, nose: 0x2a2224, iris: 0xd89a38, pupil: 0x1a1310, shine: 0xffffff, mouth: 0x3a2626 },
  black: { coat: 0x5c4b48, dark: 0x3e3333, cream: 0xdad4da, sock: 0x282225, back: 0x1f1b1d, inner: 0x8c7e82, nose: 0x151416, iris: 0xdca63a, pupil: 0x121010, shine: 0xffffff, mouth: 0x231c1d },
};

const UPRIGHT = new THREE.Vector3(0, 0, 1); // the body's slices stand upright

const BUILD: FourLeggedBuild = {
  variations: VARIATIONS,
  pelvis: P(-68, 138),
  chest: P(42, 128),
  neck: P(72, 160),
  head: P(92, 190),
  headScale: 1.15,
  body(s: Sculpt) {
    // Rump to the neck's end, in upright slices: the neck rises steeply
    // from the withers to the head.
    const slices = [
      { z: -94, y: 146, w: 9, up: 9, down: 11 },
      { z: -84, y: 141, w: 21, up: 21, down: 26 },
      { z: -64, y: 139, w: 26, up: 31, down: 36 },
      { z: -36, y: 134, w: 24, up: 36, down: 39 },
      { z: -6, y: 131, w: 25, up: 40, down: 42 },
      { z: 24, y: 129, w: 28, up: 42, down: 46 },
      { z: 46, y: 132, w: 29, up: 43, down: 49 },
      { z: 62, y: 147, w: 27, up: 38, down: 44 },
      { z: 76, y: 166, w: 23, up: 36, down: 38 },
      { z: 88, y: 182, w: 19, up: 28, down: 28 },
      { z: 97, y: 194, w: 15, up: 20, down: 21 },
    ];
    const body = loft(
      s,
      slices.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U, normal: UPRIGHT })),
      {
        sides: 8,
        start: 6 * U,
        end: 'flat',
        paint: (r, angle, k) => {
          const z = slices[Math.max(0, Math.min(slices.length - 1, r))].z;
          const fromBelow = Math.abs(angle - Math.PI);
          if (z > 40 && fromBelow < 1.25) return tone('cream'); // the bib down the throat and chest
          if (z > 20 && fromBelow < 0.5) return tone('cream');
          if (z > -70 && z < 30 && fromBelow < 0.42) return blend('cream', 'coat', 0.15); // the pale line along the belly
          if (fromBelow > 2.4) return patchy('coat', 'dark', r, k, 0.4);
          return patchy('coat', 'dark', r, k, 0.3, 0.25);
        },
      },
    );
    // Tufts of pale fur at the bib's edges, down the chest.
    for (const side of [-1, 1]) {
      for (const [z, y, out] of [
        [70, 150, 20],
        [58, 125, 25],
      ]) {
        spike(s, P(z + 3, y + 8, side * (out - 2)), P(z + 3, y - 8, side * (out - 3)), P(z - 10, y - 16, side * (out + 5)), new THREE.Vector3(side * 3 * U, 0, 0), tone('cream'), tone('coat', 0.8));
      }
    }
    void body;
  },
  buildHead(s: Sculpt, shiny: Sculpt) {
    // A wide face narrowing fast to a long, pointed snout, down to the
    // mouth; the lower jaw is its own piece.
    const rings = [
      { z: 68, y: 198, w: 26, up: 16, down: 22 },
      { z: 84, y: 202, w: 34, up: 14, down: 30 },
      { z: 100, y: 199, w: 38, up: 14, down: 30 },
      { z: 114, y: 182, w: 33, up: 13, down: 18 },
      { z: 126, y: 177, w: 21, up: 11, down: 13 },
      { z: 140, y: 173, w: 13, up: 9, down: 9 },
      { z: 154, y: 169, w: 9, up: 7, down: 6 },
      { z: 164, y: 167, w: 6, up: 5, down: 4 },
    ];
    loft(
      s,
      rings.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U })),
      {
        sides: 8,
        turn: Math.PI / 8,
        start: 'flat',
        end: 5 * U,
        paint: (r, angle, k) => {
          const top = Math.min(angle, 2 * Math.PI - angle);
          if (top < 1.2) return r >= 5 ? blend('coat', 'dark', 0.2) : patchy('coat', 'dark', r, k, 0.3);
          if (top < 2) return r >= 3 ? tone('cream') : patchy('coat', 'dark', r, k); // white cheeks and muzzle sides
          return tone('cream');
        },
      },
    );
    // Eyes: amber almonds set wide on the face, slanting up at their outer
    // corners, seated where a ray from in front meets it.
    for (const side of [-1, 1]) {
      const look = new THREE.Vector3(side * 0.55, 0.15, 0.82).normalize();
      const at = s.onto(P(200, 190, side * 19), new THREE.Vector3(0, 0, -1)).addScaledVector(look, -0.8 * U);
      eye(shiny, at, look, new THREE.Vector3(0, 1, 0), 5.2 * U, { lid: tone('pupil'), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 8, 0.5, 0.5, 1.55, side * -0.25);
    }
    // The cheek ruff: white points flaring out and back below the eyes.
    for (const side of [-1, 1]) {
      for (const [z, y, out, length, reach] of [
        [104, 182, 36, 13, 15],
        [99, 194, 35, 11, 12],
        [101, 170, 30, 12, 12],
      ]) {
        spike(s, P(z + 6, y + 10, side * (out - 5)), P(z + 6, y - 10, side * (out - 6)), P(z - length, y - 8, side * (out + reach)), new THREE.Vector3(side * 3 * U, 0, 0), tone('cream'), tone('coat', 0.85));
      }
    }
    // Ears: tall and pointed, orange in front with black backs, white
    // inside.
    for (const side of [-1, 1]) {
      ear(s, P(98, 208, side * 26), P(101, 254, side * 35), new THREE.Vector3(side * 0.92, -0.1, -0.38), new THREE.Vector3(side * 0.38, 0.05, 0.92), 44 * U, 12 * U, 0.3, tone('back'), tone('inner'), tone('coat', 1.04));
    }
    // The nose, a small black button on the snout's tip.
    const nose = [
      { z: 161, y: 168, w: 6, up: 4.5, down: 3.5 },
      { z: 166, y: 168.5, w: 6.5, up: 4.5, down: 3.5 },
      { z: 170, y: 168, w: 4.5, up: 3, down: 2.5 },
    ].map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U }));
    loft(shiny, nose, { sides: 6, turn: Math.PI / 6, start: 'flat', end: 1.8 * U, paint: () => tone('nose') });
  },
  jaw: {
    pivot: P(94, 162),
    build(s: Sculpt) {
      const rings = [
        { z: 90, y: 160, w: 21, up: 6, down: 9 },
        { z: 110, y: 160, w: 15, up: 5, down: 6 },
        { z: 128, y: 161, w: 10, up: 4, down: 4.5 },
        { z: 145, y: 162, w: 7, up: 3, down: 3 },
        { z: 156, y: 162.5, w: 4.5, up: 2, down: 2 },
      ];
      loft(
        s,
        rings.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U })),
        { sides: 6, start: 'flat', end: 2.5 * U, paint: (r, angle) => (r === 0 && Math.min(angle, 2 * Math.PI - angle) < 0.6 ? tone('mouth') : tone('cream')) },
      );
    },
  },
  tail: {
    joints: [P(-88, 150), P(-108, 140), P(-128, 118), P(-146, 86), P(-158, 54)],
    tip: P(-164, 34),
    radii: [12, 24, 32, 31, 17].map((r) => r * U),
    build(s: Sculpt) {
      // A great brush: thickest halfway, white for its last third.
      const path = [P(-84, 152), P(-96, 148), P(-112, 138), P(-129, 116), P(-145, 86), P(-156, 58), P(-162, 38)];
      const radii = [9, 17, 27, 32, 31, 20, 6].map((r) => r * U);
      loft(s, tubeRings(path, radii), {
        sides: 7,
        start: 'flat',
        end: 10 * U,
        paint: (r, angle, k) => {
          if (r >= 4) return r === 4 && Math.abs(angle - Math.PI) > 1.6 ? blend('cream', 'coat', 0.3) : tone('cream');
          return patchy('coat', 'dark', r, k, 0.35, 0.35);
        },
      });
    },
  },
  front: {
    hip: P(42, 128, 24),
    knee: P(51, 77, 24),
    ankle: P(57, 25, 24),
    paw: P(59, 8, 24),
    bend: -1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.35, w: 13 * U, d: 17 * U },
        { t: 0, w: 14 * U, d: 18 * U },
        { t: 0.5, w: 11 * U, d: 13 * U },
        { t: 1, w: 8.5 * U, d: 9 * U, back: 10 * U },
        { t: 1.5, w: 7 * U, d: 7.5 * U },
        { t: 2, w: 6.5 * U, d: 6.5 * U },
        { t: 2.5, w: 6 * U, d: 6 * U },
        { t: 3, w: 6.5 * U, d: 6 * U, dz: 1 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        // Orange down to the elbow, black stockings below.
        paint: (r) => (r <= 1 ? patchy('coat', 'dark', r, 3) : r === 2 ? blend('sock', 'coat', 0.35) : tone('sock')),
      });
      paw(s, joints[3], side, 0.95 * U, tone('sock'), tone('sock', 0.9), tone('sock', 0.6));
    },
  },
  hind: {
    hip: P(-68, 138, 24),
    knee: P(-44, 76, 24),
    ankle: P(-82, 45, 24),
    paw: P(-84, 8, 24),
    bend: 1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.3, w: 17 * U, d: 24 * U },
        { t: 0, w: 19 * U, d: 26 * U },
        { t: 0.5, w: 15 * U, d: 19 * U, dz: 2 * U },
        { t: 1, w: 9 * U, d: 10 * U },
        { t: 1.5, w: 7 * U, d: 8 * U, back: 9.5 * U },
        { t: 2, w: 6 * U, d: 6 * U, back: 7.5 * U },
        { t: 2.5, w: 5.5 * U, d: 5.5 * U },
        { t: 3, w: 6 * U, d: 5.5 * U, dz: 1 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        // Orange thighs, darkening down the shin to black below the hock.
        paint: (r) => (r <= 2 ? patchy('coat', 'dark', r, 4) : r === 3 ? blend('coat', 'sock', 0.6) : tone('sock')),
      });
      paw(s, joints[3], side, 0.9 * U, tone('sock'), tone('sock', 0.9), tone('sock', 0.6));
    },
  },
  walk: {
    stride: 0.24,
    cadence: 1.5,
    duty: 0.64,
    offsets: [0.25, 0.75, 0, 0.5],
    lift: [0.04, 0.035],
    fold: [0.9, 0.7],
    sink: 0.018,
    bob: 0.006,
    bounces: 2,
    rock: 0.02,
    flex: 0.02,
    nod: 0.04,
    lag: 0.1,
  },
  run: {
    stride: 0.3,
    cadence: 3.2,
    duty: 0.34,
    offsets: [0.56, 0.66, 0.02, 0.12],
    lift: [0.07, 0.06],
    fold: [1.5, 1.1],
    sink: 0.05,
    bob: 0.018,
    bounces: 1,
    rock: 0.09,
    flex: 0.14,
    nod: 0.05,
    lag: 0.72,
  },
};

export class ForestFox extends FourLegged {
  // The sheet's colour variations, the default (orange) first.
  static readonly COLORS = Object.keys(VARIATIONS);

  constructor(options: ForestAnimalOptions = {}) {
    super(BUILD, options);
  }

  // Sits on its haunches, front legs straight, its brush curled round its
  // feet, until told otherwise. Named as in its sheet's poses.
  Sit(): void {
    this.setMode('sit', 'stand');
  }

  // Sits and lifts its head to look up at the sky, for a few seconds, then
  // sits on. Named as in its sheet's poses.
  LookUp(): void {
    this.setMode('lookUp', 'stand');
  }

  protected pose(goal: Posture): void {
    super.pose(goal);
    const sitting = this.mode === 'sit' || this.mode === 'lookUp';
    if (sitting) {
      goal.rear = 0.13;
      goal.sit = 1;
      goal.hindReach = 0.02;
      goal.tail = -0.5;
      goal.tailTurn = 1.1;
      goal.tailCurl = 0.35;
      goal.neck = -0.25;
      goal.head = 0.1 + 0.08 * Math.sin(this.time * 0.4);
      goal.headTurn = 0.2 * Math.sin(this.time * 0.3);
    }
    if (this.mode === 'lookUp') {
      const t = this.modeTime;
      const up = t > 0.6 && t < 4.2 ? 1 : 0;
      goal.neck -= 0.25 * up;
      goal.head = -0.75 * up + (1 - up) * goal.head;
      goal.headTurn *= 1 - up;
      goal.headTilt = 0.12 * up * Math.sin(t * 0.8);
      if (t > 5) this.Sit();
    }
  }
}
