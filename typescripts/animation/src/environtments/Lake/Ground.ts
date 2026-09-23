import * as THREE from 'three';
import type { Theme } from '../../theme';

// The lake's ground: a bowl-shaped bed under the water, deepest in the middle,
// a low bank rising from the waterline, and flat land around it out to the
// horizon. The shore wanders in and out around a circle, so the lake is not
// perfectly round. Units are meters and y = 0 is the water level.
//
// It is one mesh: rings around the center, cut into spokes. It is coloured per
// vertex: the theme's floor on land, fading toward its water colour with depth,
// so the deep middle looks deeper through the water.

export const LAKE_RADIUS = 9; // average distance from the center to the shore
export const DEPTH = 1.4; // of the bed at the center
export const LAND_HEIGHT = 0.3; // of the land above the water
export const BANK_WIDTH = 1.2; // from the waterline to the top of the bank
const LAND_RADIUS = 45; // where the land ends

// The shore's bumps around the average radius: [share of the radius, bumps
// around the lake, phase].
const SHORE_BUMPS: [share: number, bumps: number, phase: number][] = [
  [0.1, 2, 0.7],
  [0.06, 3, 2.1],
  [0.03, 5, 4.0],
];

// The farthest the shore gets from the center.
export const SHORE_MAX = LAKE_RADIUS * (1 + SHORE_BUMPS.reduce((sum, [share]) => sum + share, 0));

// Where the rings sit along each spoke: under water as shares of the way from
// the center to the shore, ashore as meters past the shore, then the edge of
// the land. They are close together where the ground bends (the waterline,
// the top of the bank) and far apart on the flat land.
const RINGS_UNDERWATER = [0, 0.3, 0.5, 0.65, 0.77, 0.86, 0.92, 0.96, 0.985, 1];
const RINGS_ASHORE = [0.06, 0.15, 0.3, 0.5, 0.75, 1, 1.2, 1.6, 2.5, 4.5, 8, 15, 25];
const SPOKES = 160;

// How far the deepest bed fades toward the water colour.
const DEEP_TINT = 0.85;

// The distance from the center to the shore in a direction, as an angle from
// +x toward +z.
export function shoreRadius(angle: number): number {
  let share = 1;
  for (const [bump, count, phase] of SHORE_BUMPS) share += bump * Math.sin(count * angle + phase);
  return LAKE_RADIUS * share;
}

// The ground's height at a distance r from the center, on a spoke whose shore
// is at `shore`. Under water the bed is a bowl; ashore the bank rises and
// levels out at the land's height. Both slope at the waterline, so the water
// meets the ground in a crisp line instead of a wide, flickering flat.
function heightAlong(r: number, shore: number): number {
  if (r <= shore) {
    const f = r / shore;
    return -DEPTH * (1 - f * f);
  }
  const past = Math.min(1, (r - shore) / BANK_WIDTH);
  return LAND_HEIGHT * Math.sin((Math.PI / 2) * past);
}

// The ground's height at a point of the lake (y = 0 is the water level).
export function groundHeight(x: number, z: number): number {
  return heightAlong(Math.hypot(x, z), shoreRadius(Math.atan2(z, x)));
}

export class Ground extends THREE.Group {
  constructor(theme: Theme) {
    super();
    this.name = 'ground';

    const land = new THREE.Color(theme.scene.floor);
    const water = new THREE.Color(theme.scene.water);
    const color = new THREE.Color();
    const positions: number[] = [];
    const colors: number[] = [];

    const rings = RINGS_UNDERWATER.length + RINGS_ASHORE.length + 1;
    for (let i = 0; i < rings; i++) {
      for (let j = 0; j < SPOKES; j++) {
        const angle = (j / SPOKES) * 2 * Math.PI;
        const shore = shoreRadius(angle);
        const r =
          i < RINGS_UNDERWATER.length
            ? RINGS_UNDERWATER[i] * shore
            : i < rings - 1
              ? shore + RINGS_ASHORE[i - RINGS_UNDERWATER.length]
              : LAND_RADIUS;
        const y = heightAlong(r, shore);
        positions.push(r * Math.cos(angle), y, r * Math.sin(angle));
        color.copy(land).lerp(water, DEEP_TINT * Math.max(0, -y / DEPTH));
        colors.push(color.r, color.g, color.b);
      }
    }

    // Each quad between two rings and two spokes, as two triangles facing up.
    // The last spoke joins the first, so the ground has no seam.
    const at = (i: number, j: number) => i * SPOKES + (j % SPOKES);
    const indices: number[] = [];
    for (let i = 0; i < rings - 1; i++) {
      for (let j = 0; j < SPOKES; j++) {
        indices.push(at(i, j), at(i, j + 1), at(i + 1, j));
        indices.push(at(i, j + 1), at(i + 1, j + 1), at(i + 1, j));
      }
    }

    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    geo.setIndex(indices);
    geo.computeVertexNormals();

    // The same matte finish as the stage's floor, which the lake replaces.
    const ground = new THREE.Mesh(geo, new THREE.MeshStandardMaterial({ vertexColors: true, roughness: 0.9 }));
    ground.receiveShadow = true;
    this.add(ground);
  }
}
