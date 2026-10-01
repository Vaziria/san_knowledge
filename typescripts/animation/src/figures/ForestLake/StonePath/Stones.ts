import * as THREE from 'three';
import { matte } from '../parts';
import { matrixOf, tinted, type Laid } from './parts';

// The stone path's stones (StonePath.md): every flagstone, chip and stair's
// block laid, drawn instanced, one batch for each of its shapes
// (stoneGeometries), each copy tinted by the light on the ground under it
// and a little at random. Its shapes no stone uses are freed at once.
export class Stones extends THREE.Group {
  constructor(laid: readonly Laid[], shapes: readonly THREE.BufferGeometry[], light: (x: number, z: number) => number, random: () => number) {
    super();
    this.name = 'stones';
    const material = matte();
    const byShape: Laid[][] = shapes.map(() => []);
    for (const stone of laid) byShape[stone.template].push(stone);
    byShape.forEach((list, k) => {
      if (!list.length) {
        shapes[k].dispose();
        return;
      }
      const batch = new THREE.InstancedMesh(shapes[k], material, list.length);
      list.forEach((stone, i) => {
        batch.setMatrixAt(i, matrixOf(stone));
        batch.setColorAt(i, tinted(light(stone.x, stone.z), random, 0.07));
      });
      batch.castShadow = false;
      batch.receiveShadow = true;
      batch.computeBoundingSphere();
      this.add(batch);
    });
  }
}
