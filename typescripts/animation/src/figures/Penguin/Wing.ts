import * as THREE from 'three';
import { ellipsoid, type PenguinMaterials, type Side } from './parts';

// One wing (flipper): long and flat, tapering to a narrow tip, hanging down
// from the shoulder and angled away from the body. Its origin is the
// shoulder, the pivot it swings around: rotation.z away from the body is
// negative on the left wing and positive on the right.

const WING_RADII = new THREE.Vector3(0.028, 0.13, 0.055);
const WING_TAPER = { top: 0.1, bottom: 0.55 };
const WING_SPREAD = 0.35; // resting angle away from the body

export class Wing extends THREE.Group {
  constructor(m: PenguinMaterials, side: Side) {
    super();
    this.name = side < 0 ? 'left-wing' : 'right-wing';
    // The rest tilt sits in its own group, so rotation.z of the wing is 0 at rest.
    const rest = new THREE.Group();
    rest.rotation.z = side * WING_SPREAD;
    rest.add(ellipsoid(WING_RADII, m.back, 0, -WING_RADII.y * 0.8, 0, WING_TAPER));
    this.add(rest);
  }
}
