import * as THREE from 'three';
import { between } from '../../figures/Tree/parts';
import type { Theme } from '../../theme';
import { CLEAR, groundHeight, shoreRadius } from './Ground';

// Stones along the lakeside: rough, flat-faced rocks lying in clumps around
// the shore, from pebbles to rocks nearly a meter across, some up on the bank
// and some at the water's edge with their feet in the water. Each is bedded a
// little into the ground. They are placed by a seeded random generator, so
// the lake looks the same every time, and none lie near `keepClear` (where
// standing figures go).
//
// A rock shape is a sphere cut by a few random planes, like chipped stone,
// drawn with flat faces. A handful of shapes is shared by all the stones,
// each stone turned, tilted and sized on its own, and each shape is one
// InstancedMesh, so however many stones there are they take SHAPES draw
// calls.
//
// The first 75 are the lake's first stones, in 16 clumps (the constructor,
// with the same random numbers as ever, so they keep their places). The user
// then asked for more ("add more stone in sidelake"): scatter() adds about
// 145 more, from a generator of their own, once the boulders, trees and mud
// pits are placed, since the new ones keep off them: more clumps all round,
// loose pebbles and small stones along the waterline between them, and a few
// big stones where the preview cameras look, on the near bank either side
// of the landing and on the far shore. The lake calls scatter(), which also
// builds the meshes.

const CLUMPS = 16;
const STONES_PER_CLUMP = { min: 2, max: 7 };
const CLUMP_SPREAD = 0.9; // m along the shore either side of a clump's middle
const REACH = { water: 0.5, bank: 1.6 }; // m from the waterline: out into the water, up the bank
const RADIUS = { min: 0.06, max: 0.4 }; // m; small stones are far more common than big ones
const FLATNESS = { min: 0.45, max: 0.75 }; // height as a share of width
const SINK = 0.35; // share of its height a stone is bedded into the ground
const TILT = 0.15; // radians it may lean either way
const MARGIN = 0.8; // m kept free of stones beyond the landing's clearing (3 m round keepClear by default)
const SHAPES = 6;
const CUTS = 8; // planes cut from each shape's sphere
const CUT_DEPTH = { min: 0.55, max: 0.85 }; // where a cut sits, as a share of the radius
const SEED = 7;
const LARGEST = 0.4; // m, the largest stone's radius

// The stones scatter() adds.
const MORE = {
  clumps: 14,
  // Loose along the waterline: how many, their radii (m, small ones more
  // common), and m from the waterline, out into the water to up the bank.
  pebbles: { count: 90, radius: [0.025, 0.12] as const, reach: [-0.35, 0.5] as const },
  // Big stones, 0.5-0.7 m across, where the preview cameras look: round the
  // lake (radians from +x toward +z), and m from the waterline.
  big: [
    { around: [1.95, 2.25] as const, reach: [0.3, 1.2] as const }, // the near bank, left of the landing
    { around: [0.9, 1.2] as const, reach: [0.3, 1.2] as const }, // and right of it
    { around: [2.15, 2.45] as const, reach: [-0.2, 0.8] as const }, // further left, at the water's edge
    { around: [-1.9, -1.6] as const, reach: [-0.2, 1] as const }, // the far shore, facing the cameras
    { around: [-1.5, -1.2] as const, reach: [-0.2, 1] as const },
    { around: [-2.3, -2] as const, reach: [-0.2, 1] as const },
  ],
  bigRadius: [0.25, 0.35] as const,
  seed: 71,
};

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

// A stone: its shape, where it lies (x, z, and its radius across), and its
// matrix in the lake.
export interface Stone {
  shape: number;
  x: number;
  z: number;
  radius: number;
  matrix: THREE.Matrix4;
}

export class Stones extends THREE.Group {
  readonly placed: Stone[] = []; // the first 75, then the ones scatter() adds
  // The stones by the square meter they lie in, so near() looks only at those
  // round a point: the grass asks it for 17 k spots.
  private readonly cells = new Map<string, Stone[]>();
  private readonly shapes: THREE.BufferGeometry[];
  private readonly material: THREE.MeshStandardMaterial;
  private readonly keepClear: THREE.Vector3;
  private readonly clearing: number; // m round keepClear kept level and clear (LakeOptions.clear)

  // Places the lake's first 75 stones; scatter() adds the rest and builds
  // the meshes.
  constructor(theme: Theme, keepClear: THREE.Vector3, clear = CLEAR) {
    super();
    this.name = 'stones';
    this.keepClear = keepClear;
    this.clearing = clear;
    const rand = random(SEED);
    this.shapes = Array.from({ length: SHAPES }, () => rockShape(rand));
    // The same matte finish as the ground, with flat faces.
    this.material = new THREE.MeshStandardMaterial({ color: theme.scene.stone, roughness: 0.9, flatShading: true });

    let clumps = 0;
    for (let attempt = 0; clumps < CLUMPS && attempt < CLUMPS * 10; attempt++) {
      const middle = rand() * 2 * Math.PI;
      const shore = shoreRadius(middle);
      const spot = new THREE.Vector3(shore * Math.cos(middle), 0, shore * Math.sin(middle));
      if (Math.hypot(spot.x - keepClear.x, spot.z - keepClear.z) < clear + MARGIN + REACH.bank + CLUMP_SPREAD) continue;
      clumps++;

      const count = STONES_PER_CLUMP.min + Math.floor(rand() * (STONES_PER_CLUMP.max - STONES_PER_CLUMP.min + 1));
      for (let k = 0; k < count; k++) {
        const angle = middle + ((rand() * 2 - 1) * CLUMP_SPREAD) / shore;
        const r = shoreRadius(angle) - REACH.water + rand() * (REACH.water + REACH.bank);
        const radius = RADIUS.min + (RADIUS.max - RADIUS.min) * rand() ** 3;
        this.stone(rand, r * Math.cos(angle), r * Math.sin(angle), radius);
      }
    }
  }

  // Where each stone lies and how big it is.
  get spots(): { x: number; z: number; radius: number }[] {
    return this.placed.map(({ x, z, radius }) => ({ x, z, radius }));
  }

  // Whether a point of the lake is within `gap` m of a stone.
  near(x: number, z: number, gap: number): boolean {
    const reach = Math.ceil(LARGEST + Math.max(0, gap));
    const cx = Math.floor(x);
    const cz = Math.floor(z);
    for (let i = cx - reach; i <= cx + reach; i++) {
      for (let j = cz - reach; j <= cz + reach; j++) {
        const cell = this.cells.get(`${i},${j}`);
        if (cell?.some((stone) => Math.hypot(x - stone.x, z - stone.z) < stone.radius + gap)) return true;
      }
    }
    return false;
  }

  // Adds the stones the user asked for later, keeping them off whatever
  // `blocked` says is there (a stone `radius` m across at x, z: on a boulder,
  // at a trunk, in a mud pit), and builds the meshes.
  scatter(blocked: (x: number, z: number, radius: number) => boolean): void {
    const rand = random(MORE.seed);
    const free = (x: number, z: number, radius: number) =>
      Math.hypot(x - this.keepClear.x, z - this.keepClear.z) >= this.clearing + MARGIN + radius && !blocked(x, z, radius);

    // More clumps, as the first ones.
    let clumps = 0;
    for (let attempt = 0; clumps < MORE.clumps && attempt < MORE.clumps * 10; attempt++) {
      const middle = rand() * 2 * Math.PI;
      const shore = shoreRadius(middle);
      const spot = new THREE.Vector3(shore * Math.cos(middle), 0, shore * Math.sin(middle));
      if (Math.hypot(spot.x - this.keepClear.x, spot.z - this.keepClear.z) < this.clearing + MARGIN + REACH.bank + CLUMP_SPREAD) continue;
      clumps++;
      const count = STONES_PER_CLUMP.min + Math.floor(rand() * (STONES_PER_CLUMP.max - STONES_PER_CLUMP.min + 1));
      for (let k = 0; k < count; k++) {
        const angle = middle + ((rand() * 2 - 1) * CLUMP_SPREAD) / shore;
        const r = shoreRadius(angle) - REACH.water + rand() * (REACH.water + REACH.bank);
        const radius = RADIUS.min + (RADIUS.max - RADIUS.min) * rand() ** 3;
        const x = r * Math.cos(angle);
        const z = r * Math.sin(angle);
        if (free(x, z, radius)) this.stone(rand, x, z, radius);
      }
    }

    // Loose pebbles and small stones along the waterline, all round.
    for (let k = 0; k < MORE.pebbles.count; k++) {
      const angle = rand() * 2 * Math.PI;
      const r = shoreRadius(angle) + between(rand, MORE.pebbles.reach[0], MORE.pebbles.reach[1]);
      const radius = MORE.pebbles.radius[0] + (MORE.pebbles.radius[1] - MORE.pebbles.radius[0]) * rand() ** 2;
      const x = r * Math.cos(angle);
      const z = r * Math.sin(angle);
      if (free(x, z, radius)) this.stone(rand, x, z, radius);
    }

    // A few big stones where the preview cameras look.
    for (const place of MORE.big) {
      for (let attempt = 0; attempt < 30; attempt++) {
        const angle = between(rand, place.around[0], place.around[1]);
        const r = shoreRadius(angle) + between(rand, place.reach[0], place.reach[1]);
        const radius = between(rand, MORE.bigRadius[0], MORE.bigRadius[1]);
        const x = r * Math.cos(angle);
        const z = r * Math.sin(angle);
        if (!free(x, z, radius) || this.near(x, z, radius)) continue;
        this.stone(rand, x, z, radius);
        break;
      }
    }
    this.build();
  }

  // A stone of a random shape at x, z: flattened, stretched, tilted and
  // turned at random, and bedded into the ground. The random numbers are
  // drawn in the order the first stones always drew them.
  private stone(rand: () => number, x: number, z: number, radius: number): void {
    const halfHeight = radius * (FLATNESS.min + rand() * (FLATNESS.max - FLATNESS.min));
    const shape = Math.floor(rand() * SHAPES);
    const scale = new THREE.Vector3(radius, halfHeight, radius * (0.8 + 0.4 * rand()));
    const turn = new THREE.Euler((rand() * 2 - 1) * TILT, rand() * 2 * Math.PI, (rand() * 2 - 1) * TILT);
    const position = new THREE.Vector3(x, groundHeight(x, z, this.clearing) + halfHeight * (1 - 2 * SINK), z);
    const matrix = new THREE.Matrix4().compose(position, new THREE.Quaternion().setFromEuler(turn), scale);
    const stone = { shape, x, z, radius, matrix };
    this.placed.push(stone);
    const key = `${Math.floor(x)},${Math.floor(z)}`;
    const cell = this.cells.get(key);
    if (cell) cell.push(stone);
    else this.cells.set(key, [stone]);
  }

  // Takes away the stones `gone` picks, from the meshes, from placed and from
  // near(): those in a dock's way (Lake.siteDock), as its builders would
  // clear its site. Every other stone keeps its place. How many went.
  takeAway(gone: (stone: Stone) => boolean): number {
    const kept = this.placed.filter((stone) => !gone(stone));
    const count = this.placed.length - kept.length;
    if (!count) return 0;
    this.placed.length = 0;
    this.placed.push(...kept);
    this.cells.clear();
    for (const stone of kept) {
      const key = `${Math.floor(stone.x)},${Math.floor(stone.z)}`;
      const cell = this.cells.get(key);
      if (cell) cell.push(stone);
      else this.cells.set(key, [stone]);
    }
    for (const mesh of this.children.filter((child): child is THREE.InstancedMesh => child instanceof THREE.InstancedMesh)) {
      this.remove(mesh);
      mesh.dispose(); // its instances; the shapes and the material are shared
    }
    this.build();
    return count;
  }

  // One InstancedMesh for each shape, holding every stone of that shape.
  private build(): void {
    this.shapes.forEach((shape, k) => {
      const stones = this.placed.filter((stone) => stone.shape === k);
      const mesh = new THREE.InstancedMesh(shape, this.material, stones.length);
      mesh.name = 'stones';
      stones.forEach((stone, i) => mesh.setMatrixAt(i, stone.matrix));
      mesh.computeBoundingSphere();
      mesh.castShadow = true;
      mesh.receiveShadow = true;
      this.add(mesh);
    });
  }
}
