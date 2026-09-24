import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { limbGeometry, mesh, type Flare, type TreeMaterials } from './parts';
import type { Limb } from './skeleton';

// The wood of a tree: its trunk and every branch in bark, drawn as one mesh
// so a tree is one draw call however many branches it has. The first limb is
// the trunk, and `flare` swells its foot into roots. The origin is the tree's,
// on the ground at the foot of the trunk.
export class Trunk extends THREE.Group {
  constructor(m: TreeMaterials, limbs: Limb[], flare?: Flare) {
    super();
    this.name = 'trunk';
    const parts = limbs.map((limb, i) => limbGeometry(limb.points, limb.radii, i === 0 ? flare : undefined));
    const geo = mergeGeometries(parts);
    for (const part of parts) part.dispose();
    this.add(mesh(geo, m.bark));
  }
}
