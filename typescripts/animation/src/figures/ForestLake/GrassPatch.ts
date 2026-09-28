import * as THREE from 'three';
import { addTuft, between, matte, mesh, palette, seededRandom, Shape, type Season } from './parts';

export interface GrassPatchOptions {
  height?: number; // m, the tallest blades; 0.55 by default
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's grass patch (GrassPatch.md), as its asset sheet draws
// it: a clump of tall, broad, lime-green blades standing up and arching a
// little outward, darker at their feet, lighter toward their tips, each
// folded along its middle. As the reference's seasons show it: deeper green
// in summer, golden orange in autumn, and dry and pale with frost in
// winter.
export class GrassPatch extends THREE.Group {
  readonly blades: THREE.Mesh;

  constructor(options: GrassPatchOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 61);
    const height = options.height ?? 0.55;
    const P = palette(options.season);
    const shape = new Shape();
    addTuft(shape, new THREE.Vector3(0, 0, 0), 14, height, random, 0.02, 0.85, P);
    // A second, lower ring round it fills its foot.
    addTuft(shape, new THREE.Vector3(between(random, -0.03, 0.03), 0, between(random, -0.03, 0.03)), 9, height * 0.6, random, 0.08, 0.5, P);
    this.blades = mesh(shape.geometry(), matte());
    this.add(this.blades);
  }
}
