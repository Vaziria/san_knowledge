import * as THREE from 'three';
import { loftFaces, napeOf, pair, solid, stretchedHead } from './animalPark';
import type { Dress, Point } from './Head';
import type { Loft } from './parts';
import type { Model } from './Quadruped';

// The ox as the user modelled it: a low poly ox in three.js, like folded
// paper, from their animal park (the shape reference in Ox.md, 2026-09-26).
// Its numbers are the user's, in the model's own units; `unit` makes it
// 1.5 m to the top of its shoulders (5.16 units), a working ox's height, so
// it is 2.8 m from its nose to the tip of its tail and 1.66 m across its
// horns.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, big and deep, a loft of six-sided rings from the rump, with a
//   point behind, over the shoulders and down the neck to where the head
//   joins it: all brown;
// - the head, the park's (animalPark.ts) made broad with a short, wide snout
//   hanging low: brown, a pale muzzle and chin, a pinkish nose, dark eyes,
//   ears standing out to the sides, pale inside; and long horns going out to
//   the sides and curving up, pale with dark tips;
// - the legs, thick lofts with dark hooves, the left ones the same as the
//   right;
// - the tail, thin and hanging, with a dark tuft.
//
// A leg bends at two of its rings (Leg.ts): a front leg, as the deer's does,
// at the wrist (an ox's "knee") and at the fetlock, above the hoof; a hind
// leg at the knee, pointing forward, and at the fetlock, its hock keeping the
// model's bend. Bent at the hock, as the deer's is, its long thigh stood all
// but upright when the leg folded at a gallop, and the hock went 5 cm into
// the ground.

const BODY: Model['body'] = {
  sections: [
    [0.4, -2.8, 0.6, 0.6],
    [0.5, -2.3, 1.25, 1.25],
    [0.55, -1.0, 1.3, 1.3],
    [0.5, 0.4, 1.4, 1.45],
    [0.8, 1.6, 1.35, 1.5], // the shoulders
    [0.7, 2.5, 1.0, 1.1],
    [0.5, 3.1, 0.8, 0.85], // the neck
  ],
  start: [0.45, -3.1],
  end: 'flat',
  paint: () => 'brown',
};

// Legs brown down to their hooves, from band n on.
const hoof = (n: number) => (band: number) => (band >= n ? 'dark' : 'brown');

const HEAD_AT: Point = [0, 0.3, 4.1];
const HEAD_SCALE = 0.95;

// The horns: out to the side, then curving up (drawn along z, turned to
// point along x).
const L = 1.9; // horn length (1 = the old small horns)
const T = 1.6; // horn thickness
const HORN: Loft = {
  sections: [
    [0.0 * L, 0.0 * L, 0.16 * T, 0.16 * T],
    [0.03 * L, 0.35 * L, 0.14 * T, 0.14 * T],
    [0.1 * L, 0.7 * L, 0.12 * T, 0.12 * T],
    [0.28 * L, 1.0 * L, 0.1 * T, 0.1 * T],
    [0.55 * L, 1.22 * L, 0.07 * T, 0.07 * T],
  ],
  start: 'flat',
  end: [0.85 * L, 1.32 * L],
};
// Both horns on the head, in the head's units: pale, their tips dark.
const HORNS = pair(loftFaces(HORN, (band) => (band >= 3 ? 'hornTip' : 'horn')), [0.5, 1.45, -0.2], [0, Math.PI / 2, 0]);

function horns(dress: Dress): THREE.Object3D {
  return solid(dress, HORNS, 'horns');
}

export const OX_MODEL: Model = {
  unit: 0.291,
  body: BODY,
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    ...stretchedHead(
      {
        w: 1.1,
        h: 1.0,
        snout: 0.75,
        snoutW: 1.6,
        snoutDrop: -0.3,
        ear: { scale: 0.9, tip: [1.75, 1.0, -0.4] },
        // The page paints the eyes dark; they are glossy, as every animal's are.
        colors: { top: 'brown', muzzle: 'muzzle', snoutTop: 'brown', nose: 'nose', chin: 'muzzle', throat: 'brown', eye: 'eye', earIn: 'earIn' },
      },
      napeOf(BODY, HEAD_AT, HEAD_SCALE),
    ),
    extras: [horns],
    top: Math.max(...HORNS.flatMap((f) => f.corners.map((p) => p[1]))), // the horns reach above the ears
  },
  tail: {
    at: [0, 1.3, -2.9],
    sections: [
      [0, 0, 0.12, 0.12],
      [-0.4, -0.3, 0.08, 0.08],
      [-1.2, -0.4, 0.07, 0.07],
      [-2.0, -0.4, 0.06, 0.06],
      [-2.3, -0.38, 0.14, 0.14], // the tuft starts
      [-2.6, -0.35, 0.15, 0.15],
    ],
    start: 'flat',
    end: [-2.85, -0.33],
    paint: (band) => (band >= 3 ? 'dark' : 'brown'),
  },
  front: {
    at: [0.75, 0.2, 1.6],
    sections: [
      [0, 0, 0.45, 0.6],
      [-0.9, -0.05, 0.34, 0.45],
      [-1.7, 0.05, 0.22, 0.26], // the wrist
      [-2.4, 0.05, 0.2, 0.22],
      [-2.65, 0.1, 0.22, 0.23], // the fetlock
      [-2.85, 0.18, 0.25, 0.25],
      [-3.0, 0.22, 0.26, 0.2], // the hoof
    ],
    start: 'flat',
    end: 'flat',
    middle: 2,
    ankle: 4,
    bend: 1,
    paint: hoof(5),
  },
  hind: {
    at: [0.8, 0.2, -2.1],
    sections: [
      [0, 0, 0.55, 0.85], // the thigh
      [-0.7, 0.1, 0.45, 0.65],
      [-1.3, 0.2, 0.28, 0.32], // the knee
      [-1.8, -0.1, 0.22, 0.26],
      [-2.2, -0.3, 0.2, 0.24], // the hock
      [-2.65, -0.2, 0.19, 0.22],
      [-2.85, -0.1, 0.24, 0.25], // the fetlock
      [-3.0, -0.05, 0.26, 0.2], // the hoof
    ],
    start: 'flat',
    end: 'flat',
    middle: 2,
    ankle: 6,
    bend: 1,
    paint: hoof(6),
  },
};
