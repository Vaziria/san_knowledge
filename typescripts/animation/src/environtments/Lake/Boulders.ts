import * as THREE from 'three';
import { soilTexture } from '../../textures/soil';
import type { Theme } from '../../theme';
import { groundHeight, shoreRadius } from './Ground';

// Four big rocks round the lake, as the user asked: boulders 1.9-2.8 m across
// and up to 1.6 m out of the ground, far bigger than the lakeside stones
// (Stones.ts). One lies at the water's edge on the far shore with its foot in
// the water, one up the far bank, one stands in the water off the west shore
// and one on the land to the east. They are placed by hand, in view of the
// preview cameras, which look across the water toward -z, and well away from
// the landing, where figures stand.
//
// Each boulder is its own shape: a sphere swelled and dented by waves running
// across it (big ones for its bulk, small ones for a rough surface), then cut
// by a few planes into flat faces, like split stone, and drawn with flat
// faces. Its faces are the theme's stone colour, a little lighter or darker
// in broad patches and darker toward the ground. The soil texture's bump map
// (textures/soil.ts), laid on each face in meters, gives the stone its grain.
// A boulder is bedded into the ground up to its lowest side, so it sits and
// never floats where the ground slopes.

// Where each boulder lies: `angle` round the lake from +x toward +z, `out` m
// past the shoreline along it (under 0 in the water), its half width
// `radius`, half depth `depth` (as a share of the width) and half `height`,
// all in m, and the seed its shape grows from.
const BOULDERS = [
  { angle: -1.75, out: -0.35, radius: 1.3, depth: 0.8, height: 1.05, seed: 1 }, // the far shore, its foot in the water
  { angle: -2.2, out: 2.4, radius: 1, depth: 0.85, height: 0.9, seed: 2 }, // up the far bank
  { angle: 3.4, out: -1.7, radius: 0.95, depth: 0.75, height: 0.95, seed: 3 }, // in the water off the west shore
  { angle: 0.3, out: 3.6, radius: 1.4, depth: 0.7, height: 1.15, seed: 4 }, // on the land to the east
];
const DETAIL = 12; // subdivisions of the icosahedron: 3380 faces
const BULK = { waves: 5, depth: 0.12, frequency: [1.2, 2.6] as const }; // big swells and dents, as a share of the radius
const GRAIN = { waves: 12, depth: 0.025, frequency: [6, 12] as const }; // a rough surface
const CUTS = { count: 9, depth: [0.66, 0.92] as const }; // planes cut off each shape, where they sit as a share of the radius
const SINK = 0.3; // share of its height bedded below the lowest ground under it
const TILT = 0.12; // radians it may lean either way
const TONE = { patches: 3, amount: 0.07, jitter: 0.03 }; // broad lighter and darker patches, and each face on its own
const FOOT_SHADE = 0.75; // its colour where it meets the ground, times its top's
const FOOT_HEIGHT = 0.5; // m up from the ground over which that shade fades
const BUMP = 0.008; // m between the grain's highest and lowest
const SEED = 41;

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

function between(rand: () => number, [min, max]: readonly [number, number]): number {
  return min + (max - min) * rand();
}

function direction(rand: () => number): THREE.Vector3 {
  return new THREE.Vector3(rand() * 2 - 1, rand() * 2 - 1, rand() * 2 - 1).normalize();
}

// Waves running across a shape in random directions, each `depth / √count`
// high; their sum at a point on the unit sphere.
function waves(rand: () => number, set: { waves: number; depth: number; frequency: readonly [number, number] }) {
  const list = Array.from({ length: set.waves }, () => ({
    direction: direction(rand),
    frequency: between(rand, set.frequency),
    phase: 2 * Math.PI * rand(),
  }));
  const height = set.depth / Math.sqrt(set.waves);
  return (p: THREE.Vector3) => list.reduce((sum, w) => sum + height * Math.sin(w.frequency * p.dot(w.direction) + w.phase), 0);
}

// One boulder's shape, `size` its half width, height and depth in meters,
// with its origin in its middle. Non-indexed, so every face has its own
// corners: flat normals, a colour of its own and texture coordinates
// projected along its own facing.
function boulderGeometry(size: THREE.Vector3, rand: () => number): THREE.BufferGeometry {
  const geo = new THREE.IcosahedronGeometry(1, DETAIL);
  geo.deleteAttribute('uv');
  const bulk = waves(rand, BULK);
  const grain = waves(rand, GRAIN);
  const cuts = Array.from({ length: CUTS.count }, () => ({ normal: direction(rand), depth: between(rand, CUTS.depth) }));
  const position = geo.getAttribute('position');
  const p = new THREE.Vector3();
  for (let i = 0; i < position.count; i++) {
    p.fromBufferAttribute(position, i);
    p.multiplyScalar(1 + bulk(p) + grain(p));
    // Every point beyond a cutting plane is pressed back onto it.
    for (const cut of cuts) {
      const beyond = p.dot(cut.normal) - cut.depth;
      if (beyond > 0) p.addScaledVector(cut.normal, -beyond);
    }
    position.setXYZ(i, p.x * size.x, p.y * size.y, p.z * size.z);
  }
  geo.computeVertexNormals(); // flat, since the corners aren't shared

  // Each face's texture coordinates, in meters, looking along the axis it
  // faces most, so the grain is never stretched.
  const normal = geo.getAttribute('normal');
  const uvs = new Float32Array(position.count * 2);
  for (let i = 0; i < position.count; i++) {
    const face = i - (i % 3);
    const nx = Math.abs(normal.getX(face));
    const ny = Math.abs(normal.getY(face));
    const nz = Math.abs(normal.getZ(face));
    const [u, v] =
      ny >= nx && ny >= nz
        ? [position.getX(i), position.getZ(i)]
        : nx >= nz
          ? [position.getZ(i), position.getY(i)]
          : [position.getX(i), position.getY(i)];
    uvs[i * 2] = u;
    uvs[i * 2 + 1] = v;
  }
  geo.setAttribute('uv', new THREE.BufferAttribute(uvs, 2));
  return geo;
}

// A boulder's colours: the stone colour in broad lighter and darker patches,
// each face a touch different, darker toward the ground. `bottom` is the
// ground's height under it, in the boulder's own coordinates.
function colorFaces(geo: THREE.BufferGeometry, stone: THREE.Color, bottom: number, rand: () => number): void {
  const patches = waves(rand, { waves: TONE.patches, depth: TONE.amount * Math.sqrt(TONE.patches), frequency: [1.5, 3] });
  const position = geo.getAttribute('position');
  const colors = new Float32Array(position.count * 3);
  const middle = new THREE.Vector3();
  const color = new THREE.Color();
  for (let face = 0; face < position.count; face += 3) {
    middle.set(0, 0, 0);
    for (let k = 0; k < 3; k++) middle.add(new THREE.Vector3().fromBufferAttribute(position, face + k));
    middle.divideScalar(3);
    const foot = THREE.MathUtils.lerp(FOOT_SHADE, 1, THREE.MathUtils.smoothstep(middle.y - bottom, 0, FOOT_HEIGHT));
    const tone = (1 + patches(middle) + TONE.jitter * (2 * rand() - 1)) * foot;
    color.copy(stone).multiplyScalar(tone);
    for (let k = 0; k < 3; k++) colors.set([color.r, color.g, color.b], (face + k) * 3);
  }
  geo.setAttribute('color', new THREE.BufferAttribute(colors, 3));
}

export class Boulders extends THREE.Group {
  // Where each boulder meets the ground, as a circle round its middle.
  readonly footprints: { x: number; z: number; radius: number }[] = [];

  constructor(theme: Theme) {
    super();
    this.name = 'boulders';
    const rand = random(SEED);
    const stone = new THREE.Color(theme.scene.stone);
    // The same matte finish as the stones, with flat faces and the grain.
    const material = new THREE.MeshStandardMaterial({
      vertexColors: true,
      roughness: 0.9,
      flatShading: true,
      bumpMap: soilTexture(),
      bumpScale: BUMP,
    });

    for (const b of BOULDERS) {
      const shape = random(SEED + b.seed);
      const r = shoreRadius(b.angle) + b.out;
      const x = r * Math.cos(b.angle);
      const z = r * Math.sin(b.angle);
      const size = new THREE.Vector3(b.radius, b.height, b.radius * b.depth);
      // Bedded up to the lowest ground under it, so no side floats.
      let lowest = groundHeight(x, z);
      for (let k = 0; k < 8; k++) {
        const a = (k / 8) * 2 * Math.PI;
        lowest = Math.min(lowest, groundHeight(x + 0.8 * b.radius * Math.cos(a), z + 0.8 * b.radius * Math.sin(a)));
      }
      const y = lowest + b.height * (1 - 2 * SINK);

      const geo = boulderGeometry(size, shape);
      colorFaces(geo, stone, groundHeight(x, z) - y, shape);
      const boulder = new THREE.Mesh(geo, material);
      boulder.name = 'boulder';
      boulder.position.set(x, y, z);
      boulder.rotation.set((rand() * 2 - 1) * TILT, rand() * 2 * Math.PI, (rand() * 2 - 1) * TILT);
      boulder.castShadow = true;
      boulder.receiveShadow = true;
      this.add(boulder);
      this.footprints.push({ x, z, radius: b.radius });
    }
  }

  // Whether a point of the lake is within `gap` m of a boulder's footprint
  // (inside it for a gap under 0), so that trees and grass keep off them.
  near(x: number, z: number, gap: number): boolean {
    return this.footprints.some((f) => Math.hypot(x - f.x, z - f.z) < f.radius + gap);
  }
}
