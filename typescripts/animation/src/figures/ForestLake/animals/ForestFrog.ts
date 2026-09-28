import * as THREE from 'three';
import { ForestAnimal, type ForestAnimalOptions } from './ForestAnimal';
import { blend, bone, coat, ease, ellipsoid, eye, gloss, loft, rigidMesh, Sculpt, seededRandom, skinnedMesh, smooth, tone, type Cap, type Palette, type Tone, type Weights } from './parts';

// The forest lake's frog (ForestFrog.md), as its reference sheet draws it: a
// chunky low poly pond frog sitting up on its front legs, its back tilted up
// toward the head, a bright lime-green back with darker spots, a cream belly
// and throat, a wide mouth line, big bulging eyes on top of its head (green
// lids, dark brown irises, a glint), front legs straight under the chest with
// splayed hands, big hind legs folded at its sides, and peach-orange webbed
// feet with round toe tips. Its colours are the sheet's five variations.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground under
// its belly. Its shape is traced from the sheet's side and front views in
// their pixels (U m each): 8 cm from the snout to the rump sitting, as a
// grown pond frog is, and 5.8 cm to the top of its eyes.
//
// Built as it sits. The body (with the head) bends with a spine of four
// bones (the pelvis, the trunk, the head and the throat, which pulses); each
// leg is three rigid pieces on three bones (thigh, shin and foot; upper arm,
// forearm and hand), placed by two-bone inverse kinematics: a planted foot
// stays where it landed, in the space the frog moves in. On land nothing
// goes into the ground (flat, at its origin's height): knees and elbows turn
// up, free feet turn flat, and the body rides up over its legs. The eyes sit
// on the head's bone and draw back into their lids to blink.

type V3 = THREE.Vector3;

const U = 0.08 / 229; // m per pixel of the sheet's side view: 229 px from the snout to the rump

// A point traced from the sheet: `z` forward and `y` up from the ground, in
// the side view's pixels; `x` to its right, in the front view's.
function P(z: number, y: number, x = 0): V3 {
  return new THREE.Vector3(x * U, y * U, z * U);
}

// The sheet's five colour variations, picked from its pictures (lit faces,
// a little brightened). `mark` is the poison dart frogs' bold black bands,
// only a shade darker than the skin on the others.
const VARIATIONS: Record<string, Palette> = {
  green: { skin: 0xabb846, light: 0xd6d25c, dark: 0x879a32, spot: 0x5b6c2b, mark: 0x98a83c, cream: 0xf2cda4, belly: 0xd9b28c, foot: 0xf2a86c, web: 0xdd8649, tip: 0xf9cc9e, mouth: 0x6b3d2b, nostril: 0x2b2117, lid: 0xcdb553, iris: 0x4b2c19, pupil: 0x07080a, shine: 0xffffff },
  brown: { skin: 0xb98a5e, light: 0xd4a878, dark: 0x94693f, spot: 0x7a5434, mark: 0xa27749, cream: 0xe6c9a4, belly: 0xc9a883, foot: 0xf2b27a, web: 0xd98c52, tip: 0xf8cfa2, mouth: 0x4a2c1d, nostril: 0x2a1c12, lid: 0xc79a66, iris: 0x2a1a12, pupil: 0x07080a, shine: 0xffffff },
  blue: { skin: 0x1e8fe0, light: 0x46aef2, dark: 0x1470b8, spot: 0x151a20, mark: 0x151a20, cream: 0xc3c9d3, belly: 0x9ea6b3, foot: 0xf6c08a, web: 0xe39a5c, tip: 0xfbd6ac, mouth: 0x202a36, nostril: 0x101820, lid: 0x2a7fc4, iris: 0x0c0d10, pupil: 0x050506, shine: 0xffffff },
  yellow: { skin: 0xf0c022, light: 0xfbd84a, dark: 0xcf9c14, spot: 0x1c1a16, mark: 0x1c1a16, cream: 0xf2dc8c, belly: 0xd8bd6a, foot: 0xf4b870, web: 0xe0944c, tip: 0xfad29e, mouth: 0x3a2c14, nostril: 0x1c160c, lid: 0xd9a81c, iris: 0x0c0d10, pupil: 0x050506, shine: 0xffffff },
  red: { skin: 0xe5302e, light: 0xf45a52, dark: 0xb81e22, spot: 0x1e1416, mark: 0x1e1416, cream: 0xf0b2b0, belly: 0xd88e8e, foot: 0xf06a56, web: 0xd84a40, tip: 0xf89a88, mouth: 0x3c1416, nostril: 0x1c0c0c, lid: 0xc8262a, iris: 0x0c0d10, pupil: 0x050506, shine: 0xffffff },
};

// ---------------------------------------------------------------- the skeleton, as it sits

const PELVIS = P(-100, 48); // between the hips: the body pitches about it
const SPINE = P(-66, 80); // the sacral hump, where the trunk bends from the rump
const HEAD = P(24, 118); // where the head nods
const THROAT = P(78, 104); // the throat pulses about it
// The right legs' joints (the left's are their mirror).
const HIP = P(-98, 46, 24);
const KNEE = P(-10, 58, 60);
const HEEL = P(-112, 24, 50);
const BALL = P(-45, 7, 58); // where the hind toes spread
const SHOULDER = P(15, 86, 42);
const ELBOW = P(13, 52, 47);
const WRIST = P(35, 16, 48);
const PALM = P(49, 6, 48); // where the fingers spread
const EYE_AT = P(63, 141, 32); // inside an eye's lid, where its look is cast from
const EYE_RADIUS = 18 * U;
const BODY_TILT = 0.47; // rad its back rises toward its head, sitting

// eye() stands its glint off the dome by a fraction of the eye's radius that
// grows with the dome's bulge: on the frog's tall domes enough to poke out
// of the eye's outline seen from the front. Lowered this much (of the
// eye's radius) onto it, and made this much smaller, so its corners keep
// to the dome too.
const EYE_GLINT_DROP = 0.13;
const EYE_GLINT_SIZE = 0.65;

// Lowers an eye's glint (the last facet eye() adds) `drop` m back along its
// look, onto the dome, and shrinks it about its middle.
function seatGlint(geometry: THREE.BufferGeometry, look: V3, drop: number): void {
  const position = geometry.getAttribute('position') as THREE.BufferAttribute;
  const first = position.count - 3;
  const middle = new THREE.Vector3();
  const corner = new THREE.Vector3();
  for (let i = first; i < position.count; i++) middle.add(corner.fromBufferAttribute(position, i));
  middle.divideScalar(3);
  for (let i = first; i < position.count; i++) {
    corner.fromBufferAttribute(position, i).sub(middle).multiplyScalar(EYE_GLINT_SIZE).add(middle).addScaledVector(look, -drop);
    position.setXYZ(i, corner.x, corner.y, corner.z);
  }
  position.needsUpdate = true;
  geometry.computeBoundingBox();
  geometry.computeBoundingSphere();
}

// ---------------------------------------------------------------- motion

const G = 9.81; // m/s², it leaps as far as it really would
const SWIM_STROKE = 0.9; // s a swimming stroke takes
const SWIM_SLOW = 0.03; // m/s it glides at, at the end of a stroke
const SWIM_FAST = 0.2; // m/s it shoots forward at, kicking
const FLOAT = -40 * U; // m the rig sinks in the water: its eyes and back just out
const LYING = 16 * U; // m the rig rises to swim on dry ground, its belly on it
const BLINK = 0.2; // s a blink takes
// On land: how high a knee or an elbow stays off the ground at least; how
// far above where it plants a landing foot starts turning flat, to come
// down flat; and how high a free foot's lowest corner stays, as it does
// planted.
const JOINT_CLEAR = 15 * U;
const FLATTEN = 0.045;
const SOLE_CLEAR = 0.00045;
const BODY_CLEAR = 0.0003; // m its body's lowest corner stays above the ground

// A hop: gathered (all four feet down), a push of the hind legs (the front
// feet leaving), a flight, and landing on the front feet, the body sliding
// on over them while it stops, the hind feet coming down under it.
interface Leap {
  gather: number; // s it sits gathered before the push
  crouch: Posture; // how it sits gathered
  push: number; // s its hind legs push
  vx: number; // m/s it goes forward, leaving the ground
  vy: number; // m/s it goes up, leaving
  pitch: readonly [leave: number, land: number]; // rad (> 0 its nose down) leaving and landing
  hind: V3; // where the hind feet reach in the air: from the hip, along the body (px)
  straight: number; // how far the hind feet straighten out in the air (0..1)
  front: V3; // where the front feet reach in the air: from the shoulder, along the body (px)
  fold: number; // a second, how fast its hind legs fold back under it coming down
  stop: number; // s it slides on over its front feet, landing
  settle: number; // s after landing before it gathers again
}

// Walk(): small crawling hops, low and level, as the sheet's Walk.
const HOP: Leap = {
  gather: 0.2,
  crouch: { lift: -3 * U, shift: 0, pitch: 0.2, flex: 0.04, nod: -0.12 },
  push: 0.1,
  vx: 0.3,
  vy: 0.22,
  pitch: [0.12, 0.26],
  hind: new THREE.Vector3(18, -26, -80),
  straight: 0.35,
  front: new THREE.Vector3(10, -72, 24),
  fold: 16,
  stop: 0.05,
  settle: 0.12,
};

// Jump(): the sheet's Jump (Start), crouched, its head up; Jump (Air), rising
// steeply nose up, its hands tucked under its chest and its hind legs
// stretched straight out behind; Jump (Land), down on its front feet, the
// hind legs folded in again, sitting.
const LEAP: Leap = {
  gather: 0.3,
  crouch: { lift: -5 * U, shift: -4 * U, pitch: 0.16, flex: 0.06, nod: -0.2 },
  push: 0.06,
  vx: 0.9,
  vy: 0.95,
  pitch: [-0.35, 0.3],
  hind: new THREE.Vector3(14, -18, -300),
  straight: 1,
  front: new THREE.Vector3(10, -50, 8),
  fold: 14,
  stop: 0.035,
  settle: 0.45,
};

// How far a landing leap has slid on land `t` s after touching down.
function slid(spec: Leap, t: number): number {
  const s = THREE.MathUtils.clamp(t, 0, spec.stop);
  return spec.vx * (s - (s * s) / (2 * spec.stop));
}

interface Posture {
  lift: number; // m the pelvis rises (< 0 crouched)
  shift: number; // m it moves forward
  pitch: number; // rad the body tips about the hips (> 0 its nose down)
  flex: number; // rad the trunk bends from the rump (> 0 down)
  nod: number; // rad the head tips (> 0 its nose down)
}

const SIT: Readonly<Posture> = { lift: 0, shift: 0, pitch: 0, flex: 0, nod: 0 };
const KEYS = Object.keys(SIT) as (keyof Posture)[];

// ---------------------------------------------------------------- the body's cross-sections

// Corners down each half of a cross-section of the body, top to bottom.
const ROWS = 11;

// A cross-section of the body at `z`: `top` and `bottom` of its middle, `w`
// its half width at the height `at`, rounded as superellipses (`nu` above,
// `nd` below), in the sheet's pixels. The head's sections (`lip`) have the
// mouth line along `at`: a band of the upper lip above it, the thin line,
// and under it the jaw curving in to the throat and chest, `chest` wide.
interface Section {
  z: number;
  top: number;
  bottom: number;
  w: number;
  at: number;
  lip?: boolean;
  chest?: number;
  wb?: number; // a trunk section's half width toward its bottom (the belly between the front legs), `w` by default
  nu?: number;
  nd?: number;
}

// From the rump to the snout, traced from the side view (the heights) and
// the front and back views (the widths): narrow behind, between the thighs,
// widest across the mouth.
const SECTIONS: readonly Section[] = [
  { z: -106, top: 54, bottom: 12, w: 16, at: 32, nd: 3 },
  { z: -95, top: 70, bottom: 7, w: 22, at: 36, nd: 3.2 },
  { z: -80, top: 87, bottom: 6, w: 28, at: 44, nd: 3.2 },
  { z: -62, top: 104, bottom: 12, w: 34, at: 55, nd: 2.8 },
  { z: -42, top: 116, bottom: 15, w: 40, at: 63, wb: 32 },
  { z: -22, top: 125, bottom: 19, w: 44, at: 70, wb: 30 },
  { z: -3, top: 133, bottom: 24, w: 45, at: 84, wb: 24, nd: 2 },
  { z: 15, top: 139, bottom: 30, w: 46, at: 92, wb: 24, nd: 2 },
  { z: 31, top: 144, bottom: 36, w: 51, at: 99, lip: true, chest: 33 },
  { z: 44, top: 149, bottom: 50, w: 51, at: 105, lip: true, chest: 32 },
  { z: 57, top: 153, bottom: 64, w: 49, at: 110, lip: true, chest: 31 },
  { z: 70, top: 155, bottom: 83, w: 45, at: 113, lip: true, chest: 30 },
  { z: 83, top: 155, bottom: 89, w: 39, at: 115.5, lip: true, chest: 30 },
  { z: 95, top: 152, bottom: 97, w: 32, at: 117, lip: true, chest: 24 },
  { z: 106, top: 146, bottom: 107, w: 23, at: 118.5, lip: true, chest: 16 },
  { z: 114, top: 139, bottom: 113, w: 13, at: 119.5, lip: true, chest: 8 },
];
const RUMP_TIP = P(-113, 30);
const SNOUT_TIP = P(122, 120); // on the mouth line, so it runs on level across the front
const LIP_BAND = 5; // px of the upper lip's pale band above the mouth line
const MOUTH = 1.8; // px the mouth line is wide

const quarter = (c: number, n: number) => Math.sign(c) * Math.abs(c) ** (2 / n);

// The right half of a section, top to bottom.
function sectionPoints(s: Section): V3[] {
  const nu = s.nu ?? 2.4;
  const nd = s.nd ?? 2.4;
  const points: V3[] = [];
  if (s.lip) {
    const hi = s.at + LIP_BAND;
    for (let k = 0; k <= 4; k++) {
      const a = (k / 4) * (Math.PI / 2);
      points.push(P(s.z, hi + (s.top - hi) * quarter(Math.cos(a), nu), s.w * quarter(Math.sin(a), nu)));
    }
    points.push(P(s.z, s.at, s.w - 0.4));
    const lo = s.at - MOUTH;
    const wl = s.w - 1.6;
    const chest = s.chest ?? wl * 0.8;
    const drop = lo - s.bottom;
    points.push(P(s.z, lo, wl));
    points.push(P(s.z, lo - 0.12 * drop, chest + 0.4 * (wl - chest)));
    points.push(P(s.z, lo - 0.45 * drop, chest));
    points.push(P(s.z, lo - 0.84 * drop, chest * 0.7));
    points.push(P(s.z, s.bottom, 0));
  } else {
    for (let k = 0; k < ROWS; k++) {
      const a = (k / (ROWS - 1)) * Math.PI;
      const c = Math.cos(a);
      const n = c >= 0 ? nu : nd;
      const w = c >= 0 || s.wb === undefined ? s.w : s.w + (s.wb - s.w) * (a / (Math.PI / 2) - 1);
      points.push(P(s.z, s.at + (c >= 0 ? s.top - s.at : s.at - s.bottom) * quarter(c, n), w * quarter(Math.sin(a), n)));
    }
  }
  return points;
}

// The middle of the body at `z` px: halfway between its top and bottom.
function middleAt(z: number): V3 {
  const i = Math.max(0, Math.min(SECTIONS.length - 2, SECTIONS.findIndex((s) => s.z > z) - 1));
  const a = SECTIONS[i];
  const b = SECTIONS[i + 1];
  const f = THREE.MathUtils.clamp((z - a.z) / (b.z - a.z), 0, 1);
  return P(z, ((a.top + a.bottom) / 2) * (1 - f) + ((b.top + b.bottom) / 2) * f);
}

// Spots painted on the back and sides, [z px, angle round the body from its
// top (rad, > 0 on its right), radius px]: the green frog's dark green ones
// (every colour has them).
const SPOTS: readonly (readonly [number, number, number])[] = [
  [-46, 0.02, 18], // the big one in the middle of the back
  [-68, 0.06, 14],
  [-4, -0.02, 10], // behind the head
  [-88, 0.12, 12], // over the rump
  [-12, -0.46, 7],
  [-6, 0.42, 6],
  [-30, 0.8, 7],
  [-36, -0.84, 8],
  [-70, 0.72, 8],
  [-74, -0.62, 9],
  [2, 1.28, 6],
  [6, -1.32, 6],
  [-36, 1.5, 8],
  [-26, -1.48, 7],
  [-60, 1.78, 7],
  [-54, -1.8, 8],
  [40, 0.42, 4],
  [42, -0.44, 4],
  [-94, 0.9, 6],
  [-92, -1, 6],
];
// The poison dart frogs' bold black patches and bands (`mark`; on the
// others a shade darker than the skin).
const MARKS: readonly (readonly [number, number, number])[] = [
  [28, 0.95, 10], // behind each eye, along the side of the head
  [28, -0.95, 10],
  [10, 1.12, 10],
  [10, -1.12, 10],
  [-18, 1.2, 12],
  [-22, -1.22, 12],
  [-64, 1.3, 11],
  [-60, -1.28, 11],
  [-20, 0.05, 10],
  [-100, 0.5, 9],
  [-100, -0.5, 9],
];

// A repeatable fraction for a face, to vary the skin's facets.
function hash(a: number, b: number, c = 0): number {
  const h = Math.sin(a * 12.9898 + b * 78.233 + c * 37.719) * 43758.5453;
  return h - Math.floor(h);
}

// The skin of a facet: now and then a little lighter (yellow-green) or
// darker, as the sheet's facets are.
function skin(a: number, b: number, c: number): Tone {
  const f = hash(a, b, c);
  if (f < 0.16) return blend('skin', 'light', 0.28 + f);
  if (f > 0.92) return blend('skin', 'dark', 0.35);
  return tone('skin');
}

// Round patches painted on a surface: each spot's outline, a few irregular
// corners round its middle, cast onto the surface from outside and raised a
// hair off it, so it lies across the facets as the sheet's spots do.
function decals(s: Sculpt, spots: readonly (readonly [number, number, number])[], t: Tone, weights: (z: number) => Weights, random: () => number): void {
  const lift = 0.45 * U;
  const shapes: { centre: V3; ring: V3[]; normal: V3 }[] = [];
  const onto = (from: V3, way: V3) => s.onto(from.clone().addScaledVector(way, -300 * U), way);
  for (const [z, angle, radius] of spots) {
    const axis = middleAt(z);
    const inward = new THREE.Vector3(-Math.sin(angle), -Math.cos(angle), 0);
    const centre = onto(axis, inward);
    // The surface's own lean there, from where rays either side meet it.
    const du = onto(middleAt(z + 3), inward).sub(onto(middleAt(z - 3), inward));
    const side = new THREE.Vector3(Math.cos(angle), -Math.sin(angle), 0);
    const normal = new THREE.Vector3().crossVectors(side, du).normalize();
    if (normal.dot(inward) > 0) normal.negate();
    const along = new THREE.Vector3().crossVectors(normal, side).normalize();
    const corners = 7;
    const ring: V3[] = [];
    for (let i = 0; i < corners; i++) {
      const a = ((i + random() * 0.4) / corners) * Math.PI * 2;
      const r = radius * U * (0.8 + 0.35 * random());
      const p = centre.clone().addScaledVector(side, Math.cos(a) * r).addScaledVector(along, Math.sin(a) * r);
      ring.push(s.onto(p.addScaledVector(normal, 20 * U), normal.clone().negate()).addScaledVector(normal, lift));
    }
    shapes.push({ centre: centre.addScaledVector(normal, lift), ring, normal });
  }
  for (const { centre, ring, normal } of shapes) {
    for (let i = 0; i < ring.length; i++) {
      const a = ring[i];
      const b = ring[(i + 1) % ring.length];
      s.toward(centre, a, b, normal, t, [weights(centre.z / U), weights(a.z / U), weights(b.z / U)]);
    }
  }
}

// ---------------------------------------------------------------- sculpting helpers

interface HullOptions {
  // The tone of the face from ring r to r + 1 between rows k and k + 1 (r is
  // -1 for the rump's cap and the last ring's index for the snout's).
  paint(ring: number, row: number, middle: V3, side: 1 | -1): Tone;
  weights(ring: number, row: number): Weights;
  back: V3; // where the first ring closes to a point
  front: V3; // where the last one does
}

// A closed surface through rings of corners, each the right half of a
// cross-section, top to bottom, mirrored for the left: flat faces, split
// alternately one way and the other.
function hull(s: Sculpt, rings: readonly V3[][], o: HullOptions): void {
  const count = rings.length;
  const rows = rings[0].length;
  const pick = (r: number, k: number, side: 1 | -1) => (side > 0 ? rings[r][k] : new THREE.Vector3(-rings[r][k].x, rings[r][k].y, rings[r][k].z));
  const centre = (r: number) => {
    const c = new THREE.Vector3();
    for (const v of rings[r]) c.add(v);
    c.divideScalar(rows);
    c.x = 0;
    return c;
  };
  const middle = new THREE.Vector3();
  for (let r = 0; r + 1 < count; r++) {
    const inside = centre(r).add(centre(r + 1)).multiplyScalar(0.5);
    for (const side of [1, -1] as const) {
      for (let k = 0; k + 1 < rows; k++) {
        const a = pick(r, k, side);
        const b = pick(r, k + 1, side);
        const c = pick(r + 1, k + 1, side);
        const d = pick(r + 1, k, side);
        const [wa, wb, wc, wd] = [o.weights(r, k), o.weights(r, k + 1), o.weights(r + 1, k + 1), o.weights(r + 1, k)];
        middle.copy(a).add(b).add(c).add(d).multiplyScalar(0.25);
        const t = o.paint(r, k, middle, side);
        if ((r + k) % 2 === 0) {
          s.facing(a, b, c, inside, t, [wa, wb, wc]);
          s.facing(a, c, d, inside, t, [wa, wc, wd]);
        } else {
          s.facing(a, b, d, inside, t, [wa, wb, wd]);
          s.facing(b, c, d, inside, t, [wb, wc, wd]);
        }
      }
    }
  }
  const cap = (r: number, tip: V3, toward: number, ring: number) => {
    const inside = centre(r).lerp(centre(r + toward), 0.3);
    const w = o.weights(r, 0);
    for (const side of [1, -1] as const) {
      for (let k = 0; k + 1 < rows; k++) {
        const a = pick(r, k, side);
        const b = pick(r, k + 1, side);
        middle.copy(a).add(b).add(tip).divideScalar(3);
        s.facing(a, b, tip, inside, o.paint(ring, k, middle, side), [o.weights(r, k), o.weights(r, k + 1), w]);
      }
    }
  };
  cap(0, o.back, 1, -1);
  cap(count - 1, o.front, -1, count - 1);
}

interface SegmentOptions {
  sides: number;
  turn?: number;
  across?: V3; // its rings' sideways axis (the right leg's); level and square to it by default
  start?: Cap;
  end?: Cap;
  paint: (ring: number, angle: number, corner: number) => Tone;
}

// A piece of a leg from `from` to `to` (the right leg's; mirrored for the
// left), with rings at `t` along it (0 at from, 1 at to), [t, w, up, down]
// in pixels: `w` across and `up`/`down` square to that.
function segment(s: Sculpt, side: 1 | -1, from: V3, to: V3, stations: readonly (readonly number[])[], o: SegmentOptions): void {
  const mirror = (v: V3) => new THREE.Vector3(v.x * side, v.y, v.z);
  const a = mirror(from);
  const b = mirror(to);
  const t = to.clone().sub(from).normalize();
  const across = o.across?.clone() ?? new THREE.Vector3(t.z, 0, -t.x);
  if (across.lengthSq() < 1e-6) across.set(1, 0, 0);
  const rings = stations.map(([f, w, up, down]) => ({ at: a.clone().lerp(b, f), w: w * U, up: up * U, down: (down ?? up) * U }));
  loft(s, rings, { sides: o.sides, turn: o.turn, side: mirror(across.normalize()), mirror: side < 0, start: o.start, end: o.end, paint: o.paint });
}

// A toe or a finger from `base` along `dir` (the right foot's), `length` px
// long and `thick` px round, with a round pad at its tip.
function digit(s: Sculpt, side: 1 | -1, base: V3, dir: V3, length: number, thick: number): void {
  const tip = base.clone().addScaledVector(dir, length * U);
  segment(
    s,
    side,
    base,
    tip,
    [
      [0, thick, thick * 0.72],
      [0.62, thick * 0.74, thick * 0.6],
      [0.86, thick * 1.28, thick * 0.9],
    ],
    { sides: 4, turn: Math.PI / 4, start: 'flat', end: thick * 0.9 * U, paint: (r) => (r >= 2 ? tone('tip') : r >= 1 ? blend('foot', 'tip', 0.3) : tone('foot')) },
  );
}

// Level directions round from straight ahead (radians, > 0 toward the
// right), dipping `drop` px over `length`.
function heading(angle: number, length: number, drop: number): V3 {
  return new THREE.Vector3(Math.sin(angle) * length, -drop, Math.cos(angle) * length).normalize();
}

// The hind toes (the right foot's), inner to outer: [angle, length, thick].
const TOES: readonly (readonly [number, number, number])[] = [
  [-0.52, 24, 4.3],
  [-0.17, 32, 4.5],
  [0.14, 38, 4.6],
  [0.45, 42, 4.7],
  [0.86, 32, 4.5],
];
// The fingers (the right hand's), inner to outer.
const FINGERS: readonly (readonly [number, number, number])[] = [
  [-0.85, 30, 4.4],
  [-0.28, 40, 4.6],
  [0.26, 38, 4.6],
  [0.85, 32, 4.4],
];

// ---------------------------------------------------------------- limbs

interface Limb {
  front: boolean;
  side: 1 | -1;
  bones: readonly [THREE.Bone, THREE.Bone, THREE.Bone]; // upper, lower, foot
  parent: THREE.Bone;
  parentAt: V3; // the parent bone's place in the rig, as it sits
  joints: readonly [V3, V3, V3]; // root, middle and end joints as it sits, in the rig's space
  a: number; // m the upper bone is long
  b: number; // and the lower one
  restBend: V3; // which way the middle joint stands off the line from root to end, as it sits
  restN: V3; // square to the leg's plane, as it sits
  restUpper: V3;
  restLower: V3;
  straight: THREE.Quaternion; // the foot's own turn that puts it in line with the lower bone
  height: number; // m its end joint stands above the ground, planted
  down: boolean;
  anchor: V3; // where its end joint is planted, in the frog's parent's space
  yaw: number; // the frog's heading when it was planted
  target: V3; // where its end joint is this frame, in the rig's space
  foot: THREE.Quaternion; // the foot bone's own turn while it is up
  // Set by the behaviour every frame: stay down (or come back down under
  // its place), move freely toward `goal`, or come down on the ground as it
  // reaches it.
  want: 'down' | 'free' | 'land';
  goal: V3; // in the rig's space
  bend: number; // 0: its foot as it sits, 1: straight out along the leg
  rate: number; // a second, how fast it follows its goal
  end: V3; // where its end joint reached this frame, in the rig's space
  sole: V3[]; // its foot's corners from its end joint, as it sits: to keep them off the ground
}

// The first index of each distinct corner of a geometry.
function cornersOf(geometry: THREE.BufferGeometry): number[] {
  const position = geometry.getAttribute('position');
  const seen = new Map<string, number>();
  for (let i = 0; i < position.count; i++) {
    const key = [position.getX(i), position.getY(i), position.getZ(i)].map((x) => Math.round(x * 1e5)).join(',');
    if (!seen.has(key)) seen.set(key, i);
  }
  return [...seen.values()];
}

const keelPoint = new THREE.Vector3();

// The corners of a leg's geometry on its foot bone (skin index `bone`), from
// its end joint `joint`, each once.
function soleOf(geometry: THREE.BufferGeometry, bone: number, joint: V3): V3[] {
  const position = geometry.getAttribute('position');
  const skin = geometry.getAttribute('skinIndex');
  const seen = new Map<string, V3>();
  for (let i = 0; i < position.count; i++) {
    if (skin.getX(i) !== bone) continue;
    const p = new THREE.Vector3().fromBufferAttribute(position, i).sub(joint);
    seen.set(p.toArray().map((x) => Math.round(x * 1e5)).join(','), p);
  }
  return [...seen.values()];
}

type Mode = 'idle' | 'walk' | 'jump' | 'swim';
type Stage = 'gather' | 'push' | 'air' | 'land';

const X = new THREE.Vector3(1, 0, 0);
const Y = new THREE.Vector3(0, 1, 0);
const alignFrom = new THREE.Matrix4();
const alignTo = new THREE.Matrix4();
const alignSide = new THREE.Vector3();

// The turn that takes the frame (u0, n0) to (u1, n1): unit vectors, each n
// square to its u.
function align(u0: V3, n0: V3, u1: V3, n1: V3, out: THREE.Quaternion): THREE.Quaternion {
  alignFrom.makeBasis(u0, n0, alignSide.crossVectors(u0, n0));
  alignTo.makeBasis(u1, n1, alignSide.crossVectors(u1, n1));
  return out.setFromRotationMatrix(alignTo.multiply(alignFrom.transpose()));
}

// Turns a leg's bend (unit, square to `dir`) about the line from its root
// along `dir`, up, just as far as puts its middle joint (`along` down that
// line and `out` off it) at the height `lowest` or higher, where it can.
function lift(bend: V3, dir: V3, root: V3, along: number, out: number, lowest: number): void {
  if (out < 1e-9 || root.y + dir.y * along + bend.y * out >= lowest) return;
  const up = new THREE.Vector3(0, 1, 0).addScaledVector(dir, -dir.y);
  const h = up.length();
  if (h < 1e-6) return;
  up.divideScalar(h); // its y is h; `side` is level
  const side = new THREE.Vector3().crossVectors(dir, up);
  const need = Math.min(1, (lowest - root.y - dir.y * along) / (out * h));
  const which = bend.dot(side) < 0 ? -1 : 1;
  bend.copy(up).multiplyScalar(need).addScaledVector(side, which * Math.sqrt(Math.max(0, 1 - need * need)));
}

const soleTurn = new THREE.Quaternion();
const soleTry = new THREE.Quaternion();
const soleFrom = new THREE.Quaternion();
const solePoint = new THREE.Vector3();

// The height of a free foot's lowest corner, its lower bone turned `lower`
// and the foot `local` from it (in the rig's space).
function soleHeight(limb: Limb, lower: THREE.Quaternion, local: THREE.Quaternion): number {
  soleTurn.multiplyQuaternions(lower, local);
  let low = Infinity;
  for (const corner of limb.sole) low = Math.min(low, solePoint.copy(corner).applyQuaternion(soleTurn).y);
  return limb.end.y + low;
}

// Turns a free foot (`shown`, from its lower bone) toward `flat` just as far
// as keeps its lowest corner at `floor` or higher, where it can.
function keepUp(limb: Limb, lower: THREE.Quaternion, shown: THREE.Quaternion, flat: THREE.Quaternion, floor: number): void {
  if (soleHeight(limb, lower, shown) >= floor) return;
  soleFrom.copy(shown);
  let lo = 0;
  let hi = 1;
  for (let i = 0; i < 10; i++) {
    const mid = (lo + hi) / 2;
    if (soleHeight(limb, lower, soleTry.copy(soleFrom).slerp(flat, mid)) >= floor) hi = mid;
    else lo = mid;
  }
  shown.copy(soleFrom).slerp(flat, hi);
}

export class ForestFrog extends ForestAnimal {
  // The sheet's colour variations, the default (green) first.
  static readonly COLORS = Object.keys(VARIATIONS);
  static readonly LENGTH = 0.08; // m from the snout to the rump, sitting

  readonly body: THREE.SkinnedMesh;
  readonly leftEye = new THREE.Group();
  readonly rightEye = new THREE.Group();
  readonly leftFrontLeg: THREE.SkinnedMesh;
  readonly rightFrontLeg: THREE.SkinnedMesh;
  readonly leftHindLeg: THREE.SkinnedMesh;
  readonly rightHindLeg: THREE.SkinnedMesh;

  // In the water (the preview sets it where there is water): it floats
  // rather than sits, and swims rather than hops; its surface is y = 0 of
  // its parent, where its origin is.
  inWater = false;
  // Called where it breaks the water's surface (a kick, leaving or coming
  // back in on a leap), in its parent's space, moving at `velocity` m/s.
  onSplash: ((x: number, z: number, velocity: THREE.Vector3) => void) | null = null;

  private readonly spine: { pelvis: THREE.Bone; trunk: THREE.Bone; head: THREE.Bone; throat: THREE.Bone };
  private readonly limbs: Limb[];
  private readonly eyes: { group: THREE.Group; rest: V3; look: V3 }[];
  private readonly keel: number[]; // its body's corners, each once, to keep them off the ground
  private readonly random: () => number;
  private readonly posture: Posture = { ...SIT };
  private readonly goal: Posture = { ...SIT };
  private poseRate = 6;
  private mode: Mode = 'idle';
  private next: Mode | null = null;
  private time = 0;
  private stage: Stage = 'gather';
  private stageTime = 0;
  private leap: Leap = HOP;
  private hop = 0; // m its hop lifts it
  private rise = 0; // m/s its hop's upward speed
  private stroke = 0; // 0..1 through a swimming stroke
  private blinkAt = 2.2;
  private blinkTime = -1;
  private throatPulse = 0;
  private baseHeight = 0; // m the rig stands: 0 on land, lower afloat, higher swimming on dry ground
  private started = false;
  // This frame's: from the rig's space to the frog's parent's and back, and
  // from the world to the rig's space.
  private readonly toWorld = new THREE.Matrix4();
  private readonly toRig = new THREE.Matrix4();
  private readonly rigInverse = new THREE.Matrix4();

  constructor(options: ForestAnimalOptions = {}) {
    super(VARIATIONS, options);
    this.random = seededRandom(options.seed ?? 11);
    const palette = this.palette;
    const material = coat();
    const shine = gloss(0.22);

    // The spine.
    const pelvis = bone('pelvis', this.rig, PELVIS);
    const trunk = bone('trunk', pelvis, SPINE, PELVIS);
    const head = bone('head', trunk, HEAD, SPINE);
    const throat = bone('throat', head, THROAT, HEAD);
    this.spine = { pelvis, trunk, head, throat };
    const all: THREE.Bone[] = [pelvis, trunk, head, throat];

    // The legs' bones and what they need to be placed.
    const specs: { front: boolean; side: 1 | -1 }[] = [
      { front: true, side: -1 },
      { front: true, side: 1 },
      { front: false, side: -1 },
      { front: false, side: 1 },
    ];
    this.limbs = specs.map(({ front, side }) => {
      const mirror = (v: V3) => new THREE.Vector3(v.x * side, v.y, v.z);
      const [j0, j1, j2] = (front ? [SHOULDER, ELBOW, WRIST] : [HIP, KNEE, HEEL]).map(mirror);
      const tip = mirror(front ? PALM : BALL);
      const parent = front ? trunk : pelvis;
      const parentAt = front ? SPINE : PELVIS;
      const name = `${side < 0 ? 'left' : 'right'}${front ? 'Front' : 'Hind'}`;
      const b0 = bone(`${name}Upper`, parent, j0, parentAt);
      const b1 = bone(`${name}Lower`, b0, j1, j0);
      const b2 = bone(`${name}Foot`, b1, j2, j1);
      all.push(b0, b1, b2);
      const dir = j2.clone().sub(j0).normalize();
      const restBend = j1.clone().sub(j0);
      restBend.addScaledVector(dir, -restBend.dot(dir)).normalize();
      const restLower = j2.clone().sub(j1).normalize();
      const footDir = tip.clone().sub(j2).normalize();
      return {
        front,
        side,
        bones: [b0, b1, b2],
        parent,
        parentAt,
        joints: [j0, j1, j2],
        a: j0.distanceTo(j1),
        b: j1.distanceTo(j2),
        restBend,
        restN: new THREE.Vector3().crossVectors(dir, restBend).normalize(),
        restUpper: j1.clone().sub(j0).normalize(),
        restLower,
        straight: new THREE.Quaternion().setFromUnitVectors(footDir, restLower),
        height: j2.y,
        down: true,
        anchor: new THREE.Vector3(),
        yaw: 0,
        target: j2.clone(),
        foot: new THREE.Quaternion(),
        want: 'down',
        goal: j2.clone(),
        bend: 0,
        rate: 12,
        end: j2.clone(),
        sole: [],
      } satisfies Limb;
    });

    this.updateMatrixWorld(true);
    const skeleton = new THREE.Skeleton(all);
    const index = (b: THREE.Bone) => all.indexOf(b);

    // The body and head: one surface through the sections, bending with the
    // spine, the throat's underside with the throat's bone.
    const s = new Sculpt(this.random, 0.03);
    const rings = SECTIONS.map(sectionPoints);
    const [bP, bT, bH, bThroat] = [pelvis, trunk, head, throat].map(index);
    const along = (z: number): Weights => {
      if (z <= -88) return [bP, bP, 0];
      if (z < -52) return [bP, bT, (z + 88) / 36];
      if (z <= 10) return [bT, bT, 0];
      if (z < 34) return [bT, bH, (z - 10) / 24];
      return [bH, bH, 0];
    };
    const weights = (r: number, k: number): Weights => {
      const z = SECTIONS[r].z;
      if (z < 34) return along(z);
      const row = k >= 9 ? 1 : k === 8 ? 0.75 : k === 7 ? 0.35 : 0;
      const share = smooth((z - 58) / 14) * (1 - smooth((z - 100) / 14));
      return [bH, bThroat, row * share];
    };
    hull(s, rings, {
      back: RUMP_TIP,
      front: SNOUT_TIP,
      weights,
      paint: (r, k, _m, side) => {
        const last = SECTIONS.length - 1;
        const here = SECTIONS[Math.max(0, Math.min(last, r))];
        const ahead = SECTIONS[Math.max(0, Math.min(last, r + 1))];
        if ((r >= 0 && here.lip && ahead.lip) || r === last) {
          // The head: green above the pale band of the upper lip, the mouth
          // line, and the cream jaw, throat and chest.
          if (k <= 3) return skin(r, k, side);
          if (k === 4) return blend('light', 'skin', 0.15);
          if (k === 5) return tone('mouth');
          return k >= 8 && here.z < 62 ? blend('cream', 'belly', 0.3) : tone('cream');
        }
        // The trunk: green, paler along the flanks, the belly cream; behind,
        // where the thighs are, green nearly to the ground.
        if (k <= 3) return skin(r, k, side);
        if (here.z < -70 || r < 0) return k >= 9 ? blend('cream', 'belly', 0.5) : k >= 6 ? blend('skin', 'light', 0.4) : skin(r, k, side);
        if (k === 4) return blend('skin', 'light', 0.55);
        if (k === 5) return here.z < -40 ? blend('light', 'skin', 0.3) : blend('light', 'cream', 0.3);
        if (k >= 8) return blend('cream', 'belly', 0.5);
        return tone('cream');
      },
    });

    // The eyes' lids: a big bump on each side of the head's top, green, with
    // the eye on its outer side.
    s.weights = [bH, bH, 0];
    for (const side of [-1, 1] as const) {
      const centre = P(60, 139, side * 22);
      const basis = new THREE.Matrix4().makeBasis(new THREE.Vector3(0, 0, -side), new THREE.Vector3(0, 1, 0), new THREE.Vector3(side, 0, 0));
      ellipsoid(s, centre, new THREE.Vector3(24 * U, 29 * U, 19 * U), basis, 4, 10, (latitude, angle) => {
        const top = Math.min(angle, 2 * Math.PI - angle);
        if (latitude > 0.55) return blend('light', 'lid', 0.35);
        return top < 0.9 ? blend('skin', 'light', 0.45) : hash(latitude * 7, angle, side) < 0.25 ? blend('skin', 'light', 0.4) : tone('skin');
      });
    }
    // Where the eyeballs sit: on each bump, where a ray from outside meets it.
    const eyeAt = [-1, 1].map((side) => {
      const look = new THREE.Vector3(side * 0.62, 0.36, 0.7).normalize();
      const from = EYE_AT.clone().setX(EYE_AT.x * side).addScaledVector(look, 40 * U);
      return { look, at: s.onto(from, look.clone().negate()).addScaledVector(look, -3.5 * U) };
    });
    // The nostrils: two dark dots on the snout.
    for (const side of [-1, 1] as const) {
      const at = s.onto(P(110, 170, side * 11), new THREE.Vector3(0, -1, 0));
      const normal = new THREE.Vector3(side * 0.25, 0.85, 0.45).normalize();
      at.addScaledVector(normal, 0.3 * U);
      const across = new THREE.Vector3(1, 0, 0);
      const up = new THREE.Vector3().crossVectors(normal, across).normalize();
      const r = 2.2 * U;
      const corners = [0, 1, 2, 3, 4].map((i) => {
        const a = (i / 5) * Math.PI * 2;
        return at.clone().addScaledVector(across, Math.cos(a) * r * 1.2).addScaledVector(up, Math.sin(a) * r);
      });
      for (let i = 0; i < 5; i++) s.toward(at, corners[i], corners[(i + 1) % 5], normal, tone('nostril'));
    }
    // The spots, and the poison dart frogs' patches.
    decals(s, SPOTS, tone('spot'), along, this.random);
    decals(s, MARKS, tone('mark'), along, this.random);
    const bodyGeometry = s.geometry(palette, true);
    this.body = this.painting(skinnedMesh(bodyGeometry, material, skeleton));
    this.keel = cornersOf(bodyGeometry);

    // The eyeballs: big and round, dark brown round a black pupil, with a
    // glint, in a thin golden lid; each its own part, on the head's bone.
    this.eyes = [this.leftEye, this.rightEye].map((group, i) => {
      const { look, at } = eyeAt[i];
      const e = new Sculpt(this.random, 0.012);
      eye(e, at, look, new THREE.Vector3(0, 1, 0), EYE_RADIUS, { lid: tone('lid'), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 12, 0.56, 0.9);
      const geometry = e.geometry(palette);
      seatGlint(geometry, look, EYE_GLINT_DROP * EYE_RADIUS);
      const mesh = this.painting(rigidMesh(geometry.applyMatrix4(new THREE.Matrix4().makeTranslation(-at.x, -at.y, -at.z)), shine));
      group.add(mesh);
      group.position.copy(at).sub(HEAD);
      head.add(group);
      return { group, rest: group.position.clone(), look };
    });

    // The legs: each piece rigid on its bone.
    const legMeshes = this.limbs.map((limb) => {
      const g = new Sculpt(this.random, 0.035);
      const [i0, i1, i2] = limb.bones.map(index);
      if (limb.front) frontLeg(g, limb.side, [i0, i1, i2]);
      else hindLeg(g, limb.side, [i0, i1, i2]);
      const geometry = g.geometry(palette, true);
      limb.sole.push(...soleOf(geometry, i2, limb.joints[2]));
      return this.painting(skinnedMesh(geometry, material, skeleton));
    });
    [this.leftFrontLeg, this.rightFrontLeg, this.leftHindLeg, this.rightHindLeg] = legMeshes;
    this.rig.add(this.body, ...legMeshes);
  }

  // Sits still, its throat pulsing, blinking now and then; afloat in the
  // water, it hangs at the surface, its legs spread. Named as in its
  // sheet's poses.
  Idle(): void {
    this.request('idle');
  }

  // Small crawling hops forward, low and level, the way it faces, until told
  // otherwise; in the water it swims instead. Named as in its sheet's poses.
  Walk(): void {
    this.request(this.inWater ? 'swim' : 'walk');
  }

  // One leap forward: it crouches (the sheet's Jump (Start)), leaps in an
  // arc with its hind legs stretched out behind (Jump (Air)), lands on its
  // front feet (Jump (Land)) and sits again, then idles. From the water, it
  // leaps out and dives back in. Named as in its sheet's poses.
  Jump(): void {
    if (this.mode === 'jump' || this.next === 'jump') return;
    this.request('jump');
  }

  // Swims forward at the water's surface (y = 0 of its parent), its body
  // flat, its hind legs kicking together frog-style and its front legs
  // tucked back, until told otherwise. On dry ground it does the same on
  // its belly. Named as in its sheet's poses.
  Swim(): void {
    this.request('swim');
  }

  // What it is doing: its behaviour, from the sheet's poses.
  get doing(): Mode {
    return this.mode;
  }

  // Off the ground (or out of the water) in a hop or a leap.
  get airborne(): boolean {
    return (this.mode === 'walk' || this.mode === 'jump') && (this.stage === 'air' || (this.stage === 'push' && this.inWater));
  }

  private request(mode: Mode): void {
    // A hop or a leap under way ends first.
    if ((this.mode === 'walk' || this.mode === 'jump') && this.stage !== 'gather') {
      this.next = mode;
      return;
    }
    this.start(mode);
  }

  private start(mode: Mode): void {
    this.next = null;
    this.mode = mode;
    if (mode === 'walk' || mode === 'jump') {
      this.leap = mode === 'jump' ? LEAP : HOP;
      this.stage = 'gather';
      this.stageTime = 0;
    }
  }

  update(delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    this.time += dt;
    this.frame();
    if (!this.started) {
      this.started = true;
      for (const limb of this.limbs) this.plantAt(limb, limb.joints[2].clone().applyMatrix4(this.toWorld));
    }

    // What it does this frame: its speed, its hop, its posture's goal and
    // its legs' wants.
    Object.assign(this.goal, SIT);
    this.throatPulse = 0;
    if (this.mode === 'swim') this.swim(dt);
    else if (this.mode === 'idle') this.idle(dt);
    else this.hopping(dt);
    this.advance(dt);
    for (const key of KEYS) this.posture[key] = ease(this.posture[key], this.goal[key], this.poseRate, dt);

    // The spine, then the legs.
    const base = this.inWater ? FLOAT : this.mode === 'swim' ? LYING : 0;
    this.baseHeight = ease(this.baseHeight, base, 5, dt);
    this.rig.position.y = this.baseHeight + this.hop;
    const p = this.posture;
    const { pelvis, trunk, head, throat } = this.spine;
    pelvis.position.set(PELVIS.x, PELVIS.y + p.lift, PELVIS.z + p.shift);
    pelvis.rotation.x = p.pitch;
    trunk.rotation.x = p.flex;
    head.rotation.x = p.nod;
    const pulse = this.throatPulse;
    throat.scale.set(1 + 0.06 * pulse, 1 + 0.3 * pulse, 1 + 0.04 * pulse);
    this.frame();
    // On land its body never goes into the ground: it rides up over its
    // legs as far as that needs (landing on its chest, say).
    if (!this.inWater) {
      const under = -this.rig.position.y + BODY_CLEAR - this.bodyLow();
      if (under > 0) {
        this.rig.position.y += under;
        this.frame();
      }
    }
    for (const limb of this.limbs) this.place(limb, dt);
    this.blink(dt);
    this.rig.updateMatrixWorld(true);
  }

  // The lowest corner of its body as posed now, in the rig's space.
  private bodyLow(): number {
    let low = Infinity;
    for (const i of this.keel) low = Math.min(low, this.body.getVertexPosition(i, keelPoint).y);
    return low;
  }

  // Brings its matrices up to date for this frame.
  private frame(): void {
    this.updateMatrix();
    this.rig.updateMatrix();
    this.rig.updateMatrixWorld(true);
    this.toWorld.multiplyMatrices(this.matrix, this.rig.matrix);
    this.toRig.copy(this.toWorld).invert();
    this.rigInverse.copy(this.rig.matrixWorld).invert();
  }

  // ---------------------------------------------------------------- behaviours

  private idle(dt: number): void {
    this.speedNow = 0;
    this.hop = ease(this.hop, 0, 8, dt);
    this.poseRate = 5;
    this.throatPulse = 0.5 + 0.5 * Math.sin(this.time * Math.PI * 2 * 1.7);
    if (this.inWater) {
      // Hanging at the surface, its head up, its legs spread and trailing.
      this.goal.pitch = 0.3;
      this.goal.nod = -0.1;
      this.goal.lift = 1.2 * U * Math.sin(this.time * 1.6);
      for (const limb of this.limbs) this.reach(limb, limb.front ? FLOAT_FRONT : FLOAT_HIND, limb.front ? 0.2 : 0.4, 4);
      return;
    }
    this.goal.lift = 0.5 * U * Math.sin(this.time * 1.3);
    for (const limb of this.limbs) limb.want = 'down';
  }

  private hopping(dt: number): void {
    const spec = this.leap;
    this.stageTime += dt;
    const t = this.stageTime;
    const water = this.inWater;
    switch (this.stage) {
      case 'gather': {
        Object.assign(this.goal, spec.crouch);
        this.poseRate = 9;
        this.speedNow = 0;
        this.hop = ease(this.hop, 0, 10, dt);
        this.throatPulse = this.mode === 'jump' ? 0 : 0.3;
        if (water) for (const limb of this.limbs) this.reach(limb, limb.front ? FLOAT_FRONT : FOLDED_HIND, 0, 10);
        else for (const limb of this.limbs) limb.want = 'down';
        const ready = water || this.limbs.every((l) => l.down);
        if (t >= spec.gather && ready) this.enter('push');
        break;
      }
      case 'push': {
        const f = Math.min(1, t / spec.push);
        this.speedNow = spec.vx * f;
        this.hop = (spec.vy * spec.push * f * f) / 2;
        this.goal.pitch = spec.pitch[0];
        this.goal.nod = -0.1;
        this.poseRate = 16;
        for (const limb of this.limbs) {
          if (limb.front) this.reach(limb, spec.front, 0, 16);
          else if (water) this.reach(limb, spec.hind, spec.straight, 30);
          else limb.want = 'down';
        }
        if (t >= spec.push) {
          this.rise = spec.vy;
          this.enter('air');
          if (water) this.splash(0.3);
        }
        break;
      }
      case 'air': {
        this.speedNow = spec.vx;
        this.hop += this.rise * dt - (G * dt * dt) / 2;
        this.rise -= G * dt;
        const top = spec.vy / G;
        const fall = Math.sqrt((2 * ((spec.vy * spec.push) / 2 + (spec.vy * spec.vy) / (2 * G))) / G);
        const f = Math.min(1, t / (top + fall));
        this.goal.pitch = spec.pitch[0] + (spec.pitch[1] - spec.pitch[0]) * smooth(f * 1.2);
        this.goal.nod = -0.1;
        this.poseRate = 12;
        // Rising, its hind legs stretch out behind; coming down, they fold
        // back under it and the front feet reach for the ground, ahead by
        // as far as it slides on landing.
        for (const limb of this.limbs) {
          if (this.rise > 0 || water) this.reach(limb, limb.front ? spec.front : spec.hind, limb.front ? 0 : spec.straight, limb.front ? 14 : 22);
          else if (limb.front) this.landing(limb, slid(spec, spec.stop));
          else this.fold(limb, spec.fold);
        }
        if (water) {
          if (this.hop <= 0 && this.rise < 0) {
            this.splash(1);
            this.enter('land');
          }
        } else if (this.limbs.some((l) => l.front && l.down) || (this.hop <= 0 && this.rise < 0)) {
          // Down: on a front foot, or back at the height it sits (the front
          // feet not down yet set down where they are).
          this.hop = Math.max(this.hop, 0);
          for (const limb of this.limbs) if (limb.front && !limb.down) this.plantAt(limb, null);
          // This frame is the first of its landing: it slides only as far.
          this.enter('land');
          this.stageTime = dt;
          this.speedNow = dt > 0 ? slid(spec, dt) / dt : 0;
        }
        break;
      }
      case 'land': {
        // On land it slides on over its front feet, slowing to a stop,
        // exactly as far whatever the frame rate.
        if (water) this.speedNow = spec.vx * Math.exp(-t * 14);
        else this.speedNow = dt > 0 ? (slid(spec, t) - slid(spec, t - dt)) / dt : 0;
        this.hop = water ? ease(this.hop, 0, 6, dt) : ease(this.hop, 0, 16, dt);
        const settled = this.mode === 'walk' ? spec.crouch : SIT;
        Object.assign(this.goal, settled);
        this.poseRate = 8;
        for (const limb of this.limbs) {
          if (water) this.reach(limb, limb.front ? FLOAT_FRONT : FLOAT_HIND, 0.4, 5);
          else if (limb.front) limb.want = 'down';
          else this.landing(limb, slid(spec, spec.stop) - slid(spec, t));
        }
        const down = water || this.limbs.every((l) => l.down);
        if (t >= spec.settle && down) {
          if (this.next) this.start(this.next);
          else if (this.mode === 'jump') this.start('idle');
          else this.enter('gather');
        }
        break;
      }
    }
  }

  private enter(stage: Stage): void {
    this.stage = stage;
    this.stageTime = 0;
  }

  private swim(dt: number): void {
    this.stroke = (this.stroke + dt / SWIM_STROKE) % 1;
    const u = this.stroke;
    // The kick (a fifth of the stroke), a glide with the legs together, and
    // the legs drawn up again, slowly.
    const reach = u < 0.22 ? smooth(u / 0.22) : u < 0.45 ? 1 : 1 - smooth((u - 0.45) / 0.55);
    const surge = u < 0.22 ? smooth(u / 0.22) : Math.exp(-(u - 0.22) * 4.5);
    this.speedNow = ease(this.speedNow, SWIM_SLOW + (SWIM_FAST - SWIM_SLOW) * surge, 10, dt);
    this.hop = ease(this.hop, 0, 8, dt);
    this.goal.pitch = 0.36;
    this.goal.nod = -0.12;
    this.goal.lift = 0;
    this.poseRate = 6;
    const hind = new THREE.Vector3().lerpVectors(FOLDED_HIND, KICKED_HIND, reach);
    for (const limb of this.limbs) {
      if (limb.front) this.reach(limb, TUCKED_FRONT, 1, 8);
      else this.reach(limb, hind, 0.15 + 0.85 * reach, 18);
    }
    const before = this.stroke - dt / SWIM_STROKE;
    if (this.inWater && before < 0.05 && this.stroke >= 0.05) this.splash(0.35);
  }

  // ---------------------------------------------------------------- legs

  // Moves a leg freely toward a place along the body, `at` px from its root
  // (x outward, y up and z forward along the body as it sits), its foot
  // `bend` of the way to straight out, following at `rate` a second.
  private reach(limb: Limb, at: V3, bend: number, rate: number): void {
    limb.want = 'free';
    limb.bend = bend;
    limb.rate = rate;
    this.alongBody(limb, at, limb.goal);
  }

  // Where a place `at` px from a leg's root along the body is, in the rig's
  // space.
  private alongBody(limb: Limb, at: V3, out: V3): V3 {
    out.set(at.x * limb.side, at.y, at.z).multiplyScalar(U).applyAxisAngle(X, -BODY_TILT);
    out.add(limb.joints[0]).sub(limb.parentAt);
    return out.applyMatrix4(this.parentMatrix(limb));
  }

  // A leg's parent bone's matrix, in the rig's space.
  private parentMatrix(limb: Limb): THREE.Matrix4 {
    return new THREE.Matrix4().multiplyMatrices(this.rigInverse, limb.parent.matrixWorld);
  }

  // Moves a leg freely back to where its foot sits on the body, following at
  // `rate` a second.
  private fold(limb: Limb, rate: number): void {
    limb.want = 'free';
    limb.bend = 0;
    limb.rate = rate;
    limb.goal.copy(limb.joints[2]).sub(limb.parentAt).applyMatrix4(this.parentMatrix(limb));
  }

  // Brings a leg down to the ground under where its foot sits on the body,
  // `ahead` m further forward, to plant it there as it arrives.
  private landing(limb: Limb, ahead: number): void {
    limb.want = 'land';
    limb.bend = 0;
    limb.rate = 16;
    // Where it sits, whatever the body's pose now: it sits again once down.
    const spot = limb.goal.copy(limb.joints[2]).applyMatrix4(this.toWorld);
    spot.y = this.position.y + limb.height;
    spot.x += Math.sin(this.rotation.y) * ahead;
    spot.z += Math.cos(this.rotation.y) * ahead;
    spot.applyMatrix4(this.toRig);
  }

  // Plants a leg's end on the ground where it is now, or at `world` (in the
  // frog's parent's space).
  private plantAt(limb: Limb, world: V3 | null): void {
    if (world) limb.anchor.copy(world);
    else limb.anchor.copy(limb.end).applyMatrix4(this.toWorld);
    limb.anchor.y = this.position.y + limb.height;
    limb.yaw = this.rotation.y;
    limb.down = true;
  }

  // Places a leg for this frame: its end planted or following its goal, and
  // its bones reaching it.
  private place(limb: Limb, dt: number): void {
    const parent = this.parentMatrix(limb);
    const parentTurn = new THREE.Quaternion().setFromRotationMatrix(parent);
    const root = limb.joints[0].clone().sub(limb.parentAt).applyMatrix4(parent);
    const most = limb.a + limb.b;
    // On land (flat, at its origin's height): the ground in the rig's space.
    const land = !this.inWater;
    const ground = -this.rig.position.y;

    let want = limb.want;
    if (this.inWater) want = 'free';
    if (want === 'down' && !limb.down) {
      // Back down under where it sits.
      this.landing(limb, 0);
      want = 'land';
    }
    if (want === 'free' && limb.down) limb.down = false;
    if (limb.down) {
      limb.target.copy(limb.anchor).applyMatrix4(this.toRig);
      // A foot left far behind (the frog put somewhere else) is set down
      // under it again.
      if (limb.target.distanceTo(root) > 1.6 * most) {
        this.plantAt(limb, limb.joints[2].clone().sub(limb.parentAt).applyMatrix4(parent).applyMatrix4(this.toWorld));
        limb.target.copy(limb.anchor).applyMatrix4(this.toRig);
      }
    } else {
      const k = 1 - Math.exp(-limb.rate * dt);
      limb.target.lerp(limb.goal, k);
      limb.foot.slerp(new THREE.Quaternion().slerp(limb.straight, limb.bend), k);
      // On land its end joint goes no lower than it stands planted.
      if (land) limb.target.y = Math.max(limb.target.y, ground + limb.height);
    }

    // Two bones reaching from the root to the target, the middle joint
    // bending the way it stood as it sat, turned with the body.
    const pole = limb.restBend.clone().applyQuaternion(parentTurn);
    const dir = new THREE.Vector3().subVectors(limb.target, root);
    let d = dir.length();
    if (d < 1e-9) dir.set(0, -1, 0);
    else dir.divideScalar(d);
    d = THREE.MathUtils.clamp(d, Math.abs(limb.a - limb.b) * 1.001 + 1e-7, most * 0.9995);
    const bend = pole.addScaledVector(dir, -pole.dot(dir));
    if (bend.lengthSq() < 1e-12) bend.crossVectors(limb.restN, dir);
    bend.normalize();
    const cos = THREE.MathUtils.clamp((limb.a * limb.a + d * d - limb.b * limb.b) / (2 * limb.a * d), -1, 1);
    const sin = Math.sqrt(1 - cos * cos);
    // On land its middle joint (a knee or an elbow) stays off the ground.
    if (land) lift(bend, dir, root, limb.a * cos, limb.a * sin, ground + JOINT_CLEAR);
    const mid = root.clone().addScaledVector(dir, limb.a * cos).addScaledVector(bend, limb.a * sin);
    limb.end.copy(root).addScaledVector(dir, d);
    const n = new THREE.Vector3().crossVectors(dir, bend).normalize();
    const upper = align(limb.restUpper, limb.restN, mid.clone().sub(root).normalize(), n, new THREE.Quaternion());
    const lower = align(limb.restLower, limb.restN, limb.end.clone().sub(mid).normalize(), n, new THREE.Quaternion());
    const [b0, b1, b2] = limb.bones;
    b0.quaternion.copy(parentTurn).invert().multiply(upper);
    b1.quaternion.copy(upper).invert().multiply(lower);
    if (limb.down) {
      // Flat on the ground, turned as the frog was when it came down.
      const flat = new THREE.Quaternion().setFromAxisAngle(Y, limb.yaw - this.rotation.y);
      b2.quaternion.copy(lower).invert().multiply(flat);
      limb.foot.copy(b2.quaternion);
    } else {
      const shown = b2.quaternion.copy(limb.foot);
      if (land) {
        // Flat on the ground, as it sits.
        const flat = lower.clone().invert();
        // Landing, it turns flat as it nears the ground, to come down flat;
        // and no corner of it ever goes into the ground.
        if (want === 'land') shown.slerp(flat, 1 - THREE.MathUtils.clamp((limb.end.y - ground - limb.height) / FLATTEN, 0, 1));
        keepUp(limb, lower, shown, flat, ground + SOLE_CLEAR);
      }
      // Coming down: planted as it reaches the ground.
      if (want === 'land' && limb.end.clone().applyMatrix4(this.toWorld).y <= this.position.y + limb.height + 0.3 * U) this.plantAt(limb, null);
    }
  }

  private splash(strength: number): void {
    if (!this.onSplash) return;
    const back = -0.03;
    const x = this.position.x + Math.sin(this.rotation.y) * back;
    const z = this.position.z + Math.cos(this.rotation.y) * back;
    const v = new THREE.Vector3(Math.sin(this.rotation.y), 0, Math.cos(this.rotation.y)).multiplyScalar(-0.4 * strength);
    v.y = 0.6 * strength;
    this.onSplash(x, z, v);
  }

  // ---------------------------------------------------------------- the eyes

  private blink(dt: number): void {
    if (this.blinkTime < 0 && this.time >= this.blinkAt) this.blinkTime = 0;
    let shut = 0;
    if (this.blinkTime >= 0) {
      this.blinkTime += dt;
      const f = this.blinkTime / BLINK;
      shut = Math.sin(Math.PI * Math.min(1, f));
      if (f >= 1) {
        this.blinkTime = -1;
        this.blinkAt = this.time + 2.5 + 3 * this.random();
      }
    }
    for (const e of this.eyes) {
      e.group.position.copy(e.rest).addScaledVector(e.look, -0.55 * EYE_RADIUS * shut);
      e.group.scale.set(1, 1 - 0.75 * shut, 1);
    }
  }
}

// Where the legs reach, from their roots along the body (px; x outward).
const FLOAT_FRONT = new THREE.Vector3(34, -60, 16); // afloat: the arms spread
const FLOAT_HIND = new THREE.Vector3(52, -30, -80); // the hind legs hanging open behind
const FOLDED_HIND = new THREE.Vector3(46, -14, -34); // drawn up for a kick, knees out
const KICKED_HIND = new THREE.Vector3(16, -14, -300); // kicked out straight behind, together
const TUCKED_FRONT = new THREE.Vector3(12, -34, -44); // swimming: the arms back along the belly

// ---------------------------------------------------------------- the legs

// A hind leg as it sits: the big thigh forward from the hip to the knee, the
// shin back to the heel, the foot forward along the ground with its five
// webbed toes spread; each piece on its own bone.
function hindLeg(s: Sculpt, side: 1 | -1, [thigh, shin, foot]: readonly number[]): void {
  s.weights = [thigh, thigh, 0];
  segment(
    s,
    side,
    HIP,
    KNEE,
    [
      [-0.1, 13, 14],
      [0.15, 21, 26, 21],
      [0.42, 23, 28, 22],
      [0.7, 19.5, 22.5, 18],
      [0.93, 14, 15, 14],
    ],
    {
      sides: 8,
      turn: Math.PI / 8,
      start: 6 * U,
      end: 9 * U,
      paint: (r, angle, k) => {
        const top = Math.min(angle, 2 * Math.PI - angle);
        if (top > 2.2) return blend('light', 'cream', 0.45);
        if ((r === 2 && k === 0) || (r === 3 && k === 1)) return tone('spot');
        if (r === 1 && (k === 0 || k === 7)) return tone('mark');
        if (r === 3 && k === 7) return tone('mark');
        if (top > 1.5) return blend('skin', 'light', 0.6);
        return hash(r, k, side) < 0.3 ? blend('skin', 'light', 0.35) : tone('skin');
      },
    },
  );
  s.weights = [shin, shin, 0];
  segment(
    s,
    side,
    KNEE,
    HEEL,
    [
      [0.02, 12, 13],
      [0.3, 12.5, 13],
      [0.65, 10, 10.5],
      [0.95, 7.5, 8],
    ],
    {
      sides: 7,
      start: 9 * U,
      end: 8 * U,
      paint: (r, angle, k) => {
        const top = Math.min(angle, 2 * Math.PI - angle);
        if (top > 2.1) return blend('light', 'cream', 0.4);
        if (r === 1 && k <= 1) return tone('mark');
        if (r === 2 && k === 6) return tone('spot');
        return top > 1.3 ? blend('skin', 'light', 0.5) : tone('skin');
      },
    },
  );
  s.weights = [foot, foot, 0];
  segment(
    s,
    side,
    HEEL,
    BALL,
    [
      [0, 8, 7, 5.5],
      [0.5, 9, 5, 4],
      [1, 11, 3.8, 2.8],
    ],
    {
      sides: 6,
      turn: Math.PI / 6,
      start: 5 * U,
      end: 'flat',
      paint: (r, angle) => (Math.abs(angle - Math.PI) < 1 ? blend('light', 'foot', 0.5) : r <= 0 ? tone('skin') : r === 1 ? blend('skin', 'light', 0.5) : blend('light', 'foot', 0.55)),
    },
  );
  // The toes, spread from the ball of the foot, and the webs between them.
  const across = new THREE.Vector3(1, 0, -0.14).normalize();
  const bases: V3[] = [];
  const tips: V3[] = [];
  TOES.forEach(([angle, length, thick], i) => {
    const base = BALL.clone().addScaledVector(across, (i - 2) * 4.2 * U);
    const dir = heading(angle, length, 3);
    digit(s, side, base, dir, length, thick);
    bases.push(base);
    tips.push(base.clone().addScaledVector(dir, length * U));
  });
  const mirror = (v: V3) => new THREE.Vector3(v.x * side, v.y, v.z);
  const up = new THREE.Vector3(0, 1, 0);
  const down = new THREE.Vector3(0, -1, 0);
  for (let i = 0; i + 1 < TOES.length; i++) {
    const hub = mirror(BALL.clone().add(new THREE.Vector3(0, -1 * U, -3 * U)));
    const a = mirror(bases[i].clone().lerp(tips[i], 0.62).add(new THREE.Vector3(0, -0.6 * U, 0)));
    const b = mirror(bases[i + 1].clone().lerp(tips[i + 1], 0.62).add(new THREE.Vector3(0, -0.6 * U, 0)));
    const m = mirror(bases[i].clone().lerp(tips[i], 0.4).lerp(bases[i + 1].clone().lerp(tips[i + 1], 0.4), 0.5).add(new THREE.Vector3(0, -0.8 * U, 0)));
    for (const facing of [up, down]) {
      s.toward(hub, a, m, facing, tone('web'));
      s.toward(hub, m, b, facing, tone('web'));
    }
  }
}

// A front leg as it sits: the upper arm and forearm straight down under the
// chest, the hand flat on the ground with its four fingers splayed.
function frontLeg(s: Sculpt, side: 1 | -1, [upper, lower, hand]: readonly number[]): void {
  const outward = new THREE.Vector3(1, 0, 0);
  s.weights = [upper, upper, 0];
  segment(
    s,
    side,
    SHOULDER,
    ELBOW,
    [
      [-0.18, 12, 13],
      [0.1, 15.5, 16.5],
      [0.55, 13, 14],
      [1.02, 11, 11.5],
    ],
    {
      sides: 7,
      across: outward,
      start: 6 * U,
      end: 6 * U,
      paint: (r, angle) => {
        const front = Math.min(angle, 2 * Math.PI - angle);
        if (r === 1 && angle > 2.6 && angle < 3.6) return tone('mark');
        return front < 1.1 ? blend('skin', 'light', 0.55) : hash(r, angle) < 0.3 ? blend('skin', 'dark', 0.3) : tone('skin');
      },
    },
  );
  s.weights = [lower, lower, 0];
  segment(
    s,
    side,
    ELBOW,
    WRIST,
    [
      [-0.05, 11, 11.5],
      [0.4, 10, 10.5],
      [0.85, 9, 9],
      [1, 8.5, 8.5],
    ],
    {
      sides: 7,
      across: outward,
      start: 5 * U,
      end: 5 * U,
      paint: (r, angle) => {
        const front = Math.min(angle, 2 * Math.PI - angle);
        if (r >= 2) return blend('light', 'foot', 0.35);
        if (r === 1 && angle > 2.4 && angle < 3.8) return tone('mark');
        return front < 1.2 ? blend('skin', 'light', 0.6) : tone('skin');
      },
    },
  );
  s.weights = [hand, hand, 0];
  segment(
    s,
    side,
    WRIST,
    PALM,
    [
      [0, 7.5, 6],
      [0.55, 10, 4.2],
      [1, 9.5, 3.2],
    ],
    { sides: 6, turn: Math.PI / 6, start: 4 * U, end: 'flat', paint: (r) => (r === 0 ? blend('light', 'foot', 0.6) : tone('foot')) },
  );
  const across = new THREE.Vector3(1, 0, 0);
  FINGERS.forEach(([angle, length, thick], i) => {
    const base = PALM.clone().addScaledVector(across, (i - 1.5) * 4.2 * U);
    digit(s, side, base, heading(angle, length, 2), length, thick);
  });
}
