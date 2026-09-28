import * as THREE from 'three';
import type { ForestAnimalOptions } from './ForestAnimal';
import { FourLegged, legRings, type FourLeggedBuild, type Gait, type Posture } from './FourLegged';
import { blend, ellipsoid, eye, loft, tone, tubeRings, type Palette, type Sculpt, type Tone } from './parts';

// The forest lake's rabbit (Rabbit.md), as its reference sheet draws it: a
// round rabbit of flat facets sitting up, warm tan, with a white chest,
// muzzle and paws, long upright ears pink inside, big dark eyes with a
// glint, a pink nose, big hind feet flat on the ground and a white cotton
// tail. Its colours are the sheet's five variations.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground
// under its middle. It is built sitting, as the sheet draws it: that is how
// it stands still, and it hops from there. Traced from the sheet's side and
// front views in their pixels (U m each: 25 cm to the traced top of its
// head), its head then drawn a little bigger, as the sheet's is: 27 cm to
// the top of its head sitting, 37 cm from nose to tail, as a European
// rabbit is.

const U = 0.25 / 172; // m per pixel of the sheet's side view: 172 px from the ground to the top of its head

function P(z: number, y: number, x = 0): THREE.Vector3 {
  return new THREE.Vector3(x * U, y * U, z * U);
}

// A point of the head: traced from the side view, then raised to sit up
// higher over its chest, as the sheet's front and 3/4 views show it.
const HEAD_LIFT = 9;
function PH(z: number, y: number, x = 0): THREE.Vector3 {
  return P(z, y + HEAD_LIFT, x);
}

// The sheet's five colour variations, picked from its pictures. `spot` is
// the spotted rabbit's patches, the coat itself on the others.
const VARIATIONS: Record<string, Palette> = {
  brown: { coat: 0xc68a6c, spot: 0xc68a6c, dark: 0xa8735a, cream: 0xf6ede8, inner: 0xf4a896, nose: 0xd98a86, iris: 0x3a2a24, pupil: 0x120d0c, shine: 0xffffff, cotton: 0xfdf2ea },
  white: { coat: 0xfbf4ee, spot: 0xfbf4ee, dark: 0xe6dcd6, cream: 0xffffff, inner: 0xf4b4aa, nose: 0xe89a96, iris: 0x2a1a14, pupil: 0x120d0c, shine: 0xffffff, cotton: 0xffffff },
  gray: { coat: 0xbba09a, spot: 0xbba09a, dark: 0x9c827c, cream: 0xefe8e8, inner: 0xe8aaa4, nose: 0xcf8a88, iris: 0x2a1a14, pupil: 0x120d0c, shine: 0xffffff, cotton: 0xf4eeee },
  black: { coat: 0x54413d, spot: 0x54413d, dark: 0x3c2e2c, cream: 0x8a7672, inner: 0xb07c78, nose: 0x6a4a48, iris: 0x1a1210, pupil: 0x0c0908, shine: 0xffffff, cotton: 0xa0908c },
  spotted: { coat: 0xf6ede6, spot: 0xd29773, dark: 0xbf8a6a, cream: 0xfffaf6, inner: 0xf4a896, nose: 0xd98a86, iris: 0x3a2a24, pupil: 0x120d0c, shine: 0xffffff, cotton: 0xfff6ee },
};

const UPRIGHT = new THREE.Vector3(0, 0, 1);

// Some faces of the coat are its spots (the spotted rabbit's), the same
// faces every time.
function coatOrSpot(r: number, k: number): Tone {
  const h = Math.sin(r * 91.7 + k * 37.3) * 43758.5453;
  const f = h - Math.floor(h);
  return f < 0.42 ? tone('spot') : f < 0.55 ? blend('coat', 'dark', 0.3) : tone('coat');
}

const WALK: Gait = {
  // Slow hops: the hind feet push together, the fore feet land together.
  stride: 0.045,
  cadence: 1.8,
  duty: 0.5,
  offsets: [0.5, 0.53, 0, 0.02],
  lift: [0.03, 0.03],
  fold: [0.35, 0.5],
  sink: 0.004,
  bob: 0.014,
  bounces: 1,
  rock: 0.1,
  flex: 0.12,
  nod: 0.03,
  lag: 0.35,
};

const RUN: Gait = {
  // Bounding: long leaps, stretched out and gathered up.
  stride: 0.075,
  cadence: 3.2,
  duty: 0.26,
  offsets: [0.5, 0.56, 0, 0.04],
  lift: [0.05, 0.06],
  fold: [0.7, 0.8],
  sink: 0.01,
  bob: 0.035,
  bounces: 1,
  rock: 0.12,
  flex: 0.16,
  nod: 0.03,
  lag: 0.35,
};

const BUILD: FourLeggedBuild = {
  variations: VARIATIONS,
  pelvis: P(-38, 62),
  chest: P(50, 94),
  neck: P(66, 121),
  head: P(60, 138 + HEAD_LIFT),
  headScale: 1.15,
  body(s: Sculpt) {
    // Sitting up: a round rump over the folded haunches, the back rising
    // to the shoulders and the neck.
    const slices = [
      { z: -106, y: 50, w: 16, up: 14, down: 20 },
      { z: -94, y: 58, w: 32, up: 30, down: 40 },
      { z: -70, y: 68, w: 40, up: 42, down: 52 },
      { z: -40, y: 76, w: 40, up: 46, down: 52 },
      { z: -8, y: 84, w: 36, up: 42, down: 34 },
      { z: 22, y: 94, w: 34, up: 38, down: 38 },
      { z: 48, y: 98, w: 34, up: 36, down: 44 },
      { z: 70, y: 102, w: 31, up: 34, down: 44 },
      { z: 86, y: 106, w: 25, up: 28, down: 46 },
      { z: 94, y: 106, w: 15, up: 16, down: 34 },
    ];
    loft(
      s,
      slices.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U, normal: UPRIGHT })),
      {
        sides: 8,
        start: 8 * U,
        end: 'flat',
        paint: (r, angle, k) => {
          const z = slices[Math.max(0, Math.min(slices.length - 1, r))].z;
          const fromBelow = Math.abs(angle - Math.PI);
          if (z >= 70 && fromBelow < 1.7) return tone('cream'); // the white chest, down its front
          if (fromBelow < 0.8) return tone('cream'); // and underneath
          return coatOrSpot(r, k);
        },
      },
    );
    // The chest hangs low between the front legs, white, as the front view
    // shows it down to the paws; behind the near leg from the side.
    // (No lower: its chest drops 66 mm as it lands a bound.)
    ellipsoid(s, P(70, 64), new THREE.Vector3(12, 18, 12).multiplyScalar(U), new THREE.Matrix4(), 3, 8, () => tone('cream'));
  },
  buildHead(s: Sculpt, shiny: Sculpt) {
    // A round head with a short, blunt muzzle.
    const rings = [
      { z: 44, y: 148, w: 22, up: 20, down: 26 },
      { z: 60, y: 150, w: 32, up: 24, down: 35 },
      { z: 78, y: 147, w: 32, up: 22, down: 36 },
      { z: 94, y: 140, w: 26, up: 16, down: 30 },
      { z: 108, y: 132, w: 18, up: 11, down: 21 },
      { z: 118, y: 127, w: 11, up: 7, down: 13 },
    ];
    loft(
      s,
      rings.map((r) => ({ at: PH(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U })),
      {
        sides: 8,
        turn: Math.PI / 8,
        start: 'flat',
        end: 7 * U,
        paint: (r, angle, k) => {
          const top = Math.min(angle, 2 * Math.PI - angle);
          if (r >= 3 && top > 1.2) return tone('cream'); // the white muzzle and chin
          if (r === 2 && top > 2) return tone('cream');
          if (r >= 4 && top < 1.2) return blend('coat', 'cream', 0.35);
          return coatOrSpot(r + 10, k);
        },
      },
    );
    // Eyes: big, round and dark, on the sides of the head.
    for (const side of [-1, 1]) {
      const look = new THREE.Vector3(side * 0.85, 0.08, 0.52).normalize();
      const at = s.onto(PH(88, 146, side * 60), new THREE.Vector3(-side, 0, 0)).addScaledVector(look, -1 * U);
      eye(shiny, at, look, new THREE.Vector3(0, 1, 0), 7 * U, { lid: tone('pupil'), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 8, 0.62, 0.55, 1.15);
    }
    // The nose, a small pink wedge on the muzzle's tip.
    const nose = [
      { z: 120, y: 131, w: 6, up: 3.5, down: 3 },
      { z: 125, y: 131, w: 4.5, up: 2.5, down: 2.5 },
    ].map((r) => ({ at: PH(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U }));
    loft(shiny, nose, { sides: 6, turn: Math.PI / 6, start: 'flat', end: 1.5 * U, paint: () => tone('nose') });
    // Ears: long flat leaves standing up from the top of the head, pink on
    // their fronts, spreading apart a little toward their tips.
    for (const side of [-1, 1]) {
      const path = [PH(66, 164, side * 10), PH(64, 184, side * 15), PH(62, 205, side * 21), PH(60, 224, side * 27), PH(58, 238, side * 31)];
      const widths = [10, 16.5, 18, 15, 7.5];
      // The ring's top is the ear's front: turned to face forward and out.
      const facing = new THREE.Vector3(side * 0.3, 0, 0.95);
      const along = path[path.length - 1].clone().sub(path[0]).normalize();
      const rings = tubeRings(
        path,
        widths.map((w) => [w * U, 2.8 * U] as const),
        new THREE.Vector3().crossVectors(facing, along),
      );
      loft(s, rings, {
        sides: 6,
        turn: Math.PI / 6,
        start: 'flat',
        end: 6 * U,
        // The ring's top is its front (it runs up): pink there, fur round it.
        paint: (r, angle) => {
          const front = Math.min(angle, 2 * Math.PI - angle) < 1;
          if (front && r >= 1 && r <= 3) return tone('inner');
          return r >= 3 ? blend('coat', 'dark', 0.2) : coatOrSpot(r + 20, side < 0 ? 0 : 1);
        },
      });
    }
  },
  tail: {
    joints: [P(-92, 50), P(-102, 46)],
    tip: P(-120, 48),
    radii: [10, 12].map((r) => r * U),
    build(s: Sculpt) {
      // A white cotton ball.
      ellipsoid(s, P(-107, 48), new THREE.Vector3(14, 14, 13).multiplyScalar(U), new THREE.Matrix4(), 3, 8, (lat) => tone('cotton', 0.97 + 0.04 * lat));
    },
  },
  front: {
    hip: P(62, 72, 20),
    knee: P(54, 44, 20),
    ankle: P(66, 15, 20),
    paw: P(70, 7, 20),
    bend: -1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.5, w: 15 * U, d: 18 * U },
        { t: 0, w: 14.5 * U, d: 17 * U },
        { t: 0.5, w: 12.5 * U, d: 14 * U },
        { t: 1, w: 11 * U, d: 11.5 * U },
        { t: 1.5, w: 10 * U, d: 10 * U },
        { t: 2, w: 9.5 * U, d: 9 * U },
        { t: 3, w: 9.5 * U, d: 8 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        paint: (r) => (r <= 1 ? tone('cream', 0.98) : tone('cream')),
      });
      foot(s, joints[3], side, 11, 28, 10);
    },
  },
  hind: {
    hip: P(-38, 62, 31),
    knee: P(-8, 40, 31),
    ankle: P(-62, 10, 31),
    paw: P(-4, 6, 31),
    bend: 1,
    build(s, side, joints) {
      // The big haunch, the shin folded back under it, and the long foot
      // flat on the ground from the heel.
      const rings = legRings(joints, [
        { t: -0.4, w: 26 * U, d: 34 * U },
        { t: 0, w: 28 * U, d: 38 * U },
        { t: 0.5, w: 25 * U, d: 32 * U, dz: -4 * U },
        { t: 1, w: 20 * U, d: 22 * U, dz: -10 * U },
        { t: 1.5, w: 16 * U, d: 17 * U, back: 20 * U },
        { t: 2, w: 9 * U, d: 7 * U },
        { t: 2.5, w: 9 * U, d: 5 * U },
        { t: 3, w: 9.5 * U, d: 5 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        paint: (r, angle) => (r <= 2 ? coatOrSpot(r + 30, 0) : r >= 5 ? tone('cream') : angle > Math.PI ? tone('cream') : blend('coat', 'cream', 0.4)),
      });
      foot(s, joints[3], side, 9.5, 14, 5);
    },
  },
  walk: WALK,
  run: RUN,
};

// A rabbit's paw at a foot's joint, its sole on the ground: a rounded white
// wedge `length` px forward, `w` px to either side.
function foot(s: Sculpt, at: THREE.Vector3, side: number, w: number, length: number, height: number): void {
  const rings = [
    { z: -3, y: at.y / U, w: w * 0.9, up: height * 0.6, down: at.y / U },
    { z: length * 0.45, y: height * 0.55, w, up: height * 0.5, down: height * 0.55 },
    { z: length * 0.85, y: height * 0.45, w: w * 0.85, up: height * 0.4, down: height * 0.45 },
  ].map((r) => ({ at: new THREE.Vector3(at.x, r.y * U, at.z + r.z * U), w: r.w * U, up: r.up * U, down: r.down * U, n: 2.5 }));
  loft(s, rings, {
    sides: 8,
    turn: Math.PI / 8,
    side: new THREE.Vector3(side, 0, 0),
    mirror: side < 0,
    start: 'flat',
    end: length * 0.25 * U,
    paint: (_r, angle) => (Math.abs(angle - Math.PI) < 0.6 ? tone('cream', 0.85) : tone('cream')),
  });
}

export class Rabbit extends FourLegged {
  // The sheet's colour variations, the default (brown) first.
  static readonly COLORS = Object.keys(VARIATIONS);

  constructor(options: ForestAnimalOptions = {}) {
    super(BUILD, options);
  }

  // Leaps high and forward from its haunches, stretching out in the air,
  // and lands. Named as in its sheet's poses.
  Jump(): void {
    this.setMode('jump', 'stand');
    this.leap(0.3, 0.55, 0.14);
  }

  // Lowers its head to nibble the grass, its ears twitching, until told
  // otherwise. Named as in its sheet's poses ("Eat / Forage").
  Eat(): void {
    this.setMode('eat', 'stand');
  }

  protected pose(goal: Posture): void {
    super.pose(goal);
    const t = this.modeTime;
    if (this.mode === 'jump' && !this.jumping && t > 0.2) this.Idle();
    if (this.mode === 'eat') {
      goal.front = 0.02;
      goal.neck = 0.55;
      goal.head = 0.75 + 0.05 * Math.sin(t * 9);
      goal.headTurn = 0.1 * Math.sin(t * 0.9);
    }
  }

  // Hopping, it raises its rump and stretches out along the ground.
  protected gaitPosture(goal: Posture, running: number, moving: number): void {
    super.gaitPosture(goal, running, moving);
    goal.rear -= (0.012 + 0.014 * running) * moving;
    goal.pitch += (0.12 + 0.12 * running) * moving;
    goal.neck -= 0.1 * running * moving;
  }
}
