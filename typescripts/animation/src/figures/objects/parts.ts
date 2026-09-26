import * as THREE from 'three';
import { noise } from '../../textures/noise';
import type { Theme } from '../../theme';

export { between, randomUnit, seededRandom } from '../Tree/parts';

// Helpers shared by the objects (the cliff, the cloud and the fog): their
// options, the size they are built at, and fitting what they build to exactly
// that size.

export interface ObjectOptions {
  theme?: Theme;
  // The seed the object is built from: the same seed always gives the same
  // object, another seed another one of the same kind. Each object has its
  // own default.
  seed?: number;
  // Its size in meters: across (x), from the bottom to the top (y), and from
  // back to front (z). Each object has its own default for each (its SIZE).
  width?: number;
  height?: number;
  depth?: number;
}

export interface Size {
  width: number;
  height: number;
  depth: number;
}

// The size an object is built at: the options, and the object's default for
// any left out.
export function sizeOf(options: ObjectOptions, size: Size): Size {
  return {
    width: options.width ?? size.width,
    height: options.height ?? size.height,
    depth: options.depth ?? size.depth,
  };
}

// Stretches geometries together so that they fill exactly the box of `size`:
// centered on x and z, from y = 0 up. The objects are built at about the size
// asked for, so this only corrects what their random shapes missed by.
// applyMatrix4 turns the normals too. Returns the stretch, for a point
// planned before it.
export function fit(geometries: THREE.BufferGeometry[], size: Size): THREE.Matrix4 {
  const box = new THREE.Box3();
  for (const geo of geometries) {
    geo.computeBoundingBox();
    box.union(geo.boundingBox!);
  }
  const extent = box.getSize(new THREE.Vector3());
  const matrix = new THREE.Matrix4()
    .makeScale(size.width / extent.x, size.height / extent.y, size.depth / extent.z)
    .multiply(new THREE.Matrix4().makeTranslation(-(box.min.x + box.max.x) / 2, -box.min.y, -(box.min.z + box.max.z) / 2));
  for (const geo of geometries) {
    geo.applyMatrix4(matrix);
    geo.computeBoundingBox();
    geo.computeBoundingSphere();
  }
  return matrix;
}

// Smooth value noise from -1 to 1, with features about a unit apart. Each
// `seed` gives another pattern.
export function wobble(x: number, y: number, seed: number): number {
  return 2 * noise(x, y, 0, seed) - 1;
}

export function mesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}
