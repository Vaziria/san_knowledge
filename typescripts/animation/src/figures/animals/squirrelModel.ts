import { napeOf, rand, stretchedHead } from './animalPark';
import type { Point } from './Head';
import type { Model } from './Quadruped';

// The squirrel as the user modelled it: a low poly red squirrel in three.js,
// like folded paper, from their animal park (the shape reference in
// Squirrel.md, 2026-09-26). Its numbers are the user's, in the model's own
// units; `unit` makes it 22 cm from its rump to its nose (4.61 units), a red
// squirrel's length, so it is 10 cm to the top of its shoulders and 23 cm to
// the top of its tail.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, small and rounded, a loft of six-sided rings from the rump,
//   with a point behind, rising at the front to where the head joins it:
//   red-brown, a white belly;
// - the head, the park's (animalPark.ts) with a short snout: red-brown, a
//   white brow, cheeks, chin and throat, a dark nose, black eyes, and tall
//   tufted ears, white inside and darker behind;
// - the legs: short front legs with small flat hands, and big haunches with
//   long flat feet, the left ones the same as the right;
// - the tail, huge and bushy, curling up over its back, eight-sided, red-
//   brown with darker tufts the page picks by its repeatable "random"
//   numbers.
//
// A front leg bends at the elbow, pointing back, and at the wrist, where the
// hand begins (Leg.ts). A hind leg bends partway down its thigh, forward, and
// at the ankle, above its long foot, its knee keeping the model's bend: bent
// at the knee, its long thigh stood upright when the leg folded at a gallop,
// and the knee went 3.5 mm into the ground.

const BODY: Model['body'] = {
  sections: [
    [0.5, -1.3, 0.5, 0.5],
    [0.6, -0.95, 0.85, 0.85], // the haunches
    [0.6, -0.2, 0.72, 0.75],
    [0.6, 0.55, 0.65, 0.7],
    [0.75, 1.1, 0.52, 0.55], // the shoulders
    [0.95, 1.4, 0.42, 0.45], // the neck
  ],
  start: [0.5, -1.5],
  end: 'flat',
  paint: (_band, c, hub) => (c.y < hub.y - 0.35 && c.z > -0.7 ? 'white' : 'red'),
};

const HEAD_AT: Point = [0, 0.94, 1.98];
const HEAD_SCALE = 0.55;

export const SQUIRREL_MODEL: Model = {
  unit: 0.048,
  body: BODY,
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    ...stretchedHead(
      {
        w: 0.9,
        h: 1.0,
        snout: 0.7,
        snoutW: 0.8,
        snoutDrop: -0.1,
        ear: { scale: 0.8, tip: [0.8, 2.35, -0.1] }, // tall tufted ears
        colors: { top: 'red', brow: 'white', chin: 'white', throat: 'white', cheekLow: 'white', muzzle: 'red', nose: 'nose', eye: 'eye', earIn: 'white', earBack: 'dark' },
      },
      napeOf(BODY, HEAD_AT, HEAD_SCALE),
    ),
  },
  // A huge bushy tail curling up over the back.
  tail: {
    at: [0, 0.6, -1.35],
    sections: [
      [0, 0, 0.2, 0.2],
      [0.4, -0.3, 0.42, 0.4],
      [1.1, -0.5, 0.6, 0.55],
      [1.9, -0.45, 0.68, 0.62],
      [2.6, -0.2, 0.66, 0.6],
      [3.0, 0.25, 0.52, 0.48],
      [3.1, 0.65, 0.34, 0.32],
    ],
    start: 'flat',
    end: [2.95, 0.95],
    sides: 8,
    paint: (_band, c) => (rand(c.x * 13 + c.y * 7 + c.z * 3) < 0.3 ? 'dark' : 'red'),
  },
  front: {
    at: [0.4, 0.3, 0.9],
    sections: [
      [0, 0, 0.18, 0.22],
      [-0.4, 0.05, 0.13, 0.15], // the elbow
      [-0.75, 0.1, 0.1, 0.11], // the wrist
      [-0.9, 0.18, 0.12, 0.08],
      [-0.93, 0.32, 0.11, 0.05], // the hand
    ],
    start: 'flat',
    end: [-0.94, 0.4],
    middle: 1,
    ankle: 2,
    bend: -1,
    paint: () => 'red',
  },
  // Big haunches and long feet.
  hind: {
    at: [0.52, 0.35, -0.8],
    sections: [
      [0, 0, 0.42, 0.6],
      [-0.35, 0.25, 0.33, 0.45], // where it bends
      [-0.6, 0.35, 0.18, 0.2], // the knee
      [-0.8, 0.05, 0.12, 0.14], // the ankle
      [-0.95, 0.1, 0.14, 0.08], // the heel
      [-1.0, 0.5, 0.14, 0.05],
      [-1.01, 0.7, 0.12, 0.04], // the toes
    ],
    start: 'flat',
    end: [-1.02, 0.8],
    middle: 1,
    ankle: 3,
    bend: 1,
    paint: () => 'red',
  },
};
