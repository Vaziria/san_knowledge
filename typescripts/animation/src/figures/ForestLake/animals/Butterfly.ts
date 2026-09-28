import * as THREE from 'three';
import { ForestAnimal, type ForestAnimalOptions } from './ForestAnimal';
import { blend, coat, ease, eye, gloss, loft, rigidMesh, Sculpt, seededRandom, smooth, thin, tone, tubeRings, type Palette, type Tone } from './parts';

// The forest lake's butterfly (Butterfly.md), a monarch as its reference
// sheet draws it: bright orange wings, fore and hind, split into cells by
// dark veins, with broad dark borders carrying rows of white spots and a dark
// forewing tip; a dark brown body (a round head with big black eyes in a tan
// ring, a deep thorax with a tan collar, a slim tapering abdomen, a few pale
// dots), six thin dark legs, and long thin antennae ending in tan clubs.
// Low poly: every face flat, the wings flat sheets seen from both sides,
// their veins, borders and spots cut into them as faces of their own so the
// pattern stays crisp. Its colours are the sheet's six variations.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground under
// its thorax, which hovers `Butterfly.HOVER` above it. Its wings and body are
// traced from the sheet's front view, which shows it from above with its
// wings spread flat, in the sheet's pixels (U m each), so it is 10 cm across
// its wings, as a monarch is; its heights come from the side view.

const WINGSPAN = 0.1; // m, from one forewing's tip to the other's
const U = WINGSPAN / 220; // m per pixel of the sheet's front view: its forewings' tips are 220 px apart
const MIDDLE = 776; // px, the front view's middle line, down the body
const THORAX_Y = 211; // px, the thorax's middle, down the front view
const TAU = Math.PI * 2;

// A point traced from the sheet's front view (from above, head up): `x` and
// `y` in its pixels (its left half is the butterfly's left), `h` pixels above
// the thorax's middle.
function F(x: number, y: number, h = 0): THREE.Vector3 {
  return new THREE.Vector3((x - MIDDLE) * U, h * U, (THORAX_Y - y) * U);
}

// The sheet's six colour variations, picked from its pictures: the wings'
// four oranges (or blues, purples...) from deep to pale; the veins and
// borders, which stay dark; the white spots; the body and its tan facets;
// the eyes and the collars round them; the antennae's clubs.
const VARIATIONS: Record<string, Palette> = {
  monarch: { deep: 0xfc621d, wing: 0xfd8b20, light: 0xfea224, pale: 0xfec23e, vein: 0x503a39, spot: 0xfff0e6, body: 0x4e3835, tan: 0xc4886a, rim: 0xd67c48, eye: 0x0d0b0b, shine: 0xffffff, club: 0xe6a272 },
  blue: { deep: 0x2c74cc, wing: 0x42a4dc, light: 0x6ccbee, pale: 0x8edcf5, vein: 0x2b3444, spot: 0xeef2f6, body: 0x3a3c4a, tan: 0x8290a8, rim: 0x8ea4c4, eye: 0x0b0c10, shine: 0xffffff, club: 0xa2d2e8 },
  purple: { deep: 0x9442d6, wing: 0xbf64f2, light: 0xd881f5, pale: 0xea9cfc, vein: 0x3e2d52, spot: 0xf3ecf6, body: 0x483a55, tan: 0xa48cbc, rim: 0xb698d2, eye: 0x0c0a10, shine: 0xffffff, club: 0xd4aced },
  yellow: { deep: 0xf7a620, wing: 0xfbc02a, light: 0xfcd63e, pale: 0xfde858, vein: 0x5a4034, spot: 0xf8ecd8, body: 0x6c5046, tan: 0xc49e72, rim: 0xd6a45e, eye: 0x0d0b0b, shine: 0xffffff, club: 0xeecf7c },
  white: { deep: 0xeee0d2, wing: 0xf6e8da, light: 0xfaf2ea, pale: 0xfdfaf5, vein: 0x4f3d3b, spot: 0xfbf6f0, body: 0x584e54, tan: 0xb4a4a4, rim: 0xc8b0a8, eye: 0x0d0b0b, shine: 0xffffff, club: 0xf2e6dc },
  pink: { deep: 0xfa6caa, wing: 0xfb88c0, light: 0xfca4d3, pale: 0xfcbde0, vein: 0x5c3444, spot: 0xf8eaf0, body: 0x604c5c, tan: 0xcc94ac, rim: 0xdc98b8, eye: 0x0d0b0b, shine: 0xffffff, club: 0xf2acca },
};

// ---------------------------------------------------------------- wings

// The wings hinge on the thorax's upper sides, along it.
const HINGE_X = 7.5; // px out from the middle line
const HINGE_H = 4; // px above the thorax's middle: low enough that the thorax shows broad between the wings, as the sheet has it
const FLAP_Y = 207; // px, where along the hinge each wing's flapping pivot sits
const FORE_ROOT_Y = 201; // px, where the forewing's root turns (sweeps) on the hinge
const HIND_ROOT_Y = 214; // px, and the hind wing's
const REACH = 110; // px out from the middle line to the forewing's tip
const FORE_LIFT = 0.0008; // m the forewing lies above the hind wing, where it overlaps it
const CAMBER = 0.0008; // m a wing bows up halfway out
const RUMPLE = 0.00025; // m each facet's corner stands off the bow, either way, so the facets catch the light differently

type V2 = readonly [number, number];

// A cell of a wing between its veins: pieces of one colour each, sharing
// corners, as the sheet's cells are two or three flat facets of different
// oranges.
interface Piece {
  poly: readonly V2[];
  tone: Tone;
}
type Cell = readonly Piece[];

// A white spot: its middle, width, height and turn (degrees, in the
// picture's own axes, y down).
type Spot = readonly [x: number, y: number, w: number, h: number, turn: number];

interface WingTrace {
  outline: readonly V2[];
  cells: readonly Cell[];
  spots: readonly Spot[];
}

const along = (a: V2, b: V2, t: number): V2 => [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t];

// The forewing, traced from the front view's left wing: its outline from the
// root up the leading edge to the tip, down the outer edge and back along the
// inner edge; its orange cells inside the dark veins and border; the white
// spots in the border and the dark tip.
const O2 = { a: [695.5, 150.5], b: [715.8, 153], c: [730.5, 172.8], d: [699, 170.5], e: [695, 160] } as const satisfies Record<string, V2>;
const O2_TOP = along(O2.a, O2.b, (704 - 695.5) / (715.8 - 695.5));
const O2_FOOT = along(O2.c, O2.d, (730.5 - 712) / (730.5 - 699));
const NOTCH: V2 = [733.5, 173];
const FORE_BASE: V2 = [766.5, 204];
const FORE: WingTrace = {
  outline: [
    [769.5, 207.5], [770, 198.5], [766, 183], [760, 168], [751.5, 154], [740, 139.5], [728.5, 128], [717, 118.7], [703, 109.4], [688.6, 102.3], [673, 97.3], [666.4, 98.7],
    [665.7, 103.7], [667, 116.6], [669.3, 128], [673, 138], [677, 148], [681.4, 159.4], [685, 169.4], [688, 179.4], [693, 190.9], [698.6, 199.4], [704.3, 206.6],
    [725, 204], [750, 205.5],
  ],
  cells: [
    // The pale cell under the dark tip.
    [{ poly: [[687.5, 134.5], [707, 136.2], [716.2, 151.8], [690.5, 146.5]], tone: tone('pale') }],
    // The middle cell: deep by the border, lighter toward the vein it shares
    // with the long cell.
    [
      { poly: [O2.a, O2_TOP, O2_FOOT, O2.d, O2.e], tone: tone('deep') },
      { poly: [O2_TOP, O2.b, O2.c, O2_FOOT], tone: tone('wing') },
    ],
    // The long deep cell along the leading edge and the broad light one
    // along the inner edge, one cell from the root to a notched vein.
    [
      { poly: [[714.5, 140.5], [726, 145], [736, 153.5], [746, 165.5], [755, 179], [762, 193], FORE_BASE, NOTCH], tone: tone('deep') },
      { poly: [NOTCH, FORE_BASE, [750, 202], [730, 197], [704, 190], [700, 174.5], [731, 175.5]], tone: tone('light') },
    ],
  ],
  spots: [
    [669.8, 102.6, 3.6, 3.2, 0],
    [683, 113, 10, 8, 20],
    [676, 125.2, 6, 5, 10],
    [676.6, 131.8, 3.2, 3, 0],
    [712, 128.5, 21, 6.5, 36], // the long bar under the leading edge
    [686.4, 153.8, 6.5, 8, 70],
    [692.5, 175.4, 5.5, 7, 75],
  ],
};

// The hind wing, traced the same way: a fan of cells from the root, a
// deep band along its top (mostly under the forewing), the big middle
// cell with a deep stripe along it, and the border's two rows of spots.
const H2_ROOT: V2 = [766.5, 216.1];
const H2_N: V2 = [731.75, 232.9];
const H6_TOP: V2 = [744.4, 232.4];
const STRIPE_END = along(H2_N, H6_TOP, 0.69);
const STRIPE_TOP: V2 = [766.5, 217.2];
const STRIPE_FOOT: V2 = [766.5, 219];
const H5_TOP: V2 = [730.3, 235.2];
const H5_CUT: V2 = [713.5, 247.5];
const HIND: WingTrace = {
  outline: [
    [769, 206], [768, 200], [740, 199.5], [718, 201], [708, 204.5], [704, 207], [697, 213], [693, 221], [691.5, 231], [692, 238], [694, 246], [697, 254], [701, 262],
    [706, 268], [714, 274], [722, 278], [729, 281], [732, 281.5], [741, 276], [749, 268], [756, 258], [762, 247], [766, 236], [768.5, 225], [769, 215],
  ],
  cells: [
    [{ poly: [[766.5, 202.5], [722, 202.5], [719.2, 208], [719.6, 212.9], [734, 214.9], [766.5, 213.9]], tone: tone('deep') }],
    [{ poly: [[732.2, 216.9], [717, 215.1], [714, 216.5], [711, 223], [710.2, 227.4], [727, 225.3]], tone: tone('deep') }],
    [
      { poly: [H5_TOP, [727.3, 227.8], [709.6, 229.6], [708, 234], [709, 241], [712, 246], H5_CUT], tone: tone('wing') },
      { poly: [H5_TOP, H5_CUT, [716, 250], [719, 256], [724, 259], [730, 262], [734, 264], [741.6, 235]], tone: tone('pale') },
    ],
    [
      { poly: [H2_ROOT, [734.6, 217.1], [729.1, 226.8], H2_N, STRIPE_END, STRIPE_TOP], tone: tone('wing') },
      { poly: [STRIPE_TOP, STRIPE_END, H6_TOP, STRIPE_FOOT], tone: blend('deep', 'wing', 0.3) },
      { poly: [STRIPE_FOOT, H6_TOP, [736, 265], [741, 267.5], [746, 266], [749, 260], [756, 248], [762, 236], [766, 223]], tone: tone('light') },
    ],
  ],
  spots: [
    [712.8, 209, 4, 3.4, 0],
    [703.8, 220, 7.5, 9, 15],
    [702.8, 239.4, 7, 9, 0],
    [712.5, 259.2, 8, 9.5, -30],
    [722.5, 267.7, 3, 3, 0],
    [733.3, 272.7, 4.5, 4.5, 0],
  ],
};

// A number from 0 to 1 that looks random but is always the same for a point
// (so corners two faces share move and shade alike).
function grain(p: V2, seed = 0): number {
  const h = Math.sin(p[0] * 12.9898 + p[1] * 78.233 + seed * 37.719) * 43758.5453;
  return h - Math.floor(h);
}

function area(p: readonly V2[]): number {
  let a = 0;
  p.forEach(([x0, y0], i) => {
    const [x1, y1] = p[(i + 1) % p.length];
    a += x0 * y1 - x1 * y0;
  });
  return a / 2;
}

function centroid(p: readonly V2[]): V2 {
  let x = 0;
  let y = 0;
  for (const [px, py] of p) {
    x += px;
    y += py;
  }
  return [x / p.length, y / p.length];
}

function convex(p: readonly V2[]): boolean {
  let sign = 0;
  for (let i = 0; i < p.length; i++) {
    const [ax, ay] = p[i];
    const [bx, by] = p[(i + 1) % p.length];
    const [cx, cy] = p[(i + 2) % p.length];
    const cross = (bx - ax) * (cy - by) - (by - ay) * (cx - bx);
    if (Math.abs(cross) < 1e-9) continue;
    if (sign === 0) sign = Math.sign(cross);
    else if (Math.sign(cross) !== sign) return false;
  }
  return true;
}

// A cell's outline: its pieces put together, the edges they share taken out.
function outlineOf(cell: Cell): V2[] {
  const key = (p: V2) => `${p[0]},${p[1]}`;
  const points = new Map<string, V2>();
  const edges = new Set<string>();
  for (const { poly } of cell) {
    const ring = area(poly) > 0 ? poly : [...poly].reverse();
    ring.forEach((a, i) => {
      const b = ring[(i + 1) % ring.length];
      points.set(key(a), a);
      const back = `${key(b)}>${key(a)}`;
      if (edges.has(back)) edges.delete(back);
      else edges.add(`${key(a)}>${key(b)}`);
    });
  }
  const next = new Map<string, string>();
  for (const edge of edges) {
    const [a, b] = edge.split('>');
    next.set(a, b);
  }
  const start = next.keys().next().value as string;
  const loop: V2[] = [];
  let k = start;
  do {
    loop.push(points.get(k)!);
    k = next.get(k)!;
  } while (k !== start && loop.length <= next.size);
  return loop;
}

// Triangles filling an outline less its holes (none may touch another).
function triangulate(contour: readonly V2[], holes: readonly (readonly V2[])[] = []): [V2, V2, V2][] {
  const v = (p: V2) => new THREE.Vector2(p[0], p[1]);
  const all = [...contour, ...holes.flat()];
  const faces = THREE.ShapeUtils.triangulateShape(contour.map(v), holes.map((h) => h.map(v)));
  return faces.map(([a, b, c]) => [all[a], all[b], all[c]] as [V2, V2, V2]);
}

// A convex piece is fanned from its middle, so it shows as a few facets;
// any other shape is cut into as few triangles as it takes.
function facets(poly: readonly V2[]): [V2, V2, V2][] {
  if (!convex(poly)) return triangulate(poly);
  const c = centroid(poly);
  return poly.map((a, i) => [c, a, poly[(i + 1) % poly.length]] as [V2, V2, V2]);
}

// A spot's outline: an angular six-cornered patch, each corner pushed in or
// out a little, as the sheet's spots are cut unevenly.
function spotRing([x, y, w, h, turn]: Spot): V2[] {
  const c = Math.cos(THREE.MathUtils.degToRad(turn));
  const s = Math.sin(THREE.MathUtils.degToRad(turn));
  const corners = [[0.5, 0], [0.22, 0.5], [-0.28, 0.46], [-0.5, 0], [-0.24, -0.5], [0.26, -0.46]];
  return corners.map(([u, v], k) => {
    const j = 0.82 + 0.32 * grain([x + k, y - k], 3);
    const px = u * w * j;
    const py = v * h * j;
    return [x + px * c - py * s, y + px * s + py * c] as V2;
  });
}

const shaded = (t: Tone, k: number): Tone => ({ ...t, shade: (t.shade ?? 1) * k });

// Where a traced point of a wing is on the butterfly, as it is built: the
// left wing as traced, the right one its mirror; bowed and rumpled a little.
function wingPoint(p: V2, side: 1 | -1, lift: number): THREE.Vector3 {
  const out = MIDDLE - p[0];
  const span = THREE.MathUtils.clamp((out - HINGE_X) / (REACH - HINGE_X), 0, 1);
  const rise = CAMBER * Math.sin(Math.PI * span) + RUMPLE * (2 * grain(p) - 1);
  return new THREE.Vector3(side * out * U, HINGE_H * U + lift + rise, (THORAX_Y - p[1]) * U);
}

// A wing as one flat sheet: the dark veins and border (all of it that is
// not a cell or a spot), the cells and the spots, every face its own.
function sculptWing(s: Sculpt, trace: WingTrace, side: 1 | -1, lift: number, pivot: THREE.Vector3): void {
  const up = new THREE.Vector3(0, 1, 0);
  const place = (p: V2) => wingPoint(p, side, lift).sub(pivot);
  const face = (a: V2, b: V2, c: V2, t: Tone) => s.toward(place(a), place(b), place(c), up, t);
  const holes = trace.cells.map(outlineOf);
  const spots = trace.spots.map(spotRing);
  for (const [a, b, c] of triangulate(trace.outline, [...holes, ...spots])) face(a, b, c, tone('vein', 0.88 + 0.26 * grain(centroid([a, b, c]), 1)));
  for (const cell of trace.cells) {
    for (const piece of cell) {
      for (const [a, b, c] of facets(piece.poly)) face(a, b, c, shaded(piece.tone, 0.9 + 0.2 * grain(centroid([a, b, c]), 2)));
    }
  }
  for (const ring of spots) {
    const c = centroid(ring);
    ring.forEach((a, i) => face(c, a, ring[(i + 1) % ring.length], tone('spot', 0.96 + 0.06 * grain(a, 4))));
  }
}

// ---------------------------------------------------------------- body

// Rings traced from the front view (y down it, h up from the thorax's
// middle, half width w) and the side view (up and down), in pixels, from
// back to front.
interface Station {
  y: number;
  h: number;
  w: number;
  up: number;
  down: number;
}

// The thorax is deep, as the side view draws it: taller than it is wide.
const THORAX: readonly Station[] = [
  { y: 227.5, h: -1.5, w: 4.8, up: 4.8, down: 6 },
  { y: 223.5, h: -1, w: 7.4, up: 7.8, down: 10 },
  { y: 218, h: -0.5, w: 9.8, up: 10.2, down: 14 },
  { y: 211, h: 0, w: 10.4, up: 11, down: 16 },
  { y: 204, h: 0.3, w: 10.1, up: 10.8, down: 15.5 },
  { y: 198.5, h: 0.8, w: 8.2, up: 8.8, down: 12 },
  { y: 195.5, h: 1.2, w: 5.5, up: 5.5, down: 7 },
];

const NECK = F(MIDDLE, 196, 1); // where the head sits on the thorax
const HEAD: readonly Station[] = [
  { y: 197.5, h: 1.2, w: 5, up: 5.4, down: 5.4 },
  { y: 194.5, h: 1.5, w: 7.3, up: 8.4, down: 8 },
  { y: 190.5, h: 1.5, w: 8.2, up: 9.3, down: 9 },
  { y: 186.5, h: 1.2, w: 7.3, up: 8.2, down: 8.2 },
  { y: 183, h: 0.4, w: 5, up: 5.2, down: 6 },
];
const EYE_Y = 190; // px down the front view, where the eyes bulge from the head's sides
const EYE_H = 1.5;
const EYE_RADIUS = 5.6; // px: big, filling most of the head's side, as the sheet's close-up draws them
const COLLAR_IN = 1.02; // of the eye's radius, where the collar round it starts
const COLLAR_OUT = 1.32; // and where it sinks into the head

const WAIST = F(MIDDLE, 225, -1.5); // where the abdomen hangs from the thorax
// Its middle line from the waist back to its tip, sagging, and how wide it
// is there (px).
const ABDOMEN: readonly [y: number, h: number, w: number][] = [
  [225, -1.5, 6],
  [231, -2.8, 7],
  [238, -4.3, 6.8],
  [245, -5.9, 6.1],
  [252, -7.5, 5.1],
  [259, -9.1, 3.8],
  [265.5, -10.6, 2.4],
];

// A face's patchiness: now and then a facet a little lighter or darker, as
// the sheet's body catches the light facet by facet.
function patchy(role: string, r: number, k: number): Tone {
  const f = grain([r * 3.1, k * 7.7], 5);
  return tone(role, f < 0.25 ? 0.86 : f > 0.8 ? 1.12 : 1);
}

function sculptThorax(s: Sculpt): void {
  const rings = THORAX.map((r) => ({ at: F(MIDDLE, r.y, r.h), w: r.w * U, up: r.up * U, down: r.down * U }));
  loft(s, rings, {
    sides: 8,
    turn: Math.PI / 8,
    start: 1.5 * U,
    end: 'flat',
    paint: (r, angle, k) => {
      const top = Math.min(angle, TAU - angle); // 0 on top, π underneath
      if (r >= THORAX.length - 2 && top < 1.9) return tone('tan', 0.96); // the collar behind the head
      if (r === THORAX.length - 3 && top > 0.35 && top < 1.2) return blend('tan', 'body', 0.35); // the wings' bases
      if (top > 2.3) return tone('body', 0.8); // underneath, between the legs
      return patchy('body', r, k);
    },
  });
  // Its pale dots, two a side.
  for (const side of [-1, 1]) {
    dot(s, F(MIDDLE + side * 20, 205, 2), new THREE.Vector3(-side, 0, 0), 1.6);
    dot(s, F(MIDDLE + side * 20, 215, -3), new THREE.Vector3(-side, 0, 0), 1.3);
  }
}

// A pale dot seated on a sculpted surface where a ray from `from` going
// `way` meets it (none where it misses): a small hexagon of the spots'
// colour standing just off it.
function dot(s: Sculpt, from: THREE.Vector3, way: THREE.Vector3, size: number): void {
  const w = way.clone().normalize();
  const hit = s.cast(from, w);
  if (!Number.isFinite(hit)) return;
  const at = from.clone().addScaledVector(w, hit);
  const n = w.clone().negate();
  const x = new THREE.Vector3().crossVectors(n, new THREE.Vector3(0, 0, 1));
  if (x.lengthSq() < 1e-8) x.set(1, 0, 0);
  x.normalize();
  const y = new THREE.Vector3().crossVectors(n, x);
  const middle = at.clone().addScaledVector(n, 0.00005);
  const r = size * U * 0.5;
  const ring = Array.from({ length: 6 }, (_, k) =>
    middle
      .clone()
      .addScaledVector(x, Math.cos((k * TAU) / 6) * r)
      .addScaledVector(y, Math.sin((k * TAU) / 6) * r),
  );
  ring.forEach((a, k) => s.toward(middle, a, ring[(k + 1) % 6], n, tone('spot', 0.95)));
}

// The head, as it sits on the neck (in the head's own space, the neck at its
// origin): a faceted ball, tan round the eyes and over the crown, dark below
// and at the face; the eyes, big black domes with a glint, on `shiny`.
function sculptHead(s: Sculpt, shiny: Sculpt): void {
  const rings = HEAD.map((r) => ({ at: F(MIDDLE, r.y, r.h).sub(NECK), w: r.w * U, up: r.up * U, down: r.down * U }));
  loft(s, rings, {
    sides: 8,
    turn: Math.PI / 8,
    start: 'flat',
    end: 2.2 * U,
    paint: (r, angle, k) => {
      const top = Math.min(angle, TAU - angle);
      if (r >= HEAD.length - 1) return top < 1.2 ? blend('body', 'tan', 0.3) : tone('body', 0.82); // the face
      if (top < 0.6) return blend('body', 'tan', 0.55); // the crown
      if (top < 2.3 && r >= 1) return r === 1 ? blend('tan', 'body', 0.45) : tone('tan'); // the ring round each eye
      if (top > 2.3) return tone('body', 0.85);
      return patchy('body', r, k);
    },
  });
  for (const side of [-1, 1]) {
    // Seated where a ray from the head's middle along its look leaves the
    // head, so that it and its collar sit square on the surface.
    const look = new THREE.Vector3(side * 0.85, 0.16, 0.5).normalize();
    const middle = F(MIDDLE, EYE_Y, EYE_H).sub(NECK);
    const at = s.onto(middle.clone().addScaledVector(look, 0.02), look.clone().negate()).addScaledVector(look, -0.12 * EYE_RADIUS * U);
    eye(shiny, at, look, new THREE.Vector3(0, 1, 0), EYE_RADIUS * U, { lid: tone('eye'), iris: tone('eye', 1.4), pupil: tone('eye'), shine: tone('shine') }, 8, 0.6, 1.05);
    collar(s, at, look, EYE_RADIUS * U);
    dot(s, F(MIDDLE + side * 3.5, 193.5, 20).sub(NECK), new THREE.Vector3(0, -1, 0), 1.2);
  }
}

// The collar round an eye, as the sheet's close-up draws it: a raised ring
// of tan facets sloping from the eye's edge down into the head.
function collar(s: Sculpt, centre: THREE.Vector3, look: THREE.Vector3, radius: number): void {
  const z = look.clone().normalize();
  const x = new THREE.Vector3().crossVectors(new THREE.Vector3(0, 1, 0), z).normalize();
  const y = new THREE.Vector3().crossVectors(z, x);
  const sides = 8;
  const ring = (r: number, out: number) =>
    Array.from({ length: sides }, (_, k) => {
      const a = (TAU * (k + 0.5)) / sides;
      return centre
        .clone()
        .addScaledVector(x, Math.cos(a) * r)
        .addScaledVector(y, Math.sin(a) * r)
        .addScaledVector(z, out);
    });
  const inner = ring(radius * COLLAR_IN, radius * 0.08);
  const outer = ring(radius * COLLAR_OUT, -radius * 0.38);
  for (let k = 0; k < sides; k++) {
    const k1 = (k + 1) % sides;
    const t = tone('rim', 0.9 + 0.2 * grain([k, 7], 6));
    s.toward(inner[k], outer[k], outer[k1], z, t);
    s.toward(inner[k], outer[k1], inner[k1], z, t);
  }
}

// The abdomen, as it hangs from the waist (in its own space, the waist at its
// origin): a slim six-sided taper in dark bands, a row of pale dots along
// each side.
function sculptAbdomen(s: Sculpt): void {
  const path = [...ABDOMEN].reverse().map(([y, h]) => F(MIDDLE, y, h).sub(WAIST));
  const radii = [...ABDOMEN].reverse().map(([, , w]) => [w * U, w * 1.0 * U, w * 1.06 * U] as const);
  loft(s, tubeRings(path, radii), {
    sides: 6,
    turn: Math.PI / 6, // a flat back and belly, a ridge down each side
    start: 2.6 * U,
    end: 'flat',
    paint: (r, angle, k) => {
      const top = Math.min(angle, TAU - angle);
      const band = r % 2 === 0 ? 1 : 0.86;
      if (top > 2.4) return tone('body', 0.78 * band);
      return shaded(patchy('body', r, k), band);
    },
  });
  for (const side of [-1, 1]) {
    for (const [y, h] of [
      [236, -7],
      [245, -8.8],
      [254, -10.6],
    ]) {
      dot(s, F(MIDDLE + side * 18, y, h).sub(WAIST), new THREE.Vector3(-side, 0.35, 0), 1.1);
    }
  }
}

// ---------------------------------------------------------------- antennae

// Each antenna runs from its root on the head's brow forward, out and up to
// a tan club shaped like a long diamond (px, the right one; the left is its
// mirror).
const ANTENNA_ROOT = { x: 4, y: 183.5, h: 5.5 };
const ANTENNA_REACH = { x: 21, y: -55.5, h: 17 }; // px from the root to the club's tip
const CLUB_SHARE = 0.28; // of its length, the club
const STALK = 0.00036; // m, the stalk's radius at its root
const CLUB = 0.0013; // m, the club's radius at its widest

function sculptAntenna(s: Sculpt, side: 1 | -1): void {
  const root = F(MIDDLE + side * ANTENNA_ROOT.x, ANTENNA_ROOT.y, ANTENNA_ROOT.h);
  const reach = new THREE.Vector3(side * ANTENNA_REACH.x * U, ANTENNA_REACH.h * U, -ANTENNA_REACH.y * U);
  const bow = new THREE.Vector3(0, 2.2 * U, 0); // it arches up a little
  const point = (t: number) => root.clone().addScaledVector(reach, t).addScaledVector(bow, Math.sin(Math.PI * Math.min(1, t / (1 - CLUB_SHARE))));
  const clubAt = 1 - CLUB_SHARE;
  const stalk = [0, 0.12, 0.4, clubAt].map(point).map((p) => p.sub(root));
  loft(s, tubeRings(stalk, [STALK, STALK * 0.9, STALK * 0.8, STALK * 0.75]), {
    sides: 4,
    start: 'flat',
    end: 'flat',
    paint: (r) => (r === 0 ? tone('tan', 0.9) : r === 1 ? blend('body', 'tan', 0.4) : tone('body', 0.78)),
  });
  const club = [clubAt, clubAt + CLUB_SHARE * 0.55, clubAt + CLUB_SHARE * 0.8].map(point).map((p) => p.sub(root));
  loft(s, tubeRings(club, [STALK * 0.9, CLUB, CLUB * 0.7]), {
    sides: 6,
    start: 'flat',
    end: point(1).distanceTo(point(clubAt + CLUB_SHARE * 0.8)),
    paint: (r, angle) => tone('club', r === 0 ? 0.92 : Math.cos(angle) > 0.3 ? 1.08 : 0.94),
  });
}

// ---------------------------------------------------------------- legs

const FEMUR = 0.01; // m, hip to knee
const TIBIA = 0.012; // m, knee to ankle
const TARSUS = 0.0035; // m, the foot
const TARSUS_SLOPE = 0.35; // rad a standing foot slopes down to its tip
const TOE = 0.0005; // m a standing foot's tip is held above what it stands on, so its pointed end and its thickness rest on it rather than in it

// The right legs (the left are their mirrors): the hip under the thorax;
// standing, where its foot's tip is (m, x out and z forward of the thorax's
// middle, on what it stands on); flying, where its ankle hangs and which way
// its foot dangles (in the body's own space); and which way each knee bends
// either way.
interface LegSpec {
  hip: THREE.Vector3;
  tip: readonly [x: number, z: number];
  hang: THREE.Vector3;
  dangle: THREE.Vector3;
  kneeStand: THREE.Vector3;
  kneeHang: THREE.Vector3;
}

const LEGS: readonly LegSpec[] = [
  {
    hip: F(779.5, 201, -12),
    tip: [0.013, 0.013],
    hang: new THREE.Vector3(0.0035, -0.0195, 0.0095),
    dangle: new THREE.Vector3(0.1, -1, 0.3),
    kneeStand: new THREE.Vector3(1, 1.1, 0.6),
    kneeHang: new THREE.Vector3(0.3, 0.1, 1),
  },
  {
    hip: F(780, 207.5, -14.5),
    tip: [0.016, 0.001],
    hang: new THREE.Vector3(0.004, -0.0215, 0.002),
    dangle: new THREE.Vector3(0.12, -1, 0.1),
    kneeStand: new THREE.Vector3(1, 1.2, 0),
    kneeHang: new THREE.Vector3(0.45, 0.2, 0.6),
  },
  {
    hip: F(779.5, 214, -14.5),
    tip: [0.014, -0.011],
    hang: new THREE.Vector3(0.0035, -0.0205, -0.0055),
    dangle: new THREE.Vector3(0.1, -1, -0.25),
    kneeStand: new THREE.Vector3(1, 1.1, -0.6),
    kneeHang: new THREE.Vector3(0.35, 0.2, -0.9),
  },
];

// A leg's segment, running down its own -y from its joint: four-sided,
// tapering, closed at both ends (to a point at a foot's tip).
function sculptSegment(s: Sculpt, length: number, r0: number, r1: number, t: Tone, pointed = false): void {
  loft(
    s,
    [
      { at: new THREE.Vector3(0, 0, 0), w: r0, up: r0 },
      { at: new THREE.Vector3(0, -length, 0), w: r1, up: r1 },
    ],
    { sides: 4, start: 'flat', end: pointed ? r1 * 3 : 'flat', paint: () => t },
  );
}

interface LegRig {
  side: 1 | -1;
  spec: LegSpec;
  hip: THREE.Vector3;
  segments: readonly [femur: THREE.Mesh, tibia: THREE.Mesh, tarsus: THREE.Mesh];
}

// ---------------------------------------------------------------- motion

const HOVER = 0.3; // m, its thorax above the ground as it hovers
const SPEED = 0.45; // m/s flying
const STAND = 0.022; // m, its thorax above what it stands on
const SLOWING = 3; // how fast its speed changes, a second
const DESCENT = 1.5; // s it takes to settle when landing
const REST = 6; // s it rests once landed
const RISE = 1.1; // s it takes to rise back to hovering
const CLEARANCE = 0.06; // m above what it stands on from which its downbeats grow shallower
const HOVER_SPRING = 3.2; // a second: how briskly it gets back to its hover height

// A glide from `from`, going at first at `climb` (m/s, up), to rest at `to`,
// over `time` seconds: where it is `t` seconds in, and how fast it goes (a
// cubic, so a glide that starts while it is moving starts smoothly).
function glide(from: number, climb: number, to: number, time: number, t: number): readonly [number, number] {
  const u = THREE.MathUtils.clamp(t / time, 0, 1);
  const u2 = u * u;
  const u3 = u2 * u;
  const at = (2 * u3 - 3 * u2 + 1) * from + (u3 - 2 * u2 + u) * time * climb + (3 * u2 - 2 * u3) * to;
  const rate = ((6 * u2 - 6 * u) * from + (3 * u2 - 4 * u + 1) * time * climb + (6 * u - 6 * u2) * to) / time;
  return [at, u < 1 ? rate : 0];
}

// How its wings beat: beats a second, and the angle (rad, up from flat) they
// swing about and how far either way.
interface Beat {
  rate: number;
  mid: number;
  amp: number;
}
const HOVERING: Beat = { rate: 2.2, mid: 0.35, amp: 0.95 };
const FLYING: Beat = { rate: 3.4, mid: 0.4, amp: 1.05 };
const SETTLING: Beat = { rate: 2.4, mid: 0.55, amp: 0.75 };
const RESTING: Beat = { rate: 0.32, mid: 0.9, amp: 0.6 };
const RISING: Beat = { rate: 3.6, mid: 0.45, amp: 1.0 };
const HIGHEST = 1.53; // rad, the wings nearly together over its back
// On the upstroke the hind wings trail the forewings, lower than them, as the
// sheet's Flap Up and Fly (Side) draw them: at most this share of the beat's
// swing (under 0.5, so they never dip below the beat's bottom), most of all
// HIND_LAG (rad of the beat) after halfway up, and never above the forewings.
const HIND_TRAIL = 0.45;
const HIND_LAG = 0.6;

// Its body's pitch, nose up (rad).
const IDLE_PITCH = 0.22;
const FLY_PITCH = 0.55; // flying, it goes nose up, as the hero picture and Fly (Side) show it
const REST_PITCH = 0.35; // resting, it sits up, its abdomen hanging under its hind wings as the side view has it

const m1 = new THREE.Matrix4();
const m2 = new THREE.Matrix4();
const v1 = new THREE.Vector3();
const v2 = new THREE.Vector3();
const v3 = new THREE.Vector3();
const v4 = new THREE.Vector3();
const v5 = new THREE.Vector3();
const v6 = new THREE.Vector3();

type Mode = 'idle' | 'fly' | 'land';

export class Butterfly extends ForestAnimal {
  // The sheet's colour variations, the default (monarch) first.
  static readonly COLORS = Object.keys(VARIATIONS);
  static readonly WINGSPAN = WINGSPAN;
  static readonly HOVER = HOVER;
  static readonly SPEED = SPEED;
  static readonly STAND = STAND;
  // How fast its speed eases, a second: flying at `speed`, it goes on about
  // speed / SLOWING before it stops (for a preview that lands it on a spot).
  static readonly SLOWING = SLOWING;

  readonly head = new THREE.Group();
  readonly body: THREE.Mesh;
  readonly abdomen = new THREE.Group();
  readonly leftWing = new THREE.Group();
  readonly rightWing = new THREE.Group();
  readonly antennae: readonly THREE.Group[]; // left, right
  readonly legs: readonly THREE.Group[]; // left front, middle, hind, then the right ones

  // What Land() settles on when not told: m above the ground under it (a
  // flower's top, which a preview sets while it is over one).
  perch = 0;

  private readonly wings: { side: 1 | -1; flap: THREE.Group; fore: THREE.Group; hind: THREE.Group }[];
  private readonly legRigs: LegRig[];
  private mode: Mode = 'idle';
  private modeTime = 0;
  private time = 0;
  private base = HOVER; // m, its thorax's height before its bob and flutter
  private climb = 0; // m/s, how fast that height changes
  private landFrom = HOVER;
  private landClimb = 0;
  private landAt = 0; // m, what it last settled on (or is settling on)
  private surface = 0; // m, what is under it: the ground, or what it has just left
  private stand = 0; // 0 legs hanging, 1 standing
  private flutter = 0;
  private drift = 1;
  private rate = HOVERING.rate;
  private mid = HOVERING.mid;
  private amp = HOVERING.amp;
  private phase = Math.acos(-HOVERING.mid / HOVERING.amp); // first shown with its wings spread flat
  private pitch = IDLE_PITCH;
  private roll = 0;
  private droop = 0.05;
  private turn = 0;
  private heading = 0;

  constructor(options: ForestAnimalOptions = {}) {
    super(VARIATIONS, options);
    const random = seededRandom(options.seed ?? 11);
    const palette = this.palette;
    const material = coat();
    const shine = gloss();
    const sheet = thin(0.72);

    // The thorax.
    const thorax = new Sculpt(random);
    sculptThorax(thorax);
    this.body = this.painting(rigidMesh(thorax.geometry(palette), material));

    // The head on the neck, with its eyes and antennae.
    const headSculpt = new Sculpt(random);
    const eyes = new Sculpt(random, 0.01);
    sculptHead(headSculpt, eyes);
    this.head.position.copy(NECK);
    this.head.add(this.painting(rigidMesh(headSculpt.geometry(palette), material)), this.painting(rigidMesh(eyes.geometry(palette), shine)));
    this.antennae = ([-1, 1] as const).map((side) => {
      const s = new Sculpt(random, 0.02);
      sculptAntenna(s, side);
      const pivot = new THREE.Group();
      pivot.position.copy(F(MIDDLE + side * ANTENNA_ROOT.x, ANTENNA_ROOT.y, ANTENNA_ROOT.h)).sub(NECK);
      pivot.add(this.painting(rigidMesh(s.geometry(palette), material)));
      this.head.add(pivot);
      return pivot;
    });

    // The abdomen, hanging from the waist.
    const abdomenSculpt = new Sculpt(random);
    sculptAbdomen(abdomenSculpt);
    this.abdomen.position.copy(WAIST);
    this.abdomen.add(this.painting(rigidMesh(abdomenSculpt.geometry(palette), material)));

    // The wings: each side flaps on its hinge; within it the forewing and
    // the hind wing each turn back or forward on their roots.
    const flapAt = (side: number) => F(MIDDLE + side * HINGE_X, FLAP_Y, HINGE_H);
    this.wings = ([-1, 1] as const).map((side) => {
      const flap = side < 0 ? this.leftWing : this.rightWing;
      flap.position.copy(flapAt(side));
      const pivots = ([
        [FORE, FORE_ROOT_Y, FORE_LIFT],
        [HIND, HIND_ROOT_Y, 0],
      ] as const).map(([trace, rootY, lift]) => {
        const pivot = new THREE.Group();
        const at = F(MIDDLE + side * HINGE_X, rootY, HINGE_H);
        pivot.position.copy(at).sub(flapAt(side));
        const s = new Sculpt(random, 0.03);
        sculptWing(s, trace, side, lift, at);
        pivot.add(this.painting(rigidMesh(s.geometry(palette), sheet)));
        flap.add(pivot);
        return pivot;
      });
      return { side, flap, fore: pivots[0], hind: pivots[1] };
    });

    // The legs: three segments each, shared by all six, set every frame.
    const segment = (length: number, r0: number, r1: number, t: Tone, pointed = false) => {
      const s = new Sculpt(random, 0.02);
      sculptSegment(s, length, r0, r1, t, pointed);
      return s.geometry(palette);
    };
    const femur = segment(FEMUR, 0.00055, 0.00042, blend('body', 'tan', 0.22));
    const tibia = segment(TIBIA, 0.00042, 0.0003, tone('body', 0.92));
    const tarsus = segment(TARSUS, 0.0003, 0.00015, tone('body', 0.8), true);
    this.legRigs = [];
    this.legs = ([-1, 1] as const).flatMap((side) =>
      LEGS.map((spec) => {
        const leg = new THREE.Group();
        const segments = [femur, tibia, tarsus].map((geometry) => rigidMesh(geometry, material)) as [THREE.Mesh, THREE.Mesh, THREE.Mesh];
        if (this.legRigs.length === 0) segments.forEach((mesh) => this.painting(mesh)); // the geometries are shared: repainting one leg repaints them all
        leg.add(...segments);
        this.legRigs.push({ side, spec, hip: new THREE.Vector3(side * Math.abs(spec.hip.x), spec.hip.y, spec.hip.z), segments });
        return leg;
      }),
    );

    this.rig.add(this.body, this.head, this.abdomen, this.leftWing, this.rightWing, ...this.legs);
    this.pose();
  }

  // Hovers where it is, its wings beating gently between the sheet's Flap Up
  // (nearly together over its back) and Flap Down (pressed down past its
  // sides), bobbing a little with each beat. Named as in its sheet's poses.
  Idle(): void {
    this.setMode('idle');
  }

  // Flies forward the way it faces until told otherwise, beating faster and
  // further, on a fluttering path that rises and falls. Named as in its
  // sheet's poses ("Fly (Side)"). Steer it by turning it.
  Fly(): void {
    this.setMode('fly');
  }

  // Settles down onto what is `height` m above the ground under it (the
  // ground itself unless a preview has set `perch`, such as a flower's top),
  // its legs reaching down to stand; rests there, opening and closing its
  // wings slowly, then rises back to hovering. Named as in its sheet's poses
  // ("Land on Flower").
  Land(height = this.perch): void {
    this.mode = 'land';
    this.modeTime = 0;
    this.landAt = Math.max(0, height);
    this.surface = this.landAt;
    // It glides down from where it is, going on as it was going at first,
    // but never so fast downward that it would overshoot into what it lands on.
    this.landFrom = this.base;
    this.landClimb = Math.max(this.climb, (-2.5 * Math.max(0, this.base - this.landAt - STAND)) / DESCENT);
  }

  // What it is doing: its behaviour, from the sheet's poses.
  get doing(): Mode {
    return this.mode;
  }

  private setMode(mode: Mode): void {
    if (mode !== this.mode) this.modeTime = 0;
    this.mode = mode;
  }

  update(delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    this.time += dt;
    this.modeTime += dt;
    // How fast it is being turned (the preview steers it), to bank into it.
    if (dt > 0) this.turn = ease(this.turn, THREE.MathUtils.clamp((this.rotation.y - this.heading) / dt, -8, 8), 5, dt);
    this.heading = this.rotation.y;

    // What it is doing now.
    let speed = 0;
    let beat = HOVERING;
    let pitch = IDLE_PITCH;
    let stand = 0;
    let flutter = 0;
    let drift = 0;
    let droop = 0.05;
    let planned: readonly [height: number, climb: number] | null = null;
    if (this.mode === 'idle') drift = 1;
    else if (this.mode === 'fly') {
      speed = SPEED;
      beat = FLYING;
      pitch = FLY_PITCH;
      flutter = 1;
      droop = 0.1;
    } else {
      const t = this.modeTime;
      const top = this.landAt + STAND;
      if (t < DESCENT) {
        const u = t / DESCENT;
        planned = glide(this.landFrom, this.landClimb, top, DESCENT, t);
        beat = SETTLING;
        pitch = 0.2;
        stand = smooth((u - 0.4) / 0.6);
      } else if (t < DESCENT + REST) {
        planned = [top, 0];
        beat = RESTING;
        pitch = REST_PITCH;
        stand = 1;
        droop = 0.45;
      } else if (t < DESCENT + REST + RISE) {
        const u = (t - DESCENT - REST) / RISE;
        planned = glide(top, 0, HOVER, RISE, t - DESCENT - REST);
        beat = RISING;
        pitch = 0.25;
        stand = 1 - smooth(u * 2.5);
      } else {
        this.Idle(); // back to hovering, up where it has risen to
        drift = 1;
      }
    }

    this.speedNow = ease(this.speedNow, speed, SLOWING, dt);
    this.advance(dt);
    if (planned) [this.base, this.climb] = planned;
    else {
      // Free, it springs back to its hover height, smoothly from however it
      // was going (critically damped).
      this.climb += (HOVER_SPRING * HOVER_SPRING * (HOVER - this.base) - 2 * HOVER_SPRING * this.climb) * dt;
      this.base += this.climb * dt;
    }
    // Once well clear of what it stood on, the ground is what is under it.
    if (this.mode !== 'land' && this.base > this.surface + STAND + 0.08) this.surface = 0;
    this.stand = ease(this.stand, stand, 7, dt);
    this.flutter = ease(this.flutter, flutter, 2, dt);
    this.drift = ease(this.drift, drift, 2, dt);
    this.rate = ease(this.rate, beat.rate, 3, dt);
    this.mid = ease(this.mid, beat.mid, 3, dt);
    this.amp = ease(this.amp, beat.amp, 3, dt);
    this.pitch = ease(this.pitch, pitch, 3, dt);
    this.droop = ease(this.droop, droop, 3, dt);
    this.roll = ease(this.roll, THREE.MathUtils.clamp(-this.turn * this.speedNow * 0.4, -0.45, 0.45), 4, dt);
    this.phase = (this.phase + TAU * this.rate * dt) % TAU;
    this.pose();
  }

  // Puts every part where the state has it this frame.
  private pose(): void {
    // Near what it stands on, its downbeats grow shallower, so its wings
    // never beat into it.
    const above = this.base - this.surface - STAND;
    const near = 1 - smooth(above / CLEARANCE);
    const lowest = THREE.MathUtils.lerp(-0.75, 0.15, near);
    let mid = this.mid;
    let amp = this.amp;
    if (mid - amp < lowest) {
      amp = Math.max(0, (mid + amp - lowest) / 2);
      mid = lowest + amp;
    }
    const flap = Math.min(HIGHEST, mid + amp * Math.cos(this.phase));
    // Up and back, down and forward (spread flat, they are as the sheet's
    // front view draws them); resting, the forewings draw back into the hind
    // wings as they close, as the sheet's side view shows them.
    const free = 1 - this.stand;
    const rising = Math.max(0, -Math.sin(this.phase - HIND_LAG));
    const trail = HIND_TRAIL * amp * rising * rising * free;
    const closed = this.stand * smooth((flap - 0.3) / 1.2);
    const foreSweep = 0.15 * flap * free + 0.85 * closed;
    const hindSweep = 0.05 * flap * free + 0.62 * closed;
    for (const { side, flap: hinge, fore, hind } of this.wings) {
      hinge.rotation.z = side * flap;
      fore.rotation.y = side * foreSweep;
      hind.rotation.set(0, side * hindSweep, -side * trail);
    }

    // The body rises with each downbeat and falls back as the wings lift,
    // and hovers or flutters on its way.
    const t = this.time;
    const bob = -0.004 * Math.cos(this.phase) * free * Math.min(1, amp);
    const wave =
      this.flutter * (0.02 * Math.sin(TAU * 0.75 * t) + 0.008 * Math.sin(TAU * 1.9 * t + 0.7)) + this.drift * (0.006 * Math.sin(0.9 * t) + 0.003 * Math.sin(2.3 * t + 1));
    this.rig.position.set(0, this.base + bob + wave, 0);
    this.rig.rotation.set(-(this.pitch + 0.05 * Math.cos(this.phase) * free), 0, this.roll);
    this.abdomen.rotation.x = -(this.droop + 0.1 * Math.cos(this.phase) * free);
    this.antennae.forEach((antenna, i) => {
      const side = i === 0 ? -1 : 1;
      antenna.rotation.set(0.05 * Math.sin(1.7 * t + side) - 0.12 * this.flutter, side * 0.04 * Math.sin(1.1 * t), 0);
    });
    this.rig.updateMatrix();
    this.placeLegs();
  }

  // Each leg reaches its ankle by two-bone inverse kinematics: hanging under
  // it in flight, or standing, its foot on what it has landed on, as far
  // along between the two as it stands.
  private placeLegs(): void {
    const toRig = m1.copy(this.rig.matrix).invert();
    const s = this.stand;
    for (const { side, spec, hip, segments } of this.legRigs) {
      // Standing: the foot's tip on what it stands on, beside where it
      // would be with its body level; the foot sloping down to it.
      const tipX = side * spec.tip[0];
      const tipZ = spec.tip[1];
      const outX = tipX - hip.x;
      const outZ = tipZ - hip.z;
      const flat = Math.hypot(outX, outZ);
      const cos = Math.cos(TARSUS_SLOPE);
      const footWorld = v1.set((outX / flat) * cos, -Math.sin(TARSUS_SLOPE), (outZ / flat) * cos);
      const tip = v2.set(tipX, this.landAt + TOE, tipZ).applyMatrix4(toRig);
      const standFoot = footWorld.transformDirection(toRig);
      const standAnkle = tip.addScaledVector(standFoot, -TARSUS);
      // Flying: hanging where it is set.
      const hangAnkle = v3.set(side * spec.hang.x, spec.hang.y, spec.hang.z);
      const ankle = hangAnkle.lerp(standAnkle, s);
      const foot = v4.set(side * spec.dangle.x, spec.dangle.y, spec.dangle.z).normalize().lerp(standFoot, s).normalize();
      const hint = new THREE.Vector3(side * spec.kneeHang.x, spec.kneeHang.y, spec.kneeHang.z)
        .normalize()
        .lerp(new THREE.Vector3(side * spec.kneeStand.x, spec.kneeStand.y, spec.kneeStand.z).normalize(), s);
      // The knee, and the ankle as far toward its mark as the leg reaches.
      const toAnkle = ankle.clone().sub(hip);
      const d = THREE.MathUtils.clamp(toAnkle.length(), Math.abs(FEMUR - TIBIA) + 1e-5, (FEMUR + TIBIA) * 0.999);
      const way = toAnkle.normalize();
      const cosA = (FEMUR * FEMUR + d * d - TIBIA * TIBIA) / (2 * FEMUR * d);
      const out = hint.addScaledVector(way, -hint.dot(way));
      if (out.lengthSq() < 1e-10) out.set(side, 0, 0).addScaledVector(way, -way.x * side);
      out.normalize();
      const knee = hip.clone().addScaledVector(way, FEMUR * cosA).addScaledVector(out, FEMUR * Math.sqrt(Math.max(0, 1 - cosA * cosA)));
      const reached = hip.clone().addScaledVector(way, d);
      // A foot never reaches into what it stands on: tipped up if it would
      // (as it swings from hanging to standing).
      const ankleUp = v5.copy(reached).applyMatrix4(this.rig.matrix);
      const footUp = v6.copy(foot).transformDirection(this.rig.matrix);
      const floor = this.landAt + TOE;
      if (ankleUp.y + footUp.y * TARSUS < floor) {
        const drop = THREE.MathUtils.clamp((floor - ankleUp.y) / TARSUS, -1, 1);
        const level = Math.hypot(footUp.x, footUp.z) || 1;
        const across = Math.sqrt(1 - drop * drop);
        foot.set((footUp.x / level) * across, drop, (footUp.z / level) * across).transformDirection(toRig);
      }
      const [femur, tibia, tarsus] = segments;
      orient(femur, hip, knee.clone().sub(hip));
      orient(tibia, knee, reached.clone().sub(knee));
      orient(tarsus, reached, foot);
    }
  }
}

// Puts a leg's segment at its joint, running down its own -y along `way`,
// turned about it so that its faces keep still as it swings.
function orient(mesh: THREE.Object3D, at: THREE.Vector3, way: THREE.Vector3): void {
  const y = way.clone().normalize().negate();
  const x = new THREE.Vector3(0, 0, 1).cross(y);
  if (x.lengthSq() < 1e-8) x.set(1, 0, 0).cross(y);
  x.normalize();
  const z = new THREE.Vector3().crossVectors(x, y);
  mesh.position.copy(at);
  mesh.quaternion.setFromRotationMatrix(m2.makeBasis(x, y, z));
}
