import { napeOf, stretchedHead } from './animalPark';
import type { Point } from './Head';
import type { Model } from './Quadruped';

// The coyote as the user modelled it: a low poly coyote in three.js, like
// folded paper, from their animal park (the shape reference in Coyote.md,
// 2026-09-26). Its numbers are the user's, in the model's own units; `unit`
// makes it 60 cm to the top of its shoulders (4.26 units), a coyote's
// height, so it is 1.36 m from its nose to the tip of its tail.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, lean, a loft of six-sided rings from the rump, with a point
//   behind, up through the chest and neck to where the head joins it: tawny,
//   greyer along its back, a cream belly;
// - the head, the park's (animalPark.ts) made narrow, with a long, thin snout
//   and tall ears: grey on top, tawny on its sides, a cream muzzle, cheeks and
//   chin, a black nose, yellow eyes, the ears cream inside and tawny behind;
// - the legs, thin lofts with small paws, the left ones the same as the
//   right;
// - the tail, bushy and hanging low, with a black tip.
//
// A leg bends at two of its rings (Leg.ts), as the wolf's do: a front leg at
// the wrist and at the ankle, where the paw begins; a hind leg at the hock
// and the ankle, its knee keeping the model's bend.

const BODY: Model['body'] = {
  sections: [
    [0.25, -2.5, 0.45, 0.45],
    [0.3, -1.95, 0.82, 0.88], // the hips
    [0.45, -0.8, 0.7, 0.72],
    [0.25, 0.55, 0.85, 1.0], // the chest
    [0.35, 1.7, 0.8, 0.98], // the shoulders
    [0.85, 2.5, 0.55, 0.6],
    [1.45, 2.9, 0.45, 0.5], // the neck
  ],
  start: [0.25, -2.75],
  end: 'flat',
  paint: (_band, c, hub) => (c.y < hub.y - 0.5 ? 'cream' : c.y > hub.y + 0.5 ? 'grey' : 'fur'),
};

const HEAD_AT: Point = [0, 1.26, 3.66];
const HEAD_SCALE = 0.72;

export const COYOTE_MODEL: Model = {
  unit: 0.141,
  body: BODY,
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    ...stretchedHead(
      {
        w: 0.85,
        h: 0.9,
        snout: 1.1,
        snoutW: 0.85,
        ear: { scale: 1.25, tip: [1.15, 2.85, -0.25] },
        colors: { top: 'grey', side: 'fur', snoutTop: 'fur', muzzle: 'cream', cheekLow: 'cream', chin: 'cream', nose: 'black', eye: 'eye', earIn: 'cream', earBack: 'fur' },
      },
      napeOf(BODY, HEAD_AT, HEAD_SCALE),
    ),
  },
  // A bushy tail hanging low, with a black tip.
  tail: {
    at: [0, 0.8, -2.55],
    sections: [
      [0, 0, 0.18, 0.18],
      [-0.2, -0.5, 0.3, 0.28],
      [-0.7, -0.95, 0.36, 0.34],
      [-1.3, -1.2, 0.34, 0.32],
      [-1.85, -1.3, 0.25, 0.24],
    ],
    start: 'flat',
    end: [-2.25, -1.3],
    paint: (band) => (band >= 3 ? 'black' : 'fur'),
  },
  front: {
    at: [0.5, 0.2, 1.45],
    sections: [
      [0, 0, 0.32, 0.48],
      [-0.9, -0.05, 0.24, 0.34],
      [-1.6, 0.05, 0.16, 0.19], // the wrist
      [-2.5, 0.05, 0.14, 0.16],
      [-2.78, 0.12, 0.18, 0.19], // the ankle
      [-2.96, 0.3, 0.26, 0.18],
      [-3.05, 0.55, 0.27, 0.13], // the paw
    ],
    start: 'flat',
    end: [-3.08, 0.75],
    middle: 2,
    ankle: 4,
    bend: 1,
    paint: () => 'fur',
  },
  hind: {
    at: [0.5, 0.2, -1.85],
    sections: [
      [0, 0, 0.4, 0.66], // the thigh
      [-0.55, 0.15, 0.35, 0.55],
      [-1.05, 0.3, 0.22, 0.28], // the knee
      [-1.55, 0, 0.17, 0.2],
      [-1.95, -0.3, 0.14, 0.18], // the hock
      [-2.5, -0.18, 0.13, 0.15],
      [-2.8, -0.05, 0.18, 0.19], // the ankle
      [-2.96, 0.15, 0.26, 0.18],
      [-3.05, 0.43, 0.27, 0.13], // the paw
    ],
    start: 'flat',
    end: [-3.08, 0.65],
    middle: 4,
    ankle: 6,
    bend: -1,
    paint: () => 'fur',
  },
};
