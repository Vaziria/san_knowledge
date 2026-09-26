import * as THREE from 'three';
import type { Dress, Point } from './Head';
import { loftGeometry, lofted, mesh, type Loft } from './parts';
import type { Model } from './Quadruped';

// The deer as the user modelled it: a low poly buck in three.js, like folded
// paper, the shape reference in Deer.md (2026-09-26). Its numbers are the
// user's, in the model's own units; `unit` makes it 95 cm to the top of its
// shoulders (5.59 units), as tall as the deer was, so it is 1.56 m from its
// nose to its tail and 1.7 m to the tips of its antlers.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, slim, a loft of six-sided rings from the rump, with a point
//   behind, up through the chest and a long neck rising at the front to where
//   the head joins it: a warm tan-brown coat, a white belly and throat;
// - the head, flat faces round its right half mirrored to the left: a long,
//   narrow face, eyes on its sides, a white chin, big ears that stick out
//   sideways, pinkish inside; and its antlers, each a curved main beam and
//   three tines, thin five-sided lofts ending in points, leaning out;
// - the legs, long and thin lofts with black hooves, the left ones the same
//   as the right;
// - the tail, short, white underneath.
//
// A leg bends at two of its rings (Leg.ts): a front leg at the wrist (the
// model's "knee") and at the fetlock, above the hoof; a hind leg at the hock,
// the model's sharp backward point, and at the fetlock, its knee keeping the
// model's bend. The model sets the antlers on the deer; here they sit on the
// head, where the model has them, and turn with it.
//
// Beyond the model, as for the wolf, the ears' faces are turned to face out
// and their bases closed, and the back of the head is closed to the nape,
// where the neck ends, which the head turns about.

// Where the model puts the head, and how big it is against the body.
const HEAD_AT = [0, 3, 3.8] as const;
const HEAD_SCALE = 0.8;

// An antler (the right one), in the deer's units: a curved main beam and
// three tines, each a thin loft that ends in a point.
const ANTLER: readonly Loft[] = [
  // the main beam: up, back, then curving forward
  { sections: [[0, 0, 0.1, 0.1], [0.6, -0.25, 0.09, 0.09], [1.2, -0.3, 0.08, 0.08], [1.7, -0.1, 0.065, 0.065], [2, 0.25, 0.05, 0.05]], start: 'flat', end: [2.15, 0.5], sides: 5 },
  // the brow tine (low, pointing forward)
  { sections: [[0.25, -0.1, 0.06, 0.06], [0.45, 0.2, 0.04, 0.04]], start: 'flat', end: [0.55, 0.4], sides: 5 },
  // the middle tine (straight up)
  { sections: [[0.6, -0.25, 0.06, 0.06], [1.1, 0.05, 0.045, 0.045]], start: 'flat', end: [1.4, 0.2], sides: 5 },
  // the back tine
  { sections: [[1.2, -0.3, 0.055, 0.055], [1.65, -0.35, 0.04, 0.04]], start: 'flat', end: [1.95, -0.3], sides: 5 },
];
const ANTLER_AT: Point = [0.3, 4.05, 3.85]; // where the right antler sits on the deer, in its units; the left one at -x
const ANTLER_LEAN = 0.45; // radians each leans out from upright

const HEAD_POINTS = {
  C_back: [0, 1.3, -1],
  C_forehead: [0, 1.45, 0.15],
  C_brow: [0, 0.9, 1],
  C_stop: [0, 0.7, 1.3],
  C_snoutTip: [0, 0.25, 2.5],
  C_noseBot: [0, -0.2, 2.6],
  C_chin: [0, -0.55, 1.8],
  C_throat: [0, -0.75, 0.2],
  topPlate: [0.4, 1.4, 0.15],
  temple: [0.7, 1.05, 0.25],
  brow: [0.4, 0.85, 0.9],
  browOuter: [0.72, 0.8, 0.55],
  eyeIn: [0.5, 0.58, 0.8],
  eyeOut: [0.78, 0.62, 0.38],
  eyeLow: [0.64, 0.36, 0.62],
  eyeDeep: [0.55, 0.55, 0.45],
  cheek: [0.95, 0.5, -0.1],
  cheekLow: [0.8, -0.25, 0.3],
  jaw: [0.4, -0.45, 1.3],
  back: [0.75, 0.95, -0.9],
  backLow: [0.85, -0.2, -0.6],
  snoutBase: [0.35, 0.6, 1.3],
  snoutTip: [0.25, 0.2, 2.45],
  noseBot: [0.22, -0.18, 2.55],
  snoutBot: [0.28, -0.45, 1.6],
  earFront: [0.55, 1.25, 0],
  earOut: [0.85, 1, -0.15],
  earBack: [0.6, 1.2, -0.5],
  earTip: [1.85, 1.8, -0.45],
  earInner: [1.1, 1.42, -0.2],
  C_nape: [0, 0.125, -0.9375], // beyond the model: where the neck ends, the back of the head's middle
} as const;

export const DEER_MODEL: Model<keyof typeof HEAD_POINTS> = {
  unit: 0.1699,
  body: {
    sections: [
      [0.35, -2.4, 0.45, 0.45],
      [0.4, -1.9, 0.85, 0.9], // the hips
      [0.45, -0.7, 0.8, 0.8],
      [0.35, 0.5, 0.85, 1], // the chest
      [0.5, 1.6, 0.75, 0.95], // the shoulders
      [1.3, 2.3, 0.5, 0.55], // the base of the neck
      [2.3, 2.75, 0.4, 0.45],
      [3.1, 3.05, 0.36, 0.4], // the top of the neck
    ],
    start: [0.4, -2.65],
    end: 'flat',
    paint: (band, c, hub) => (band >= 1 && band <= 3 && c.y < hub.y - 0.6 ? 'white' : band >= 5 && c.z > hub.z + 0.2 ? 'white' : 'fur'), // the belly, the front of the neck
  },
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    points: HEAD_POINTS,
    // The model's faces in its colours: tan-brown fur, a white chin and throat, ears pinkish inside, a black nose, and the eyes (glossy).
    side: [
      ['fur', 'C_back', 'C_forehead', 'topPlate'],
      ['fur', 'C_back', 'topPlate', 'back'],
      ['fur', 'C_forehead', 'brow', 'topPlate'],
      ['fur', 'C_forehead', 'C_brow', 'brow'],
      ['fur', 'topPlate', 'brow', 'temple'],
      ['fur', 'temple', 'brow', 'browOuter'],
      ['fur', 'topPlate', 'temple', 'back'],
      ['fur', 'back', 'temple', 'cheek'],
      ['fur', 'temple', 'browOuter', 'cheek'],
      ['fur', 'brow', 'eyeOut', 'browOuter'],
      ['fur', 'brow', 'eyeIn', 'eyeOut'],
      ['fur', 'C_brow', 'eyeIn', 'brow'],
      ['fur', 'browOuter', 'eyeOut', 'cheek'],
      ['fur', 'eyeOut', 'cheekLow', 'cheek'],
      ['fur', 'eyeOut', 'eyeLow', 'cheekLow'],
      ['fur', 'eyeIn', 'snoutBase', 'eyeLow'],
      ['eye', 'eyeIn', 'eyeLow', 'eyeDeep'],
      ['eye', 'eyeLow', 'eyeOut', 'eyeDeep'],
      ['eye', 'eyeOut', 'eyeIn', 'eyeDeep'],
      ['fur', 'C_brow', 'snoutBase', 'eyeIn'],
      ['fur', 'C_brow', 'C_stop', 'snoutBase'],
      ['fur', 'eyeLow', 'jaw', 'cheekLow'],
      ['fur', 'eyeLow', 'snoutBase', 'jaw'],
      ['fur', 'snoutBase', 'snoutBot', 'jaw'],
      ['fur', 'C_stop', 'snoutTip', 'snoutBase'],
      ['fur', 'C_stop', 'C_snoutTip', 'snoutTip'],
      ['white', 'snoutBase', 'noseBot', 'snoutBot'],
      ['fur', 'snoutBase', 'snoutTip', 'noseBot'],
      ['black', 'C_snoutTip', 'noseBot', 'snoutTip'],
      ['black', 'C_snoutTip', 'C_noseBot', 'noseBot'],
      ['white', 'C_noseBot', 'snoutBot', 'noseBot'],
      ['white', 'C_noseBot', 'C_chin', 'snoutBot'],
      ['white', 'C_chin', 'jaw', 'snoutBot'],
      ['white', 'C_chin', 'C_throat', 'jaw'],
      ['white', 'C_throat', 'backLow', 'jaw'],
      ['fur', 'jaw', 'backLow', 'cheekLow'],
      ['fur', 'cheek', 'cheekLow', 'backLow'],
      ['fur', 'cheek', 'backLow', 'back'],
      // The ear, its corners in the other order from the model's so its
      // faces face out, and its base, closed.
      ['inner', 'earInner', 'earTip', 'earFront'],
      ['inner', 'earOut', 'earTip', 'earInner'],
      ['fur', 'earOut', 'earInner', 'earFront'],
      ['fur', 'earBack', 'earTip', 'earOut'],
      ['fur', 'earFront', 'earTip', 'earBack'],
      ['fur', 'earFront', 'earBack', 'earOut'],
      // The back of the head, closed from its rim to the nape.
      ['fur', 'C_nape', 'C_back', 'back'],
      ['fur', 'C_nape', 'back', 'backLow'],
      ['fur', 'C_nape', 'backLow', 'C_throat'],
    ],
    middle: [],
    mouth: [0, -0.3075, 2.24], // under the muzzle
    pitch: 0,
    extras: [antlers],
    top: antlersTop(), // the antlers reach above the ears
  },
  tail: {
    at: [0, 0.8, -2.45],
    sections: [
      [0, 0, 0.18, 0.14],
      [0.1, -0.35, 0.22, 0.16],
      [-0.1, -0.7, 0.16, 0.12],
    ],
    start: 'flat',
    end: [-0.25, -0.85],
    paint: (_band, c, hub) => (c.y < hub.y ? 'white' : 'fur'), // white underneath
  },
  front: {
    at: [0.45, 0.2, 1.3],
    sections: [
      [0, 0, 0.32, 0.5],
      [-0.9, -0.05, 0.24, 0.35],
      [-1.7, 0.05, 0.14, 0.17], // the "knee" (really the wrist)
      [-2.1, 0.05, 0.12, 0.14],
      [-3.4, 0.1, 0.09, 0.11],
      [-3.7, 0.12, 0.12, 0.12], // the fetlock
      [-4, 0.2, 0.13, 0.14],
      [-4.15, 0.28, 0.14, 0.1], // the hoof
    ],
    start: 'flat',
    end: [-4.18, 0.34],
    middle: 2,
    ankle: 5,
    bend: 1,
    paint: (band) => (band >= 6 ? 'black' : 'fur'),
  },
  hind: {
    at: [0.48, 0.2, -1.8],
    sections: [
      [0, 0, 0.42, 0.72], // the thigh
      [-0.7, 0.15, 0.35, 0.55],
      [-1.3, 0.2, 0.2, 0.26], // the knee
      [-1.9, -0.2, 0.15, 0.2],
      [-2.4, -0.5, 0.12, 0.16], // the hock (a sharp backward point)
      [-3.4, -0.35, 0.09, 0.11],
      [-3.75, -0.25, 0.12, 0.12], // the fetlock
      [-4, -0.15, 0.13, 0.14],
      [-4.15, -0.08, 0.14, 0.1], // the hoof
    ],
    start: 'flat',
    end: [-4.18, 0],
    middle: 4,
    ankle: 6,
    bend: -1,
    paint: (band) => (band >= 7 ? 'black' : 'fur'),
  },
};

// Where an antler's point p (in the deer's units, on the right antler) is on
// the head, in the head's units: the antler leans out about its root, which
// sits at ANTLER_AT on the deer (side -1: the left one, the right mirrored).
function onHead([x, y, z]: Point, side: 1 | -1): Point {
  const lean = -side * ANTLER_LEAN;
  const [ax, ay, az] = ANTLER_AT;
  const mirrored = side * x;
  const turned = [mirrored * Math.cos(lean) - y * Math.sin(lean), mirrored * Math.sin(lean) + y * Math.cos(lean), z];
  return [(side * ax + turned[0] - HEAD_AT[0]) / HEAD_SCALE, (ay + turned[1] - HEAD_AT[1]) / HEAD_SCALE, (az + turned[2] - HEAD_AT[2]) / HEAD_SCALE];
}

// The antlers, in the model's bone colour ('antler'), each piece of flat
// faces, built on the head. The left antler is the right one mirrored, its
// faces wound the other way round so they still face out.
function antlers({ m, colors, point }: Dress): THREE.Object3D {
  const group = new THREE.Group();
  group.name = 'antlers';
  for (const side of [1, -1] as const) {
    for (const piece of ANTLER) {
      const surface = lofted(piece);
      for (const p of surface.points) p.copy(point(onHead([p.x, p.y, p.z], side)));
      if (side < 0) surface.faces = surface.faces.map(([a, b, c]) => [a, c, b] as const);
      group.add(mesh(loftGeometry(surface, () => colors.antler), m.coat));
    }
  }
  return group;
}

// How high the antlers reach, in the head's units.
function antlersTop(): number {
  return Math.max(...ANTLER.flatMap((piece) => lofted(piece).points.map((p) => onHead([p.x, p.y, p.z], 1)[1])));
}
