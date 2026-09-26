import * as THREE from 'three';
import { noise } from './noise';

// A sand texture made in code, for sandy ground (environtments/terrains/
// SandTerrain.ts): the ripples wind raises on dry sand, long wavy crests
// about 17 cm apart, each rising gently on the side the wind came from and
// dropping steeply on the side it blew to, high in some patches and all but
// gone in others, over fine grain. It is grey, so it shades whatever colour
// its material has: the theme's sand. Its brightness is its height too
// (crests lighter and raised, troughs darker, where the heavier grains
// gather), so the same texture serves as the bump map.
//
// One tile is 2 m x 2 m and repeats seamlessly both ways. The wind blew
// along its u: the crests run along v. Give a surface texture coordinates in
// meters, u the way the wind blew. Its average brightness (in linear light)
// is SAND_MEAN, so a material tinted 1 / SAND_MEAN keeps its colour on
// average.

export const SAND_TILE = 2; // meters a tile covers, each way
export const SAND_MEAN = 0.7; // the texture's average, in linear light
const SIZE = 512; // pixels a side, about 4 mm a pixel
const RIPPLES = 12; // across a tile, so 17 cm apart; a whole number, or the tile would show a seam
const LEE = 0.3; // the share of a ripple's width that is its steep side
const BASE = 0.78; // the sand's brightness before its ripples and grain
export const RIPPLE_SPAN = 0.26; // brightness from a ripple's trough to its crest, where they are highest
// The crests bend: the ripple's place is shifted by up to `amount` ripples
// by a slow noise, whose cells are longer along the crests than across.
const BENDS = [
  { across: 256, along: 128, amount: 0.6, seed: 21 },
  { across: 64, along: 32, amount: 0.12, seed: 22 },
];
const PATCHES = { cell: 128, from: 0.2, to: 0.7, seed: 23 }; // 50 cm patches: where the ripples are high, and where nearly flat
const LOWEST = 0.15; // how high the ripples still are where they are lowest, as a share
const GRAIN = [
  { cell: 1, amount: 0.05, seed: 24 }, // single grains, 4 mm
  { cell: 4, amount: 0.04, seed: 25 }, // small clumps, 1.6 cm
];

let cached: THREE.DataTexture | null = null;

// The texture, made once and shared by every material that uses it.
export function sandTexture(): THREE.DataTexture {
  if (cached) return cached;
  const values = new Float32Array(SIZE * SIZE);
  const periodic = (x: number, y: number, cellX: number, cellY: number, seed: number) =>
    noise(x / cellX, y / cellY, SIZE / cellX, seed, SIZE / cellY);
  for (let y = 0; y < SIZE; y++) {
    for (let x = 0; x < SIZE; x++) {
      let phase = (RIPPLES * x) / SIZE;
      for (const b of BENDS) phase += b.amount * (2 * periodic(x, y, b.across, b.along, b.seed) - 1);
      const p = PATCHES;
      const high = THREE.MathUtils.lerp(LOWEST, 1, THREE.MathUtils.smoothstep(periodic(x, y, p.cell, p.cell, p.seed), p.from, p.to));
      let v = BASE + RIPPLE_SPAN * high * (ripple(phase - Math.floor(phase)) - 0.5);
      for (const g of GRAIN) v += g.amount * (periodic(x, y, g.cell, g.cell, g.seed) - 0.5);
      values[y * SIZE + x] = v;
    }
  }

  // The brightness as an sRGB byte, scaled in linear light so the average is
  // SAND_MEAN, through tables of the 256 bytes, as the soil texture does.
  const bytes = new Uint8Array(values.length);
  for (let i = 0; i < values.length; i++) bytes[i] = Math.round(255 * THREE.MathUtils.clamp(values[i], 0, 1));
  const color = new THREE.Color();
  const linear = Array.from({ length: 256 }, (_, b) => color.setRGB(b / 255, 0, 0, THREE.SRGBColorSpace).r);
  let sum = 0;
  for (let i = 0; i < bytes.length; i++) sum += linear[bytes[i]];
  const scale = SAND_MEAN / (sum / bytes.length);
  const scaled = linear.map((l) => {
    const s = Math.min(1, scale * l);
    return Math.round(255 * color.setRGB(s, 0, 0).getRGB(color, THREE.SRGBColorSpace).r);
  });
  const data = new Uint8Array(SIZE * SIZE * 4);
  for (let i = 0; i < bytes.length; i++) {
    data[i * 4] = data[i * 4 + 1] = data[i * 4 + 2] = scaled[bytes[i]];
    data[i * 4 + 3] = 255;
  }

  const texture = new THREE.DataTexture(data, SIZE, SIZE, THREE.RGBAFormat);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.wrapS = texture.wrapT = THREE.RepeatWrapping;
  texture.repeat.set(1 / SAND_TILE, 1 / SAND_TILE); // coordinates in meters
  texture.magFilter = THREE.LinearFilter;
  texture.minFilter = THREE.LinearMipmapLinearFilter;
  texture.generateMipmaps = true;
  texture.anisotropy = 8; // keeps the ripples where the sand is seen edge-on
  texture.needsUpdate = true;
  cached = texture;
  return texture;
}

// A ripple's height across it, 0 in the trough and 1 at the crest, for its
// share of the way across (0..1): rising gently from a rounded trough to a
// sharp crest, then dropping steeply over LEE.
function ripple(t: number): number {
  const rise = 1 - LEE;
  return t < rise ? (t / rise) ** 1.4 : (1 - (t - rise) / LEE) ** 1.2;
}
