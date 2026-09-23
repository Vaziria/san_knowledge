import * as THREE from 'three';
import { noise } from './noise';

// A bark texture made in code, for branches and trunks: rough ridges running
// along the branch, split by darker furrows, with fine fibres. It is grey and
// darker than the wood grain, so it shades the theme's wood colour into bark,
// and a cut end in the plain wood colour shows paler, as fresh wood does.
//
// One tile is 20 cm x 20 cm: texture u runs along the branch, v around it.
// Give a surface texture coordinates in meters along the branch, and one tile
// around it (v from 0 to 0.2), and the tile wraps round without a seam.

export const BARK_TILE = 0.2; // meters a tile covers, each way
const SIZE = 512; // pixels a side, about 0.4 mm a pixel
const RIDGE_CELL = { along: 128, across: 16 }; // the ridges' noise cells, in pixels
const FIBRE_CELL = { along: 32, across: 2 };
const FURROW = 0.32; // ridge values below this sink into a furrow

let cached: THREE.DataTexture | null = null;

// The texture, made once and shared by every material that uses it.
export function barkTexture(): THREE.DataTexture {
  if (cached) return cached;
  const data = new Uint8Array(SIZE * SIZE * 4);
  for (let y = 0; y < SIZE; y++) {
    for (let x = 0; x < SIZE; x++) {
      const ridge = noise(
        x / RIDGE_CELL.along,
        y / RIDGE_CELL.across,
        SIZE / RIDGE_CELL.along,
        5,
        SIZE / RIDGE_CELL.across,
      );
      const fibre = noise(x / FIBRE_CELL.along, y / FIBRE_CELL.across, SIZE / FIBRE_CELL.along, 6, SIZE / FIBRE_CELL.across);
      let v = 0.62 + 0.26 * ridge - 0.08 * fibre;
      if (ridge < FURROW) v *= 0.55 + 0.45 * (ridge / FURROW);
      const byte = Math.round(255 * Math.min(1, Math.max(0, v)));
      const i = (y * SIZE + x) * 4;
      data[i] = data[i + 1] = data[i + 2] = byte;
      data[i + 3] = 255;
    }
  }

  const texture = new THREE.DataTexture(data, SIZE, SIZE, THREE.RGBAFormat);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.wrapS = texture.wrapT = THREE.RepeatWrapping;
  texture.repeat.set(1 / BARK_TILE, 1 / BARK_TILE); // coordinates in meters
  texture.magFilter = THREE.LinearFilter;
  texture.minFilter = THREE.LinearMipmapLinearFilter;
  texture.generateMipmaps = true;
  texture.anisotropy = 8;
  texture.needsUpdate = true;
  cached = texture;
  return texture;
}
