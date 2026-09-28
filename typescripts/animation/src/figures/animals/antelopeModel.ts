import * as THREE from 'three';
import { loftFaces, napeOf, pair, solid, stretchedHead } from './animalPark';
import type { Dress, Point } from './Head';
import type { Loft, LoftSection } from './parts';
import type { Model } from './Quadruped';

// The antelope as the user modelled it: a low poly gazelle in three.js, like
// folded paper, from their animal park (the shape reference in Antelope.md,
// 2026-09-26). Its numbers are the user's, in the model's own units; `unit`
// makes it 80 cm to the top of its shoulders (4.77 units), a gazelle's
// height, so it is 1.39 m from its nose to its tail and 1.51 m to the tips of
// its horns.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, slim, a loft of six-sided rings from the rump, with a point
//   behind, up through the chest and a long neck rising at the front to where
//   the head joins it: tan, a white belly and throat, and a dark stripe along
//   each side above the belly;
// - the head, the park's (animalPark.ts) made narrow and long: tan, a white
//   brow, cheeks and chin, dark down the face from the eyes, a black nose and
//   eyes, and long ears, white inside; and long ringed horns sweeping up and
//   back;
// - the legs, long and thin lofts with black hooves, the left ones the same
//   as the right;
// - the tail, short and black.
//
// A leg bends at two of its rings (Leg.ts), as the deer's do: a front leg at
// the wrist (its "knee") and at the fetlock, above the hoof; a hind leg at
// the hock, its backward point, and at the fetlock, its knee keeping the
// model's bend.

const BODY: Model['body'] = {
  sections: [
    [0.35, -2.2, 0.4, 0.4],
    [0.4, -1.75, 0.72, 0.78],
    [0.45, -0.65, 0.66, 0.66],
    [0.35, 0.45, 0.72, 0.85], // the chest
    [0.5, 1.45, 0.62, 0.8],
    [1.2, 2.1, 0.42, 0.46],
    [2.0, 2.5, 0.33, 0.37],
    [2.6, 2.75, 0.3, 0.34], // the neck
  ],
  start: [0.4, -2.45],
  end: 'flat',
  paint: (band, c, hub) => {
    const d = c.y - hub.y;
    if (band >= 1 && band <= 3 && d < -0.45) return 'white'; // belly
    if (band >= 1 && band <= 3 && d < -0.22) return 'dark'; // dark side stripe
    if (band >= 5 && c.z > hub.z + 0.15) return 'white'; // throat
    return 'tan';
  },
};

const HEAD_AT: Point = [0, 2.53, 3.5];
const HEAD_SCALE = 0.72;

// Long ringed horns sweeping up and back, in the head's units.
const HORN_SECTIONS: LoftSection[] = [];
for (let j = 0; j <= 8; j++) {
  const u = j / 8;
  HORN_SECTIONS.push([u * 2.4, -0.7 * Math.sin(u * 2.2), 0.12 - u * 0.08, 0.12 - u * 0.08]);
}
const HORN: Loft = { sections: HORN_SECTIONS, start: 'flat', end: [2.65, -0.45] };
const HORNS = pair(loftFaces(HORN, (band) => (band % 2 ? 'ring' : 'horn')), [0.28, 1.45, 0.15], [0, 0, -0.18]);

function horns(dress: Dress): THREE.Object3D {
  return solid(dress, HORNS, 'horns');
}

export const ANTELOPE_MODEL: Model = {
  unit: 0.168,
  body: BODY,
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    ...stretchedHead(
      {
        w: 0.72,
        h: 0.8,
        snout: 1.1,
        snoutW: 0.8,
        snoutDrop: -0.1,
        ear: { scale: 1.1, tip: [1.25, 2.0, -0.55] },
        // The page paints the eyes black; they are glossy, as every animal's are.
        colors: { top: 'tan', brow: 'white', mask: 'dark', cheekLow: 'white', muzzle: 'tan', chin: 'white', throat: 'white', nose: 'black', eye: 'eye', earIn: 'white' },
      },
      napeOf(BODY, HEAD_AT, HEAD_SCALE),
    ),
    extras: [horns],
    top: Math.max(...HORNS.flatMap((f) => f.corners.map((p) => p[1]))), // the horns reach above the ears
  },
  tail: {
    at: [0, 0.8, -2.25],
    sections: [
      [0, 0, 0.1, 0.08],
      [-0.15, -0.3, 0.09, 0.07],
      [-0.35, -0.5, 0.06, 0.05],
    ],
    start: 'flat',
    end: [-0.5, -0.6],
    paint: () => 'black',
  },
  front: {
    at: [0.38, 0.2, 1.15],
    sections: [
      [0, 0, 0.28, 0.44],
      [-0.8, -0.05, 0.2, 0.3],
      [-1.55, 0.05, 0.12, 0.15], // the "knee" (really the wrist)
      [-1.9, 0.05, 0.1, 0.12],
      [-3.1, 0.1, 0.075, 0.09],
      [-3.35, 0.12, 0.1, 0.1], // the fetlock
      [-3.6, 0.18, 0.11, 0.12],
      [-3.72, 0.24, 0.12, 0.08], // the hoof
    ],
    start: 'flat',
    end: [-3.75, 0.3],
    middle: 2,
    ankle: 5,
    bend: 1,
    paint: (band) => (band >= 6 ? 'black' : 'tan'),
  },
  hind: {
    at: [0.4, 0.2, -1.65],
    sections: [
      [0, 0, 0.36, 0.62], // the thigh
      [-0.6, 0.13, 0.3, 0.48],
      [-1.1, 0.18, 0.17, 0.22], // the knee
      [-1.6, -0.15, 0.13, 0.17],
      [-2.05, -0.42, 0.1, 0.14], // the hock
      [-3.0, -0.3, 0.075, 0.09],
      [-3.3, -0.2, 0.1, 0.1], // the fetlock
      [-3.55, -0.12, 0.11, 0.12],
      [-3.72, -0.06, 0.12, 0.08], // the hoof
    ],
    start: 'flat',
    end: [-3.75, 0],
    middle: 4,
    ankle: 6,
    bend: -1,
    paint: (band) => (band >= 7 ? 'black' : 'tan'),
  },
};
