// The snake as the user modelled it: a low poly snake in three.js, given as
// the Shape Reference in its spec (Snake.md) on 2026-09-26, with the other
// animals' models ("rebuild every animal in its model's shape"). Its numbers
// are the user's, in the model's own units and axes (x right, y up, the
// snake facing +z); Snake.ts scales them by one `unit` and puts the body's
// middle at its origin.
//
// - The body is one tube through SEGMENTS rings of SIDES corners, a little
//   flatter than round, along a wavy path; `s` runs along it from 0 at the
//   tail's tip to LENGTH at the neck, and the model lays it along z = s - 6.
//   It is a thin tail thickening, the body, then a slightly thinner neck
//   (radiusAt), which lifts up at the front (liftAt). The tail's end closes
//   to a point TAIL_TIP behind its last ring, and the neck's end is closed
//   flat inside the head.
// - The path swings side to side, x = 0.9 * fade * sin(1.15 s - 2.2 t): the
//   S-shaped wave, the head end wiggling less (fadeAt). The model's wave runs
//   along a body that stays in one place; Snake.ts keeps its shape but lays
//   the body along a wave fixed on the ground instead (see there).
// - Each side panel of a band takes one colour (panelColor): the two panels
//   underneath the belly's, the two on top a zig-zag, alternating, the sides
//   small dark blotches, and the tail's point dark.
// - The head is a flat wedge of faces round its right half, mirrored to the
//   left, placed HEAD.ahead past the neck's end and pointing along the neck,
//   kept fairly level (the neck's rise times HEAD.level).
// - Small yellow eyes with a dark slit (their back half dark), and a thin
//   forked tongue that flicks out for half a second every 2.5 s while it
//   moves.
//
// Beyond the model (it drew every face from both sides, and some of its
// shells are open): the head's back is closed, three faces a side from its
// rim to NAPE, and each eye's base is closed, so every surface can face out
// and be drawn from one side. The model's faces are wound to face in along
// the body and its caps; Snake.ts turns them to face out.

export type Paint = 'green' | 'dark' | 'belly' | 'eye' | 'red';

export const SEGMENTS = 48; // rings along the body
export const SIDES = 6; // corners per ring
export const LENGTH = 10;
export const FLAT = 0.85; // a ring's height, as a share of its width: a little flatter than round
export const TAIL_TIP = 0.3; // the tail's point, this far behind its last ring along the body

// How thick the body is at position s.
export function radiusAt(s: number): number {
  if (s < 3) return 0.06 + 0.128 * s; // thin tail, getting thicker
  if (s > 8.6) return 0.44 - (s - 8.6) * 0.09; // slightly thinner neck
  return 0.44;
}

// The wave the body lies along, x = height * fade * sin(number * s - 2.2 t),
// and how far each part swings, as a share of it: the head end wiggles less.
export const WAVE = { height: 0.9, number: 1.15 };

export function fadeAt(s: number): number {
  return s > 8 ? Math.max(0.15, 1 - (s - 8) * 0.45) : 1;
}

// How high the middle of the body is above the ground: resting on it (the
// ring's radius, flattened), and the neck lifting up.
export function heightAt(s: number): number {
  return radiusAt(s) * FLAT + liftAt(s);
}

export function liftAt(s: number): number {
  return s > 8.4 ? Math.pow(s - 8.4, 2) * 0.55 : 0;
}

// The colour of each side panel, by ring and side: panels 2 and 3 are
// underneath (the belly), panels 0 and 5 on top (an alternating zig-zag),
// panels 1 and 4 the sides (small blotches). Panel k lies between corners k
// and k + 1, corner 0 on top and the others round the right side first.
export function panelColor(i: number, k: number): Paint {
  if (k === 2 || k === 3) return 'belly';
  const step = i % 4;
  if (k === 0) return step < 2 ? 'dark' : 'green';
  if (k === 5) return step >= 2 ? 'dark' : 'green';
  return step === 1 ? 'dark' : 'green';
}
export const TAIL_COLOR: Paint = 'dark'; // the tail's point
export const NECK_COLOR: Paint = 'green'; // the neck's end, inside the head

type Point = readonly [x: number, y: number, z: number];

const HEAD_POINTS = {
  C_topBack: [0, 0.25, -0.35],
  C_topFront: [0, 0.18, 0.55],
  C_snout: [0, 0.05, 0.75],
  C_chin: [0, -0.18, 0.55],
  C_bottomBack: [0, -0.22, -0.35],
  brow: [0.28, 0.2, 0.2],
  cheek: [0.42, 0.0, 0.0],
  back: [0.3, 0.02, -0.4],
  snoutSide: [0.15, 0.06, 0.65],
  jaw: [0.3, -0.15, 0.2],
  jawBack: [0.25, -0.18, -0.35],
  NAPE: [0, -0.03, -0.4], // beyond the model: the middle of the back of the head, which closes it
} as const satisfies Record<string, Point>;

export type HeadPoint = keyof typeof HEAD_POINTS;

// The head: its faces on the right half (x >= 0), each three corners and a
// colour, mirrored to the left with their corners in the other order, all
// facing out. The back's faces are beyond the model.
export const HEAD = {
  ahead: 0.25, // its origin, past the neck's end along the neck
  level: 0.3, // it points along the neck with the neck's rise times this: kept fairly level
  points: HEAD_POINTS,
  faces: [
    ['C_topBack', 'C_topFront', 'brow', 'green'],
    ['C_topBack', 'brow', 'back', 'dark'],
    ['C_topFront', 'snoutSide', 'brow', 'green'],
    ['C_topFront', 'C_snout', 'snoutSide', 'green'],
    ['brow', 'snoutSide', 'cheek', 'green'],
    ['brow', 'cheek', 'back', 'dark'],
    ['snoutSide', 'jaw', 'cheek', 'belly'],
    ['cheek', 'jaw', 'jawBack', 'belly'],
    ['cheek', 'jawBack', 'back', 'green'],
    // The underside's last four, their corners in the other order from the
    // model's (which faced them in), so they face out.
    ['C_snout', 'jaw', 'snoutSide', 'belly'],
    ['C_snout', 'C_chin', 'jaw', 'belly'],
    ['C_chin', 'jawBack', 'jaw', 'belly'],
    ['C_chin', 'C_bottomBack', 'jawBack', 'belly'],
  ],
  // Beyond the model: the back of the head, closed from its rim to the nape,
  // each face the colour of the model's face along that edge of the rim.
  back: [
    ['NAPE', 'C_topBack', 'back', 'dark'],
    ['NAPE', 'back', 'jawBack', 'green'],
    ['NAPE', 'jawBack', 'C_bottomBack', 'belly'],
  ],
} as const satisfies {
  ahead: number;
  level: number;
  points: Record<string, Point>;
  faces: readonly (readonly [HeadPoint, HeadPoint, HeadPoint, Paint])[];
  back: readonly (readonly [HeadPoint, HeadPoint, HeadPoint, Paint])[];
};

const EYE_POINTS = { top: [0, 0.07, 0], front: [0, 0, 0.08], bottom: [0, -0.07, 0], back: [0, 0, -0.08], out: [0.04, 0, 0] } as const satisfies Record<string, Point>;
type EyePoint = keyof typeof EYE_POINTS;

// A small yellow eye with a dark slit: four faces round a point sticking out
// to the side, the front two yellow and the back two dark. The right eye is
// at `at`; the left is its mirror image.
export const EYE = {
  at: [0.33, 0.13, 0.3],
  points: EYE_POINTS,
  faces: [
    ['top', 'front', 'out', 'eye'],
    ['front', 'bottom', 'out', 'eye'],
    ['bottom', 'back', 'out', 'dark'],
    ['back', 'top', 'out', 'dark'],
  ],
  // Beyond the model: its base, closed, against the head.
  base: ['top', 'back', 'bottom', 'front'],
} as const satisfies {
  at: Point;
  points: Record<string, Point>;
  faces: readonly (readonly [EyePoint, EyePoint, EyePoint, Paint])[];
  base: readonly EyePoint[];
};

// The thin forked tongue, made of flat triangles lying level: a stem and two
// forks, from its root at `at` (inside the mouth) forward along +z. It flicks
// out (stretching along z from nothing to its length and back) for `out` s
// every `every` s while the snake moves.
const W = 0.018;
export const TONGUE = {
  at: [0, -0.02, 0.68],
  width: W, // half the stem's width
  triangles: [
    [[-W, 0, 0], [W, 0, 0], [0, 0, 0.4]], // stem
    [[-W, 0, 0.3], [0, 0, 0.36], [-0.08, 0, 0.55]], // left fork
    [[0, 0, 0.36], [W, 0, 0.3], [0.08, 0, 0.55]], // right fork
  ],
  every: 2.5,
  out: 0.5,
} as const satisfies { at: Point; width: number; triangles: readonly (readonly [Point, Point, Point])[]; every: number; out: number };
