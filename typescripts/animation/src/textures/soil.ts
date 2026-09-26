import * as THREE from 'three';
import { hash, noise } from './noise';

// A soil texture made in code, for bare ground such as the lake's bank and
// bed: lumpy earth with crumbs and fine grit, pebbles and a few bigger stones
// lying in it, each with a dark rim where it sits in the earth, and thin
// cracks in patches where the earth has dried. It is grey, so it shades
// whatever colour its material has: the ground's theme colours. Its
// brightness is its height too (pebbles lighter and raised, cracks darker and
// sunk), so the same texture serves as the bump map and the sun picks out the
// lumps.
//
// One tile is 2 m x 2 m. Give a surface texture coordinates in meters (the
// ground's x and z) and the pebbles come out their real size. The tile
// repeats seamlessly both ways. Its average brightness (in linear light) is
// SOIL_MEAN, so a material tinted 1 / SOIL_MEAN keeps its colour on average.

export const SOIL_TILE = 2; // meters a tile covers, each way
export const SOIL_MEAN = 0.7; // the texture's average, in linear light
const SIZE = 1024; // pixels a side, about 2 mm a pixel
const EARTH = 0.8; // the earth's brightness before its lumps
const LUMPS = [
  { cell: 64, amount: 0.14, seed: 11 }, // clods, 13 cm
  { cell: 16, amount: 0.1, seed: 12 }, // crumbs, 3 cm
  { cell: 2, amount: 0.12, seed: 13 }, // grit, 4 mm
];
// Cracks run where a slow noise crosses its middle, only in patches.
const CRACK = { cell: 128, width: 0.01, depth: 0.3, seed: 14 };
const CRACK_PATCH = { cell: 256, from: 0.5, to: 0.7, seed: 15 }; // 50 cm patches of dried earth
// Pebbles lie one to a cell at most, `chance` of the cells, their radii in
// pixels (small ones more common).
const PEBBLES = [
  { cell: 32, chance: 0.35, radius: [2.5, 7], seed: 16 }, // 5-14 mm
  { cell: 128, chance: 0.4, radius: [8, 18], seed: 17 }, // 1.6-3.6 cm
];
const PEBBLE_TONE = [0.86, 1] as const; // each pebble's brightness at its edge
const PEBBLE_DOME = 0.06; // how much lighter its top is than its edge
const RIM = { width: 0.35, depth: 0.12 }; // the dark ring round a pebble, as a share of its radius, and how dark

let cached: THREE.DataTexture | null = null;

// The texture, made once and shared by every material that uses it.
export function soilTexture(): THREE.DataTexture {
  if (cached) return cached;
  const values = new Float32Array(SIZE * SIZE);
  for (let y = 0; y < SIZE; y++) {
    for (let x = 0; x < SIZE; x++) {
      let v = EARTH;
      for (const l of LUMPS) v += l.amount * (noise(x / l.cell, y / l.cell, SIZE / l.cell, l.seed, SIZE / l.cell) - 0.5);
      const line = Math.abs(noise(x / CRACK.cell, y / CRACK.cell, SIZE / CRACK.cell, CRACK.seed, SIZE / CRACK.cell) - 0.5);
      const p = CRACK_PATCH;
      const dried = THREE.MathUtils.smoothstep(noise(x / p.cell, y / p.cell, SIZE / p.cell, p.seed, SIZE / p.cell), p.from, p.to);
      v -= CRACK.depth * dried * (1 - THREE.MathUtils.smoothstep(line, 0, CRACK.width));
      values[y * SIZE + x] = v;
    }
  }
  for (const layer of PEBBLES) stampPebbles(values, layer);

  // The brightness as an sRGB byte, scaled in linear light so the average is
  // SOIL_MEAN. Both steps go through tables of the 256 bytes.
  const bytes = new Uint8Array(values.length);
  for (let i = 0; i < values.length; i++) bytes[i] = Math.round(255 * THREE.MathUtils.clamp(values[i], 0, 1));
  const color = new THREE.Color();
  const linear = Array.from({ length: 256 }, (_, b) => color.setRGB(b / 255, 0, 0, THREE.SRGBColorSpace).r);
  let sum = 0;
  for (let i = 0; i < bytes.length; i++) sum += linear[bytes[i]];
  const scale = SOIL_MEAN / (sum / bytes.length);
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
  texture.repeat.set(1 / SOIL_TILE, 1 / SOIL_TILE); // coordinates in meters
  texture.magFilter = THREE.LinearFilter;
  texture.minFilter = THREE.LinearMipmapLinearFilter;
  texture.generateMipmaps = true;
  texture.anisotropy = 8; // keeps the ground sharp where it is seen edge-on, toward the far shore
  texture.needsUpdate = true;
  cached = texture;
  return texture;
}

// Pebbles lying in the earth, at most one in each cell of a grid: a flat
// ellipse at a random spot, turn and size, lighter toward its top, with a
// dark rim round it. A pebble near the tile's edge wraps round to the other
// side, so the tile stays seamless.
function stampPebbles(values: Float32Array, layer: (typeof PEBBLES)[number]): void {
  const cells = SIZE / layer.cell;
  for (let cy = 0; cy < cells; cy++) {
    for (let cx = 0; cx < cells; cx++) {
      const r = (k: number) => hash(cx * 8 + k, cy, layer.seed);
      if (r(0) > layer.chance) continue;
      const centerX = (cx + r(1)) * layer.cell;
      const centerY = (cy + r(2)) * layer.cell;
      const radius = layer.radius[0] + (layer.radius[1] - layer.radius[0]) * r(3) ** 2;
      const aspect = 0.6 + 0.4 * r(4); // its width across, as a share of its length
      const angle = Math.PI * r(5);
      const tone = PEBBLE_TONE[0] + (PEBBLE_TONE[1] - PEBBLE_TONE[0]) * r(6);
      const cos = Math.cos(angle);
      const sin = Math.sin(angle);
      const reach = Math.ceil(radius * (1 + RIM.width)) + 1;
      for (let dy = -reach; dy <= reach; dy++) {
        for (let dx = -reach; dx <= reach; dx++) {
          const px = Math.floor(centerX) + dx;
          const py = Math.floor(centerY) + dy;
          const ox = px + 0.5 - centerX;
          const oy = py + 0.5 - centerY;
          const along = (ox * cos + oy * sin) / radius;
          const across = (oy * cos - ox * sin) / (radius * aspect);
          const d = Math.hypot(along, across); // 1 on its edge
          if (d > 1 + RIM.width) continue;
          const i = (((py % SIZE) + SIZE) % SIZE) * SIZE + (((px % SIZE) + SIZE) % SIZE);
          if (d < 1) {
            const pebble = tone + PEBBLE_DOME * Math.sqrt(1 - d * d);
            values[i] = THREE.MathUtils.lerp(values[i], pebble, 1 - THREE.MathUtils.smoothstep(d, 0.8, 1));
          } else {
            values[i] -= RIM.depth * (1 - (d - 1) / RIM.width);
          }
        }
      }
    }
  }
}
