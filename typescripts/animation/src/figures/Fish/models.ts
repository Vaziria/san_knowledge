import type * as THREE from 'three';
import { mix, type Palette } from '../animals/parts';

// The three kinds of fish as the user modelled them in three.js, low poly
// like folded paper (animals/Fish.md, given on 2026-09-26): "Shape Reference
// (Salmon & Piranha)", one factory, createFish(spec), making a salmon and a
// piranha, and "Shape Reference (Nemo)", a clownfish. The user's choice for
// them: "Three kinds of the fish: the fish figure takes a kind; all three
// swim, dive and leap as the fish does; each gets a preview. The lake and the
// Hold menu use the salmon."
//
// The numbers here are the user's, in each model's own units and axes (x
// right, y up, the fish facing +z); `unit` turns them into meters, and the
// parts are built from them as the models build theirs (Body.ts, Tail.ts,
// Fin.ts). What each model does:
// - the body: rings of `sides` corners, `rings` of them evenly from the
//   profile's first z (the tail end) to its last, each an oval of the
//   profile's half-width rx and half-height ry, raised by its y-offset,
//   read between the profile's rows by straight lines; closed to the nose
//   tip in front and flat behind;
// - fins growing from the rings' top or bottom corner, each a strip from the
//   body out to its height (an inner strip and, the last 20%, its edge);
// - a forked tail fin, a flat sheet at the tail end; two pectoral paddles;
//   two eyes, a coloured ring (the iris) round a black pupil;
// - the piranha's underbite jaw and two rows of teeth.
// Every face is flat and of one colour: a named colour of the model, which
// `colors` maps onto the theme (rules.md, Colour and theme).

export type FishKind = 'salmon' | 'piranha' | 'clownfish';
export const FISH_KINDS: readonly FishKind[] = ['salmon', 'piranha', 'clownfish'];

export type Point = readonly [x: number, y: number, z: number];
// A triangle by the names of its corners, and the name of its colour.
export type Triangle<C extends string> = readonly [a: string, b: string, c: string, color: C];

// A row of the body's profile, from the tail to the nose: at z, the half-width
// rx and half-height ry of the ring there, and how far its middle is raised
// (the clownfish's profile has no such column: its middle stays at y = 0).
export type ProfileRow = readonly [z: number, rx: number, ry: number, yOffset?: number];

export interface FinModel<C extends string> {
  side: 'top' | 'bottom'; // grows from the rings' top corner or their bottom one
  from: number; // it grows from the rings whose z is from `from` to `to`
  to: number;
  height: (z: number) => number; // how far it stands out at a ring's z
  inner: C;
  edge: C;
}

export interface FishModel<C extends string> {
  unit: number; // m per unit of the model
  profile: readonly ProfileRow[];
  noseTip: readonly [z: number, y: number];
  rings: number;
  sides: number;
  // The model's swimming: the back of the body sways, more and more toward
  // the tail, from `stillFrom` back, by `amplitude` at the tail end; its
  // wave has `wavelength` radians a unit and runs at `speed` radians a
  // second. Fish.ts drives it with its own beat.
  swim: { amplitude: number; speed: number; wavelength: number; stillFrom: number };
  // The colour of the band between two rings, for its face at `height` round
  // the ring (the sine of its middle's angle from the side: 1 on top, -1 at
  // the belly), with a repeatable random number for spots.
  colorAt: (z: number, height: number, random: number) => C;
  cap: C; // the faces closing the nose and the tail end
  fins: readonly FinModel<C>[];
  tail: { points: Readonly<Record<string, Point>>; faces: readonly Triangle<C>[]; flick: number }; // flick: radians it turns further than the tail end each way
  pectoral: {
    // Where its root is: x, or null for 0.92 of the body's half-width there,
    // as the factory puts it.
    at: { x: number | null; y: number; z: number };
    points: Readonly<Record<string, Point>>;
    faces: readonly Triangle<C>[];
    flap: { middle: number; stroke: number; speed: number }; // its turn about y: middle + stroke * sin(speed * t)
  };
  eye: {
    // Where its middle is: x, or null for on the ring's oval at 0.97, as the
    // factory puts it.
    at: { x: number | null; y: number; z: number };
    radius: number; // the iris's outer edge
    pupil: number; // the pupil's edge
    pupilX: number; // how far the pupil's edge stands out of the iris's
    centreX: number; // and its middle
    iris: C;
    black: C;
  };
  mouth?: { points: Readonly<Record<string, Point>>; faces: readonly Triangle<C>[]; teeth: readonly (readonly [Point, Point, Point])[]; tooth: C };
  colors: (p: Palette) => Record<C, THREE.Color>;
}

// The factory's helpers, as the user wrote them.

// A repeatable "random" number between 0 and 1 (used for spots).
export const hash = (n: number) => {
  const v = Math.sin(n * 127.1) * 43758.5453;
  return v - Math.floor(v);
};

// A fin's outline: a triangle (sharp peak) or round (a smooth hump); u runs
// 0 -> 1 from its back to its front.
const finShape =
  (type: 'tri' | 'round', peak = 0.6) =>
  (u: number) =>
    type === 'round' ? Math.sin(Math.PI * u) : u < peak ? u / peak : (1 - u) / (1 - peak);

// The factory's fin: its height times its shape at the ring's share of the
// way from `from` to `to`.
function fin<C extends string>(f: { side: 'top' | 'bottom'; from: number; to: number; height: number; shape: (u: number) => number; inner: C; edge: C }): FinModel<C> {
  return { ...f, height: (z) => f.height * f.shape((z - f.from) / (f.to - f.from)) };
}

// The factory's tail fin: a forked flat sheet standing up, pointing -z.
function forkedTail<C extends string>({ size = 1, fork = 0.58, inner, edge }: { size?: number; fork?: number; inner: C; edge: C }) {
  const s = size;
  const points: Record<string, Point> = {
    rootT: [0, 0.22 * s, 0],
    rootB: [0, -0.22 * s, 0],
    iT: [0, 0.6 * s, -0.72 * s],
    iB: [0, -0.6 * s, -0.72 * s],
    iN: [0, 0, -fork * 0.78 * s],
    tT: [0, 0.8 * s, -0.98 * s],
    tB: [0, -0.8 * s, -0.98 * s],
    notch: [0, 0, -fork * s],
  };
  return { points, faces: tailFaces(inner, edge), flick: 0.35 };
}

// Both models' tail fin: the inner part round the root, and the edge.
function tailFaces<C extends string>(inner: C, edge: C): Triangle<C>[] {
  return [
    ['rootT', 'rootB', 'iN', inner],
    ['rootT', 'iN', 'iT', inner],
    ['rootB', 'iB', 'iN', inner],
    ['iT', 'iN', 'notch', edge],
    ['iT', 'notch', 'tT', edge],
    ['iB', 'notch', 'iN', edge],
    ['iB', 'tB', 'notch', edge],
    ['rootT', 'iT', 'tT', edge],
    ['rootB', 'tB', 'iB', edge],
  ];
}

// The factory's small paddle-shaped side fin, the right one (the left is its
// mirror image).
function paddle<C extends string>(color: C, size = 1) {
  const s = size;
  const points: Record<string, Point> = {
    rt: [0, 0.1 * s, 0],
    rb: [0, -0.1 * s, 0],
    a: [0.12 * s, 0.18 * s, -0.4 * s],
    b: [0.22 * s, 0, -0.55 * s],
    c: [0.12 * s, -0.16 * s, -0.4 * s],
  };
  const faces: Triangle<C>[] = [
    ['rt', 'rb', 'b', color],
    ['rt', 'b', 'a', color],
    ['rb', 'c', 'b', color],
  ];
  return { points, faces };
}

// The factory's eye: its iris ring from `radius` in to 0.55 of it, and the
// pupil standing a little out of it.
function hexEye<C extends string>(at: [z: number, y: number], iris: C, radius = 0.13, black: C) {
  return { at: { x: null, y: at[1], z: at[0] }, radius, pupil: radius * 0.55, pupilX: 0.025, centreX: 0.04, iris, black };
}

// PIRANHA: short, very deep body, blunt head, big underbite with teeth.

type PiranhaColor = 'back' | 'silver' | 'speck' | 'red' | 'fin' | 'finRed' | 'black' | 'tooth' | 'iris' | 'pupil';

// The underbite's teeth: lower teeth point up along the jaw's edge, upper
// teeth point down under the snout.
function piranhaTeeth(): (readonly [Point, Point, Point])[] {
  const teeth: (readonly [Point, Point, Point])[] = [];
  const tooth = (x: number, y: number, z: number, dir: number, w = 0.055, h = 0.13) =>
    teeth.push([
      [x, y, z - w],
      [x, y, z + w],
      [x * 0.9, y + h * dir, z],
    ]);
  for (let j = 0; j < 5; j++) {
    const z = 1.45 + j * 0.13,
      x = 0.19 - j * 0.004;
    tooth(x, -0.32, z, 1); // lower row
    tooth(-x, -0.32, z, 1);
    tooth(x * 0.95, -0.27, z + 0.06, -1, 0.05, 0.1); // upper row
    tooth(-x * 0.95, -0.27, z + 0.06, -1, 0.05, 0.1);
  }
  tooth(0.08, -0.33, 2.02, 1, 0.04, 0.12);
  tooth(-0.08, -0.33, 2.02, 1, 0.04, 0.12);
  return teeth;
}

export const PIRANHA: FishModel<PiranhaColor> = {
  unit: 0.0542, // 25 cm from the upper front teeth to the tail fin's tips
  //        z     rx    ry    y-offset
  profile: [
    [-1.6, 0.08, 0.22, 0.0],
    [-1.2, 0.2, 0.55, 0.0],
    [-0.5, 0.38, 1.0, -0.05],
    [0.2, 0.46, 1.15, -0.1], // very deep body
    [0.9, 0.42, 0.95, -0.12],
    [1.4, 0.34, 0.7, -0.15],
    [1.75, 0.24, 0.45, -0.2], // blunt face
  ],
  noseTip: [1.88, -0.22],
  rings: 36,
  sides: 8,
  swim: { amplitude: 0.28, speed: 8, wavelength: 2.4, stillFrom: 0.6 },
  colorAt: (_z, h, rnd) =>
    h < -0.35
      ? 'red' // red belly
      : h > 0.55
        ? 'back' // dark back
        : rnd < 0.25
          ? 'speck'
          : 'silver', // speckled silver sides
  cap: 'silver',
  fins: [
    fin({ side: 'top', from: -0.95, to: 0.15, height: 0.5, shape: finShape('tri', 0.75), inner: 'fin', edge: 'black' }),
    fin({ side: 'top', from: -1.45, to: -1.22, height: 0.14, shape: finShape('round'), inner: 'fin', edge: 'fin' }), // adipose fin
    fin({ side: 'bottom', from: -1.35, to: -0.35, height: 0.48, shape: finShape('tri', 0.8), inner: 'finRed', edge: 'black' }),
  ],
  tail: forkedTail({ size: 0.95, fork: 0.45, inner: 'fin', edge: 'black' }),
  pectoral: { at: { x: null, y: -0.55, z: 1.05 }, ...paddle('finRed', 0.9), flap: { middle: 0.55, stroke: 0.3, speed: 7 } },
  eye: hexEye([1.35, 0.12], 'iris', 0.13, 'pupil'),
  // The underbite jaw.
  mouth: {
    points: {
      tl: [0.2, -0.3, 1.3], // back of the jaw (top)
      tr: [-0.2, -0.3, 1.3],
      fl: [0.17, -0.33, 2.02], // front of the jaw (top)
      fr: [-0.17, -0.33, 2.02],
      bl: [0.18, -0.62, 1.35], // bottom back
      br: [-0.18, -0.62, 1.35],
      cf: [0.0, -0.56, 1.95], // chin
    },
    faces: [
      ['fl', 'fr', 'cf', 'silver'], // front
      ['tl', 'fl', 'cf', 'silver'], // right side
      ['tl', 'cf', 'bl', 'red'],
      ['tr', 'cf', 'fr', 'silver'], // left side
      ['tr', 'br', 'cf', 'red'],
      ['bl', 'cf', 'br', 'red'], // underneath
      ['tl', 'tr', 'fr', 'black'], // inside the mouth
      ['tl', 'fr', 'fl', 'black'],
    ],
    teeth: piranhaTeeth(),
    tooth: 'tooth',
  },
  // Felt has no red: the model's reds (its belly, its jaw's underside, the
  // anal and pectoral fins, the iris) are the theme's rust (autumn), the
  // nearest (every mix of the theme's colours is 30 or more CIELAB units off
  // the model's red; trim, the only one lower in green, is the clownfish's
  // orange); the fins' red a shade darker, as the model's is. Its greys are
  // the water's colour darkened, its black the dark.
  colors: (p) => ({
    back: mix(p.dark, p.water, 0.25),
    silver: p.water.clone(),
    speck: mix(p.dark, p.water, 0.55),
    red: p.autumn.clone(),
    fin: mix(p.dark, p.water, 0.2),
    finRed: mix(p.autumn, p.dark, 0.1),
    black: p.dark.clone(),
    tooth: p.light.clone(),
    iris: p.autumn.clone(),
    pupil: p.dark.clone(),
  }),
};

// SALMON: long, streamlined body, silver sides with a pink stripe, black
// spots on the back and a small adipose fin near the tail.

type SalmonColor = 'back' | 'silver' | 'pink' | 'belly' | 'spot' | 'fin' | 'edge' | 'iris' | 'pupil';

export const SALMON: FishModel<SalmonColor> = {
  unit: 0.0475, // 29.6 cm from the snout to the tail fin's tips, as long as the fish was before its kinds
  profile: [
    [-2.5, 0.08, 0.2, 0.05],
    [-2.1, 0.16, 0.35, 0.05], // narrow tail stem
    [-1.2, 0.35, 0.6, 0.02],
    [-0.2, 0.45, 0.72, 0.0], // thickest part
    [0.8, 0.42, 0.66, -0.02],
    [1.6, 0.33, 0.5, -0.05],
    [2.1, 0.22, 0.33, -0.08],
    [2.4, 0.1, 0.16, -0.12], // pointed snout
  ],
  noseTip: [2.55, -0.14],
  rings: 36,
  sides: 8,
  swim: { amplitude: 0.45, speed: 6, wavelength: 1.5, stillFrom: 1.2 },
  // With eight corners a ring's faces sit at heights of +-0.92 and +-0.38, so
  // no face falls in the pink stripe's band (-0.2 to 0.15): as on the user's
  // page, the sides show silver, and the pink never shows.
  colorAt: (_z, h, rnd) =>
    h > 0.55
      ? rnd < 0.3
        ? 'spot'
        : 'back' // spotted back
      : h > 0.15
        ? rnd < 0.15
          ? 'spot'
          : 'silver' // a few spots on upper sides
        : h > -0.2
          ? 'pink' // pink stripe
          : h > -0.55
            ? 'silver'
            : 'belly', // white belly
  cap: 'silver',
  fins: [
    fin({ side: 'top', from: -0.3, to: 0.65, height: 0.5, shape: finShape('tri', 0.7), inner: 'fin', edge: 'edge' }),
    fin({ side: 'top', from: -1.95, to: -1.65, height: 0.16, shape: finShape('round'), inner: 'fin', edge: 'fin' }), // adipose fin
    fin({ side: 'bottom', from: -1.65, to: -1.05, height: 0.34, shape: finShape('tri', 0.7), inner: 'fin', edge: 'edge' }),
    fin({ side: 'bottom', from: -0.1, to: 0.3, height: 0.22, shape: finShape('tri', 0.7), inner: 'fin', edge: 'fin' }), // pelvic fin
  ],
  tail: forkedTail({ size: 1.2, fork: 0.35, inner: 'fin', edge: 'edge' }),
  pectoral: { at: { x: null, y: -0.3, z: 1.55 }, ...paddle('fin', 1.0), flap: { middle: 0.55, stroke: 0.3, speed: 5 } },
  eye: hexEye([1.95, 0.08], 'iris', 0.11, 'pupil'),
  // Its blue-greys are the sky's (zenith) and the water's colours darkened,
  // its silver the light toward the sky, its spots the body colour (black in
  // felt), its belly the light; the pink (never shown) the lilac toward the
  // orange, and its grey iris the light toward metal.
  colors: (p) => ({
    back: mix(p.dark, p.zenith, 0.3),
    silver: mix(p.light, p.zenith, 0.4),
    pink: mix(p.flower, p.trim, 0.55),
    belly: p.light.clone(),
    spot: p.body.clone(),
    fin: mix(p.dark, p.water, 0.55),
    edge: mix(p.dark, p.zenith, 0.15),
    iris: mix(p.light, p.metal, 0.5),
    pupil: p.dark.clone(),
  }),
};

// CLOWNFISH ("Nemo"): tall and thin, orange with three white bands edged in
// black; the dorsal and anal fins are part of the body, so they bend with it.

type ClownfishColor = 'orange' | 'white' | 'black' | 'fin';

// Three white bands with thin black edges: [centre z, half width].
const BANDS = [
  [1.0, 0.17],
  [0.0, 0.2],
  [-1.45, 0.13],
] as const;

function bandColor(z: number): ClownfishColor {
  for (const [c, w] of BANDS) {
    const d = Math.abs(z - c) - w;
    if (d < 0) return 'white';
    if (d < 0.1) return 'black';
  }
  return 'orange';
}

export const CLOWNFISH: FishModel<ClownfishColor> = {
  unit: 0.0216, // 10 cm from the nose to the tail fin's tips
  // The fish is tall and thin, so each ring is an oval: narrow (rx) and tall
  // (ry). This table is the shape from tail to nose: [z, rx, ry].
  profile: [
    [-1.7, 0.1, 0.28], // narrow tail stem
    [-1.3, 0.2, 0.5],
    [-0.7, 0.38, 0.85],
    [0.0, 0.5, 1.0], // deepest part of the body
    [0.7, 0.48, 0.9],
    [1.2, 0.38, 0.7],
    [1.6, 0.22, 0.42],
    [1.85, 0.08, 0.18], // blunt nose
  ],
  noseTip: [1.95, -0.02],
  rings: 36,
  sides: 8, // 0 = top, 4 = bottom
  // The front stays still, the back swings more and more.
  swim: { amplitude: 0.4, speed: 7, wavelength: 2.2, stillFrom: 0.8 },
  colorAt: (z) => bandColor(z),
  cap: 'orange',
  fins: [
    { side: 'top', from: -1.25, to: 0.9, height: (z) => 0.55 * Math.sin((Math.PI * (z + 1.25)) / 2.15), inner: 'fin', edge: 'black' },
    { side: 'bottom', from: -1.35, to: -0.4, height: (z) => 0.4 * Math.sin((Math.PI * (z + 1.35)) / 0.95), inner: 'fin', edge: 'black' },
  ],
  tail: {
    points: {
      rootT: [0, 0.25, 0],
      rootB: [0, -0.25, 0],
      iT: [0, 0.62, -0.72], // inner outline
      iB: [0, -0.62, -0.72],
      iN: [0, 0, -0.45],
      tT: [0, 0.82, -0.98], // outer edge
      tB: [0, -0.82, -0.98],
      notch: [0, 0, -0.58],
    },
    faces: tailFaces('fin', 'black'),
    flick: 0.35,
  },
  pectoral: {
    at: { x: 0.46, y: -0.25, z: 0.55 },
    points: {
      rt: [0, 0.12, 0],
      rb: [0, -0.12, 0],
      a: [0.15, 0.25, -0.35],
      b: [0.28, 0, -0.5],
      c: [0.15, -0.22, -0.35],
      a2: [0.2, 0.3, -0.42],
      b2: [0.34, 0, -0.6],
      c2: [0.2, -0.27, -0.42],
    },
    faces: [
      ['rt', 'rb', 'b', 'fin'],
      ['rt', 'b', 'a', 'fin'],
      ['rb', 'c', 'b', 'fin'],
      ['a', 'b', 'b2', 'black'],
      ['a', 'b2', 'a2', 'black'],
      ['b', 'c', 'c2', 'black'],
      ['b', 'c2', 'b2', 'black'],
    ],
    flap: { middle: 0.5, stroke: 0.3, speed: 6 },
  },
  eye: { at: { x: 0.31, y: 0.25, z: 1.3 }, radius: 0.15, pupil: 0.085, pupilX: 0.03, centreX: 0.045, iris: 'white', black: 'black' },
  // Orange is the theme's trim (the penguin's beak), white its light and
  // black its dark; the fins, "a lighter, see-through-looking orange", are the
  // trim a fifth of the way to the light, as much lighter as the model's are.
  colors: (p) => ({
    orange: p.trim.clone(),
    white: p.light.clone(),
    black: p.dark.clone(),
    fin: mix(p.trim, p.light, 0.2),
  }),
};

export const FISH_MODELS: Record<FishKind, FishModel<string>> = {
  salmon: SALMON as FishModel<string>,
  piranha: PIRANHA as FishModel<string>,
  clownfish: CLOWNFISH as FishModel<string>,
};
