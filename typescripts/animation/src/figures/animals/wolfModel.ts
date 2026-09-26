import type { Model } from './Quadruped';

// The wolf as the user modelled it: a low poly wolf in three.js, like folded
// paper, given on 2026-09-26 ("use this shape for the wolf"). Its numbers are
// the user's, in the model's own units; `unit` makes it 78 cm to the top of
// its shoulders (4.39 units), as tall as the wolf was, so it is 2 m from its
// nose to the tip of its tail and 1.14 m to the tips of its ears.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, a loft of six-sided rings from the rump, with a point behind,
//   up through the chest and neck to where the head joins it;
// - the head, flat faces round its right half, mirrored to the left, with
//   tall ears whose insides are darker, dark eyes set in pockets, and a
//   long muzzle ending in the dark nose;
// - the legs, lofts from the joint down to a paw pointing forward, the left
//   ones the same as the right: a front leg almost straight, a hind leg with
//   the knee bent forward and the hock back;
// - the tail, a loft hanging from the rump in a curve, fullest in the
//   middle, its last band and its point in the tip's colour.
//
// A leg bends at two of its rings (Leg.ts): a front leg at the wrist, where
// its thick top narrows, and at the ankle, where the paw begins; a hind leg
// at the hock and the ankle, its knee keeping the model's bend. Beyond the
// model, the head is closed: the model drew it from both sides, open at the
// back round the neck, with the ears' faces facing in. Here the ears' faces
// are turned to face out, their bases are closed, and the back of the head
// is three faces a side from its rim to the nape, where the neck ends, which
// the head turns about.

const HEAD_POINTS = {
  C_back: [0, 1.45, -1.2],
  C_forehead: [0, 1.7, 0.05],
  C_brow: [0, 0.55, 0.9],
  C_stop: [0, 0.35, 1.1],
  C_snoutTip: [0, 0.12, 2.4],
  C_noseBot: [0, -0.42, 2.5],
  C_chin: [0, -0.72, 1.2],
  C_throat: [0, -0.7, 0.25],
  C_nape: [0, 0.3125, -1.05], // beyond the model: where the neck ends, the back of the head's middle
  topPlate: [0.45, 1.62, 0.1],
  temple: [0.78, 1.2, 0.3],
  brow: [0.4, 0.55, 0.85],
  browOuter: [0.82, 0.78, 0.6],
  eyeIn: [0.38, 0.32, 0.93],
  eyeOut: [0.74, 0.38, 0.68],
  eyeLow: [0.52, 0.1, 0.84],
  eyeDeep: [0.56, 0.3, 0.55],
  cheek: [1.22, 0.6, 0.0],
  cheekLow: [1.0, -0.2, 0.3],
  jaw: [0.6, -0.45, 0.9],
  back: [0.9, 1.0, -1.0],
  backLow: [1.1, -0.1, -0.7],
  snoutBase: [0.3, 0.3, 1.15],
  snoutTip: [0.25, 0.05, 2.35],
  noseBot: [0.22, -0.4, 2.45],
  snoutBot: [0.3, -0.6, 1.3],
  earFront: [0.5, 1.55, 0.15],
  earOut: [1.25, 0.95, 0.05],
  earBack: [0.75, 1.4, -0.55],
  earTip: [1.1, 2.55, -0.2],
  earInner: [0.9, 1.45, -0.15],
} as const;

export const WOLF_MODEL: Model<keyof typeof HEAD_POINTS> = {
  unit: 0.1776,
  body: {
    sections: [
      [0.25, -2.6, 0.5, 0.5],
      [0.25, -2.0, 0.95, 0.95],
      [0.45, -0.8, 0.8, 0.7],
      [0.2, 0.6, 1.0, 1.15],
      [0.35, 1.8, 0.95, 1.1],
      [0.9, 2.6, 0.62, 0.66],
      [1.6, 3.0, 0.5, 0.55],
    ],
    start: [0.25, -2.9],
    end: 'flat',
  },
  head: {
    at: [0, 1.35, 3.84],
    scale: 0.8,
    points: HEAD_POINTS,
    // The model's faces, its skin the coat, its mid grey the muzzle colour
    // (the ears' insides) and its dark the nose and eyes.
    side: [
      ['coat', 'C_back', 'C_forehead', 'topPlate'],
      ['coat', 'C_back', 'topPlate', 'back'],
      ['coat', 'C_forehead', 'brow', 'topPlate'],
      ['coat', 'C_forehead', 'C_brow', 'brow'],
      ['coat', 'topPlate', 'brow', 'temple'],
      ['coat', 'temple', 'brow', 'browOuter'],
      ['coat', 'topPlate', 'temple', 'back'],
      ['coat', 'back', 'temple', 'cheek'],
      ['coat', 'temple', 'browOuter', 'cheek'],
      ['coat', 'brow', 'eyeOut', 'browOuter'],
      ['coat', 'brow', 'eyeIn', 'eyeOut'],
      ['coat', 'C_brow', 'eyeIn', 'brow'],
      ['coat', 'browOuter', 'eyeOut', 'cheek'],
      ['coat', 'eyeOut', 'cheekLow', 'cheek'],
      ['coat', 'eyeOut', 'eyeLow', 'cheekLow'],
      ['coat', 'eyeIn', 'snoutBase', 'eyeLow'],
      ['eye', 'eyeIn', 'eyeLow', 'eyeDeep'],
      ['eye', 'eyeLow', 'eyeOut', 'eyeDeep'],
      ['eye', 'eyeOut', 'eyeIn', 'eyeDeep'],
      ['coat', 'C_brow', 'snoutBase', 'eyeIn'],
      ['coat', 'C_brow', 'C_stop', 'snoutBase'],
      ['coat', 'eyeLow', 'jaw', 'cheekLow'],
      ['coat', 'eyeLow', 'snoutBase', 'jaw'],
      ['coat', 'snoutBase', 'snoutBot', 'jaw'],
      ['coat', 'C_stop', 'snoutTip', 'snoutBase'],
      ['coat', 'C_stop', 'C_snoutTip', 'snoutTip'],
      ['coat', 'snoutBase', 'noseBot', 'snoutBot'],
      ['coat', 'snoutBase', 'snoutTip', 'noseBot'],
      ['nose', 'C_snoutTip', 'noseBot', 'snoutTip'],
      ['nose', 'C_snoutTip', 'C_noseBot', 'noseBot'],
      ['coat', 'C_noseBot', 'snoutBot', 'noseBot'],
      ['coat', 'C_noseBot', 'C_chin', 'snoutBot'],
      ['coat', 'C_chin', 'jaw', 'snoutBot'],
      ['coat', 'C_chin', 'C_throat', 'jaw'],
      ['coat', 'C_throat', 'backLow', 'jaw'],
      ['coat', 'jaw', 'backLow', 'cheekLow'],
      ['coat', 'cheek', 'cheekLow', 'backLow'],
      ['coat', 'cheek', 'backLow', 'back'],
      // The ear, its corners in the other order from the model's so its
      // faces face out, and its base, closed.
      ['muzzle', 'earInner', 'earTip', 'earFront'],
      ['muzzle', 'earOut', 'earTip', 'earInner'],
      ['coat', 'earOut', 'earInner', 'earFront'],
      ['coat', 'earBack', 'earTip', 'earOut'],
      ['coat', 'earFront', 'earTip', 'earBack'],
      ['coat', 'earFront', 'earBack', 'earOut'],
      // The back of the head, closed from its rim to the nape.
      ['coat', 'C_nape', 'C_back', 'back'],
      ['coat', 'C_nape', 'back', 'backLow'],
      ['coat', 'C_nape', 'backLow', 'C_throat'],
    ],
    middle: [],
    mouth: [0, -0.5, 1.9], // under the muzzle, 8 cm behind the nose
    pitch: 0,
  },
  tail: {
    at: [0, 0.7, -2.5],
    sections: [
      [0.0, 0.0, 0.16, 0.16],
      [0.2, -0.5, 0.26, 0.25],
      [0.25, -1.0, 0.38, 0.36],
      [0.1, -1.5, 0.47, 0.44],
      [-0.25, -2.0, 0.49, 0.46],
      [-0.7, -2.45, 0.41, 0.38],
      [-1.15, -2.75, 0.26, 0.24],
    ],
    start: 'flat',
    end: [-1.55, -2.9],
    tip: 5,
  },
  front: {
    at: [0.62, 0.2, 1.4],
    sections: [
      [0.0, 0.0, 0.38, 0.55],
      [-0.9, -0.05, 0.3, 0.42],
      [-1.6, 0.05, 0.2, 0.24], // the wrist
      [-2.5, 0.05, 0.17, 0.2],
      [-2.78, 0.12, 0.21, 0.22], // the ankle
      [-2.96, 0.3, 0.31, 0.21],
      [-3.05, 0.58, 0.33, 0.16],
    ],
    start: 'flat',
    end: [-3.08, 0.82],
    middle: 2,
    ankle: 4,
    bend: 1,
  },
  hind: {
    at: [0.6, 0.2, -1.9],
    sections: [
      [0.0, 0.0, 0.45, 0.75], // the thigh
      [-0.55, 0.15, 0.4, 0.62],
      [-1.05, 0.3, 0.26, 0.34], // the knee, bent forward
      [-1.55, 0.0, 0.2, 0.24],
      [-1.95, -0.3, 0.17, 0.22], // the hock, bent back
      [-2.5, -0.18, 0.15, 0.18],
      [-2.8, -0.05, 0.2, 0.21], // the ankle
      [-2.96, 0.15, 0.3, 0.21],
      [-3.05, 0.43, 0.32, 0.16], // the paw
    ],
    start: 'flat',
    end: [-3.08, 0.68],
    middle: 4,
    ankle: 6,
    bend: -1,
  },
};
