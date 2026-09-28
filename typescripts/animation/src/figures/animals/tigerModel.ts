import { napeOf, stretchedHead, stripe } from './animalPark';
import type { Point } from './Head';
import type { Model } from './Quadruped';

// The tiger as the user modelled it: a low poly tiger in three.js, like
// folded paper, from their animal park (the shape reference in Tiger.md,
// 2026-09-26). Its numbers are the user's, in the model's own units; `unit`
// makes it 1 m to the top of its shoulders (4.51 units), a tiger's height,
// so it is 2.1 m from its nose to where its tail hangs, and its tail is
// 0.9 m long.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, big and muscular, a loft of six-sided rings from the rump, with
//   a point behind, up through the chest and neck to where the head joins
//   it: orange with narrow black bands, and a white belly;
// - the head, the park's (animalPark.ts) made broad with a short, wide snout,
//   like the lion's: orange with black stripes over the forehead and the
//   sides, a white brow, muzzle, cheeks and chin, a pink nose, yellow eyes,
//   and small ears, white inside and black behind;
// - the legs, thick lofts with big paws, striped down to the paw, the left
//   ones the same as the right;
// - the tail, long and hanging down, in orange and black rings, its end
//   black.
//
// A leg bends at two of its rings (Leg.ts), as the lion's do: a front leg at
// the wrist and at the ankle, where the paw begins; a hind leg at the hock
// and the ankle, its knee keeping the model's bend.

// Narrow black bands.
const striped = (v: number, base: string) => (stripe(v, 0.72) ? 'black' : base);

const BODY: Model['body'] = {
  sections: [
    [0.4, -2.6, 0.55, 0.55],
    [0.5, -2.05, 0.92, 0.98], // the hips
    [0.55, -0.8, 0.85, 0.88],
    [0.45, 0.5, 0.95, 1.05], // the chest
    [0.6, 1.6, 0.92, 1.08], // the shoulders
    [1.1, 2.35, 0.7, 0.78],
    [1.5, 2.75, 0.6, 0.66], // the neck
  ],
  start: [0.4, -2.85],
  end: 'flat',
  paint: (_band, c, hub) => (c.y < hub.y - 0.55 ? 'white' : striped(c.z * 3.2 + c.y * 0.8, 'orange')),
};

const HEAD_AT: Point = [0, 1.25, 3.64];
const HEAD_SCALE = 0.85;

export const TIGER_MODEL: Model = {
  unit: 0.222,
  body: BODY,
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    ...stretchedHead(
      {
        w: 1.05,
        h: 1.0,
        snout: 0.55,
        snoutW: 1.45,
        snoutDrop: -0.15,
        ear: { scale: 0.75, tip: [0.95, 1.95, -0.05] },
        colors: { top: 'orange', brow: 'white', muzzle: 'white', snoutTop: 'orange', chin: 'white', cheekLow: 'white', nose: 'nose', eye: 'eye', earIn: 'white', earBack: 'black' },
        // Stripes on the forehead and the sides of the head.
        faceColor: (region, c) => ((region === 'top' || region === 'side') && stripe(c[0] * 7 + c[2] * 2, 0.7) ? 'black' : null),
      },
      napeOf(BODY, HEAD_AT, HEAD_SCALE),
    ),
  },
  tail: {
    at: [0, 0.9, -2.7],
    sections: [
      [0, 0, 0.16, 0.16],
      [0.05, -0.5, 0.14, 0.14],
      [-0.35, -1.0, 0.13, 0.13],
      [-0.95, -1.35, 0.12, 0.12],
      [-1.6, -1.5, 0.11, 0.11],
      [-2.1, -1.35, 0.1, 0.1],
      [-2.45, -1.05, 0.09, 0.09],
    ],
    start: 'flat',
    end: [-2.65, -0.85],
    paint: (band) => (band >= 5 || band % 2 ? 'black' : 'orange'),
  },
  front: {
    at: [0.56, 0.2, 1.4],
    sections: [
      [0, 0, 0.4, 0.58],
      [-0.9, -0.05, 0.3, 0.43],
      [-1.7, 0.05, 0.22, 0.26], // the wrist
      [-2.5, 0.05, 0.2, 0.23],
      [-2.75, 0.12, 0.24, 0.24], // the ankle
      [-2.9, 0.3, 0.31, 0.19],
      [-2.95, 0.55, 0.31, 0.14], // the big paw
    ],
    start: 'flat',
    end: [-2.97, 0.72],
    middle: 2,
    ankle: 4,
    bend: 1,
    paint: (band, c) => (band < 4 ? striped(c.y * 5, 'orange') : 'orange'),
  },
  hind: {
    at: [0.58, 0.2, -2.0],
    sections: [
      [0, 0, 0.5, 0.82], // the thigh
      [-0.6, 0.15, 0.42, 0.68],
      [-1.1, 0.3, 0.27, 0.32], // the knee
      [-1.6, 0, 0.21, 0.25],
      [-2.05, -0.32, 0.18, 0.22], // the hock
      [-2.55, -0.2, 0.17, 0.2],
      [-2.78, -0.05, 0.23, 0.23], // the ankle
      [-2.9, 0.15, 0.31, 0.19],
      [-2.95, 0.42, 0.31, 0.14], // the paw
    ],
    start: 'flat',
    end: [-2.97, 0.62],
    middle: 4,
    ankle: 6,
    bend: -1,
    paint: (band, c) => (band < 6 ? striped(c.y * 5, 'orange') : 'orange'),
  },
};
