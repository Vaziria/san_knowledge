import * as THREE from 'three';
import { SOIL_MEAN, soilTexture } from '../../textures/soil';
import type { Theme } from '../../theme';
import { hazeEdge } from '../haze';

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
// environment) beyond the bank, where the meadow grows (Meadow.ts). Over the
// colours lies the soil texture (textures/soil.ts): lumpy earth, grit,
// pebbles and cracks, laid flat in meters, which also roughens the surface
// (its bump map), so the bank and the shallows read as earth up close. It
// shows fully on the bank and fades out with depth, since the deep bed is
// seen through the water as one colour, and on the lawn, whose earth is
// hidden by grass; how much it shows is kept per vertex (`soil`).
//
// The swamps by the lake (Swamps.ts) are dug into the land and lie over it:
// the mesh follows their hollows (`cover.dig`) and leaves out its triangles
// wherever a swamp covers at least half the ground (`cover.covers`,
// COVERED). Its rings, 40 cm apart, would cut straight across the foot of a
// dug slope, up to about 5 cm above it, through the swamp lying 3 cm over
// the land; and across the land's own hollows, up to 2 cm above them, at the
// swamp's edge, where it lies lower.

export const LAKE_RADIUS = 9; // average distance from the center to the shore
export const DEPTH = 1.4; // of the bed at the center
export const LAND_HEIGHT = 0.3; // of the land above the water at the top of the bank, before its humps
export const BANK_WIDTH = 1.2; // from the waterline to the top of the bank
// Where the land ends, behind the border's cliffs, mist and tall grass
// (../Border.ts). Over its last meters it fades into the sky's colour
// (../haze.ts), so it meets the sky without a line.
export const LAND_RADIUS = 45;

// The land's humps and hollows: a few waves running across it in different
// directions, each `height` m high either way and `length` m from crest to
// crest. They grow from nothing at the top of the bank over RISE, so the bank
// still meets the water in a clean line, and die away round the landing
// (CLEAR), where figures stand.
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
// m round the landing kept level, by default: room for one figure. A lake
// built for more (the lake meeting's animals) keeps a wider clearing level,
// mown and free of stones and meadow plants (LakeOptions.clear in Lake.ts).
export const CLEAR = 2.2;
const FLAT_OVER = 1.8; // m beyond the clearing over which the humps come back
const LOWEST = 0.08; // m above the water: the land never dips lower, even where every hollow meets
const LAWN = { shade: 0.75, from: -0.3, over: 0.8 }; // the lawn colour times the grass colour, and m past the top of the bank where it starts and takes over, as the sward does
const COVERED = 0.5; // of the ground a swamp covers, at all three corners of a triangle, where the mesh is left out: from there in the swamp's own colours show

// What lies over the land and is dug into it: the swamps (Swamps.ts). How
// many meters the land is dug at a point, and how much of the ground there
// is covered, 0 to 1.
export interface Cover {
  dig(x: number, z: number): number;
  covers(x: number, z: number): number;
}

// The lawn beyond the bank, before its mottling: the grass colour a shade
// darker. A swamp lying on it fades to it at its edge (Swamps.ts).
export function lawnColor(theme: Theme): THREE.Color {
  return new THREE.Color(theme.scene.grass).multiplyScalar(LAWN.shade);
}
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
// the center to the shore, ashore as meters past the shore, then OUTER rings
// evenly out to the edge of the land, the last on it. They are close together
// where the ground bends (the waterline, the top of the bank), every 40 cm
// over the humps near the lake, and further apart out among the hills; the
// outer ones keep the hills under the border's tall grass (../Border.ts)
// where the grass stands.
const RINGS_UNDERWATER = [0, 0.3, 0.5, 0.65, 0.77, 0.86, 0.92, 0.96, 0.985, 1];
const RINGS_ASHORE = ringsAshore();
const OUTER = 6;
const SPOKES = 320;

function ringsAshore(): number[] {
  const rings = [0.06, 0.15, 0.3, 0.5, 0.75, 1, 1.2];
  for (let d = 1.6; d <= 10; d += 0.4) rings.push(d);
  for (let step = 0.45, d = 10 + step; d < 30; step *= 1.12, d += step) rings.push(d);
  return rings;
}

// How far the deepest bed fades toward the water colour.
const DEEP_TINT = 0.85;

// How high the soil texture's brightest bump stands above its darkest, in
// meters: a pebble's top above the bottom of a crack.
const BUMP = 0.012;
// How much of the soil texture shows, from 0 (the plain colour) to 1: all of
// it on the bank, fading to `deep` over the first `over` m under the water,
// and to `lawn` where the lawn takes over.
const SOIL = { deep: 0.15, over: 0.5, lawn: 0.3 };

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
// top of the bank, where the land is kept level (CLEAR), so a figure up to
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
// past the top of the bank; within `clear` m of the landing the land is level.
function unevenness(x: number, z: number, past: number, clear: number): number {
  if (past <= 0) return 0;
  const level = smoothstep(clear, clear + FLAT_OVER, Math.hypot(x - LANDING.x, z - LANDING.z));
  const humps = waves(x, z, ROLLS) * smoothstep(0, RISE, past) * level;
  const hills = HILLS.reduce((sum, h) => sum + h.height * 0.5 * (1 + Math.sin(((2 * Math.PI) / h.length) * (x * Math.cos(h.angle) + z * Math.sin(h.angle)) + h.phase)), 0);
  return humps + hills * smoothstep(HILLS_FROM, HILLS_FROM + HILLS_OVER, past);
}

// The ground's height at a point of the lake (y = 0 is the water level), on
// a lake whose landing is level for `clear` m round it.
export function groundHeight(x: number, z: number, clear = CLEAR): number {
  const r = Math.hypot(x, z);
  const shore = shoreRadius(Math.atan2(z, x));
  const base = heightAlong(r, shore);
  const past = r - shore - BANK_WIDTH;
  return past <= 0 ? base : Math.max(LOWEST, base + unevenness(x, z, past, clear));
}

export class Ground extends THREE.Group {
  // `sky` is the colour the land fades into at its end: the lake's sky at
  // the horizon, which darkens at night (DayNight.ts), or the theme's
  // background. `cover` is what lies over the land and is dug into it (the
  // swamps), if anything.
  constructor(theme: Theme, clear = CLEAR, sky: THREE.ColorRepresentation = theme.scene.background, cover?: Cover) {
    super();
    this.name = 'ground';

    const land = new THREE.Color(theme.scene.floor);
    const water = new THREE.Color(theme.scene.water);
    const lawn = lawnColor(theme);
    const color = new THREE.Color();
    const positions: number[] = [];
    const colors: number[] = [];
    const uvs: number[] = [];
    const soils: number[] = [];
    const covered: boolean[] = [];

    const ashore = RINGS_UNDERWATER.length + RINGS_ASHORE.length;
    const rings = ashore + OUTER;
    const lastAshore = RINGS_ASHORE[RINGS_ASHORE.length - 1];
    for (let i = 0; i < rings; i++) {
      for (let j = 0; j < SPOKES; j++) {
        const angle = (j / SPOKES) * 2 * Math.PI;
        const shore = shoreRadius(angle);
        const r =
          i < RINGS_UNDERWATER.length
            ? RINGS_UNDERWATER[i] * shore
            : i < ashore
              ? shore + RINGS_ASHORE[i - RINGS_UNDERWATER.length]
              : THREE.MathUtils.lerp(shore + lastAshore, LAND_RADIUS, (i - ashore + 1) / OUTER);
        const x = r * Math.cos(angle);
        const z = r * Math.sin(angle);
        const y = groundHeight(x, z, clear) - (cover?.dig(x, z) ?? 0);
        positions.push(x, y, z);
        covered.push(cover !== undefined && cover.covers(x, z) >= COVERED);
        uvs.push(x, z); // the soil texture, laid flat in meters
        color.copy(land).lerp(water, DEEP_TINT * Math.max(0, -y / DEPTH));
        const mottled = lawn.clone().multiplyScalar(1 + MOTTLE * waves(x, z, PATCHES));
        const lawnShare = smoothstep(LAWN.from, LAWN.from + LAWN.over, r - shore - BANK_WIDTH);
        color.lerp(mottled, lawnShare);
        colors.push(color.r, color.g, color.b);
        const soil = THREE.MathUtils.lerp(1, SOIL.deep, smoothstep(0, SOIL.over, -y));
        soils.push(THREE.MathUtils.lerp(soil, SOIL.lawn, lawnShare));
      }
    }

    // Each quad between two rings and two spokes, as two triangles facing up,
    // less those a swamp covers at all three corners. The last spoke joins
    // the first, so the ground has no seam.
    const at = (i: number, j: number) => i * SPOKES + (j % SPOKES);
    const indices: number[] = [];
    const triangle = (a: number, b: number, c: number) => {
      if (!(covered[a] && covered[b] && covered[c])) indices.push(a, b, c);
    };
    for (let i = 0; i < rings - 1; i++) {
      for (let j = 0; j < SPOKES; j++) {
        triangle(at(i, j), at(i, j + 1), at(i + 1, j));
        triangle(at(i, j + 1), at(i + 1, j + 1), at(i + 1, j));
      }
    }

    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
    geo.setAttribute('soil', new THREE.Float32BufferAttribute(soils, 1));
    geo.setIndex(indices);
    geo.computeVertexNormals();

    // The same matte finish as the stage's floor, which the lake replaces,
    // with the soil texture over its colours.
    const soil = soilTexture();
    const material = new THREE.MeshStandardMaterial({ vertexColors: true, roughness: 0.9, map: soil, bumpMap: soil, bumpScale: BUMP });
    showSoil(material);
    hazeEdge(material, sky, LAND_RADIUS);
    const ground = new THREE.Mesh(geo, material);
    ground.receiveShadow = true;
    this.add(ground);
  }
}

// Makes the soil texture show only as much as each vertex's `soil` says: its
// colour is mixed from its plain average (SOIL_MEAN) toward the texture, and
// divided by that average, so the ground keeps its colours on average
// wherever the texture shows; the bumps shrink with it.
function showSoil(material: THREE.MeshStandardMaterial): void {
  material.onBeforeCompile = (shader) => {
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', '#include <common>\nattribute float soil;\nvarying float vSoil;')
      .replace('#include <begin_vertex>', '#include <begin_vertex>\nvSoil = soil;');
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', `#include <common>\nvarying float vSoil;\n#define SOIL_MEAN ${SOIL_MEAN.toFixed(4)}`)
      .replace('#include <map_fragment>', 'diffuseColor.rgb *= mix(vec3(SOIL_MEAN), texture2D(map, vMapUv).rgb, vSoil) / SOIL_MEAN;')
      .replace('#include <bumpmap_pars_fragment>', THREE.ShaderChunk.bumpmap_pars_fragment.replaceAll('bumpScale *', 'bumpScale * vSoil *'));
  };
  material.customProgramCacheKey = () => 'soil';
}
