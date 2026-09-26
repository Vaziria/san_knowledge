import * as THREE from 'three';
import { wobble } from '../../figures/objects/parts';
import type { Theme } from '../../theme';

export { between, seededRandom } from '../../figures/Tree/parts';
export { wobble };

// What the terrains share. A terrain is a kind of ground (grass, mud, water,
// sand) that an environment lays over its land: the environment shapes the
// land (hills, a lake's hollow) and says where each terrain lies, and the
// terrain covers that land with its own surface and what belongs on it
// (tufts of grass, puddles, waves, ripples). So environments are put
// together from terrains, as figures are from parts.
//
// A terrain lies a little above the land (LIFT) wherever it is all that
// terrain, so the land never shows through it, and sinks just under the
// land (SINK) at its edge, where it meets the land in a line with no gap.
// Where the land is drawn by the environment around it, that line is its
// edge; where two terrains overlap, the one lying higher shows.

export interface TerrainOptions {
  theme?: Theme;
  // Another seed, another stretch of the same kind of ground. Each terrain
  // has its own default.
  seed?: number;
  // m across its footprint along x, and along z: all of it lies within
  // that box, centred on its origin. Each terrain has its own default
  // (its SIZE).
  width?: number;
  depth?: number;
  // The land's height at a point, in the terrain's own coordinates (from
  // its origin); flat when left out.
  heightAt?: (x: number, z: number) => number;
  // How much of the ground at a point is this terrain: 1 where it is all
  // this terrain, 0 where there is none, and between at a soft edge. By
  // default an uneven oval filling the footprint (ovalMask).
  mask?: (x: number, z: number) => number;
}

// What every terrain answers, so an environment can place things on it.
export interface Terrain extends THREE.Object3D {
  readonly width: number;
  readonly depth: number;
  // The height of its top at a point (its own coordinates): the ground a
  // figure would stand on, or for water its surface now.
  heightAt(x: number, z: number): number;
  // How much of the ground at a point is this terrain (its mask), so that
  // other things can keep off it.
  covers(x: number, z: number): number;
  // Moves it on by delta seconds (the water's waves); call once per frame.
  update(delta: number): void;
}

export interface Footprint {
  width: number;
  depth: number;
}

export const LIFT = 0.012; // m a terrain lies above the land where it is all that terrain
export const SINK = 0.015; // m under the land at its outer edge
export const EDGE = 0.5; // m over which the default outline's edge goes from none of the terrain to all of it

// The footprint a terrain is built to: the options, and its default for any
// left out.
export function footprintOf(options: TerrainOptions, size: Footprint): Footprint {
  return { width: options.width ?? size.width, depth: options.depth ?? size.depth };
}

// How uneven the default outline is: its radius swells and dips by up to
// `amount` in about `lumps` waves round it.
const OUTLINE = { lumps: 3, amount: 0.12, samples: 360 };

// The default mask: an oval that swells and dips round it, stretched and
// moved so its outline fills the footprint exactly (width by depth, centred
// on the origin), 0 on the outline and outside it, rising to 1 over `edge`
// m inside. Each seed gives another outline.
export function ovalMask(size: Footprint, seed: number, edge = EDGE): (x: number, z: number) => number {
  const inside = ovalInside(size, seed);
  return (x, z) => THREE.MathUtils.smoothstep(inside(x, z), 0, edge);
}

// How far a point is inside ovalMask()'s outline, in meters, about (below 0
// outside it): measured from the outline toward its middle, along the line
// through the middle. An environment digging a hollow for a terrain's water
// (Lake/Swamps.ts) keeps it this far in.
export function ovalInside(size: Footprint, seed: number): (x: number, z: number) => number {
  const phase = 37.1 * seed;
  const swell = (angle: number) =>
    1 + OUTLINE.amount * wobble(phase + OUTLINE.lumps * Math.cos(angle), OUTLINE.lumps * Math.sin(angle), seed);
  let minX = Infinity;
  let maxX = -Infinity;
  let minZ = Infinity;
  let maxZ = -Infinity;
  for (let i = 0; i < OUTLINE.samples; i++) {
    const angle = (2 * Math.PI * i) / OUTLINE.samples;
    const r = swell(angle);
    minX = Math.min(minX, r * Math.cos(angle));
    maxX = Math.max(maxX, r * Math.cos(angle));
    minZ = Math.min(minZ, r * Math.sin(angle));
    maxZ = Math.max(maxZ, r * Math.sin(angle));
  }
  const scaleX = size.width / (maxX - minX);
  const scaleZ = size.depth / (maxZ - minZ);
  const shiftX = -((maxX + minX) / 2) * scaleX;
  const shiftZ = -((maxZ + minZ) / 2) * scaleZ;
  return (x, z) => {
    // Where the point lies on the oval before it was stretched, and about
    // how far in from the outline that is, in meters.
    const u = (x - shiftX) / scaleX;
    const v = (z - shiftZ) / scaleZ;
    const angle = Math.atan2(v, u);
    return (swell(angle) - Math.hypot(u, v)) * Math.hypot(scaleX * Math.cos(angle), scaleZ * Math.sin(angle));
  };
}

// How a terrain's surface looks at one of its points, filled in by the
// terrain for each point (surfaceGeometry).
export interface Paint {
  lift: number; // m above the land (under it where below 0)
  color: THREE.Color;
  soil: number; // how much of the material's texture shows, 0 (none) to 1 (all)
  wet: number; // 0 dry to 1 wet: smoother to the light, with a sheen
  u: number; // texture coordinates, in meters; the point's x and z unless changed
  v: number;
}

// A terrain's surface: a grid of points `cell` m apart over its footprint,
// each on the land (`heightAt`) plus what `paint` says, with its colour,
// texture coordinates and how much of its texture shows (the `soil`
// attribute) and how wet it is (`wet`), for surfaceMaterial(). Cells the
// mask leaves out entirely are left out.
export function surfaceGeometry(
  size: Footprint,
  cell: number,
  mask: (x: number, z: number) => number,
  heightAt: (x: number, z: number) => number,
  paint: (x: number, z: number, cover: number, out: Paint) => void,
): THREE.BufferGeometry {
  const columns = Math.max(1, Math.round(size.width / cell));
  const rows = Math.max(1, Math.round(size.depth / cell));
  const positions: number[] = [];
  const colors: number[] = [];
  const uvs: number[] = [];
  const soils: number[] = [];
  const wets: number[] = [];
  const covers: number[] = [];
  const out: Paint = { lift: 0, color: new THREE.Color(), soil: 1, wet: 0, u: 0, v: 0 };
  for (let j = 0; j <= rows; j++) {
    for (let i = 0; i <= columns; i++) {
      const x = size.width * (i / columns - 0.5);
      const z = size.depth * (j / rows - 0.5);
      const cover = mask(x, z);
      out.lift = 0;
      out.color.setRGB(1, 1, 1);
      out.soil = 1;
      out.wet = 0;
      out.u = x;
      out.v = z;
      paint(x, z, cover, out);
      positions.push(x, heightAt(x, z) + out.lift, z);
      colors.push(out.color.r, out.color.g, out.color.b);
      uvs.push(out.u, out.v);
      soils.push(out.soil);
      wets.push(out.wet);
      covers.push(cover);
    }
  }
  // Two triangles a cell, facing up: (a, c, b) and (b, c, d), with a and b
  // along x and c and d the next row along z.
  const indices: number[] = [];
  const at = (i: number, j: number) => j * (columns + 1) + i;
  for (let j = 0; j < rows; j++) {
    for (let i = 0; i < columns; i++) {
      const a = at(i, j);
      const b = at(i + 1, j);
      const c = at(i, j + 1);
      const d = at(i + 1, j + 1);
      if (covers[a] <= 0 && covers[b] <= 0 && covers[c] <= 0 && covers[d] <= 0) continue;
      indices.push(a, c, b, b, c, d);
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
  geo.setAttribute('soil', new THREE.Float32BufferAttribute(soils, 1));
  geo.setAttribute('wet', new THREE.Float32BufferAttribute(wets, 1));
  geo.setIndex(indices);
  geo.computeVertexNormals();
  return geo;
}

export interface SurfaceLook {
  texture: THREE.Texture; // grey, laid in meters (the soil's, the sand's)
  mean: number; // its average brightness in linear light (SOIL_MEAN, SAND_MEAN)
  bump: number; // m of height for a unit of the texture's brightness, where all of it shows
  roughness: number; // where it is dry
  wetRoughness?: number; // where it is wet: lower, for a sheen; as dry by default
}

// The material of a terrain's surface: its vertex colours, with the texture
// over them showing as much as each point's `soil` says (mixed from its plain
// average toward the texture and divided by that average, so the colours stay
// the same on average) and its bumps shrinking with it, as on the lake's
// ground; and smoother to the light where `wet` says so.
export function surfaceMaterial(look: SurfaceLook): THREE.MeshStandardMaterial {
  const { texture, mean, bump, roughness } = look;
  const wet = ((look.wetRoughness ?? roughness) / roughness).toFixed(4);
  const material = new THREE.MeshStandardMaterial({ vertexColors: true, roughness, map: texture, bumpMap: texture, bumpScale: bump });
  material.onBeforeCompile = (shader) => {
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', '#include <common>\nattribute float soil;\nattribute float wet;\nvarying float vSoil;\nvarying float vWet;')
      .replace('#include <begin_vertex>', '#include <begin_vertex>\nvSoil = soil;\nvWet = wet;');
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', `#include <common>\nvarying float vSoil;\nvarying float vWet;\n#define TEXTURE_MEAN ${mean.toFixed(4)}`)
      .replace('#include <map_fragment>', 'diffuseColor.rgb *= mix(vec3(TEXTURE_MEAN), texture2D(map, vMapUv).rgb, vSoil) / TEXTURE_MEAN;')
      .replace('#include <bumpmap_pars_fragment>', THREE.ShaderChunk.bumpmap_pars_fragment.replaceAll('bumpScale *', 'bumpScale * vSoil *'))
      .replace('#include <roughnessmap_fragment>', `#include <roughnessmap_fragment>\nroughnessFactor *= mix(1.0, ${wet}, vWet);`);
  };
  const key = `terrain-${mean.toFixed(4)}-${wet}`;
  material.customProgramCacheKey = () => key;
  return material;
}

// The way the land faces at a point: up, tipped by its slope.
export function landNormal(heightAt: (x: number, z: number) => number, x: number, z: number): THREE.Vector3 {
  const e = 0.05;
  const dx = (heightAt(x + e, z) - heightAt(x - e, z)) / (2 * e);
  const dz = (heightAt(x, z + e) - heightAt(x, z - e)) / (2 * e);
  return new THREE.Vector3(-dx, 1, -dz).normalize();
}

// A patch of mottling from -1 to 1: broad blotches about `size` m across,
// with smaller ones in them.
export function mottle(x: number, z: number, size: number, seed: number): number {
  return 0.65 * wobble(x / size, z / size, seed) + 0.35 * wobble((2.3 * x) / size, (2.3 * z) / size, seed + 1);
}
