import * as THREE from 'three';
import { grainUVs, mesh, type BoatMaterials } from './parts';

// The rudder: a flat wooden blade hung on the transom, with a wooden tiller
// reaching forward into the boat and a grip at its end. Its origin is the rudder's pivot at the
// top of the transom; turning it (rotation.y) swings the blade and the tiller
// together, to opposite sides.

const BLADE_THICKNESS = 0.024;
const BLADE_BEVEL = 0.004;
const TILLER_START = new THREE.Vector3(0, 0.07, 0);
const TILLER_END = new THREE.Vector3(0, 0.12, 0.85);
const TILLER_RADIUS = 0.02;
const GRIP_RADIUS = 0.028;
const GRIP_LENGTH = 0.16;

// Side view of the blade: x is the distance aft of the pivot, y the height
// above it. It reaches down to just above the floor.
function bladeShape(): THREE.Shape {
  const s = new THREE.Shape();
  s.moveTo(-0.02, 0.06);
  s.lineTo(0.06, 0.06);
  s.lineTo(0.08, -0.08);
  s.quadraticCurveTo(0.26, -0.18, 0.22, -0.36);
  s.quadraticCurveTo(0.12, -0.41, 0.02, -0.38);
  s.lineTo(-0.02, 0.06);
  return s;
}

// A rod from one point to another.
function rod(from: THREE.Vector3, to: THREE.Vector3, radius: number, mat: THREE.Material): THREE.Mesh {
  const geo = new THREE.CylinderGeometry(radius, radius, from.distanceTo(to), 12);
  geo.rotateX(Math.PI / 2); // along z
  grainUVs(geo); // for a wooden rod, the grain runs along it
  const m = mesh(geo, mat);
  m.position.copy(from).add(to).multiplyScalar(0.5);
  m.lookAt(to); // turns its +z toward `to`; it has no parent yet, so local = world
  return m;
}

export class Rudder extends THREE.Group {
  constructor(m: BoatMaterials) {
    super();
    this.name = 'rudder';

    const bladeGeo = new THREE.ExtrudeGeometry(bladeShape(), {
      depth: BLADE_THICKNESS,
      bevelEnabled: true,
      bevelThickness: BLADE_BEVEL,
      bevelSize: BLADE_BEVEL,
      bevelSegments: 2,
      curveSegments: 16,
    });
    bladeGeo.translate(0, 0, -BLADE_THICKNESS / 2); // centered on the pivot
    bladeGeo.rotateY(Math.PI / 2); // outline x -> aft (-z), thickness -> x
    grainUVs(bladeGeo); // the grain runs down the blade
    this.add(mesh(bladeGeo, m.timber));

    this.add(rod(TILLER_START, TILLER_END, TILLER_RADIUS, m.timber));
    const gripStart = TILLER_END.clone().sub(TILLER_START).setLength(GRIP_LENGTH).negate().add(TILLER_END);
    this.add(rod(gripStart, TILLER_END, GRIP_RADIUS, m.grip));
  }
}
