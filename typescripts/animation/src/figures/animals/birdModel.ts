import type * as THREE from 'three';
import type { Loft } from './parts';

// The bird as the user modelled it: a low poly songbird in three.js, like
// folded paper, given in Bird.md's "Shape Reference" on 2026-09-26 with the
// other animals' models. Its numbers are the user's, in the model's own units
// and axes (x to its right, y up, z forward: it faces +z); `unit` makes it
// 23 cm from the tip of its beak to the tips of its tail feathers
// (4.725 units), as long as the bird was, so it is 14 cm to the top of its
// head (2.87 units) and 9 cm across its folded wings.
//
// Its parts, each where the model puts it:
// - the body and head, one loft of seven-sided rings (a bird's neck is hidden
//   in its feathers) from the base of the tail, with a point behind, through
//   the round belly, the chest and the neck to the round head and the face,
//   with a point in front; blue, the chest and throat orange and the belly
//   white (bodyColor);
// - the beak, a short dark loft, closed flat at its base, pointed;
// - the eyes, short double-pointed lofts turned across the head;
// - the wings, flat faces drawn by hand spread out along +x with a small
//   ridge on top, blue with dark blue flight feathers, the left one the right
//   one mirrored; the model folds them back along the body (`folded`) and
//   opens them (`spread`), and flaps them about their length;
// - the tail feathers, a flat dark blue fan with a ridge, tilted 0.2 rad down
//   from the body behind;
// - the legs, each a five-sided shank from the hip down to the foot, closed
//   flat at both ends, and four toes fanned about the leg's upright axis,
//   three forward and one back, the back one 0.8 the size.
//
// Beyond the model: `neck` names the ring the head turns about, which the body
// and the head share (Bird.ts), and the colours are the theme's (Bird.ts maps
// the model's colour names onto them).

export type Point = readonly [x: number, y: number, z: number];

// The model's colour names (its COLORS).
export type Role = 'blue' | 'darkBlue' | 'orange' | 'white' | 'beak' | 'black' | 'leg';

// A wing's pose as the model gives it: rotations about x, y and z, in the
// order YXZ (the flap about z first, then the twist about x, then the swing
// about y). Written for the right wing; the left one takes -y and -z.
export interface WingPose {
  x: number;
  y: number;
  z: number;
}

export interface BirdModel {
  unit: number; // m per unit of the model
  body: Loft; // the body and head in one
  neck: number; // the ring the head turns about (beyond the model)
  beak: Loft & { at: Point; end: readonly [y: number, z: number] }; // pointed
  eye: Loft & { at: Point; turn: number }; // the right eye; the left one at -x. turn: radians about y
  wing: {
    at: Point; // the right wing's shoulder; the left one at -x
    points: Record<string, Point>;
    faces: readonly (readonly [a: string, b: string, c: string, color: Role])[];
    folded: WingPose;
    spread: WingPose;
    flap: number; // radians the wings beat up and down, fully spread
  };
  tail: {
    at: Point; // the root of the fan
    tilt: number; // radians about x
    points: Record<string, Point>;
    faces: readonly (readonly [a: string, b: string, c: string])[];
  };
  leg: {
    at: Point; // the right leg's hip; the left one at -x
    shank: Loft;
    toe: Loft;
    toes: readonly number[]; // radians each toe is turned about the leg's upright axis: three forward, one back
    back: number; // the back toe's size, as a share of the others'
  };
}

export const BIRD_MODEL: BirdModel = {
  unit: 0.0487,
  body: {
    sections: [
      [0.9, -1.5, 0.18, 0.15], // base of the tail
      [0.75, -1.1, 0.45, 0.4],
      [0.75, -0.4, 0.75, 0.75], // round belly
      [0.95, 0.3, 0.72, 0.8], // chest
      [1.35, 0.75, 0.5, 0.55], // neck
      [1.75, 0.95, 0.52, 0.52], // back of the head
      [1.9, 1.35, 0.45, 0.45],
      [1.85, 1.65, 0.25, 0.25], // face
    ],
    sides: 7,
    start: [0.95, -1.65],
    end: [1.82, 1.75],
  },
  neck: 4,
  // A short pointed beak.
  beak: {
    at: [0, 1.8, 1.68],
    sections: [
      [0.0, 0.0, 0.13, 0.1],
      [-0.03, 0.2, 0.08, 0.06],
    ],
    start: 'flat',
    end: [-0.06, 0.42],
  },
  // A tiny eye: a short double-pointed shape.
  eye: {
    at: [0.4, 1.95, 1.4],
    turn: Math.PI / 2,
    sections: [
      [0, -0.04, 0.09, 0.09],
      [0, 0.04, 0.09, 0.09],
    ],
    start: [0, -0.1],
    end: [0, 0.1],
  },
  wing: {
    at: [0.66, 1.2, 0.3],
    points: {
      rootF: [0.0, 0.0, 0.35],
      rootB: [0.0, 0.0, -0.45],
      ridge: [0.8, 0.12, 0.1], // small bump on top
      midF: [1.0, 0.05, 0.4],
      midB: [1.0, 0.0, -0.7],
      f1: [1.6, 0.0, -0.85], // feather tips
      f2: [2.0, 0.0, -0.65],
      tip: [2.1, 0.0, -0.3],
    },
    faces: [
      ['rootF', 'ridge', 'midF', 'blue'],
      ['rootF', 'rootB', 'ridge', 'blue'],
      ['rootB', 'midB', 'ridge', 'blue'],
      ['ridge', 'midB', 'midF', 'blue'],
      ['midF', 'midB', 'f1', 'darkBlue'],
      ['midF', 'f1', 'f2', 'darkBlue'],
      ['midF', 'f2', 'tip', 'darkBlue'],
    ],
    // x twists the wing so it stands up flat against the body, y swings it
    // back along the body, z lifts it (the flap).
    folded: { x: -1.5, y: 1.62, z: -0.18 },
    spread: { x: 0.0, y: 0.15, z: 0.0 },
    flap: 0.8,
  },
  // Fan-shaped tail feathers.
  tail: {
    at: [0, 0.95, -1.4],
    tilt: -0.2,
    points: {
      root: [0, 0, 0],
      ridge: [0, 0.08, -0.6],
      a: [-0.38, -0.05, -1.1],
      b: [-0.13, 0, -1.25],
      c: [0.13, 0, -1.25],
      d: [0.38, -0.05, -1.1],
    },
    faces: [
      ['root', 'a', 'ridge'],
      ['ridge', 'a', 'b'],
      ['ridge', 'b', 'c'],
      ['ridge', 'c', 'd'],
      ['root', 'ridge', 'd'],
    ],
  },
  // A leg with three toes forward and one back.
  leg: {
    at: [0.3, 0.3, -0.2],
    shank: {
      sections: [
        [0, 0, 0.1, 0.12],
        [-0.35, 0.05, 0.06, 0.06],
        [-0.78, -0.02, 0.04, 0.04],
      ],
      sides: 5,
      start: 'flat',
      end: 'flat',
    },
    toe: {
      sections: [
        [-0.8, 0.0, 0.035, 0.03],
        [-0.8, 0.28, 0.025, 0.02],
      ],
      sides: 5,
      start: 'flat',
      end: [-0.8, 0.36],
    },
    toes: [-0.45, 0, 0.45, Math.PI],
    back: 0.8,
  },
};

// The model's colours for the body and head, as its colorFor gives them: a
// face's band (i), its centre (c) and the middle of its band (ring), in the
// model's units. Below the middle, the chest and throat are orange and the
// belly white; the rest is blue.
export function bodyColor(i: number, c: THREE.Vector3, ring: THREE.Vector3): Role {
  const below = c.y < ring.y - 0.15;
  if (below && i >= 2 && i <= 4 && c.z > -0.1) return 'orange'; // chest + throat
  if (below && i <= 2) return 'white'; // belly
  return 'blue';
}
