import * as THREE from 'three';
import { ForestAnimal, type ForestAnimalOptions } from './ForestAnimal';
import { blend, bone, bump, coat, ease, eye, gloss, loft, rigidMesh, Sculpt, seededRandom, skinnedMesh, smooth, thin, tone, tubeRings, weighAlong, type Lofted, type Palette, type Ring, type Tone, type Weights } from './parts';

type V3 = THREE.Vector3;

// The forest lake's fish (ForestFish.md), as its reference sheet draws it: a
// trout of flat facets, olive green along its back, fading through a golden
// side to a cream belly and covered in small dark spots; orange fins (the
// dorsal, the little adipose fin behind it, two pectorals, two pelvics and
// the anal fin) and an orange forked tail, each a pleated fan of rays; a big
// dark eye with a glint, in a golden ring; and pale lips round a mouth that
// opens. Its colours are the sheet's five variations. Named ForestFish since
// the project already has a Fish.
//
// Meters; it faces +z and its left is +x. Its origin is level with its
// lowest point as it rests (the tips of its pelvic fins), under the middle
// of its length, which is where it turns and pitches. Its shape is traced
// from the sheet's side view in its pixels (U m each) and its widths from
// the front and top views, so it is 40 cm from its snout to the tips of its
// tail, as a grown trout in a lake is.
//
// It swims in water whose surface is y = 0 of its parent, its middle
// `depth` under it. Out of the water (its middle above that surface, as on
// a floor) it rests where it is put, on its fins, and its poses still play;
// a leap there is a flop.

// ---------------------------------------------------------------- measures

const LENGTH = 0.4; // m from the snout to the tips of the tail
const SNOUT = 830.7; // sheet px, side view: the tip of the snout
const TAIL_TIP = 1147.5; // the tip of the tail's upper lobe
const U = LENGTH / (TAIL_TIP - SNOUT); // m per pixel of the sheet's side view
const X_MID = (SNOUT + TAIL_TIP) / 2; // the middle of its length, over its origin
const Y_LOW = 293; // its lowest point (the pelvic fins' tips), level with its origin
const AXIS = 210; // the line along its middle that its rings and spine are centred on
const MIDDLE_Y = (Y_LOW - AXIS) * U; // m its middle stands above its origin
const SIDES = 14; // corners round its body and head
const ROUND = 2.2; // their rings' roundness (2 an ellipse)
const BUMPS = 0.03; // how far a ring's corners stand in or out, as the sheet's facets are uneven

// A point traced from the sheet's side view: `xs` along it (the snout at
// the left), `ys` down it, in its pixels; `x` out to the fish's left.
function P(xs: number, ys: number, x = 0): V3 {
  return new THREE.Vector3(x * U, (Y_LOW - ys) * U, (X_MID - xs) * U);
}

// A cross-section of the body or the head: at `xs`, from `top` to `bottom`
// in the side view, `hw` to either side (from the top and front views), and
// how square it is (ROUND unless given).
type Station = readonly [xs: number, top: number, bottom: number, hw: number, n?: number];

// From the tail's root (its flat end, where the tail fin grows) forward to
// the gills.
const BODY: readonly Station[] = [
  [1104, 187, 225, 10],
  [1096, 185.5, 224.5, 13.5],
  [1084, 184, 223.5, 17],
  [1068, 179, 230, 22.5],
  [1050, 173.5, 237.5, 27.5],
  [1030, 168, 244.5, 31.5],
  [1008, 161, 251.5, 34.5],
  [985, 156.5, 255.5, 36.5],
  [960, 155, 257, 36.8],
  [935, 156.5, 255.5, 35.5],
  [910, 161, 249, 32],
];

// From the gills to the snout; ahead of the mouth's corner (MOUTH) the
// bottom is the upper jaw's, the roof of the mouth, and the rings are
// squarer, so the pale upper lip faces out along the jaw's edge.
const HEAD: readonly Station[] = [
  [910, 161, 249, 32],
  [895, 164.5, 245, 31, 2.35],
  [880, 169, 240.5, 28.5, 2.5],
  [868, 173.5, 236.5, 26.5, 2.5],
  [861, 177, 228, 23.5, 2.4],
  [853, 181.5, 222.5, 19.5, 2.6],
  [845, 186.5, 218.5, 15, 2.6],
  [838, 193, 216, 10.5, 2.6],
  [833.5, 199, 214, 6, 2.4],
];
const MOUTH = 862; // the mouth's corner: the upper jaw ahead of it
const THROAT = 869; // the lower jaw's hinge is under here

// The lower jaw, from its hinge (inside the head) to the chin. Shut, its
// top is tucked up inside the upper jaw from its hinge to 848; ahead of
// that it stands a little under it, so the mouth shows a crack of red in
// front, as the sheet draws it.
const JAW: readonly Station[] = [
  [876, 229.5, 238, 20, 2.6],
  [868, 226, 237.5, 21.5, 2.6],
  [858, 222.5, 235, 19.5, 2.8],
  [848, 218.5, 232, 16.5, 2.8],
  [840, 220, 229, 12.5, 2.8],
  [834.5, 220, 226, 8.5, 2.6],
];
const HINGE: readonly [number, number] = [870, 229];
// The upper lip: a pale rounded bar along the upper jaw's edge, from the
// front of the snout down and back to the mouth's corner under the eye,
// [xs, ys, radius] along it.
const LIP: readonly (readonly [number, number, number])[] = [
  [835.5, 207.5, 2.1],
  [841, 211, 2.5],
  [848, 216, 2.7],
  [855, 220.5, 2.6],
  [860.5, 224.5, 2.2],
];

const EYE: readonly [number, number] = [864, 201.7]; // its middle
const EYE_R = 8.3; // px, the dark of the eye
const NOSTRIL: readonly [number, number] = [840.5, 199];
// The back edge of the gill cover, a dark crease.
const CREASE: readonly (readonly [number, number])[] = [
  [894, 181],
  [901, 193.5],
  [907.5, 206.5],
  [905.5, 221],
  [900.5, 234],
];

// The spine: the head's bone at the gills, the middle (its root), and on
// back to the tail's root, where the tail fin is.
const SPINE = [910, X_MID, 1030, 1060, 1084, 1103] as const;

// The fins' rays, from the root (a little inside the body) to the tip on
// the fin's edge: [xs, ys, x] each, x out to its left for a pair (mirrored for its right).
type Ray = readonly [xs: number, ys: number, x: number, toXs: number, toYs: number, toX: number];

const DORSAL: readonly Ray[] = [
  [953, 158, 0, 989, 113, 0],
  [962, 158.5, 0, 995, 113, 0],
  [971, 159, 0, 1000, 119.5, 0],
  [980, 159.5, 0, 1004.5, 126, 0],
  [990, 160.5, 0, 1013, 140.5, 0],
  [1002, 162.5, 0, 1020, 153, 0],
];
const ADIPOSE: readonly Ray[] = [
  [1049, 178, 0, 1056, 166.5, 0],
  [1056, 179.5, 0, 1064, 165.5, 0],
  [1063, 181, 0, 1071, 169, 0],
  [1069, 182.5, 0, 1073.5, 177, 0],
];
const ANAL: readonly Ray[] = [
  [1037, 239, 0, 1060, 266.5, 0],
  [1046, 236, 0, 1065.5, 260, 0],
  [1055, 233, 0, 1069.5, 251, 0],
  [1064, 229.5, 0, 1072.5, 240, 0],
];
// The tail fin: its rays fan from its root (inside the tail's flat end) to
// the two lobes and the fork between them.
const CAUDAL: readonly Ray[] = (
  [
    [1146.5, 153.5],
    [1148, 162],
    [1148, 170.5],
    [1142, 189],
    [1135, 207],
    [1140, 225],
    [1145, 243],
    [1144, 251],
    [1141, 259.5],
  ] as const
).map(([xs, ys], i, all) => {
  const f = i / (all.length - 1);
  return [1097 + 2.5 * f, 188 + 35 * f, 0, xs, ys, 0] as const;
});
// The paired fins splay out from the body toward their back edges, so they
// show broad from the front and from above as well as from the side, as
// the sheet's views draw them.
const PECTORAL: readonly Ray[] = fan(
  [905, 234, 22.5],
  [914, 229.5, 25],
  [
    [932, 261, 44],
    [937.5, 257.5, 46],
    [942, 251.5, 48],
    [945.5, 242.5, 49.5],
    [946.5, 231, 50],
  ],
);
const PELVIC: readonly Ray[] = fan(
  [966, 251.5, 9],
  [988, 250.5, 11],
  [
    [990, 293, 38],
    [995.5, 286.5, 37.5],
    [999, 279, 36],
    [1001, 271, 33],
    [1001, 263, 28],
  ],
);

// Rays from along a root line to each of a fin's tips.
function fan(from: readonly [number, number, number], to: readonly [number, number, number], tips: readonly (readonly [number, number, number])[]): Ray[] {
  return tips.map((tip, i) => {
    const f = i / (tips.length - 1);
    return [from[0] + (to[0] - from[0]) * f, from[1] + (to[1] - from[1]) * f, from[2] + (to[2] - from[2]) * f, tip[0], tip[1], tip[2]] as const;
  });
}

// ---------------------------------------------------------------- motion

const SWIM_SPEED = 0.5; // m/s, a trout's cruise: 1.25 lengths a second
const STRIDE = 0.75; // lengths it goes a tail beat
const IDLE_BEAT = 0.55; // tail beats a second as it hovers
const IDLE_WAVE = 0.22; // the size of its body's wave hovering, of swimming's
const SPEED_RATE = 2.2; // how fast it speeds up and slows down, a second
const POSE_RATE = 5; // how fast its posture follows what it does, a second
const BEND_RATE = 12; // how fast its body bends into a turn, a second
const DEPTH = 0.15; // m its middle swims under the surface: its dorsal fin 3 cm under
const CLIMB = 0.25; // m/s at most it rises or dives to its depth
const PITCH_LIMIT = 0.45; // rad its nose tips up or down, rising or diving
const STEER_BEND = 0.12; // its body bends this much a rad/s it is turned
const STEER_LIMIT = 0.3;
const SWAY = 0.045; // rad it sways from side to side, hovering
const BOB = 0.003; // m it rises and sinks, hovering (never below where it rests)
const SCULL = 1.15; // strokes of its pectoral fins a second, hovering
const TURN_TIME = 0.45; // s a turn takes, and
const TURN_PER_RAD = 0.16; // s more for each radian it turns
const TURN_BEND = 0.8; // rad its body bends into a C, turning right round
const MAX_TURN = 720; // degrees a turn goes round at most
const JAW_REST = 0.05; // rad its mouth hangs open at rest, a crack of red in front
const JAW_OPEN = 0.5; // rad opened wide
const BREATH = 0.035; // rad its jaw works as it breathes
const GAPE = 0.8; // s an opening and closing of its mouth
const GAPES = 3; // how many OpenMouth does
const G = 9.81; // m/s²
const LEAP = 0.3; // m a leap clears the water by: its lowest point at the top
const MAX_LEAP = 1;
const LEAP_ANGLE = THREE.MathUtils.degToRad(60); // how steeply its middle leaves the water
const RUN_UP = 0.32; // s at least from its depth to the surface
const PLUNGE = 0.36; // s at least from the surface back down to its depth
const FLOP = 0.045; // m it hops out of water
const FLOP_TIME = 0.55; // s a flop takes
const FLOP_BEND = 0.45; // rad it bends, flopping

// The size of each part of its wave (the head, the middle, the spine's
// bones back to the tail's root, and the tail fin), rad, and how far behind
// the head's it comes, rad: the wave runs back along it to the tail, and
// the head sways against the tail.
const WAVE: readonly (readonly [size: number, lag: number])[] = [
  [0.05, -0.5],
  [0.02, 0],
  [0.1, 0.9],
  [0.19, 1.6],
  [0.28, 2.2],
  [0.36, 2.7],
  [0.42, 3.2],
];
// How far each turns as its body bends into a C: the head into the turn,
// the tail round after it.
const BEND: readonly number[] = [0.5, 0, -0.3, -0.55, -0.8, -1, -1.1];

// ---------------------------------------------------------------- colours

// The sheet's five colour variations, picked from its pictures (the lit
// sides, a little brighter). The koi's orange patches are faces of the
// `...Patch` roles, the others' own colours in the other variations.
interface Colors {
  back: number; // along the back and the top of the head
  side: number; // the band along its side
  belly: number;
  cheek: number; // the head's sides, under and behind the eye
  lip: number; // the upper jaw's edge
  jaw: number; // the lower jaw
  mouth: number; // inside the mouth
  crease: number; // the gill cover's edge
  spot: number;
  fleck: number; // one spot in six (the koi's red flecks)
  fin: number;
  finRoot: number; // the fins toward their roots
  ring: number; // the skin round the eye
}

const EYES = { iris: 0x4b2b1d, pupil: 0x0d0b0b, lid: 0x14110f, shine: 0xffffff, nostril: 0x2e2724 };

function palette(c: Colors, patch?: number): Palette {
  return { ...EYES, ...c, backPatch: patch ?? c.back, sidePatch: patch ?? c.side, bellyPatch: patch ?? c.belly, cheekPatch: patch ?? c.cheek, spotPatch: patch ?? c.spot };
}

const VARIATIONS: Record<string, Palette> = {
  trout: palette({
    back: 0x6f7351,
    side: 0xe8b870,
    belly: 0xfff4e4,
    cheek: 0xe6c47c,
    lip: 0xf4d898,
    jaw: 0xf8f0ea,
    mouth: 0xb8453a,
    crease: 0x5a5032,
    spot: 0x3b332d,
    fleck: 0x3b332d,
    fin: 0xfd8d4a,
    finRoot: 0xf27d3e,
    ring: 0xf2d58e,
  }),
  salmon: palette({
    back: 0xb44a3e,
    side: 0xea7462,
    belly: 0xf6d2c6,
    cheek: 0xf28a6c,
    lip: 0xf49a80,
    jaw: 0xf6e0d8,
    mouth: 0xa8302a,
    crease: 0x9a3a32,
    spot: 0xa13a34,
    fleck: 0xa13a34,
    fin: 0xfb6a55,
    finRoot: 0xef5a48,
    ring: 0xf7a58a,
  }),
  carp: palette({
    back: 0xd88a26,
    side: 0xf3b843,
    belly: 0xf7dc98,
    cheek: 0xf7c455,
    lip: 0xf8cf78,
    jaw: 0xf5e2b8,
    mouth: 0xb7472f,
    crease: 0xc07a22,
    spot: 0xe5a53c,
    fleck: 0xe5a53c,
    fin: 0xf59c2e,
    finRoot: 0xea8c24,
    ring: 0xf8cc62,
  }),
  koi: palette(
    {
      back: 0xf6eee8,
      side: 0xf7efea,
      belly: 0xf4e8e0,
      cheek: 0xf7ece6,
      lip: 0xf5ddd2,
      jaw: 0xf3e6df,
      mouth: 0xc9504a,
      crease: 0xd8c4bc,
      spot: 0xf5e9e2,
      fleck: 0xf2603a,
      fin: 0xf8ebe5,
      finRoot: 0xf36a44,
      ring: 0xf6e2da,
    },
    0xf2552e,
  ),
  blue: palette({
    back: 0x3f67b4,
    side: 0x6aa0e6,
    belly: 0xe6edf6,
    cheek: 0x74aaf0,
    lip: 0xa7c6f2,
    jaw: 0xe9eef7,
    mouth: 0x7a3a4a,
    crease: 0x345a9e,
    spot: 0x3b66b8,
    fleck: 0x3b66b8,
    fin: 0xadc3f0,
    finRoot: 0x8aa8e8,
    ring: 0x9cc0f4,
  }),
};

// Where the koi's orange patches lie, as its picture on the sheet shows
// them (its left side; the right is its own): for each length of its head
// and body between two rings (by where its middle is along the side view),
// which of its faces round from the ridge (0) to the keel (6) are orange,
// '1', on its left side and on its right. White between them: a big patch
// over its nape and the front of its back, one along its side behind the
// dorsal fin, one before its tail, and a little one low behind its gills.
const PATCHES: readonly (readonly [xs: number, left: string, right: string])[] = [
  [902.5, '0111', '0011'],
  [922.5, '111101', '1111'],
  [947.5, '1111', '11111'],
  [972.5, '111', '1111'],
  [996.5, '1', '11'],
  [1019, '0111', '00111'],
  [1040, '0111', '0011'],
  [1059, '', '011'],
  [1076, '0111', '0'],
  [1090, '0011', '011'],
];
const FACE = (2 * Math.PI) / SIDES; // rad round a ring each face spans

function patched(xs: number, down: number, side: number): boolean {
  const row = PATCHES.find(([at]) => Math.abs(at - xs) < 1);
  if (!row) return false;
  const k = Math.min(SIDES / 2 - 1, Math.floor(down / FACE));
  return (side > 0 ? row[1] : row[2])[k] === '1';
}

// ---------------------------------------------------------------- shapes

// -1..1, the same for the same numbers: how far a ring's corner stands out.
function hash(a: number, b: number): number {
  const h = Math.sin(a * 12.9898 + b * 78.233) * 43758.5453;
  return (h - Math.floor(h)) * 2 - 1;
}

function ringsOf(stations: readonly Station[]): Ring[] {
  return stations.map(([xs, top, bottom, hw, n]) => ({ at: P(xs, AXIS), w: hw * U, up: (AXIS - top) * U, down: (bottom - AXIS) * U, n: n ?? ROUND }));
}

// A ring's corners a little in or out, the same for a ring shared by two
// lofts (it is keyed by where the ring is); the ridge along the back and the
// keel along the belly hardly move.
function bumps(stations: readonly Station[]) {
  return (r: number, _angle: number, k: number) => 1 + (k === 0 || k === SIDES / 2 ? 0.3 : 1) * BUMPS * hash(stations[Math.max(0, Math.min(stations.length - 1, r))][0], k);
}

// The middle of a face between ring r and the next, along the side view.
function between(stations: readonly Station[], r: number): number {
  const a = stations[Math.max(0, Math.min(stations.length - 1, r))][0];
  const b = stations[Math.max(0, Math.min(stations.length - 1, r + 1))][0];
  return (a + b) / 2;
}

// How far round from its back a face is (0 on the ridge, π on the keel),
// and on which side (1 its left, +x).
function around(angle: number): { down: number; side: 1 | -1 } {
  return angle <= Math.PI ? { down: angle, side: 1 } : { down: 2 * Math.PI - angle, side: -1 };
}

// Its body's bands, as the sheet shades them: olive along the back, golden
// along the side, cream underneath, blended where they meet.
function bodyTone(xs: number, angle: number): Tone {
  const { down, side } = around(angle);
  const p = patched(xs, down, side) ? 'Patch' : '';
  if (down < 0.9) return tone(`back${p}`);
  if (down < 1.35) return blend(`back${p}`, `side${p}`, 0.4);
  if (down < 1.8) return tone(`side${p}`);
  if (down < 2.25) return blend(`side${p}`, `belly${p}`, 0.6);
  return tone(`belly${p}`);
}

// The head's: its olive crown, golden cheeks and pale throat; ahead of the
// mouth's corner, the pale upper lip along the jaw's edge and the red roof
// of the mouth under it.
function headTone(xs: number, angle: number): Tone {
  const { down, side } = around(angle);
  const p = patched(xs, down, side) ? 'Patch' : '';
  if (down < 0.9) return tone(`back${p}`);
  // Level with the eye still olive, as the sheet paints it, so the golden
  // skin round the eye shows against it.
  if (down < 1.35) return blend(`back${p}`, `cheek${p}`, xs < MOUTH ? 0.45 : 0.3);
  if (xs < MOUTH) {
    if (down < 2.7) return tone('lip');
    return tone('mouth');
  }
  if (down < 2.25) return tone(`cheek${p}`);
  if (down < 2.7) return blend(`cheek${p}`, 'belly', 0.5);
  // Behind the mouth's corner the throat rises into the mouth: red where
  // it shows inside the open mouth.
  return xs < THROAT ? tone('mouth', 0.8) : tone('belly');
}

// Where a ray from `from` going `way` meets a sculpted surface; null when it
// meets none.
function hit(s: Sculpt, from: V3, way: V3): V3 | null {
  const w = way.clone().normalize();
  const t = s.cast(from, w);
  return Number.isFinite(t) ? from.clone().addScaledVector(w, t) : null;
}

// A point laid on the surface: cast onto it along -normal from just outside,
// and lifted `lift` off it.
function seat(s: Sculpt, p: V3, normal: V3, lift: number): V3 {
  const out = p.clone().addScaledVector(normal, 6 * U);
  return (hit(s, out, normal.clone().negate()) ?? p.clone()).addScaledVector(normal, lift);
}

// A flat piece laid on a sculpted surface (a spot, a nostril), `radius` m
// round `centre`, `corners` round: its corners cast onto the surface along
// -normal, so it follows the facets, lifted a little off them.
function decal(s: Sculpt, centre: V3, normal: V3, radius: number, corners: number, spin: number, t: Tone, stretch = 1): void {
  const n = normal.clone().normalize();
  const a = new THREE.Vector3(0, 0, -1).addScaledVector(n, n.z).normalize(); // along the fish, toward its tail
  const b = new THREE.Vector3().crossVectors(n, a);
  const lift = 0.45 * U;
  const c = seat(s, centre, n, lift);
  const ring = Array.from({ length: corners }, (_, k) => {
    const angle = spin + (2 * Math.PI * k) / corners;
    const p = centre.clone().addScaledVector(a, Math.cos(angle) * radius * stretch).addScaledVector(b, Math.sin(angle) * radius);
    return seat(s, p, n, lift);
  });
  for (let k = 0; k < corners; k++) s.toward(c, ring[k], ring[(k + 1) % corners], n, t);
}

// The surface's outward normal near a point of the side view on one side
// of it, from three rays cast onto it across the fish.
function normalAt(s: Sculpt, xs: number, ys: number, side: 1 | -1): { at: V3; normal: V3 } | null {
  const way = new THREE.Vector3(-side, 0, 0);
  const c = hit(s, P(xs, ys, side * 80), way);
  const a = hit(s, P(xs + 1.5, ys, side * 80), way);
  const b = hit(s, P(xs, ys + 1.5, side * 80), way);
  if (!c || !a || !b) return null;
  const normal = new THREE.Vector3().crossVectors(a.sub(c), b.sub(c)).normalize();
  if (normal.x * side < 0) normal.negate();
  return { at: c, normal };
}

// The top and bottom of the body at a point along the side view.
function profile(xs: number): { top: number; bottom: number } {
  for (let i = 0; i + 1 < BODY.length; i++) {
    const [x0, t0, b0] = BODY[i];
    const [x1, t1, b1] = BODY[i + 1];
    if (xs <= x0 && xs >= x1) {
      const f = (x0 - xs) / (x0 - x1);
      return { top: t0 + (t1 - t0) * f, bottom: b0 + (b1 - b0) * f };
    }
  }
  const [, top, bottom] = BODY[BODY.length - 1];
  return { top, bottom };
}

// The torso, from the tail's flat end to the gills, and its spots.
function sculptBody(s: Sculpt, random: () => number): void {
  loft(s, ringsOf(BODY), {
    sides: SIDES,
    start: 'flat',
    end: 'open',
    shape: bumps(BODY),
    paint: (r, angle) => bodyTone(between(BODY, r), angle),
  });
  for (const side of [1, -1] as const) spots(s, random, side);
}

// The sheet's spots: small dark facets of six or seven sides scattered over
// its back and sides (not its belly), from behind its gills to its tail,
// each side its own; one in six is a fleck (the koi's red ones).
const SPOTS = 32; // a side
const SPOT_GAP = 14.5; // px at least between two
function spots(s: Sculpt, random: () => number, side: 1 | -1): void {
  const placed: [number, number][] = [];
  for (let tries = 0; placed.length < SPOTS && tries < 5000; tries++) {
    const xs = 915 + random() * (1094 - 915);
    const { top, bottom } = profile(xs);
    const ys = top + (0.1 + random() * 0.5) * (bottom - top);
    if (placed.some(([a, b]) => Math.hypot(a - xs, b - ys) < SPOT_GAP)) continue;
    placed.push([xs, ys]);
  }
  placed.forEach(([xs, ys], i) => {
    const seated = normalAt(s, xs, ys, side);
    if (!seated) return;
    // The face of the body it lies on (its length between two rings, and
    // how far round from the ridge, on its rings' rounded shape), to tell
    // whether that face is one of the koi's patches.
    const r = Math.max(0, BODY.findIndex(([a], j) => j + 1 < BODY.length && xs <= a && xs >= BODY[j + 1][0]));
    const { top, bottom } = profile(xs);
    const f = THREE.MathUtils.clamp((AXIS - ys) / (ys < AXIS ? AXIS - top : bottom - AXIS), -1, 1);
    const round = Math.acos(Math.sign(f) * Math.abs(f) ** (ROUND / 2));
    const down = (Math.floor(round / FACE) + 0.5) * FACE;
    const role = patched(between(BODY, r), down, side) ? 'spotPatch' : i % 6 === 5 ? 'fleck' : 'spot';
    decal(s, seated.at, seated.normal, (2.2 + random() * 2) * U, random() < 0.5 ? 6 : 7, random() * Math.PI, tone(role, 0.95 + 0.1 * random()));
  });
}

// The head, from the gills to the snout, rigid on its bone: the eyes (in
// `shiny`), the golden skin round them, the nostrils and the gill covers'
// creases.
function sculptHead(s: Sculpt, shiny: Sculpt): Lofted {
  const head = loft(s, ringsOf(HEAD), {
    sides: SIDES,
    start: 'open',
    end: 1.8 * U,
    shape: bumps(HEAD),
    paint: (r, angle) => headTone(between(HEAD, r), angle),
  });
  const up = new THREE.Vector3(0, 1, 0);
  for (const side of [1, -1] as const) {
    // The eye: big and dark, looking out and a little forward, its glint
    // up toward its tail as the sheet draws it, in a ring of golden skin.
    // It bulges from the head, as the sheet's top and front views show.
    const look = new THREE.Vector3(side * 0.9, 0.1, 0.43).normalize();
    const surface = hit(s, P(EYE[0], EYE[1], side * 60), new THREE.Vector3(-side, 0, 0)) ?? P(EYE[0], EYE[1], side * 24);
    skinRing(s, surface, look, EYE_R * 0.95 * U, EYE_R * 1.42 * U, tone('ring'));
    const centre = surface.clone().addScaledVector(look, 0.12 * EYE_R * U);
    const bulge = 0.8;
    eye(shiny, centre, look, up, EYE_R * U, { lid: tone('lid'), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 10, 0.55, bulge);
    // The sheet's glint is bigger, up and toward the tail on both eyes.
    const toTail = new THREE.Vector3(0, 0, -1).addScaledVector(look, look.z).normalize();
    const lift = new THREE.Vector3().crossVectors(look, toTail).multiplyScalar(side);
    const glint = centre.clone().addScaledVector(look, EYE_R * U * bulge * 0.93).addScaledVector(lift, 0.3 * EYE_R * U).addScaledVector(toTail, 0.26 * EYE_R * U);
    const corners = Array.from({ length: 6 }, (_, k) => glint.clone().addScaledVector(toTail, Math.cos((k * Math.PI) / 3) * 0.17 * EYE_R * U).addScaledVector(lift, Math.sin((k * Math.PI) / 3) * 0.15 * EYE_R * U));
    for (let k = 0; k < 6; k++) shiny.toward(glint, corners[k], corners[(k + 1) % 6], look, tone('shine'));
    const nose = normalAt(s, NOSTRIL[0], NOSTRIL[1], side);
    if (nose) decal(s, nose.at, nose.normal, 1.3 * U, 6, 0, tone('nostril'));
    band(s, side, CREASE, 1.1, 0.8, tone('crease'));
  }
  lip(s);
  return head;
}

// The upper lip: one rounded bar laid along the jaw's edge, half sunk in it,
// from the mouth's corner on one side, across the front of the snout, to the
// corner on the other, its ends rounded; from the front it is the arch over
// the mouth that the sheet's front view draws.
function lip(s: Sculpt): void {
  const half = (side: 1 | -1) =>
    LIP.flatMap(([xs, ys, r]) => {
      const seated = normalAt(s, xs, ys, side);
      return seated ? [{ at: seated.at.addScaledVector(seated.normal, 0.45 * r * U), r: r * U }] : [];
    });
  const points = [...half(-1).reverse(), { at: P(SNOUT + 1.8, 208.5, 0), r: 2 * U }, ...half(1)];
  loft(
    s,
    tubeRings(
      points.map((p) => p.at),
      points.map((p) => p.r),
      new THREE.Vector3(0, 1, 0),
    ),
    {
      sides: 6,
      start: 2 * U,
      end: 2 * U,
      // Its faces toward the head a little darker.
      paint: (_r, angle) => tone('lip', Math.abs(angle - Math.PI) < 1.2 ? 0.94 : 1.04),
    },
  );
}

// Golden skin round the eye: a ring of faces from `inner` to `outer`,
// laid on the head, wider behind the eye than in front.
function skinRing(s: Sculpt, centre: V3, look: V3, inner: number, outer: number, t: Tone): void {
  const z = look.clone().normalize();
  const x = new THREE.Vector3(0, 1, 0).cross(z).normalize();
  const y = new THREE.Vector3().crossVectors(z, x);
  const corners = 12;
  const lift = 0.35 * U;
  const at = (radius: number, k: number, wider: boolean) => {
    const angle = (2 * Math.PI * k) / corners;
    // Behind the eye (toward its tail) and above
    // it the skin reaches further, as the sheet draws it.
    const back = new THREE.Vector3().addScaledVector(x, Math.cos(angle)).addScaledVector(y, Math.sin(angle));
    const reach = wider ? radius * (1 + 0.18 * Math.max(0, -back.z) + 0.08 * Math.max(0, back.y)) : radius;
    return seat(s, centre.clone().addScaledVector(back, reach), z, lift);
  };
  const a = Array.from({ length: corners }, (_, k) => at(inner, k, false));
  const b = Array.from({ length: corners }, (_, k) => at(outer, k + 0.5, true));
  for (let k = 0; k < corners; k++) {
    const k1 = (k + 1) % corners;
    s.toward(a[k], b[k], a[k1], z, t);
    s.toward(a[k1], b[k], b[k1], z, t);
  }
}

// A band laid on the head along a line of the side view, `width` px either
// side of it and `lift` px off the surface: the gill cover's dark crease.
function band(s: Sculpt, side: 1 | -1, line: readonly (readonly [number, number])[], width: number, lift: number, t: Tone): void {
  const strip: (readonly [V3, V3, V3])[] = [];
  line.forEach(([xs, ys], i) => {
    const [px, py] = line[Math.max(0, i - 1)];
    const [nx, ny] = line[Math.min(line.length - 1, i + 1)];
    const l = Math.hypot(nx - px, ny - py);
    const seated = normalAt(s, xs, ys, side);
    if (!seated) return;
    // Across the line in the side view, laid in the surface there.
    const across = new THREE.Vector3(0, ((px - nx) / l) * width * U, ((ny - py) / l) * width * U);
    across.addScaledVector(seated.normal, -across.dot(seated.normal));
    strip.push([seat(s, seated.at.clone().add(across), seated.normal, lift * U), seat(s, seated.at.clone().sub(across), seated.normal, lift * U), seated.normal]);
  });
  for (let i = 0; i + 1 < strip.length; i++) {
    const [a, b, n] = strip[i];
    const [c, d] = strip[i + 1];
    s.toward(a, b, c, n, t);
    s.toward(b, d, c, n, t);
  }
}

// How light the top of the lower jaw is, from its hinge to its chin: dark
// down the throat, red toward the lips, so the open mouth looks deep.
const JAW_TOP: readonly number[] = [0.35, 0.45, 0.6, 0.82, 1, 1];

// The lower jaw: pale outside, red on top, inside the mouth.
function sculptJaw(s: Sculpt): Lofted {
  const rings = JAW.map(([xs, top, bottom, hw, n]) => {
    const c = (top + bottom) / 2;
    return { at: P(xs, c), w: hw * U, up: (c - top) * U, down: (bottom - c) * U, n: n ?? ROUND };
  });
  return loft(s, rings, {
    sides: 8,
    turn: Math.PI / 8,
    start: 'flat',
    end: 2 * U,
    paint: (r, angle) => {
      const { down } = around(angle);
      if (r < 0) return tone('mouth', 0.3); // its back end, inside the head
      return down < 0.5 ? tone('mouth', JAW_TOP[r]) : tone('jaw');
    },
  });
}

// The inside of the mouth: its sides, from the upper lip's edge down to the
// lower jaw's, and its back at the mouth's corner. Their top corners are on
// the head's bone and their bottom ones on the jaw's, so the lining
// stretches as the mouth opens and the open mouth is red inside rather than
// see-through.
function mouthLining(s: Sculpt, head: Lofted, jaw: Lofted, headBone: number, jawBone: number): void {
  const onHead: Weights = [headBone, headBone, 0];
  const onJaw: Weights = [jawBone, jawBone, 0];
  const front = HEAD.map(([xs], r) => ({ xs, r })).filter(({ xs }) => xs < MOUTH);
  // The jaw's upper edge (corner k of its rings) where the head's ring is.
  const jawAt = (xs: number, k: number): V3 => {
    for (let j = 0; j + 1 < JAW.length; j++) {
      const a = JAW[j][0];
      const b = JAW[j + 1][0];
      if (xs <= a && xs >= b) return jaw.rings[j][k].clone().lerp(jaw.rings[j + 1][k], (a - xs) / (a - b));
    }
    return jaw.rings[xs > JAW[0][0] ? 0 : JAW.length - 1][k].clone();
  };
  const sides = ([1, -1] as const).map((side) => ({
    side,
    top: front.map(({ r }) => head.rings[r][side > 0 ? SIDES / 2 - 1 : SIDES / 2 + 1].clone()),
    bottom: front.map(({ xs }) => jawAt(xs, side > 0 ? 0 : 7)),
  }));
  for (const { side, top, bottom } of sides) {
    const out = new THREE.Vector3(side, 0, 0);
    for (let i = 0; i + 1 < top.length; i++) {
      s.toward(top[i], top[i + 1], bottom[i + 1], out, tone('mouth', 0.45), [onHead, onHead, onJaw]);
      s.toward(top[i], bottom[i + 1], bottom[i], out, tone('mouth', 0.45), [onHead, onJaw, onJaw]);
    }
  }
  const [left, right] = sides;
  const forward = new THREE.Vector3(0, 0, 1);
  s.toward(left.top[0], right.top[0], right.bottom[0], forward, tone('mouth', 0.25), [onHead, onHead, onJaw]);
  s.toward(left.top[0], right.bottom[0], left.bottom[0], forward, tone('mouth', 0.25), [onHead, onJaw, onJaw]);
}

// How far through each fin's rays its root part runs.
const ROOT_SHARE = 0.36;

// A fin as the sheet draws its fins: a fan of rays from its root to its
// edge, the web between each two rays a panel of two faces, deeper coloured
// toward the root. Every second ray stands a little to one side and the
// others to the other (`pleat` m between them), so the panels fold like a
// fan's and each catches the light its own way; and it is `thick` m thick
// either side at its root, thinning to nothing at its edge, so that it
// shows edge on too.
function fin(s: Sculpt, rays: readonly Ray[], side: 1 | -1, facing: V3 | null, pleat: number, thick: number): void {
  const roots = rays.map(([xs, ys, x]) => P(xs, ys, x * side));
  const edges = rays.map(([, , , xs, ys, x]) => P(xs, ys, x * side));
  const f = facing?.clone().normalize() ?? new THREE.Vector3().crossVectors(edges[0].clone().sub(roots[0]), edges[edges.length - 1].clone().sub(roots[0])).normalize();
  if (!facing && f.x * side < 0) f.negate();
  const n = rays.length;
  // How far each ray stands to one side; the leading one stays straight,
  // so a fin's tip there (the pelvics' lowest point) is where it is traced.
  const fold = (i: number) => (i === 0 ? 0 : i % 2 === 1 ? 0.5 : -0.5) * pleat;
  const tips = edges.map((e, i) => e.clone().addScaledVector(f, fold(i)));
  const faces = ([1, -1] as const).map((face) => ({
    face,
    root: roots.map((r) => r.clone().addScaledVector(f, face * thick)),
    mid: roots.map((r, i) => r.clone().lerp(edges[i], ROOT_SHARE).addScaledVector(f, fold(i) * ROOT_SHARE + face * thick * (1 - ROOT_SHARE))),
  }));
  for (const { face, root, mid } of faces) {
    const out = f.clone().multiplyScalar(face);
    for (let i = 0; i + 1 < n; i++) {
      const shade = i % 2 === 0 ? 1.04 : 0.9;
      s.toward(root[i], root[i + 1], mid[i + 1], out, tone('finRoot', shade));
      s.toward(root[i], mid[i + 1], mid[i], out, tone('finRoot', shade));
      s.toward(mid[i], mid[i + 1], tips[i + 1], out, tone('fin', shade));
      s.toward(mid[i], tips[i + 1], tips[i], out, tone('fin', shade));
    }
  }
  // Closed along its first and last rays, between its two sides; the
  // leading edge catches the light.
  const [a, b] = faces;
  for (const i of [0, n - 1]) {
    const away = edges[i].clone().sub(edges[i === 0 ? 1 : n - 2]);
    s.toward(a.root[i], b.root[i], b.mid[i], away, tone('finRoot', 1.08));
    s.toward(a.root[i], b.mid[i], a.mid[i], away, tone('finRoot', 1.08));
    s.toward(a.mid[i], b.mid[i], tips[i], away, tone('fin', 1.08));
  }
}

// The middle of a fin's root line.
function rootOf(rays: readonly Ray[], side: 1 | -1): V3 {
  const a = rays[0];
  const b = rays[rays.length - 1];
  return P((a[0] + b[0]) / 2, (a[1] + b[1]) / 2, ((a[2] + b[2]) / 2) * side);
}

function moved(geometry: THREE.BufferGeometry, from: V3): THREE.BufferGeometry {
  return geometry.applyMatrix4(new THREE.Matrix4().makeTranslation(-from.x, -from.y, -from.z));
}

// ---------------------------------------------------------------- leaps

// A leap, planned when it starts: its middle rises from its depth to the
// surface (run-up), flies (under gravity alone) and plunges back to its
// depth, along its heading, which it holds.
interface Leap {
  time: number;
  heading: number;
  x: number; // where its middle started, in its parent
  z: number;
  from: number; // how deep its middle started
  to: number; // and how deep it ends
  speed: number; // forward, as it started
  climb: number; // up, as it started
  vh: number; // forward and up as its middle breaks the surface
  vy: number;
  run: number; // s: the run-up, the flight and the plunge
  flight: number;
  plunge: number;
  out: number; // m forward where it leaves the water
  in: number; // and where it comes back in
  end: number; // and where it ends
  finish: number; // m/s forward at the end
  splashes: number; // sent so far
}

// A cubic from p0 (slope m0) to p1 (slope m1) over `span` s: its value and
// slope at t.
function hermite(t: number, span: number, p0: number, m0: number, p1: number, m1: number): [number, number] {
  const u = THREE.MathUtils.clamp(t / span, 0, 1);
  const u2 = u * u;
  const u3 = u2 * u;
  const value = (2 * u3 - 3 * u2 + 1) * p0 + (u3 - 2 * u2 + u) * span * m0 + (-2 * u3 + 3 * u2) * p1 + (u3 - u2) * span * m1;
  const slope = ((6 * u2 - 6 * u) * p0 + (3 * u2 - 4 * u + 1) * span * m0 + (-6 * u2 + 6 * u) * p1 + (3 * u2 - 2 * u) * span * m1) / span;
  return [value, slope];
}

function wrap(angle: number): number {
  return Math.atan2(Math.sin(angle), Math.cos(angle));
}

// `splash`: in a leap its middle breaks the water's surface, leaving it or
// coming back in, at (x, z) in its parent's coordinates, moving at
// `velocity` m/s (up as it leaves, down as it comes back).
export interface ForestFishEventMap extends THREE.Object3DEventMap {
  splash: { x: number; z: number; velocity: THREE.Vector3 };
}

// Its events are a Group's and `splash`.
export interface ForestFish {
  addEventListener<T extends Extract<keyof ForestFishEventMap, string>>(type: T, listener: THREE.EventListener<ForestFishEventMap[T], T, this>): void;
  hasEventListener<T extends Extract<keyof ForestFishEventMap, string>>(type: T, listener: THREE.EventListener<ForestFishEventMap[T], T, this>): boolean;
  removeEventListener<T extends Extract<keyof ForestFishEventMap, string>>(type: T, listener: THREE.EventListener<ForestFishEventMap[T], T, this>): void;
  dispatchEvent<T extends Extract<keyof ForestFishEventMap, string>>(event: THREE.BaseEvent<T> & ForestFishEventMap[T]): void;
}

type Mode = 'idle' | 'swim' | 'turn' | 'mouth' | 'jump' | 'flop';

// ---------------------------------------------------------------- the fish

export class ForestFish extends ForestAnimal {
  // The sheet's colour variations, the default (trout) first.
  static readonly COLORS = Object.keys(VARIATIONS);
  static readonly LENGTH = LENGTH;
  static readonly SWIM_SPEED = SWIM_SPEED;
  static readonly DEPTH = DEPTH;
  static readonly LEAP = LEAP; // m a leap clears the water by, unless told otherwise
  static readonly MIDDLE_Y = MIDDLE_Y; // m its middle stands above its origin

  readonly head = new THREE.Group(); // its skin, eyes, nostrils and gill covers
  readonly jaw = new THREE.Group(); // the lower jaw, turning at its hinge
  readonly body: THREE.SkinnedMesh; // from the gills to the tail's root, with its spots
  readonly tail = new THREE.Group(); // the forked tail fin, on the spine's last bone
  readonly dorsalFin: THREE.SkinnedMesh;
  readonly adiposeFin: THREE.SkinnedMesh;
  readonly analFin: THREE.SkinnedMesh;
  readonly leftPectoralFin = new THREE.Group();
  readonly rightPectoralFin = new THREE.Group();
  readonly leftPelvicFin = new THREE.Group();
  readonly rightPelvicFin = new THREE.Group();

  // m its middle swims under the water's surface (y = 0 of its parent).
  depth = DEPTH;

  private readonly spine: THREE.Bone[];
  private readonly jawBone: THREE.Bone;
  private mode: Mode = 'idle';
  private modeTime = 0;
  private time = 0;
  private heading = 0; // its rotation.y as the last frame left it
  private phase = 0; // of its tail beat, rad
  private wave = IDLE_WAVE; // the size of its body's wave, eased
  private bend = 0; // rad it bends into a C (> 0 turning counter-clockwise), eased
  private pitch = 0; // rad its nose is up, eased
  private fold = 0; // 0..1 its fins folded back, eased
  private flare = 0; // rad its fins spread, turning or gaping, eased
  private jawAngle = JAW_REST;
  private turnBy = 0;
  private turnTime = 1;
  private turned = 0;
  private leap: Leap | null = null;
  private flopFrom = 0;
  // A behaviour asked for in the air, done once it is back in the water.
  private later: (() => void) | null = null;

  constructor(options: ForestAnimalOptions = {}) {
    super(VARIATIONS, options);
    const random = seededRandom(options.seed ?? 11);
    const palette = this.palette;

    // The bones, as it rests: the middle is the root, the head's bone ahead
    // of it at the gills, and a chain back to the tail's root.
    const joints = SPINE.map((xs) => P(xs, AXIS));
    const middle = bone('middle', this.rig, joints[1]);
    const head = bone('head', middle, joints[0], joints[1]);
    const chain: THREE.Bone[] = [head, middle];
    for (let i = 2; i < joints.length; i++) chain.push(bone(`spine${i}`, chain[i - 1], joints[i], joints[i - 1]));
    this.spine = chain;
    // The lower jaw's hinge, on the head.
    const hinge = P(HINGE[0], HINGE[1]);
    this.jawBone = bone('jaw', head, hinge, joints[0]);
    this.updateMatrixWorld(true);
    const skeleton = new THREE.Skeleton([...chain, this.jawBone]);
    const along = (geometry: THREE.BufferGeometry) => {
      weighAlong(geometry, joints, joints.map((_, i) => i), 'linear');
      return geometry;
    };

    const skin = coat();
    skin.roughness = 0.62;
    const webbing = thin(0.72);

    // The body bends along the spine.
    const bodySculpt = new Sculpt(random);
    sculptBody(bodySculpt, random);
    this.body = this.painting(skinnedMesh(along(bodySculpt.geometry(palette)), skin, skeleton));

    // The head, rigid on its bone.
    const headSculpt = new Sculpt(random);
    const eyes = new Sculpt(random, 0.01);
    const headLoft = sculptHead(headSculpt, eyes);
    this.head.add(this.painting(rigidMesh(moved(headSculpt.geometry(palette), joints[0]), skin)));
    this.head.add(this.painting(rigidMesh(moved(eyes.geometry(palette), joints[0]), gloss())));
    head.add(this.head);

    // The lower jaw, turning at its hinge under the head, and the lining of
    // the mouth, stretched between the two.
    const jawSculpt = new Sculpt(random);
    const jawLoft = sculptJaw(jawSculpt);
    this.jaw.add(this.painting(rigidMesh(moved(jawSculpt.geometry(palette), hinge), skin)));
    const liningSculpt = new Sculpt(random);
    mouthLining(liningSculpt, headLoft, jawLoft, 0, chain.length);
    const inside = coat();
    inside.side = THREE.DoubleSide;
    this.jaw.add(this.painting(skinnedMesh(liningSculpt.geometry(palette, true), inside, skeleton)));
    this.jawBone.add(this.jaw);

    // The fins along its back and belly bend with it.
    const median = (rays: readonly Ray[], pleat: number, thick: number) => {
      const s = new Sculpt(random);
      fin(s, rays, 1, new THREE.Vector3(1, 0, 0), pleat * U, thick * U);
      return this.painting(skinnedMesh(along(s.geometry(palette)), webbing, skeleton));
    };
    this.dorsalFin = median(DORSAL, 2.4, 2);
    this.adiposeFin = median(ADIPOSE, 1.2, 2.2);
    this.analFin = median(ANAL, 1.8, 1.6);

    // The tail fin, on the spine's last bone.
    const tailSculpt = new Sculpt(random);
    fin(tailSculpt, CAUDAL, 1, new THREE.Vector3(1, 0, 0), 2.6 * U, 2.2 * U);
    this.tail.add(this.painting(rigidMesh(moved(tailSculpt.geometry(palette), joints[joints.length - 1]), webbing)));
    chain[chain.length - 1].add(this.tail);

    // The paired fins, each turning about the middle of its root: the
    // pectorals behind the gills on the head's bone, the pelvics under the
    // middle on its bone.
    const pair = (rays: readonly Ray[], side: 1 | -1, pivot: THREE.Group, on: THREE.Bone, at: V3, pleat: number, thick: number) => {
      const root = rootOf(rays, side);
      const s = new Sculpt(random);
      fin(s, rays, side, null, pleat * U, thick * U);
      pivot.position.copy(root).sub(at);
      pivot.add(this.painting(rigidMesh(moved(s.geometry(palette), root), webbing)));
      on.add(pivot);
    };
    pair(PECTORAL, 1, this.leftPectoralFin, head, joints[0], 1.6, 1.2);
    pair(PECTORAL, -1, this.rightPectoralFin, head, joints[0], 1.6, 1.2);
    pair(PELVIC, 1, this.leftPelvicFin, middle, joints[1], 1.4, 1);
    pair(PELVIC, -1, this.rightPelvicFin, middle, joints[1], 1.4, 1);

    this.rig.add(this.body, this.dorsalFin, this.adiposeFin, this.analFin);
    this.pose(0);
  }

  // Where its origin is when its middle is at its depth under the surface.
  get swimY(): number {
    return -(this.depth + MIDDLE_Y);
  }

  // What it is doing: its behaviour, from the sheet's poses.
  get doing(): string {
    return this.mode === 'flop' ? 'jump' : this.mode;
  }

  // In a leap (or a flop), from its run-up until it is back at its depth.
  get jumping(): boolean {
    return this.mode === 'jump' || this.mode === 'flop';
  }

  // Hovers in place: it slows to a stop, its fins sculling, its body
  // swaying slowly. Named as in its sheet's poses (ForestFish.md).
  Idle(): void {
    this.begin('idle', () => this.Idle());
  }

  // Swims forward the way it faces (its +z) at its speed, its body bending
  // in a wave, until told otherwise; in water it keeps to its depth. Named
  // as in its sheet's poses.
  Swim(): void {
    this.begin('swim', () => this.Swim());
  }

  // A sharp turn in place: its body bent into a C, it turns `degrees` round
  // (180 by default, to face the other way; > 0 counter-clockwise seen from
  // above; at most twice round), then hovers. Named as in its sheet's poses
  // (ForestFish.md).
  Turn(degrees = 180): void {
    const by = THREE.MathUtils.degToRad(THREE.MathUtils.clamp(degrees, -MAX_TURN, MAX_TURN));
    if (!Number.isFinite(by) || by === 0) return;
    if (this.jumping) {
      this.later = () => this.Turn(degrees);
      return;
    }
    this.turnBy = by;
    this.turnTime = TURN_TIME + TURN_PER_RAD * Math.abs(by);
    this.turned = 0;
    this.mode = 'turn';
    this.modeTime = 0;
  }

  // Opens and closes its mouth three times, then hovers. Named as in its
  // sheet's poses.
  OpenMouth(): void {
    if (this.jumping) {
      this.later = () => this.OpenMouth();
      return;
    }
    this.mode = 'mouth';
    this.modeTime = 0;
  }

  // Leaps out of the water in an arc, clearing it by `height` m (its lowest
  // point at the top of the arc), and dives back in along the way it faces,
  // then hovers at its depth; it splashes where it breaks the surface, going
  // out and coming back ('splash' events). Out of water it flops: a hop
  // where it lies. A second leap is ignored until it is back at its depth.
  // Named as in its sheet's poses ("Jump (Out of Water)"; ForestFish.md).
  Jump(height = LEAP): void {
    if (this.jumping || !Number.isFinite(height)) return;
    this.later = null;
    if (this.position.y + MIDDLE_Y < -0.01) this.plan(THREE.MathUtils.clamp(height, 0.05, MAX_LEAP));
    else {
      this.flopFrom = this.position.y;
      this.mode = 'flop';
      this.modeTime = 0;
    }
  }

  private begin(mode: 'idle' | 'swim', again: () => void): void {
    if (this.jumping) {
      this.later = again;
      return;
    }
    if (mode !== this.mode) this.modeTime = 0;
    this.mode = mode;
  }

  // Back to hovering after a pose that ends by itself, or on to what was
  // asked for meanwhile.
  private done(): void {
    this.mode = 'idle';
    this.modeTime = 0;
    const later = this.later;
    this.later = null;
    later?.();
  }

  private plan(height: number): void {
    const from = -(this.position.y + MIDDLE_Y);
    const to = Math.max(this.depth, 0.05);
    const vy = Math.sqrt(2 * G * (height + MIDDLE_Y));
    const vh = vy / Math.tan(LEAP_ANGLE);
    const run = Math.max((3 * from) / vy, RUN_UP);
    const flight = (2 * vy) / G;
    const plunge = Math.max((3 * to) / vy, PLUNGE);
    const speed = this.speedNow;
    const finish = 0.6 * SWIM_SPEED;
    const out = (run * (speed + vh)) / 2;
    const into = out + vh * flight;
    this.leap = {
      time: 0,
      heading: this.rotation.y,
      x: this.position.x,
      z: this.position.z,
      from,
      to,
      speed,
      climb: this.climbNow,
      vh,
      vy,
      run,
      flight,
      plunge,
      out,
      in: into,
      end: into + (plunge * (vh + finish)) / 2,
      finish,
      splashes: 0,
    };
    this.climbNow = 0;
    this.mode = 'jump';
    this.modeTime = 0;
  }

  update(delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    this.time += dt;
    this.modeTime += dt;
    // How fast it is being turned from outside (a preview steering it).
    const steered = dt > 0 ? wrap(this.rotation.y - this.heading) / dt : 0;

    let speed = 0;
    let wave = IDLE_WAVE;
    let bend = THREE.MathUtils.clamp(steered * STEER_BEND, -STEER_LIMIT, STEER_LIMIT);
    let fold = 0;
    let jaw = JAW_REST + BREATH * (0.5 - 0.5 * Math.cos(this.time * 2.3));
    let flare = 0;
    let beat = 1;
    const water = this.position.y + MIDDLE_Y < 0;
    switch (this.mode) {
      case 'swim':
        speed = SWIM_SPEED;
        wave = 1;
        break;
      case 'turn': {
        const u = Math.min(1, this.modeTime / this.turnTime);
        const s = smooth(u);
        this.rotation.y += this.turnBy * (s - this.turned);
        this.turned = s;
        bend = Math.sign(this.turnBy) * TURN_BEND * Math.min(1, Math.abs(this.turnBy) / Math.PI) ** 0.6 * bump(u);
        flare = 0.3 * bump(u);
        wave = 0.35;
        if (u >= 1) this.done();
        break;
      }
      case 'mouth': {
        const u = this.modeTime / GAPE;
        if (u < GAPES) jaw = JAW_REST + (JAW_OPEN - JAW_REST) * bump(u % 1);
        flare = 0.12 * bump(Math.min(1, u / GAPES));
        if (u >= GAPES + 0.25) this.done();
        break;
      }
      case 'jump':
        fold = 1;
        wave = 1.2;
        beat = 2;
        break;
      case 'flop': {
        const u = Math.min(1, this.modeTime / FLOP_TIME);
        bend = FLOP_BEND * Math.sin(2 * Math.PI * u) * (1 - u);
        wave = 0.6;
        beat = 2.5;
        break;
      }
    }

    // Where it goes: along a leap as planned, a flop's hop, or on at its
    // speed, keeping to its depth in water.
    const leap = this.leap;
    if (this.mode === 'jump' && leap) {
      if (this.follow(leap, dt)) {
        this.leap = null;
        this.speedNow = leap.finish;
        this.done();
      }
    } else {
      this.leap = null;
      if (this.mode === 'flop') {
        const u = Math.min(1, this.modeTime / FLOP_TIME);
        this.position.y = this.flopFrom + FLOP * bump(u);
        if (u >= 1) {
          this.position.y = this.flopFrom;
          this.done();
        }
      }
      this.speedNow = ease(this.speedNow, speed, SPEED_RATE, dt);
      if (this.speedNow < 1e-4 && speed === 0) this.speedNow = 0;
      const climb = water && this.mode !== 'flop' ? THREE.MathUtils.clamp((this.swimY - this.position.y) * 2, -CLIMB, CLIMB) : 0;
      this.climbNow = ease(this.climbNow, climb, 3, dt);
      if (!water) this.climbNow = 0;
      this.advance(dt);
      const pitch = water ? THREE.MathUtils.clamp(Math.atan2(this.climbNow, Math.max(this.speedNow, 0.2)), -PITCH_LIMIT, PITCH_LIMIT) : 0;
      this.pitch = ease(this.pitch, pitch, POSE_RATE, dt);
    }

    // Its posture follows.
    const share = Math.min(1, this.speedNow / SWIM_SPEED);
    if (this.mode === 'swim') fold = share;
    this.wave = ease(this.wave, wave * (this.mode === 'swim' ? Math.max(IDLE_WAVE, share) : 1), POSE_RATE, dt);
    this.bend = ease(this.bend, bend, BEND_RATE, dt);
    this.fold = ease(this.fold, fold, POSE_RATE, dt);
    this.jawAngle = ease(this.jawAngle, jaw, 16, dt);
    const beats = Math.max(IDLE_BEAT, this.speedNow / (STRIDE * LENGTH)) * beat;
    this.phase = (this.phase + 2 * Math.PI * beats * dt) % (2 * Math.PI * 1000);
    this.flare = ease(this.flare, flare, POSE_RATE * 2, dt);
    this.pose(share);
    this.heading = this.rotation.y;
  }

  // Moves it dt on along its leap, splashing where its middle breaks the
  // surface. True once the leap is over.
  private follow(leap: Leap, dt: number): boolean {
    const before = leap.time;
    leap.time += dt;
    const t = leap.time;
    const { run, flight, plunge } = leap;
    let s: number;
    let ds: number;
    let y: number;
    let dy: number;
    if (t < run) {
      [s, ds] = hermite(t, run, 0, leap.speed, leap.out, leap.vh);
      [y, dy] = hermite(t, run, -leap.from, leap.climb, 0, leap.vy);
      this.pitch = Math.atan2(dy, Math.max(ds, 0.05));
    } else if (t < run + flight) {
      const f = t - run;
      s = leap.out + leap.vh * f;
      ds = leap.vh;
      y = leap.vy * f - (G * f * f) / 2;
      dy = leap.vy - G * f;
      // Nothing to push on in the air: it turns at a steady rate, from as
      // steep as it left the water to as steep nose down, level at the top.
      this.pitch = LEAP_ANGLE * (1 - (2 * f) / flight);
    } else {
      [s, ds] = hermite(t - run - flight, plunge, leap.in, leap.vh, leap.end, leap.finish);
      [y, dy] = hermite(t - run - flight, plunge, 0, -leap.vy, -leap.to, 0);
      this.pitch = Math.atan2(dy, Math.max(ds, 0.05));
    }
    this.rotation.y = leap.heading;
    const sin = Math.sin(leap.heading);
    const cos = Math.cos(leap.heading);
    this.position.set(leap.x + sin * s, y - MIDDLE_Y, leap.z + cos * s);
    this.speedNow = ds;
    this.climbNow = dy;
    // Splashes where its middle leaves the water and where it comes back.
    const splash = (at: number, up: number) => this.dispatchEvent({ type: 'splash', x: leap.x + sin * at, z: leap.z + cos * at, velocity: new THREE.Vector3(sin * leap.vh, up, cos * leap.vh) });
    if (leap.splashes === 0 && before < run && t >= run) {
      leap.splashes = 1;
      splash(leap.out, leap.vy);
    }
    if (leap.splashes === 1 && t >= run + flight) {
      leap.splashes = 2;
      splash(leap.in, -leap.vy);
    }
    return t >= run + flight + plunge;
  }

  // The spine, the fins and the jaw, from the posture.
  private pose(share: number): void {
    const [head, middle, ...back] = this.spine;
    const yaw = WAVE.map(([size, lag], i) => this.wave * size * Math.sin(this.phase - lag) + this.bend * BEND[i]);
    const still = this.mode === 'swim' || this.jumping ? 0 : 1 - share;
    const sway = SWAY * still * Math.sin(this.time * 0.8);
    head.rotation.y = yaw[0] - yaw[1];
    middle.rotation.set(-this.pitch, yaw[1] + sway, 0);
    middle.position.y = MIDDLE_Y + BOB * still * (0.5 - 0.5 * Math.cos(this.time * 1.3));
    back.forEach((b, i) => (b.rotation.y = yaw[i + 2] - yaw[i + 1]));
    this.tail.rotation.y = yaw[6] - yaw[5];

    // The pectoral fins scull, one after the other, folding back along its
    // sides as it speeds up; the pelvics stir (only ever outward and back,
    // so where it rests on them they never dig into the floor).
    const w = this.time * 2 * Math.PI * SCULL;
    const open = 1 - this.fold;
    const flare = this.flare;
    for (const [pivot, side, phase] of [
      [this.leftPectoralFin, 1, 0],
      [this.rightPectoralFin, -1, Math.PI],
    ] as const) {
      const spread = 0.22 * Math.sin(w + phase) * open + flare - 0.22 * this.fold;
      pivot.rotation.set(0.18 * Math.sin(w + phase + Math.PI / 2) * open + 0.6 * this.fold, 0, side * spread);
    }
    const stir = 0.1 * (0.5 - 0.5 * Math.cos(w * 0.8)) * open + 0.6 * flare;
    this.leftPelvicFin.rotation.set(0.35 * this.fold, 0, stir);
    this.rightPelvicFin.rotation.set(0.35 * this.fold, 0, -stir);
    this.jawBone.rotation.x = this.jawAngle;
  }
}
