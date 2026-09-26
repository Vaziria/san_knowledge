import type { Model } from './Quadruped';

// The fox as the user modelled it: a low poly red fox in three.js, like
// folded paper, the shape reference in Fox.md (2026-09-26). Its numbers are
// the user's, in the model's own units; `unit` makes it 38 cm to the top of
// its shoulders (3.71 units), as tall as the fox was, so it is 1.2 m from its
// nose to the tip of its tail and 62 cm to the tips of its ears.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, slimmer than the wolf's, a loft of six-sided rings from the
//   rump, with a point behind, up through the chest and neck to where the
//   head joins it: orange, with a white chest and belly;
// - the head, flat faces round its right half mirrored to the left: a longer,
//   thinner snout than the wolf's, bigger ears, white inside and black
//   behind, slanted eyes, a black nose and a white lower face and cheek ruff;
// - the legs, thinner lofts in black "socks", the left ones the same as the
//   right;
// - the tail, a big bushy brush with a white tip.
//
// A leg bends at two of its rings (Leg.ts): a front leg at the wrist, where
// its thick top narrows, and at the ankle, where the paw begins; a hind leg
// at the hock and the ankle, its knee keeping the model's bend.
//
// Beyond the model, as for the wolf, the ears' faces are turned to face out
// and their bases closed, and the back of the head is closed to the nape,
// where the neck ends, which the head turns about.

const HEAD_POINTS = {
  C_back: [0, 1.35, -1.1],
  C_forehead: [0, 1.6, 0.05],
  C_brow: [0, 0.55, 0.85],
  C_stop: [0, 0.35, 1.05],
  C_snoutTip: [0, 0.05, 2.65],
  C_noseBot: [0, -0.3, 2.72],
  C_chin: [0, -0.62, 1.3],
  C_throat: [0, -0.75, 0.2],
  topPlate: [0.42, 1.55, 0.1],
  temple: [0.75, 1.15, 0.3],
  brow: [0.38, 0.55, 0.8],
  browOuter: [0.8, 0.75, 0.55],
  eyeIn: [0.36, 0.33, 0.9],
  eyeOut: [0.76, 0.44, 0.62],
  eyeLow: [0.5, 0.12, 0.8],
  eyeDeep: [0.55, 0.32, 0.5],
  cheek: [1.25, 0.5, 0],
  cheekLow: [1.2, -0.35, 0.1],
  jaw: [0.5, -0.45, 1],
  back: [0.85, 0.95, -0.95],
  backLow: [1, -0.2, -0.6],
  snoutBase: [0.28, 0.28, 1.1],
  snoutTip: [0.17, 0, 2.6],
  noseBot: [0.15, -0.28, 2.66],
  snoutBot: [0.24, -0.5, 1.35],
  earFront: [0.42, 1.5, 0.15],
  earOut: [1.2, 0.95, 0.05],
  earBack: [0.7, 1.35, -0.55],
  earTip: [1.15, 2.95, -0.25],
  earInner: [0.88, 1.5, -0.18],
  C_nape: [0, 0.253333, -1.013333], // beyond the model: where the neck ends, the back of the head's middle
} as const;

export const FOX_MODEL: Model<keyof typeof HEAD_POINTS> = {
  unit: 0.1023,
  body: {
    sections: [
      [0.3, -2.4, 0.45, 0.45],
      [0.35, -1.9, 0.8, 0.8],
      [0.45, -0.8, 0.68, 0.62],
      [0.3, 0.5, 0.82, 0.92],
      [0.4, 1.6, 0.78, 0.88],
      [0.95, 2.35, 0.5, 0.54],
      [1.55, 2.7, 0.42, 0.46],
    ],
    start: [0.3, -2.65],
    end: 'flat',
    paint: (_band, c, hub) => (c.y < hub.y - 0.3 && c.z > 0 ? 'white' : 'orange'), // a white chest and belly
  },
  head: {
    at: [0, 1.36, 3.46],
    scale: 0.75,
    points: HEAD_POINTS,
    // The model's faces in its colours: orange fur, a white lower face, cheek ruff and ear insides, black ear backs and nose, and the eyes (glossy).
    side: [
      ['orange', 'C_back', 'C_forehead', 'topPlate'],
      ['orange', 'C_back', 'topPlate', 'back'],
      ['orange', 'C_forehead', 'brow', 'topPlate'],
      ['orange', 'C_forehead', 'C_brow', 'brow'],
      ['orange', 'topPlate', 'brow', 'temple'],
      ['orange', 'temple', 'brow', 'browOuter'],
      ['orange', 'topPlate', 'temple', 'back'],
      ['orange', 'back', 'temple', 'cheek'],
      ['orange', 'temple', 'browOuter', 'cheek'],
      ['orange', 'brow', 'eyeOut', 'browOuter'],
      ['orange', 'brow', 'eyeIn', 'eyeOut'],
      ['orange', 'C_brow', 'eyeIn', 'brow'],
      ['orange', 'browOuter', 'eyeOut', 'cheek'],
      ['orange', 'eyeOut', 'cheekLow', 'cheek'],
      ['white', 'eyeOut', 'eyeLow', 'cheekLow'],
      ['orange', 'eyeIn', 'snoutBase', 'eyeLow'],
      ['eye', 'eyeIn', 'eyeLow', 'eyeDeep'],
      ['eye', 'eyeLow', 'eyeOut', 'eyeDeep'],
      ['eye', 'eyeOut', 'eyeIn', 'eyeDeep'],
      ['orange', 'C_brow', 'snoutBase', 'eyeIn'],
      ['orange', 'C_brow', 'C_stop', 'snoutBase'],
      ['white', 'eyeLow', 'jaw', 'cheekLow'],
      ['white', 'eyeLow', 'snoutBase', 'jaw'],
      ['white', 'snoutBase', 'snoutBot', 'jaw'],
      ['orange', 'C_stop', 'snoutTip', 'snoutBase'],
      ['orange', 'C_stop', 'C_snoutTip', 'snoutTip'],
      ['white', 'snoutBase', 'noseBot', 'snoutBot'],
      ['orange', 'snoutBase', 'snoutTip', 'noseBot'],
      ['black', 'C_snoutTip', 'noseBot', 'snoutTip'],
      ['black', 'C_snoutTip', 'C_noseBot', 'noseBot'],
      ['white', 'C_noseBot', 'snoutBot', 'noseBot'],
      ['white', 'C_noseBot', 'C_chin', 'snoutBot'],
      ['white', 'C_chin', 'jaw', 'snoutBot'],
      ['white', 'C_chin', 'C_throat', 'jaw'],
      ['white', 'C_throat', 'backLow', 'jaw'],
      ['white', 'jaw', 'backLow', 'cheekLow'],
      ['white', 'cheek', 'cheekLow', 'backLow'],
      ['orange', 'cheek', 'backLow', 'back'],
      // The ear, its corners in the other order from the model's so its
      // faces face out, and its base, closed.
      ['white', 'earInner', 'earTip', 'earFront'],
      ['white', 'earOut', 'earTip', 'earInner'],
      ['orange', 'earOut', 'earInner', 'earFront'],
      ['black', 'earBack', 'earTip', 'earOut'],
      ['black', 'earFront', 'earTip', 'earBack'],
      ['orange', 'earFront', 'earBack', 'earOut'],
      // The back of the head, closed from its rim to the nape.
      ['orange', 'C_nape', 'C_back', 'back'],
      ['orange', 'C_nape', 'back', 'backLow'],
      ['orange', 'C_nape', 'backLow', 'C_throat'],
    ],
    middle: [],
    mouth: [0, -0.394, 2.081], // under the muzzle
    pitch: 0,
  },
  tail: {
    at: [0, 0.65, -2.35],
    sections: [
      [0, 0, 0.16, 0.16],
      [0.05, -0.5, 0.32, 0.3],
      [-0.05, -1.1, 0.5, 0.46],
      [-0.25, -1.8, 0.6, 0.55],
      [-0.55, -2.5, 0.6, 0.55],
      [-0.85, -3.1, 0.48, 0.44],
      [-1.1, -3.55, 0.3, 0.28],
    ],
    start: 'flat',
    end: [-1.25, -3.85],
    paint: (band) => (band >= 5 ? 'white' : 'orange'), // a white tip
  },
  front: {
    at: [0.52, 0.25, 1.25],
    sections: [
      [0, 0, 0.32, 0.46],
      [-0.8, -0.05, 0.25, 0.35],
      [-1.4, 0.05, 0.16, 0.19], // the wrist
      [-2.15, 0.05, 0.14, 0.16],
      [-2.38, 0.1, 0.17, 0.18], // the ankle
      [-2.52, 0.25, 0.25, 0.17],
      [-2.6, 0.48, 0.26, 0.13], // the paw
    ],
    start: 'flat',
    end: [-2.62, 0.66],
    middle: 2,
    ankle: 4,
    bend: 1,
    paint: (band) => (band >= 2 ? 'black' : 'orange'), // black socks
  },
  hind: {
    at: [0.52, 0.25, -1.75],
    sections: [
      [0, 0, 0.4, 0.66], // the thigh
      [-0.47, 0.13, 0.35, 0.54],
      [-0.9, 0.26, 0.22, 0.29], // the knee
      [-1.32, 0, 0.17, 0.2],
      [-1.66, -0.26, 0.15, 0.19], // the hock
      [-2.13, -0.15, 0.13, 0.15],
      [-2.38, -0.04, 0.17, 0.18], // the ankle
      [-2.52, 0.13, 0.25, 0.18],
      [-2.6, 0.37, 0.27, 0.14], // the paw
    ],
    start: 'flat',
    end: [-2.62, 0.58],
    middle: 4,
    ankle: 6,
    bend: -1,
    paint: (band) => (band >= 4 ? 'black' : 'orange'),
  },
};
