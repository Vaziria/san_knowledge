import * as THREE from 'three';
import type { Limb } from '../Tree/skeleton';
import { layerGeometry, mesh, woodGeometry, type GrassMaterials, type Layer } from './parts';

// The stems of a grass or herb and what they carry: flowering stems with
// their seed heads or flowers, and the leaves on them, as one mesh, and woody
// stems (rosemary) as another, in bark. The origin is the plant's, on the
// ground at the middle of its foot.
export class Stems extends THREE.Group {
  constructor(m: GrassMaterials, layer: Layer, wood: Limb[], random: () => number) {
    super();
    this.name = 'stems';
    const geo = layerGeometry(layer, random);
    if (geo) this.add(mesh(geo, m.plant));
    const woody = woodGeometry(wood);
    if (woody) this.add(mesh(woody, m.bark));
  }
}
