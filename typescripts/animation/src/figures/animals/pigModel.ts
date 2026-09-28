import * as THREE from 'three';
import { decal, napeOf, stretchedHead, type Face } from './animalPark';
import type { Dress, Point } from './Head';
import type { Model } from './Quadruped';

// The pig as the user modelled it: a low poly pig in three.js, like folded
// paper, from their animal park (the shape reference in Pig.md, 2026-09-26).
// Its numbers are the user's, in the model's own units; `unit` makes it
// 70 cm to the top of its shoulders (2.79 units), a farm pig's height, so it
// is 1.6 m from its snout to its curly tail.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, a round barrel, a loft of six-sided rings from the rump, with
//   a point behind, to where the head joins it: pink, darker underneath;
// - the head, the park's (animalPark.ts) with a short, wide snout, its flat
//   end darker pink with two nostrils, small dark eyes, and big floppy ears
//   flopping forward, darker inside;
// - the legs, short lofts with flat hooves, the left ones the same as the
//   right;
// - the tail, little and curly.
//
// A leg bends at two of its rings (Leg.ts): a front leg at the wrist and at
// the fetlock, above the hoof; a hind leg at the knee, pointing forward, and
// at the fetlock, its hock keeping the model's bend. Bent at the hock, its
// short shank folded up flat and the hock went into the ground as the body
// crouched.

const SNOUT = 0.55; // how long the snout is (the wolf's is 1.0)

const BODY: Model['body'] = {
  // A round barrel.
  sections: [
    [0.5, -2.0, 0.55, 0.55],
    [0.55, -1.6, 1.05, 1.0],
    [0.55, -0.5, 1.2, 1.12],
    [0.5, 0.6, 1.15, 1.1],
    [0.55, 1.4, 0.95, 0.95], // the shoulders
    [0.6, 1.9, 0.72, 0.72], // the neck
  ],
  start: [0.5, -2.2],
  end: 'flat',
  paint: (_band, c, hub) => (c.y < hub.y - 0.75 ? 'darkPink' : 'pink'),
};

const HEAD_AT: Point = [0, 0.49, 2.69];
const HEAD_SCALE = 0.75;

// Two nostrils on the flat end of the snout, in the head's units; the end
// sits at z = 1 + 1.45 × SNOUT.
function nostrils(dress: Dress): THREE.Object3D {
  const z = 1 + 1.45 * SNOUT + 0.04;
  const faces: Face[] = [];
  for (const x of [0.13, -0.13]) faces.push({ corners: [[x - 0.07, -0.27, z], [x + 0.07, -0.27, z], [x, -0.42, z + 0.01]], color: 'nostril' });
  return decal(dress, faces, 'nostrils');
}

export const PIG_MODEL: Model = {
  unit: 0.251,
  body: BODY,
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    ...stretchedHead(
      {
        w: 1.0,
        h: 0.95,
        snout: SNOUT,
        snoutW: 1.6,
        snoutDrop: -0.2,
        ear: { scale: 1.3, tip: [1.15, 1.5, 0.85] }, // big floppy ears flopping forward
        colors: { top: 'pink', nose: 'snout', chin: 'pink', eye: 'eye', earIn: 'darkPink' },
      },
      napeOf(BODY, HEAD_AT, HEAD_SCALE),
    ),
    extras: [nostrils],
  },
  // A little curly tail.
  tail: {
    at: [0, 0.9, -2.05],
    sections: [
      [0, 0, 0.06, 0.06],
      [0.15, -0.2, 0.05, 0.05],
      [0.35, -0.15, 0.05, 0.05],
      [0.4, 0, 0.045, 0.045],
      [0.28, 0.08, 0.04, 0.04],
      [0.2, -0.02, 0.035, 0.035],
    ],
    start: 'flat',
    end: [0.22, -0.1],
    paint: () => 'pink',
  },
  front: {
    at: [0.6, 0.1, 1.15],
    sections: [
      [0, 0, 0.3, 0.36],
      [-0.6, 0, 0.22, 0.26], // the wrist
      [-1.0, 0.02, 0.18, 0.2],
      [-1.2, 0.05, 0.19, 0.19], // the fetlock
      [-1.35, 0.08, 0.2, 0.16], // the hoof
    ],
    start: 'flat',
    end: 'flat',
    middle: 1,
    ankle: 3,
    bend: 1,
    paint: (band) => (band >= 3 ? 'hoof' : 'pink'),
  },
  hind: {
    at: [0.62, 0.1, -1.25],
    sections: [
      [0, 0, 0.4, 0.55], // the ham
      [-0.5, 0.08, 0.28, 0.35], // the knee
      [-0.9, -0.05, 0.19, 0.21], // the hock
      [-1.2, 0, 0.19, 0.19], // the fetlock
      [-1.35, 0.04, 0.2, 0.16], // the hoof
    ],
    start: 'flat',
    end: 'flat',
    middle: 1,
    ankle: 3,
    bend: 1,
    paint: (band) => (band >= 3 ? 'hoof' : 'pink'),
  },
};
