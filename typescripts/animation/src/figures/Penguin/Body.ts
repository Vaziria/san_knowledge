import * as THREE from 'three';
import { ellipsoid, type PenguinMaterials } from './parts';

// The pear-shaped body, wider at the bottom, with a large white belly
// covering most of its front. Its origin is on the floor under the body's
// center.

const BODY_RADII = new THREE.Vector3(0.145, 0.2, 0.135);
const BODY_Y = 0.2; // body center above the floor
const BODY_TAPER = { top: 0.15 };
const BELLY_RADII = new THREE.Vector3(0.12, 0.155, 0.1);
const BELLY_Y = 0.18;
const BELLY_Z = 0.05;

export class Body extends THREE.Group {
  constructor(m: PenguinMaterials) {
    super();
    this.name = 'body';
    this.add(ellipsoid(BODY_RADII, m.back, 0, BODY_Y, 0, BODY_TAPER));
    // The belly pokes out of the front of the body as a white patch.
    this.add(ellipsoid(BELLY_RADII, m.belly, 0, BELLY_Y, BELLY_Z, { top: 0.1 }));
  }
}
