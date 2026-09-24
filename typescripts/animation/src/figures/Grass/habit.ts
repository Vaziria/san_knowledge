import * as THREE from 'three';
import { BLADE, between, STEM, type Blade, type Profile, type Tube } from './parts';

// How grasses grow, as lines in meters: a tuft of leaves fanning out of a
// small crown on the ground, and flowering stems (culms) rising out of it,
// with a leaf or two on the way up. Each kind gives its habit (its growth
// form) in numbers, and puts its own seed head on each stem.

export type Range = readonly [number, number];

const UP = new THREE.Vector3(0, 1, 0);
const DOWN = new THREE.Vector3(0, -1, 0);
const MOST_BENT = 2.3; // rad from upright: a leaf's tip hangs at most this far over, about 40° below level
const GROUND = 0.003; // m: a tip that bends down to the ground lies along it, just above

// A unit vector `lean` radians from upright, toward `azimuth` (radians round
// from +x toward +z).
export function heading(lean: number, azimuth: number): THREE.Vector3 {
  return new THREE.Vector3(Math.sin(lean) * Math.cos(azimuth), Math.cos(lean), Math.sin(lean) * Math.sin(azimuth));
}

// Across a leaf growing toward `azimuth`: level, from its left edge to its right.
export function across(azimuth: number): THREE.Vector3 {
  return new THREE.Vector3(-Math.sin(azimuth), 0, Math.cos(azimuth));
}

// A curving line from `base` in `rows` equal steps: it sets off along
// `direction` (a unit vector) and turns `bend` radians toward the ground by
// its end, most of it near the end, as a leaf bends under its own weight. A
// negative bend turns it back up, at most to upright. Nothing goes below the
// ground: a tip that reaches it lies along it.
export function arch(base: THREE.Vector3, direction: THREE.Vector3, length: number, bend: number, rows: number): THREE.Vector3[] {
  const axis = new THREE.Vector3().crossVectors(direction, DOWN);
  const upright = direction.angleTo(UP);
  if (axis.lengthSq() < 1e-8) bend = 0; // straight up or down: no way to bend is likelier than another
  axis.normalize();
  const points = [base.clone()];
  const p = base.clone();
  const d = new THREE.Vector3();
  for (let i = 0; i < rows; i++) {
    const s = (i + 0.5) / rows;
    const turn = bend >= 0 ? Math.min(bend * s * s, Math.max(0, MOST_BENT - upright)) : Math.max(bend * s * s, -upright);
    d.copy(direction);
    if (turn !== 0) d.applyAxisAngle(axis, turn);
    p.addScaledVector(d, length / rows);
    points.push(new THREE.Vector3(p.x, Math.max(p.y, GROUND), p.z));
  }
  return points;
}

// The way a line runs at its end.
export function endDirection(spine: THREE.Vector3[]): THREE.Vector3 {
  return spine[spine.length - 1].clone().sub(spine[spine.length - 2]).normalize();
}

// A unit vector `angle` radians off `direction`, turned toward a random side.
export function splay(direction: THREE.Vector3, angle: number, random: () => number): THREE.Vector3 {
  const other = Math.abs(direction.y) < 0.9 ? UP : new THREE.Vector3(1, 0, 0);
  const a = new THREE.Vector3().crossVectors(direction, other).normalize();
  const b = new THREE.Vector3().crossVectors(direction, a);
  const round = 2 * Math.PI * random();
  const side = a.multiplyScalar(Math.cos(round)).addScaledVector(b, Math.sin(round));
  return direction.clone().multiplyScalar(Math.cos(angle)).addScaledVector(side, Math.sin(angle));
}

// Shoots fanning out of a crown `base` meters round on the ground: each
// starts at a random spot in it and leans away from its middle, the outer
// ones further over (the outer leaves of a tuft are the older ones, which
// flop).
export interface Sprouts {
  count: number;
  base: number; // m, the radius of the crown they come out of
  length: Range; // m
  lean: Range; // rad from upright
  bend: Range; // rad more by the tip
  rows: number;
}

export interface Sprout {
  spine: THREE.Vector3[];
  azimuth: number; // the way it leans
}

export function sprouts(spec: Sprouts, random: () => number, footY = 0): Sprout[] {
  return Array.from({ length: spec.count }, () => {
    const out = Math.sqrt(random());
    const round = 2 * Math.PI * random();
    const base = new THREE.Vector3(spec.base * out * Math.cos(round), footY, spec.base * out * Math.sin(round));
    const azimuth = round + between(random, -0.6, 0.6);
    const lean = THREE.MathUtils.lerp(spec.lean[0], spec.lean[1], 0.4 * out + 0.6 * random());
    const spine = arch(base, heading(lean, azimuth), between(random, ...spec.length), between(random, ...spec.bend), spec.rows);
    return { spine, azimuth };
  });
}

// A tuft of leaf blades.
export interface Tuft extends Sprouts {
  width: Range; // m, at the widest
  fold: number; // see Blade
  profile?: Profile;
}

export interface Paint {
  foot: THREE.Color;
  tip: THREE.Color;
  back?: [foot: THREE.Color, tip: THREE.Color];
}

const TINT = [0.88, 1.08] as const; // each leaf is a little lighter or darker than the next

export function tuft(spec: Tuft, paint: Paint, random: () => number): Blade[] {
  return sprouts(spec, random).map(({ spine, azimuth }) => blade(spine, across(azimuth), between(random, ...spec.width), spec.fold, spec.profile ?? BLADE, paint, random));
}

// A blade along `spine`, painted with a random tint.
export function blade(
  spine: THREE.Vector3[],
  side: THREE.Vector3,
  width: number,
  fold: number,
  profile: Profile,
  paint: Paint,
  random: () => number,
): Blade {
  const tint = between(random, ...TINT);
  const shade = (c: THREE.Color) => c.clone().multiplyScalar(tint);
  return {
    spine,
    side,
    width,
    profile,
    fold,
    foot: shade(paint.foot),
    tip: shade(paint.tip),
    back: paint.back && [shade(paint.back[0]), shade(paint.back[1])],
  };
}

// Flowering stems (culms): upright, leaning a little outward and nodding
// over at the top by `bend`, each with a few leaves along its lower half.
// They start a centimeter below the ground, so their open feet never show.
export interface Culms extends Sprouts {
  radius: number; // m
  leaves: { count: number; at: Range; length: Range; lean: Range; bend: number; width: number; fold: number };
}

const CULM_ROW = 0.04; // m between a stem's rings
const CULM_SIDES = 5;

export function culms(spec: Culms, paint: { stem: Paint; leaf: Paint }, random: () => number): { stems: Tube[]; leaves: Blade[] } {
  const stems: Tube[] = [];
  const leaves: Blade[] = [];
  for (const { spine } of sprouts(spec, random, -0.01)) {
    stems.push(tube(spine, spec.radius, STEM, 0, CULM_ROW, CULM_SIDES, paint.stem));
    for (let k = 0; k < spec.leaves.count; k++) {
      // From a node on the lower stem, a leaf leaning off it the way it faces.
      const at = spine[Math.round(between(random, ...spec.leaves.at) * (spine.length - 1))];
      const azimuth = 2 * Math.PI * random();
      const length = between(random, ...spec.leaves.length);
      const leaf = arch(at, heading(between(random, ...spec.leaves.lean), azimuth), length, spec.leaves.bend, 8);
      leaves.push(blade(leaf, across(azimuth), spec.leaves.width, spec.leaves.fold, BLADE, paint.leaf, random));
    }
  }
  return { stems, leaves };
}

// A tube with its colours; `row` is the m between its rings.
export function tube(spine: THREE.Vector3[], radius: number, profile: Profile, bumps: number, row: number, columns: number, paint: Paint): Tube {
  return { spine, radius, profile, bumps, row, columns, foot: paint.foot, tip: paint.tip };
}

// A turn that stands a blob's own y along `direction`.
export function along(direction: THREE.Vector3): THREE.Quaternion {
  return new THREE.Quaternion().setFromUnitVectors(UP, direction.clone().normalize());
}
