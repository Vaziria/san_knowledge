import * as THREE from 'three';
import type { FishModel } from './models';
import { sheet, triangles, type FishMaterials } from './parts';

// The tail fin (caudal fin): the model's forked flat sheet, standing up and
// reaching back along -z, its inner part round the root and its edge in the
// edge colour. Its origin is its root, the middle of the body's tail end; the
// fish puts it there every frame, turned the way the bent body runs there,
// and flicks it further about its own y as the model does.

export class Tail extends THREE.Group {
  constructor(m: FishMaterials, model: FishModel<string>, colors: Record<string, THREE.Color>) {
    super();
    this.name = 'tail';
    this.add(...sheet(triangles(model.tail.points, model.tail.faces, colors, model.unit), m));
  }
}
