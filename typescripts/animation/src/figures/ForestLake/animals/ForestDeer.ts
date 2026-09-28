import * as THREE from 'three';
import type { ForestAnimalOptions } from './ForestAnimal';
import { FourLegged, legRings, type FourLeggedBuild, type Posture } from './FourLegged';
import { blend, eye, loft, patchy, tone, tubeRings, type Palette, type Sculpt, type Tone } from './parts';

// The forest lake's deer (ForestDeer.md), as its reference sheet draws it:
// a stag of flat facets, warm tan, with a white throat, belly, muzzle and
// rump, long slender legs pale down their fronts on black cloven hooves, a
// long neck held high, a slim head with big pale-lined ears and dark eyes,
// branching bone antlers, and a short tail white underneath. Its colours
// are the sheet's five variations.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground
// under its middle. Traced from the sheet's side and front views in their
// pixels (U m each): 1 m at the shoulder, as a grown stag is.

const U = 1.0 / 174; // m per pixel of the sheet's side view: 174 px from the ground to its withers

function P(z: number, y: number, x = 0): THREE.Vector3 {
  return new THREE.Vector3(x * U, y * U, z * U);
}

// The sheet's five colour variations, picked from its pictures.
const VARIATIONS: Record<string, Palette> = {
  brown: { coat: 0xc07e56, dark: 0x9c6446, cream: 0xf2e2d8, hoof: 0x3a3033, nose: 0x2d2c33, antler: 0xe9bf98, tip: 0xf7dfc4, inner: 0xe8b0a8, iris: 0x3a2620, pupil: 0x140f0e, shine: 0xffffff, mouth: 0x3a2626 },
  winter: { coat: 0xc9c2cc, dark: 0xa9a1b0, cream: 0xf5f2f5, hoof: 0x4a4550, nose: 0x2d2c33, antler: 0xefe3d2, tip: 0xfaf5ec, inner: 0xe6c8cc, iris: 0x3a2620, pupil: 0x140f0e, shine: 0xffffff, mouth: 0x3a2e30 },
  autumn: { coat: 0xc46a44, dark: 0x9a4f33, cream: 0xf2ddd0, hoof: 0x3a3033, nose: 0x2d2c33, antler: 0xe6bb92, tip: 0xf5dcbe, inner: 0xe8a898, iris: 0x3a2620, pupil: 0x140f0e, shine: 0xffffff, mouth: 0x3a2626 },
  dark: { coat: 0x735850, dark: 0x52403b, cream: 0xbba79c, hoof: 0x262124, nose: 0x1d1c20, antler: 0xcdb699, tip: 0xe3d3bc, inner: 0xb08a84, iris: 0x2a1c18, pupil: 0x100c0b, shine: 0xffffff, mouth: 0x2a2020 },
  snow: { coat: 0xeeeaee, dark: 0xd6d0d6, cream: 0xffffff, hoof: 0x5a5560, nose: 0x2d2c33, antler: 0xf7f1ea, tip: 0xffffff, inner: 0xf0dcdc, iris: 0x3a2620, pupil: 0x140f0e, shine: 0xffffff, mouth: 0x3a2e30 },
};

const UPRIGHT = new THREE.Vector3(0, 0, 1);

// A cloven hoof under a foot's joint: the pastern down from the fetlock,
// then two dark toes side by side on the ground.
function hoof(s: Sculpt, at: THREE.Vector3, side: number): void {
  const x = new THREE.Vector3(side, 0, 0);
  const pastern = [
    { y: 0, dz: 0, w: 5.2, d: 5.5 },
    { y: -7, dz: 1.5, w: 5.4, d: 5.8 },
  ].map((r) => ({ at: new THREE.Vector3(at.x, at.y + r.y * U, at.z + r.dz * U), w: r.w * U, up: r.d * U, down: r.d * U }));
  loft(s, pastern, { sides: 6, side: new THREE.Vector3(0, 0, 1), start: 'open', end: 'open', paint: () => tone('cream', 0.95) });
  for (const toe of [-1, 1]) {
    const base = at.clone().addScaledVector(x, toe * 2.6 * U);
    const rings = [
      { y: -6, z: 1, w: 2.8, d: 4.8 },
      { y: -11, z: 2.2, w: 3.2, d: 5.6 },
      { y: -16.5, z: 3.2, w: 3.3, d: 6.2 },
    ].map((r) => ({ at: new THREE.Vector3(base.x, at.y + r.y * U, at.z + r.z * U), w: r.w * U, up: r.d * U, down: r.d * U, normal: new THREE.Vector3(0, -1, 0.15) }));
    loft(s, rings, { sides: 6, side: x, mirror: side < 0, start: 'open', end: 'flat', paint: (r) => (r === 0 ? tone('hoof', 1.1) : tone('hoof')) });
  }
}

// A big leaf of an ear, as the sheet draws the deer's: narrow at its root
// in the head, widest a little below its middle, to a point; hollowed on
// the side it faces, `facing`, and pale there inside a rim of its coat.
function leafEar(s: Sculpt, base: THREE.Vector3, tip: THREE.Vector3, facing: THREE.Vector3, width: number, cup: number, outer: Tone, inner: Tone): void {
  const length = tip.distanceTo(base);
  const up = tip.clone().sub(base).normalize();
  const f = facing.clone().addScaledVector(up, -facing.dot(up)).normalize();
  const a = new THREE.Vector3().crossVectors(up, f).normalize();
  const at = (t: number, across: number) => base.clone().addScaledVector(up, t * length).addScaledVector(a, across * width);
  // Its outline, round from the root's one side to the other.
  const rim = [at(0, -0.22), at(0.42, -0.5), at(0.75, -0.3), tip.clone(), at(0.75, 0.3), at(0.42, 0.5), at(0, 0.22)];
  // The pale inside, a ring within the rim pushed back into the hollow,
  // round its deepest point.
  const middle = at(0.42, 0);
  const inside = rim.map((p) => p.clone().lerp(middle, 0.2).addScaledVector(f, -cup * width * 0.5));
  const hollow = middle.clone().addScaledVector(f, -cup * width);
  const back = middle.clone().addScaledVector(f, -cup * width * 1.8);
  for (let i = 0; i + 1 < rim.length; i++) {
    const [p, q] = [rim[i], rim[i + 1]];
    s.toward(p, q, inside[i + 1], f, outer);
    s.toward(p, inside[i + 1], inside[i], f, outer);
    s.toward(inside[i], inside[i + 1], hollow, f, inner);
    s.toward(q, p, back, f.clone().negate(), outer);
  }
  // Closed across its root, inside the head.
  s.toward(rim[rim.length - 1], rim[0], back, up.clone().negate(), outer);
}

// A branch of antler from `path[0]`, `radii` thick along it.
function tine(s: Sculpt, path: THREE.Vector3[], radii: number[]): void {
  loft(s, tubeRings(path, radii.map((r) => r * U)), {
    sides: 5,
    start: 'flat',
    end: radii[radii.length - 1] * U * 2.5,
    paint: (r): Tone => (r >= path.length - 2 ? tone('tip') : tone('antler', 0.95 + 0.05 * (r % 2))),
  });
}

const BUILD: FourLeggedBuild = {
  variations: VARIATIONS,
  pelvis: P(-62, 150),
  chest: P(42, 138),
  neck: P(74, 160),
  head: P(100, 200),
  headScale: 1.2,
  body(s: Sculpt) {
    // Rump to the neck's end, in upright slices: a long neck held high.
    const slices = [
      { z: -86, y: 150, w: 11, up: 10, down: 12 },
      { z: -76, y: 146, w: 25, up: 18, down: 30 },
      { z: -56, y: 141, w: 31, up: 26, down: 36 },
      { z: -30, y: 139, w: 29, up: 33, down: 36 },
      { z: 0, y: 138, w: 31, up: 36, down: 36 },
      { z: 25, y: 138, w: 35, up: 37, down: 38 },
      { z: 46, y: 137, w: 35, up: 37, down: 36 },
      { z: 60, y: 151, w: 29, up: 45, down: 45 },
      { z: 72, y: 156, w: 25, up: 47, down: 46 },
      { z: 85, y: 168, w: 21, up: 42, down: 42 },
      { z: 97, y: 180, w: 18, up: 36, down: 37 },
      { z: 108, y: 190, w: 16, up: 30, down: 31 },
    ];
    loft(
      s,
      slices.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U, normal: UPRIGHT })),
      {
        sides: 8,
        start: 7 * U,
        end: 'flat',
        paint: (r, angle, k) => {
          const z = slices[Math.max(0, Math.min(slices.length - 1, r))].z;
          const fromBelow = Math.abs(angle - Math.PI);
          if (z > 55 && fromBelow < 1.2) return tone('cream'); // the white throat and chest
          if (z > -60 && z < 40 && fromBelow < 0.45) return tone('cream'); // the pale belly
          if (z < -70 && fromBelow > 1.8 && fromBelow < 2.6) return tone('cream'); // the white rump patch
          if (fromBelow > 2.5) return patchy('coat', 'dark', r, k, 0.35, 0.4);
          return patchy('coat', 'dark', r, k, 0.28, 0.25);
        },
      },
    );
  },
  buildHead(s: Sculpt, shiny: Sculpt) {
    // A slim head, widest at the eyes, tapering to a narrow muzzle, down to
    // the mouth; the lower jaw is its own piece.
    const rings = [
      { z: 92, y: 205, w: 17, up: 14, down: 16 },
      { z: 104, y: 208, w: 21, up: 17, down: 18 },
      { z: 118, y: 203, w: 18.5, up: 13, down: 20 },
      { z: 132, y: 196, w: 13.5, up: 10, down: 14 },
      { z: 146, y: 190, w: 10.5, up: 8, down: 9 },
      { z: 156, y: 186, w: 8, up: 6, down: 6 },
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
          if (top > 2.2) return tone('cream'); // chin and throat
          if (r >= 4 && top > 1.2) return tone('cream'); // the pale muzzle, either side of the nose
          if (r === 5 && top > 0.5) return blend('coat', 'cream', 0.6); // round the nose
          if (r === 3 && top > 1.2) return blend('coat', 'cream', 0.55); // back along the lip to under the eyes
          if (r >= 3 && top < 0.5) return blend('coat', 'dark', 0.3); // the darker bridge of the nose
          if (r === 2 && top > 0.5 && top < 1.2) return blend('coat', 'cream', 0.3); // pale brows over the eyes
          return patchy('coat', 'dark', r, k, 0.25);
        },
      },
    );
    // Eyes: dark and gentle, set on the sides of the face.
    for (const side of [-1, 1]) {
      const look = new THREE.Vector3(side * 0.82, 0.1, 0.56).normalize();
      const at = s.onto(P(124, 205, side * 60), new THREE.Vector3(-side, 0, 0)).addScaledVector(look, -0.6 * U);
      eye(shiny, at, look, new THREE.Vector3(0, 1, 0), 3.6 * U, { lid: blend('cream', 'coat', 0.3), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 8, 0.6, 0.45, 1.5, side * -0.2);
    }
    // Ears: big leaves standing out to the sides, pale inside and turned
    // forward, as the front view shows them whole.
    for (const side of [-1, 1]) {
      leafEar(s, P(101, 213, side * 14), P(91, 237, side * 46), new THREE.Vector3(side * 0.25, 0.1, 1), 24 * U, 0.2, tone('coat'), tone('inner'));
    }
    // Antlers: a lyre from the front. Each main beam leaves its burr nearly
    // flat, turns up and ends in two points; a brow tine reaches forward
    // over the forehead and two tines rise from the beam's bend.
    for (const side of [-1, 1]) {
      const X = (x: number) => side * x;
      tine(s, [P(110, 225, X(8)), P(104, 235, X(18)), P(96, 245, X(37)), P(88, 256, X(53)), P(81, 269, X(62)), P(76, 281, X(66)), P(73, 291, X(68)), P(72, 299, X(69))], [5.6, 5.2, 4.7, 4.3, 3.9, 3.4, 2.8, 2.0]);
      tine(s, [P(81, 269, X(61)), P(86, 280, X(58)), P(90, 291, X(56)), P(93, 300, X(55))], [3.6, 3.0, 2.4, 1.7]);
      tine(s, [P(96, 246, X(39)), P(104, 257, X(39)), P(112, 267, X(38)), P(119, 277, X(37))], [3.8, 3.2, 2.5, 1.8]);
      tine(s, [P(89, 255, X(47)), P(86, 265, X(45)), P(83, 274, X(43)), P(80, 282, X(42))], [3.8, 3.2, 2.5, 1.8]);
      tine(s, [P(105, 236, X(18)), P(115, 242, X(18)), P(126, 248, X(17)), P(137, 254, X(16))], [3.8, 3.2, 2.5, 1.8]);
    }
    // The nose, a black button on the muzzle's tip.
    const nose = [
      { z: 152, y: 187, w: 8.5, up: 6, down: 4.5 },
      { z: 158, y: 187.5, w: 8.8, up: 6, down: 4.5 },
      { z: 162, y: 187, w: 6, up: 4, down: 3 },
    ].map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U }));
    loft(shiny, nose, { sides: 6, turn: Math.PI / 6, start: 'flat', end: 1.8 * U, paint: () => tone('nose') });
  },
  jaw: {
    pivot: P(110, 180),
    build(s: Sculpt) {
      const rings = [
        { z: 104, y: 181, w: 14.5, up: 6, down: 7 },
        { z: 124, y: 181, w: 12, up: 6, down: 4.5 },
        { z: 140, y: 180, w: 9.5, up: 5, down: 3 },
        { z: 151, y: 179.5, w: 7, up: 3.5, down: 2 },
      ];
      loft(
        s,
        rings.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U })),
        // All pale: its mouth never opens.
        { sides: 6, start: 'flat', end: 2.5 * U, paint: () => tone('cream') },
      );
    },
  },
  tail: {
    joints: [P(-80, 156), P(-92, 138), P(-98, 118)],
    tip: P(-100, 106),
    radii: [8, 9, 6].map((r) => r * U),
    build(s: Sculpt) {
      // Short, brown above and white beneath.
      const path = [P(-78, 158), P(-86, 150), P(-93, 136), P(-98, 120), P(-100, 110)];
      const radii = [6, 8.5, 9, 7, 3].map((r) => r * U);
      loft(s, tubeRings(path, radii), {
        sides: 6,
        start: 'flat',
        end: 5 * U,
        paint: (_r, angle) => (Math.abs(angle - Math.PI) < 1.7 ? tone('cream') : tone('coat', 0.95)),
      });
    },
  },
  front: {
    hip: P(42, 140, 25),
    knee: P(38, 98, 25),
    ankle: P(54, 52, 25),
    paw: P(60, 18, 25),
    bend: -1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.35, w: 12 * U, d: 18 * U },
        { t: 0, w: 13 * U, d: 19 * U },
        { t: 0.5, w: 10 * U, d: 13 * U },
        { t: 1, w: 7.5 * U, d: 8 * U, back: 9.5 * U },
        { t: 1.5, w: 6.5 * U, d: 6.5 * U },
        { t: 2, w: 6 * U, d: 6.5 * U },
        { t: 2.5, w: 5 * U, d: 5 * U },
        { t: 3, w: 5.2 * U, d: 5.5 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        // Tan above, pale down the front of the shin and inside.
        paint: (r, angle) => {
          const front = Math.min(angle, 2 * Math.PI - angle) < 1.2;
          const inner = angle > Math.PI * 1.25;
          if (r <= 2) return inner ? blend('coat', 'cream', 0.5) : patchy('coat', 'dark', r, 2);
          return front || inner ? tone('cream') : blend('coat', 'cream', 0.3);
        },
      });
      hoof(s, joints[3], side);
    },
  },
  hind: {
    hip: P(-62, 150, 25),
    knee: P(-45, 100, 25),
    ankle: P(-95, 66, 25),
    paw: P(-90, 18, 25),
    bend: 1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.3, w: 17 * U, d: 28 * U },
        { t: 0, w: 19 * U, d: 30 * U },
        { t: 0.5, w: 15 * U, d: 22 * U, dz: 2 * U },
        { t: 1, w: 9 * U, d: 11 * U },
        { t: 1.5, w: 7.5 * U, d: 8.5 * U, back: 10 * U },
        { t: 2, w: 5.8 * U, d: 6 * U, back: 8 * U },
        { t: 2.5, w: 5 * U, d: 5.2 * U },
        { t: 3, w: 5.2 * U, d: 5.5 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        paint: (r, angle) => {
          const front = Math.min(angle, 2 * Math.PI - angle) < 1.1;
          const inner = angle > Math.PI * 1.2;
          if (r <= 3) return inner ? blend('coat', 'cream', 0.55) : patchy('coat', 'dark', r, 3);
          return front || inner ? tone('cream') : blend('coat', 'cream', 0.3);
        },
      });
      hoof(s, joints[3], side);
    },
  },
  walk: {
    stride: 0.5,
    cadence: 1.0,
    duty: 0.65,
    offsets: [0.25, 0.75, 0, 0.5],
    lift: [0.1, 0.09],
    fold: [1.0, 0.8],
    sink: 0.04,
    bob: 0.012,
    bounces: 2,
    rock: 0.02,
    flex: 0.02,
    nod: 0.05,
    lag: 0.1,
  },
  run: {
    stride: 0.72,
    cadence: 2.0,
    duty: 0.3,
    offsets: [0.54, 0.62, 0.02, 0.1],
    lift: [0.2, 0.16],
    fold: [1.6, 1.2],
    sink: 0.08,
    bob: 0.05,
    bounces: 1,
    rock: 0.1,
    flex: 0.12,
    nod: 0.04,
    lag: 0.72,
  },
};

export class ForestDeer extends FourLegged {
  // The sheet's colour variations, the default (brown) first.
  static readonly COLORS = Object.keys(VARIATIONS);

  constructor(options: ForestAnimalOptions = {}) {
    super(BUILD, options);
  }

  // Turns its head to look back over its shoulder for a few seconds, then
  // stands again. Named as in its sheet's poses.
  LookBack(): void {
    this.setMode('lookBack', 'stand');
  }

  protected pose(goal: Posture): void {
    super.pose(goal);
    if (this.mode === 'lookBack') {
      const t = this.modeTime;
      const on = t < 3.6 ? 1 : 0;
      goal.neckTurn = -1.25 * on;
      goal.headTurn = -0.7 * on;
      goal.neck = -0.12 * on;
      goal.head = 0.08 * on;
      goal.headTilt = 0.15 * on;
      goal.tail = 0.25 * on;
      if (t > 4.4) this.Idle();
    }
  }

  // The tail flicks up running, its white showing.
  protected gaitPosture(goal: Posture, running: number, moving: number): void {
    super.gaitPosture(goal, running, moving);
    goal.tail += 0.4 * running * moving;
  }
}
