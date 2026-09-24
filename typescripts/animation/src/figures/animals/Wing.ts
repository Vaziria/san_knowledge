import * as THREE from 'three';
import { blob, mesh, type AnimalMaterials, type Shade, type Side } from './parts';

// A bird's wing, folded along its side at rest: a flat, rounded blade
// narrowing to its tip, reaching back from the shoulder (its origin). The
// bird spreads it by swinging it out sideways about the shoulder (rotation.y)
// and beats it up and down about its own body's length (rotation.z): the
// rotation order is ZYX, so the beat turns the spread wing.

export class Wing extends THREE.Group {
  constructor(m: AnimalMaterials, length: number, width: number, shade: Shade, side: Side) {
    super();
    this.name = side < 0 ? 'left wing' : 'right wing';
    this.rotation.order = 'ZYX';
    // Folded: lying along the body, back and a little down from the shoulder.
    const rest = new THREE.Group();
    rest.rotation.x = 0.25;
    rest.add(mesh(blob(new THREE.Vector3(0, 0, -length / 2), new THREE.Vector3(width * 0.12, width / 2, length / 2), shade, { bottom: 0.4 }), m.coat));
    this.add(rest);
  }
}
