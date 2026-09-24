import * as THREE from 'three';
import type { Theme } from '../../theme';

// The lake's ground: a bowl-shaped bed under the water, deepest in the middle,
// a low bank rising from the waterline, and uneven land around it out to the
// horizon: gentle humps and hollows, rolling into low hills far out. The
// shore wanders in and out around a circle, so the lake is not perfectly
// round. Units are meters and y = 0 is the water level.
//
// It is one mesh: rings around the center, cut into spokes. It is coloured per
// vertex: bare earth in the theme's floor colour on the bank, fading toward
// its water colour with depth, so the deep middle looks deeper through the
// water, and toward a lawn (its grass colour, a shade darker, as in the grass
// environment) beyond the bank, where the meadow grows (Meadow.ts).

export const LAKE_RADIUS = 9; // average distance from the center to the shore
export const DEPTH = 1.4; // of the bed at the center
export const LAND_HEIGHT = 0.3; // of the land above the water at the top of the bank, before its humps
export const BANK_WIDTH = 1.2; // from the waterline to the top of the bank
const LAND_RADIUS = 45; // where the land ends

// The land's humps and hollows: a few waves running across it in different
// directions, each `height` m high either way and `length` m from crest to
// crest. They grow from nothing at the top of the bank over RISE, so the bank
// still meets the water in a clean line, and die away round the landing
// (FLAT), where figures stand.
const ROLLS = [
  { height: 0.12, length: 9, angle: 0.4, phase: 1.1 },
  { height: 0.1, length: 6.3, angle: 1.9, phase: 0.4 },
  { height: 0.07, length: 4.1, angle: 2.9, phase: 2.6 },
  { height: 0.04, length: 2.7, angle: 4.3, phase: 5 },
];
// Far out the land rolls into low hills, only ever rising, `from` m past the
// shore and growing to their full height over `over` m more.
const HILLS = [
  { height: 0.6, length: 23, angle: 0.9, phase: 0.3 },
  { height: 0.45, length: 15, angle: 2.6, phase: 4.1 },
];
const HILLS_FROM = 9;
const HILLS_OVER = 18;
const RISE = 1.5; // m past the top of the bank over which the humps grow to full height
const FLAT = { radius: 2.2, over: 1.8 }; // m round the landing kept level, and over which the humps come back
const LOWEST = 0.08; // m above the water: the land never dips lower, even where every hollow meets
const LAWN = { shade: 0.75, from: -0.3, over: 0.8 }; // the lawn colour times the grass colour, and m past the top of the bank where it starts and takes over, as the sward does
// The lawn is mottled lighter and darker in patches about a meter across, as
// a meadow is, by up to MOTTLE either way; the painterly filter turns them
// into strokes of grass.
const MOTTLE = 0.12;
const PATCHES = [
  { height: 0.5, length: 2.3, angle: 0.7, phase: 0.9 },
  { height: 0.3, length: 1.6, angle: 2.2, phase: 3.1 },
  { height: 0.2, length: 1.1, angle: 3.9, phase: 1.7 },
];

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
// the top of the bank), every 40 cm over the humps near the lake, and further
// apart out among the hills.
const RINGS_UNDERWATER = [0, 0.3, 0.5, 0.65, 0.77, 0.86, 0.92, 0.96, 0.985, 1];
const RINGS_ASHORE = ringsAshore();
const SPOKES = 320;

function ringsAshore(): number[] {
  const rings = [0.06, 0.15, 0.3, 0.5, 0.75, 1, 1.2];
  for (let d = 1.6; d <= 10; d += 0.4) rings.push(d);
  for (let step = 0.45, d = 10 + step; d < 30; step *= 1.12, d += step) rings.push(d);
  return rings;
}

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

// Where a figure stands on the lake's land: on the +z shore, 1.5 m past the
// top of the bank, where the land is kept level (FLAT), so a figure up to
// about 3 m across stands on flat land with the water behind it (toward -z),
// where the preview cameras look.
export const LANDING = new THREE.Vector3(0, LAND_HEIGHT, shoreRadius(Math.PI / 2) + BANK_WIDTH + 1.5);

// 0 at a and below, 1 at b and above, easing in and out between.
function smoothstep(a: number, b: number, x: number): number {
  const t = THREE.MathUtils.clamp((x - a) / (b - a), 0, 1);
  return t * t * (3 - 2 * t);
}

// Waves running across the land at a point: each rises and falls `height`
// either way, along its own direction.
function waves(x: number, z: number, list: typeof ROLLS): number {
  let sum = 0;
  for (const w of list) {
    const along = x * Math.cos(w.angle) + z * Math.sin(w.angle);
    sum += w.height * Math.sin(((2 * Math.PI) / w.length) * along + w.phase);
  }
  return sum;
}

// How far the land at a point rises above or dips below LAND_HEIGHT: its
// humps and hollows, and the hills far out. `past` is how far the point is
// past the top of the bank.
function unevenness(x: number, z: number, past: number): number {
  if (past <= 0) return 0;
  const level = smoothstep(FLAT.radius, FLAT.radius + FLAT.over, Math.hypot(x - LANDING.x, z - LANDING.z));
  const humps = waves(x, z, ROLLS) * smoothstep(0, RISE, past) * level;
  const hills = HILLS.reduce((sum, h) => sum + h.height * 0.5 * (1 + Math.sin(((2 * Math.PI) / h.length) * (x * Math.cos(h.angle) + z * Math.sin(h.angle)) + h.phase)), 0);
  return humps + hills * smoothstep(HILLS_FROM, HILLS_FROM + HILLS_OVER, past);
}

// The ground's height at a point of the lake (y = 0 is the water level).
export function groundHeight(x: number, z: number): number {
  const r = Math.hypot(x, z);
  const shore = shoreRadius(Math.atan2(z, x));
  const base = heightAlong(r, shore);
  const past = r - shore - BANK_WIDTH;
  return past <= 0 ? base : Math.max(LOWEST, base + unevenness(x, z, past));
}

export class Ground extends THREE.Group {
  constructor(theme: Theme) {
    super();
    this.name = 'ground';

    const land = new THREE.Color(theme.scene.floor);
    const water = new THREE.Color(theme.scene.water);
    const lawn = new THREE.Color(theme.scene.grass).multiplyScalar(LAWN.shade);
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
        const x = r * Math.cos(angle);
        const z = r * Math.sin(angle);
        const y = groundHeight(x, z);
        positions.push(x, y, z);
        color.copy(land).lerp(water, DEEP_TINT * Math.max(0, -y / DEPTH));
        const mottled = lawn.clone().multiplyScalar(1 + MOTTLE * waves(x, z, PATCHES));
        color.lerp(mottled, smoothstep(LAWN.from, LAWN.from + LAWN.over, r - shore - BANK_WIDTH));
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
