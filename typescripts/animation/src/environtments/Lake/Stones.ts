import * as THREE from 'three';
import type { Theme } from '../../theme';
import { groundHeight, shoreRadius } from './Ground';

// Stones along the lakeside: rough, flat-faced rocks lying in clumps around
// the shore, from pebbles to rocks nearly a meter across, some up on the bank
// and some at the water's edge with their feet in the water. Each is bedded a
// little into the ground. They are placed by a seeded random generator, so
// the lake looks the same every time, and none lie near `keepClear` (where
// standing figures go).
//
// A rock shape is a sphere cut by a few random planes, like chipped stone,
// drawn with flat faces. A handful of shapes is shared by all the stones,
// each stone turned, tilted and sized on its own.

const CLUMPS = 16;
const STONES_PER_CLUMP = { min: 2, max: 7 };
const CLUMP_SPREAD = 0.9; // m along the shore either side of a clump's middle
const REACH = { water: 0.5, bank: 1.6 }; // m from the waterline: out into the water, up the bank
const RADIUS = { min: 0.06, max: 0.4 }; // m; small stones are far more common than big ones
const FLATNESS = { min: 0.45, max: 0.75 }; // height as a share of width
const SINK = 0.35; // share of its height a stone is bedded into the ground
const TILT = 0.15; // radians it may lean either way
const CLEAR = 3; // m kept free of stones around keepClear
const SHAPES = 6;
const CUTS = 8; // planes cut from each shape's sphere
const CUT_DEPTH = { min: 0.55, max: 0.85 }; // where a cut sits, as a share of the radius
const SEED = 7;

// A repeatable random number generator (mulberry32): the same seed always
// gives the same numbers, in 0..1.
function random(seed: number): () => number {
  let a = seed;
  return () => {
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// A unit sphere with CUTS random slices taken off: every point beyond a
// cutting plane is pressed back onto it, leaving a flat face.
function rockShape(rand: () => number): THREE.BufferGeometry {
  const geo = new THREE.IcosahedronGeometry(1, 2);
  const cuts = Array.from({ length: CUTS }, () => ({
    normal: new THREE.Vector3(rand() * 2 - 1, rand() * 2 - 1, rand() * 2 - 1).normalize(),
    depth: CUT_DEPTH.min + rand() * (CUT_DEPTH.max - CUT_DEPTH.min),
  }));
  const position = geo.attributes.position;
  const p = new THREE.Vector3();
  for (let i = 0; i < position.count; i++) {
    p.fromBufferAttribute(position, i);
    for (const cut of cuts) {
      const beyond = p.dot(cut.normal) - cut.depth;
      if (beyond > 0) p.addScaledVector(cut.normal, -beyond);
    }
    position.setXYZ(i, p.x, p.y, p.z);
  }
  geo.computeVertexNormals();
  return geo;
}

export class Stones extends THREE.Group {
  constructor(theme: Theme, keepClear: THREE.Vector3) {
    super();
    this.name = 'stones';
    const rand = random(SEED);
    const shapes = Array.from({ length: SHAPES }, () => rockShape(rand));
    // The same matte finish as the ground, with flat faces.
    const material = new THREE.MeshStandardMaterial({ color: theme.scene.stone, roughness: 0.9, flatShading: true });

    let clumps = 0;
    for (let attempt = 0; clumps < CLUMPS && attempt < CLUMPS * 10; attempt++) {
      const middle = rand() * 2 * Math.PI;
      const shore = shoreRadius(middle);
      const spot = new THREE.Vector3(shore * Math.cos(middle), 0, shore * Math.sin(middle));
      if (Math.hypot(spot.x - keepClear.x, spot.z - keepClear.z) < CLEAR + REACH.bank + CLUMP_SPREAD) continue;
      clumps++;

      const count = STONES_PER_CLUMP.min + Math.floor(rand() * (STONES_PER_CLUMP.max - STONES_PER_CLUMP.min + 1));
      for (let k = 0; k < count; k++) {
        const angle = middle + ((rand() * 2 - 1) * CLUMP_SPREAD) / shore;
        const r = shoreRadius(angle) - REACH.water + rand() * (REACH.water + REACH.bank);
        const x = r * Math.cos(angle);
        const z = r * Math.sin(angle);
        const radius = RADIUS.min + (RADIUS.max - RADIUS.min) * rand() ** 3;
        const halfHeight = radius * (FLATNESS.min + rand() * (FLATNESS.max - FLATNESS.min));

        const stone = new THREE.Mesh(shapes[Math.floor(rand() * SHAPES)], material);
        stone.scale.set(radius, halfHeight, radius * (0.8 + 0.4 * rand()));
        stone.rotation.set((rand() * 2 - 1) * TILT, rand() * 2 * Math.PI, (rand() * 2 - 1) * TILT);
        stone.position.set(x, groundHeight(x, z) + halfHeight * (1 - 2 * SINK), z);
        stone.castShadow = true;
        stone.receiveShadow = true;
        this.add(stone);
      }
    }
  }
}
