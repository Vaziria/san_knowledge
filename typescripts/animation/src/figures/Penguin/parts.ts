import * as THREE from 'three';
import { gloss, surface, type Theme } from '../../theme';

// Helpers shared by the penguin's parts.

// Materials for every part, built once per penguin from the theme.
export function createMaterials(theme: Theme) {
  const c = theme.colors;
  return {
    back: surface(theme, c.body), // back, head cap, wings, tail
    belly: surface(theme, c.light), // belly and face
    beak: surface(theme, c.trim), // beak and feet
    eye: gloss(c.dark), // eyes are glossy even on a felt penguin
    claw: surface(theme, c.dark),
    shine: gloss(c.light),
  };
}

export type PenguinMaterials = ReturnType<typeof createMaterials>;

// -1 for the penguin's left (-x), 1 for its right (+x).
export type Side = -1 | 1;

export function mesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

// How much an ellipsoid narrows toward its top and bottom: 0 keeps it an
// ellipsoid, 0.3 makes that end 30% narrower (a pear shape, a tapered wing).
export interface Taper {
  top?: number;
  bottom?: number;
}

// A sphere stretched to the given radii, centered at (x, y, z), optionally
// narrowed toward its top or bottom.
export function ellipsoid(
  radii: THREE.Vector3,
  mat: THREE.Material,
  x: number,
  y: number,
  z: number,
  taper: Taper = {},
): THREE.Mesh {
  const geo = new THREE.SphereGeometry(1, 48, 32);
  const pos = geo.attributes.position;
  for (let i = 0; i < pos.count; i++) {
    const ny = pos.getY(i); // -1 at the bottom, 1 at the top
    const f = 1 - (taper.top ?? 0) * Math.max(0, ny) - (taper.bottom ?? 0) * Math.max(0, -ny);
    pos.setXYZ(i, pos.getX(i) * f * radii.x, ny * radii.y, pos.getZ(i) * f * radii.z);
  }
  geo.computeVertexNormals();
  const m = mesh(geo, mat);
  m.position.set(x, y, z);
  return m;
}
