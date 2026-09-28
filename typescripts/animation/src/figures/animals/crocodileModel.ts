import * as THREE from 'three';
import { decal, eyeDisc, loftFaces, napeOf, pair, ridge, sheet, solid, stretchedHead, type Face } from './animalPark';
import type { Dress, Point } from './Head';
import type { Loft, LoftSection } from './parts';
import type { Model } from './Quadruped';

// The crocodile as the user modelled it: a low poly crocodile in three.js,
// like folded paper, from their animal park (the shape reference in
// Crocodile.md, 2026-09-26). Its numbers are the user's, in the model's own
// units; `unit` makes it 3.5 m from its nose to the tip of its tail (14.7
// units), a grown saltwater crocodile's length, so it is 41 cm to the top of
// its back and 86 cm across its splayed feet.
//
// Its parts, each where the model puts it (Quadruped.ts, Model):
// - the body, long, low and broad, a loft of six-sided rings flat at both
//   ends: green, with a pale belly and darker bands across its back, and two
//   rows of dark scutes along it;
// - the head, the park's (animalPark.ts) stretched long, low and narrow into
//   a long snout, with no ears: green, a darker top of the snout and nose, a
//   pale jaw and throat; teeth along both sides of the snout, and its eyes on
//   little bumps on top of the head, yellow round a dark middle;
// - the legs, short and turned out 0.5 radians from its sides, each ending
//   in a flat, dark foot pointing forward, the hind ones 1.1 times the front
//   ones;
// - the tail, long and tapering, in green and dark bands with a pale
//   underside, and a row of dark spikes along its top.
//
// A leg bends at two of its rings (Leg.ts): a front leg at the elbow,
// pointing back, and at the wrist, where the foot begins; a hind leg at the
// knee, pointing forward, and at the ankle.

const BODY: Model['body'] = {
  sections: [
    [0.45, -3.0, 0.55, 0.4],
    [0.5, -2.2, 1.05, 0.6],
    [0.5, -0.8, 1.2, 0.65],
    [0.5, 0.6, 1.15, 0.62],
    [0.5, 1.8, 0.9, 0.52],
    [0.48, 2.5, 0.7, 0.45],
  ],
  start: 'flat',
  end: 'flat',
  // A pale belly, and darker bands across its back.
  paint: (band, c, hub) => (c.y < hub.y - 0.3 ? 'belly' : c.y > hub.y + 0.3 && band % 2 ? 'dark' : 'green'),
  // Two rows of scutes along the back.
  extras: [(dress) => sheet(dress, ridge([[0.5, -2.9, 0.3, 0.6], [0.5, 1.6, 0.3, 0.5]], 14, 0.12, 'dark', [-0.25, 0.25]), 'scutes')],
};

const HEAD_AT: Point = [0, 0.4, 3.2];
const HEAD_SCALE = 0.7;

const TAIL_SECTIONS: LoftSection[] = [
  [0, 0, 0.5, 0.4],
  [0, -1.2, 0.42, 0.35],
  [-0.05, -2.4, 0.32, 0.3],
  [-0.1, -3.6, 0.22, 0.22],
  [-0.12, -4.6, 0.13, 0.14],
];

export const CROCODILE_MODEL: Model = {
  unit: 0.239,
  body: BODY,
  head: {
    at: HEAD_AT,
    scale: HEAD_SCALE,
    ...stretchedHead(
      {
        w: 0.75,
        h: 0.5,
        snout: 2.3,
        snoutW: 2.6,
        snoutDrop: 0.05,
        ear: { show: false },
        colors: { top: 'green', snoutTop: 'dark', muzzle: 'green', chin: 'belly', throat: 'belly', cheekLow: 'belly', nose: 'dark', eye: 'green' },
      },
      napeOf(BODY, HEAD_AT, HEAD_SCALE),
    ),
    extras: [teeth, eyes],
  },
  tail: {
    at: [0, 0.45, -2.95],
    sections: TAIL_SECTIONS,
    start: 'flat',
    end: [-0.14, -5.4],
    paint: (_band, c, hub) => (c.y < hub.y - 0.2 ? 'belly' : Math.floor(-c.z * 1.6) % 2 ? 'dark' : 'green'),
    extras: [(dress) => sheet(dress, ridge(TAIL_SECTIONS, 16, 0.18, 'dark'), 'ridge')],
  },
  front: {
    at: [0.95, 0.45, 1.6],
    splay: 0.5,
    sections: [
      [0, 0, 0.28, 0.3],
      [-0.45, 0.05, 0.22, 0.24], // the elbow
      [-0.8, 0.12, 0.18, 0.2], // the wrist
      [-0.95, 0.3, 0.26, 0.1],
      [-0.98, 0.5, 0.3, 0.06], // the flat foot
    ],
    start: 'flat',
    end: [-1.0, 0.65],
    middle: 1,
    ankle: 2,
    bend: -1,
    paint: (band) => (band >= 3 ? 'dark' : 'green'),
  },
  hind: {
    at: [1.0, 0.45, -1.9],
    splay: 0.5,
    scale: 1.1,
    sections: [
      [0, 0, 0.3, 0.32],
      [-0.45, 0.05, 0.24, 0.26], // the knee
      [-0.8, 0.12, 0.19, 0.21], // the ankle
      [-0.95, 0.3, 0.28, 0.1],
      [-0.98, 0.55, 0.32, 0.06], // the flat foot
    ],
    start: 'flat',
    end: [-1.0, 0.72],
    middle: 1,
    ankle: 2,
    bend: 1,
    paint: (band) => (band >= 3 ? 'dark' : 'green'),
  },
};

// Teeth sticking down along both sides of the long snout, in the head's
// units.
function teeth(dress: Dress): THREE.Object3D {
  const faces: Face[] = [];
  for (let z = 1.6; z < 4.0; z += 0.26) {
    for (const s of [1, -1]) {
      const x = s * (0.6 - (z - 1.35) * 0.045);
      faces.push({ corners: [[x, -0.06, z - 0.06], [x, -0.06, z + 0.06], [x * 0.98, -0.24, z]], color: 'tooth' });
    }
  }
  return sheet(dress, faces, 'teeth');
}

// A bump on top of the head, for an eye to sit on.
const BUMP: Loft = { sections: [[0, -0.15, 0.14, 0.1], [0, 0.15, 0.14, 0.1]], start: [0, -0.28], end: [0, 0.28] };

// The eyes on top of the head, on little bumps: yellow round a dark middle.
function eyes(dress: Dress): THREE.Object3D {
  const group = new THREE.Group();
  group.name = 'eyes';
  group.add(
    solid(dress, pair(loftFaces(BUMP, () => 'green'), [0.34, 0.78, 0.45]), 'bumps'),
    decal(dress, pair(eyeDisc(0.1, 'eye', 'pupil'), [0.45, 0.84, 0.47], [0, -0.3, 0.6]), 'discs', 'pupil'),
  );
  return group;
}
