import * as THREE from 'three';
import { whiskers, type Dress, type Point } from './Head';
import { mesh, polygons } from './parts';
import type { Model } from './Quadruped';

// The lion as the user modelled it: a low poly lion in three.js, like folded
// paper, the shape reference in Lion.md (2026-09-26). Its numbers are the
// user's, in the model's own units; `unit` makes it 1.2 m to the top of its
// shoulders (4.65 units), a male lion's height, so it is 2.4 m from its nose
// to the tip of its tail and 1.76 m to the top of its mane.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, big and muscular, a loft of six-sided rings from the rump, with
//   a point behind, up through the chest and neck to where the head joins it:
//   golden tawny, with a lighter cream belly;
// - the head, like the cat's but bigger and broader, flat faces round its
//   right half mirrored to the left: a long, wide cream muzzle, a broad brown
//   nose, amber eyes, small rounded ears, dark behind; the mane, a shaggy
//   collar of spikes round the face in three browns, and three whiskers a
//   side;
// - the legs, thick lofts with big paws, the left ones the same as the right;
// - the tail, long and hanging down, ending in a dark tuft.
//
// A leg bends at two of its rings (Leg.ts): a front leg at the wrist, where
// its thick top narrows, and at the ankle, where the paw begins; a hind leg
// at the hock and the ankle, its knee keeping the model's bend.
//
// Beyond the model, as for the wolf, the ears' faces are turned to face out
// and their bases closed, and the back of the head is closed to the nape,
// where the neck ends, which the head turns about.

// The mane, as the model builds it: a shaggy collar round the face, two
// layers of spikes. Each layer goes round the head in a circle, from its
// inner edge close to the face out to its spiky outer edge and back in behind
// it; longer spikes underneath make it hang down over the chest. In the
// head's units: `n` spikes, the inner edge's radius and z, the spikes' tips'
// and the valleys' radii and z, the back edge's z, and a turn of part of a
// spike, so the second layer's spikes stand between the first's.
const MANE = {
  centerY: 0.25,
  layers: [
    { n: 16, rIn: 1.2, zIn: 0.15, rTip: 2, rValley: 1.55, zOut: -0.4, zBack: -1.2, twist: 0 },
    { n: 16, rIn: 1.15, zIn: -0.25, rTip: 2.25, rValley: 1.7, zOut: -0.85, zBack: -1.55, twist: 0.5 },
  ],
} as const;

const HEAD_POINTS = {
  C_back: [0, 1.3, -0.9],
  C_forehead: [0, 1.5, 0.2],
  C_brow: [0, 0.95, 0.95],
  C_stop: [0, 0.7, 1.25],
  C_snoutTip: [0, 0.45, 1.95],
  C_noseBot: [0, 0.1, 2.02],
  C_chin: [0, -0.6, 1.55],
  C_throat: [0, -0.8, 0.2],
  topPlate: [0.5, 1.45, 0.2],
  temple: [0.95, 1.1, 0.3],
  brow: [0.42, 0.95, 0.9],
  browOuter: [0.92, 0.9, 0.6],
  eyeIn: [0.32, 0.72, 1.1],
  eyeOut: [0.72, 0.78, 0.85],
  eyeLow: [0.5, 0.5, 1.02],
  eyeDeep: [0.5, 0.7, 0.85],
  cheek: [1.2, 0.4, 0.1],
  cheekLow: [1, -0.35, 0.6],
  jaw: [0.5, -0.5, 1.35],
  back: [0.85, 0.95, -0.8],
  backLow: [1, -0.3, -0.5],
  snoutBase: [0.42, 0.55, 1.3],
  snoutTip: [0.38, 0.35, 1.9],
  noseBot: [0.28, 0, 1.98],
  snoutBot: [0.35, -0.4, 1.6],
  earFront: [0.55, 1.4, 0.2],
  earOut: [1, 1.2, 0.1],
  earBack: [0.78, 1.35, -0.25],
  earTip: [0.95, 1.85, 0],
  earInner: [0.8, 1.5, 0.05],
  C_nape: [0, 0.070588, -0.835294], // beyond the model: where the neck ends, the back of the head's middle
} as const;

export const LION_MODEL: Model<keyof typeof HEAD_POINTS> = {
  unit: 0.258,
  body: {
    sections: [
      [0.4, -2.6, 0.55, 0.55],
      [0.5, -2.05, 0.95, 1], // the hips
      [0.55, -0.8, 0.9, 0.92],
      [0.45, 0.5, 1, 1.1], // the chest
      [0.6, 1.6, 0.98, 1.12], // the shoulders
      [1.15, 2.35, 0.72, 0.8],
      [1.6, 2.75, 0.62, 0.68], // the neck
    ],
    start: [0.4, -2.85],
    end: 'flat',
    paint: (_band, c, hub) => (c.y < hub.y - 0.55 ? 'cream' : 'tawny'), // a lighter belly
  },
  head: {
    at: [0, 1.54, 3.46],
    scale: 0.85,
    points: HEAD_POINTS,
    // The model's faces in its colours: tawny fur, a cream muzzle, chin and ear insides, a brown nose, dark ear backs, and the amber eyes (glossy).
    side: [
      ['tawny', 'C_back', 'C_forehead', 'topPlate'],
      ['tawny', 'C_back', 'topPlate', 'back'],
      ['tawny', 'C_forehead', 'brow', 'topPlate'],
      ['tawny', 'C_forehead', 'C_brow', 'brow'],
      ['tawny', 'topPlate', 'brow', 'temple'],
      ['tawny', 'temple', 'brow', 'browOuter'],
      ['tawny', 'topPlate', 'temple', 'back'],
      ['tawny', 'back', 'temple', 'cheek'],
      ['tawny', 'temple', 'browOuter', 'cheek'],
      ['tawny', 'brow', 'eyeOut', 'browOuter'],
      ['tawny', 'brow', 'eyeIn', 'eyeOut'],
      ['tawny', 'C_brow', 'eyeIn', 'brow'],
      ['tawny', 'browOuter', 'eyeOut', 'cheek'],
      ['tawny', 'eyeOut', 'cheekLow', 'cheek'],
      ['tawny', 'eyeOut', 'eyeLow', 'cheekLow'],
      ['tawny', 'eyeIn', 'snoutBase', 'eyeLow'],
      ['eye', 'eyeIn', 'eyeLow', 'eyeDeep'],
      ['eye', 'eyeLow', 'eyeOut', 'eyeDeep'],
      ['eye', 'eyeOut', 'eyeIn', 'eyeDeep'],
      ['tawny', 'C_brow', 'snoutBase', 'eyeIn'],
      ['tawny', 'C_brow', 'C_stop', 'snoutBase'],
      ['tawny', 'eyeLow', 'jaw', 'cheekLow'],
      ['cream', 'eyeLow', 'snoutBase', 'jaw'],
      ['cream', 'snoutBase', 'snoutBot', 'jaw'],
      ['tawny', 'C_stop', 'snoutTip', 'snoutBase'],
      ['tawny', 'C_stop', 'C_snoutTip', 'snoutTip'],
      ['cream', 'snoutBase', 'noseBot', 'snoutBot'],
      ['cream', 'snoutBase', 'snoutTip', 'noseBot'],
      ['nose', 'C_snoutTip', 'noseBot', 'snoutTip'],
      ['nose', 'C_snoutTip', 'C_noseBot', 'noseBot'],
      ['cream', 'C_noseBot', 'snoutBot', 'noseBot'],
      ['cream', 'C_noseBot', 'C_chin', 'snoutBot'],
      ['cream', 'C_chin', 'jaw', 'snoutBot'],
      ['cream', 'C_chin', 'C_throat', 'jaw'],
      ['tawny', 'C_throat', 'backLow', 'jaw'],
      ['tawny', 'jaw', 'backLow', 'cheekLow'],
      ['tawny', 'cheek', 'cheekLow', 'backLow'],
      ['tawny', 'cheek', 'backLow', 'back'],
      // The ear, its corners in the other order from the model's so its
      // faces face out, and its base, closed.
      ['cream', 'earInner', 'earTip', 'earFront'],
      ['cream', 'earOut', 'earTip', 'earInner'],
      ['tawny', 'earOut', 'earInner', 'earFront'],
      ['earBack', 'earBack', 'earTip', 'earOut'],
      ['earBack', 'earFront', 'earTip', 'earBack'],
      ['tawny', 'earFront', 'earBack', 'earOut'],
      // The back of the head, closed from its rim to the nape.
      ['tawny', 'C_nape', 'C_back', 'back'],
      ['tawny', 'C_nape', 'back', 'backLow'],
      ['tawny', 'C_nape', 'backLow', 'C_throat'],
    ],
    middle: [],
    mouth: [0, -0.165, 1.8085], // under the muzzle
    pitch: 0,
    extras: [
      mane,
      // Three whiskers a side, from beside the nose.
      whiskers(
        [
          [[0.4, 0.3, 1.8], [1.7, 0.45, 1.55]],
          [[0.4, 0.2, 1.8], [1.7, 0.2, 1.6]],
          [[0.4, 0.1, 1.8], [1.7, -0.05, 1.5]],
        ],
        'whisker',
      ),
    ],
    top: maneTop(), // the mane reaches above the ears
  },
  tail: {
    at: [0, 0.9, -2.7],
    sections: [
      [0, 0, 0.15, 0.15],
      [0.05, -0.5, 0.13, 0.13],
      [-0.4, -1, 0.12, 0.12],
      [-1.1, -1.3, 0.11, 0.11],
      [-1.8, -1.4, 0.1, 0.1],
      [-2.2, -1.3, 0.2, 0.2], // the tuft starts
      [-2.5, -1.15, 0.22, 0.22],
    ],
    start: 'flat',
    end: [-2.75, -1],
    paint: (band) => (band >= 4 ? 'tuft' : 'tawny'), // ending in a dark tuft
  },
  front: {
    at: [0.58, 0.2, 1.4],
    sections: [
      [0, 0, 0.42, 0.6],
      [-0.9, -0.05, 0.32, 0.45],
      [-1.7, 0.05, 0.24, 0.28], // the wrist
      [-2.5, 0.05, 0.22, 0.25],
      [-2.75, 0.12, 0.26, 0.26], // the ankle
      [-2.9, 0.3, 0.33, 0.2],
      [-2.95, 0.55, 0.33, 0.15], // the big paw
    ],
    start: 'flat',
    end: [-2.97, 0.72],
    middle: 2,
    ankle: 4,
    bend: 1,
    paint: () => 'tawny',
  },
  hind: {
    at: [0.6, 0.2, -2],
    sections: [
      [0, 0, 0.52, 0.85], // the thigh
      [-0.6, 0.15, 0.44, 0.7],
      [-1.1, 0.3, 0.28, 0.34], // the knee
      [-1.6, 0, 0.22, 0.26],
      [-2.05, -0.32, 0.19, 0.23], // the hock
      [-2.55, -0.2, 0.18, 0.21],
      [-2.78, -0.05, 0.24, 0.24], // the ankle
      [-2.9, 0.15, 0.32, 0.2],
      [-2.95, 0.42, 0.33, 0.15], // the paw
    ],
    start: 'flat',
    end: [-2.97, 0.62],
    middle: 4,
    ankle: 6,
    bend: -1,
    paint: () => 'tawny',
  },
};

// The model's repeatable "random" number from a seed, which mixes the mane's
// colours.
function rand(n: number): number {
  const v = Math.sin(n * 91.7) * 43758.55;
  return v - Math.floor(v);
}

// The mane's faces as the model makes them, each three corners in the head's
// units and one of its three browns, and the middle of its spike.
function maneFaces(): { corners: Point[]; color: string; spike: THREE.Vector3 }[] {
  const faces: { corners: Point[]; color: string; spike: THREE.Vector3 }[] = [];
  const pick = (seed: number) => {
    const r = rand(seed);
    return r < 0.33 ? 'maneDark' : r < 0.7 ? 'mane' : 'maneLight';
  };
  MANE.layers.forEach((layer, li) => {
    // A point on an oval round the head: a little taller than wide, longer
    // at the bottom, where the mane hangs over the chest.
    const at = (angle: number, radius: number, z: number): Point => {
      const droop = 1 + 0.3 * Math.max(0, -Math.sin(angle));
      return [Math.cos(angle) * radius, MANE.centerY + Math.sin(angle) * radius * 1.08 * droop, z];
    };
    for (let k = 0; k < layer.n; k++) {
      const a0 = ((k + layer.twist) / layer.n) * Math.PI * 2;
      const a1 = ((k + 1 + layer.twist) / layer.n) * Math.PI * 2;
      const am = (a0 + a1) / 2;
      const in0 = at(a0, layer.rIn, layer.zIn);
      const in1 = at(a1, layer.rIn, layer.zIn);
      const v0 = at(a0, layer.rValley, layer.zOut);
      const v1 = at(a1, layer.rValley, layer.zOut);
      const tip = at(am, layer.rTip * (0.9 + rand(k + li * 50) * 0.25), layer.zOut - 0.15);
      const b0 = at(a0, layer.rIn * 0.9, layer.zBack);
      const b1 = at(a1, layer.rIn * 0.9, layer.zBack);
      const seed = k * 7 + li * 100;
      const spike = [in0, in1, v0, v1, tip, b0, b1].reduce((sum, p) => sum.add(new THREE.Vector3(...p)), new THREE.Vector3()).divideScalar(7);
      const add = (corners: Point[], s: number) => faces.push({ corners, color: pick(seed + s), spike });
      // The front of the collar, then its back.
      add([in0, in1, tip], 1);
      add([in0, tip, v0], 2);
      add([in1, v1, tip], 3);
      add([b0, tip, b1], 4);
      add([b0, v0, tip], 5);
      add([b1, tip, v1], 6);
    }
  });
  return faces;
}

// The mane on the head. The model draws it from both sides; here each face is
// turned to face out of its spike, and as a thin shell it is drawn twice
// (rules.md, Building shapes 9): its outside, which casts the shadow, and its
// inside in the same colours, seen through the gaps round the face.
function mane({ m, colors, point }: Dress): THREE.Object3D {
  const faces = maneFaces().map(({ corners, color, spike }) => {
    const [a, b, c] = corners.map((p) => point(p));
    const normal = new THREE.Vector3().subVectors(b, a).cross(new THREE.Vector3().subVectors(c, a));
    const middle = a.clone().add(b).add(c).divideScalar(3);
    return { corners: normal.dot(middle.sub(point([spike.x, spike.y, spike.z]))) < 0 ? [a, c, b] : [a, b, c], color: colors[color] };
  });
  const geometry = polygons(
    faces.map((f) => f.corners),
    (_n, i) => faces[i].color,
  );
  const group = new THREE.Group();
  group.name = 'mane';
  const inside = m.coat.clone();
  inside.side = THREE.BackSide;
  const back = mesh(geometry, inside);
  back.castShadow = false;
  group.add(mesh(geometry, m.coat), back);
  return group;
}

// How high the mane reaches, in the head's units.
function maneTop(): number {
  return Math.max(...maneFaces().flatMap((f) => f.corners.map((p) => p[1])));
}
