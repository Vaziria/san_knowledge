import * as THREE from 'three';
import { halo } from '../parts';
import { ForestAnimal, type ForestAnimalOptions } from './ForestAnimal';
import { blend, coat, ease, ellipsoid, eye, gloss, glowing, loft, reach, rigidMesh, Sculpt, seededRandom, sheer, sheet, skinnedMesh, thin, tone, tubeRings, type Palette, type Ring, type Tone } from './parts';

type V3 = THREE.Vector3;

// The forest lake's firefly (ForestFirefly.md), as its reference sheet draws
// it: a low poly beetle of flat facets, its head, thorax and banded abdomen
// orange-brown, big dark glossy eyes, long antennae of little segments, six
// thin jointed legs, two pairs of big see-through wings with a dark leading
// edge and pale veins, and a big faceted lantern at its tail that glows in
// its colour (the sheet's six variations), pulsing brighter and dimmer, with
// a soft halo round it. Named ForestFirefly since the project already has a
// Firefly (objects/Firefly.ts).
//
// Meters; it faces +z, its left is -x, and its origin is on the ground under
// where it hovers: it hovers with the middle of its thorax HOVER above it.
// Its shape is traced from the sheet's side, front and top views in their
// pixels (U m each), and it keeps a firefly's real size: 2 cm from the front
// of its head to the tip of its lantern, straightened (the sheet draws it
// big).
//
// Its parts, each a group whose origin is its pivot: the body (the thorax,
// which the rest hangs from), the head (turning on its neck) with its two
// antennae, the abdomen (hanging from its waist) with the lantern and its
// halo, four wings (each at its root) and six legs (each one mesh bending
// at its knee and ankle with three bones, so it can stand on what it lands
// on). A point light at the lantern is optional and off by default (lights
// are costly in the scenes).

const U = 0.02 / 168; // m per pixel of the sheet's side view: 168 px from the head's front to the lantern's tip, straightened

// A point traced from the sheet: `z` forward and `y` up from the middle of
// the thorax, in its pixels; `x` to its right.
function P(z: number, y: number, x = 0): THREE.Vector3 {
  return new THREE.Vector3(x * U, y * U, z * U);
}

// ---------------------------------------------------------------- colours

// Its body is the same orange-brown in every variation; the lantern, its
// halo and the tint its light gives the wings are the variation's colour.
// Picked from the sheet's pictures, the lit faces a little brighter.
const BODY: Palette = {
  shell: 0xcf6d2e, // head, thorax, legs and antennae
  light: 0xefa566, // the lit crown and pronotum
  dark: 0x6b3518, // between the abdomen's plates, the joints
  band: 0xae5a2b, // the abdomen's plates
  eye: 0x170d08,
  shine: 0xf6e4d8,
  costa: 0x8c5433, // the wings' leading edges
  vein: 0xefd2a4,
};

// The sheet's six colour variations, yellow (its default) first: `glow`
// the lantern, `core` its brightest facets, `edge` its lip against the brown
// plates (and the glow it throws on them), `halo` the light round it, `wing`
// the wings' membrane and `wingGlow` the membrane where the lantern lights it.
const VARIATIONS: Record<string, Palette> = {
  yellow: { ...BODY, glow: 0xffe640, core: 0xfffb8c, edge: 0xf4a82a, halo: 0xffc93a, wing: 0xe4eef8, wingGlow: 0xfbd8a6 },
  green: { ...BODY, glow: 0x8cf25a, core: 0xc8fc92, edge: 0x3fc91a, halo: 0x6ae84a, wing: 0x9fe0c4, wingGlow: 0xa8f094 },
  blue: { ...BODY, glow: 0x62f0f8, core: 0xb4feff, edge: 0x2eaee8, halo: 0x46d6f4, wing: 0x9cd0f4, wingGlow: 0x9ce8fa },
  purple: { ...BODY, glow: 0xde66fa, core: 0xf5a8fc, edge: 0x9532c4, halo: 0xc34ef0, wing: 0xb0a0f0, wingGlow: 0xd09af8 },
  red: { ...BODY, glow: 0xfc7078, core: 0xfeb0b4, edge: 0xf24a30, halo: 0xff5050, wing: 0xc8c8d8, wingGlow: 0xf8aa9c },
  white: { ...BODY, glow: 0xfef6e0, core: 0xfffdf6, edge: 0xf4cc9a, halo: 0xfff0d8, wing: 0xccdcea, wingGlow: 0xf2e6dc },
};

// Light comes through its wings as through thin film, from the sun and its
// lantern alike, so they show their colour on either side: a share of it is
// given off, as the sheet's wings glow on their shaded side.
const THROUGH = 0.4;
function translucent(material: THREE.MeshStandardMaterial): THREE.MeshStandardMaterial {
  material.onBeforeCompile = (shader) => {
    shader.fragmentShader = shader.fragmentShader.replace('#include <emissivemap_fragment>', `#include <emissivemap_fragment>
  totalEmissiveRadiance += diffuseColor.rgb * ${THROUGH.toFixed(2)};`);
  };
  material.customProgramCacheKey = () => 'forest-firefly-wing';
  return material;
}

// The broad soft highlight the sheet paints high on each eye, beside the
// small glint eye() gives it: a pale facet lying just off the dome, up and
// forward of its middle on either eye (`side`).
function glint(s: Sculpt, centre: V3, look: V3, radius: number, bulge: number, side: 1 | -1): void {
  const z = look.clone().normalize();
  const x = new THREE.Vector3().crossVectors(new THREE.Vector3(0, 1, 0), z).normalize();
  const y = new THREE.Vector3().crossVectors(z, x);
  const at = (u: number, v: number) => {
    const d = Math.min(0.95, Math.hypot(u, v));
    return centre.clone().addScaledVector(x, side * u * radius).addScaledVector(y, v * radius).addScaledVector(z, radius * bulge * Math.sqrt(1 - d * d) + radius * 0.03);
  };
  const corners = [at(-0.3, 0.52), at(-0.02, 0.62), at(0.12, 0.36), at(-0.2, 0.28)];
  s.toward(corners[0], corners[1], corners[2], z, blend('shine', 'eye', 0.3));
  s.toward(corners[0], corners[2], corners[3], z, blend('shine', 'eye', 0.3));
}

// A repeatable 0..1 for a face, so the facets vary as the sheet's do.
function hash(a: number, b: number): number {
  const h = Math.sin(a * 12.9898 + b * 78.233) * 43758.5453;
  return h - Math.floor(h);
}

// Now and then a facet a little lighter or darker, as the sheet's are.
function patchy(role: string, r: number, k: number, toward = 'dark', amount = 0.3): Tone {
  const f = hash(r, k);
  if (f < 0.25) return blend(role, toward, amount * (0.5 + f * 2));
  if (f > 0.85) return tone(role, 1.07);
  return tone(role);
}

const TAU = Math.PI * 2;

// ---------------------------------------------------------------- the sheet

// Cross-sections along the body, in the sheet's pixels: centred `z`
// forward and `y` up, `w` to either side and `up` and `down` from there.
interface Section {
  z: number;
  y: number;
  w: number;
  up: number;
  down: number;
}

function sections(list: readonly Section[]): Ring[] {
  return list.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U }));
}

// The head, from its back (inside the thorax) to its mouthparts, from the
// side view: a faceted ball, its face sloping down to a small pointed snout.
const HEAD: readonly Section[] = [
  { z: 8, y: 13, w: 10, up: 9, down: 9 },
  { z: 14, y: 14.5, w: 16.5, up: 17, down: 14.5 },
  { z: 21, y: 15.5, w: 20.5, up: 20, down: 18 },
  { z: 30, y: 15, w: 21, up: 20, down: 19 },
  { z: 38, y: 12.5, w: 19.5, up: 17.5, down: 18.5 },
  { z: 45, y: 8.5, w: 15.5, up: 13, down: 14.5 },
  { z: 50, y: 3, w: 10, up: 9, down: 8 },
];
const SNOUT = 5; // px past the last ring to the snout's point
const NECK = P(14, 12); // where the head turns
// The eyes: big, dark and domed on either side of the head.
const EYE = { z: 33, y: 18, radius: 11.5, look: new THREE.Vector3(0.84, 0.14, 0.52) };

// Each antenna from its root in the head, up, out and then forward, from
// the side, front and top views (x, y, z in pixels, the right one).
const ANTENNA: readonly (readonly [number, number, number])[] = [
  [5, 27, 34],
  [8, 33, 37],
  [12, 42, 41],
  [17, 50, 46],
  [22, 56, 53],
  [27, 60, 61],
  [31, 62, 70],
  [35, 63, 79],
];
const ANTENNA_SEGMENTS = 8;
// Its half thickness along it (px): thinning from the root, thickening to
// a club at its tip, as the sheet's close-up draws it.
function antennaRadius(t: number): number {
  return t < 0.6 ? 2.8 - 1.0 * (t / 0.6) : 1.8 + 1.3 * ((t - 0.6) / 0.4);
}

// The thorax, which the head, abdomen, wings and legs hang from: a faceted
// lump behind the head, humped over its front (the pronotum).
const THORAX: readonly Section[] = [
  { z: 19, y: 7, w: 8.5, up: 8.5, down: 8.5 },
  { z: 14, y: 4, w: 14, up: 18, down: 15.5 },
  { z: 7, y: 1, w: 17.5, up: 23.5, down: 21 },
  { z: 0, y: 0, w: 18, up: 24, down: 22 },
  { z: -7, y: -1, w: 17.5, up: 21.5, down: 20.5 },
  { z: -13, y: -2, w: 15, up: 17, down: 16.5 },
  { z: -18, y: -3, w: 12, up: 12, down: 12 },
];

// The abdomen, built straight back from its waist (where it hangs from the
// thorax): three brown plates with darker lines between them, then the
// lantern, a big faceted egg ending in a blunt point, as the sheet's top
// and back views draw it. `a` is px back from the waist.
const WAIST = P(-15, -3);
interface Plate {
  a: number;
  w: number;
  up: number;
  down: number;
}
const PLATES: readonly Plate[] = [
  { a: 0, w: 11, up: 10, down: 10 },
  { a: 5, w: 16.5, up: 15.5, down: 15.5 },
  { a: 15, w: 18.5, up: 17.5, down: 18 },
  { a: 17, w: 17.7, up: 16.7, down: 17.2 }, // the line before the second plate
  { a: 27, w: 19.8, up: 18.8, down: 19.5 },
  { a: 29, w: 19, up: 18, down: 18.7 }, // before the third
  { a: 40, w: 20.8, up: 19.8, down: 20.8 },
];
const LANTERN: readonly Plate[] = [
  { a: 40, w: 20.8, up: 19.8, down: 20.8 },
  { a: 43, w: 22, up: 21, down: 22 },
  { a: 54, w: 23.2, up: 22.2, down: 23.2 },
  { a: 66, w: 22.5, up: 21.5, down: 22.5 },
  { a: 78, w: 19.5, up: 18.5, down: 19.5 },
  { a: 89, w: 13.5, up: 13, down: 13.5 },
  { a: 97, w: 6.5, up: 6.5, down: 6.5 },
];
const LANTERN_TIP = 7; // px past its last ring
const LANTERN_MIDDLE = 62; // px back from the waist, where its halo and light sit
const SIDES = 10; // round the abdomen and the lantern

function plates(list: readonly Plate[]): Ring[] {
  return list.map((p) => ({ at: new THREE.Vector3(0, 0, -p.a * U), w: p.w * U, up: p.up * U, down: p.down * U }));
}

// A wing's outline, from its root along its leading edge (c > 0) to its tip
// and back along its trailing edge, in shares of its length: `s` out along
// it, `c` across it. Traced from the sheet's close-up: a long oval whose
// leading edge is nearly straight. Its veins fork from FORK, as there.
const WING_OUTLINE: readonly (readonly [number, number])[] = [
  [0, 0.025],
  [0.1, 0.075],
  [0.25, 0.125],
  [0.42, 0.16],
  [0.6, 0.18],
  [0.76, 0.18],
  [0.88, 0.16],
  [0.96, 0.11],
  [1, 0.04],
  [0.99, -0.04],
  [0.95, -0.11],
  [0.87, -0.18],
  [0.75, -0.23],
  [0.6, -0.255],
  [0.44, -0.245],
  [0.29, -0.2],
  [0.16, -0.13],
  [0.07, -0.065],
  [0.01, -0.025],
];
const LEADING = 8; // the outline's corners 0..LEADING are its leading edge
const FORK: readonly [number, number] = [0.62, -0.01];
const VEINS: readonly (readonly [readonly [number, number], readonly [number, number]])[] = [
  [[0.03, 0], FORK],
  [FORK, [0.99, 0.035]],
  [FORK, [0.86, 0.16]],
  [FORK, [0.93, -0.14]],
  [[0.3, -0.004], [0.43, 0.155]],
  [[0.37, -0.006], [0.57, -0.245]],
];
const MEMBRANE = 0.62; // the wings' opacity
const COSTA = { from: 1.7, to: 0.9 }; // px half thickness of the leading edge, root to tip
const VEIN = 0.65; // px half width of a vein
const RIM = 0.55; // px half width of the pale trailing edge

// A wing's pose (radians): `sweep` its tip back from straight out, `raise`
// it up from level, `twist` its leading edge down about its length.
interface WingPose {
  sweep: number;
  raise: number;
  twist: number;
}
interface WingBuild {
  root: readonly [number, number, number]; // px, the right one's
  length: number; // px
  spread: WingPose; // hovering, as the sheet's views draw it
  fly: WingPose; // swept back a little, flying
  folded: WingPose; // lying back over the abdomen, landed
  beat: number; // share of the beat's swing
  lag: number; // radians its beat comes after the front pair's
  glow: number; // how much the lantern tints it
}
const FRONT_WING: WingBuild = {
  root: [7, 13, -2],
  length: 136,
  spread: { sweep: 0.75, raise: 0.42, twist: -0.85 },
  fly: { sweep: 0.9, raise: 0.36, twist: -0.7 },
  folded: { sweep: 1.47, raise: -0.02, twist: 0 },
  beat: 1,
  lag: 0,
  glow: 0.75,
};
const HIND_WING: WingBuild = {
  root: [7, 10.5, -8],
  length: 106,
  spread: { sweep: 0.97, raise: -0.15, twist: -0.7 },
  fly: { sweep: 1.15, raise: -0.1, twist: -0.55 },
  folded: { sweep: 1.42, raise: -0.05, twist: 0 },
  beat: 0.85,
  lag: 0.5,
  glow: 1,
};
// The blur a fast beat leaves: the wing drawn this far either way of its
// middle, at these shares of its swing, faintly.
const BLUR_SWING = 0.62;
const BLUR_AT = [-1, 0, 1];
const WING_CLEAR = 0.0003; // m its wings keep above what it lands on

// The lowest a wing may be raised (rad; < 0 is below level) with the middle
// of the thorax `room` m above what it lands on, so that its tip and its
// edges (out to 0.26 of its length either side of its line, turned `twist`
// rad about it) stay WING_CLEAR above that; -Infinity with room to spare.
function lowestRaise(b: WingBuild, twist: number, room: number): number {
  const L = b.length * U;
  const s = (room + b.root[1] * U - WING_CLEAR - 0.26 * L * Math.sin(Math.min(twist, Math.PI / 2))) / L;
  return s >= 1 ? -Infinity : -Math.asin(Math.max(-1, s));
}

// `x` kept above `low`, easing onto it rather than stopping dead.
function above(x: number, low: number): number {
  const k = 0.06; // rad
  const d = (x - low) / k;
  return d > 20 ? x : low + k * Math.log1p(Math.exp(d));
}

// The legs: three a side, each rooted under the thorax and bending in its
// own upright plane, turned `yaw` from straight ahead toward its side (the
// front pair forward, the hind pair back), its femur, tibia and tarsus
// (px) at `fly` radians from straight down (outward > 0) as it hangs in
// flight, and its foot `stance` px out from its root when it stands.
interface LegBuild {
  root: readonly [number, number, number]; // px, the right one's
  yaw: number;
  femur: number;
  tibia: number;
  tarsus: number;
  fly: readonly [number, number, number];
  stance: number;
}
const LEGS: readonly LegBuild[] = [
  { root: [8, -13, 11], yaw: 0.7, femur: 27, tibia: 31, tarsus: 17, fly: [1.15, 0.4, 0.95], stance: 40 },
  { root: [9.5, -16, 1], yaw: 1.62, femur: 29, tibia: 34, tarsus: 18, fly: [1.15, 0.38, 0.8], stance: 38 },
  { root: [9.5, -14, -9], yaw: 2.3, femur: 31, tibia: 37, tarsus: 20, fly: [1.15, 0.32, 0.72], stance: 44 },
];
const TARSUS_STAND = 1.05; // rad from straight down its tarsus lies at, standing
const CLAW = 1.6; // px its tarsus ends in a point past its last joint
const FOOT_CLEAR = 0.00012; // m its claws stay above what it stands on
const FOOT_REACH = 0.002; // m a foot reaches above or below what the firefly stands on
const STAND = 48; // px the middle of the thorax stands above what it lands on
// m it rises, taking off, before its legs hang again: they hang lower than
// they stand, so sooner they would brush what it stood on.
const LEGS_HANG = 0.0045;

// ---------------------------------------------------------------- motion

const HOVER = 0.045; // m its thorax's middle hovers above its origin
const SPEED = 0.18; // m/s it flies at
const TURN_RADIUS = 0.02; // m round the circle a Turn() follows
const TURN_SPEED = 0.08; // m/s it slows to, turning
const TURN_LEAST = 0.05; // m/s it turns at least as fast as, from a hover
const TURN_BANK = 0.12; // s: it banks this many radians for each radian a second it turns
const BANK_MOST = 0.7;
const DESCENT = 0.03; // m/s at most it comes down or goes up, landing and taking off
const REST = 3; // s it rests after landing before it takes off
const SETTLE = 0.9; // s it takes to fold its wings, landed
const RATES = { speedUp: 3, slowDown: 4.5, pose: 4, beat: 3, fold: 3.5, legs: 5, bank: 6 };

// The lantern pulses brighter and dimmer every PULSE.period s: its glow
// from `low` of its colour to all of it, its halo from `halo[0]` to
// `halo[1]` of its strength and a little bigger, and the light (if any).
const PULSE = { period: 2.4, low: 0.8, halo: [0.35, 1] as const, grow: 0.16, sharp: 1.5 };
const HALO = { size: 0.022, strength: 0.85 }; // m across
const LIGHT = { low: 0.004, high: 0.02, reach: 0.07 }; // candela, m
const OWN = 0.1; // with a seed, it pulses up to this share faster or slower

type Mode = 'idle' | 'hover' | 'fly' | 'turn' | 'land';
type Landing = 'down' | 'settle' | 'rest' | 'up';

// What each pose eases toward.
interface Pose {
  pitch: number; // rad its nose is down (< 0 up)
  hang: number; // rad the abdomen hangs below the thorax's line
  rate: number; // wing beats a second
  swing: number; // rad each beat swings either way
  blur: number; // 0..1: how much the wings show as a blur
  flying: number; // 0 hovering, 1 flying: the wings' sweep
  fold: number; // 0 spread, 1 folded back over the body
  legs: number; // 0 hanging, 1 reaching for what it lands on
  tuck: number; // 0..1: the legs drawn back, flying
  feelers: number; // rad the antennae are swept back
  bob: number; // m it rises and falls
  bobRate: number; // times a second
  sway: number; // rad it turns either way, hovering
}

const STILL: Readonly<Pose> = { pitch: 0, hang: 0, rate: 0, swing: 0, blur: 0, flying: 0, fold: 0, legs: 0, tuck: 0, feelers: 0, bob: 0, bobRate: 0.7, sway: 0 };

const POSES: Record<Exclude<Mode, 'land'>, Readonly<Pose>> = {
  idle: { ...STILL, pitch: -0.06, hang: 0.7, rate: 4.5, swing: 0.45, feelers: 0, bob: 0.0015, bobRate: 0.7, sway: 0.1 },
  hover: { ...STILL, pitch: -0.12, hang: 0.74, rate: 24, swing: 0.62, blur: 1, bob: 0.0006, bobRate: 2.2, sway: 0.03 },
  fly: { ...STILL, pitch: 0.18, hang: 0.3, rate: 16, swing: 0.55, blur: 0.8, flying: 1, tuck: 1, feelers: 0.3, bob: 0.0008, bobRate: 1.6 },
  turn: { ...STILL, pitch: 0.05, hang: 0.42, rate: 18, swing: 0.6, blur: 0.9, flying: 0.6, tuck: 0.6, feelers: 0.2, bob: 0.0004, bobRate: 2 },
};

export interface ForestFireflyOptions extends ForestAnimalOptions {
  // A small light at its lantern, in its glow colour, pulsing with it; off
  // by default, since every light costs every lit surface in a scene.
  light?: boolean;
  // m/s it flies at (its cruise); SPEED by default.
  speed?: number;
}

// Its wings' pieces, as each wing is posed.
interface WingRig {
  build: WingBuild;
  side: 1 | -1;
  blade: THREE.Group;
  blur: THREE.Group;
}

interface LegRig {
  build: LegBuild;
  side: 1 | -1;
  bones: readonly [THREE.Bone, THREE.Bone, THREE.Bone];
}

const q1 = new THREE.Vector3();
const q2 = new THREE.Vector3();
const m1 = new THREE.Matrix4();

export class ForestFirefly extends ForestAnimal {
  // The sheet's colour variations, the default (yellow) first.
  static readonly COLORS = Object.keys(VARIATIONS);
  // m its thorax's middle hovers above its origin.
  static readonly HOVER = HOVER;
  // m/s it flies at, unless told otherwise.
  static readonly SPEED = SPEED;
  // m round the circle Turn() takes it round: a turn from flying along a
  // circle twice as wide ends over the circle's middle.
  static readonly TURN_RADIUS = TURN_RADIUS;

  readonly body = new THREE.Group();
  readonly head = new THREE.Group();
  readonly antennae: THREE.Group[] = []; // left, right
  readonly abdomen = new THREE.Group();
  readonly lantern: THREE.Mesh;
  readonly wings: THREE.Group[] = []; // front left, front right, hind left, hind right
  readonly legs: THREE.Group[] = []; // front left, front right, middle left, middle right, hind left, hind right
  readonly light: THREE.PointLight | null;
  // m/s it flies at.
  cruise: number;
  // What lies under it, for Land(): the height (y) at a point (x, z) of its
  // parent's space of what it comes down on there. Unset, the ground at its
  // origin's level. The preview sets it to its leaf.
  surfaceAt: ((x: number, z: number) => number) | null = null;

  private readonly glowMaterial: THREE.MeshBasicMaterial;
  private readonly membrane: THREE.MeshStandardMaterial;
  private readonly blurMaterial: THREE.MeshStandardMaterial;
  private readonly glowSprite: THREE.Sprite;
  private readonly wingRigs: WingRig[] = [];
  private readonly legRigs: LegRig[] = [];
  private readonly pose: Pose = { ...POSES.idle };
  private mode: Mode = 'idle';
  private landing: Landing = 'down';
  private phaseTime = 0;
  private time = 0;
  private lift = HOVER; // m its thorax's middle is above its origin
  private perch = 0; // m above its origin what it lands on is
  private beat = 0; // radians through its wing beat
  private bobPhase = 0;
  private bank = 0;
  private yawRate = 0; // rad/s it is turning, eased
  private lastYaw = 0;
  private turnWay: 1 | -1 = 1;
  private turned = 0;
  private pulseRate = 1 / PULSE.period;
  private pulseAt = 0.5; // at its brightest, unless a seed says otherwise
  private pulseNow = 0;

  constructor(options: ForestFireflyOptions = {}) {
    super(VARIATIONS, options);
    this.name = 'forest firefly';
    this.cruise = options.speed ?? SPEED;
    const random = seededRandom(options.seed ?? 12);
    if (options.seed !== undefined) {
      this.pulseAt = random();
      this.pulseRate *= 1 + OWN * (2 * random() - 1);
      this.bobPhase = random() * TAU;
    }
    const palette = this.palette;
    const matte = coat();
    const shiny = gloss(0.22);
    this.glowMaterial = glowing();
    this.membrane = translucent(sheer(MEMBRANE));
    this.membrane.name = 'firefly wing';
    this.blurMaterial = translucent(sheer(0.12));
    this.blurMaterial.name = 'firefly wing blur';
    const frame = thin(0.6);

    this.rig.rotation.order = 'YXZ'; // it sways, pitches, then banks
    this.rig.add(this.body);

    // The thorax.
    const thorax = new Sculpt(random);
    loft(thorax, sections(THORAX), {
      sides: 8,
      turn: Math.PI / 8,
      start: 'flat',
      end: 'flat',
      paint: (r, angle, k) => {
        const top = Math.min(angle, TAU - angle);
        if (top > 2.2) return blend('shell', 'dark', 0.45); // underneath
        if (top < 0.6 && r >= 1 && r <= 2) return blend('shell', 'light', 0.4); // the pronotum's crown
        return patchy('shell', r, k);
      },
    });
    this.body.add(this.painting(rigidMesh(thorax.geometry(palette), matte)));

    // The head, rigid on its neck, with its eyes and antennae.
    const headSculpt = new Sculpt(random);
    const eyes = new Sculpt(random, 0.01);
    loft(headSculpt, sections(HEAD), {
      sides: SIDES,
      turn: Math.PI / SIDES,
      start: 'flat',
      end: SNOUT * U,
      paint: (r, angle, k) => {
        const top = Math.min(angle, TAU - angle); // 0 on top, π underneath
        if (r >= HEAD.length - 2) return top > 1.8 ? tone('dark', 1.15) : blend('shell', 'dark', 0.3); // the mouthparts
        if (top > 2.3) return blend('shell', 'dark', 0.4); // under the head
        if (top < 0.5 && r <= 3) return blend('shell', 'light', 0.45); // the crown
        if (r === 3 && top > 1.1 && top < 1.9) return blend('shell', 'light', 0.2); // round the eyes
        return patchy('shell', r, k);
      },
    });
    for (const side of [-1, 1] as const) {
      const look = EYE.look.clone().setX(EYE.look.x * side).normalize();
      const at = headSculpt.onto(P(EYE.z, EYE.y, side * 40), new THREE.Vector3(-side, 0, 0)).addScaledVector(look, 0.22 * EYE.radius * U);
      eye(eyes, at, look, new THREE.Vector3(0, 1, 0), EYE.radius * U, { lid: tone('eye'), iris: tone('eye', 1.35), pupil: tone('eye'), shine: tone('shine') }, 9, 0.6, 0.8);
      glint(eyes, at, look, EYE.radius * U, 0.8, side);
    }
    const toNeck = new THREE.Matrix4().makeTranslation(-NECK.x, -NECK.y, -NECK.z);
    this.head.position.copy(NECK);
    this.head.rotation.order = 'YXZ';
    this.head.add(this.painting(rigidMesh(headSculpt.geometry(palette).applyMatrix4(toNeck), matte)));
    this.head.add(this.painting(rigidMesh(eyes.geometry(palette).applyMatrix4(toNeck), shiny)));
    this.body.add(this.head);
    for (const side of [-1, 1] as const) this.antennae.push(this.buildAntenna(side, random, matte));

    // The abdomen, hanging from its waist: its plates, and the lantern.
    const abdomen = new Sculpt(random);
    loft(abdomen, plates(PLATES), {
      sides: SIDES,
      turn: Math.PI / SIDES,
      start: 'flat',
      end: 'flat',
      paint: (r, angle, k) => {
        const below = Math.abs(angle - Math.PI) < 1.3;
        if (r <= 0 || r === 2 || r === 4) return tone('dark', r <= 0 ? 0.9 : 1); // inside the waist, and the lines between the plates
        if (r === 5 && below) return blend('band', 'edge', 0.5); // lit by the lantern behind it
        return patchy('band', r, k, 'dark', 0.35);
      },
    });
    this.abdomen.position.copy(WAIST);
    this.abdomen.add(this.painting(rigidMesh(abdomen.geometry(palette), matte)));
    const lantern = new Sculpt(random, 0.06);
    const last = LANTERN.length - 1;
    loft(lantern, plates(LANTERN), {
      sides: SIDES,
      turn: Math.PI / SIDES,
      start: 'flat',
      end: LANTERN_TIP * U,
      paint: (r, angle, k) => {
        // Deep at its lip against the brown, its own colour along its front,
        // paler toward its tail and underneath, where it shines from, as the
        // sheet's close-up of it glows; its facets each a little apart.
        const f = hash(r + 3, k);
        const top = Math.min(angle, TAU - angle) < 0.9;
        if (r <= 0) return blend('edge', 'glow', 0.15);
        if (r === 1) return blend('edge', 'glow', top ? 0.45 : 0.65);
        if (f < 0.18 && r <= 3) return blend('glow', 'edge', 0.3);
        const under = Math.abs(angle - Math.PI) < 1.3 ? 0.3 : top ? -0.15 : 0;
        return blend('glow', 'core', THREE.MathUtils.clamp(-0.1 + (0.75 * Math.min(r, last)) / last + under + (f - 0.5) * 0.45, 0, 1));
      },
    });
    this.lantern = this.painting(rigidMesh(lantern.geometry(palette), this.glowMaterial));
    this.lantern.receiveShadow = false;
    this.abdomen.add(this.lantern);
    this.glowSprite = halo(palette.halo, HALO.size, HALO.strength);
    this.glowSprite.position.set(0, 0, -LANTERN_MIDDLE * U);
    this.lantern.add(this.glowSprite);
    this.light = options.light ? new THREE.PointLight(palette.halo, LIGHT.low, LIGHT.reach, 2) : null; // casts no shadow
    if (this.light) {
      this.light.position.set(0, 0, -LANTERN_MIDDLE * U);
      this.lantern.add(this.light);
    }
    this.body.add(this.abdomen);

    // The wings, the front pair then the hind, left then right.
    for (const build of [FRONT_WING, HIND_WING]) {
      for (const side of [-1, 1] as const) this.wings.push(this.buildWing(build, side, random, frame));
    }

    // The legs: each one mesh bending with its three bones, bound as it
    // hangs in flight with the firefly at its origin.
    this.updateMatrixWorld(true);
    for (const build of LEGS) {
      for (const side of [-1, 1] as const) this.legs.push(this.buildLeg(build, side, random, matte));
    }

    this.update(0);
  }

  // Rests in the air, bobbing gently, its wings beating slowly. Named as in
  // its sheet's poses.
  Idle(): void {
    this.setMode('idle');
  }

  // Hovers in place, its wings a blur of quick beats. Named as in its
  // sheet's poses.
  Hover(): void {
    this.setMode('hover');
  }

  // Flies forward the way it faces (+z) at its cruise, a little nose down,
  // until told otherwise. Named as in its sheet's poses ("Fly (Forward)").
  Fly(): void {
    this.setMode('fly');
  }

  // Banks and turns round, half a circle TURN_RADIUS round, the way it is
  // already turning (or to its right, +x, flying straight or hovering),
  // slowing as it goes, then hovers. Named as in its sheet's poses.
  Turn(): void {
    if (this.mode === 'turn') return;
    this.turnWay = this.yawRate < -0.3 ? -1 : 1;
    this.turned = 0;
    this.setMode('turn');
  }

  // Settles down onto what is below it (surfaceAt, the ground unless told
  // otherwise), its legs reaching for it, folds its wings back over its
  // body and rests a few seconds, its lantern still glowing, then takes
  // off again and hovers where it was (Idle). Named as in its sheet's poses
  // ("Land on Leaf").
  Land(): void {
    if (this.mode === 'land') return;
    this.setMode('land');
    this.landing = 'down';
    this.phaseTime = 0;
  }

  SetColor(variation: string): void {
    super.SetColor(variation);
    const p = this.palette;
    this.glowSprite.material.color.setHex(p.halo).multiplyScalar(HALO.strength);
    this.light?.color.setHex(p.halo);
  }

  // What it is doing: its behaviour, from the sheet's poses.
  get doing(): Mode {
    return this.mode;
  }

  // How far into a pulse its lantern is: 0 at its dimmest, 1 at its
  // brightest.
  get pulse(): number {
    return this.pulseNow;
  }

  // Whether it stands on what it landed on (its feet down, until it takes
  // off again).
  get landed(): boolean {
    return this.mode === 'land' && this.landing !== 'down' && this.landing !== 'up';
  }

  private setMode(mode: Mode): void {
    if (mode !== this.mode) this.phaseTime = 0;
    this.mode = mode;
  }

  update(delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    this.time += dt;
    this.phaseTime += dt;

    // How fast it is being turned (the preview steers it by turning it),
    // and its own turn.
    let own = 0;
    if (this.mode === 'turn') {
      let step = (Math.max(this.speedNow, TURN_LEAST) / TURN_RADIUS) * dt;
      if (this.turned + step >= Math.PI) step = Math.PI - this.turned;
      this.turned += step;
      own = this.turnWay * step;
    }
    if (dt > 0) {
      // A jump in its heading is it being put somewhere else, not a turn.
      const steered = this.rotation.y - this.lastYaw;
      const turning = THREE.MathUtils.clamp((Math.abs(steered) > 0.5 ? 0 : steered + own) / dt, -12, 12);
      this.yawRate = ease(this.yawRate, turning, RATES.bank, dt);
    }
    this.rotation.y += own;
    this.lastYaw = this.rotation.y;
    if (this.mode === 'turn' && this.turned >= Math.PI - 1e-9) this.setMode('hover');

    // Its speed, and how high it flies.
    const goal = this.goal(dt);
    const speed = this.mode === 'fly' ? this.cruise : this.mode === 'turn' ? TURN_SPEED : 0;
    this.speedNow = ease(this.speedNow, speed, speed > this.speedNow ? RATES.speedUp : RATES.slowDown, dt);
    if (this.speedNow < 1e-5 && speed === 0) this.speedNow = 0;
    this.advance(dt);

    // The pose: what it is doing, eased.
    const p = this.pose;
    p.pitch = ease(p.pitch, goal.pitch, RATES.pose, dt);
    p.hang = ease(p.hang, goal.hang, RATES.pose, dt);
    p.rate = ease(p.rate, goal.rate, RATES.beat, dt);
    p.swing = ease(p.swing, goal.swing, RATES.beat, dt);
    p.blur = ease(p.blur, goal.blur, RATES.beat, dt);
    p.flying = ease(p.flying, goal.flying, RATES.pose, dt);
    p.fold = ease(p.fold, goal.fold, RATES.fold, dt);
    p.legs = ease(p.legs, goal.legs, RATES.legs, dt);
    p.tuck = ease(p.tuck, goal.tuck, RATES.pose, dt);
    p.feelers = ease(p.feelers, goal.feelers, RATES.pose, dt);
    p.bob = ease(p.bob, goal.bob, RATES.pose, dt);
    p.bobRate = ease(p.bobRate, goal.bobRate, RATES.pose, dt);
    p.sway = ease(p.sway, goal.sway, RATES.pose, dt);
    this.beat = (this.beat + TAU * p.rate * dt) % TAU;
    this.bobPhase = (this.bobPhase + TAU * p.bobRate * dt) % TAU;

    // The body: its height, sway, pitch and bank into its turns.
    const t = this.time;
    const moving = Math.min(1, this.speedNow / 0.1);
    this.bank = THREE.MathUtils.clamp(-TURN_BANK * this.yawRate * moving, -BANK_MOST, BANK_MOST);
    this.rig.position.y = this.lift + p.bob * Math.sin(this.bobPhase);
    this.rig.rotation.set(p.pitch + 0.02 * p.swing * Math.sin(this.beat), p.sway * Math.sin(t * 0.5) * Math.sin(t * 0.23 + 1), this.bank);
    this.abdomen.rotation.x = -p.hang + 0.015 * Math.sin(t * 1.3);

    // The head looks about now and then; the antennae sway, and sweep back
    // flying.
    const looking = this.mode === 'idle' || this.landed ? 1 : 0.2;
    this.head.rotation.set(0.06 * Math.sin(t * 0.43) * looking, 0.22 * Math.sin(t * 0.37) * Math.sin(t * 0.21) * looking, 0);
    this.antennae.forEach((antenna, i) => {
      const side = i === 0 ? -1 : 1;
      antenna.rotation.set(-p.feelers + 0.07 * Math.sin(t * 1.7 + i), 0, side * (0.05 * Math.sin(t * 1.1 + 2 * i)));
    });

    this.poseWings();
    this.poseLegs();
    this.glowNow(dt);
  }

  // What its behaviour asks for this frame, and where it flies: its lift,
  // eased here, and the landing's steps.
  private goal(dt: number): Pose {
    if (this.mode !== 'land') {
      this.lift = this.rise(HOVER, dt);
      return POSES[this.mode];
    }
    // What it lands on, under where it is now (it may glide on a little).
    const under = this.surfaceAt ? this.surfaceAt(this.position.x, this.position.z) - this.position.y : 0;
    const stand = under + STAND * U;
    const pose: Pose = { ...STILL, hang: 0.06, rate: 9, swing: 0.5, blur: 0.25, legs: 1, bobRate: 1, feelers: -0.1 };
    switch (this.landing) {
      case 'down':
        this.perch = under;
        this.lift = this.rise(stand, dt);
        if (this.lift - stand < 0.0002 && this.speedNow < 0.004) this.step('settle');
        break;
      case 'settle':
      case 'rest':
        this.lift = stand;
        pose.rate = 0;
        pose.swing = 0;
        pose.blur = 0;
        pose.fold = 1;
        pose.feelers = -0.15;
        if (this.landing === 'settle' && this.phaseTime > SETTLE) this.step('rest');
        if (this.landing === 'rest' && this.phaseTime > REST) this.step('up');
        break;
      case 'up':
        this.lift = this.rise(HOVER, dt);
        pose.rate = 6;
        pose.swing = 0.5;
        pose.legs = this.lift - stand < LEGS_HANG ? 1 : 0;
        pose.hang = 0.4;
        if (this.lift > HOVER - 0.0005) this.Idle();
        break;
    }
    return pose;
  }

  private step(landing: Landing): void {
    this.landing = landing;
    this.phaseTime = 0;
  }

  // Its lift, going toward `height` no faster than DESCENT, and gently at
  // the last.
  private rise(height: number, dt: number): number {
    const gap = height - this.lift;
    const most = DESCENT * dt;
    const step = gap * (1 - Math.exp(-3.5 * dt));
    return this.lift + THREE.MathUtils.clamp(step, -most, most);
  }

  // Each wing at its pose: spread (swept back a little flying), or folded
  // back over the body, beating about it; and the blur of a fast beat.
  private poseWings(): void {
    const p = this.pose;
    const mix = (a: WingPose, b: WingPose, f: number): WingPose => ({
      sweep: a.sweep + (b.sweep - a.sweep) * f,
      raise: a.raise + (b.raise - a.raise) * f,
      twist: a.twist + (b.twist - a.twist) * f,
    });
    const open = 1 - p.fold;
    // Coming down onto something and taking off from it, its thorax's
    // middle is this far above it.
    const room = this.mode === 'land' ? this.lift - this.perch : Infinity;
    for (const wing of this.wingRigs) {
      const b = wing.build;
      const rest = mix(mix(b.spread, b.fly, p.flying), b.folded, p.fold);
      const phase = this.beat - b.lag;
      const swing = p.swing * b.beat * open;
      const twist = rest.twist + 0.3 * swing * Math.cos(phase);
      // Near it, the downstroke stops short of it, and so does the blur.
      const low = lowestRaise(b, Math.abs(twist) + 0.15, room);
      const raise = above(rest.raise + swing * Math.sin(phase), low);
      wing.blade.rotation.set(twist, wing.side * rest.sweep, wing.side * raise, 'YZX');
      // The blur, round the middle of the beat, fading in with a fast beat;
      // the wing itself fades a little into it.
      // Hidden, it shrinks to its root, so that it takes no room in a box
      // round the firefly.
      wing.blur.visible = p.blur * open > 0.02;
      wing.blur.scale.setScalar(wing.blur.visible ? 1 : 1e-6);
      wing.blur.rotation.set(0, wing.side * rest.sweep, wing.side * above(rest.raise, lowestRaise(b, Math.abs(b.spread.twist), room) + BLUR_SWING), 'YZX');
    }
    const blur = p.blur * open;
    this.blurMaterial.opacity = 0.13 * blur;
    this.membrane.opacity = MEMBRANE * (1 - 0.45 * blur);
  }

  // The legs hang in flight, drawn back as it flies, and reach down for
  // what it lands on as it comes down, standing on it once it has.
  private poseLegs(): void {
    const p = this.pose;
    const reaching = p.legs > 0.001;
    if (reaching) {
      this.updateMatrix();
      this.rig.updateMatrix();
      this.body.updateMatrix();
    }
    for (const leg of this.legRigs) {
      const b = leg.build;
      const yaw = b.yaw + 0.35 * p.tuck;
      let [femur, tibia, tarsus] = [b.fly[0] + 0.15 * p.tuck, b.fly[1] + 0.25 * p.tuck, b.fly[2] + 0.3 * p.tuck];
      if (reaching) {
        // Its hip in the firefly's own space, and what it lands on under its
        // foot, stance px out from the hip along the leg's heading: a leaf
        // is arched, so each foot finds its own height on it. A foot past a
        // leaf's edge, where what lies under it is out of FOOT_REACH, stays
        // at the leaf's height halfway out rather than reaching down past
        // the edge; and no foot goes further than FOOT_REACH from what the
        // firefly itself stands on.
        const [hip] = leg.bones;
        m1.multiplyMatrices(this.rig.matrix, this.body.matrix);
        const root = q1.copy(hip.position).applyMatrix4(m1);
        let surface = this.perch;
        const surfaceAt = this.surfaceAt;
        if (surfaceAt) {
          const under = (share: number) => {
            const out = share * b.stance * U;
            q2.set(root.x + leg.side * Math.sin(b.yaw) * out, 0, root.z + Math.cos(b.yaw) * out).applyMatrix4(this.matrix);
            return surfaceAt(q2.x, q2.z) - this.position.y;
          };
          const foot = under(1);
          const low = this.perch - FOOT_REACH;
          surface = THREE.MathUtils.clamp(foot >= low ? foot : Math.max(under(0.5), low), low, this.perch + FOOT_REACH);
        }
        const down = surface - root.y; // m to the surface, < 0
        const ankleY = down + FOOT_CLEAR + (b.tarsus + CLAW) * U * Math.cos(TARSUS_STAND);
        const ankleOut = (b.stance - (b.tarsus + CLAW) * Math.sin(TARSUS_STAND)) * U;
        // Out of reach, it stretches toward it, a little short of straight.
        const most = 0.94 * (b.femur + b.tibia) * U;
        const d = Math.hypot(ankleY, ankleOut);
        const k = d > most ? most / d : 1;
        const solved = reach(0, 0, ankleY * k, ankleOut * k, b.femur * U, b.tibia * U, 1);
        const f = p.legs;
        femur += (solved.upper - femur) * f;
        tibia += (solved.lower - tibia) * f;
        tarsus += (TARSUS_STAND - tarsus) * f;
      }
      const [hip, knee, ankle] = leg.bones;
      hip.rotation.set(-femur, leg.side * yaw, 0, 'YXZ');
      knee.rotation.x = -(tibia - femur);
      ankle.rotation.x = -(tarsus - tibia);
    }
  }

  // The lantern's pulse, its halo and its light.
  private glowNow(dt: number): void {
    this.pulseAt = (this.pulseAt + this.pulseRate * dt) % 1;
    const p = (this.pulseNow = (0.5 - 0.5 * Math.cos(TAU * this.pulseAt)) ** PULSE.sharp);
    this.glowMaterial.color.setScalar(PULSE.low + (1 - PULSE.low) * p);
    this.glowSprite.material.opacity = PULSE.halo[0] + (PULSE.halo[1] - PULSE.halo[0]) * p;
    this.glowSprite.scale.setScalar(HALO.size * (1 - PULSE.grow / 2 + PULSE.grow * p));
    if (this.light) this.light.intensity = LIGHT.low + (LIGHT.high - LIGHT.low) * p;
  }

  // ---------------------------------------------------------------- building

  // An antenna: little segments pinched at their joints, thinning from its
  // root, then thickening to a club with a slanted tip, in a group at its
  // root on the head.
  private buildAntenna(side: 1 | -1, random: () => number, material: THREE.Material): THREE.Group {
    const path = ANTENNA.map(([x, y, z]) => P(z, y, side * x));
    // Evenly along the path: a joint, then a segment's middle, and so on.
    const lengths = path.slice(1).map((p, i) => p.distanceTo(path[i]));
    const total = lengths.reduce((a, b) => a + b, 0);
    const at = (d: number): V3 => {
      let rest = d;
      for (let i = 0; i < lengths.length; i++) {
        if (rest <= lengths[i] || i === lengths.length - 1) return path[i].clone().lerp(path[i + 1], Math.min(1, rest / lengths[i]));
        rest -= lengths[i];
      }
      return path[path.length - 1].clone();
    };
    const points: V3[] = [];
    const radii: number[] = [];
    for (let i = 0; i <= 2 * ANTENNA_SEGMENTS; i++) {
      const t = i / (2 * ANTENNA_SEGMENTS);
      points.push(at(t * total));
      radii.push(antennaRadius(t) * (i % 2 === 0 ? 0.72 : 1) * U);
    }
    const s = new Sculpt(random);
    loft(s, tubeRings(points, radii, new THREE.Vector3(1, 0, 0)), {
      sides: 5,
      mirror: side < 0,
      start: 'flat',
      end: 2.5 * U,
      paint: (r, angle) => {
        const shade = Math.abs(angle - Math.PI) < 1.4 ? 0.8 : 1; // darker underneath
        if (r >= 2 * ANTENNA_SEGMENTS - 1) return blend('shell', 'light', 0.35, shade); // the club's tip
        return r % 2 === 0 ? tone('shell', shade) : blend('shell', 'dark', 0.35, shade);
      },
    });
    const root = path[0];
    const group = new THREE.Group();
    group.position.copy(root).sub(NECK);
    group.rotation.order = 'XZY';
    group.add(this.painting(rigidMesh(s.geometry(this.palette).applyMatrix4(new THREE.Matrix4().makeTranslation(-root.x, -root.y, -root.z)), material)));
    this.head.add(group);
    return group;
  }

  // A wing, in a group at its root: the membrane (see-through, tinted by
  // the lantern toward its root and trailing edge), its frame (the dark
  // leading edge, the pale veins and trailing edge) in a blade that beats,
  // and the blur of a fast beat.
  private buildWing(build: WingBuild, side: 1 | -1, random: () => number, frameMaterial: THREE.Material): THREE.Group {
    const L = build.length;
    // A point of the wing in its own space: out along +x (or -x, the left
    // one), its leading edge toward +z, flat, facing +y.
    const W = (s: number, c: number, lift = 0) => new THREE.Vector3(side * s * L * U, lift * U, c * L * U);
    const up = new THREE.Vector3(0, 1, 0);
    const glow = (middle: V3): Tone => {
      const s = Math.abs(middle.x) / (L * U);
      const c = middle.z / (L * U);
      const f = THREE.MathUtils.clamp((1.05 - s + Math.max(0, -c) * 1.4) * build.glow, 0, 0.95);
      return blend('wing', 'wingGlow', f);
    };
    const outline = WING_OUTLINE.map(([s, c]) => W(s, c));
    const hub = W(FORK[0], FORK[1]);

    const membrane = new Sculpt(random, 0.05);
    sheet(membrane, outline, up, glow, hub);
    const frame = new Sculpt(random, 0.02);
    // The leading edge: a dark rod along it, thinning to the tip.
    const edge = [0, 2, 4, 6, LEADING - 1].map((i) => W(Math.max(0.012, WING_OUTLINE[i][0]), WING_OUTLINE[i][1] - 0.006 - (i === 0 ? 0.01 : 0)));
    const thickness = edge.map((_, i) => (COSTA.from + ((COSTA.to - COSTA.from) * i) / (edge.length - 1)) * U);
    loft(frame, tubeRings(edge, thickness, up), { sides: 4, mirror: side < 0, start: 'flat', end: 1.5 * U, paint: () => tone('costa') });
    // The veins and the trailing edge: pale strips just off the membrane.
    const strip = (a: V3, b: V3, half: number, t: Tone) => {
      const across = new THREE.Vector3().subVectors(b, a).cross(up).normalize().multiplyScalar(half * U);
      const lift = new THREE.Vector3(0, 0.25 * U, 0);
      const [a1, a2, b1, b2] = [a.clone().add(across).add(lift), a.clone().sub(across).add(lift), b.clone().add(across).add(lift), b.clone().sub(across).add(lift)];
      frame.toward(a1, a2, b1, up, t);
      frame.toward(a2, b2, b1, up, t);
    };
    for (const [a, b] of VEINS) strip(W(a[0], a[1]), W(b[0], b[1]), VEIN, tone('vein'));
    for (let i = LEADING; i < WING_OUTLINE.length - 1; i++) {
      const [s0, c0] = WING_OUTLINE[i];
      const [s1, c1] = WING_OUTLINE[i + 1];
      strip(W(s0 - 0.004, c0 * 0.985), W(s1 - 0.004, c1 * 0.985), RIM, blend('vein', 'wingGlow', 0.25));
    }

    const part = new THREE.Group();
    part.position.copy(P(build.root[2], build.root[1], side * build.root[0]));
    const blade = new THREE.Group();
    blade.add(this.painting(rigidMesh(membrane.geometry(this.palette), this.membrane, false)));
    blade.add(this.painting(rigidMesh(frame.geometry(this.palette), frameMaterial, false)));
    part.add(blade);

    // The blur: the membrane again at the ends and middle of its swing,
    // round its twist as it hovers; posed with the wing's sweep and raise.
    const blurSculpt = new Sculpt(random, 0.02);
    const twist = new THREE.Matrix4().makeRotationX(build.spread.twist);
    for (const share of BLUR_AT) {
      const turn = new THREE.Matrix4().makeRotationZ(side * share * BLUR_SWING).multiply(twist);
      const copy = outline.map((p) => p.clone().applyMatrix4(turn));
      sheet(blurSculpt, copy, up.clone().applyMatrix4(turn), glow, hub.clone().applyMatrix4(turn));
    }
    const blur = new THREE.Group();
    blur.add(this.painting(rigidMesh(blurSculpt.geometry(this.palette), this.blurMaterial, false)));
    blur.visible = false;
    part.add(blur);
    this.body.add(part);
    this.wingRigs.push({ build, side, blade, blur });
    return part;
  }

  // A leg: its femur, tibia and tarsus, jointed with knobs at the knee and
  // ankle, one mesh bending with three bones (hip, knee, ankle) in a group
  // of its own on the body. Built and bound as it hangs in flight.
  private buildLeg(build: LegBuild, side: 1 | -1, random: () => number, material: THREE.Material): THREE.Group {
    const part = new THREE.Group();
    this.body.add(part);
    const hip = new THREE.Bone();
    const knee = new THREE.Bone();
    const ankle = new THREE.Bone();
    hip.name = 'hip';
    knee.name = 'knee';
    ankle.name = 'ankle';
    hip.position.copy(P(build.root[2], build.root[1], side * build.root[0]));
    hip.rotation.set(-build.fly[0], side * build.yaw, 0, 'YXZ');
    knee.position.set(0, -build.femur * U, 0);
    knee.rotation.x = -(build.fly[1] - build.fly[0]);
    ankle.position.set(0, -build.tibia * U, 0);
    ankle.rotation.x = -(build.fly[2] - build.fly[1]);
    part.add(hip);
    hip.add(knee);
    knee.add(ankle);
    part.updateMatrixWorld(true);
    const skeleton = new THREE.Skeleton([hip, knee, ankle]);

    const joint = (bone: THREE.Object3D, y = 0) => bone.localToWorld(new THREE.Vector3(0, y * U, 0));
    const [h, k, a] = [joint(hip), joint(knee), joint(ankle)];
    const foot = joint(ankle, -build.tarsus);
    // The plane it bends in: its sideways axis, square to it.
    const across = new THREE.Vector3(1, 0, 0).transformDirection(hip.matrixWorld);
    const s = new Sculpt(random);
    const rod = (from: V3, to: V3, radii: readonly number[], bone: number, tones: (r: number) => Tone, end: number) => {
      s.weights = [bone, bone, 0];
      const points = radii.map((_, i) => from.clone().lerp(to, i / (radii.length - 1)));
      loft(s, tubeRings(points, radii.map((r) => r * U), across), {
        sides: 5,
        mirror: side < 0,
        start: 'flat',
        end: end > 0 ? end * U : 'flat',
        paint: (r, angle) => {
          const t = tones(r);
          return Math.abs(angle - Math.PI) < 1.3 ? { ...t, shade: (t.shade ?? 1) * 0.82 } : t;
        },
      });
    };
    rod(h.clone().lerp(k, -0.12), k, [2.8, 3.7, 3.0], 0, () => tone('shell'), 0);
    rod(k, a, [2.5, 2.7, 2.1], 1, (r) => (r === 0 ? blend('shell', 'dark', 0.3) : tone('shell', 0.95)), 0);
    rod(a, foot, [1.8, 1.4, 1.55, 1.1], 2, (r) => (r % 2 === 1 ? blend('shell', 'dark', 0.45) : blend('shell', 'dark', 0.2)), CLAW);
    // Knobs at the knee and the ankle, as the sheet's close-up of a leg draws
    // them.
    s.weights = [1, 1, 0];
    ellipsoid(s, k, new THREE.Vector3(3, 3, 3).multiplyScalar(U), new THREE.Matrix4().lookAt(k, a, new THREE.Vector3(0, 1, 0)), 1, 5, () => blend('shell', 'dark', 0.4));
    s.weights = [2, 2, 0];
    ellipsoid(s, a, new THREE.Vector3(2.2, 2.2, 2.2).multiplyScalar(U), new THREE.Matrix4().lookAt(a, foot, new THREE.Vector3(0, 1, 0)), 1, 5, () => blend('shell', 'dark', 0.5));
    const mesh = this.painting(skinnedMesh(s.geometry(this.palette, true), material, skeleton));
    part.add(mesh);
    this.legRigs.push({ build, side, bones: [hip, knee, ankle] });
    return part;
  }
}

// ---------------------------------------------------------------- the leaf

// The leaf its preview has it land on, as its sheet's "Land on Leaf" pose
// draws one: broad and pointed, low poly, lying on the ground on its stalk,
// arched along its midrib. Not a part of the firefly: the preview puts it in
// the scenery under where it lands. Meters; it lies along z, its tip toward
// +z, and its origin is on the ground under its middle.
const LEAF_COLORS: Palette = { leaf: 0x4f9a34, rib: 0x8cc454 };
const LEAF = {
  length: 0.085,
  width: 0.042,
  height: 0.012, // m its midrib arches up to
  // Along it, base to tip: share of its length, of its half width, and of
  // its height.
  stations: [
    [0, 0.06, 0.3],
    [0.12, 0.55, 0.62],
    [0.28, 0.88, 0.9],
    [0.45, 1, 1],
    [0.62, 0.93, 1],
    [0.78, 0.72, 0.9],
    [0.9, 0.42, 0.72],
    [1, 0, 0.5],
  ] as const,
  droop: 0.1, // share of its half width its edges fall below its midrib
  stalk: 0.022, // m its stalk runs back to the ground
};

function leafGeometry(seed: number): THREE.BufferGeometry {
  const s = new Sculpt(seededRandom(seed), 0.05);
  const half = LEAF.width / 2;
  const across = [-1, -0.5, 0, 0.5, 1];
  const rows = LEAF.stations.map(([t, w, h]) =>
    across.map((u) => {
      const drop = LEAF.droop * half * w * Math.abs(u) ** 1.5 - (u === 0 ? 0.0005 : 0);
      return new THREE.Vector3(u * w * half, LEAF.height * h - drop, (t - 0.5) * LEAF.length);
    }),
  );
  const up = new THREE.Vector3(0, 1, 0);
  for (let r = 0; r + 1 < rows.length; r++) {
    for (let c = 0; c + 1 < across.length; c++) {
      const rib = c === 1 || c === 2;
      const t = rib && (r + c) % 2 === 0 ? blend('leaf', 'rib', 0.35) : tone('leaf', (r + c) % 2 === 0 ? 1.06 : 0.95);
      const [a, b, cc, d] = [rows[r][c], rows[r][c + 1], rows[r + 1][c + 1], rows[r + 1][c]];
      s.toward(a, b, cc, up, t);
      s.toward(a, cc, d, up, t);
    }
  }
  // The stalk, from the leaf's base back down to the ground.
  const base = rows[0][2];
  const stalk = [base.clone(), base.clone().add(new THREE.Vector3(0, -base.y * 0.55, -LEAF.stalk * 0.55)), new THREE.Vector3(0, 0.0012, base.z - LEAF.stalk)];
  loft(s, tubeRings(stalk, [0.0012, 0.0011, 0.001]), { sides: 4, start: 'flat', end: 'flat', paint: () => blend('leaf', 'rib', 0.5) });
  return s.geometry(LEAF_COLORS);
}

export class FireflyLeaf extends THREE.Mesh {
  // m its midrib arches up to.
  static readonly HEIGHT = LEAF.height;
  private readonly ray = new THREE.Raycaster();

  constructor(seed = 11) {
    super(leafGeometry(seed), thin(0.75));
    this.name = 'leaf';
    this.castShadow = true;
    this.receiveShadow = true;
  }

  // The height (world y) of its top at a world point (x, z), or null off
  // it: what the firefly lands on there.
  heightAt(x: number, z: number): number | null {
    this.updateWorldMatrix(true, false);
    this.ray.set(new THREE.Vector3(x, 10, z), new THREE.Vector3(0, -1, 0));
    const hit = this.ray.intersectObject(this, false)[0];
    return hit ? hit.point.y : null;
  }
}
