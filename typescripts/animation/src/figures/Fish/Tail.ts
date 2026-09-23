import * as THREE from 'three';
import { finGeometry, mesh, type FishMaterials } from './parts';

// The tail fin (caudal fin): a forked fin with two pointed lobes, standing
// upright behind the tail base. Its origin is its root in the tail base, the
// pivot it sweeps from side to side around (rotation.y); it reaches back
// along -z.

export const TAIL_LENGTH = 0.07; // from the root to the lobe tips
const TAIL_SPAN = 0.095; // from the upper lobe tip to the lower one
const NOTCH = 0.045; // the fork's notch, behind the root
const ROOT_DEPTH = 0.024; // where the fin grows out of the tail base
const ROOT_OVERLAP = 0.004; // the root reaches this far into the tail base

export class Tail extends THREE.Group {
  constructor(m: FishMaterials) {
    super();
    this.name = 'tail';

    // Drawn from the side, +x toward the head: the upper edge flares out to
    // the upper tip, the fork curves in to the notch and out to the lower
    // tip, and the lower edge comes back to the root.
    const tip = TAIL_SPAN / 2;
    const root = ROOT_DEPTH / 2;
    const outline = new THREE.Shape();
    outline.moveTo(ROOT_OVERLAP, root);
    outline.quadraticCurveTo(-0.03, root + 0.004, -TAIL_LENGTH, tip);
    outline.quadraticCurveTo(-NOTCH - 0.006, 0.022, -NOTCH, 0);
    outline.quadraticCurveTo(-NOTCH - 0.006, -0.022, -TAIL_LENGTH, -tip);
    outline.quadraticCurveTo(-0.03, -root - 0.004, ROOT_OVERLAP, -root);
    outline.closePath();
    this.add(mesh(finGeometry(outline), m.fin));
  }
}
