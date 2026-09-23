import * as THREE from 'three';
import { finGeometry, mesh, type FishMaterials, type Side } from './parts';

// A pectoral fin, one of the pair behind the gills: a rounded paddle that
// points back and fans out from the body, tipped down a little. Its origin is
// the fin's base on the side of the body, the pivot it paddles around.

const FIN_LENGTH = 0.042;
const FIN_WIDTH = 0.018;
const BASE_OVERLAP = 0.003; // the base reaches this far into the body
const SPREAD = 0.5; // how far it fans out from the body (radians)
const DROOP = 0.3; // how far its tip is tipped down (radians)

export class Fin extends THREE.Group {
  constructor(m: FishMaterials, side: Side) {
    super();
    this.name = side < 0 ? 'left fin' : 'right fin';

    // Drawn from the side, +x toward the head: a teardrop from the base back
    // to a rounded tip.
    const half = FIN_WIDTH / 2;
    const outline = new THREE.Shape();
    outline.moveTo(BASE_OVERLAP, half * 0.6);
    outline.quadraticCurveTo(-FIN_LENGTH * 0.45, half * 1.5, -FIN_LENGTH, half * 0.2);
    outline.quadraticCurveTo(-FIN_LENGTH * 0.5, -half * 1.3, BASE_OVERLAP, -half * 0.6);
    outline.closePath();

    // The resting pose: fanned out from the body and tipped down. Turning
    // about y by -side swings the tip (at -z) out to this side.
    const rest = new THREE.Group();
    rest.rotation.set(-DROOP, -side * SPREAD, 0);
    rest.add(mesh(finGeometry(outline), m.fin));
    this.add(rest);
  }
}
