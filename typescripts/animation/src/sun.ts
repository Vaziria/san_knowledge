import * as THREE from 'three';

// Where the sun is: the way the stage's key light shines from (Stage.ts puts
// the light out along it, as far as the figure needs for its shadow), and
// where a sky draws the sun (environtments/Sky.ts), so shadows fall away from
// it. Toward +x and +z, behind and to the right of the preview cameras, which
// look toward -z, about 50° up. Read it; never change it in place.
export const SUN = new THREE.Vector3(0.6, 1, 0.6);
