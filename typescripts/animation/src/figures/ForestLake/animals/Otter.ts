import * as THREE from 'three';
import { keepSharp } from '../../../effects/kuwahara';
import type { ForestAnimalOptions } from './ForestAnimal';
import { FourLegged, legRings, type Foot, type FourLeggedBuild, type FreeFoot, type Gait, type Posture } from './FourLegged';
import { blend, bump, ease, ellipsoid, eye, loft, paint, patchy, smooth, tone, tubeRings, type Palette, type Sculpt, type Tone } from './parts';

// The forest lake's otter (Otter.md), as its reference sheet draws it: a
// Eurasian river otter of flat facets, long, low and sleek, a soft greyish
// brown with a darker crown, a cream muzzle, cheeks and chin running down its
// throat and chest as a bib, and a pale patch on its belly; a broad flat head
// with small round ears, big glossy dark eyes with a glint, a broad black nose
// over puffed whisker pads, a smiling mouth and long pale whiskers; short
// thick legs, dark below the elbows and knees, on big dark paws of four toes;
// and a long, thick tail tapering to a point. Its colours are the sheet's five
// variations.
//
// On land it walks and bounds, sits up on its haunches (its Idle) and stands
// up on its hind legs. In the water (afloat) it swims low at the surface,
// paddling, dives headfirst under and comes back up, and rears up out of the
// water to look about.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground under
// its middle. Afloat, the water's surface is y = 0 of its parent (or
// `surface`) and its origin sits SINK under it. Traced from the sheet's side
// and front views in their pixels (U m each): 97 cm from nose to tail tip and
// 34 cm at the shoulder, a Eurasian otter's length in the sheet's proportions.

type V3 = THREE.Vector3;

const U = 0.0026; // m per pixel of the sheet's side view: its 373 px from nose to tail tip make 97 cm
const FLOOR = 302; // the side view's ground line, in its image pixels
const MIDDLE = 960; // the side view's pixel column over the origin, between the fore and hind feet

// A point traced from the sheet's side view, in its image pixels: `ix` along
// it (the head to the left, so a smaller one is further forward), `iy` down
// it, and `x` pixels to the otter's right (+x; widths from the front view,
// scaled to the side view's pixels).
function P(ix: number, iy: number, x = 0): V3 {
  return new THREE.Vector3(x * U, (FLOOR - iy) * U, (MIDDLE - ix) * U);
}

// The sheet's five colour variations, picked from its pictures (the lit
// faces of its front and side views for the default, and the others'
// thumbnails measured against it). Its roles: the coat, darker patches and
// the crown, the cream bib and muzzle, the darker lower legs (sock) and paws,
// the ears' hollows, the nose, the eyes, the whiskers and the mouth's line.
const VARIATIONS: Record<string, Palette> = {
  brown: { coat: 0xab7a6a, dark: 0x80594f, cream: 0xefd2bf, sock: 0x584039, paws: 0x3c2b27, inner: 0x44302b, nose: 0x2b2224, lid: 0x2e1f1b, iris: 0x3d2520, pupil: 0x120c0b, shine: 0xffffff, whisker: 0xf1ebe6, mouth: 0x4b3029 },
  dark: { coat: 0xad765c, dark: 0x74493a, cream: 0xebcfbc, sock: 0x4f372e, paws: 0x352520, inner: 0x3a2620, nose: 0x221a1b, lid: 0x251814, iris: 0x33201a, pupil: 0x120c0b, shine: 0xffffff, whisker: 0xefe8e2, mouth: 0x3a241e },
  light: { coat: 0xd89c80, dark: 0xb07c64, cream: 0xf2dccd, sock: 0x8a6454, paws: 0x6a4a40, inner: 0x7a5446, nose: 0x6d4b43, lid: 0x4a3129, iris: 0x5e3a2c, pupil: 0x160e0b, shine: 0xffffff, whisker: 0xf6f0ea, mouth: 0x7a4e40 },
  gray: { coat: 0xa8897e, dark: 0x76625d, cream: 0xe4d6d0, sock: 0x5f5352, paws: 0x4a4144, inner: 0x3f3638, nose: 0x2a2528, lid: 0x2a2224, iris: 0x3a2e2c, pupil: 0x120e0e, shine: 0xffffff, whisker: 0xf2eeec, mouth: 0x4a3d3c },
  albino: { coat: 0xf6e2d8, dark: 0xe4c9bd, cream: 0xfcf0e8, sock: 0xf1dad0, paws: 0xefd5ca, inner: 0xdc9894, nose: 0xe39a96, lid: 0xd9a19b, iris: 0xb4565a, pupil: 0x6e2228, shine: 0xffffff, whisker: 0xffffff, mouth: 0xcf9d95 },
};

// Walking: the four-beat walk, the body held low; bounding: the hind feet
// together, then the fore, the back flexing hard, as the sheet's Run.
const WALK: Gait = {
  stride: 0.12,
  cadence: 1.7,
  duty: 0.64,
  offsets: [0.25, 0.75, 0, 0.5],
  lift: [0.035, 0.03],
  fold: [0.4, 0.45],
  sink: 0.014,
  bob: 0.004,
  bounces: 2,
  rock: 0.02,
  flex: 0.03,
  nod: 0.04,
  lag: 0.1,
};

const RUN: Gait = {
  stride: 0.2,
  cadence: 3,
  duty: 0.32,
  offsets: [0.5, 0.56, 0, 0.06],
  lift: [0.05, 0.05],
  fold: [0.9, 0.8],
  sink: 0.035,
  bob: 0.018,
  bounces: 1,
  rock: 0.07,
  flex: 0.18,
  nod: 0.05,
  lag: 0.35,
};

const UPRIGHT = new THREE.Vector3(0, 0, 1); // the torso's slices stand upright
const NECK = new THREE.Vector3(0, 0.85, 0.53).normalize(); // the neck's way, up and forward from the chest
const SHOULDER = new THREE.Vector3().addVectors(UPRIGHT, NECK).normalize();

// The torso and neck, rump to the neck's end inside the head, traced from the
// side view (the middle of each slice, and how far up and down it reaches)
// and the front and back views (how far across): upright slices along the
// back, then slices square to the neck, which rises steeply to the head.
const BODY: { ix: number; iy: number; w: number; up: number; down: number; normal?: V3 }[] = [
  { ix: 1092, iy: 258, w: 26, up: 20, down: 20 },
  { ix: 1080, iy: 251.5, w: 38, up: 29.5, down: 26 },
  { ix: 1065, iy: 239, w: 46, up: 38, down: 38 },
  { ix: 1045, iy: 229, w: 50, up: 42, down: 42 },
  { ix: 1030, iy: 224.5, w: 50, up: 43.5, down: 43 },
  { ix: 1010, iy: 220.5, w: 48.5, up: 44.5, down: 44 },
  { ix: 985, iy: 218.5, w: 46, up: 44.5, down: 44.5 },
  { ix: 962, iy: 218, w: 44, up: 44.5, down: 45 },
  { ix: 945, iy: 217.5, w: 43, up: 45, down: 45.5 },
  { ix: 919, iy: 214, w: 41, up: 43, down: 46, normal: SHOULDER },
  { ix: 897, iy: 188, w: 37, up: 33, down: 36, normal: NECK },
  { ix: 879, iy: 161, w: 33, up: 26, down: 20, normal: NECK },
];

// The head's slices, from the back of the skull to the tip of the muzzle:
// the right half of each, from the top down to the chin, (x, iy) in the side
// view's pixels. Narrow over the brow, where the eyes sit on its upper front,
// and broad at the cheeks and the puffed whisker pads, as the front view
// draws it.
const HEAD: [ix: number, half: [x: number, iy: number][]][] = [
  [906, [[0, 136], [17, 137], [27, 144], [28, 156], [17, 163], [0, 165]]],
  [894, [[0, 129], [29, 131], [37, 141], [38, 156], [28, 167], [0, 171]]],
  [878, [[0, 124], [29, 126], [36, 138], [38, 156], [34, 169], [0, 175]]],
  [862, [[0, 123], [20, 125], [22, 137], [35, 153.5], [33, 168.5], [0, 175.5]]],
  [848, [[0, 124.5], [12, 126], [18, 139], [32, 152.5], [29, 168], [0, 173]]],
  [835, [[0, 128.5], [9, 130.5], [15, 142], [29, 152], [25, 166], [0, 169.5]]],
  [825, [[0, 140], [7, 141.5], [12, 148], [20, 157], [15, 161], [0, 162.5]]],
];

// The broad black nose on the muzzle's tip: flat and wide on top, narrowing
// below, as the front view draws it.
const NOSE: [ix: number, half: [x: number, iy: number][]][] = [
  [835, [[0, 140], [8.5, 140.5], [12, 144.5], [8, 150.5], [0, 152.5]]],
  [827, [[0, 138.5], [11.5, 139.5], [14.5, 144], [9.5, 151.5], [0, 155]]],
  [820, [[0, 140], [9, 141], [11.5, 145], [7.5, 151], [0, 154]]],
];

// A steady "random" number in -1..1 for ring r's corner k.
function wobble(r: number, k: number): number {
  const h = Math.sin(r * 91.7 + k * 47.3 + 3.1) * 24634.6345;
  return 2 * (h - Math.floor(h)) - 1;
}

// A ring of a slice's corners: its right half as given, then the left half
// mirrored, round from the top.
function ring(ix: number, half: readonly (readonly [number, number])[]): V3[] {
  const right = half.map(([x, iy]) => P(ix, iy, x));
  const left = half
    .slice(1, -1)
    .reverse()
    .map(([x, iy]) => P(ix, iy, -x));
  return [...right, ...left];
}

// A closed surface through rings of corners given one by one (as many in
// each, in order round it), joined by flat faces facing away from the line
// through the rings' middles, each end closed to a point: its middle
// ('flat') or one given. `painter(ring, corner)` colours the face from ring
// r to r + 1 that starts at corner k; the start's cap is ring -1 and the
// end's the last ring's index.
function hull(s: Sculpt, rings: readonly (readonly V3[])[], painter: (ring: number, corner: number) => Tone, start: V3 | 'flat', end: V3 | 'flat'): void {
  const n = rings[0].length;
  const centres = rings.map((r) => r.reduce((sum, p) => sum.add(p), new THREE.Vector3()).divideScalar(r.length));
  const middle = new THREE.Vector3();
  for (let r = 0; r + 1 < rings.length; r++) {
    middle.addVectors(centres[r], centres[r + 1]).multiplyScalar(0.5);
    for (let k = 0; k < n; k++) {
      const k1 = (k + 1) % n;
      const [a, b, c, d] = [rings[r][k], rings[r][k1], rings[r + 1][k1], rings[r + 1][k]];
      const t = painter(r, k);
      if ((r + k) % 2 === 0) {
        s.facing(a, b, c, middle, t);
        s.facing(a, c, d, middle, t);
      } else {
        s.facing(a, b, d, middle, t);
        s.facing(b, c, d, middle, t);
      }
    }
  }
  const last = rings.length - 1;
  const cap = (r: number, next: number, tip: V3 | 'flat', index: number) => {
    const point = tip === 'flat' ? centres[r] : tip;
    const inside = centres[r].clone().lerp(centres[next], 0.25);
    for (let k = 0; k < n; k++) s.facing(rings[r][k], rings[r][(k + 1) % n], point, inside, painter(index, k));
  };
  cap(0, 1, start, -1);
  cap(last, last - 1, end, last);
}

// A small round ear (an otter's): most of a disc standing out from the head,
// its rim in the coat round a dark hollow, as the sheet's close-up draws it,
// `radius` round `centre`, its hollow facing `facing`, `thick` deep.
function roundEar(s: Sculpt, centre: V3, facing: V3, up: V3, radius: number, thick: number, outer: Tone, inner: Tone): void {
  const f = facing.clone().normalize();
  const u = up.clone().addScaledVector(f, -up.dot(f)).normalize();
  const a = new THREE.Vector3().crossVectors(u, f).normalize();
  const n = 8;
  const from = -0.75; // the arc round it, open at the foot where it sits on the head
  const to = Math.PI + 0.75;
  const at = (angle: number, r: number, depth: number, lift = 0) =>
    centre
      .clone()
      .addScaledVector(a, Math.cos(angle) * r)
      .addScaledVector(u, Math.sin(angle) * r + lift)
      .addScaledVector(f, -depth);
  const rim: V3[] = [];
  const lip: V3[] = [];
  const back: V3[] = [];
  for (let i = 0; i <= n; i++) {
    const angle = from + ((to - from) * i) / n;
    rim.push(at(angle, radius, 0));
    lip.push(at(angle, radius * 0.6, thick * 0.3, radius * 0.08));
    back.push(at(angle, radius * 0.92, thick));
  }
  const hollow = at(0, 0, thick * 0.7, radius * 0.1);
  const behind = at(0, 0, thick * 1.2);
  const out = new THREE.Vector3();
  const backward = f.clone().negate();
  for (let i = 0; i < n; i++) {
    const m = from + ((to - from) * (i + 0.5)) / n;
    out.copy(a).multiplyScalar(Math.cos(m)).addScaledVector(u, Math.sin(m));
    s.toward(rim[i], rim[i + 1], lip[i + 1], f, outer);
    s.toward(rim[i], lip[i + 1], lip[i], f, outer);
    s.toward(lip[i], lip[i + 1], hollow, f, inner);
    s.toward(rim[i], back[i], back[i + 1], out, outer);
    s.toward(rim[i], back[i + 1], rim[i + 1], out, outer);
    s.toward(back[i], behind, back[i + 1], backward, outer);
  }
  // Closed across its foot.
  const down = u.clone().negate();
  s.toward(rim[n], lip[n], lip[0], f, outer);
  s.toward(rim[n], lip[0], rim[0], f, outer);
  s.toward(lip[n], hollow, lip[0], f, inner);
  s.toward(rim[0], back[0], back[n], down, outer);
  s.toward(rim[0], back[n], rim[n], down, outer);
  s.toward(back[n], behind, back[0], backward, outer);
}

// An otter's paw on a foot's joint `at`, its sole on the ground (y 0): a
// broad flat palm from `heel` px behind the joint to `length` px ahead of it,
// `width` px either side, and four thick toes fanned out ahead of it, `toe`
// px long, dark, as the sheet's close-up draws them.
function paw(s: Sculpt, at: V3, side: number, heel: number, length: number, width: number, toe: number): void {
  const h = at.y / U; // the joint's height, px
  const across = new THREE.Vector3(side, 0, 0);
  const palm = [
    { z: -heel, y: h * 0.62, w: width * 0.6, up: h * 0.38, down: h * 0.5 },
    { z: 0, y: h * 0.52, w: width * 0.82, up: h * 0.5, down: h * 0.45 },
    { z: length * 0.55, y: h * 0.45, w: width, up: h * 0.42, down: h * 0.38 },
    { z: length, y: h * 0.45, w: width * 0.9, up: h * 0.33, down: h * 0.3 },
  ].map((r) => ({ at: new THREE.Vector3(at.x, r.y * U, at.z + r.z * U), w: r.w * U, up: r.up * U, down: r.down * U, n: 2.5 }));
  loft(s, palm, {
    sides: 8,
    turn: Math.PI / 8,
    side: across,
    mirror: side < 0,
    start: 'flat',
    end: 'flat',
    paint: (_r, angle) => (Math.abs(angle - Math.PI) < 0.8 ? tone('paws', 0.7) : tone('paws')), // the sole darker
  });
  for (let i = 0; i < 4; i++) {
    const u = (i - 1.5) / 1.5; // -1 .. 1 across the paw
    const spread = u * 0.24;
    const way = new THREE.Vector3(Math.sin(spread), -0.12, Math.cos(spread)).normalize();
    const root = new THREE.Vector3(at.x + u * width * 0.68 * U, 4.2 * U, at.z + (length - 2.5 - Math.abs(u) * 2) * U);
    const rings = [0, 0.55, 1].map((f, j) => ({
      at: root.clone().addScaledVector(way, f * toe * U),
      w: [3.6, 3.2, 2.1][j] * U,
      up: [3, 2.6, 1.7][j] * U,
      down: [2.9, 2.5, 1.6][j] * U,
    }));
    loft(s, rings, { sides: 4, turn: Math.PI / 4, side: across, mirror: side < 0, start: 'flat', end: 1.6 * U, paint: (r) => (r >= 2 ? tone('paws', 0.62) : tone('paws', 1.06)) });
  }
}

// Long pale whiskers fanned out and back from the whisker pads, each a line
// of two segments drooping a little, kept sharp through the painterly filter
// (lines a pixel wide, as the sheet draws them).
function whiskers(palette: Palette): THREE.LineSegments {
  // Each: its root on the right pad (x, iy, ix), its way out (to the right,
  // up, forward: they sweep back) and its length, px.
  const list: [number, number, number, number, number, number, number][] = [
    [28, 154, 836, 0.8, 0.12, -0.45, 30],
    [30, 156.5, 835, 0.82, 0, -0.5, 34],
    [30, 159, 836, 0.8, -0.1, -0.55, 33],
    [28, 161.5, 838, 0.75, -0.2, -0.6, 30],
    [25, 164, 841, 0.68, -0.3, -0.65, 26],
  ];
  const positions: number[] = [];
  const tones: Tone[] = [];
  for (const side of [-1, 1]) {
    for (const [x, iy, ix, dx, dy, dz, length] of list) {
      const root = P(ix, iy, side * x);
      const way = new THREE.Vector3(side * dx, dy, dz).normalize();
      const mid = root.clone().addScaledVector(way, length * 0.5 * U);
      mid.y -= 1 * U;
      const end = root.clone().addScaledVector(way, length * U);
      end.y -= 3 * U;
      positions.push(...root.toArray(), ...mid.toArray(), ...mid.toArray(), ...end.toArray());
      for (let i = 0; i < 4; i++) tones.push(tone('whisker'));
    }
  }
  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geometry.setAttribute('color', new THREE.Float32BufferAttribute(new Float32Array(tones.length * 3), 3));
  geometry.userData.tones = tones;
  paint(geometry, palette);
  const material = new THREE.LineBasicMaterial({ vertexColors: true });
  keepSharp(material);
  const lines = new THREE.LineSegments(geometry, material);
  lines.name = 'whiskers';
  return lines;
}

const BUILD: FourLeggedBuild = {
  variations: VARIATIONS,
  pelvis: P(1042, 222),
  chest: P(919, 214),
  neck: P(900, 192),
  head: P(887, 163),
  body(s: Sculpt) {
    loft(
      s,
      BODY.map((r) => ({ at: P(r.ix, r.iy), w: r.w * U, up: r.up * U, down: r.down * U, normal: r.normal ?? UPRIGHT })),
      {
        sides: 12,
        start: 7 * U,
        end: 'flat',
        // Each corner a little in or out, so the facets vary as the sheet's do.
        shape: (r, _angle, k) => 1 + 0.045 * wobble(r, k),
        paint: (r, angle, k) => {
          const below = Math.min(angle, 2 * Math.PI - angle); // 0 along the top, π underneath
          if (r >= 10 && below > 1.7) return tone('cream'); // the throat
          if (r === 9 && below > 1.9) return tone('cream'); // the bib down the chest
          if (r === 8 && below > 2.3) return tone('cream'); // its tip between the fore legs
          if (r >= 5 && r <= 7 && below > 2.1) return blend('cream', 'coat', 0.12); // the pale patch on the belly
          return patchy('coat', 'dark', r, k, 0.35, 0.3);
        },
      },
    );
  },
  buildHead(s: Sculpt, shiny: Sculpt) {
    // The head: a darker crown and nose bridge, brown sides, the cream
    // cheeks, muzzle and chin rising toward the nose.
    hull(
      s,
      HEAD.map(([ix, half]) => ring(ix, half)),
      (r, k) => {
        const band = k <= 4 ? k : 9 - k; // 0 the crown, 1 the upper side, 2 the side, 3 the lower side, 4 the chin
        if (r < 0) return tone('coat'); // the back of the skull, in the neck
        if (r >= HEAD.length - 1) return band === 0 ? tone('coat') : tone('cream'); // the upper lip under the nose
        if (band === 0) return blend('coat', 'dark', r >= 4 ? 0.35 : 0.6);
        if (band === 4) return tone('cream');
        if (band === 3) return r >= 1 ? tone('cream') : patchy('coat', 'dark', r, k);
        if (band === 2) return r >= 5 ? tone('cream') : patchy('coat', 'dark', r, k);
        return r >= 5 ? blend('coat', 'cream', 0.3) : patchy('coat', 'dark', r, k, 0.3);
      },
      'flat',
      P(820, 155),
    );
    // The puffed whisker pads either side under the nose.
    for (const side of [-1, 1]) {
      ellipsoid(s, P(831, 158, side * 15), new THREE.Vector3(16, 10, 11.5).multiplyScalar(U), new THREE.Matrix4(), 3, 7, () => tone('cream'));
    }
    // Big round dark eyes on the sloping front of the brow, looking forward
    // and out, a glint in each, seated where a ray toward the head meets it.
    for (const side of [-1, 1]) {
      const look = new THREE.Vector3(side * 0.58, 0.2, 0.79).normalize();
      const at = s.onto(P(863, 140, side * 22).addScaledVector(look, 25 * U), look.clone().negate()).addScaledVector(look, 1.2 * U);
      eye(shiny, at, look, new THREE.Vector3(0, 1, 0), 7.2 * U, { lid: tone('lid'), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 9, 0.62, 0.6);
    }
    // Round ears at the back corners of the head, their dark hollows out
    // and forward.
    for (const side of [-1, 1]) {
      const surface = s.onto(P(898, 143, side * 60), new THREE.Vector3(-side, 0, 0));
      const centre = surface.add(new THREE.Vector3(side * 3 * U, 0, 0));
      roundEar(s, centre, new THREE.Vector3(side * 0.7, 0.15, 0.7), new THREE.Vector3(side * 0.15, 1, -0.1), 11.5 * U, 6 * U, patchy('coat', 'dark', 2, side + 5, 0.4), tone('inner'));
    }
    // The mouth: a thin dark line from under the nose, round the whisker
    // pads and back along the muzzle's sides to its corners below the eyes,
    // as the sheet draws its smile; and the line from the nose down to it.
    const onHead = (x: number, iy: number, ix: number) => {
      const way = new THREE.Vector3(-Math.sign(x) * 0.8, 0, -0.6 * (1 - Math.abs(x) / 25)).normalize();
      if (x === 0) way.set(0, 0, -1);
      const from = P(ix, iy, x).addScaledVector(way, -30 * U);
      return s.onto(from, way).addScaledVector(way, -0.35 * U);
    };
    const line = (p: V3, q: V3, thick: V3) => {
      const out = p.clone().add(q).multiplyScalar(0.5).sub(P(860, 160));
      s.toward(p.clone().sub(thick), q.clone().sub(thick), q.clone().add(thick), out, tone('mouth'));
      s.toward(p.clone().sub(thick), q.clone().add(thick), p.clone().add(thick), out, tone('mouth'));
    };
    const thin = new THREE.Vector3(0, 0.6 * U, 0);
    for (const side of [-1, 1]) {
      const smile = [
        [0, 161, 822],
        [7, 162.5, 826],
        [14, 165, 834],
        [19, 166.5, 845],
        [21, 167.5, 854],
      ].map(([x, iy, ix]) => onHead(side * x, iy, ix));
      for (let i = 0; i + 1 < smile.length; i++) line(smile[i], smile[i + 1], thin);
    }
    line(onHead(0, 153, 820), onHead(0, 160.5, 822), new THREE.Vector3(0.6 * U, 0, 0));
    // The nose, matte.
    hull(
      s,
      NOSE.map(([ix, half]) => ring(ix, half)),
      () => tone('nose'),
      'flat',
      P(816.5, 146),
    );
  },
  tail: {
    joints: [P(1068, 245), P(1098, 262), P(1125, 275), P(1150, 285), P(1172, 292)],
    tip: P(1190, 290),
    radii: [26, 21, 16.5, 11.5, 7].map((r) => r * U),
    build(s: Sculpt) {
      // Thick where it leaves the rump, flattening a little and tapering to
      // a point, lying along the ground behind.
      const path = [P(1062, 242), P(1078, 250), P(1098, 262), P(1120, 273), P(1143, 283), P(1165, 290), P(1182, 294.5)];
      const radii: [number, number, number][] = [
        [36, 26, 26],
        [34, 24, 25],
        [29, 20.5, 21.5],
        [23, 16, 16.5],
        [16, 11, 11.5],
        [10, 6.5, 6.5],
        [4.5, 3, 3],
      ];
      loft(s, tubeRings(path, radii.map(([w, up, down]) => [w * U, up * U, down * U] as const)), {
        sides: 7,
        start: 'flat',
        end: 8 * U,
        paint: (r, angle, k) => (Math.abs(angle - Math.PI) < 1 ? blend('coat', 'cream', 0.12) : patchy('coat', 'dark', r, k, 0.35, 0.35)),
      });
    },
  },
  front: {
    hip: P(918, 224, 28),
    knee: P(919, 254, 28),
    ankle: P(907, 284, 28),
    paw: P(904, 295, 28),
    bend: -1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.45, w: 13 * U, d: 18 * U },
        { t: 0, w: 15 * U, d: 18 * U },
        { t: 0.5, w: 16.5 * U, d: 16 * U },
        { t: 1, w: 15.5 * U, d: 14 * U },
        { t: 1.5, w: 15 * U, d: 13 * U },
        { t: 2, w: 14.5 * U, d: 12 * U },
        { t: 2.5, w: 14 * U, d: 11 * U },
        { t: 3, w: 14 * U, d: 10 * U, dz: 1 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        // Brown above the elbow, dark below it.
        paint: (r, _angle, k) => (r <= 1 ? patchy('coat', 'dark', r, k + 3, 0.3, 0.3) : r === 2 ? blend('coat', 'sock', 0.5) : tone('sock')),
      });
      paw(s, joints[3], side, 7, 17, 15, 8.5);
    },
  },
  hind: {
    hip: P(1042, 222, 36),
    knee: P(1026, 262, 36),
    ankle: P(1058, 286, 36),
    paw: P(1053, 296, 36),
    bend: 1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.15, w: 18 * U, d: 24 * U },
        { t: 0, w: 19 * U, d: 26 * U },
        { t: 0.5, w: 18 * U, d: 21 * U, dz: 2 * U },
        { t: 1, w: 15 * U, d: 14 * U },
        { t: 1.5, w: 14 * U, d: 13 * U },
        { t: 2, w: 13.5 * U, d: 11.5 * U, back: 12 * U },
        { t: 2.5, w: 13 * U, d: 10 * U },
        { t: 3, w: 13.5 * U, d: 9.5 * U, dz: 1 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        // The brown haunch, dark below the knee.
        paint: (r, _angle, k) => (r <= 2 ? patchy('coat', 'dark', r, k + 7, 0.3, 0.3) : r === 3 ? blend('coat', 'sock', 0.6) : tone('sock')),
      });
      paw(s, joints[3], side, 5, 24, 16, 9.5);
    },
  },
  walk: WALK,
  run: RUN,
};

const SINK = 0.225; // m its origin sits under the water's surface afloat: its back, upper sides and head out of the water
const SWIM = 0.45; // m/s it swims
const DASH = 0.9; // m/s it swims told to run
const DIVE_SPEED = 0.6; // m/s it goes, diving
const DIVE_DEPTH = 0.5; // m it dives below where it swims
const DIVE_TIME = 3.4; // s a dive lasts: over headfirst and down (to 1 s), along under the water (to 2.2 s), and back up
const FLOAT_RATE = 2.5; // how fast it settles into the water (or out), a second
const STAND = 4.6; // s it stands up, before it drops back to Idle
const REAR = 0.07; // m it rises in the water, rearing up
// The middle of its body, which it tips about in the water, in the rig; and
// its snout, which breaks the surface diving.
const BODY_MIDDLE = P(980, 218);
const SNOUT = P(819, 150);
const X_AXIS = new THREE.Vector3(1, 0, 0);
const q1 = new THREE.Quaternion();
const v1 = new THREE.Vector3();

// How far below where it swims a dive has taken it, `t` s into it: a little
// up as it arches over, down headfirst, along under the water and back up.
function diveDepth(t: number): number {
  if (t <= 0 || t >= DIVE_TIME) return 0;
  return 0.05 * bump(t / 0.5) - DIVE_DEPTH * smooth((t - 0.2) / 0.8) * (1 - smooth((t - 2.2) / 1.2));
}

// How far it tips nose down (rad) `t` s into a dive: over headfirst, its
// tail flicking up out of the water, level along, nose up coming back.
function diveTip(t: number): number {
  if (t <= 0 || t >= DIVE_TIME) return 0;
  return 0.95 * smooth(t / 0.35) * (1 - smooth((t - 0.7) / 0.45)) - 0.6 * bump((t - 2.1) / 1.3);
}

type Mode = 'idle' | 'swim' | 'dash' | 'standUp';

export class Otter extends FourLegged {
  // The sheet's colour variations, the default (brown) first.
  static readonly COLORS = Object.keys(VARIATIONS);
  // m its origin sits under the water's surface afloat.
  static readonly SINK = SINK;
  // m/s it swims.
  static readonly SWIM_SPEED = SWIM;
  // m it dives below where it swims.
  static readonly DIVE_DEPTH = DIVE_DEPTH;

  // Its whiskers, on the head.
  readonly whiskers: THREE.LineSegments;
  // Whether it is in the water: Swim() puts it there, and a preview on the
  // lake does. Afloat, Walk() and Run() swim.
  afloat = false;
  // The water's surface under it, in its parent's y (0 when still): a
  // preview follows the waves with it.
  surface = 0;
  // Called where it breaks the water's surface, diving in and coming back
  // up, with its velocity there, in its parent's space: a preview hands it
  // to the water.
  onSplash: ((x: number, z: number, velocity: THREE.Vector3) => void) | null = null;

  private float = 0; // 0 on land, 1 afloat; eased
  private settled = false;
  private swimSpeed = 0; // m/s through the water; eased
  private stroke = 0; // how far through a paddling stroke
  private tip = 0; // rad its whole body tips in the water (< 0 nose up); eased
  private rise = 0; // m it rises in the water, rearing up; eased
  private diveTime = -1; // s into a dive, or -1
  private snoutUnder = false;
  private after: Mode = 'swim'; // what it does once a dive is over

  constructor(options: ForestAnimalOptions = {}) {
    super(BUILD, options);
    this.whiskers = whiskers(this.palette);
    this.whiskers.geometry.translate(-BUILD.head.x, -BUILD.head.y, -BUILD.head.z);
    this.head.add(this.whiskers);
  }

  // Whether it is under way in a dive.
  get diving(): boolean {
    return this.diveTime >= 0;
  }

  SetColor(variation: string): void {
    super.SetColor(variation);
    paint(this.whiskers.geometry, this.palette);
  }

  // On land, sits up on its haunches, its fore legs straight and its tail
  // along the ground, looking about, as the sheet's Idle draws it. Afloat,
  // treads water, its head up, looking about. Named as in its sheet's poses.
  Idle(): void {
    if (this.queue('idle')) return;
    super.Idle();
  }

  // Walks forward the way it faces until told otherwise; afloat, swims.
  // Named as in its sheet's poses.
  Walk(): void {
    if (this.queue('swim')) return;
    if (this.afloat) this.setMode('swim', 'stand');
    else super.Walk();
  }

  // Bounds forward the way it faces until told otherwise, its back flexing
  // and its tail streaming out; afloat, swims fast. Named as in its sheet's
  // poses.
  Run(): void {
    if (this.queue('dash')) return;
    if (this.afloat) this.setMode('dash', 'stand');
    else super.Run();
  }

  // Swims forward at the water's surface the way it faces, low in the water
  // with its back and head out, paddling, its tail waving behind, until told
  // otherwise. It takes the water to be where it is (y = 0 of its parent, or
  // `surface`). Named as in its sheet's poses.
  Swim(): void {
    if (this.queue('swim')) return;
    this.afloat = true;
    this.setMode('swim', 'stand');
  }

  // Afloat, arches over headfirst and dives, its tail flicking up out of the
  // water, swims on under the water DIVE_DEPTH down and comes back up (about
  // 3.4 s), then swims on. Ignored on land, and while it dives. Named as in
  // its sheet's poses.
  Dive(): void {
    if (!this.afloat || this.diving) return;
    this.diveTime = 0;
    this.snoutUnder = false;
    this.after = 'swim';
    this.setMode('dive', 'stand');
  }

  // Rears up on its hind legs, its fore paws held before its chest, its tail
  // on the ground behind, and looks about, then drops back to Idle (about
  // 4.6 s). Afloat, rears up out of the water, treading water. Named as in
  // its sheet's poses.
  StandUp(): void {
    if (this.queue('standUp')) return;
    this.setMode('standUp', 'stand');
  }

  // A behaviour asked for while it dives waits until the dive is over.
  private queue(mode: Mode): boolean {
    if (!this.diving) return false;
    this.after = mode;
    return true;
  }

  protected pose(goal: Posture): void {
    super.pose(goal); // the idle look about
    const t = this.modeTime;
    const tip = this.tip;
    switch (this.mode) {
      case 'idle':
        if (this.afloat) {
          // Treading water, its head up and level.
          goal.neck = -0.15;
          goal.head += -0.04 - tip;
          goal.tail = 0.3;
        } else {
          // Sitting on its haunches, its fore legs straight, its tail along
          // the ground to one side: once it has stopped (a sit coming in on
          // top of a gallop's crouch took its rump into the ground).
          const sit = Math.max(0, 1 - 2 * this.moving) ** 2;
          goal.rear = 0.035 * sit;
          goal.sit = 0.55 * sit;
          goal.hindReach = 0.03 * sit;
          goal.flex = -0.75 * sit;
          goal.neck = 0.45 * sit;
          goal.head += 0.12 * sit; // looking a little down, at whoever is before it
          goal.tail = -0.2 * sit;
          goal.tailTurn = 0.5 * sit;
        }
        break;
      case 'standUp': {
        // On land it rises once its feet have settled from any step.
        const up = (this.afloat ? 1 : smooth((t - 0.35) / 0.4)) * (t < STAND - 0.6 ? 1 : 0);
        const look = smooth((t - 1) / 0.6) * up;
        goal.headTurn = 0.55 * Math.sin((t - 1) * 1.1) * look;
        goal.headTilt = 0.08 * Math.sin(t * 0.9) * look;
        if (this.afloat) {
          goal.neck = 0.45 * up;
          goal.head = -0.05 - tip;
          goal.tail = 0.1;
        } else {
          // Its fore paws leave the ground a moment after its chest starts up,
          // so they are lifted clear before they curl.
          goal.front = -0.3 * up;
          goal.rear = 0.01 * up;
          goal.frontFree = smooth((t - 0.45) / 0.4) * (t < STAND - 0.6 ? 1 : 0);
          goal.sit = 0.55 * up;
          goal.hindReach = 0.03 * up;
          goal.neck = 0.85 * up;
          goal.head = -0.05 * up;
          goal.tail = 0.55 * up;
        }
        if (t > STAND) this.Idle();
        break;
      }
      case 'swim':
      case 'dash':
        // Head up and forward, the tail out behind near the surface.
        goal.neck = this.mode === 'dash' ? 0.05 : -0.1;
        goal.head = -0.04 - tip;
        goal.tail = 0.35;
        break;
      case 'dive': {
        // Arched over headfirst going down, stretched out along, head up
        // coming back.
        const s = this.diveTime;
        const arch = smooth(s / 0.35) * (1 - smooth((s - 0.6) / 0.5));
        goal.flex = 0.3 * arch;
        goal.neck = 0.35 * arch;
        goal.head = 0.25 * arch - 0.1 * smooth((s - 2.1) / 0.6);
        goal.tail = 0.25 - 0.3 * arch;
        break;
      }
      case 'walk':
        goal.neck = 0.25; // the head held forward, low
        break;
      case 'run':
        goal.neck = 0.3;
        goal.head = 0.05;
        break;
    }
  }

  // In the water it has a speed of its own, and no gait.
  protected cruise(): number | null {
    return this.afloat || this.float > 0.02 ? this.swimSpeed : null;
  }

  // In the water its feet paddle: the fore feet under the chest, the hind
  // feet kicking back, the pairs taking turns; folded back along the body
  // diving; the fore paws drawn up before the chest rearing up.
  protected freeFoot(foot: Foot): FreeFoot | null {
    if (!this.afloat && this.float < 0.02) return null;
    const L = foot.length;
    const phase = 2 * Math.PI * (this.stroke + (foot.side > 0 ? 0.5 : 0) + (foot.front ? 0.25 : 0));
    const s = Math.sin(phase);
    const c = Math.cos(phase);
    const at = foot.rest.clone();
    let tilt: number;
    let curl: number;
    if (foot.front) {
      at.y += L * (0.32 + 0.12 * c);
      at.z += L * (0.1 + 0.22 * s);
      tilt = -0.5 + 0.35 * s;
      curl = 0.4 + 0.2 * c;
    } else {
      at.y += L * (0.25 + 0.1 * c);
      at.z += L * (-0.2 + 0.28 * s);
      tilt = -0.3 + 0.4 * s;
      curl = 0.2 + 0.2 * c;
    }
    // Folded back along the body, diving.
    const tuck = this.diving ? smooth(this.diveTime / 0.4) * (1 - smooth((this.diveTime - DIVE_TIME + 0.6) / 0.6)) : 0;
    if (tuck > 0) {
      const to = foot.rest.clone();
      to.y += L * (foot.front ? 0.55 : 0.35);
      to.z += L * (foot.front ? -0.2 : -0.55);
      at.lerp(to, tuck);
      tilt += ((foot.front ? -1.2 : 0.6) - tilt) * tuck;
      curl += ((foot.front ? 0.8 : -0.4) - curl) * tuck;
    }
    // The fore paws before the chest, rearing up.
    const reared = foot.front ? Math.min(1, this.rise / REAR) : 0;
    if (reared > 0) {
      const to = foot.rest.clone();
      to.y += L * 0.75;
      to.z += L * 0.3;
      at.lerp(to, reared);
      tilt += (-1.1 - tilt) * reared;
      curl += (0.9 - curl) * reared;
    }
    return { at, tilt, curl };
  }

  update(delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    // Placed in the water before its first frame, it starts afloat.
    this.float = this.settled ? ease(this.float, this.afloat ? 1 : 0, FLOAT_RATE, dt) : this.afloat ? 1 : 0;
    this.settled = true;
    this.inWater(dt);
    super.update(delta);
    // Its tail waves up and down in the water.
    if (this.float > 0.01) {
      const k = this.float * Math.min(1, 0.3 + this.swimSpeed * 1.5);
      const rate = 4 + 5 * this.swimSpeed;
      this.spine.tail.forEach((b, i) => (b.rotation.x += 0.12 * k * Math.sin(this.time * rate - i * 0.9)));
    }
  }

  // Its speed and strokes in the water, a dive's depth, the tip of its whole
  // body (the rig, about the body's middle), and how deep it floats.
  private inWater(dt: number): void {
    const w = this.float;
    const diving = this.diving;
    const goal = !this.afloat ? 0 : diving ? DIVE_SPEED : this.mode === 'swim' ? SWIM : this.mode === 'dash' ? DASH : 0;
    this.swimSpeed = ease(this.swimSpeed, goal, 2.2, dt);
    this.stroke = (this.stroke + dt * (0.8 + 2.4 * this.swimSpeed)) % 1;
    let depth = 0;
    let dip = 0; // rad it tips nose down, diving
    if (diving) {
      this.diveTime += dt;
      const s = this.diveTime;
      depth = diveDepth(s);
      dip = diveTip(s);
    }
    const rearing = this.afloat && this.mode === 'standUp' && this.modeTime < STAND - 0.6;
    const swimming = this.mode === 'swim' || this.mode === 'dash';
    this.tip = ease(this.tip, w * (rearing ? -0.95 : swimming ? -0.14 : diving ? 0 : -0.05), 3, dt);
    this.rise = ease(this.rise, rearing ? REAR : 0, 3, dt);
    const pitch = this.tip + dip * w;
    q1.setFromAxisAngle(X_AXIS, pitch);
    this.rig.quaternion.copy(q1);
    this.rigOffset.copy(BODY_MIDDLE).applyQuaternion(q1).negate().add(BODY_MIDDLE);
    this.rigOffset.y += this.rise * w;
    if (w > 0.0005 || this.afloat) {
      const bob = 0.006 * Math.sin(this.time * 2.3) * Math.max(0, 1 - this.swimSpeed * 2);
      this.position.y = this.surface - SINK * w + depth + bob * w;
    }
    if (diving) {
      // Splashes where its snout goes in, and comes back out.
      const snout = v1.copy(SNOUT).applyQuaternion(q1).add(this.rigOffset);
      const under = this.position.y + snout.y < this.surface;
      if (under !== this.snoutUnder) {
        this.snoutUnder = under;
        const heading = this.rotation.y;
        const along = snout.z;
        const x = this.position.x + Math.sin(heading) * along;
        const z = this.position.z + Math.cos(heading) * along;
        const velocity = new THREE.Vector3(Math.sin(heading) * this.swimSpeed, under ? -1.2 : 1, Math.cos(heading) * this.swimSpeed);
        this.onSplash?.(x, z, velocity);
      }
      if (this.diveTime >= DIVE_TIME) {
        this.diveTime = -1;
        if (this.after === 'idle') super.Idle();
        else this.setMode(this.after, 'stand');
      }
    }
  }
}
