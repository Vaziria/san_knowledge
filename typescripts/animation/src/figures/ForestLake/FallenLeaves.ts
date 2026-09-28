import * as THREE from 'three';
import { addLeaf, between, color, FALLEN_LEAVES, matte, mesh, seededRandom, Shape, vary } from './parts';

export interface FallenLeavesOptions {
  radius?: number; // m, how far the patch reaches; 0.8 by default
  count?: number; // leaves in it; 26 by default
  seed?: number;
}

// The forest lake's fallen leaves (FallenLeaves.md; the reference's autumn
// "Fallen Leaves (Ground)"): a patch of autumn leaves lying on the ground,
// thickest in its middle and thinning out, each a small five-lobed leaf,
// orange, red, gold or rust, some overlapping. They lie at y 0.
export class FallenLeaves extends THREE.Group {
  readonly leaves: THREE.Mesh;

  constructor(options: FallenLeavesOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 261);
    const radius = options.radius ?? 0.8;
    const count = options.count ?? 26;
    const shape = new Shape();
    for (let k = 0; k < count; k++) {
      const a = random() * Math.PI * 2;
      // Thickest in the middle.
      const d = radius * random() ** 1.4;
      const tone = vary(color(FALLEN_LEAVES[Math.floor(random() * FALLEN_LEAVES.length)]), random, 1.2);
      // Each a little higher than the last, so overlapping ones don't flicker.
      addLeaf(shape, new THREE.Vector3(Math.cos(a) * d, 0, Math.sin(a) * d), between(random, 0.1, 0.16), random() * Math.PI * 2, tone, random, 0.01 + k * 0.0006);
    }
    this.leaves = mesh(shape.geometry(), matte(), false);
    this.add(this.leaves);
  }
}
