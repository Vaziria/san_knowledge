import * as THREE from 'three';
import { ellipsoid, mesh, type PenguinMaterials } from './parts';

// The head: a big round head that merges into the body, with a heart-shaped
// white face made of two lobes (the dark cap dips to a point between them),
// large glossy eyes inside the white, and a short, wide, flat beak in two
// parts. Its origin is the neck, the pivot it turns and nods around.

const HEAD_RADIUS = 0.13;
const HEAD_Y = 0.075; // head center above the neck

// The two face lobes, one around each eye, tilted so their tops lean outward.
const LOBE_RADII = new THREE.Vector3(0.066, 0.078, 0.055);
const LOBE_X = 0.045;
const LOBE_Z = 0.072;
const LOBE_TILT = 0.2;
// White under the beak, joining the two lobes.
const CHIN_RADII = new THREE.Vector3(0.07, 0.04, 0.05);

const EYE_RADIUS = 0.021;
const EYE_X = 0.047;
const EYE_Y = 0.015;
const EYE_Z = 0.116;
const SHINE_RADIUS = 0.006;

const UPPER_BEAK_RADII = new THREE.Vector3(0.038, 0.016, 0.034);
const LOWER_BEAK_RADII = new THREE.Vector3(0.028, 0.01, 0.028);

export class Head extends THREE.Group {
  constructor(m: PenguinMaterials) {
    super();
    this.name = 'head';

    const skull = mesh(new THREE.SphereGeometry(HEAD_RADIUS, 48, 32), m.back);
    skull.position.y = HEAD_Y;
    this.add(skull);

    for (const side of [-1, 1]) {
      const lobe = ellipsoid(LOBE_RADII, m.belly, side * LOBE_X, HEAD_Y - 0.005, LOBE_Z);
      lobe.rotation.z = -side * LOBE_TILT;
      this.add(lobe);
    }
    this.add(ellipsoid(CHIN_RADII, m.belly, 0, HEAD_Y - 0.075, 0.065));

    const eyeGeo = new THREE.SphereGeometry(EYE_RADIUS, 32, 16);
    const shineGeo = new THREE.SphereGeometry(SHINE_RADIUS, 12, 8);
    for (const side of [-1, 1]) {
      const eye = mesh(eyeGeo, m.eye);
      eye.position.set(side * EYE_X, HEAD_Y + EYE_Y, EYE_Z);
      this.add(eye);

      // A highlight at the upper right of each eye, as in the reference.
      const shine = mesh(shineGeo, m.shine);
      shine.position.set(side * EYE_X + 0.008, HEAD_Y + EYE_Y + 0.008, EYE_Z + 0.016);
      this.add(shine);
    }

    this.add(ellipsoid(UPPER_BEAK_RADII, m.beak, 0, HEAD_Y - 0.028, 0.12));
    this.add(ellipsoid(LOWER_BEAK_RADII, m.beak, 0, HEAD_Y - 0.045, 0.115));
  }
}
