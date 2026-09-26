import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { clumpGeometry, mesh, type Clump, type TreeMaterials } from './parts';

// The foliage of a tree: clumps of leaves or needles, drawn as one mesh so a
// crown is one draw call however many clumps it has. Besides each clump's own
// shading (see clumpGeometry), the crown is darker low down and toward the
// trunk, where the leaves are in the shade of the rest. The origin is the
// tree's, on the ground at the foot of the trunk. In autumn (`leafTints` in
// the materials) each clump is also tinted with one of them at random.

const LOW_SHADE = 0.8; // the bottom of the crown, times the top's
const INNER_SHADE = 0.85; // next to the trunk, times the outside's

export class Crown extends THREE.Group {
  constructor(m: TreeMaterials, clumps: Clump[], random: () => number) {
    super();
    this.name = 'crown';
    const parts = clumps.map((c) => {
      const part = clumpGeometry(c, random);
      if (m.leafTints) tint(part, m.leafTints[Math.floor(random() * m.leafTints.length)]);
      return part;
    });
    const geo = mergeGeometries(parts);
    for (const part of parts) part.dispose();

    const position = geo.getAttribute('position');
    const color = geo.getAttribute('color');
    geo.computeBoundingBox();
    const { min, max } = geo.boundingBox!;
    let outer = 0;
    for (let i = 0; i < position.count; i++) outer = Math.max(outer, Math.hypot(position.getX(i), position.getZ(i)));
    for (let i = 0; i < position.count; i++) {
      const height = (position.getY(i) - min.y) / (max.y - min.y);
      const out = Math.hypot(position.getX(i), position.getZ(i)) / outer;
      const shade = THREE.MathUtils.lerp(LOW_SHADE, 1, height) * THREE.MathUtils.lerp(INNER_SHADE, 1, out);
      color.setXYZ(i, color.getX(i) * shade, color.getY(i) * shade, color.getZ(i) * shade);
    }
    this.add(mesh(geo, m.foliage));
  }
}

// Multiplies a clump's vertex colours by a tint.
function tint(geo: THREE.BufferGeometry, color: THREE.Color): void {
  const c = geo.getAttribute('color');
  for (let i = 0; i < c.count; i++) c.setXYZ(i, c.getX(i) * color.r, c.getY(i) * color.g, c.getZ(i) * color.b);
}
