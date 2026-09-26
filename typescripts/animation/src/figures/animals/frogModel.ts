import type * as THREE from 'three';
import type { Loft } from './parts';

// The frog as the user modelled it: a low poly frog in three.js, like folded
// paper, pasted into its spec as its shape on 2026-09-26 (Frog.md, "Shape
// reference"). Its numbers are the user's, in the model's own units, and in
// its axes: x to its right, y up, z forward. `unit` makes it 8.6 cm from its
// rump to the tip of its snout (3.85 units), as long as the frog it replaced,
// so it is 5.4 cm to the top of its eyes and 11.7 cm across its hind feet.
//
// Its parts, each a loft of six-sided rings (lofted() in parts.ts), where the
// model puts it:
// - the body and head in one piece (a frog has no neck), tilted up toward
//   the front: pale underneath, green on top, with dark green spots where
//   the model's noise() of a face's centre is under 0.3;
// - two bulging eyes, short lofts pointing up: a green lid ring, gold above
//   it, and three black faces in front, the pupil. The eyes are the same
//   loft, moved, not mirrored, as the model has them;
// - two short front legs ending in a flat hand, dark green, the same loft
//   turned 0.35 rad about z one way and the other, which slants them in
//   under the chest (the model's comment says it splays them out);
// - two hind legs folded as a sitting frog's, 1.4 times the model's units:
//   three lofts side by side, the thigh going forward (pale underneath), the
//   shin coming back beside it, and the long webbed foot lying flat, dark
//   green. The left leg is the right one mirrored (scale -1.4 in x).
//
// The model then stands the frog on y = 0 and centres it front to back, by
// the boxes round its parts as three.js measures them (Box3.setFromObject,
// each part's box turned as the part is); Frog.ts does the same.

// The model's colours, by its names for them (Frog.ts takes them from the
// theme).
export type FrogColor = 'green' | 'darkGreen' | 'cream' | 'gold' | 'black';

// A lofted part, and how the model colours its faces: its colorFor, given a
// face's band, its centre and its hub, in the loft's own units (paintLoft()).
export interface FrogLoft extends Loft {
  paint: (band: number, centre: THREE.Vector3, hub: THREE.Vector3) => FrogColor;
}

type Point = readonly [x: number, y: number, z: number];

// Where the model puts a part: its position, its turn (radians about x, y and
// z, in three.js's order) and its scale.
export interface Placement {
  at: Point;
  turn: Point;
  scale: Point;
}

// A hind leg's lofts, each moved `x` along the leg's x, as the model moves
// the shin and foot beside the thigh.
export interface HindSegment extends FrogLoft {
  x: number;
}

// The model's "random" number from a position, which scatters the spots.
export const noise = (c: THREE.Vector3): number => {
  const v = Math.sin(c.x * 12.99 + c.y * 4.1 + c.z * 78.23) * 43758.55;
  return v - Math.floor(v);
};

const ONE: Point = [1, 1, 1];

export const FROG_MODEL = {
  unit: 0.02234,
  // The body and head in one piece, from the rump to the snout.
  body: {
    sections: [
      [0.4, -1.6, 0.5, 0.4], // rear
      [0.6, -1.2, 1.1, 0.75],
      [0.85, -0.3, 1.3, 0.85], // widest part of the belly
      [1.05, 0.6, 1.25, 0.75], // shoulders
      [1.1, 1.3, 1.05, 0.55], // wide flat head
      [1.0, 1.85, 0.6, 0.3], // snout
    ],
    start: [0.35, -1.8],
    end: [0.95, 2.05],
    paint: (_band, c, hub) => (c.y < hub.y - 0.15 ? 'cream' : noise(c) < 0.3 ? 'darkGreen' : 'green'), // pale underside, spots on top
  } satisfies FrogLoft,
  // A bulging eye, a short tube pointing up.
  eye: {
    sections: [
      [0.0, 0.0, 0.3, 0.3],
      [0.18, 0.0, 0.33, 0.33],
      [0.36, 0.0, 0.24, 0.24],
    ],
    start: 'flat',
    end: [0.44, 0.0],
    paint: (band, c) => (band === 0 ? 'green' : c.z > 0.2 && c.y > 0.22 ? 'black' : 'gold'), // the lid, the pupil, the eye
  } satisfies FrogLoft,
  eyes: [
    { at: [-0.55, 1.55, 1.15], turn: [0, 0, 0], scale: ONE },
    { at: [0.55, 1.55, 1.15], turn: [0, 0, 0], scale: ONE },
  ] satisfies Placement[],
  // A front leg, short, ending in a flat hand.
  front: {
    sections: [
      [0.0, 0.0, 0.22, 0.25],
      [-0.35, 0.12, 0.17, 0.19],
      [-0.62, 0.2, 0.13, 0.15],
      [-0.72, 0.38, 0.24, 0.07], // flat hand
      [-0.74, 0.6, 0.3, 0.05],
    ],
    start: 'flat',
    end: [-0.75, 0.75],
    paint: (band) => (band >= 3 ? 'darkGreen' : 'green'),
  } satisfies FrogLoft,
  fronts: [
    { at: [-0.85, 0.7, 0.9], turn: [0, 0, 0.35], scale: ONE }, // left
    { at: [0.85, 0.7, 0.9], turn: [0, 0, -0.35], scale: ONE }, // right
  ] satisfies Placement[],
  // A hind leg, folded like a sitting frog's: the thigh goes forward, the
  // shin comes back beside it, and the long webbed foot lies flat.
  hind: [
    {
      // the thigh
      x: 0,
      sections: [
        [0.0, 0.0, 0.38, 0.42],
        [-0.15, 0.5, 0.34, 0.36],
        [-0.25, 0.95, 0.26, 0.26], // knee
      ],
      start: 'flat',
      end: [-0.27, 1.1],
      paint: (_band, c, hub) => (c.y < hub.y - 0.1 ? 'cream' : 'green'),
    },
    {
      // the shin, in the model's colour for a part it gives none
      x: 0.38,
      sections: [
        [-0.25, 1.0, 0.22, 0.24],
        [-0.33, 0.45, 0.2, 0.22],
        [-0.4, -0.05, 0.16, 0.18], // ankle
      ],
      start: [-0.2, 1.15],
      end: 'flat',
      paint: () => 'green',
    },
    {
      // the foot
      x: 0.5,
      sections: [
        [-0.42, -0.1, 0.15, 0.1],
        [-0.47, 0.4, 0.22, 0.06],
        [-0.47, 0.9, 0.4, 0.05], // wide webbed toes
      ],
      start: 'flat',
      end: [-0.47, 1.15],
      paint: () => 'darkGreen',
    },
  ] satisfies HindSegment[],
  hinds: [
    { at: [-1.0, 0.4, -1.15], turn: [0, -0.45, 0], scale: [-1.4, 1.4, 1.4] }, // left: negative x mirrors the right leg
    { at: [1.0, 0.4, -1.15], turn: [0, 0.45, 0], scale: [1.4, 1.4, 1.4] }, // right: big powerful jumping legs
  ] satisfies Placement[],
};
