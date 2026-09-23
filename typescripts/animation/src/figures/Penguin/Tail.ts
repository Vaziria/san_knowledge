import * as THREE from 'three';
import { ellipsoid, type PenguinMaterials } from './parts';

// A short, flat tail sticking out of the lower back and sloping down toward
// the floor. Its origin is the tail's base, the pivot it wags around
// (rotation.y).

const TAIL_RADII = new THREE.Vector3(0.045, 0.014, 0.06);
const TAIL_SLOPE = -0.35; // tip lower than the base

export class Tail extends THREE.Group {
  constructor(m: PenguinMaterials) {
    super();
    this.name = 'tail';
    const slope = new THREE.Group();
    slope.rotation.x = TAIL_SLOPE;
    slope.add(ellipsoid(TAIL_RADII, m.back, 0, 0, -TAIL_RADII.z * 0.8));
    this.add(slope);
  }
}
