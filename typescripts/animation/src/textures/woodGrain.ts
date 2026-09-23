import * as THREE from 'three';
import { hash, noise } from './noise';

// A wood-grain texture made in code: planks laid side by side, each with its
// own tone, wavy growth rings along its length, fine streaks, dark seams
// between planks and one butt joint (where two boards meet end to end) at a
// different place in each. It is grey, so it shades whatever colour its
// material has: the theme's wood colour.
//
// One tile is 1 m x 1 m: texture u runs along the grain, v across the planks.
// Give a surface texture coordinates in meters and the planks come out their
// real size. The tile repeats seamlessly both ways.

const SIZE = 1024; // pixels a side, about 1 mm a pixel
const PLANKS = 8; // across the tile: planks 12.5 cm wide
const PLANK = SIZE / PLANKS;
const SEAM = 2; // pixels of dark seam at each plank edge and joint
const RING_SPACING = 9; // pixels between growth rings across a plank
const RING_WAVE = 28; // pixels the rings wander across the plank
const RING_WAVE_CELL = 256; // pixels along the grain per wander
const STREAK_CELL = { along: 128, across: 4 }; // the fine streaks' noise cells

let cached: THREE.DataTexture | null = null;

// The texture, made once and shared by every material that uses it.
export function woodGrain(): THREE.DataTexture {
  if (cached) return cached;
  const data = new Uint8Array(SIZE * SIZE * 4);
  for (let p = 0; p < PLANKS; p++) {
    const tone = 0.88 + 0.1 * hash(p, 0, 1); // each board a little lighter or darker
    const joint = Math.floor(hash(p, 0, 2) * SIZE); // where its butt joint is
    for (let y = 0; y < PLANK; y++) {
      const row = p * PLANK + y;
      for (let x = 0; x < SIZE; x++) {
        let v = tone;
        // Growth rings: lines across the plank that wander along it.
        const wander = RING_WAVE * noise(x / RING_WAVE_CELL, p * 7, SIZE / RING_WAVE_CELL, 3);
        const ring = (y + wander) / RING_SPACING;
        v -= 0.12 * Math.pow(0.5 + 0.5 * Math.cos(2 * Math.PI * ring), 4);
        // Fine streaks along the grain.
        v -= 0.07 * noise(x / STREAK_CELL.along, row / STREAK_CELL.across, SIZE / STREAK_CELL.along, 4);
        // Seams at the plank's edges and its joint.
        const dx = Math.min(Math.abs(x - joint), SIZE - Math.abs(x - joint));
        if (y < SEAM || y >= PLANK - SEAM || dx < SEAM / 2 + 0.5) v *= 0.55;
        const byte = Math.round(255 * Math.min(1, Math.max(0, v)));
        const i = (row * SIZE + x) * 4;
        data[i] = data[i + 1] = data[i + 2] = byte;
        data[i + 3] = 255;
      }
    }
  }

  const texture = new THREE.DataTexture(data, SIZE, SIZE, THREE.RGBAFormat);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.wrapS = texture.wrapT = THREE.RepeatWrapping;
  texture.magFilter = THREE.LinearFilter;
  texture.minFilter = THREE.LinearMipmapLinearFilter;
  texture.generateMipmaps = true;
  texture.anisotropy = 8; // keeps the grain sharp on surfaces seen edge-on
  texture.needsUpdate = true;
  cached = texture;
  return texture;
}
