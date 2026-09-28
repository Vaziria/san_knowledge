import * as THREE from 'three';
import { bands, box, decal, faceAngle, lay, oval, P, ring, slice, stackPoint, strip, U, yRing, type Frame, type Slice, type V3 } from './ExplorerKit';
import { blend, loft, patchy, spike, tone, tubeRings, type Palette, type Sculpt, type Tone } from './parts';

// The explorer's body parts (Explorer.ts), sculpted from its sheet's front,
// side and back views and its close-ups, in the sheet's pixels (ExplorerKit:
// U m each, x to the figure's right (+x), y up from the ground, z forward
// from under its ankles). The head, the torso and the backpack are sculpted
// where they stand in the figure; an arm or a leg straight down from its
// shoulder or hip, in its own joint's space (its rest pose turns it).

// The sheet's one colour variation: its Color Palette (the scarf's greens,
// the shirt's cream, the leather's browns, the skin's salmon and the gold of
// the buckles), and what its pictures add, picked from the views with
// ref.ps1 sample: the lit faces of the sheet are about the albedo.
export const VARIATIONS: Record<string, Palette> = {
  default: {
    skin: 0xfcc08e,
    skinShade: 0xe8946a,
    blush: 0xf9a888,
    nose: 0xf6a882,
    mouth: 0x9a4434,
    mouthIn: 0x7c2c26,
    teeth: 0xfff7f0,
    tongue: 0xe57a6c,
    hair: 0x6e4c39,
    hairLight: 0x88603f,
    hairDark: 0x4e3629,
    brow: 0x43291c,
    sclera: 0xfbf4ec,
    iris: 0x6b3a22,
    pupil: 0x2b1810,
    lash: 0x2e1c15,
    shine: 0xffffff,
    shirt: 0xf0d4b0,
    shirtLight: 0xf7e6cb,
    shirtShade: 0xd6b28b,
    green: 0x758446,
    greenDark: 0x4f5a35,
    leaf: 0xc9c052,
    leather: 0x8a5c3e,
    leatherLight: 0xa86f44,
    leatherDark: 0x5a3929,
    shorts: 0x5e4c3b,
    shortsDark: 0x463a2e,
    boot: 0xa96f44,
    bootDark: 0x7e5032,
    cuff: 0xc08c62,
    sole: 0x4a3526,
    bracer: 0x6f5036,
    bracerStrap: 0x9a7552,
    gold: 0xfaba5c,
    goldDark: 0xc98b3e,
    cream: 0xecd3b3,
    creamShade: 0xcdb08e,
    wood: 0xdca06a,
    woodShade: 0xb3774b,
    handle: 0x8a5433,
    handleDark: 0x6b4d3b,
    wrap: 0xf4cc92,
    metal: 0x9a8d83,
    metalDark: 0x766a62,
    metalEdge: 0xc9c3bb,
    copper: 0xa36c49,
    copperDark: 0x603c23,
    glass: 0xfce1a8,
    glow: 0xffd77a,
    bobber: 0xdb5640,
    white: 0xf4ede4,
    line: 0xe8e2d8,
  },
};

// ------------------------------------------------------------ joints (px)

// Where each joint stands in the figure as built, in the sheet's pixels.
export const JOINTS = {
  hips: [0, 138, 4], // between the hip joints, the body's root
  hip: [34, 138, 4], // the right hip joint (the left at -x)
  spine: [0, 170, 12], // the waist, where the torso bends
  neck: [0, 268, 14],
  head: [0, 290, 16], // under the chin, where the head turns
  shoulder: [50, 250, 16], // the right shoulder (the left at -x)
  hook: [68, 172, -32], // the lantern's hook, under the backpack's right pocket
} as const;

export function joint(name: keyof typeof JOINTS, side = 1): V3 {
  const [x, y, z] = JOINTS[name];
  return P(x * side, y, z);
}

export const THIGH = 64; // px, hip to knee (inside the shorts' hem)
export const SHIN = 52; // px, knee to ankle (inside the boot)
export const UPPER_ARM = 52; // px, shoulder to elbow
export const FOREARM = 34; // px, elbow to wrist
export const ANKLE_HEIGHT = 24; // px, the ankle over the soles

const X = new THREE.Vector3(1, 0, 0);
const Y = new THREE.Vector3(0, 1, 0);
const Z = new THREE.Vector3(0, 0, 1);
const gold = (f: string) => (f === 'z' ? tone('gold') : tone('goldDark'));

// ------------------------------------------------------------ the head

// The skin of the head: slices from the chin up, traced from the front view
// (widths: a wide, round jaw) and the side view (the face's profile and the
// back of the skull), a flat facet down the middle of the face.
const HEAD_SLICES: Slice[] = [
  { y: 289, z: 45, w: 11, f: 9, b: 9 },
  { y: 294, z: 38, w: 22, wb: 22, f: 21, b: 20 },
  { y: 301, z: 29, w: 30, wb: 32, f: 33, b: 33 },
  { y: 309, z: 21, w: 37, wb: 38, f: 42, b: 41 },
  { y: 319, z: 15, w: 43, f: 48, b: 46 },
  { y: 332, z: 11, w: 46, f: 50, b: 49 },
  { y: 347, z: 10, w: 46.5, f: 50, b: 50 },
  { y: 362, z: 9, w: 45.5, f: 47, b: 49 },
  { y: 377, z: 7, w: 41, f: 40, b: 45 },
  { y: 392, z: 5, w: 29, f: 25, b: 31 },
].map((s) => ({ ...s, n: 2.3 }));
const HEAD_TOP = [0, 401, 4] as const;
const HEAD_SIDES = 14;
const HEAD_DOWN = [...HEAD_SLICES].reverse(); // from the top down, for stackPoint

export function skinRings(): V3[][] {
  return HEAD_SLICES.map((s) => slice(s, HEAD_SIDES, Math.PI / HEAD_SIDES));
}

// The face's frame: features are drawn as the front view shows them (u, v
// the sheet's x and height in px) and cast straight back onto the skin.
export function faceFrame(): Frame {
  return { origin: P(0, 0, 80), right: X.clone(), up: Y.clone(), out: Z.clone() };
}

// The skin alone, to lay the face onto.
export function sculptSkin(s: Sculpt): void {
  bands(s, skinRings(), () => tone('skin'), 'flat', P(...HEAD_TOP));
}

// The hair's cap over the skull: much bigger than it, as the sheet's mop of
// hair is (70 px either side at the temples against a face 46 px either
// side), its front edge raised over the forehead where the fringe hangs.
const CAP: Slice[] = [
  { y: 421, z: 6, w: 14, f: 12, b: 14 },
  { y: 416, z: 6, w: 40, f: 34, b: 40 },
  { y: 406, z: 7, w: 57, f: 50, b: 57 },
  { y: 393, z: 8, w: 65, f: 57, b: 64 },
  { y: 378, z: 9, w: 68, f: 59, b: 67 },
  { y: 364, z: 9, w: 65, f: 55, b: 66, drop: (a: number) => -8 * Math.max(0, Math.cos(a)) ** 2 },
  { y: 352, z: 8, w: 60, f: 51, b: 63, drop: (a: number) => -16 * Math.max(0, Math.cos(a)) ** 2 },
].map((s) => ({ ...s, n: 2.2 }));

// The head: its skin (hair-coloured where the hair covers it), the ears, the
// nose and rosy cheeks, and the hair. `skin` holds the bare skin to lay
// things on.
export function sculptHead(s: Sculpt, skin: Sculpt): void {
  const rings = skinRings();
  const centres = rings.map((r) => r.reduce((c, p) => c.add(p), new THREE.Vector3()).divideScalar(r.length));
  bands(
    s,
    rings,
    (r, k) => {
      // Hair above its line: low at the nape, over the ears at the sides,
      // above the brows in front (where the fringe hangs).
      if (r < 0) return tone('skinShade');
      const rr = Math.min(rings.length - 2, r);
      const k1 = (k + 1) % HEAD_SIDES;
      const mid = rings[rr][k].clone().add(rings[rr][k1]).add(rings[rr + 1][k]).add(rings[rr + 1][k1]).multiplyScalar(0.25);
      const d = mid.clone().sub(centres[rr]);
      const c = d.z / Math.max(1e-6, Math.hypot(d.x, d.z));
      const line = c > 0 ? 346 + 18 * c : 346 - 44 * -c;
      if (r >= rings.length - 1 || mid.y / U > line) return tone('hairDark', 1.1);
      return tone('skin');
    },
    'flat',
    P(...HEAD_TOP),
  );

  // Ears: a rounded flap on each side, its hollow (in shade) forward, under
  // the side hair at its top.
  for (const side of [-1, 1]) {
    const ear = [
      { y: 303, x: 46, z: 0, w: 2.5, f: 4.5, b: 4.5 },
      { y: 310, x: 48.5, z: -0.5, w: 4, f: 8, b: 7.5 },
      { y: 322, x: 50, z: -2, w: 4.2, f: 9, b: 8.5 },
      { y: 333, x: 49, z: -3.5, w: 3.5, f: 7.5, b: 7 },
      { y: 338, x: 47, z: -4, w: 2, f: 4, b: 4 },
    ];
    bands(
      s,
      ear.map((e) => slice({ y: e.y, x: side * e.x, z: e.z, w: e.w, f: e.f, b: e.b }, 6, Math.PI / 6)),
      (r, k) => {
        const a = faceAngle(k, 6);
        if (r >= 1 && r <= 2 && Math.cos(a) > 0.2 && Math.sin(a) * side > -0.2) return tone('skinShade');
        return tone('skin', 0.98);
      },
      'flat',
      'flat',
    );
  }

  // The nose: a small, soft point on the middle of the face.
  const nf = faceFrame();
  const n1 = lay(skin, nf, -2.8, 312.8, 0);
  const n2 = lay(skin, nf, 2.8, 312.8, 0);
  const n3 = lay(skin, nf, 0, 317.8, 0);
  const middle = n1.clone().add(n2).add(n3).divideScalar(3);
  const tip = middle.clone().add(P(0, -0.6, 1.7));
  const inside = middle.clone().add(P(0, 0, -4));
  s.facing(n1, n2, tip, inside, blend('skin', 'nose', 0.6, 0.97));
  s.facing(n2, n3, tip, inside, blend('skin', 'nose', 0.3));
  s.facing(n3, n1, tip, inside, blend('skin', 'nose', 0.3));

  // Rosy cheeks.
  for (const side of [-1, 1]) decal(s, skin, nf, oval(side * 29, 315.5, 10, 5.5, 8), blend('skin', 'blush', 0.75), 0.0009);

  sculptHair(s);
}

// Where a lock starts or ends: on the hair's cap or the head's skin at an
// angle (degrees from the front toward +x) and height (px), `out` px
// further out from the head's upright axis.
function onCap(az: number, y: number, out = 0): THREE.Vector3 {
  return outward(stackPoint(CAP, THREE.MathUtils.degToRad(az), y), out);
}

function onHead(az: number, y: number, out = 0): THREE.Vector3 {
  return outward(stackPoint(HEAD_DOWN, THREE.MathUtils.degToRad(az), y), out);
}

function outward(p: THREE.Vector3, out: number): THREE.Vector3 {
  const d = new THREE.Vector3(p.x, 0, p.z - 8 * U);
  if (d.lengthSq() > 1e-12) p.addScaledVector(d.normalize(), out * U);
  return p;
}

// The locks: chunky pointed shingles from a root on the cap to a tip, their
// middles standing up: [root angle, root height, tip angle, tip height, tip
// out, root half width, lift] (degrees and px), the tip on the skin
// (hanging over the face, ears and nape) or on the cap. Traced from the
// front, side and back views and the face close-up.
type Lock = readonly [number, number, number, number, number, number, number];
const FRINGE: Lock[] = [
  [-54, 366, -60, 346, 5, 15, 7],
  [-33, 369, -31, 347, 6, 17, 8],
  [-11, 370, -13, 351, 6, 16, 8],
  [11, 370, 9, 356, 6, 15, 8],
  [32, 369, 34, 349, 6, 17, 8],
  [54, 366, 60, 347, 5, 15, 7],
];
const HANGING: Lock[] = [
  // Down the sides, ending over the ears.
  [-74, 368, -77, 344, 6, 16, 7],
  [-98, 366, -101, 341, 7, 16, 7],
  [-122, 364, -126, 334, 7, 15, 7],
  [74, 368, 77, 344, 6, 16, 7],
  [98, 366, 101, 341, 7, 16, 7],
  [122, 364, 126, 334, 7, 15, 7],
  // In front of the ears.
  [-60, 358, -60, 330, 3, 8, 3],
  [60, 358, 60, 330, 3, 8, 3],
  // Down the back to the nape, full, as the sheet's back view has it.
  [-145, 366, -150, 314, 17, 17, 7],
  [-166, 365, -168, 306, 19, 17, 7],
  [166, 365, 168, 306, 19, 17, 7],
  [145, 366, 150, 314, 17, 17, 7],
  [180, 364, 180, 302, 18, 14, 6],
  [-126, 366, -130, 322, 14, 15, 6],
  [126, 366, 130, 322, 14, 15, 6],
];
const ON_CAP: Lock[] = [
  // Flaring out at the top of the sides: the hair's wide, spiky outline.
  [-78, 395, -91, 387, 11, 12, 6],
  [-102, 385, -109, 372, 10, 12, 6],
  [78, 395, 91, 387, 11, 12, 6],
  [102, 385, 109, 372, 10, 12, 6],
  // Big locks lying back from the crown.
  [-140, 418, -150, 394, 3, 17, 7],
  [-60, 419, -66, 398, 3, 17, 7],
  [40, 419, 46, 399, 3, 17, 7],
  [120, 418, 130, 395, 3, 17, 7],
  // The forelock jutting forward over the fringe.
  [6, 407, 9, 394, 20, 12, 6],
  [-18, 404, -22, 390, 14, 11, 6],
];

function sculptHair(s: Sculpt): void {
  const tops: Tone[] = [tone('hair'), blend('hair', 'hairLight', 0.55), blend('hair', 'hairDark', 0.3), tone('hairLight', 0.94), tone('hair', 1.05)];
  bands(
    s,
    CAP.map((c) => slice(c, 10, Math.PI / 10)),
    (r, k) => (r < 0 ? tone('hairDark') : patchy('hair', (r + k) % 2 ? 'hairLight' : 'hairDark', r, k, 0.4, 0.35)),
    P(0, 423, 6),
    'flat',
  );
  const centre = P(0, 350, 8);
  const lock = (root: THREE.Vector3, tip: THREE.Vector3, half: number, lift: number, i: number) => {
    const normal = root.clone().sub(centre).normalize();
    const across = new THREE.Vector3().crossVectors(tip.clone().sub(root), normal).normalize();
    const a = root.clone().addScaledVector(across, -half * U);
    const b = root.clone().addScaledVector(across, half * U);
    spike(s, a, b, tip, normal.multiplyScalar(lift * U), tops[i % tops.length], tone('hairDark'));
  };
  let i = 0;
  for (const [ra, ry, ta, ty, out, half, lift] of [...FRINGE, ...HANGING]) lock(onCap(ra, ry, -2), onHead(ta, ty, out), half, lift, i++);
  for (const [ra, ry, ta, ty, out, half, lift] of ON_CAP) lock(onCap(ra, ry, -2), onCap(ta, ty, out), half, lift, i++);
  // The tuft on the crown: a lock standing up and over to the left, and a
  // smaller one to the right.
  lock(onCap(-12, 419, -3), P(-22, 437, 10), 11, 5, 1);
  lock(onCap(-40, 417, -3), P(-26, 433, 2), 8, 4, 0);
  lock(onCap(28, 416, -3), P(44, 425, 4), 7, 3, 3);
}

// ------------------------------------------------------------ the torso

// The shirt: a cream tunic, bloused over the belt, its skirt flaring to a
// hem that dips to a point in front.
const SHIRT: Slice[] = [
  { y: 272, z: 13, w: 21, f: 17, b: 16 },
  { y: 258, z: 15, w: 42, f: 28, b: 26 },
  { y: 236, z: 18, w: 49, f: 32, b: 30 },
  { y: 212, z: 19, w: 47.5, f: 33, b: 30 },
  { y: 199, z: 19, w: 45.5, f: 32.5, b: 30 },
  { y: 191, z: 18, w: 44, f: 31, b: 29 },
  { y: 184, z: 18, w: 45, f: 31.5, b: 29.5 },
  { y: 166, z: 16, w: 51, f: 35, b: 32 },
  { y: 150, z: 14, w: 55, f: 37.5, b: 34, drop: (a: number) => 14 * Math.max(0, Math.cos(a)) ** 6 },
].map((s) => ({ ...s, n: 2.4 }));

// The body on the spine: the shirt, the neck, the scarf and the cape down
// the back, the belt with its buckle and pouches. `glow` takes the pouch's
// glowing leaf.
export function sculptTorso(s: Sculpt, glow: Sculpt): void {
  const sides = 10;
  bands(
    s,
    SHIRT.map((sl) => slice(sl, sides, Math.PI / sides)),
    (r, k) => patchy('shirt', 'shirtShade', r, k, 0.3, 0.22),
    'flat',
    'open',
  );

  // The neck, from inside the collar up into the head.
  bands(
    s,
    [258, 298].map((y, i) => yRing(y, 11.5, 10.5, 10.5, 6, 2, Math.PI / 6, 0, 12 + i * 5)),
    () => tone('skin', 0.95),
    'flat',
    'flat',
  );

  // The scarf: a thick green cowl rolled round the neck up to the chin and
  // over the shoulders, its front falling to a point on the chest.
  const front = (a: number, lift: number, point: number) => lift * Math.max(0, Math.cos(a)) + point * Math.max(0, Math.cos(a)) ** 8;
  const cowl: Slice[] = [
    { y: 288, z: 14, w: 15, f: 14, b: 13 },
    { y: 286, z: 15, w: 26, f: 24, b: 21 },
    { y: 279, z: 16, w: 37, f: 33, b: 29 },
    { y: 268, z: 17, w: 47, f: 39, b: 34, drop: (a: number) => front(a, 3, 0) },
    { y: 257, z: 17, w: 55, f: 42, b: 36, drop: (a: number) => front(a, 6, 6) },
    { y: 248, z: 17, w: 57, f: 41, b: 36, drop: (a: number) => front(a, 8, 16) },
    { y: 245, z: 17, w: 49, f: 33, b: 30, drop: (a: number) => front(a, 7, 14) },
  ].map((c) => ({ ...c, n: 2.2 }));
  bands(
    s,
    cowl.map((sl) => slice(sl, 12, Math.PI / 12)),
    (r, k) => {
      if (r >= 5) return tone('greenDark', 1.05);
      if (r === 4) return k % 2 === 0 ? tone('greenDark', 1.18) : tone('green', 0.97); // folds in the drape
      if (r === 2) return (k + 1) % 3 === 0 ? tone('greenDark', 1.15) : tone('green', 1.04); // the roll's top
      return (r + k) % 3 === 0 ? blend('green', 'greenDark', 0.35) : patchy('green', 'greenDark', r, k, 0.3, 0.25);
    },
    'open',
    'open',
  );
  // Its gold button on the roll, to the figure's left.
  const button = new THREE.Vector3(-0.38, 0.15, 0.91).normalize();
  const bc = P(-12, 262, 54);
  const bx = new THREE.Vector3().crossVectors(Y, button).normalize();
  const by = new THREE.Vector3().crossVectors(button, bx);
  const coin = [
    ring(bc.clone().addScaledVector(button, -2 * U), bx, by, 7.5, 7.5, 7.5, 7, 2, Math.PI / 7),
    ring(bc.clone().addScaledVector(button, 1.5 * U), bx, by, 7.3, 7.3, 7.3, 7, 2, Math.PI / 7),
    ring(bc.clone().addScaledVector(button, 2.4 * U), bx, by, 5, 5, 5, 7, 2, Math.PI / 7),
  ];
  bands(s, coin, (r) => (r === 1 ? tone('goldDark', 1.05) : tone('gold')), 'flat', 'flat');

  // The cape down the back, over the backpack: a green cloth panel from
  // under the bedroll to a point, with the leaf emblem.
  sculptCape(s);

  // The belt, with its gold buckle.
  const belt: Slice[] = [
    { y: 200, z: 19, w: 46.3, f: 33.3, b: 30.3 },
    { y: 198, z: 19, w: 47.5, f: 34.5, b: 31.5 },
    { y: 182, z: 18, w: 47, f: 34, b: 31 },
    { y: 180, z: 18, w: 45.5, f: 32, b: 29.5 },
  ].map((b) => ({ ...b, n: 2.4 }));
  bands(
    s,
    belt.map((sl) => slice(sl, 12, Math.PI / 12)),
    (r, k) => (r === 1 ? patchy('leatherDark', 'leather', r, k, 0.25) : tone('leatherDark', 0.9)),
    'open',
    'open',
  );
  const bz = 19 + 34.5 * Math.cos(Math.PI / 12) + 0.8;
  box(s, P(0, 198.5, bz), X, Y, Z, 13.5, 2.3, 1.4, gold);
  box(s, P(0, 179.5, bz), X, Y, Z, 13.5, 2.3, 1.4, gold);
  box(s, P(-11.5, 189, bz), X, Y, Z, 2.2, 7.5, 1.4, gold);
  box(s, P(11.5, 189, bz), X, Y, Z, 2.2, 7.5, 1.4, gold);

  // A leather pouch hanging from the belt on the figure's left, a smaller
  // one on its right with a glowing leaf on its flap.
  pouch(s, glow, -1, 168, 15, 19, 7, false);
  pouch(s, glow, 1, 165, 11, 14, 6, true);
}

// A pouch on the belt at `side`, its middle `y` px up, `w` wide and `h`
// tall (half sizes), `d` deep, facing out from the hip.
function pouch(s: Sculpt, glow: Sculpt, side: number, y: number, w: number, h: number, d: number, glowing: boolean): void {
  const a = side * (glowing ? 1.05 : 1.12); // round from the front
  const out = new THREE.Vector3(Math.sin(a), 0, Math.cos(a) * 1.05).normalize();
  const along = new THREE.Vector3().crossVectors(Y, out).normalize();
  // On the skirt's surface there: 51 px wide, 35 in front (the shirt at 166).
  const at = P(51 * Math.sin(a) * 0.98, y, 16 + 35 * Math.cos(a) * 0.98).addScaledVector(out, (d + 1) * U);
  const rings = [-h, -h + 3, h].map((dy, i) => ring(at.clone().addScaledVector(Y, dy * U), along, out, w - (i === 0 ? 2 : 0), d - (i === 0 ? 1.5 : 0), d - (i === 0 ? 1.5 : 0), 6, 3.2, Math.PI / 6));
  bands(s, rings, (r, k) => (r >= 1 && (k === 5 || k === 0) ? tone('leatherLight') : tone('leather')), 'flat', 'flat');
  // The flap over its top, and a toggle, or a glowing leaf.
  const flapTop = at.clone().addScaledVector(Y, h * U).addScaledVector(out, (d + 0.6) * U);
  const flapBottom = at.clone().addScaledVector(Y, -h * 0.15 * U).addScaledVector(out, (d + 1.2) * U);
  const fl = [flapTop.clone().addScaledVector(along, -(w + 0.5) * U), flapTop.clone().addScaledVector(along, (w + 0.5) * U), flapBottom.clone().addScaledVector(along, (w - 1) * U), flapBottom.clone().addScaledVector(along, -(w - 1) * U)];
  const flapTip = at.clone().addScaledVector(Y, -h * 0.35 * U).addScaledVector(out, (d + 1.3) * U);
  s.toward(fl[0], fl[3], fl[1], out, tone('leatherDark', 1.1));
  s.toward(fl[1], fl[3], fl[2], out, tone('leatherDark', 1.1));
  s.toward(fl[3], flapTip, fl[2], out, tone('leatherDark', 1.1));
  if (glowing) {
    const c = at.clone().addScaledVector(Y, -h * 0.05 * U).addScaledVector(out, (d + 1.9) * U);
    const leaf = [c.clone().addScaledVector(Y, 5 * U), c.clone().addScaledVector(along, 3.5 * U), c.clone().addScaledVector(Y, -4 * U), c.clone().addScaledVector(along, -3.5 * U)];
    glow.toward(leaf[0], leaf[3], leaf[1], out, tone('glow'));
    glow.toward(leaf[1], leaf[3], leaf[2], out, tone('glow'));
  } else {
    box(s, flapTip.clone().addScaledVector(out, 0.8 * U), along, Y, out, 1.3, 3, 1, gold);
  }
}

// The cape's back face (z, px) at a height: tucked under the bedroll at its
// top, then over the backpack's back.
function capeZ(y: number): number {
  return y > 232 ? -58 - (8 * (258 - y)) / 26 : -66 - (232 - y) / 60;
}

function sculptCape(s: Sculpt): void {
  // Its outline, from the top left round by the point: x, y in px.
  const outline: [number, number][] = [
    [-44, 258],
    [-47, 232],
    [-45, 200],
    [-41, 172],
    [-30, 165],
    [-22, 160],
    [-12, 155],
    [0, 146],
    [12, 155],
    [22, 160],
    [30, 165],
    [41, 172],
    [45, 200],
    [47, 232],
    [44, 258],
  ];
  const THICK = 2.2;
  const back = outline.map(([x, y]) => P(x, y, capeZ(y) - THICK / 2));
  const front = outline.map(([x, y]) => P(x, y, capeZ(y) + THICK / 2));
  const hub = P(0, 205, capeZ(205) - THICK / 2);
  const hubIn = P(0, 205, capeZ(205) + THICK / 2);
  const out = new THREE.Vector3(0, 0, -1);
  for (let i = 0; i + 1 < outline.length; i++) {
    const t = (i + 1) % 3 === 0 ? blend('green', 'greenDark', 0.3) : tone('green', 1.02);
    s.toward(hub, back[i], back[i + 1], out, t);
    s.toward(hubIn, front[i + 1], front[i], out.clone().negate(), tone('greenDark'));
    // Its edge.
    const edgeOut = new THREE.Vector3().subVectors(back[i + 1], back[i]).cross(Z).normalize();
    s.toward(back[i], front[i], front[i + 1], edgeOut, tone('greenDark', 0.9));
    s.toward(back[i], front[i + 1], back[i + 1], edgeOut, tone('greenDark', 0.9));
  }
  s.toward(hub, back[outline.length - 1], back[0], out, tone('green'));
  s.toward(hubIn, front[0], front[outline.length - 1], out.clone().negate(), tone('greenDark'));

  // The leaf emblem: a leaf standing up and one to either side, in pale
  // yellow-green, a little proud of the cloth.
  const leafAt = (cx: number, cy: number, angle: number, length: number, width: number, t: Tone, both = false) => {
    const dir = new THREE.Vector3(Math.sin(angle), Math.cos(angle), 0);
    const across = new THREE.Vector3(Math.cos(angle), -Math.sin(angle), 0);
    const base = P(cx, cy, capeZ(cy) - THICK / 2 - 0.5);
    const tip = base.clone().addScaledVector(dir, length * U);
    const mid = base.clone().addScaledVector(dir, length * 0.45 * U);
    const l = mid.clone().addScaledVector(across, -width * U);
    const r = mid.clone().addScaledVector(across, width * U);
    const midline = base.clone().addScaledVector(dir, length * 0.5 * U).addScaledVector(out, 0.4 * U);
    s.toward(base, l, midline, out, t);
    s.toward(l, tip, midline, out, blend('leaf', 'white', 0.18));
    s.toward(base, midline, r, out, t);
    s.toward(midline, tip, r, out, t);
    if (both) {
      const inward = out.clone().negate();
      s.toward(base, l, tip, inward, tone('leaf', 0.85));
      s.toward(base, tip, r, inward, tone('leaf', 0.85));
    }
  };
  leafAt(0, 181, 0, 42, 8.5, tone('leaf'));
  leafAt(-3, 180, -0.95, 25, 6.5, tone('leaf', 0.96));
  leafAt(3, 180, 0.95, 25, 6.5, tone('leaf', 0.96));
  // Two little leaves hanging from its lower corners.
  for (const side of [-1, 1]) leafAt(side * 31, 165, Math.PI - side * 0.12, 13, 4, tone('leaf', 1.02), true);
}

// ------------------------------------------------------------ the shorts

// Their seat, on the hips: from under the belt to the crotch, wide and
// baggy as the sheet draws them.
export function sculptSeat(s: Sculpt): void {
  const seat: Slice[] = [
    { y: 186, z: 16, w: 40, f: 27, b: 26 },
    { y: 164, z: 11, w: 47, f: 30, b: 29 },
    { y: 146, z: 6, w: 54, f: 31, b: 32 },
    { y: 132, z: 3, w: 54, f: 29, b: 32 },
    { y: 121, z: 1, w: 34, f: 20, b: 25 },
  ].map((sl) => ({ ...sl, n: 2.3 }));
  bands(
    s,
    seat.map((sl) => slice(sl, 10, Math.PI / 10)),
    (r, k) => patchy('shorts', 'shortsDark', r, k, 0.4, 0.3),
    'flat',
    'flat',
  );
}

// ------------------------------------------------------------ a leg

// A leg's thigh: the shorts' leg, from its top inside the seat to the hem
// round the knee, with a pocket on its outer side. In the leg's own space:
// the hip joint at its origin, the leg down -y; `side` 1 is the right (+x).
export function sculptThigh(s: Sculpt, side: number): void {
  const rings = [
    [16, 22, 20, 22],
    [5, 29.5, 27.5, 29.5],
    [-18, 31.5, 29, 30],
    [-46, 32.5, 29.5, 30.5],
    [-58, 33, 30, 31],
    [-63.5, 31.5, 28.5, 29.5],
    [-64.5, 20, 19, 20],
  ].map(([y, w, f, b]) => yRing(y, w, f, b, 8, 2.2, Math.PI / 8));
  bands(s, rings, (r, k) => (r >= 5 ? tone('shortsDark', 0.8) : r === 4 ? tone('shortsDark', 1.05) : patchy('shorts', 'shortsDark', r, k + side, 0.4, 0.3)), 'open', 'flat');
  // The pocket: a flap on the front of its outer side.
  const a = side * 1.0;
  const out = new THREE.Vector3(Math.sin(a), 0, Math.cos(a)).normalize();
  const along = new THREE.Vector3().crossVectors(Y, out).normalize();
  const at = P(31 * Math.sin(a) * 0.93, -26, 29 * Math.cos(a) * 0.93).addScaledVector(out, 1.4 * U);
  box(s, at, along, Y, out, 8.5, 11, 1.2, (f) => (f === 'z' ? tone('shortsDark', 1.1) : tone('shortsDark', 0.85)));
  box(s, at.clone().addScaledVector(Y, 8 * U).addScaledVector(out, 1.3 * U), along, Y, out, 9, 3.5, 1.1, (f) => (f === 'z' ? tone('shorts', 1.12) : tone('shortsDark')));
}

// The shin, bare, from inside the hem to inside the boot, and the boot's
// shaft and its thick turned-down cuff round the lower shin: in the knee's
// space.
export function sculptShin(s: Sculpt): void {
  const rings = [
    [8, 11, 10, 11],
    [-12, 12, 11, 13],
    [-40, 10, 9.5, 10.5],
  ].map(([y, w, f, b]) => yRing(y, w, f, b, 6, 2, Math.PI / 6));
  bands(s, rings, () => tone('skin'), 'flat', 'flat');
  const boot = [
    [-54, 17, 18, 18],
    [-37, 19, 20, 20],
    [-35.5, 25, 26, 26],
    [-31, 27, 28, 28],
    [-21, 27.5, 28.5, 28.5],
    [-18, 25.5, 26.5, 26.5],
    [-17, 15, 15, 15.5],
  ].map(([y, w, f, b]) => yRing(y, w, f, b, 8, 2.4, Math.PI / 8));
  bands(s, boot, (r, k) => (r <= 1 ? patchy('boot', 'bootDark', r, k, 0.3) : r === 5 ? tone('bootDark') : patchy('cuff', 'boot', r, k, 0.35, 0.3)), 'open', 'flat');
}

// The boot's foot, in the ankle's space: heel back, toe forward, its sole
// on the ground (24 px below the ankle), a strap over the instep with a
// gold buckle on its outer side.
export function sculptFoot(s: Sculpt, side: number): void {
  const slices = [
    [-27, -14, 16, 8, 10],
    [-19, -11, 20, 13, 13],
    [-4, -8, 22.5, 16, 16],
    [16, -11, 24, 12.5, 13],
    [36, -14.5, 23.5, 8.5, 9.5],
    [51, -15.5, 20.5, 7, 8.5],
    [59, -17, 14, 4.5, 7],
  ];
  const rings = slices.map(([z, cy, w, up, down]) => ring(P(0, cy, z), X, Y, w, up, down, 8, 3, Math.PI / 8));
  bands(
    s,
    rings,
    (r, k) => {
      // Faces run from the top round toward +x: 3 is underneath, 2 and 4
      // its lower sides, 7 on top.
      if (k === 3) return tone('sole');
      if (k === 2 || k === 4) return r <= 0 || r >= 6 ? tone('sole', 1.1) : tone('bootDark', 0.95);
      if (r === 2 && (k === 7 || k === 0 || k === 6)) return tone('bootDark', 1.05); // the strap over the instep
      return patchy('boot', 'bootDark', r, k, 0.3, 0.25);
    },
    'flat',
    'flat',
  );
  box(s, P(side * 18.5, 0, 12), Z, Y, new THREE.Vector3(side, 0, 0), 3.5, 3, 1.2, gold);
}

// ------------------------------------------------------------ an arm

// The upper arm: the short, puffy sleeve with its rolled cuff, and the bare
// arm below it to the elbow. In the shoulder's space, down -y.
export function sculptUpperArm(s: Sculpt, side: number): void {
  const sleeve = [
    [11, 11, 11, 11],
    [2, 19, 18.5, 18.5],
    [-30, 22.5, 22, 21.5],
    [-36, 24.8, 24.3, 23.8],
    [-47, 23.5, 23, 22.5],
    [-48.5, 13, 12.5, 12.5],
  ].map(([y, w, f, b]) => yRing(y, w, f, b, 8, 2.2, Math.PI / 8));
  bands(
    s,
    sleeve,
    (r, k) => {
      if (r >= 4) return tone('shirtShade', 0.85);
      if (r === 3) return (k + side + 8) % 2 === 0 ? tone('shirtLight') : tone('shirtLight', 0.95); // the rolled cuff
      return patchy('shirt', 'shirtShade', r, k + (side > 0 ? 4 : 0), 0.35, 0.25);
    },
    'flat',
    'flat',
  );
  bands(
    s,
    [
      [-38, 11, 11, 11],
      [-58, 10, 10, 10],
    ].map(([y, w, f, b]) => yRing(y, w, f, b, 6, 2, Math.PI / 6)),
    () => tone('skin', 0.97),
    'flat',
    'flat',
  );
}

// The forearm, with its thick leather bracer, in the elbow's space.
export function sculptForearm(s: Sculpt): void {
  bands(
    s,
    [
      [7, 10, 10, 10],
      [-8, 10.5, 10.5, 10.5],
    ].map(([y, w, f, b]) => yRing(y, w, f, b, 6, 2, Math.PI / 6)),
    () => tone('skin'),
    'flat',
    'flat',
  );
  const bracer = [
    [-5, 13.5, 13.5, 13.5],
    [-9, 16.2, 16.2, 16.2],
    [-13, 16.3, 16.3, 16.3],
    [-30, 15.5, 15.5, 15.5],
    [-34, 12, 12, 12],
  ].map(([y, w, f, b]) => yRing(y, w, f, b, 8, 2.5, Math.PI / 8));
  // Dark leather, a paler strap round its top.
  bands(s, bracer, (r, k) => (r === 1 ? tone('bracerStrap', k % 2 ? 0.95 : 1.05) : patchy('bracer', 'leatherDark', r, k, 0.35, 0.3)), 'flat', 'flat');
}

// An open hand, relaxed, its palm toward the body and its thumb forward, in
// the wrist's space: as big as the sheet draws a chibi's hands.
export function sculptHand(s: Sculpt, side: number): void {
  const inward = -side;
  const palm = [
    [3, 7, 10, 10],
    [-7, 7.5, 12.5, 12],
    [-15, 7, 13, 12.5],
    [-18, 5.5, 12, 11.5],
  ].map(([y, w, f, b]) => yRing(y, w, f, b, 6, 2.4, Math.PI / 6));
  bands(s, palm, (_r, k) => (Math.sign(Math.round(Math.sin(faceAngle(k, 6)) * 100)) === inward ? tone('skin', 0.93) : tone('skin')), 'flat', 'flat');
  // Four chunky fingers, curling a little toward the palm, and the thumb.
  const fingers = [
    [8.5, 21],
    [3, 23],
    [-3, 22],
    [-8.5, 19],
  ];
  for (const [z, length] of fingers) {
    const path = [P(0, -15, z), P(inward * 1.5, -15 - length * 0.5, z), P(inward * 3.8, -15 - length * 0.9, z)];
    const radii: [number, number, number][] = [
      [2.9 * U, 2.6 * U, 2.6 * U],
      [2.7 * U, 2.4 * U, 2.4 * U],
      [2.4 * U, 2.2 * U, 2.2 * U],
    ];
    loft(s, tubeRings(path, radii, Z), { sides: 4, turn: Math.PI / 4, end: 2.2 * U, start: 'flat', paint: (_r, angle) => (angle > Math.PI * 0.6 && angle < Math.PI * 1.4 ? tone('skin', 0.93) : tone('skin')) });
  }
  const thumb = [P(inward * 1.5, -3, 10), P(inward * 3.5, -10, 14.5), P(inward * 5.5, -16, 15.5)];
  loft(
    s,
    tubeRings(thumb, [
      [3.4 * U, 3 * U, 3 * U],
      [3.1 * U, 2.8 * U, 2.8 * U],
      [2.7 * U, 2.4 * U, 2.4 * U],
    ]),
    { sides: 4, turn: Math.PI / 4, end: 2.5 * U, start: 'flat', paint: () => tone('skin') },
  );
}

// A fist, closed round a handle, in the wrist's space.
export function sculptFist(s: Sculpt, side: number): void {
  const inward = -side;
  const fist = [
    [3, 7.5, 10.5, 10],
    [-6, 10.5, 13, 12],
    [-17, 11, 13, 12],
    [-23, 8.5, 10.5, 9.5],
  ].map(([y, w, f, b]) => yRing(y, w, f, b, 8, 2.8, Math.PI / 8, inward * 1));
  bands(
    s,
    fist,
    (r, k) => {
      const a = faceAngle(k, 8);
      // The curled fingers' front and the palm side a little darker.
      if (r >= 1 && Math.sign(Math.sin(a)) === inward && Math.cos(a) > -0.3) return tone('skin', 0.9);
      return tone('skin');
    },
    'flat',
    'flat',
  );
  // The thumb, wrapped over the front of the fingers.
  const thumb = [P(inward * 2.5, -5, 12), P(inward * 7.5, -11, 13.5), P(inward * 10.5, -15, 11)];
  loft(
    s,
    tubeRings(thumb, [
      [3.3 * U, 2.8 * U, 2.8 * U],
      [3 * U, 2.6 * U, 2.6 * U],
      [2.6 * U, 2.4 * U, 2.4 * U],
    ]),
    { sides: 4, turn: Math.PI / 4, end: 2 * U, start: 'flat', paint: () => tone('skin', 1.02) },
  );
}

// ------------------------------------------------------------ the backpack

// The backpack on the back: a leather pack with side pockets, a cream
// bedroll strapped on top (rolled, a spiral at each end), the straps over
// the shoulders and down the chest, the green knot of the scarf on the
// bedroll, and a hook on its right side for the lantern. Where it stands in
// the figure. `flat` takes a plain copy of the bedroll, to lay things on.
export function sculptPack(s: Sculpt, flat: Sculpt): void {
  const pack: Slice[] = [
    { y: 162, z: -36, w: 46, f: 20, b: 20 },
    { y: 168, z: -36, w: 51, f: 23, b: 23 },
    { y: 246, z: -37, w: 52, f: 23, b: 23 },
    { y: 253, z: -37, w: 48, f: 21, b: 21 },
  ].map((p) => ({ ...p, n: 4 }));
  bands(
    s,
    pack.map((sl) => slice(sl, 10, Math.PI / 10)),
    (r, k) => (r === 1 ? patchy('leather', 'leatherDark', r, k, 0.35, 0.3) : tone('leatherDark', 1.1)),
    'flat',
    'flat',
  );
  // A pocket on either side, its flap darker.
  for (const side of [-1, 1]) {
    box(s, P(side * 59, 200, -37), X, Y, Z, 8, 26, 13, (f) => (f === (side > 0 ? 'x' : '-x') ? tone('leatherLight') : f === 'y' ? tone('leatherDark', 1.1) : tone('leather', 0.95)));
  }

  // The bedroll: a cream roll along x on top, strapped twice.
  const at = (x: number) => P(x, 274, -42);
  const roll = [
    [-62, 17],
    [-60, 20],
    [60, 20],
    [62, 17],
  ].map(([x, r]) => ring(at(x), Z, Y, r, r, r, 8, 2, Math.PI / 8));
  bands(s, roll, (r, k) => (r !== 1 ? tone('creamShade') : k % 3 === 0 ? tone('creamShade', 1.05) : patchy('cream', 'creamShade', r, k, 0.4, 0.3)), 'flat', 'flat');
  bands(flat, roll, () => tone('cream'), 'flat', 'flat');
  for (const side of [-1, 1]) {
    const strap = [side * 24 - 3.5, side * 24 + 3.5].map((x) => ring(at(x), Z, Y, 21.3, 21.3, 21.3, 8, 2, Math.PI / 8));
    bands(s, strap, (_r, k) => tone('leatherDark', k % 2 ? 0.95 : 1.05), 'open', 'open');
  }
  // A spiral at each end, where the roll shows.
  for (const side of [-1, 1]) {
    const f: Frame = { origin: P(side * 75, 274, -42), right: new THREE.Vector3(0, 0, side), up: Y.clone(), out: new THREE.Vector3(side, 0, 0) };
    const spiral: [number, number][] = [];
    for (let i = 0; i <= 10; i++) {
      const a = (i / 10) * Math.PI * 3;
      const r = 2.5 + (i / 10) * 13;
      spiral.push([r * Math.cos(a), r * Math.sin(a)]);
    }
    strip(s, flat, f, spiral, () => 2.4, tone('leather', 1.05), 0.0008);
  }
  // The scarf's knot on the bedroll's back: a green diamond.
  const kf: Frame = { origin: P(0, 272, -90), right: new THREE.Vector3(-1, 0, 0), up: Y.clone(), out: new THREE.Vector3(0, 0, -1) };
  decal(
    s,
    flat,
    kf,
    [
      [0, 8],
      [-10, 0],
      [0, -8],
      [10, 0],
    ],
    tone('green', 1.05),
    0.001,
  );
  // The straps from the bedroll down onto the cape, buckled.
  for (const side of [-1, 1]) {
    box(s, P(side * 24, 241, -66.8), X, Y, Z, 4.6, 2.2, 0.9, (f) => (f === '-z' ? tone('gold', 0.9) : tone('goldDark')));
  }

  // The shoulder straps: over each shoulder from the bedroll and down the
  // front of the chest to the belt.
  for (const side of [-1, 1]) {
    const path = [P(side * 25, 262, -24), P(side * 29, 272, 0), P(side * 33, 266, 22), P(side * 37, 252, 40), P(side * 40, 230, 49.5), P(side * 41, 206, 50.5), P(side * 41.5, 199, 50)];
    const radii = path.map(() => [4.6 * U, 1.4 * U, 1.4 * U] as [number, number, number]);
    loft(s, tubeRings(path, radii, X), { sides: 4, turn: Math.PI / 4, start: 'flat', end: 'flat', paint: () => tone('leatherDark') });
    // A small brass buckle on the chest.
    box(s, P(side * 40.3, 226, 51.2), X, Y, Z, 3.4, 2.4, 0.8, gold);
  }
  // The lantern's hook.
  const [hx, hy, hz] = JOINTS.hook;
  box(s, P(hx - 3.5, hy + 4, hz), X, Y, Z, 3.5, 1.3, 1.3, () => tone('copperDark'));
  box(s, P(hx, hy + 1.5, hz), X, Y, Z, 1.1, 3.2, 1.1, () => tone('copperDark'));
}

// ------------------------------------------------------------ the lantern

export const LANTERN_HEIGHT = 46; // px from the top of its ring to its base

// The small lantern, as the sheet's accessories and tools draw it: a ring,
// a cone of a roof, a glowing glass body in a frame of four posts with an X
// across its front and back, and a round base. In its own space: hanging
// from the top of its ring, down -y. `glass` takes the glowing glass.
export function sculptLantern(s: Sculpt, glass: Sculpt): void {
  // The ring on top, facing forward.
  const ringPath: V3[] = [];
  for (let i = 0; i <= 8; i++) {
    const a = (i / 8) * Math.PI * 2;
    ringPath.push(P(4.2 * Math.sin(a), -4.8 + 4.2 * Math.cos(a), 0));
  }
  loft(s, tubeRings(ringPath, ringPath.map(() => 1.1 * U), Z), { sides: 3, paint: () => tone('copperDark') });
  const lathe = (list: [number, number][], t: (r: number) => Tone, target: Sculpt) =>
    bands(
      target,
      list.map(([y, r]) => yRing(y, r, r, r, 6, 2, Math.PI / 6)),
      (r) => t(r),
      'flat',
      'flat',
    );
  // Roof and rim.
  lathe(
    [
      [-8.5, 2.2],
      [-14.5, 8.5],
      [-15.5, 9.8],
      [-17, 9.6],
    ],
    (r) => (r === 1 ? tone('copper', 1.1) : tone('copper')),
    s,
  );
  // The glass, glowing.
  lathe(
    [
      [-17, 7.4],
      [-25.5, 8.6],
      [-34, 7.4],
    ],
    () => tone('glass'),
    glass,
  );
  // Base.
  lathe(
    [
      [-33.5, 9.2],
      [-35.5, 10],
      [-39.5, 9.4],
      [-46, 6.5],
    ],
    (r) => (r === 1 ? tone('copperDark') : tone('copper', 0.95)),
    s,
  );
  // Four posts up the glass, and an X across its front and back.
  for (let i = 0; i < 4; i++) {
    const a = Math.PI / 6 + (i * Math.PI) / 2;
    const out = new THREE.Vector3(Math.sin(a), 0, Math.cos(a));
    const along = new THREE.Vector3().crossVectors(Y, out);
    box(s, P(8.8 * Math.sin(a), -25.5, 8.8 * Math.cos(a)), along, Y, out, 1.1, 8.8, 1.1, () => tone('copperDark'));
  }
  for (const f of [1, -1]) {
    for (const lean of [0.62, -0.62]) {
      const dir = new THREE.Vector3(Math.sin(lean), Math.cos(lean), 0);
      const across = new THREE.Vector3(Math.cos(lean), -Math.sin(lean), 0);
      const c = P(0, -25.5, 8.4 * f);
      const face = Z.clone().multiplyScalar(f);
      const a = c.clone().addScaledVector(dir, -9.5 * U).addScaledVector(across, -0.8 * U);
      const b = c.clone().addScaledVector(dir, -9.5 * U).addScaledVector(across, 0.8 * U);
      const cc = c.clone().addScaledVector(dir, 9.5 * U).addScaledVector(across, 0.8 * U);
      const d = c.clone().addScaledVector(dir, 9.5 * U).addScaledVector(across, -0.8 * U);
      s.toward(a, b, cc, face, tone('copperDark'));
      s.toward(a, cc, d, face, tone('copperDark'));
    }
  }
}
