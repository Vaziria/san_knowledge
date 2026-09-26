import type * as THREE from 'three';
import type { Loft } from './parts';

// The penguin as the user modelled it: a low poly penguin in three.js, flat
// faces like folded paper, pasted into its spec as its shape on 2026-09-26
// (Penguin.md, "Shape Reference"). Its numbers are the user's, in the model's
// own units and axes: x to its right, y up, z forward. `unit` makes it 55 cm
// tall (3.63 units, from the underside of its heels to the top of its head),
// as tall as the penguin it replaced.
//
// Its parts, each where the model puts it (Penguin.ts builds them):
// - the body and head in one loft of eight-sided rings standing up the y
//   axis, from a flat bottom to a point on top: each ring's first corner
//   points back (-z), its width is rx and its depth ry. Black, with a white
//   belly in front and yellow patches either side of the neck;
// - the beak, a short loft pointing forward and down, black on top of an
//   orange stripe;
// - two flippers, thin side to side and wide front to back, hanging from the
//   shoulders and tilted a little away from the body, white on the inside.
//   The left one is the right one mirrored;
// - two flat orange feet pointing forward, turned out a little, the same
//   loft both sides;
// - a short stubby tail, low on the back;
// - two small eyes, flat discs: a light rim round a black middle.
//
// The model then stands the penguin on y = 0 and centres it front to back,
// by the boxes round its parts as three.js measures them (Box3.setFromObject,
// each part's box turned as the part is): 0.01 up and 0.03 forward, since its
// heels reach 0.01 below its origin, its tail 1.14 behind and its beak 1.08
// ahead. Penguin.ts does the same.
//
// Its page waddles the penguin in place (`waddle`): everything but the feet
// (its `upper` group) rocks from side to side about the model's origin, on
// the ground between the feet, the foot bearing no weight lifts, and the
// flippers go out a little further with each step.

// The model's colours, by its names for them (its COLORS; Penguin.ts takes
// them from the theme).
export type PenguinColor = 'black' | 'white' | 'yellow' | 'orange' | 'eyeRim';

type Point = readonly [x: number, y: number, z: number];

// A lofted part (lofted() in parts.ts builds it exactly as the model's
// loft() does): where the model puts its origin, and how the model colours
// its faces, its colorFor given a face's band, its centre and its hub, in the
// loft's own units (paintLoft()).
export interface PenguinLoft extends Loft {
  at: Point;
  paint: (band: number, centre: THREE.Vector3, hub: THREE.Vector3) => PenguinColor;
}

// A flat face in the model's units, its corners in the order the model gives
// them, and its colour.
export interface PenguinFace {
  corners: [Point, Point, Point];
  color: PenguinColor;
}

export const PENGUIN_MODEL = {
  unit: 0.1515, // m: 3.63 units tall is 55 cm

  // PART 1: the body and head in one loft, standing up.
  body: {
    at: [0, 0, 0],
    sections: [
      [0.15, 0.0, 0.55, 0.5], // bottom
      [0.5, 0.0, 0.95, 0.85],
      [1.2, 0.02, 1.05, 0.95], // round belly
      [2.0, 0.0, 0.9, 0.8], // chest
      [2.6, -0.02, 0.62, 0.58], // neck
      [3.0, 0.03, 0.62, 0.6], // head
      [3.35, 0.05, 0.52, 0.5],
    ],
    sides: 8,
    start: 'flat',
    end: [3.62, 0.03],
    paint: (i, c, ring) => {
      const front = c.z - ring.z; // + = toward the front
      if (i === 3 && Math.abs(c.x) > 0.3 && front > -0.2) return 'yellow'; // neck patches
      if (i <= 3 && front > 0.25) return 'white'; // white belly
      return 'black';
    },
  } satisfies PenguinLoft,

  // The beak: black on top, an orange stripe underneath.
  beak: {
    at: [0, 3.12, 0.46],
    sections: [
      [0.0, 0.0, 0.15, 0.12],
      [-0.04, 0.28, 0.1, 0.08],
      [-0.1, 0.5, 0.05, 0.05],
    ],
    start: 'flat',
    end: [-0.15, 0.62],
    paint: (_i, c, ring) => (c.y < ring.y ? 'orange' : 'black'),
  } satisfies PenguinLoft,

  // A flipper: very thin side to side, wide front to back, hanging down from
  // its shoulder (the right one's; the left is mirrored), white on the
  // inside. It rests `rest` radians away from the body.
  flipper: {
    at: [0.88, 2.15, -0.05],
    rest: 0.22,
    sections: [
      [0.0, 0.0, 0.08, 0.26],
      [-0.6, -0.05, 0.07, 0.3],
      [-1.2, -0.1, 0.05, 0.2],
    ],
    start: 'flat',
    end: [-1.55, -0.15],
    paint: (_i, c) => (c.x < -0.02 ? 'white' : 'black'),
  } satisfies PenguinLoft & { rest: number },

  // A flat orange foot pointing forward (the right one's place; the left is
  // at -x), turned out `turn` radians. The left foot is the same loft,
  // turned the other way, not mirrored.
  foot: {
    at: [0.35, 0.05, 0.3],
    turn: 0.25,
    sections: [
      [0, -0.1, 0.12, 0.06],
      [0, 0.2, 0.22, 0.05],
      [0, 0.42, 0.27, 0.04],
    ],
    start: 'flat',
    end: [0, 0.55],
    paint: () => 'orange',
  } satisfies PenguinLoft & { turn: number },

  // A short stubby tail, black (the model's loft() colour when it is given
  // none).
  tail: {
    at: [0, 0.35, -0.72],
    sections: [
      [0.0, 0.0, 0.18, 0.07],
      [-0.05, -0.28, 0.1, 0.04],
    ],
    start: 'flat',
    end: [-0.08, -0.42],
    paint: () => 'black',
  } satisfies PenguinLoft,

  // An eye (the right one's place and turn about y; see eyeFaces()).
  eye: { at: [0.47, 3.25, 0.25] as Point, turn: -0.5 },

  // Its page's waddle: `rock` radians from side to side, the foot bearing no
  // weight up `lift`, the flippers `flippers` radians further out and up to
  // `flap` more with each step, `rate` radians of the step's wave a second.
  waddle: { rate: 4, rock: 0.12, lift: 0.12, flippers: 0.12, flap: 0.25 },
};

// An eye, exactly as the model's buildEye() makes it: a flat disc facing +x,
// a light rim of six corners round a black middle, the middle raised a
// little (its inner ring 0.02 out, its centre 0.03). The model gives each
// face its corners so that it faces +x, out of the head.
export function eyeFaces(): PenguinFace[] {
  const N = 6;
  const R = 0.09;
  const ring = (r: number, x: number): Point[] =>
    [...Array(N).keys()].map((k) => {
      const a = (2 * Math.PI * k) / N;
      return [x, Math.cos(a) * r, Math.sin(a) * r];
    });
  const outer = ring(R, 0);
  const inner = ring(R * 0.6, 0.02);
  const centre: Point = [0.03, 0, 0];
  const faces: PenguinFace[] = [];
  for (let k = 0; k < N; k++) {
    const k2 = (k + 1) % N;
    faces.push({ corners: [outer[k], outer[k2], inner[k2]], color: 'eyeRim' });
    faces.push({ corners: [outer[k], inner[k2], inner[k]], color: 'eyeRim' });
    faces.push({ corners: [centre, inner[k], inner[k2]], color: 'black' });
  }
  return faces;
}
