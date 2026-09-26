import * as THREE from 'three';
import type { BirdModel, Role } from './birdModel';
import { mesh, polygons, type AnimalMaterials, type Side } from './parts';

// A bird's wing as the user modelled it (birdModel.ts): a few flat faces
// drawn by hand spread out along +x from the shoulder (its origin), blue with
// dark blue flight feathers and a small ridge on top, the left wing the right
// one mirrored. The model folds it back along the body and opens it by two
// poses, each three turns in the order YXZ (a lift about z, a twist about x,
// a swing about y), mixed turn by turn; the bird beats it by adding to the
// lift. The pose is the inner blade's, so the wing's own rotation stays 0.
//
// A sheet has two sides and the model draws it from both (a double-sided
// material). Here it is two meshes of the same faces, one for each side
// (rules.md, Building shapes rule 9): the coat on the side the model's faces
// face (under the spread wing, and out from the body when it is folded) and a
// back-side coat on the other, the same colours. Only the first casts a
// shadow, and it casts one from both sides (a double-sided shadow), or a
// folded wing lit on its outside would cast none.

export class Wing extends THREE.Group {
  private readonly blade = new THREE.Group();
  private readonly wing: BirdModel['wing'];
  private readonly side: Side;

  constructor(sheet: Sheet, model: BirdModel, colors: Record<Role, THREE.Color>, side: Side) {
    super();
    this.name = side < 0 ? 'left wing' : 'right wing';
    this.wing = model.wing;
    this.side = side;
    const { points, faces } = model.wing;
    const corner = (name: string) => new THREE.Vector3(...points[name]).multiplyScalar(model.unit);
    const geometry = polygons(
      faces.map(([a, b, c]) => [corner(a), corner(b), corner(c)]),
      (_n, f) => colors[faces[f][3]],
    );
    this.blade.rotation.order = 'YXZ';
    this.blade.scale.x = side; // the left wing is the right one mirrored, as in the model
    this.blade.add(...sheet(geometry));
    this.add(this.blade);
    this.pose(0, 0);
  }

  // Poses the wing as the model's poseWings does: `spread` from 0 folded to 1
  // spread, each turn mixed on its own, and `flap` radians added to the lift.
  // The left wing takes the right one's turns about y and z the other way.
  pose(spread: number, flap: number): void {
    const { folded, spread: open } = this.wing;
    const x = folded.x + (open.x - folded.x) * spread;
    const y = folded.y + (open.y - folded.y) * spread;
    const z = folded.z + (open.z - folded.z) * spread + flap;
    this.blade.rotation.set(x, this.side * y, this.side * z);
  }
}

// Makes the two meshes of a flat sheet from its faces (see the header).
export type Sheet = (geometry: THREE.BufferGeometry) => [front: THREE.Mesh, back: THREE.Mesh];

// The two sides of a flat sheet: the coat on its front, a back-side copy of
// the coat on its back. The front casts the shadow, from both sides.
export function sheets(m: AnimalMaterials): Sheet {
  const front = m.coat.clone();
  front.shadowSide = THREE.DoubleSide;
  const back = m.coat.clone();
  back.side = THREE.BackSide;
  return (geometry) => {
    const inside = mesh(geometry, back);
    inside.castShadow = false;
    return [mesh(geometry, front), inside];
  };
}
