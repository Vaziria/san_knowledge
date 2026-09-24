import * as THREE from 'three';
import { layerGeometry, mesh, type GrassMaterials, type Layer } from './parts';

// The leaves of a grass or herb: every blade (and a chive's tubular leaves,
// or a rosemary's needles), drawn as one mesh so a tuft is one draw call
// however many leaves it has. The origin is the plant's, on the ground at the
// middle of its foot.
export class Leaves extends THREE.Group {
  constructor(m: GrassMaterials, layer: Layer, random: () => number) {
    super();
    this.name = 'leaves';
    const geo = layerGeometry(layer, random);
    if (geo) this.add(mesh(geo, m.plant));
  }
}
