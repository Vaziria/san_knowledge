import { napeOf, stretchedHead } from './animalPark';
import type { Point } from './Head';
import type { LoftSection } from './parts';
import type { Model } from './Quadruped';

// The raccoon as the user modelled it: a low poly raccoon in three.js, like
// folded paper, from their animal park (the shape reference in Raccoon.md,
// 2026-09-26). Its numbers are the user's, in the model's own units; `unit`
// makes it 28 cm to the top of its shoulders (2.69 units), a raccoon's
// height, so it is 31 cm to the top of its humped back and 87 cm from its
// nose to the tip of its tail.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, round and humped over the hips, a loft of six-sided rings from
//   the rump, with a point behind, to where the head joins it: grey, lighter
//   underneath;
// - the head, the park's (animalPark.ts) with a short, narrow snout: grey, a
//   black mask round the eyes, down the nose and on the lower cheeks, a white
//   brow and muzzle, light cheeks, black eyes and nose, and small ears, white
//   inside and black behind;
// - the legs, short lofts ending in flat hands and feet, dark from the
//   forearm and the heel down, the left ones the same as the right;
// - the tail, bushy, fuller in its middle, with dark rings.
//
// It walks on its whole sole, as a raccoon does: a front leg bends at the
// elbow, pointing back, and at the wrist, where the hand begins; a hind leg
// at the knee, pointing forward, and at the heel (Leg.ts).
//
// The page stands its hind feet 1.1 cm above the ground (their hips are
// higher on the body); here they stand on it, and its front legs bend a
// little more than the model's to meet them (Quadruped.ts, layout()).

const BODY: Model['body'] = {
  sections: [
    [0.5, -1.9, 0.5, 0.5],
    [0.6, -1.45, 0.95, 0.95], // the hips
    [0.65, -0.5, 1.0, 1.0],
    [0.5, 0.5, 0.92, 0.95],
    [0.45, 1.3, 0.78, 0.82], // the shoulders
    [0.6, 1.85, 0.55, 0.58], // the neck
  ],
  start: [0.5, -2.1],
  end: 'flat',
  paint: (_band, c, hub) => (c.y < hub.y - 0.5 ? 'light' : 'grey'),
};

const HEAD_AT: Point = [0, 0.48, 2.5];
const HEAD_SCALE = 0.62;

// A bushy tail, fuller in its middle, curving down behind.
const TAIL: LoftSection[] = [];
for (let j = 0; j <= 8; j++) {
  const u = j / 8;
  TAIL.push([0.05 * Math.sin(u * 3) - u * 0.55, -u * 2.3, 0.2 + Math.sin(u * Math.PI) * 0.15, 0.2 + Math.sin(u * Math.PI) * 0.15]);
}

export const RACCOON_MODEL: Model = {
  unit: 0.104,
  body: BODY,
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    ...stretchedHead(
      {
        w: 1.0,
        h: 0.9,
        snout: 0.8,
        snoutW: 0.7,
        snoutDrop: -0.05,
        ear: { scale: 0.75, tip: [1.0, 2.0, -0.2] },
        // The page's eyes are all but black ([0.02, 0.02, 0.02]); glossy here, as every animal's are.
        colors: {
          top: 'grey',
          brow: 'white',
          mask: 'dark',
          cheek: 'light',
          cheekLow: 'dark',
          bridge: 'dark',
          snoutTop: 'dark',
          muzzle: 'white',
          chin: 'white',
          throat: 'light',
          nose: 'dark',
          eye: 'eye',
          earIn: 'white',
          earBack: 'dark',
        },
      },
      napeOf(BODY, HEAD_AT, HEAD_SCALE),
    ),
  },
  // Bushy, with dark rings.
  tail: {
    at: [0, 0.75, -1.95],
    sections: TAIL,
    start: 'flat',
    end: [-0.62, -2.5],
    paint: (band) => (band % 2 || band >= 7 ? 'dark' : 'light'),
  },
  front: {
    at: [0.55, 0.2, 1.2],
    sections: [
      [0, 0, 0.3, 0.4],
      [-0.6, 0, 0.22, 0.28], // the elbow
      [-1.1, 0.05, 0.16, 0.18],
      [-1.35, 0.12, 0.18, 0.18], // the wrist
      [-1.5, 0.3, 0.22, 0.12],
      [-1.53, 0.5, 0.22, 0.09], // the flat hand
    ],
    start: 'flat',
    end: [-1.55, 0.62],
    middle: 1,
    ankle: 3,
    bend: -1,
    paint: (band) => (band >= 2 ? 'dark' : 'grey'),
  },
  hind: {
    at: [0.6, 0.3, -1.3],
    sections: [
      [0, 0, 0.38, 0.6], // the thigh
      [-0.5, 0.1, 0.3, 0.45],
      [-0.95, 0.15, 0.2, 0.24], // the knee
      [-1.25, 0, 0.17, 0.19], // the heel
      [-1.45, 0.15, 0.22, 0.14],
      [-1.53, 0.45, 0.22, 0.09], // the flat foot
    ],
    start: 'flat',
    end: [-1.55, 0.62],
    middle: 2,
    ankle: 3,
    bend: 1,
    paint: (band) => (band >= 3 ? 'dark' : 'grey'),
  },
};
