import { whiskers } from './Head';
import type { Model } from './Quadruped';

// The cat as the user modelled it: a low poly grey tabby in three.js, like
// folded paper, the shape reference in Cat.md (2026-09-26). Its numbers are
// the user's, in the model's own units; `unit` makes it 25 cm to the top of
// its shoulders (3.83 units), as tall as the cat was, so it is 52 cm from its
// nose to the back of its tail and 41 cm to the top of its tail.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, slim, a loft of six-sided rings from the rump, with a point
//   behind, up through the chest and neck to where the head joins it: grey,
//   tabby stripes across the back and sides, a white chest;
// - the head, flat faces round its right half mirrored to the left: a round
//   face, a short white muzzle, big forward-facing green eyes, a small pink
//   nose, tall pointed ears pink inside, the tabby's "M" stripes on its
//   forehead and stripes on its cheeks; and three whiskers a side;
// - the legs, lofts with stripes and white paws ("socks"), the left ones the
//   same as the right;
// - the tail, long, rising and curling forward, ringed with stripes.
//
// A leg bends at two of its rings (Leg.ts): a front leg at the wrist, where
// its thick top narrows, and at the ankle, where the paw begins; a hind leg
// at the hock and the ankle, its knee keeping the model's bend.
//
// Beyond the model, as for the wolf, the ears' faces are turned to face out
// and their bases closed, and the back of the head is closed to the nape,
// where the neck ends, which the head turns about.

// The model's tabby stripes: bands that repeat along one direction.
const stripe = (value: number, width = 0.35) => Math.sin(value) > width;

const HEAD_POINTS = {
  C_back: [0, 1.3, -0.9],
  C_forehead: [0, 1.55, 0.2],
  C_brow: [0, 0.95, 0.95],
  C_stop: [0, 0.6, 1.2],
  C_snoutTip: [0, 0.4, 1.6],
  C_noseBot: [0, 0.1, 1.65],
  C_chin: [0, -0.45, 1.25],
  C_throat: [0, -0.7, 0.2],
  topPlate: [0.5, 1.5, 0.2],
  temple: [0.9, 1.15, 0.3],
  brow: [0.42, 0.95, 0.9],
  browOuter: [0.9, 0.95, 0.6],
  eyeIn: [0.25, 0.72, 1.12],
  eyeOut: [0.75, 0.8, 0.85],
  eyeLow: [0.5, 0.45, 1.05],
  eyeDeep: [0.48, 0.7, 0.85],
  cheek: [1.2, 0.45, 0.1],
  cheekLow: [1, -0.2, 0.5],
  jaw: [0.45, -0.35, 1.05],
  back: [0.85, 0.95, -0.8],
  backLow: [1, -0.2, -0.5],
  snoutBase: [0.3, 0.45, 1.3],
  snoutTip: [0.28, 0.3, 1.58],
  noseBot: [0.2, 0.02, 1.62],
  snoutBot: [0.25, -0.3, 1.3],
  earFront: [0.35, 1.5, 0.35],
  earOut: [1.05, 1.2, 0.25],
  earBack: [0.7, 1.45, -0.3],
  earTip: [0.95, 2.35, 0.05],
  earInner: [0.72, 1.6, 0.15],
  C_nape: [0, 0.121951, -0.841463], // beyond the model: where the neck ends, the back of the head's middle
} as const;

export const CAT_MODEL: Model<keyof typeof HEAD_POINTS> = {
  unit: 0.06534,
  body: {
    sections: [
      [0.35, -2.3, 0.4, 0.4],
      [0.45, -1.85, 0.72, 0.75], // the hips
      [0.5, -0.7, 0.68, 0.7],
      [0.4, 0.5, 0.72, 0.8], // the chest
      [0.5, 1.5, 0.68, 0.78], // the shoulders
      [1, 2.1, 0.48, 0.52],
      [1.45, 2.4, 0.42, 0.45], // the neck
    ],
    start: [0.35, -2.55],
    end: 'flat',
    paint: (_band, c, hub) => (c.y < hub.y - 0.3 && c.z > 0.3 ? 'white' : c.y > hub.y - 0.2 && stripe(c.z * 5) ? 'stripe' : 'grey'),
  },
  head: {
    at: [0, 1.35, 3.09],
    scale: 0.82,
    points: HEAD_POINTS,
    // The model's faces in its colours: grey fur with tabby stripes, a white muzzle and chin, a pink nose and ear insides, and the green eyes (glossy).
    side: [
      ['stripe', 'C_back', 'C_forehead', 'topPlate'],
      ['grey', 'C_back', 'topPlate', 'back'],
      ['grey', 'C_forehead', 'brow', 'topPlate'],
      ['stripe', 'C_forehead', 'C_brow', 'brow'],
      ['stripe', 'topPlate', 'brow', 'temple'],
      ['grey', 'temple', 'brow', 'browOuter'],
      ['grey', 'topPlate', 'temple', 'back'],
      ['stripe', 'back', 'temple', 'cheek'],
      ['grey', 'temple', 'browOuter', 'cheek'],
      ['grey', 'brow', 'eyeOut', 'browOuter'],
      ['grey', 'brow', 'eyeIn', 'eyeOut'],
      ['grey', 'C_brow', 'eyeIn', 'brow'],
      ['stripe', 'browOuter', 'eyeOut', 'cheek'],
      ['grey', 'eyeOut', 'cheekLow', 'cheek'],
      ['grey', 'eyeOut', 'eyeLow', 'cheekLow'],
      ['white', 'eyeIn', 'snoutBase', 'eyeLow'],
      ['eye', 'eyeIn', 'eyeLow', 'eyeDeep'],
      ['eye', 'eyeLow', 'eyeOut', 'eyeDeep'],
      ['eye', 'eyeOut', 'eyeIn', 'eyeDeep'],
      ['grey', 'C_brow', 'snoutBase', 'eyeIn'],
      ['grey', 'C_brow', 'C_stop', 'snoutBase'],
      ['white', 'eyeLow', 'jaw', 'cheekLow'],
      ['white', 'eyeLow', 'snoutBase', 'jaw'],
      ['white', 'snoutBase', 'snoutBot', 'jaw'],
      ['white', 'C_stop', 'snoutTip', 'snoutBase'],
      ['white', 'C_stop', 'C_snoutTip', 'snoutTip'],
      ['white', 'snoutBase', 'noseBot', 'snoutBot'],
      ['white', 'snoutBase', 'snoutTip', 'noseBot'],
      ['pink', 'C_snoutTip', 'noseBot', 'snoutTip'],
      ['pink', 'C_snoutTip', 'C_noseBot', 'noseBot'],
      ['white', 'C_noseBot', 'snoutBot', 'noseBot'],
      ['white', 'C_noseBot', 'C_chin', 'snoutBot'],
      ['white', 'C_chin', 'jaw', 'snoutBot'],
      ['white', 'C_chin', 'C_throat', 'jaw'],
      ['white', 'C_throat', 'backLow', 'jaw'],
      ['grey', 'jaw', 'backLow', 'cheekLow'],
      ['stripe', 'cheek', 'cheekLow', 'backLow'],
      ['grey', 'cheek', 'backLow', 'back'],
      // The ear, its corners in the other order from the model's so its
      // faces face out, and its base, closed.
      ['pink', 'earInner', 'earTip', 'earFront'],
      ['pink', 'earOut', 'earTip', 'earInner'],
      ['grey', 'earOut', 'earInner', 'earFront'],
      ['grey', 'earBack', 'earTip', 'earOut'],
      ['grey', 'earFront', 'earTip', 'earBack'],
      ['grey', 'earFront', 'earBack', 'earOut'],
      // The back of the head, closed from its rim to the nape.
      ['grey', 'C_nape', 'C_back', 'back'],
      ['grey', 'C_nape', 'back', 'backLow'],
      ['grey', 'C_nape', 'backLow', 'C_throat'],
    ],
    middle: [],
    mouth: [0, -0.0975, 1.47], // under the muzzle
    pitch: 0,
    extras: [
      // Three whiskers a side, from beside the nose.
      whiskers(
        [
          [[0.3, 0.28, 1.45], [1.45, 0.45, 1.25]],
          [[0.3, 0.2, 1.45], [1.45, 0.2, 1.3]],
          [[0.3, 0.12, 1.45], [1.45, -0.05, 1.2]],
        ],
        'whisker',
      ),
    ],
  },
  tail: {
    at: [0, 0.7, -2.4],
    sections: [
      [0, 0, 0.14, 0.14],
      [0.3, -0.45, 0.14, 0.14],
      [0.9, -0.8, 0.13, 0.13],
      [1.6, -0.95, 0.13, 0.13],
      [2.3, -0.85, 0.12, 0.12],
      [2.8, -0.55, 0.11, 0.11],
      [3.05, -0.2, 0.1, 0.1],
    ],
    start: 'flat',
    end: [3.1, 0],
    paint: (band) => (band % 2 === 1 || band === 5 ? 'stripe' : 'grey'),
  },
  front: {
    at: [0.42, 0.2, 1.2],
    sections: [
      [0, 0, 0.28, 0.4],
      [-0.8, -0.05, 0.2, 0.28],
      [-1.5, 0.05, 0.13, 0.15], // the wrist
      [-2.2, 0.05, 0.12, 0.13],
      [-2.4, 0.12, 0.16, 0.15], // the ankle
      [-2.5, 0.25, 0.2, 0.13],
      [-2.55, 0.42, 0.19, 0.1], // the small round paw
    ],
    start: 'flat',
    end: [-2.57, 0.55],
    middle: 2,
    ankle: 4,
    bend: 1,
    paint: (band, c) => (band >= 4 ? 'white' : stripe(c.y * 6, 0.5) ? 'stripe' : 'grey'),
  },
  hind: {
    at: [0.45, 0.2, -1.75],
    sections: [
      [0, 0, 0.36, 0.62], // the thigh
      [-0.5, 0.15, 0.3, 0.5],
      [-0.95, 0.28, 0.18, 0.24], // the knee
      [-1.4, 0, 0.14, 0.17],
      [-1.8, -0.3, 0.12, 0.15], // the hock
      [-2.25, -0.18, 0.11, 0.13],
      [-2.45, -0.05, 0.15, 0.15], // the ankle
      [-2.53, 0.12, 0.2, 0.13],
      [-2.57, 0.35, 0.19, 0.1], // the paw
    ],
    start: 'flat',
    end: [-2.59, 0.5],
    middle: 4,
    ankle: 6,
    bend: -1,
    paint: (band, c) => (band >= 6 ? 'white' : stripe(c.y * 6, 0.5) ? 'stripe' : 'grey'),
  },
};
