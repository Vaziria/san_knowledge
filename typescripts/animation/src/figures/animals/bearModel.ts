import type { Model } from './Quadruped';

// The bear as the user modelled it: a low poly brown bear in three.js, like
// folded paper, the shape reference in Bear.md (2026-09-26, after the wolf's
// "use this shape for the wolf"). Its numbers are the user's, in the model's
// own units; `unit` makes it 1.2 m to the top of its shoulder hump (4.66
// units), so it is 2.2 m from its nose to the tip of its tail and 1.38 m to
// the tips of its ears.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, big and heavy, a loft of six-sided rings from the rump, with a
//   point behind, over the shoulder hump and up the neck to where the head
//   joins it, darker low on its sides;
// - the head, rounder and wider than the wolf's, flat faces round its right
//   half mirrored to the left: a short, broad tan muzzle, a big black nose,
//   small eyes, and small round ears, tan inside;
// - the legs, thick lofts with big flat paws, darker from the forearm and the
//   heel down, the left ones the same as the right;
// - the tail, a tiny stub.
//
// A leg bends at two of its rings (Leg.ts), as a bear's does: a front leg at
// the elbow, pointing back, and at the wrist, where the paw begins; a hind
// leg at the knee, pointing forward, and at the heel, so it walks on its
// whole sole.
//
// Beyond the model, as for the wolf, the ears' faces are turned to face out
// and their bases closed, and the back of the head is closed to the nape,
// where the neck ends, which the head turns about.

const HEAD_POINTS = {
  C_back: [0, 1.5, -1.1],
  C_forehead: [0, 1.65, 0.2],
  C_brow: [0, 0.75, 1],
  C_stop: [0, 0.45, 1.25],
  C_snoutTip: [0, 0.15, 2],
  C_noseBot: [0, -0.3, 2.1],
  C_chin: [0, -0.75, 1.35],
  C_throat: [0, -0.9, 0.2],
  topPlate: [0.55, 1.6, 0.2],
  temple: [0.95, 1.2, 0.35],
  brow: [0.45, 0.75, 0.95],
  browOuter: [0.9, 0.85, 0.7],
  eyeIn: [0.42, 0.52, 1],
  eyeOut: [0.72, 0.56, 0.85],
  eyeLow: [0.55, 0.36, 0.98],
  eyeDeep: [0.57, 0.5, 0.75],
  cheek: [1.35, 0.5, 0],
  cheekLow: [1.2, -0.4, 0.3],
  jaw: [0.62, -0.6, 1.05],
  back: [1, 1.05, -0.95],
  backLow: [1.2, -0.2, -0.6],
  snoutBase: [0.45, 0.4, 1.25],
  snoutTip: [0.35, 0.1, 1.95],
  noseBot: [0.3, -0.3, 2.05],
  snoutBot: [0.4, -0.62, 1.4],
  earFront: [0.75, 1.55, 0.15],
  earOut: [1.2, 1.3, 0.05],
  earBack: [0.95, 1.45, -0.35],
  earTip: [1.15, 2, -0.05],
  earInner: [1, 1.6, -0.02],
  C_nape: [0, 0.164706, -1.035294], // beyond the model: where the neck ends, the back of the head's middle
} as const;

export const BEAR_MODEL: Model<keyof typeof HEAD_POINTS> = {
  unit: 0.2575,
  body: {
    sections: [
      [0.3, -2.5, 0.6, 0.6],
      [0.35, -2, 1.25, 1.2], // the rump
      [0.4, -0.8, 1.2, 1.15],
      [0.45, 0.4, 1.3, 1.3],
      [0.7, 1.4, 1.25, 1.4], // the shoulder hump
      [0.85, 2.3, 0.85, 0.9],
      [1.2, 2.75, 0.72, 0.75], // the neck
    ],
    start: [0.3, -2.75],
    end: 'flat',
    paint: (_band, c, hub) => (c.y < hub.y - 0.7 ? 'dark' : 'brown'), // darker low on its sides
  },
  head: {
    at: [0, 1.06, 3.63],
    scale: 0.85,
    points: HEAD_POINTS,
    // The model's faces in its colours: brown fur, a tan muzzle and ear insides, a black nose, and the eyes (glossy).
    side: [
      ['brown', 'C_back', 'C_forehead', 'topPlate'],
      ['brown', 'C_back', 'topPlate', 'back'],
      ['brown', 'C_forehead', 'brow', 'topPlate'],
      ['brown', 'C_forehead', 'C_brow', 'brow'],
      ['brown', 'topPlate', 'brow', 'temple'],
      ['brown', 'temple', 'brow', 'browOuter'],
      ['brown', 'topPlate', 'temple', 'back'],
      ['brown', 'back', 'temple', 'cheek'],
      ['brown', 'temple', 'browOuter', 'cheek'],
      ['brown', 'brow', 'eyeOut', 'browOuter'],
      ['brown', 'brow', 'eyeIn', 'eyeOut'],
      ['brown', 'C_brow', 'eyeIn', 'brow'],
      ['brown', 'browOuter', 'eyeOut', 'cheek'],
      ['brown', 'eyeOut', 'cheekLow', 'cheek'],
      ['brown', 'eyeOut', 'eyeLow', 'cheekLow'],
      ['brown', 'eyeIn', 'snoutBase', 'eyeLow'],
      ['eye', 'eyeIn', 'eyeLow', 'eyeDeep'],
      ['eye', 'eyeLow', 'eyeOut', 'eyeDeep'],
      ['eye', 'eyeOut', 'eyeIn', 'eyeDeep'],
      ['brown', 'C_brow', 'snoutBase', 'eyeIn'],
      ['brown', 'C_brow', 'C_stop', 'snoutBase'],
      ['brown', 'eyeLow', 'jaw', 'cheekLow'],
      ['tan', 'eyeLow', 'snoutBase', 'jaw'],
      ['tan', 'snoutBase', 'snoutBot', 'jaw'],
      ['tan', 'C_stop', 'snoutTip', 'snoutBase'],
      ['tan', 'C_stop', 'C_snoutTip', 'snoutTip'],
      ['tan', 'snoutBase', 'noseBot', 'snoutBot'],
      ['tan', 'snoutBase', 'snoutTip', 'noseBot'],
      ['black', 'C_snoutTip', 'noseBot', 'snoutTip'],
      ['black', 'C_snoutTip', 'C_noseBot', 'noseBot'],
      ['tan', 'C_noseBot', 'snoutBot', 'noseBot'],
      ['tan', 'C_noseBot', 'C_chin', 'snoutBot'],
      ['tan', 'C_chin', 'jaw', 'snoutBot'],
      ['brown', 'C_chin', 'C_throat', 'jaw'],
      ['brown', 'C_throat', 'backLow', 'jaw'],
      ['brown', 'jaw', 'backLow', 'cheekLow'],
      ['brown', 'cheek', 'cheekLow', 'backLow'],
      ['brown', 'cheek', 'backLow', 'back'],
      // The ear, its corners in the other order from the model's so its
      // faces face out, and its base, closed.
      ['tan', 'earInner', 'earTip', 'earFront'],
      ['tan', 'earOut', 'earTip', 'earInner'],
      ['brown', 'earOut', 'earInner', 'earFront'],
      ['brown', 'earBack', 'earTip', 'earOut'],
      ['brown', 'earFront', 'earTip', 'earBack'],
      ['brown', 'earFront', 'earBack', 'earOut'],
      // The back of the head, closed from its rim to the nape.
      ['brown', 'C_nape', 'C_back', 'back'],
      ['brown', 'C_nape', 'back', 'backLow'],
      ['brown', 'C_nape', 'backLow', 'C_throat'],
    ],
    middle: [],
    mouth: [0, -0.4525, 1.7625], // under the muzzle
    pitch: 0,
  },
  tail: {
    at: [0, 0.75, -2.55],
    sections: [
      [0, 0, 0.2, 0.2],
      [0.05, -0.25, 0.28, 0.26],
      [-0.05, -0.5, 0.22, 0.2],
    ],
    start: 'flat',
    end: [-0.15, -0.65],
    paint: () => 'brown',
  },
  front: {
    at: [0.75, 0.2, 1.3],
    sections: [
      [0, 0, 0.55, 0.75],
      [-0.8, 0, 0.48, 0.6],
      [-1.5, 0.05, 0.4, 0.48], // the elbow
      [-2.1, 0.05, 0.38, 0.44],
      [-2.35, 0.12, 0.42, 0.42], // the wrist
      [-2.5, 0.35, 0.48, 0.3],
      [-2.55, 0.65, 0.5, 0.22],
    ],
    start: 'flat',
    end: [-2.58, 0.9],
    middle: 2,
    ankle: 4,
    bend: -1,
    paint: (band) => (band >= 3 ? 'dark' : 'brown'),
  },
  hind: {
    at: [0.75, 0.2, -1.9],
    sections: [
      [0, 0, 0.65, 0.95], // the thigh
      [-0.6, 0.1, 0.58, 0.8],
      [-1.2, 0.15, 0.45, 0.55], // the knee
      [-1.7, -0.05, 0.38, 0.44],
      [-2.1, -0.1, 0.36, 0.42], // the heel (much less bent than a wolf's)
      [-2.38, 0, 0.42, 0.4],
      [-2.5, 0.3, 0.48, 0.3],
      [-2.55, 0.65, 0.5, 0.22], // the paw
    ],
    start: 'flat',
    end: [-2.58, 0.92],
    middle: 2,
    ankle: 4,
    bend: 1,
    paint: (band) => (band >= 4 ? 'dark' : 'brown'),
  },
};
