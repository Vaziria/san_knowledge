import * as THREE from 'three';
import { keepSharp } from '../../effects/kuwahara';
import { bark, surface, type Theme } from '../../theme';

// Helpers shared by the fishing rod's parts.

// The line is a cord 8 mm thick: the user asked twice for a thicker rope, so
// it reads clearly at preview distance.
export const LINE_RADIUS = 0.004;

// Materials for every part, built once per rod from the theme: bark in the
// theme's wood colour (see bark() in theme.ts), the paler plain wood colour
// where the branch was cut or broken, leaves in the theme's grass colour (its
// foliage colour), and the line in its dark colour.
export function createMaterials(theme: Theme) {
  const c = theme.colors;
  const leaf = surface(theme, theme.scene.grass);
  leaf.side = THREE.DoubleSide; // a leaf is a thin sheet, seen from both sides
  // The line is only a few pixels wide on screen, so the painterly filter
  // would average it away; it stays sharp instead.
  const line = surface(theme, c.dark);
  keepSharp(line);
  return {
    bark: bark(theme), // the branch, its twigs and stubs
    cut: surface(theme, c.wood), // cut and broken ends: fresh wood, paler than the bark
    leaf,
    line, // the line and its lashing
  };
}

export type RodMaterials = ReturnType<typeof createMaterials>;

export function mesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

// A flat round face of the given radius at `at`, facing `toward`: the cut end
// of a branch or stub.
export function cutFace(radius: number, at: THREE.Vector3, toward: THREE.Vector3, mat: THREE.Material): THREE.Mesh {
  const face = mesh(new THREE.CircleGeometry(radius, 14), mat);
  face.position.copy(at);
  face.quaternion.setFromUnitVectors(new THREE.Vector3(0, 0, 1), toward.clone().normalize());
  return face;
}
