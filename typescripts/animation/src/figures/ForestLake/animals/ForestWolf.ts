import * as THREE from 'three';
import type { ForestAnimalOptions } from './ForestAnimal';
import { FourLegged, legRings, type FourLeggedBuild, type Posture } from './FourLegged';
import { blend, ear, eye, loft, spike, tone, tubeRings, type Palette, type Sculpt, type Tone } from './parts';

// The forest lake's wolf (ForestWolf.md), as its reference sheet draws it: a
// big grey wolf of large flat facets like folded paper, a mane of fur in
// shingles over its neck and shoulders, a pale muzzle, chest, belly and
// legs, a darker saddle, tall pointed ears, amber eyes and a black nose,
// and a bushy tail with a pale underside. Its colours are the sheet's five
// variations.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground
// under its middle. Its shape is traced from the sheet's side and front
// views in their pixels (U m each), so it stands 78 cm at the shoulder, as
// a grown grey wolf does.

const U = 0.78 / 188; // m per pixel of the sheet's side view: 188 px from the ground to its withers

// A point traced from the sheet: `z` forward and `y` up from the ground, in
// its pixels; `x` to its right.
function P(z: number, y: number, x = 0): THREE.Vector3 {
  return new THREE.Vector3(x * U, y * U, z * U);
}

// The sheet's five colour variations, picked from its pictures (the lit
// and shaded sides averaged).
const VARIATIONS: Record<string, Palette> = {
  gray: { coat: 0x9e8681, dark: 0x6f6166, cream: 0xefdcd2, inner: 0xb8aab2, nose: 0x23252c, iris: 0xf0a81c, pupil: 0x1c1816, shine: 0xffffff, mouth: 0x3b2b2d },
  white: { coat: 0xe6e2ed, dark: 0xbfc1d6, cream: 0xf7f4f7, inner: 0xd6d1df, nose: 0x2a2d38, iris: 0xe6ad26, pupil: 0x1c1816, shine: 0xffffff, mouth: 0x4a3336 },
  brown: { coat: 0xd09564, dark: 0x966440, cream: 0xf1d2b2, inner: 0xc49774, nose: 0x2a211c, iris: 0xeeaa22, pupil: 0x1c1816, shine: 0xffffff, mouth: 0x3b2a26 },
  black: { coat: 0x524644, dark: 0x383234, cream: 0x76696c, inner: 0x5a4f51, nose: 0x17181c, iris: 0xeebc28, pupil: 0x121010, shine: 0xffffff, mouth: 0x231c1d },
  snow: { coat: 0xf5e4d9, dark: 0xddcdc9, cream: 0xfdf6f0, inner: 0xe6d6d3, nose: 0x2b2a30, iris: 0xe6aa26, pupil: 0x1c1816, shine: 0xffffff, mouth: 0x4a3336 },
};

// A face's patchiness: now and then a facet a little toward the saddle's
// colour, as the sheet's grey is patched with darker facets.
function patchy(role: string, r: number, k: number, amount = 0.35): Tone {
  const h = Math.sin(r * 12.9898 + k * 78.233) * 43758.5453;
  const f = h - Math.floor(h);
  return f < 0.3 ? blend(role, 'dark', amount * (0.5 + f)) : tone(role);
}

const BUILD: FourLeggedBuild = {
  variations: VARIATIONS,
  pelvis: P(-95, 152),
  chest: P(45, 148),
  neck: P(82, 168),
  head: P(106, 196),
  body(s: Sculpt) {
    // The torso and neck, rump to the neck's end inside the head.
    const rings = [
      { z: -130, y: 148, w: 16, up: 15, down: 16 },
      { z: -120, y: 146, w: 29, up: 29, down: 30 },
      { z: -100, y: 146, w: 35, up: 36, down: 38 },
      { z: -72, y: 148, w: 36, up: 39, down: 41 },
      { z: -40, y: 145, w: 35, up: 43, down: 50 },
      { z: -5, y: 140, w: 38, up: 49, down: 56 },
      { z: 28, y: 138, w: 45, up: 53, down: 58 },
      { z: 55, y: 140, w: 46, up: 55, down: 59 },
      { z: 76, y: 154, w: 41, up: 49, down: 51 },
      { z: 94, y: 174, w: 35, up: 41, down: 43 },
      { z: 110, y: 192, w: 29, up: 32, down: 33 },
    ].map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U }));
    const zs = [-130, -120, -100, -72, -40, -5, 28, 55, 76, 94, 110];
    const body = loft(s, rings, {
      sides: 8,
      start: 8 * U,
      end: 'flat',
      paint: (r, angle, k) => {
        const z = zs[Math.max(0, Math.min(zs.length - 1, r))];
        const fromBelow = Math.abs(angle - Math.PI); // 0 at the belly, π along the back
        if (fromBelow < 0.8) return tone('cream');
        if (fromBelow < 1.3 && z > 30) return tone('cream');
        if (fromBelow < 1.3 && z < 20) return blend('coat', 'cream', 0.45);
        if (fromBelow > 2.5 && z > -115 && z < 60) return patchy('dark', r, k, 0.2);
        return patchy('coat', r, k);
      },
    });
    // The mane: shingles of fur over the neck and shoulders, two rows,
    // pointing back and out.
    const rows: [ring: number, length: number, spread: number][] = [
      [6, 26, 0.5],
      [7, 36, 0.55],
      [8, 38, 0.5],
      [9, 34, 0.45],
      [10, 26, 0.4],
    ];
    for (const [r, length, spread] of rows) {
      const ring = body.rings[r];
      const frame = body.frames[r];
      const centre = rings[r].at;
      for (let k = 0; k < 8; k++) {
        const k1 = (k + 1) % 8;
        const a = ring[k];
        const b = ring[k1];
        const mid = a.clone().add(b).multiplyScalar(0.5);
        const out = mid.clone().sub(centre).normalize();
        const up = out.dot(frame.y);
        if (up < -0.6) continue; // not under the neck
        // Those along the top lie back along it; those down the sides stand
        // out more. The lowest, over the throat and chest, are pale.
        const tip = mid
          .clone()
          .addScaledVector(frame.t, -length * U)
          .addScaledVector(out, length * U * spread * (up > 0.5 ? 0.6 : 1));
        const lift = out.clone().multiplyScalar(6 * U);
        const top = up < 0.15 ? tone('cream') : (r + k) % 3 === 0 ? tone('dark') : patchy('coat', r, k, 0.5);
        spike(s, a, b, tip, lift, top, tone('dark', 0.8));
      }
    }
  },
  buildHead(s: Sculpt, shiny: Sculpt) {
    // From the back of the skull (in the mane) to the muzzle's tip, traced
    // from the sheet's side view, its widths from the front view: a broad
    // crown between the ears, narrower at the eyes, a boxy muzzle, down to
    // the mouth (the lower jaw is its own piece); the cheek ruff makes the
    // face as broad as the sheet's.
    const rings = [
      { z: 88, y: 205, w: 26, up: 25, down: 27 },
      { z: 102, y: 208, w: 32, up: 25, down: 30 },
      { z: 116, y: 206, w: 31, up: 20, down: 30 },
      { z: 128, y: 201, w: 22, up: 13, down: 22 },
      { z: 142, y: 194, w: 16, up: 10, down: 14 },
      { z: 156, y: 189, w: 13, up: 8, down: 9 },
      { z: 167, y: 185.5, w: 10, up: 6, down: 5.5 },
    ];
    loft(
      s,
      rings.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U })),
      {
        sides: 8,
        turn: Math.PI / 8,
        start: 'flat',
        end: 4 * U,
        paint: (r, angle, k) => {
          const top = Math.min(angle, 2 * Math.PI - angle); // 0 on top, π underneath
          if (top < 0.5) return r >= 3 ? tone('coat', 1.04) : patchy('dark', r, k, 0.3); // the bridge of the nose, the crown
          if (top < 1.2) {
            if (r === 2) return tone('cream', 1.02); // the brows
            return r >= 3 ? blend('coat', 'dark', 0.25) : patchy('coat', r, k);
          }
          if (top < 2) {
            if (r === 2) return blend('dark', 'coat', 0.3); // round the eyes
            return r >= 3 ? tone('cream') : patchy('coat', r, k); // the muzzle's white sides
          }
          return tone('cream'); // cheeks, lips and chin
        },
      },
    );
    // Eyes: almond-shaped and amber, slanting up at their outer corners,
    // seated on the face where a ray from in front meets it, looking
    // forward and out.
    for (const side of [-1, 1]) {
      const look = new THREE.Vector3(side * 0.5, 0.12, 0.86).normalize();
      const at = s.onto(P(200, 204, side * 17), new THREE.Vector3(0, 0, -1)).addScaledVector(look, -0.8 * U);
      eye(shiny, at, look, new THREE.Vector3(0, 1, 0), 3.4 * U, { lid: tone('pupil'), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 8, 0.45, 0.35, 2.1, side * -0.32);
    }
    // The cheek ruff: pale points from the cheeks back and out, framing the
    // face as the sheet's front view shows it.
    for (const side of [-1, 1]) {
      for (const [z, y, out, length, reach] of [
        [112, 198, 30, 22, 18],
        [116, 188, 30, 24, 22],
        [114, 178, 25, 22, 18],
        [106, 206, 30, 20, 14],
      ]) {
        const a = P(z + 8, y + 9, side * (out - 4));
        const b = P(z + 8, y - 9, side * (out - 6));
        const tip = P(z - length, y - 7, side * (out + reach));
        spike(s, a, b, tip, new THREE.Vector3(side * 5 * U, 0, 0), tone('cream'), tone('coat', 0.85));
      }
    }
    // Ears: big and pointed, standing up from the corners of the crown,
    // their hollows forward, paler inside.
    for (const side of [-1, 1]) {
      ear(s, P(110, 228, side * 25), P(115, 270, side * 29), new THREE.Vector3(side, -0.1, -0.2), new THREE.Vector3(side * 0.22, 0.05, 1), 32 * U, 10 * U, 0.26, patchy('dark', 3, side, 0.3), tone('inner'));
    }
    // The nose: a black button capping the muzzle.
    const nose = [
      { z: 163, y: 186, w: 8, up: 5.5, down: 4 },
      { z: 168, y: 186.5, w: 8.5, up: 5.5, down: 4 },
      { z: 172, y: 185.5, w: 6, up: 3.5, down: 3 },
    ].map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U }));
    loft(shiny, nose, { sides: 6, turn: Math.PI / 6, start: 'flat', end: 2 * U, paint: () => tone('nose') });
  },
  jaw: {
    pivot: P(112, 176),
    build(s: Sculpt) {
      // Tucked under the muzzle, dark along its top where the mouth opens.
      const rings = [
        { z: 106, y: 173, w: 23, up: 5, down: 9 },
        { z: 124, y: 173, w: 19, up: 6, down: 7 },
        { z: 140, y: 174, w: 14, up: 5.5, down: 5.5 },
        { z: 154, y: 175, w: 10, up: 4.5, down: 4 },
        { z: 164, y: 176, w: 6.5, up: 3.5, down: 3 },
      ];
      loft(
        s,
        rings.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U })),
        {
          sides: 6,
          start: 'flat',
          end: 3 * U,
          // Dark only inside the mouth, behind its lips.
          paint: (r, angle) => (r === 0 && Math.min(angle, 2 * Math.PI - angle) < 0.6 ? tone('mouth') : tone('cream')),
        },
      );
    },
  },
  tail: {
    joints: [P(-122, 162), P(-140, 150), P(-157, 124), P(-167, 92), P(-172, 58)],
    tip: P(-172, 33),
    radii: [9, 17, 23, 21, 13].map((r) => r * U),
    build(s: Sculpt) {
      const path = [P(-118, 164), P(-130, 158), P(-146, 144), P(-159, 118), P(-167, 88), P(-171, 60), P(-172, 42)];
      const radii = [8, 14, 21, 24, 21, 14, 5].map((r) => r * U);
      loft(s, tubeRings(path, radii), {
        sides: 6,
        start: 'flat',
        end: 9 * U,
        paint: (r, angle, k) => {
          const fromBelow = Math.abs(angle - Math.PI);
          if (r >= 5) return tone('cream', 0.96);
          if (fromBelow < 1.1) return tone('cream');
          return patchy(r >= 2 ? 'dark' : 'coat', r, k, 0.3);
        },
      });
    },
  },
  front: {
    hip: P(45, 150, 27),
    knee: P(49, 88, 27),
    ankle: P(51, 33, 27),
    paw: P(54, 11, 27),
    bend: -1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.35, w: 17 * U, d: 21 * U },
        { t: 0, w: 19 * U, d: 24 * U },
        { t: 0.5, w: 15 * U, d: 18 * U },
        { t: 1, w: 12 * U, d: 12 * U, back: 14 * U },
        { t: 1.5, w: 10 * U, d: 10 * U },
        { t: 2, w: 9 * U, d: 9 * U },
        { t: 2.5, w: 8.5 * U, d: 8 * U },
        { t: 3, w: 9 * U, d: 8 * U, dz: 1 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        paint: (r, angle) => {
          const front = Math.min(angle, 2 * Math.PI - angle) < 1.2;
          if (r <= 1) return front || angle > Math.PI * 1.2 ? blend('coat', 'cream', 0.5) : patchy('coat', r, 1);
          return front || angle > Math.PI * 1.25 ? tone('cream') : blend('cream', 'coat', 0.35);
        },
      });
      paw(s, joints[3], side, 1);
    },
  },
  hind: {
    hip: P(-95, 153, 28),
    knee: P(-70, 84, 28),
    ankle: P(-108, 46, 28),
    paw: P(-103, 11, 28),
    bend: 1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.3, w: 22 * U, d: 29 * U },
        { t: 0, w: 25 * U, d: 33 * U },
        { t: 0.5, w: 19 * U, d: 25 * U, dz: 3 * U },
        { t: 1, w: 13 * U, d: 14 * U },
        { t: 1.5, w: 10.5 * U, d: 11 * U, back: 13 * U },
        { t: 2, w: 8.5 * U, d: 8 * U, back: 10 * U },
        { t: 2.5, w: 8 * U, d: 7.5 * U },
        { t: 3, w: 8.5 * U, d: 8 * U, dz: 1 * U },
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
          if (r <= 2) return inner ? blend('coat', 'cream', 0.55) : patchy('coat', r, 2);
          return front || inner ? tone('cream') : blend('cream', 'coat', 0.45);
        },
      });
      paw(s, joints[3], side, 0.95);
    },
  },
  walk: {
    stride: 0.44,
    cadence: 1.1,
    duty: 0.64,
    offsets: [0.25, 0.75, 0, 0.5],
    lift: [0.07, 0.06],
    fold: [0.9, 0.7],
    sink: 0.035,
    bob: 0.012,
    bounces: 2,
    rock: 0.02,
    flex: 0.02,
    nod: 0.04,
    lag: 0.1,
  },
  run: {
    stride: 0.66,
    cadence: 2.3,
    duty: 0.34,
    offsets: [0.56, 0.66, 0.02, 0.12],
    lift: [0.13, 0.11],
    fold: [1.5, 1.1],
    sink: 0.08,
    bob: 0.035,
    bounces: 1,
    rock: 0.09,
    flex: 0.14,
    nod: 0.05,
    lag: 0.72,
  },
};

// A wolf's paw at the foot's joint, as the sheet draws it: a pale, rounded
// block with its toes split by dark lines in front.
function paw(s: Sculpt, at: THREE.Vector3, side: number, size: number): void {
  const k = size * U;
  const rings = [
    { z: -8, y: 8, w: 8, up: 5, down: 7 },
    { z: 0, y: 7, w: 10.5, up: 7, down: 7 },
    { z: 9, y: 6, w: 11, up: 6, down: 6 },
    { z: 16, y: 4.5, w: 10, up: 4, down: 4.5 },
  ].map((r) => ({ at: new THREE.Vector3(at.x, r.y * k, at.z + r.z * k), w: r.w * k, up: r.up * k, down: r.down * k, n: 2.6 }));
  loft(s, rings, {
    sides: 8,
    turn: Math.PI / 8,
    side: new THREE.Vector3(side, 0, 0),
    mirror: side < 0,
    start: 'flat',
    end: 3 * k,
    paint: (r, angle) => {
      if (Math.abs(angle - Math.PI) < 0.8) return tone('dark', 0.7); // the pads
      if (r >= 2 && Math.min(angle, 2 * Math.PI - angle) < 0.5) return tone('cream', 0.92);
      return tone('cream');
    },
  });
}

export class ForestWolf extends FourLegged {
  // The sheet's colour variations, the default (gray) first.
  static readonly COLORS = Object.keys(VARIATIONS);

  constructor(options: ForestAnimalOptions = {}) {
    super(BUILD, options);
  }

  // Sits on its haunches and howls, its neck and head raised to the sky
  // and its mouth open, for four seconds, then stands again. Named as in
  // its sheet's poses.
  Howl(): void {
    this.setMode('howl', 'stand');
  }

  protected pose(goal: Posture): void {
    super.pose(goal);
    if (this.mode === 'howl') {
      const t = this.modeTime;
      const on = t < 4.2 ? 1 : 0;
      // Sitting on its haunches, as the sheet draws it, head to the sky.
      goal.rear = 0.2 * on;
      goal.front = -0.02 * on;
      goal.sit = on;
      goal.hindReach = 0.05 * on;
      goal.neck = -0.55 * on;
      goal.head = -0.95 * on;
      goal.jaw = on * (t > 0.7 && t < 3.8 ? 0.8 + 0.2 * Math.sin(t * 5) : 0);
      goal.tail = 0.1 * on;
      goal.headTilt = 0.04 * Math.sin(t * 2) * on;
      if (t > 4.8) this.Idle();
    }
  }
}
