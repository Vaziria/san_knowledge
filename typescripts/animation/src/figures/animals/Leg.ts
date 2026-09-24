import * as THREE from 'three';
import { blob, capsule, mesh, solid, type AnimalMaterials, type Shade } from './parts';

// A leg: an upper and a lower segment and a foot (a paw, a hoof or a bird's
// toes), from the joint where it leaves the body (the shoulder or hip, the
// leg's origin) down. It is placed by reach(): given where the foot should
// be, it bends at the middle joint to put it there (two-bone inverse
// kinematics, in the leg's own y-z plane), so the animal decides where each
// foot goes and a planted foot stays exactly where it was put. The foot is
// kept level with the ground, tipping only when asked (a paw peeling off the
// ground as it lifts).
//
// `bend` says which way the middle joint points: +1 forward, as a front leg's
// wrist does when the paw folds back, and -1 backward, as a hind leg's hock
// and a bird's ankle do.

export type Foot = 'paw' | 'hoof' | 'claws';

export interface LegShape {
  upper: number; // m, the upper segment, joint to joint
  lower: number; // m, the lower segment, to the foot
  radius: number; // m, the upper segment's at its thickest; the lower is thinner
  foot: Foot;
  bend: 1 | -1;
}

const ROWS = 10;
const LOWER_RADIUS = 0.55; // the lower segment's radius, as a share of the upper's
const REACH_SHARE = 0.999; // never quite straight, so the joint always knows which way it bends

export class Leg extends THREE.Group {
  readonly upper = new THREE.Group();
  readonly lower = new THREE.Group();
  readonly foot = new THREE.Group();
  readonly shape: LegShape;

  constructor(m: AnimalMaterials, shape: LegShape, shade: Shade) {
    super();
    this.name = 'leg';
    this.shape = shape;
    const { upper, lower, radius } = shape;
    const r = radius * LOWER_RADIUS;
    const round = capsule(4);

    // Each segment runs straight down from its joint and is a little thicker
    // at the joints, so the knee reads as a joint.
    const segment = (length: number, top: number, bottom: number, from: number, to: number) =>
      solid(
        [new THREE.Vector3(0, top * 0.6, 0), new THREE.Vector3(0, -length / 2, 0), new THREE.Vector3(0, -length - bottom * 0.15, 0)],
        (u) => {
          const w = THREE.MathUtils.lerp(top, bottom, u) * round(u);
          return { width: w, top: w, bottom: w };
        },
        (u, out) => shade(THREE.MathUtils.lerp(from, to, u), out),
        ROWS,
        12,
      );
    const reach = upper + lower;
    this.upper.add(mesh(segment(upper, radius, r * 1.15, 0, upper / reach), m.coat));
    this.lower.add(mesh(segment(lower, r * 1.1, r * 0.9, upper / reach, 1), m.coat));
    this.lower.position.y = -upper;
    this.foot.position.y = -lower;

    const toe = shade(1, new THREE.Vector3(0, 1, 0));
    if (shape.foot === 'paw') {
      // A rounded paw, longer than wide, a little ahead of the ankle.
      this.foot.add(mesh(blob(new THREE.Vector3(0, r * 0.35, r * 0.7), new THREE.Vector3(r * 1.25, r * 0.7, r * 1.7), () => toe), m.coat));
    } else if (shape.foot === 'hoof') {
      // A hoof: dark, narrowing upward, its front sloping.
      this.foot.add(mesh(blob(new THREE.Vector3(0, r * 0.7, r * 0.2), new THREE.Vector3(r * 1.05, r * 0.9, r * 1.2), () => toe, { top: 0.35 }), m.dark));
    } else {
      // A bird's foot: three toes forward and one back, thin and dark.
      const toes = [-0.5, 0, 0.5].map((a) => new THREE.Euler(Math.PI / 2, 0, a)).concat([new THREE.Euler(-Math.PI / 2, 0, 0)]);
      for (const turn of toes) {
        const length = turn.x > 0 ? r * 5 : r * 3;
        const geo = blob(new THREE.Vector3(0, length / 2, 0), new THREE.Vector3(r * 0.55, length / 2, r * 0.55), () => toe);
        geo.applyMatrix4(new THREE.Matrix4().makeRotationFromEuler(turn));
        geo.translate(0, r * 0.4, 0);
        this.foot.add(mesh(geo, m.dark));
      }
    }

    this.lower.add(this.foot);
    this.upper.add(this.lower);
    this.add(this.upper);
  }

  // Bends the leg so its foot is at `target` (in the space of what the leg is
  // on, from its joint), with the foot tipped `tip` radians (positive lifts
  // its heel, as a paw peels off the ground). The leg first swings out or in
  // at its joint toward the target, so a body that rolls as it walks still
  // puts its feet exactly where they are planted, then bends in that plane.
  reach(target: THREE.Vector3, tip = 0): void {
    const { upper: a, lower: b, bend } = this.shape;
    this.rotation.z = Math.atan2(target.x, -target.y); // > 0 swings the foot toward +x
    const down = Math.hypot(target.x, target.y); // how far below the joint the foot is, in the swung plane
    const d = THREE.MathUtils.clamp(Math.hypot(down, target.z), Math.abs(a - b) + 1e-4, (a + b) * REACH_SHARE);
    const along = Math.atan2(target.z, down); // the joint-to-foot line, forward from straight down
    const hip = Math.acos(THREE.MathUtils.clamp((a * a + d * d - b * b) / (2 * a * d), -1, 1));
    const knee = Math.acos(THREE.MathUtils.clamp((a * a + b * b - d * d) / (2 * a * b), -1, 1));
    // rotation.x > 0 swings a segment back, so forward is negative.
    this.upper.rotation.x = -(along + bend * hip);
    this.lower.rotation.x = bend * (Math.PI - knee);
    this.foot.rotation.x = -(this.upper.rotation.x + this.lower.rotation.x) + tip;
  }
}
