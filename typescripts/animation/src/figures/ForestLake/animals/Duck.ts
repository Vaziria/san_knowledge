import * as THREE from 'three';
import { ForestAnimal, type ForestAnimalOptions } from './ForestAnimal';
import { blend, bone, bump, coat, downAngle, ease, eye, gloss, loft, reach, rigidMesh, Sculpt, seededRandom, skinnedMesh, smooth, tone, weighAlong, type Palette, type Ring, type Tone, type Weights } from './parts';

// The forest lake's duck (Duck.md), a mallard drake as its reference sheet
// draws it: low poly, of flat facets, with a glossy green head, a yellow bill
// with a small dark nail, a white ring round the neck, a chestnut breast, pale
// grey-beige flanks, a brown back, a black rump (glossy green on top) with
// the drake's curl feathers over a white tail, folded wings of tan coverts
// over a brown band and a blue speculum edged in white, dark wing tips
// meeting over the rump, dark eyes with a glint, and orange legs and webbed
// feet. Its colours are the sheet's five variations.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground
// between its feet (on the water it floats with its origin SWIM_DEPTH under
// the surface, y = 0 of its parent). Its shape is traced from the sheet's
// side, front and top views in their pixels (U m each): 52 cm from bill to
// tail and 45 cm to its crown, standing, and 89 cm across its spread wings,
// as a mallard is.

type V3 = THREE.Vector3;
const TAU = Math.PI * 2;

const U = 0.002; // m per pixel of the sheet's side view
const FLOOR = 303; // the side view's ground line, in its image pixels
const MIDDLE = 966; // the side view's pixel column over the origin, between the feet

// A point traced from the sheet's side view, in its image pixels: `ix` along
// it (the bill to the left, so a smaller one is further forward), `iy` down
// it, and `x` pixels to the duck's right (+x).
function P(ix: number, iy: number, x = 0): V3 {
  return new THREE.Vector3(x * U, (FLOOR - iy) * U, (MIDDLE - ix) * U);
}

// The sheet's five colour variations, picked from its pictures (the lit
// faces, a little brighter). The drake's head is three roles (crown, face
// and the stripe the eye sits in) and its wings six, so that the mandarin's
// blue crown, orange cheeks and white eye stripe, the hen's plain brown head
// and the black duck's dark face fall where the sheet paints them.
const EYES = { iris: 0x3c2116, pupil: 0x0c0907, shine: 0xffffff, lid: 0x151313 };
const VARIATIONS: Record<string, Palette> = {
  mallard: {
    ...EYES,
    crown: 0x44aa5c,
    face: 0x2a8c58,
    stripe: 0x44aa5c,
    neck: 0x3a9c54,
    collar: 0xf6f2ec,
    bill: 0xffc840,
    nail: 0x503523,
    nostril: 0x3a281c,
    breast: 0x80402c,
    flank: 0xf3ddc9,
    mottle: 0xd6c1b5,
    belly: 0xeedac8,
    back: 0x80553f,
    rump: 0x27262e,
    curl: 0x1f1e25,
    sheen: 0x1d7a46,
    tail: 0xf3f0ed,
    coverts: 0xe7b592,
    pale: 0xf8ebda,
    tertial: 0x8a5842,
    speculum: 0x2049a8,
    edge: 0xf6f2ec,
    primary: 0x625452,
    under: 0xefe9e5,
    underTip: 0xa39b99,
    leg: 0xf8923a,
    claw: 0x5e4c4e,
  },
  white: {
    ...EYES,
    lid: 0x2a2624,
    crown: 0xf8f4f1,
    face: 0xf6f2ef,
    stripe: 0xf8f4f1,
    neck: 0xf7f3f0,
    collar: 0xf8f4f1,
    bill: 0xf8a83c,
    nail: 0xe0903a,
    nostril: 0x8a5a36,
    breast: 0xf7f2ef,
    flank: 0xf6f0ec,
    mottle: 0xe6e0dd,
    belly: 0xefe9e6,
    back: 0xece6e3,
    rump: 0xede8e5,
    curl: 0xe6e1df,
    sheen: 0xe9e4e1,
    tail: 0xf9f7f5,
    coverts: 0xf4efec,
    pale: 0xfbf9f7,
    tertial: 0xe8e2df,
    speculum: 0xebe6e4,
    edge: 0xf9f7f5,
    primary: 0xdfd9d7,
    under: 0xf7f4f2,
    underTip: 0xd9d3d1,
    leg: 0xf38c2e,
    claw: 0x7a5a4a,
  },
  brown: {
    ...EYES,
    crown: 0x9a6346,
    face: 0xa87352,
    stripe: 0x7e5039,
    neck: 0xa06a4c,
    collar: 0xa06a4c,
    bill: 0x7b777e,
    nail: 0x3c3a40,
    nostril: 0x2e2c30,
    breast: 0x8f5d44,
    flank: 0x94634a,
    mottle: 0x6b4531,
    belly: 0xeedac8,
    back: 0x6e4834,
    rump: 0x806b62,
    curl: 0x6b5a54,
    sheen: 0x74615a,
    tail: 0xd5c0b5,
    coverts: 0xb07a58,
    pale: 0xd0a484,
    tertial: 0x7a5038,
    speculum: 0x3a5aa8,
    edge: 0xf2eee8,
    primary: 0x6b5a52,
    under: 0xe9ddd5,
    underTip: 0xa8988e,
    leg: 0xf38c2e,
    claw: 0x5e4c4e,
  },
  mandarin: {
    ...EYES,
    crown: 0x4f86c8,
    face: 0xee7d42,
    stripe: 0xf6f2ee,
    neck: 0x4a7ec0,
    collar: 0xf6f2ee,
    bill: 0x63637a,
    nail: 0x2e2e3a,
    nostril: 0x2a2a34,
    breast: 0xe2677b,
    flank: 0xf07c3b,
    mottle: 0xe06a30,
    belly: 0xf2eef1,
    back: 0x2f5aa6,
    rump: 0x2c4a86,
    curl: 0x1f3466,
    sheen: 0x2f5aa6,
    tail: 0xf6f4f2,
    coverts: 0x3f68b2,
    pale: 0xf6f2ee,
    tertial: 0xf08040,
    speculum: 0x233f86,
    edge: 0xf6f2ee,
    primary: 0x3a3448,
    under: 0xf0e9e7,
    underTip: 0xa9a1a1,
    leg: 0xf38c2e,
    claw: 0x5e4c4e,
  },
  black: {
    ...EYES,
    crown: 0x2f7d4f,
    face: 0x24272d,
    stripe: 0x2a5c43,
    neck: 0x2c7649,
    collar: 0xeceee8,
    bill: 0xfcb450,
    nail: 0x4a3226,
    nostril: 0x3a2a1f,
    breast: 0x403e49,
    flank: 0x444148,
    mottle: 0x38353c,
    belly: 0x3c3a42,
    back: 0x36333b,
    rump: 0x27262c,
    curl: 0x201f25,
    sheen: 0x2a5c43,
    tail: 0xe9e5e4,
    coverts: 0x56515b,
    pale: 0x6c6772,
    tertial: 0x302d34,
    speculum: 0x2b2d3b,
    edge: 0x5c5762,
    primary: 0x2d2b31,
    under: 0x6b676f,
    underTip: 0x4b484f,
    leg: 0xf38c2e,
    claw: 0x4a4044,
  },
};

// A repeatable number in 0..1 for a pair of counts.
function hash(a: number, b: number): number {
  const h = Math.sin(a * 12.9898 + b * 78.233) * 43758.5453;
  return h - Math.floor(h);
}

// Now and then a facet a little toward another colour, as the sheet's
// facets are patched.
function patchy(role: string, to: string, r: number, k: number, amount: number, share = 0.3): Tone {
  const f = hash(r, k);
  return f < share ? blend(role, to, amount * (0.5 + (0.5 * f) / share)) : tone(role);
}

// ---------------------------------------------------------------- the body

// Its cross-sections from the rump to the breast, traced from the sheet's
// side view (the top and bottom at each column) and its front and top views
// (the width): [column, middle, half width, half height], in its pixels. The
// breast is full and round, bulging forward right under the white collar.
const BODY: readonly (readonly [number, number, number, number])[] = [
  [1064, 221, 15, 15],
  [1052, 221, 24, 23],
  [1038, 218.5, 33, 31.5],
  [1020, 216, 42, 40],
  [1000, 214, 50, 48],
  [978, 213, 56, 53],
  [955, 210.5, 58, 53.5],
  [932, 207, 56, 51.2],
  [912, 202, 53, 48.5],
  [896, 195.5, 47, 43.5],
  [884, 189, 40, 36.5],
  [876, 186, 32, 27],
  [871, 186, 20, 16],
];

// The body's middle, half width and half height at a column.
function bodyAt(ix: number): { cy: number; w: number; h: number } {
  const last = BODY.length - 1;
  const row = (r: readonly [number, number, number, number]) => ({ cy: r[1], w: r[2], h: r[3] });
  if (ix >= BODY[0][0]) return row(BODY[0]);
  if (ix <= BODY[last][0]) return row(BODY[last]);
  let i = 0;
  while (BODY[i + 1][0] > ix) i++;
  const [a, b] = [BODY[i], BODY[i + 1]];
  const f = (ix - a[0]) / (b[0] - a[0]);
  return { cy: a[1] + (b[1] - a[1]) * f, w: a[2] + (b[2] - a[2]) * f, h: a[3] + (b[3] - a[3]) * f };
}

// How far over the back (rad round the body from its top) the folded wings
// reach at a column of the side view: they leave the brown back between them,
// a V from the shoulders to the rump, as the sheet's back and top views show.
function wingTop(ix: number): number {
  return 0.6 - 0.48 * smooth((ix - 935) / 110);
}

// The colour of the body at a point of the side view (`x` px to the right):
// the chestnut breast, a shield in front of a line slanting back up from
// under it and coming to a point low in the middle of its front, the black
// rump (glossy green on top, between the wing tips) behind a line slanting
// the other way, the brown back along the top,
// and the pale flanks and belly.
function bodyTone(ix: number, iy: number, x: number, fromTop: number, r: number, k: number): Tone {
  if (ix > 1036 + 0.28 * (iy - 200)) return fromTop < 0.5 ? patchy('sheen', 'rump', r, k, 0.35) : patchy('rump', 'curl', r, k, 0.6);
  if (ix < 921 - 0.8 * Math.max(0, iy - 176) && iy < 223 - 0.45 * Math.abs(x)) return patchy('breast', 'rump', r, k, 0.22, 0.28);
  if (fromTop < wingTop(ix) + 0.1 && ix > 905) return patchy('back', 'tertial', r, k, 0.5);
  if (fromTop > 2.2) return patchy('belly', 'mottle', r, k, 0.6);
  return patchy('flank', 'mottle', r, k, 0.7, 0.35);
}

// How much slimmer the body is under the folded wings: a share of its
// radius at each of its rings (BODY's), times this.
const UNDER_WING = 0.08;
const TUCK: readonly number[] = [0, 0.4, 0.8, 1, 1, 1, 1, 0.75, 0.25];

// How far round from its top (rad) the wings lie: from the back's V down to
// the band under the speculum.
function tuckAt(fromTop: number): number {
  return smooth((fromTop - 0.15) / 0.25) * (1 - smooth((fromTop - 1.4) / 0.4));
}

function buildBody(s: Sculpt): void {
  const last = BODY.length - 1;
  // Full, a little boxy, as the sheet's front view is round and broad low down.
  const rings: Ring[] = BODY.map(([ix, cy, w, h], r) => ({ at: P(ix, cy), w: w * U, up: h * U, down: h * U, n: r < 2 || r > last - 2 ? 2 : 2.3 }));
  loft(s, rings, {
    sides: 10,
    start: 8 * U,
    end: 2 * U,
    // A little unevenness, as the sheet's facets have, and slimmer where the
    // folded wings lie on its upper sides, so body and wings together fill
    // the sheet's outline.
    shape: (r, angle, k) => (1 + 0.05 * (hash(r + 3, k) - 0.5)) * (1 - UNDER_WING * (TUCK[r] ?? 0) * tuckAt(Math.min(angle, TAU - angle))),
    paint: (r, angle, k) => {
      const a = BODY[Math.max(0, Math.min(last, r))];
      const b = BODY[Math.max(0, Math.min(last, r + 1))];
      const ix = r < 0 ? 1068 : r >= last ? 869 : (a[0] + b[0]) / 2;
      const iy = (a[1] + b[1]) / 2 - ((a[3] + b[3]) / 2) * Math.cos(angle);
      const x = ((a[2] + b[2]) / 2) * Math.sin(angle);
      return bodyTone(ix, iy, x, Math.min(angle, TAU - angle), r, k);
    },
  });
}

// ---------------------------------------------------------------- the head

// The head, from inside the neck to under the crown, traced from the sheet's
// side view (its front and back at each height) and its front view (its
// width): [height, middle column, half width, half depth], in its pixels. It
// tucks into the neck at its bottom.
const SKULL: readonly (readonly [number, number, number, number])[] = [
  [142, 911.2, 20.3, 20.4],
  [136, 910.4, 22.2, 22.6],
  [127, 908.8, 23.1, 26.2],
  [117, 905.8, 23.4, 30.2],
  [107.5, 902, 22.8, 32],
  [97.5, 900.6, 21.3, 28.3],
  [90, 901, 18, 23.8],
  [84.5, 901.5, 14.2, 18],
  [81.2, 901.5, 8.6, 10.5],
];
const CROWN = 79.8; // the top of its head

// The bill, from inside the face to its tip: [column, middle, half width,
// half height]; a flat, broad spatula, its top sloping down to the tip.
const BILL: readonly (readonly [number, number, number, number])[] = [
  [884, 119.3, 12.4, 10],
  [872, 118.6, 13.8, 12.4],
  [862, 121.8, 13.2, 10.4],
  [852, 125, 12, 8],
  [843, 128.1, 10.8, 5.6],
  [838, 129.8, 9.8, 4.3],
];
const EYE = { ix: 891.9, iy: 99.4, radius: 5.3 };
const NOSTRIL = { ix: 864, x: 8.2 };

function skullTone(iy: number, fromBack: number, r: number, k: number): Tone {
  if (iy < 88) return patchy('crown', 'face', r, k, 0.35);
  if (iy < 101 && fromBack > 0.35 && fromBack < 2.35) return tone('stripe');
  if (fromBack > 1.75 && iy > 100) return patchy('face', 'crown', r, k, 0.35);
  return patchy('crown', 'face', r, k, 0.35);
}

// The head as it stands, in the rig's space: `s` the head, `shiny` the eyes
// and the bill.
function buildHead(s: Sculpt, shiny: Sculpt): void {
  const last = SKULL.length - 1;
  loft(
    s,
    SKULL.map(([iy, ix, w, d]) => ({ at: P(ix, iy), w: w * U, up: d * U, down: d * U })),
    {
      sides: 10,
      start: 'open',
      end: (SKULL[last][0] - CROWN) * U,
      shape: (r, _angle, k) => 1 + 0.04 * (hash(r + 11, k) - 0.5),
      paint: (r, angle, k) => {
        const iy = r >= last ? CROWN + 2 : (SKULL[r][0] + SKULL[r + 1][0]) / 2;
        return skullTone(iy, Math.min(angle, TAU - angle), r, k);
      },
    },
  );
  // The bill: yellow, a little orange underneath, the nail a small dark spot
  // on its tip.
  const bill = BILL.map(([ix, cy, w, h]) => ({ at: P(ix, cy), w: w * U, up: h * U, down: h * U, n: 2.6 }));
  const tip = BILL.length - 1;
  loft(shiny, bill, {
    sides: 8,
    turn: Math.PI / 8,
    start: 'flat',
    end: 2 * U,
    paint: (r, angle) => {
      const fromTop = Math.min(angle, TAU - angle);
      if (r >= tip && fromTop < 0.5) return tone('nail');
      if (fromTop > 2.2) return blend('bill', 'leg', 0.3);
      return tone('bill', r === 0 ? 0.96 : 1);
    },
  });
  // The nostrils: dark slits on its top, near its base.
  for (const side of [-1, 1]) {
    const at = shiny.onto(P(NOSTRIL.ix, 100, side * NOSTRIL.x), new THREE.Vector3(0, -1, 0));
    const out = new THREE.Vector3(side * 0.35, 0.93, 0.1).normalize();
    at.addScaledVector(out, 0.35 * U);
    const along = new THREE.Vector3(0, -0.45, 1).normalize().multiplyScalar(3 * U);
    const across = new THREE.Vector3(side, 0.35, 0).normalize().multiplyScalar(1.1 * U);
    const [a, b, c, d] = [at.clone().add(along), at.clone().add(across), at.clone().sub(along), at.clone().sub(across)];
    shiny.toward(a, b, c, out, tone('nostril'));
    shiny.toward(a, c, d, out, tone('nostril'));
  }
  // The eyes: dark and round with a glint, in a black lid, seated on the head
  // where a ray from the side meets it, looking out and a little forward.
  for (const side of [-1, 1]) {
    const look = new THREE.Vector3(side * 0.88, 0.1, 0.46).normalize();
    const at = s.onto(P(EYE.ix, EYE.iy, side * 40), new THREE.Vector3(-side, 0, 0)).addScaledVector(look, -0.6 * U);
    eye(shiny, at, look, new THREE.Vector3(0, 1, 0), EYE.radius * U, { lid: tone('lid'), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 8, 0.52, 0.42);
  }
}

// The neck, from inside the body up into the head: [height, middle column,
// half width, half depth]. The white ring round it sits just above the breast.
const NECK: readonly (readonly [number, number, number, number])[] = [
  [178, 912, 19.5, 19.5],
  [162, 910, 20.6, 20.2],
  [154.5, 909, 21.4, 20.5],
  [146.5, 909.5, 21.3, 20.9],
  [138, 910.5, 20.9, 21.4],
  [128, 909.5, 19.6, 21.5],
  [116, 906.5, 17.5, 21],
];

function buildNeck(s: Sculpt): void {
  loft(
    s,
    NECK.map(([iy, ix, w, d]) => ({ at: P(ix, iy), w: w * U, up: d * U, down: d * U })),
    {
      sides: 10,
      start: 'flat',
      end: 'flat',
      paint: (r, angle, k) => {
        const front = Math.min(angle, TAU - angle) > Math.PI / 2;
        if (r <= 1) return tone(front ? 'breast' : 'back');
        if (r === 2) return tone('collar', 1 + 0.03 * (hash(k, 5) - 0.5));
        return front ? patchy('face', 'neck', r, k, 0.4) : patchy('neck', 'face', r, k, 0.35);
      },
    },
  );
}

// ---------------------------------------------------------------- the tail

// A feather of the tail, flat with two sides: from `root` along `along`,
// `across` wide, rising `curl` (times the square of the way) toward its tip,
// its rows [share of the length, share of the half width] as in a wing's
// blades.
function plume(s: Sculpt, root: V3, along: V3, across: V3, length: number, width: number, curl: number, rows: readonly (readonly [number, number])[], top: Tone, under: Tone): void {
  const a = along.clone().normalize();
  const c = across.clone().normalize();
  const n = new THREE.Vector3().crossVectors(c, a).normalize();
  const thick = 0.5 * U;
  const at = (u: number, v: number, layer: number) =>
    root
      .clone()
      .addScaledVector(a, u * length)
      .addScaledVector(c, (v * width) / 2)
      .addScaledVector(n, curl * u * u + layer);
  for (const layer of [thick, -thick]) {
    const face = layer > 0 ? n : n.clone().negate();
    const t = layer > 0 ? top : under;
    for (let i = 0; i + 1 < rows.length; i++) {
      const [u0, h0] = rows[i];
      const [u1, h1] = rows[i + 1];
      const [p0, q0] = [at(u0, h0, layer), at(u0, -h0, layer)];
      const [p1, q1] = [at(u1, h1, layer), at(u1, -h1, layer)];
      s.toward(p0, q0, q1, face, t);
      if (h1 > 0) s.toward(p0, q1, p1, face, t);
    }
  }
}

// The tail as it stands, in the rig's space: a fan of white feathers from
// under the rump, rising toward the back and arching down to their tips, the
// outer ones rising more (a white smile under the black rump, from behind),
// and the drake's black curl feathers over its root, the middle one glossy
// green and curling up.
function buildTail(s: Sculpt): void {
  const root = P(1048, 217);
  // [angle out from straight back (rad, > 0 to its right), length (px), how
  // far it is raised (px), how far its outer edge is rolled up (rad, < 0
  // down)].
  for (const [angle, length, rise, roll] of [
    [-0.42, 42, 12, -0.35],
    [-0.21, 46, 5, -0.15],
    [0, 47, 0, 0],
    [0.21, 46, 5, -0.15],
    [0.42, 42, 12, -0.35],
  ]) {
    const along = new THREE.Vector3(Math.sin(angle), 0.22 + rise * 0.014, -Math.cos(angle));
    // Their outer edges rolled a little down, their white tops showing from
    // the side.
    const across = new THREE.Vector3(Math.cos(angle) * Math.cos(roll), Math.sign(angle) * Math.sin(roll), Math.sin(angle) * Math.cos(roll));
    plume(s, root.clone().add(new THREE.Vector3(0, rise * 0.2 * U, 0)), along, across, length * U, 19 * U, -9 * U, FEATHER, tone('tail', 1 + 0.02 * angle), tone('tail', 0.86));
  }
  for (const [x, length, curl, width] of [
    [-7, 30, 2, 10],
    [7, 30, 2, 10],
    [0, 34, 11, 9],
  ]) {
    const start = P(1036, 205 + Math.abs(x) * 0.2, x);
    const along = new THREE.Vector3(x * 0.004, -0.02, -1);
    const top = x === 0 ? blend('curl', 'sheen', 0.6) : patchy('curl', 'rump', x, length, 0.5);
    plume(s, start, along, new THREE.Vector3(1, 0, 0), length * U, width * U, curl * U, FEATHER, top, tone('curl', 0.8));
  }
}

// ---------------------------------------------------------------- the wings

// A feather or a panel of feathers of the wing, lying on the body when the
// wing is folded (as the sheet's side view shows it) and in the wing's plane
// when it is spread; the geometry is the folded wing and the spread one its
// morph target, corner for corner.
interface Blade {
  // Folded, on the body's side: its root and tip on the side view (image
  // pixels), its width, and how far off the body it lies (px): higher ones
  // lie over lower ones. A stiff one (the primaries) runs straight from where
  // its root lies on the body to `tipX` px from the middle at its tip, lying
  // on the body only where the body bulges out further. One at the folded
  // wing's front edge keeps only `tuck` of its lift at its root, all of it a
  // third of the way along, so the front edge tucks into the body.
  fold: { root: readonly [number, number]; tip: readonly [number, number]; width: number; lift: number; tipX?: number; tuck?: number };
  // Spread, in the wing's plane (px): its root `s` out from the shoulder and
  // `c` forward, the way it points (degrees from straight out, < 0 back), its
  // length and width, and its height over the plane.
  open: { root: readonly [number, number]; angle: number; length: number; width: number; height: number };
  // Its outline: [share of the length, share of the half width] root to tip.
  rows: readonly (readonly [number, number])[];
  // Strips across it: one or two, so the folded wing is a few large facets.
  strips: number;
  // The role of each stretch between rows, on top (outside, folded) and under.
  top: readonly string[];
  under: readonly string[];
  shade?: number;
  // Its top toward another colour at its root, fading to none at its tip:
  // [role, how far].
  from?: readonly [string, number];
}

const FEATHER = [
  [0, 0.55],
  [0.3, 0.9],
  [0.6, 1],
  [0.84, 0.7],
  [1, 0],
] as const;
const SECONDARY = [
  [0, 0.75],
  [0.55, 1],
  [0.92, 0.95],
  [1, 0.6],
] as const;
const PANEL = [
  [0, 1],
  [0.3, 1],
  [0.6, 0.95],
  [0.85, 0.75],
  [1, 0.3],
] as const;
const TERTIAL = [
  [0, 0.65],
  [0.25, 0.95],
  [0.5, 1],
  [0.78, 0.75],
  [1, 0],
] as const;

// Folded, each layer lies further off the body than the one under it by more
// than its flat facets sink across the body's curve, so none shows through
// another.
const BLADES: readonly Blade[] = [
  // The primaries, dark: under everything, folded straight back from under
  // the tertials to their tips over the rump, the long outer feathers of the
  // hand when spread.
  { fold: { root: [1034, 195], tip: [1097, 198], width: 12, lift: 0.4, tipX: 3 }, open: { root: [128, -2], angle: -8, length: 62, width: 17, height: 0.3 }, rows: FEATHER, strips: 1, top: ['primary'], under: ['underTip'] },
  { fold: { root: [1034, 198.5], tip: [1090, 202], width: 12, lift: 0.6, tipX: 4.5 }, open: { root: [117, -4], angle: -24, length: 64, width: 17, height: 0.45 }, rows: FEATHER, strips: 1, top: ['primary'], under: ['underTip'] },
  { fold: { root: [1034, 202], tip: [1083, 206], width: 12, lift: 0.8, tipX: 6 }, open: { root: [106, -6], angle: -40, length: 64, width: 17, height: 0.6 }, rows: FEATHER, strips: 1, top: ['primary'], under: ['underTip'] },
  { fold: { root: [1034, 205.5], tip: [1075, 209.5], width: 12, lift: 1, tipX: 7.5 }, open: { root: [95, -8], angle: -56, length: 62, width: 17, height: 0.75 }, rows: FEATHER, strips: 1, top: ['primary'], under: ['underTip'] },
  // The secondaries: brown, then the blue speculum, then a white edge. Folded
  // they lie one on another, so under the brown band one clean blue patch
  // shows, edged in white; spread, the speculum is a band across them.
  { fold: { root: [964, 205.5], tip: [1022, 223.5], width: 10, lift: 2 }, open: { root: [22, -8], angle: -92, length: 64, width: 25, height: 0.9 }, rows: SECONDARY, strips: 1, top: ['tertial', 'speculum', 'edge'], under: ['under', 'under', 'underTip'] },
  { fold: { root: [964, 205.5], tip: [1022, 223.5], width: 10, lift: 2.1 }, open: { root: [44, -8], angle: -95, length: 64, width: 25, height: 1 }, rows: SECONDARY, strips: 1, top: ['tertial', 'speculum', 'edge'], under: ['under', 'under', 'underTip'] },
  { fold: { root: [964, 205.5], tip: [1022, 223.5], width: 10, lift: 2.2 }, open: { root: [66, -6], angle: -98, length: 62, width: 25, height: 1.1 }, rows: SECONDARY, strips: 1, top: ['tertial', 'speculum', 'edge'], under: ['under', 'under', 'underTip'] },
  { fold: { root: [964, 205.5], tip: [1022, 223.5], width: 10, lift: 2.3 }, open: { root: [86, -6], angle: -102, length: 60, width: 23, height: 1.2 }, rows: SECONDARY, strips: 1, top: ['tertial', 'speculum', 'edge'], under: ['under', 'under', 'underTip'] },
  // The brown band along the folded wing's lower edge, back to the rump.
  { fold: { root: [950, 197], tip: [1040, 208], width: 20, lift: 3.2 }, open: { root: [12, -4], angle: -97, length: 64, width: 21, height: 1.4 }, rows: PANEL, strips: 2, top: ['tertial'], under: ['under'] },
  // The coverts: the front of the folded wing, cream at the front turning tan
  // toward the back; the spread wing's leading edge.
  { fold: { root: [933, 191], tip: [998, 199], width: 17, lift: 4, tuck: 0.4 }, open: { root: [8, -8], angle: -6, length: 102, width: 18, height: 1.9 }, rows: PANEL, strips: 2, top: ['coverts'], under: ['under'], shade: 0.9 },
  { fold: { root: [927, 180], tip: [1006, 180], width: 36, lift: 6.9, tuck: 0.55 }, open: { root: [0, 6], angle: -3, length: 120, width: 26, height: 2.2 }, rows: PANEL, strips: 2, top: ['coverts'], under: ['under'], from: ['pale', 0.45] },
  // The pale tertial along the top.
  { fold: { root: [960, 161], tip: [1054, 194], width: 30, lift: 9.8 }, open: { root: [4, -2], angle: -103, length: 62, width: 18, height: 2.4 }, rows: TERTIAL, strips: 2, top: ['pale'], under: ['under'] },
];

const SHOULDER = { ix: 932, iy: 165, x: 36 }; // where a wing joins the body
const WRIST = { s: 78, c: 8 }; // the spread wing's wrist, in its plane (px)
const DIHEDRAL = 0.14; // rad the spread wings rise from level
const BLADE_THICK = 0.12; // px from a blade's middle to either face
const SQUASH = 0.14; // rad over which a folded wing's upper edge is squeezed toward wingTop()

// A point on the folded wing: on the body's surface where a ray from the side
// meets it at this point of the side view, `lift` px off it; beyond the body,
// `beyond` px from the middle. Its angle round the body from the top (above
// the body there, further round the higher it is) is squeezed so the wing
// reaches over the back no further than wingTop(), in order, so no two of
// its corners meet.
function onBody(body: Sculpt, ix: number, iy: number, side: 1 | -1, lift: number, beyond: number): V3 {
  const { cy, h } = bodyAt(ix);
  const axis = P(ix, cy);
  const from = P(ix, iy, side * 200);
  const way = new THREE.Vector3(-side, 0, 0);
  const hit = body.cast(from, way);
  let angle: number;
  if (Number.isFinite(hit)) {
    const at = from.addScaledVector(way, hit);
    angle = Math.atan2(Math.abs(at.x - axis.x), at.y - axis.y);
  } else if (ix < BODY[0][0]) angle = iy < cy ? -(cy - h - iy) / h : Math.PI - 0.12;
  else return P(ix, iy, side * (beyond + lift));
  const top = wingTop(ix) + SQUASH;
  if (angle < top) angle = top - SQUASH + SQUASH * Math.exp((angle - top) / SQUASH);
  const round = new THREE.Vector3(side * Math.sin(angle), Math.cos(angle), 0);
  const t = body.cast(axis, round);
  if (!Number.isFinite(t)) return P(ix, iy, side * (beyond + lift));
  const at = axis.clone().addScaledVector(round, t);
  const out = new THREE.Vector3(at.x - axis.x, at.y - axis.y, 0);
  if (out.lengthSq() < 1e-12) out.set(side, 0, 0);
  return at.addScaledVector(out.normalize(), lift * U);
}

// A wing, folded on the body (the geometry), and spread (returned: its morph
// target's positions), each corner weighed to the shoulder or, out past the
// wrist, to the wrist.
function buildWing(s: Sculpt, body: Sculpt, side: 1 | -1, shoulder: V3, bones: readonly [number, number]): number[] {
  const spread: number[] = [];
  const out = new THREE.Vector3(side * Math.cos(DIHEDRAL), Math.sin(DIHEDRAL), 0);
  const up = new THREE.Vector3(-side * Math.sin(DIHEDRAL), Math.cos(DIHEDRAL), 0);
  const fwd = new THREE.Vector3(0, 0, 1);
  const weigh = (sPx: number): Weights => {
    const f = smooth((sPx - (WRIST.s - 18)) / 34);
    return f <= 0 ? [bones[0], bones[0], 0] : f >= 1 ? [bones[1], bones[1], 0] : [bones[0], bones[1], f];
  };
  interface Corner {
    folded: V3;
    opened: V3;
    w: Weights;
  }
  for (const blade of BLADES) {
    const { fold, open, rows, strips } = blade;
    // Folded: along the side view from its root to its tip.
    const fl = Math.hypot(fold.tip[0] - fold.root[0], fold.tip[1] - fold.root[1]);
    const e = [(fold.tip[0] - fold.root[0]) / fl, (fold.tip[1] - fold.root[1]) / fl];
    // Spread: out from its root the way it points.
    const a = THREE.MathUtils.degToRad(open.angle);
    const d = [Math.cos(a), Math.sin(a)];
    // A stiff one's root, px from the middle.
    const rootX = fold.tipX === undefined ? 0 : Math.abs(onBody(body, fold.root[0], fold.root[1], side, 0, 0).x) / U;
    const corner = (u: number, v: number, layer: number): Corner => {
      const ix = fold.root[0] + e[0] * u * fl + e[1] * v * (fold.width / 2);
      const iy = fold.root[1] + e[1] * u * fl - e[0] * v * (fold.width / 2);
      const lift = fold.lift * (fold.tuck === undefined ? 1 : fold.tuck + (1 - fold.tuck) * smooth(u / 0.35));
      let folded = onBody(body, ix, iy, side, lift + layer, 3 + fold.lift);
      if (fold.tipX !== undefined) {
        const x = rootX + (fold.tipX - rootX) * u + fold.lift + layer;
        if (x > Math.abs(folded.x) / U) folded = P(ix, iy, side * x);
      }
      const sPx = open.root[0] + d[0] * u * open.length + d[1] * v * (open.width / 2);
      const cPx = open.root[1] + d[1] * u * open.length - d[0] * v * (open.width / 2);
      const opened = shoulder
        .clone()
        .addScaledVector(out, sPx * U)
        .addScaledVector(fwd, cPx * U)
        .addScaledVector(up, (open.height + layer) * U);
      return { folded, opened, w: weigh(sPx) };
    };
    for (const layer of [BLADE_THICK, -BLADE_THICK]) {
      const topSide = layer > 0;
      // Each row's corners across the blade, from its +v edge to its -v one.
      const grid = rows.map(([u, h]) => (h > 0 ? Array.from({ length: strips + 1 }, (_, j) => corner(u, h * (1 - (2 * j) / strips), layer)) : [corner(u, 0, layer)]));
      for (let i = 0; i + 1 < grid.length; i++) {
        const roles = topSide ? blade.top : blade.under;
        const role = roles[Math.min(i, roles.length - 1)];
        const shade = (blade.shade ?? 1) * (topSide ? 1 : 0.97);
        const fade = blade.from ? blade.from[1] * (1 - i / (grid.length - 2)) : 0;
        const t = topSide && blade.from && fade > 0 ? blend(role, blade.from[0], fade, shade) : tone(role, shade);
        const [row, next] = [grid[i], grid[i + 1]];
        const faces: Corner[][] = [];
        for (let j = 0; j < strips; j++) {
          // Counter-clockwise in the blade's own (along, across).
          if (next.length === 1) faces.push([row[j], row[j + 1], next[0]]);
          else faces.push([row[j], row[j + 1], next[j + 1]], [row[j], next[j + 1], next[j]]);
        }
        for (const face of faces) {
          // That faces the top of the right wing; the left one is its mirror
          // image, and the undersides face the other way.
          const [p, q, r] = (side > 0) === topSide ? face : [face[0], face[2], face[1]];
          const before = s.triangles;
          s.triangle(p.folded, q.folded, r.folded, t, [p.w, q.w, r.w]);
          if (s.triangles > before) for (const c of [p, q, r]) spread.push(c.opened.x, c.opened.y, c.opened.z);
        }
      }
    }
  }
  return spread;
}

// ---------------------------------------------------------------- the legs

const LEG = { x: 26, hip: [966, 232], heel: [952, 260], ankle: [962, 291] } as const;
// The toes, from the inside out: [angle out from straight ahead (rad, for the
// right foot), length (px)].
const TOES: readonly (readonly [number, number])[] = [
  [-0.5, 38],
  [0, 46],
  [0.5, 40],
];
const TOE_OUT = 0.12; // rad each foot turns out

// A leg as it stands, in the rig's space, its bones' skeleton indices: the
// hip's (in the body), the heel's, and the foot's (at the ankle).
function buildLeg(s: Sculpt, side: 1 | -1, [bh, bk, bf]: readonly [number, number, number]): void {
  const x = side * LEG.x;
  const hip = P(LEG.hip[0], LEG.hip[1], x);
  const heel = P(LEG.heel[0], LEG.heel[1], x);
  const ankle = P(LEG.ankle[0], LEG.ankle[1], x);
  const pad = P(LEG.ankle[0] - 1, 297.5, x);
  const hipW: Weights = [bh, bh, 0];
  const heelW: Weights = [bk, bk, 0];
  const footW: Weights = [bf, bf, 0];
  const rings: Ring[] = [
    { at: hip.clone().lerp(heel, -0.3), w: 8.5 * U, up: 9 * U, bone: hipW },
    { at: hip.clone().lerp(heel, 0.45), w: 8.5 * U, up: 9 * U, bone: hipW },
    { at: heel.clone(), w: 8.4 * U, up: 8.8 * U, bone: [bh, bk, 0.5] },
    { at: heel.clone().lerp(ankle, 0.5), w: 7 * U, up: 7.4 * U, bone: heelW },
    { at: heel.clone().lerp(ankle, 0.88), w: 7.2 * U, up: 7.6 * U, bone: [bk, bf, 0.5] },
    { at: ankle.clone(), w: 8 * U, up: 8.6 * U, bone: footW },
    { at: pad, w: 8.6 * U, up: 9.4 * U, bone: footW },
  ];
  loft(s, rings, {
    sides: 6,
    side: new THREE.Vector3(side, 0, 0),
    mirror: side < 0,
    start: 'flat',
    end: 'flat',
    paint: (_r, angle) => tone('leg', Math.abs(angle - Math.PI) < 1 ? 0.88 : 1),
  });
  s.weights = footW;
  // The webbed foot on the ground: three toes, ridged, the webs between them,
  // dark claws, and a little hind toe.
  const base = new THREE.Vector3(ankle.x, 0, ankle.z + 1 * U);
  const lines = TOES.map(([angle, length]) => {
    const a = side * angle + side * TOE_OUT;
    return { dir: new THREE.Vector3(Math.sin(a), 0, Math.cos(a)), length };
  });
  const along = (i: number, share: number, y: number) =>
    base
      .clone()
      .addScaledVector(lines[i].dir, (3 + (lines[i].length - 3) * share) * U)
      .setY(y * U);
  lines.forEach(({ dir, length }, i) => {
    const at = (share: number, y: number) =>
      base
        .clone()
        .addScaledVector(dir, (2 + (length - 2) * share) * U)
        .setY(y * U);
    const toe: Ring[] = [
      { at: at(0, 3.2), w: 3.9 * U, up: 2.6 * U, down: 3 * U },
      { at: at(0.55, 2.2), w: 2.5 * U, up: 2.1 * U, down: 2 * U },
      { at: at(0.86, 1.5), w: 1.8 * U, up: 1.4 * U, down: 1.3 * U },
      { at: at(1, 1.1), w: 1.6 * U, up: 1 * U, down: 0.9 * U },
    ];
    loft(s, toe, {
      sides: 4,
      side: new THREE.Vector3(dir.z, 0, -dir.x),
      start: 'flat',
      end: 1.8 * U,
      paint: (r, angle) => (r >= 2 ? tone('claw') : tone('leg', Math.min(angle, TAU - angle) < 1 ? 1.03 : 0.94 + 0.02 * i)),
    });
  });
  // The webs, between the toes a little below their ridges, out nearly to
  // their tips, their front edges curving back a little between them.
  for (const [i, j] of [
    [0, 1],
    [1, 2],
  ]) {
    for (const [y, faceUp] of [
      [1.6, true],
      [0.9, false],
    ] as const) {
      const root = base.clone().addScaledVector(lines[1].dir, 3 * U).setY(y * U);
      const tipI = along(i, 0.93, y);
      const tipJ = along(j, 0.93, y);
      const front = tipI.clone().lerp(tipJ, 0.5).lerp(root, 0.1);
      const outline = [along(i, 0.35, y), tipI, front, tipJ, along(j, 0.35, y)];
      const face = new THREE.Vector3(0, faceUp ? 1 : -1, 0);
      const t = tone('leg', faceUp ? 0.97 : 0.8);
      for (let k = 0; k + 1 < outline.length; k++) s.toward(root, outline[k], outline[k + 1], face, t);
    }
  }
  // The hind toe, a small spur behind the ankle.
  const back = new THREE.Vector3(side * -0.3, 0, -1).normalize();
  loft(
    s,
    [
      { at: base.clone().addScaledVector(back, 2 * U).setY(4.6 * U), w: 1.8 * U, up: 1.6 * U },
      { at: base.clone().addScaledVector(back, 7 * U).setY(3 * U), w: 1.3 * U, up: 1.1 * U },
    ],
    { sides: 4, side: new THREE.Vector3(1, 0, 0), start: 'flat', end: 1.8 * U, paint: (r) => (r >= 1 ? tone('claw') : tone('leg', 0.92)) },
  );
}

// ---------------------------------------------------------------- motion

// A way of going on land.
interface Gait {
  stride: number; // m a planted foot travels back under the body
  cadence: number; // strides a second
  duty: number; // share of a stride each foot is down
  lift: number; // m a foot rises as it swings
  curl: number; // rad its toes droop as it swings
  roll: number; // rad the body rocks onto the planted foot (the waddle)
  sway: number; // m it shifts over it
  yaw: number; // rad it swings its front toward the stepping foot
  bob: number; // m it rises over each planted foot
}

const WALK: Gait = { stride: 0.075, cadence: 2.3, duty: 0.6, lift: 0.03, curl: 0.3, roll: 0.1, sway: 0.007, yaw: 0.06, bob: 0.005 };
const RUN: Gait = { stride: 0.13, cadence: 3.6, duty: 0.42, lift: 0.04, curl: 0.36, roll: 0.06, sway: 0.004, yaw: 0.04, bob: 0.009 };

const SWIM_SPEED = 0.42; // m/s swimming
const DASH_SPEED = 0.85; // m/s told to run on the water: it paddles hard
const SWIM_DEPTH = 0.15; // m its origin sits under the surface afloat, its waterline a third up its body
const STROKES = 1.7; // paddle strokes a second, swimming
const FLAP_RATE = 3.8; // wing beats a second, flapping
const FLY_HEIGHT = 0.9; // m it climbs to, taking off
const FLY_SPEED = 2.3; // m/s it flies at
// FlapWings(): rising, beating, folding, done (seconds from its start).
const FLAP = { rise: 0.35, beats: 1.95, fold: 2.35, end: 2.8 };
// TakeOff(): the run up, the climb, level flight, the glide down, the flare
// to land, done (seconds from its start).
const TAKEOFF = { run: 0.9, climb: 2.2, level: 2.9, glide: 4.5, flare: 5.1, end: 5.9 };
const GAIT_RATE = 3.5; // how fast it starts, stops and changes gait, a second
const SETTLE = 0.25; // s a foot still in the air takes to land once it stops
const REACH_SHARE = 0.998; // of a leg's length it stretches at most
const MOST_SLACK = 0.04; // m the body is lowered at most, for its legs to reach
// How far the foot's corners reach from the ankle, as it stands (m): none is
// further ahead of it (the middle toe's claw) or further under it (the soles).
// A corner that far ahead and that far under it drops the most as the toes
// droop, so its toes may droop only as far as keeps that corner off the
// ground.
const TOE_REACH = 50 * U;
const TOE_DROP = 12 * U;

// How far the toes may droop (rad) with the ankle this high over the ground.
function droopWithin(height: number): number {
  const r = Math.hypot(TOE_REACH, TOE_DROP);
  return height >= r ? Math.PI / 2 : Math.asin(Math.max(0, height) / r) - Math.atan2(TOE_DROP, TOE_REACH);
}

// What each pose eases toward.
interface Posture {
  pitch: number; // rad the body tips from as built (> 0 its front down)
  rise: number; // m the body is held higher
  neck: number; // rad the neck leans forward at its root
  neckTop: number; // rad its upper half bends forward
  head: number; // rad the head's own pitch in the world (> 0 its bill down)
  headTurn: number; // rad it looks to its right (> 0)
  tail: number; // rad the tail lifts (> 0 up)
  tailTurn: number; // rad it swings to its right (> 0)
  spread: number; // 0 wings folded, 1 spread
  beat: number; // rad the spread wings beat either way
  raise: number; // rad they are held up at the middle of a beat
  sweep: number; // rad they sweep back
  twist: number; // rad they turn their undersides forward (flapping standing, braking)
  float: number; // 0..1: how deep it sits in the water (1 swimming)
}

const REST: Readonly<Posture> = { pitch: 0, rise: 0, neck: 0, neckTop: 0, head: 0, headTurn: 0, tail: 0, tailTurn: 0, spread: 0, beat: 0, raise: 0, sweep: 0, twist: 0, float: 0 };
const KEYS = Object.keys(REST) as (keyof Posture)[];
const RATES: Partial<Record<keyof Posture, number>> = { spread: 9, beat: 8, float: 3, pitch: 5 };

type Mode = 'idle' | 'walk' | 'run' | 'swim' | 'flap' | 'takeoff';
// How its feet go when they are not on the ground: paddling under the water,
// running over it, hanging in flight, or reaching forward to land.
type Feet = 'paddle' | 'patter' | 'dangle' | 'reach';

interface LegRig {
  side: 1 | -1;
  offset: number; // where it is in the stride
  hip: THREE.Bone;
  heel: THREE.Bone;
  foot: THREE.Bone;
  upper: number; // m, hip to heel
  lower: number; // m, heel to ankle
  angles: readonly [number, number]; // the two bones' rest angles from straight down
  rest: V3; // where its ankle stands, in the rig's space
  restInTorso: V3; // and in the body's space
  down: boolean;
  plant: V3; // where its planted ankle is, in the space it moves in
  from: V3; // where it lifted off, in the rig's space
  q: number; // how far through its swing
  target: V3; // where its ankle is this frame, in the rig's space
  pitch: number; // its foot's pitch in the world (> 0 toes down)
  web: number; // how far its web is spread (1 flat)
}

interface WingRig {
  side: 1 | -1;
  mesh: THREE.SkinnedMesh;
  shoulder: THREE.Bone;
  wrist: THREE.Bone;
}

const v1 = new THREE.Vector3();
const v2 = new THREE.Vector3();
const m1 = new THREE.Matrix4();
const m2 = new THREE.Matrix4();
const q1 = new THREE.Quaternion();
const q2 = new THREE.Quaternion();
const e1 = new THREE.Euler();

export class Duck extends ForestAnimal {
  // The sheet's colour variations, the default (mallard) first.
  static readonly COLORS = Object.keys(VARIATIONS);
  static readonly WALK_SPEED = (WALK.stride * WALK.cadence) / WALK.duty;
  static readonly RUN_SPEED = (RUN.stride * RUN.cadence) / RUN.duty;
  static readonly SWIM_SPEED = SWIM_SPEED;
  static readonly SWIM_DEPTH = SWIM_DEPTH;
  static readonly FLY_HEIGHT = FLY_HEIGHT;

  readonly head = new THREE.Group();
  readonly neck: THREE.SkinnedMesh;
  readonly body: THREE.Mesh;
  readonly leftWing: THREE.SkinnedMesh;
  readonly rightWing: THREE.SkinnedMesh;
  readonly tail = new THREE.Group();
  readonly leftLeg: THREE.SkinnedMesh;
  readonly rightLeg: THREE.SkinnedMesh;

  // Whether it is in the water: Swim() puts it there, and a preview on the
  // lake does. Afloat, Walk() and Run() swim.
  afloat = false;
  // The water's surface under it, in its parent's y (0 when still): a
  // preview follows the waves with it.
  surface = 0;
  // Called where it breaks the water's surface (running over it, lifting off
  // it, landing on it, beating its wings on it), with its velocity there, in
  // its parent's space: a preview hands it to the water.
  onSplash: ((x: number, z: number, velocity: THREE.Vector3) => void) | null = null;

  private readonly torso: THREE.Bone;
  private readonly torsoRest: V3;
  private readonly neckBones: readonly [THREE.Bone, THREE.Bone, THREE.Bone];
  private readonly tailBone: THREE.Bone;
  private readonly wings: readonly WingRig[];
  private readonly legs: readonly LegRig[];
  private readonly posture: Posture = { ...REST };
  private readonly goal: Posture = { ...REST };
  private mode: Mode = 'idle';
  private modeTime = 0;
  private time = 0;
  private moving = 0; // 0 standing, 1 going; eased
  private running = 0; // 0 walking, 1 running; eased
  private phase = 0; // round a stride
  private stroke = 0; // round a paddle stroke
  private beatPhase = 0; // rad round a wing beat (0 at the top)
  private beatRate = 0; // beats a second, eased
  private altitude = 0; // m it flies over the ground or the water
  private footing: 'ground' | 'free' = 'ground';
  private lastYaw = 0;
  private bank = 0;
  private started = false;
  private splashes = 0; // how many of this one-shot's splashes it has made
  private steps = 0; // how many settling steps it has taken, landed

  constructor(options: ForestAnimalOptions = {}) {
    super(VARIATIONS, options);
    const random = seededRandom(options.seed ?? 11);
    const palette = this.palette;
    const material = coat();
    const shine = gloss(0.3);

    // The bones, as it stands.
    this.torsoRest = P(966, 232);
    const torso = bone('torso', this.rig, this.torsoRest);
    const neckAt = [P(910, 170), P(910, 146), P(909, 126)];
    const neck0 = bone('neck0', torso, neckAt[0], this.torsoRest);
    const neck1 = bone('neck1', neck0, neckAt[1], neckAt[0]);
    const headBone = bone('head', neck1, neckAt[2], neckAt[1]);
    const tailAt = P(1052, 214);
    const tailBone = bone('tail', torso, tailAt, this.torsoRest);
    const shoulderAt = [-1, 1].map((side) => P(SHOULDER.ix, SHOULDER.iy, side * SHOULDER.x));
    const wingBones = ([-1, 1] as const).map((side, i) => {
      const shoulder = bone(`${side < 0 ? 'left' : 'right'}Shoulder`, torso, shoulderAt[i], this.torsoRest);
      const wristAt = shoulderAt[i]
        .clone()
        .add(new THREE.Vector3(side * Math.cos(DIHEDRAL), Math.sin(DIHEDRAL), 0).multiplyScalar(WRIST.s * U))
        .add(new THREE.Vector3(0, 0, WRIST.c * U));
      const wrist = bone(`${side < 0 ? 'left' : 'right'}Wrist`, shoulder, wristAt, shoulderAt[i]);
      shoulder.rotation.order = 'ZYX';
      wrist.rotation.order = 'ZYX';
      return { shoulder, wrist };
    });
    const legBones = ([-1, 1] as const).map((side) => {
      const name = side < 0 ? 'left' : 'right';
      const [h, k, a] = [LEG.hip, LEG.heel, LEG.ankle].map(([ix, iy]) => P(ix, iy, side * LEG.x));
      const hip = bone(`${name}Hip`, torso, h, this.torsoRest);
      const heel = bone(`${name}Heel`, hip, k, h);
      const foot = bone(`${name}Foot`, heel, a, k);
      hip.rotation.order = 'ZYX';
      return { hip, heel, foot, joints: [h, k, a] as const };
    });
    torso.rotation.order = 'YXZ';
    for (const b of [neck0, neck1, headBone, tailBone]) b.rotation.order = 'YXZ';
    this.torso = torso;
    this.neckBones = [neck0, neck1, headBone];
    this.tailBone = tailBone;

    const all: THREE.Bone[] = [torso, neck0, neck1, headBone, tailBone, ...wingBones.flatMap((w) => [w.shoulder, w.wrist]), ...legBones.flatMap((l) => [l.hip, l.heel, l.foot])];
    this.updateMatrixWorld(true);
    const skeleton = new THREE.Skeleton(all);
    const indexOf = (b: THREE.Bone) => all.indexOf(b);
    const into = (at: V3) => new THREE.Matrix4().makeTranslation(-at.x, -at.y, -at.z);

    // The body, rigid on its bone.
    const bodySculpt = new Sculpt(random);
    buildBody(bodySculpt);
    this.body = this.painting(rigidMesh(bodySculpt.geometry(palette).applyMatrix4(into(this.torsoRest)), material));
    torso.add(this.body);

    // The head, rigid on its bone, and the neck bending from the body to it.
    const headSculpt = new Sculpt(random);
    const shinySculpt = new Sculpt(random, 0.01);
    buildHead(headSculpt, shinySculpt);
    this.head.add(this.painting(rigidMesh(headSculpt.geometry(palette).applyMatrix4(into(neckAt[2])), material)));
    this.head.add(this.painting(rigidMesh(shinySculpt.geometry(palette).applyMatrix4(into(neckAt[2])), shine)));
    headBone.add(this.head);
    const neckSculpt = new Sculpt(random);
    buildNeck(neckSculpt);
    const neckGeometry = neckSculpt.geometry(palette);
    weighAlong(neckGeometry, [...neckAt, P(907, 100)], [neck0, neck1, headBone, headBone].map(indexOf), 'linear');
    this.neck = this.painting(skinnedMesh(neckGeometry, material, skeleton));

    // The tail, rigid on its bone.
    const tailSculpt = new Sculpt(random);
    buildTail(tailSculpt);
    this.tail.add(this.painting(rigidMesh(tailSculpt.geometry(palette).applyMatrix4(into(tailAt)), material)));
    tailBone.add(this.tail);

    // The wings: folded on the body, spread as their morph target.
    const wingMeshes = ([-1, 1] as const).map((side, i) => {
      const s = new Sculpt(random, 0.015);
      const { shoulder, wrist } = wingBones[i];
      const spread = buildWing(s, bodySculpt, side, shoulderAt[i], [indexOf(shoulder), indexOf(wrist)]);
      const geometry = s.geometry(palette, true);
      geometry.morphAttributes.position = [new THREE.Float32BufferAttribute(spread, 3)];
      const mesh = skinnedMesh(geometry, material, skeleton);
      mesh.updateMorphTargets();
      return this.painting(mesh);
    });
    [this.leftWing, this.rightWing] = wingMeshes;
    this.wings = ([-1, 1] as const).map((side, i) => ({ side, mesh: wingMeshes[i], ...wingBones[i] }));

    // The legs.
    const legMeshes = ([-1, 1] as const).map((side, i) => {
      const s = new Sculpt(random);
      const { hip, heel, foot } = legBones[i];
      buildLeg(s, side, [indexOf(hip), indexOf(heel), indexOf(foot)]);
      return this.painting(skinnedMesh(s.geometry(palette, true), material, skeleton));
    });
    [this.leftLeg, this.rightLeg] = legMeshes;
    this.legs = ([-1, 1] as const).map((side, i) => {
      const { hip, heel, foot, joints } = legBones[i];
      const [h, k, a] = joints;
      return {
        side,
        offset: side < 0 ? 0 : 0.5,
        hip,
        heel,
        foot,
        upper: h.distanceTo(k),
        lower: k.distanceTo(a),
        angles: [downAngle(k.y - h.y, k.z - h.z), downAngle(a.y - k.y, a.z - k.z)],
        rest: a.clone(),
        restInTorso: a.clone().sub(this.torsoRest),
        down: true,
        plant: new THREE.Vector3(),
        from: a.clone(),
        q: 1,
        target: a.clone(),
        pitch: 0,
        web: 1,
      } satisfies LegRig;
    });

    this.rig.add(this.neck, ...wingMeshes, ...legMeshes);
  }

  // Stands, or floats, still: breathing, looking about now and then, and
  // now and then a quick wag of its tail; afloat, its feet paddle slowly.
  // Named as in its sheet's poses.
  Idle(): void {
    this.setMode('idle');
  }

  // Waddles forward the way it faces until told otherwise: its body rocking
  // onto each planted foot, its tail wagging, its head bobbing. Afloat, it
  // swims. Named as in its sheet's poses.
  Walk(): void {
    this.setMode(this.afloat ? 'swim' : 'walk');
  }

  // Runs forward, lower and leaning ahead, its neck stretched out, until told
  // otherwise. Afloat, it paddles hard. Named as in its sheet's poses.
  Run(): void {
    this.setMode('run');
  }

  // Floats on the water's surface (y = 0 of its parent, or `surface`), low
  // in it, and swims forward the way it faces, paddling, until told
  // otherwise. Named as in its sheet's poses.
  Swim(): void {
    this.afloat = true;
    this.setMode('swim');
  }

  // Rears up on its tail, beats its wings half a dozen times, folds them and
  // settles again (about 2.8 s), then idles; afloat, it rears up out of the
  // water, its wings splashing it. Named as in its sheet's poses.
  FlapWings(): void {
    this.setMode('flap');
  }

  // Runs a few steps (over the water, afloat) beating its wings, lifts off,
  // flies up and on a short way, glides back down and lands, folds its wings
  // and idles (about 6 s, some 8 m on). Named as in its sheet's poses.
  TakeOff(): void {
    if (this.mode === 'takeoff' && this.modeTime < TAKEOFF.end) return;
    this.setMode('takeoff');
  }

  // What it is doing: its behaviour, from the sheet's poses.
  get doing(): string {
    return this.mode;
  }

  // How high it flies over the ground or the water, m.
  get height(): number {
    return this.altitude;
  }

  private setMode(mode: Mode): void {
    if (mode !== this.mode) {
      this.modeTime = 0;
      this.splashes = 0;
      this.steps = 0;
    }
    this.mode = mode;
  }

  // A splash `ahead` m in front of it and `aside` m to its right.
  private splash(velocity: THREE.Vector3, ahead = 0, aside = 0): void {
    if (!this.onSplash) return;
    const heading = this.rotation.y;
    const [s, c] = [Math.sin(heading), Math.cos(heading)];
    this.onSplash(this.position.x + s * ahead + c * aside, this.position.z + c * ahead - s * aside, velocity);
  }

  // Its velocity in its parent's space now, plus `up` m/s up.
  private velocity(up: number): THREE.Vector3 {
    const heading = this.rotation.y;
    return new THREE.Vector3(Math.sin(heading) * this.speedNow, up, Math.cos(heading) * this.speedNow);
  }

  update(delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    this.time += dt;
    this.modeTime += dt;
    const t = this.modeTime;
    const now = this.time;
    // How fast it is being turned (the preview steers it), to bank into it.
    const turn = dt > 0 ? THREE.MathUtils.clamp((this.rotation.y - this.lastYaw) / dt, -5, 5) : 0;
    this.lastYaw = this.rotation.y;
    const water = this.afloat;

    // ------------------------------------------------ what it is doing
    const g = this.goal;
    Object.assign(g, REST);
    g.float = water ? 1 : 0;
    let gait: 'stand' | 'walk' | 'run' = 'stand';
    let feet: Feet | null = water ? 'paddle' : null; // null: on its feet on the ground
    let speed = 0; // the speed it eases to, off its feet
    let speedRate = 2.5;
    let paddle = water ? 0.35 : 0; // how hard it paddles
    let beatRate = 0;
    let altitude = 0;
    let lookAbout = false;
    const swim = () => {
      speed = SWIM_SPEED;
      paddle = 1;
      g.pitch = 0.12;
      g.tail = 0.22;
      g.neck = 0.05;
    };
    switch (this.mode) {
      case 'idle':
        lookAbout = true;
        if (water) {
          g.pitch = 0.1;
          g.tail = 0.2;
        }
        break;
      case 'walk':
        if (water) swim();
        else gait = 'walk';
        break;
      case 'swim':
        swim();
        break;
      case 'run':
        if (water) {
          speed = DASH_SPEED;
          paddle = 1.7;
          g.pitch = 0.2;
          g.neck = 0.28;
          g.tail = 0.12;
          g.float = 0.8;
        } else gait = 'run';
        break;
      case 'flap': {
        const on = t < FLAP.fold ? 1 : 0;
        g.spread = t < FLAP.fold - 0.05 ? 1 : 0;
        g.pitch = (water ? -0.85 : -0.68) * on;
        g.rise = water ? 0 : 0.014 * on;
        g.float = water ? 1 - 0.55 * on : 0;
        g.neck = 0.55 * on;
        g.neckTop = 0.1 * on;
        g.head = 0.05 * on;
        g.tail = water ? 0.15 : 0.55 * on;
        g.beat = t > FLAP.rise && t < FLAP.beats ? 0.95 : 0.2 * on;
        g.raise = 0.38;
        g.twist = 1.05 * on;
        beatRate = on ? FLAP_RATE : 0;
        if (water) paddle = 1.3 * on + 0.35 * (1 - on);
        if (t > FLAP.fold) lookAbout = true;
        if (t > FLAP.end) this.Idle();
        break;
      }
      case 'takeoff': {
        const T = TAKEOFF;
        g.spread = t < T.flare + 0.35 ? 1 : 0;
        g.raise = 0.3;
        if (t < T.run) {
          // The run up, beating its wings: running on land, pattering over
          // the water, rising out of it.
          if (water) {
            feet = 'patter';
            speed = 1.35;
            paddle = 2.2;
            g.float = 1 - smooth(t / (0.8 * T.run));
          } else gait = 'run';
          g.beat = 0.85;
          g.twist = 0.45;
          beatRate = 4.4;
          g.pitch = 0.04;
          g.neck = 0.35;
          g.neckTop = -0.1;
        } else if (t < T.flare) {
          // In the air: climbing, flying level, gliding down and flaring to land.
          feet = t < T.glide ? 'dangle' : 'reach';
          const climb = smooth((t - T.run) / (T.climb - T.run));
          if (t < T.level) altitude = FLY_HEIGHT * climb;
          else if (t < T.glide) altitude = 0.06 + (FLY_HEIGHT - 0.06) * (1 - smooth((t - T.level) / (T.glide - T.level)));
          else altitude = 0.06 * (1 - smooth((t - T.glide) / (T.flare - T.glide)));
          if (t < T.level) speed = FLY_SPEED;
          else if (t < T.glide) speed = 1.6;
          else {
            speed = water ? 0.5 : 0;
            speedRate = 5.5;
          }
          g.neck = 0.55;
          g.neckTop = -0.28;
          g.tail = 0.08;
          if (t < T.climb) {
            // Climbing steeply, as the sheet's pose leans back, its wings
            // beating high.
            g.pitch = -0.4;
            g.raise = 0.42;
            g.twist = 0.3;
            g.beat = 1;
            beatRate = 4.4;
          } else if (t < T.level) {
            g.pitch = 0.06;
            g.beat = 0.8;
            beatRate = 3.8;
          } else if (t < T.glide) {
            g.pitch = 0.04;
            g.beat = 0.06;
            g.raise = 0.18;
            beatRate = 1.4;
          } else {
            g.pitch = -0.62;
            g.neck = 0.25;
            g.neckTop = -0.1;
            g.beat = 0.75;
            g.raise = 0.35;
            g.sweep = -0.15;
            g.twist = 0.8;
            beatRate = 5;
          }
          if (water) g.float = 0;
        } else {
          // Down again: settling, folding its wings; on land a step with
          // each foot to stand square again.
          if (water) {
            g.pitch = 0.1;
            speedRate = 3;
          } else if (this.steps < this.legs.length && t > T.flare + 0.2 + 0.25 * this.steps) {
            const leg = this.legs[this.steps++];
            if (leg.down) {
              leg.down = false;
              leg.q = 0;
              leg.from.copy(leg.target);
            }
          }
          if (t > T.flare + 0.4) lookAbout = true;
          if (t > T.end) this.Idle();
        }
        // Splashes: leaving the water, and landing on it.
        if (water && this.splashes === 0 && t >= T.run) {
          this.splashes = 1;
          this.splash(this.velocity(1.2), -0.1);
        } else if (water && this.splashes === 1 && t >= T.flare) {
          this.splashes = 2;
          this.splash(this.velocity(-1.4), 0.1);
        }
        break;
      }
    }
    if (lookAbout) {
      // Now and then a look to one side and back, a bob of its head, and a
      // quick wag of its tail.
      g.headTurn = 0.45 * Math.sin(now * 0.37) * smooth(Math.sin(now * 0.23) * 1.5 + 0.2);
      g.head += 0.06 * Math.sin(now * 0.31);
      g.tailTurn = 0.3 * Math.sin(now * 25) * bump(((now + 2) % 6.5) / 0.7);
    }

    // ------------------------------------------------ going
    this.moving = ease(this.moving, gait === 'stand' ? 0 : 1, GAIT_RATE, dt);
    this.running = ease(this.running, gait === 'run' ? 1 : 0, GAIT_RATE, dt);
    const r = this.running;
    const mix = (a: number, b: number) => a + (b - a) * r;
    const cadence = mix(WALK.cadence, RUN.cadence);
    const duty = mix(WALK.duty, RUN.duty);
    const stride = mix(WALK.stride, RUN.stride) * this.moving;
    const onGround = feet === null;
    const walking = this.moving > 0.02 && onGround;
    if (walking) this.phase = (this.phase + cadence * dt) % 1;
    if (onGround) this.speedNow = walking ? (stride * cadence) / duty : 0;
    else this.speedNow = ease(this.speedNow, speed, speedRate, dt);
    this.advance(dt);
    const stroke = this.stroke;
    this.stroke = (this.stroke + STROKES * Math.max(paddle, 0.3) * dt) % 1;

    // The posture (at once on its first frame, so a duck put on the water
    // starts afloat), and the wings' beat.
    if (!this.started) Object.assign(this.posture, g);
    for (const key of KEYS) this.posture[key] = ease(this.posture[key], g[key], RATES[key] ?? 6, dt);
    this.beatRate = ease(this.beatRate, beatRate, 6, dt);
    const beatPhase = this.beatPhase;
    this.beatPhase = (this.beatPhase + TAU * this.beatRate * dt) % TAU;
    const p = this.posture;

    // How high it is: on the ground, or on the water (sitting in it as deep
    // as it floats), and flying over either.
    this.altitude = altitude;
    this.position.y = (water ? this.surface - SWIM_DEPTH * p.float : 0) + altitude;
    this.bank = ease(this.bank, altitude > 0.05 ? THREE.MathUtils.clamp((-this.speedNow * turn) / 9.8, -0.5, 0.5) : 0, 3, dt);

    // ------------------------------------------------ feet
    this.updateMatrix();
    this.rig.updateMatrix();
    const toWorld = m1.multiplyMatrices(this.matrix, this.rig.matrix);
    const toRig = m2.copy(toWorld).invert();
    if (!this.started) {
      // Its first frame: its feet planted where it stands, or it already
      // floating, as deep as it swims.
      this.started = true;
      for (const leg of this.legs) leg.plant.copy(leg.rest).applyMatrix4(toWorld);
    }
    if (onGround && this.footing === 'free') {
      // Landing: each foot is set down where it is.
      for (const leg of this.legs) this.touchDown(leg, toWorld);
    }
    this.footing = onGround ? 'ground' : 'free';
    if (onGround) this.stepFeet(dt, stride, duty, toWorld, toRig);

    // ------------------------------------------------ the body over them
    // Lowered as far as its legs need to reach their planted feet.
    let slack = 0;
    for (let pass = 0; pass < 6; pass++) {
      this.poseBody(slack, duty);
      if (!onGround) break;
      let need = 0;
      for (const leg of this.legs) need = Math.max(need, this.shortfall(leg));
      if (need <= 1e-6 || slack >= MOST_SLACK) break;
      slack = Math.min(MOST_SLACK, slack + need * 1.02);
    }
    if (feet !== null) for (const leg of this.legs) this.freeFoot(leg, feet, dt);
    // On land its toes never droop into the ground.
    if (!water) for (const leg of this.legs) leg.pitch = Math.min(leg.pitch, droopWithin(this.position.y + leg.target.y));
    for (const leg of this.legs) this.placeLeg(leg);

    // Splashes: its feet running over the water, and its wings beating on it.
    if (water && dt > 0) {
      if (feet === 'patter') {
        for (const leg of this.legs) {
          const [a, b] = [(stroke * 2 + leg.offset) % 1, (this.stroke * 2 + leg.offset) % 1];
          if (b < a) this.splash(this.velocity(0.6), 0.04, leg.side * LEG.x * U);
        }
      }
      const bottom = (a: number) => a < Math.PI && this.beatPhase >= Math.PI;
      if (this.mode === 'flap' && p.beat > 0.6 && this.splashes < 3 && bottom(beatPhase)) {
        this.splashes++;
        for (const side of [-1, 1]) this.splash(new THREE.Vector3(0, 1.1, 0), 0.02, side * 0.33);
      }
    }
  }

  // Steps its feet on the ground: planted feet stay where they are in the
  // space it moves in, swinging ones go forward to land again.
  private stepFeet(dt: number, stride: number, duty: number, toWorld: THREE.Matrix4, toRig: THREE.Matrix4): void {
    const r = this.running;
    const mix = (a: number, b: number) => a + (b - a) * r;
    const active = this.moving > 0.02;
    // A foot that lands once it stops still lifts a little.
    const lift = Math.max(mix(WALK.lift, RUN.lift) * Math.min(1, this.moving * 1.5), active ? 0 : 0.012);
    const curl = Math.max(mix(WALK.curl, RUN.curl) * Math.min(1, this.moving * 1.5), active ? 0 : 0.3);
    for (const leg of this.legs) {
      const land = v1.copy(leg.rest);
      land.z += stride / 2;
      if (active) {
        const u = (((this.phase + leg.offset) % 1) + 1) % 1;
        const wantDown = u < duty;
        if (wantDown && !leg.down) this.touchDown(leg, toWorld);
        else if (!wantDown && leg.down) {
          leg.down = false;
          leg.q = 0;
          leg.from.copy(leg.target);
        }
        if (!leg.down) leg.q = (u - duty) / (1 - duty);
      } else if (!leg.down) {
        leg.q = Math.min(1, leg.q + dt / SETTLE);
        if (leg.q >= 1) this.touchDown(leg, toWorld);
      }
      if (leg.down) {
        leg.target.copy(leg.plant).applyMatrix4(toRig);
        // A foot left far behind (the duck put somewhere else) is set down
        // under it again.
        if (leg.target.distanceTo(leg.rest) > 2 * (leg.upper + leg.lower)) {
          leg.plant.copy(leg.rest).applyMatrix4(toWorld);
          leg.target.copy(leg.rest);
        }
        leg.pitch = 0;
        leg.web = 1;
      } else {
        const s = smooth(leg.q);
        const arc = Math.sin(Math.PI * leg.q);
        leg.target.lerpVectors(leg.from, land, s);
        leg.target.y += lift * arc;
        leg.pitch = curl * arc;
        leg.web = 1 - 0.45 * arc;
      }
    }
  }

  private touchDown(leg: LegRig, toWorld: THREE.Matrix4): void {
    leg.down = true;
    leg.q = 1;
    leg.plant.copy(leg.target).setY(leg.rest.y).applyMatrix4(toWorld);
  }

  // Where a foot goes off the ground, in the body's space from where it
  // stands: eased there, so changing from one to another never snaps.
  private freeFoot(leg: LegRig, feet: Feet, dt: number): void {
    let dz = 0;
    let dy = 0;
    let pitch = 0;
    let web = 1;
    if (feet === 'paddle') {
      // A stroke: pushing back with the web spread and the toes down, then
      // forward again folded, trailing.
      const u = (this.stroke + leg.offset) % 1;
      const amp = this.mode === 'idle' ? 0.45 : 1;
      if (u < 0.55) {
        const f = u / 0.55;
        dz = (0.03 - 0.085 * smooth(f)) * amp;
        dy = -0.012 * Math.sin(Math.PI * f) * amp;
        pitch = 1.1 + 0.8 * f;
      } else {
        const f = (u - 0.55) / 0.45;
        dz = (-0.055 + 0.085 * smooth(f)) * amp;
        dy = 0.012 * Math.sin(Math.PI * f) * amp;
        pitch = 1.9 - 0.8 * smooth(f) + 0.9 * Math.sin(Math.PI * f);
        web = 1 - 0.55 * Math.sin(Math.PI * f);
      }
    } else if (feet === 'patter') {
      // Running over the water: down on it and back, then up and forward.
      const v = (this.stroke * 2 + leg.offset) % 1;
      if (v < 0.45) {
        dz = 0.05 - 0.1 * (v / 0.45);
        pitch = 0.1;
      } else {
        const f = (v - 0.45) / 0.55;
        dz = -0.05 + 0.1 * smooth(f);
        dy = 0.03 * Math.sin(Math.PI * f);
        pitch = 0.6 * Math.sin(Math.PI * f);
        web = 1 - 0.4 * Math.sin(Math.PI * f);
      }
    } else if (feet === 'dangle') {
      // Hanging in flight, the toes spread.
      dz = -0.018;
      dy = 0.01;
      pitch = 0.95 + 0.08 * Math.sin(this.time * 3 + leg.offset * 4);
    } else {
      // Reaching forward to land.
      dz = 0.03;
      dy = 0.01;
      pitch = -0.2;
    }
    // Into the rig's space through the body's bone.
    const want = v1.copy(leg.restInTorso).add(v2.set(0, dy, dz)).applyMatrix4(this.torso.matrix);
    const k = 1 - Math.exp(-16 * dt);
    const j = 1 - Math.exp(-12 * dt);
    leg.target.lerp(want, k);
    leg.pitch += (pitch - leg.pitch) * j;
    leg.web += (web - leg.web) * j;
    leg.down = false;
  }

  // The body, the neck, the head, the tail and the wings, from the posture
  // and the gait, `slack` m lower than the posture holds it.
  private poseBody(slack: number, duty: number): void {
    const p = this.posture;
    const m = this.moving;
    const r = this.running;
    const mix = (a: number, b: number) => a + (b - a) * r;
    // The waddle: onto the left foot at the middle of its stance, then the right.
    const c = TAU * (this.phase - duty / 2);
    const roll = mix(WALK.roll, RUN.roll) * m * Math.cos(c);
    const sway = -mix(WALK.sway, RUN.sway) * m * Math.cos(c);
    const yaw = mix(WALK.yaw, RUN.yaw) * m * Math.sin(c);
    const bob = mix(WALK.bob, RUN.bob) * m * Math.cos(2 * c);
    // Leaning ahead going, more running.
    const lean = (0.1 + 0.12 * r) * m;
    const breath = 0.0025 * Math.sin(this.time * 2.3) * (1 - m);
    const torso = this.torso;
    torso.position.set(this.torsoRest.x + sway, this.torsoRest.y + p.rise + bob + breath - slack, this.torsoRest.z);
    const pitch = p.pitch + lean;
    torso.rotation.set(pitch, yaw, roll + this.bank);
    // The neck out ahead going, the head kept level, bobbing a little with
    // each step.
    const [n0, n1, hd] = this.neckBones;
    const nod = 0.05 * m * Math.cos(2 * c - 0.6);
    n0.rotation.set(p.neck + (0.18 + 0.14 * r) * m + nod * 0.6, 0, 0);
    n1.rotation.set(p.neckTop - 0.05 * m - nod * 0.3, 0, 0);
    hd.rotation.set(p.head + nod * 0.3 - (pitch + n0.rotation.x + n1.rotation.x), p.headTurn, -0.6 * (roll + this.bank));
    // The tail wagging with its steps.
    this.tailBone.rotation.set(p.tail + 0.05 * m, p.tailTurn + 0.14 * m * Math.sin(c + 0.8), 0);
    // The wings: spread as far as the posture has them, beating.
    const spread = p.spread;
    const beat = Math.cos(this.beatPhase);
    const flex = Math.sin(this.beatPhase);
    for (const wing of this.wings) {
      const s = wing.side;
      wing.mesh.morphTargetInfluences![0] = spread;
      // Its undersides turned forward as far as the pose has them, more on the
      // downstroke, pitching as it beats.
      const twist = p.twist * (1 + 0.25 * flex) + 0.18 * p.beat * flex;
      wing.shoulder.rotation.set(-twist * spread, s * p.sweep * spread, s * (p.raise + p.beat * beat) * spread);
      wing.wrist.rotation.set(0, s * 0.3 * Math.max(0, -flex) * p.beat * spread, s * 0.4 * p.beat * flex * spread);
    }
    this.rig.updateMatrixWorld(true);
  }

  // The leg's hip, and where its ankle must be, in the body's space.
  private solve(leg: LegRig) {
    const into = m1.copy(this.rig.matrixWorld).invert().multiply(this.torso.matrixWorld).invert();
    const ankle = v1.copy(leg.target).applyMatrix4(into);
    const hip = leg.hip.position;
    const dx = ankle.x - hip.x;
    const dy = ankle.y - hip.y;
    const dz = ankle.z - hip.z;
    return { out: Math.atan2(dx, -dy), down: -Math.hypot(dx, dy), dz };
  }

  // How much lower the body must be for the leg to reach its foot.
  private shortfall(leg: LegRig): number {
    const { down, dz } = this.solve(leg);
    const d = Math.hypot(down, dz);
    const most = (leg.upper + leg.lower) * REACH_SHARE;
    if (d <= most) return 0;
    return (d - most) * Math.min(3, d / Math.max(Math.abs(down), 1e-6));
  }

  private placeLeg(leg: LegRig): void {
    const { out, down, dz } = this.solve(leg);
    const { upper, lower } = reach(0, 0, down, dz, leg.upper, leg.lower, -1);
    const b0 = leg.angles[0] - upper;
    const b1 = leg.angles[1] - lower;
    leg.hip.rotation.set(b0, 0, out);
    leg.heel.rotation.set(b1 - b0, 0, 0);
    // The foot square to the way it goes, level when planted (or pitched as
    // it goes), in the rig's space, however the body rocks and the leg bends.
    q1.copy(this.torso.quaternion).multiply(leg.hip.quaternion).multiply(leg.heel.quaternion).invert();
    leg.foot.quaternion.copy(q1.multiply(q2.setFromEuler(e1.set(leg.pitch, 0, 0))));
    leg.foot.scale.set(0.55 + 0.45 * leg.web, 1, 1);
  }
}
