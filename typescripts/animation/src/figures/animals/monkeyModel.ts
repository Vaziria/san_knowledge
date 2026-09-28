import * as THREE from 'three';
import { decal, eyeDisc, loftFaces, napeOf, pair, sheet, solid, type Face } from './animalPark';
import type { Dress, Point } from './Head';
import { lofted, type Loft, type Paint } from './parts';
import type { Model } from './Quadruped';

// The monkey as the user modelled it: a low poly monkey walking on all fours,
// like a macaque or a capuchin, in three.js, like folded paper, from their
// animal park (the shape reference in Monkey.md, 2026-09-26). Its numbers
// are the user's, in the model's own units; `unit` makes it 50 cm from its
// rump to its face (5.16 units), a macaque's length, so it is 28 cm to the
// top of its shoulders and 51 cm to the top of its curled tail.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, a deep chest, a narrow waist, broad shoulders and a flat back,
//   a loft of six-sided rings from the rump, with a point behind, to where
//   the head joins it: brown, lighter underneath;
// - the head, its own and not the park's stretched wolf's, since a primate's
//   is built differently (monkeyHead() below): a round skull ending in a
//   flat, bare face, a short muzzle below the eyes with a nose and mouth, a
//   dark brow ridge, eyes close together looking forward, and round flat ears
//   low on the sides;
// - the arms and legs: arms from the sides of the chest, their elbows bending
//   back, and hands lying flat, fingers forward; legs with their knees
//   forward and the whole sole flat on the ground, long toes, 1.1 times the
//   model's units; bare hands and feet;
// - the tail, long, rising up and curling over at its end.
//
// It walks on its palms and soles: an arm bends at the elbow, pointing back,
// and at the wrist, where the hand begins; a leg at the knee, pointing
// forward, and at the ankle (Leg.ts).

const BODY: Model['body'] = {
  // A deep chest, a narrow waist, broad shoulders and a flat back.
  sections: [
    [0.45, -1.7, 0.45, 0.45],
    [0.55, -1.3, 0.7, 0.72],
    [0.6, -0.4, 0.6, 0.62],
    [0.62, 0.6, 0.75, 0.82],
    [0.75, 1.4, 0.78, 0.78], // the shoulders
    [0.95, 1.9, 0.42, 0.45], // the neck
  ],
  start: [0.45, -1.9],
  end: 'flat',
  paint: (_band, c, hub) => (c.y < hub.y - 0.5 ? 'light' : 'fur'),
};

const HEAD_AT: Point = [0, 1.3, 2.45];
const HEAD_SCALE = 0.9;

// The head's parts, in its units. A round skull: a loft along z that ends
// in a flat face.
const SKULL: Loft = {
  sections: [
    [0.1, -0.75, 0.3, 0.3],
    [0.05, -0.5, 0.62, 0.62],
    [0.0, -0.05, 0.75, 0.72], // widest (the braincase is large)
    [0.0, 0.35, 0.72, 0.7],
    [-0.05, 0.6, 0.6, 0.6], // flat face plane
  ],
  start: [0.1, -0.85],
  end: 'flat',
  sides: 8,
};
const skullPaint: Paint<string> = (_band, c) =>
  c.z > 0.45 && c.y < 0.38
    ? 'face' // bare face (mask shape)
    : c.z > 0.2 && c.y < -0.25
      ? 'face' // bare cheeks under the face
      : 'fur';

// A short muzzle sticking out below the eyes.
const MUZZLE: Loft = {
  sections: [
    [-0.3, 0.45, 0.32, 0.26],
    [-0.33, 0.75, 0.28, 0.22],
    [-0.35, 0.9, 0.22, 0.17],
  ],
  start: 'flat',
  end: 'flat',
};
const muzzlePaint: Paint<string> = (band, c, hub) => (band === 1 && c.z > 0.82 && c.y > hub.y ? 'nose' : c.y < hub.y - 0.12 && c.z > 0.8 ? 'mouth' : 'face');

// The brow ridge: a slanted band of fur above the eyes, its two halves
// meeting in the middle.
const B = { l: [0.5, 0.3, 0.52], m: [0, 0.26, 0.7], lt: [0.5, 0.4, 0.46], mt: [0, 0.37, 0.64] } as const;
const BROW: Face[] = [1, -1].flatMap((s) => {
  const f = (p: Point): Point => [p[0] * s, p[1], p[2]];
  return [
    { corners: [f(B.l), f(B.m), f(B.mt)], color: 'dark' },
    { corners: [f(B.l), f(B.mt), f(B.lt)], color: 'dark' },
  ];
});

// The head: skull, muzzle, brow, eyes close together facing forward (a disc
// facing +x turned to face +z), and round flat ears low on the sides, at eye
// level.
function monkeyHead(dress: Dress): THREE.Object3D {
  const head = new THREE.Group();
  head.name = 'monkey-head';
  head.add(
    solid(dress, loftFaces(SKULL, skullPaint), 'skull'),
    solid(dress, loftFaces(MUZZLE, muzzlePaint), 'muzzle'),
    sheet(dress, BROW, 'brow'),
    decal(dress, pair(eyeDisc(0.11, 'eye', 'pupil'), [0.24, 0.12, 0.66], [0, -Math.PI / 2, 0]), 'eyes', 'pupil'),
    sheet(dress, pair(eyeDisc(0.24, 'dark', 'face'), [0.74, 0.05, -0.1], [0, -0.35, 0]), 'ears'),
  );
  return head;
}

// Its highest point, the top of the skull, and its furthest forward, the
// end of the muzzle, for how big the head is (where the speech bubble goes);
// and under the muzzle, where a held figure goes.
const crown = lofted(SKULL).points.reduce((a, b) => (b.y > a.y ? b : a));
const snout = lofted(MUZZLE).points.reduce((a, b) => (b.z > a.z ? b : a));
const [lipY, lipZ, , lipH] = MUZZLE.sections[1];

export const MONKEY_MODEL: Model = {
  unit: 0.097,
  body: BODY,
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    points: { crown: [0, crown.y, crown.z], snout: [0, snout.y, snout.z], C_nape: napeOf(BODY, HEAD_AT, HEAD_SCALE) },
    side: [],
    middle: [],
    mouth: [0, lipY - lipH, lipZ],
    pitch: 0,
    extras: [monkeyHead],
  },
  // A long tail rising up and curling at the end.
  tail: {
    at: [0, 0.8, -1.75],
    sections: [
      [0, 0, 0.12, 0.12],
      [0.4, -0.4, 0.11, 0.11],
      [1.1, -0.7, 0.1, 0.1],
      [1.9, -0.8, 0.09, 0.09],
      [2.6, -0.65, 0.08, 0.08],
      [3.05, -0.3, 0.07, 0.07],
      [3.15, 0.1, 0.06, 0.06],
      [2.95, 0.35, 0.05, 0.05],
    ],
    start: 'flat',
    end: [2.75, 0.3],
    paint: () => 'fur',
  },
  // An arm: the elbow bends backward, the hand lies flat with its fingers
  // forward; the shoulders are at the sides of the chest.
  front: {
    at: [0.62, 0.4, 1.35],
    sections: [
      [0, 0, 0.2, 0.24],
      [-0.75, -0.18, 0.16, 0.18], // the elbow
      [-1.45, 0.02, 0.13, 0.14], // the wrist
      [-1.62, 0.1, 0.15, 0.1],
      [-1.7, 0.3, 0.17, 0.05],
      [-1.71, 0.5, 0.15, 0.04], // the fingers
    ],
    start: 'flat',
    end: [-1.72, 0.62],
    middle: 1,
    ankle: 2,
    bend: -1,
    paint: (band) => (band >= 3 ? 'face' : 'fur'),
  },
  // A leg: the knee forward, then the whole sole flat on the ground, long
  // toes.
  hind: {
    at: [0.5, 0.4, -1.3],
    scale: 1.1,
    sections: [
      [0, 0, 0.3, 0.42],
      [-0.6, 0.3, 0.2, 0.24], // the knee
      [-1.25, 0.05, 0.14, 0.16], // the ankle
      [-1.4, 0, 0.14, 0.12], // the heel
      [-1.52, 0.3, 0.16, 0.06],
      [-1.53, 0.62, 0.16, 0.05],
      [-1.54, 0.85, 0.12, 0.04], // the toes
    ],
    start: 'flat',
    end: [-1.55, 0.95],
    middle: 1,
    ankle: 2,
    bend: 1,
    paint: (band) => (band >= 3 ? 'face' : 'fur'),
  },
};
