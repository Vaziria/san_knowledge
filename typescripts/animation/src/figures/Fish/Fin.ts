import * as THREE from 'three';
import type { FishModel } from './models';
import { sheet, triangles, type FishMaterials, type Side } from './parts';

// A pectoral fin, one of the pair behind the gills: the model's small paddle,
// a thin sheet reaching back from its root. Its origin is the root, the
// upright line it paddles about (rotation.y); its resting pose, turned in
// toward the body by the middle of the model's flap and mirrored on the
// left, is in an inner group, so its own rotation is 0 at rest.

export class Fin extends THREE.Group {
  constructor(m: FishMaterials, model: FishModel<string>, colors: Record<string, THREE.Color>, side: Side) {
    super();
    this.name = side < 0 ? 'left fin' : 'right fin';
    const { points, faces, flap } = model.pectoral;
    const rest = new THREE.Group();
    rest.rotation.y = side * flap.middle;
    rest.scale.x = side;
    rest.add(...sheet(triangles(points, faces, colors, model.unit), m));
    this.add(rest);
  }
}
