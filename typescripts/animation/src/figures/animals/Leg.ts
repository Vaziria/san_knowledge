import * as THREE from 'three';
import { blob, capsule, loftGeometry, lofted, mesh, solid, type AnimalMaterials, type Loft, type Lofted, type Shade } from './parts';

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
// and a bird's ankle do. A bear's legs bend the other way round, like ours:
// the knee forward and the elbow back.
//
// Or the leg is modelled by hand (a LegModel, from the user's models): one
// loft from the joint down to the toes, as the model poses it, which bends at
// two of its rings, the middle joint and the ankle. Its bones are the same:
// the upper segment from the joint to the middle ring's middle, the lower
// from there to the ankle ring's, the foot below; reach() places them the
// same way, and the model's pose is their pose at rest (its foot where the
// model has it). The surface is one mesh of flat faces round them, bent with
// them whenever the leg is placed (Skin below): a ring between two joints
// goes with its bone, and a ring at a joint turns half as far as the bone
// below it turns from the one above, so the leg folds there like paper,
// without a gap. A bend within a segment (the knee in a wolf's hind leg,
// between its hip and hock) keeps the angle the model gives it. Each face is
// the colour the model paints it (`paint`, one per face), or else shaded down
// the leg (the wolf).

export type Foot = 'paw' | 'hoof' | 'claws';

export interface LegShape {
  upper: number; // m, the upper segment, joint to joint
  lower: number; // m, the lower segment, to the foot
  radius: number; // m, the upper segment's at its thickest; the lower is thinner
  lowerRadius?: number; // the lower segment's radius, as a share of the upper's (LOWER_RADIUS when left out)
  foot: Foot;
  toes?: number; // toes along the front of a paw, as a bear has (none when left out)
  bend: 1 | -1;
}

// A leg modelled by hand: a loft from the joint, the leg's origin, down (see
// the header).
export interface LegModel extends Loft {
  middle: number; // the ring at the middle joint
  ankle: number; // the ring at the ankle, where the foot begins
  bend: 1 | -1;
}

// A modelled leg's bones: its surface, the lengths of its two segments, where
// its ankle is at rest (from the joint), and how high the ankle is then above
// the lowest point of the foot, which stands on the ground. The ankle at rest
// is the model's, or, for a leg all but straight there (a wolf's front leg),
// as far as reach() stretches it (REACH_SHARE), a fraction of a millimeter
// short, so the foot stands on the ground when the leg reaches for it.
export function legBones(model: LegModel) {
  const surface = lofted(model);
  const middle = surface.centers[model.middle];
  const upper = middle.length();
  const lower = middle.distanceTo(surface.centers[model.ankle]);
  const ankle = surface.centers[model.ankle].clone();
  ankle.setLength(Math.min(ankle.length(), (upper + lower) * REACH_SHARE));
  const low = Math.min(...surface.points.filter((_p, i) => surface.ring[i] >= model.ankle).map((p) => p.y));
  return { surface, upper, lower, ankle, height: ankle.y - low };
}

const ROWS = 10;
const FACETED_ROWS = 14; // halved when faceted: enough to round the ends of a segment
export const LOWER_RADIUS = 0.55; // the lower segment's radius, as a share of the upper's
export const REACH_SHARE = 0.999; // never quite straight, so the joint always knows which way it bends
const DROP = { step: 0.05, steps: 32 }; // radians between the tips drop() is measured at, and how many
const TOE = { spread: 0.72, length: 0.45, width: 0.36, height: 0.24 }; // a paw's toes: how far across its width they spread, and each toe's size, as shares of the ankle's radius

// Reused by reach().
const dir = new THREE.Vector3();
const knee = new THREE.Vector3();
const across = new THREE.Vector3();
const basis = new THREE.Matrix4();
const segments = new THREE.Quaternion();
const level = new THREE.Quaternion();
const X = new THREE.Vector3(1, 0, 0);

export class Leg extends THREE.Group {
  readonly upper = new THREE.Group();
  readonly lower = new THREE.Group();
  readonly foot = new THREE.Group();
  private readonly bones: { upper: number; lower: number; bend: 1 | -1 };
  private readonly drops: number[] = [];
  private readonly skin?: Skin; // a modelled leg's surface

  constructor(m: AnimalMaterials, shape: LegShape | LegModel, shade: Shade, paint?: readonly THREE.Color[]) {
    super();
    this.name = 'leg';
    let feet: THREE.Vector3[]; // the foot's corners, in its own axes
    if ('sections' in shape) {
      const bones = legBones(shape);
      this.bones = { upper: bones.upper, lower: bones.lower, bend: shape.bend };
      this.lower.position.y = -bones.upper;
      this.foot.position.y = -bones.lower;
      this.reach(bones.ankle); // the model's pose, its rest
      this.skin = new Skin(this, bones.surface, shape, shade, m.coat, paint);
      this.add(this.skin.mesh);
      feet = this.skin.foot();
    } else {
      this.bones = shape;
      feet = this.rounded(m, shape, shade);
    }
    this.lower.add(this.foot);
    this.upper.add(this.lower);
    this.add(this.upper);

    // How far below the ankle the foot reaches, tipped by each step.
    for (let k = 0; k <= DROP.steps; k++) {
      const t = k * DROP.step;
      this.drops.push(Math.max(...feet.map((p) => p.z * Math.sin(t) - p.y * Math.cos(t))));
    }
  }

  // A leg from its numbers: rounded segments and a foot on the bones. Returns
  // the foot's corners.
  private rounded(m: AnimalMaterials, shape: LegShape, shade: Shade): THREE.Vector3[] {
    const { upper, lower, radius } = shape;
    const r = radius * (shape.lowerRadius ?? LOWER_RADIUS);
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
        m.look === 'faceted' ? FACETED_ROWS : ROWS,
        12,
        m.look,
      );
    const reach = upper + lower;
    this.upper.add(mesh(segment(upper, radius, r * 1.15, 0, upper / reach), m.coat));
    this.lower.add(mesh(segment(lower, r * 1.1, r * 0.9, upper / reach, 1), m.coat));
    this.lower.position.y = -upper;
    this.foot.position.y = -lower;
    // Faceted, the rounded ends of the two segments close to points in a few
    // flat faces and pinch the leg in at the middle joint, so a ball fills it.
    if (m.look === 'faceted') {
      const at = upper / reach;
      this.lower.add(mesh(blob(new THREE.Vector3(), new THREE.Vector3(r * 1.2, r * 1.2, r * 1.2), (_u, out) => shade(at, out), {}, undefined, m.look), m.coat));
    }

    const toe = shade(1, new THREE.Vector3(0, 1, 0));
    if (shape.foot === 'paw') {
      // A rounded paw, longer than wide, a little ahead of the ankle, with
      // its toes, if it has any, in a row round its front.
      this.foot.add(mesh(blob(new THREE.Vector3(0, r * 0.35, r * 0.7), new THREE.Vector3(r * 1.25, r * 0.7, r * 1.7), () => toe, {}, undefined, m.look), m.coat));
      const toes = shape.toes ?? 0;
      for (let k = 0; k < toes; k++) {
        const a = toes > 1 ? (2 * k) / (toes - 1) - 1 : 0; // -1 to 1 across the paw
        const at = new THREE.Vector3(a * TOE.spread * r * 1.25, r * 0.22, r * 0.7 + r * 1.3 * Math.sqrt(1 - 0.6 * a * a));
        const turn = new THREE.Euler(0, a * 0.5, 0);
        this.foot.add(mesh(blob(at, new THREE.Vector3(r * TOE.width, r * TOE.height, r * TOE.length), () => toe, { top: 0.2 }, turn, m.look), m.coat));
      }
    } else if (shape.foot === 'hoof') {
      // A hoof: dark, narrowing upward, its front sloping.
      this.foot.add(mesh(blob(new THREE.Vector3(0, r * 0.7, r * 0.2), new THREE.Vector3(r * 1.05, r * 0.9, r * 1.2), () => toe, { top: 0.35 }, undefined, m.look), m.dark));
    } else {
      // A bird's foot: three toes forward and one back, thin and dark.
      const toes = [-0.5, 0, 0.5].map((a) => new THREE.Euler(Math.PI / 2, 0, a)).concat([new THREE.Euler(-Math.PI / 2, 0, 0)]);
      for (const turn of toes) {
        const length = turn.x > 0 ? r * 5 : r * 3;
        const geo = blob(new THREE.Vector3(0, length / 2, 0), new THREE.Vector3(r * 0.55, length / 2, r * 0.55), () => toe, {}, undefined, m.look);
        geo.applyMatrix4(new THREE.Matrix4().makeRotationFromEuler(turn));
        geo.translate(0, r * 0.4, 0);
        this.foot.add(mesh(geo, m.dark));
      }
    }

    const points: THREE.Vector3[] = [];
    this.foot.traverse((o) => {
      if (!(o instanceof THREE.Mesh)) return;
      const position = o.geometry.getAttribute('position');
      for (let i = 0; i < position.count; i++) points.push(new THREE.Vector3().fromBufferAttribute(position, i));
    });
    return points;
  }

  // The nearest its foot can come to its joint: both segments folded
  // together, the lower turned back up along the upper.
  get shortest(): number {
    return Math.abs(this.bones.upper - this.bones.lower);
  }

  // How far below the ankle the foot reaches with the foot tipped `tip`
  // radians (its heel lifted): a long paw's toes swing down as it peels off
  // the ground, so the animal lifts the ankle by the difference.
  drop(tip: number): number {
    const k = THREE.MathUtils.clamp(tip / DROP.step, 0, DROP.steps);
    const i = Math.min(Math.floor(k), DROP.steps - 1);
    return THREE.MathUtils.lerp(this.drops[i], this.drops[i + 1], k - i);
  }

  // Bends the leg so its foot is at `target` (in the space of what the leg is
  // on, from its joint), with the foot tipped `tip` radians (positive lifts
  // its heel, as a paw peels off the ground). The leg first swings out or in
  // at its joint toward the target, so a body that rolls as it walks still
  // puts its feet exactly where they are planted, then bends in that plane.
  //
  // Given a `pole` (a direction in the same space), the leg instead turns at
  // its joint so the middle joint points as near that way as it can, and the
  // foot is kept level in that space rather than the leg's, tipped `tip`
  // about its x. This holds whichever way the body is turned: swinging only
  // about the leg's own z would spin the knee round once the body stands up
  // on its hind legs (a bear), since that axis then points up.
  //
  // A modelled leg's surface is bent to its bones' new places each time.
  reach(target: THREE.Vector3, tip = 0, pole?: THREE.Vector3): void {
    if (pole) this.reachToward(target, tip, pole);
    else this.reachAlong(target, tip);
    this.skin?.bend(this);
  }

  private reachAlong(target: THREE.Vector3, tip: number): void {
    const { upper: a, lower: b, bend } = this.bones;
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

  private reachToward(target: THREE.Vector3, tip: number, pole: THREE.Vector3): void {
    const { upper: a, lower: b, bend } = this.bones;
    // Turn the leg so its -y points at the target and the middle joint's side
    // (its +z times bend, as in reach()) faces the pole.
    const d = THREE.MathUtils.clamp(target.length(), Math.abs(a - b) + 1e-4, (a + b) * REACH_SHARE);
    dir.copy(target).normalize();
    knee.copy(pole).addScaledVector(dir, -pole.dot(dir));
    if (knee.lengthSq() < 1e-8) knee.set(0, 0, 1).addScaledVector(dir, -dir.z); // a pole along the leg says nothing; take ahead
    knee.normalize().multiplyScalar(bend);
    dir.negate(); // the leg's y
    across.crossVectors(dir, knee); // its x
    this.quaternion.setFromRotationMatrix(basis.makeBasis(across, dir, knee));
    const hip = Math.acos(THREE.MathUtils.clamp((a * a + d * d - b * b) / (2 * a * d), -1, 1));
    const bent = Math.acos(THREE.MathUtils.clamp((a * a + b * b - d * d) / (2 * a * b), -1, 1));
    this.upper.rotation.set(-bend * hip, 0, 0);
    this.lower.rotation.set(bend * (Math.PI - bent), 0, 0);
    // The foot: level in the space the target is in, tipped about its x.
    segments.copy(this.quaternion).multiply(this.upper.quaternion).multiply(this.lower.quaternion).invert();
    this.foot.quaternion.copy(segments).multiply(level.setFromAxisAngle(X, tip));
  }
}

// Reused by Skin: the three bones (upper, lower, foot) in the leg's axes,
// each one's turn, and the two joints (the middle joint, the ankle).
const bone = [new THREE.Matrix4(), new THREE.Matrix4(), new THREE.Matrix4()];
const turn = [new THREE.Quaternion(), new THREE.Quaternion(), new THREE.Quaternion()];
const half = [new THREE.Quaternion(), new THREE.Quaternion()];
const pivot = [new THREE.Vector3(), new THREE.Vector3()];
const side = new THREE.Vector3();
const normal = new THREE.Vector3();

// Where a leg's bones are now, in the leg's axes, and how each is turned.
function bonesOf(leg: Leg): void {
  leg.upper.updateMatrix();
  leg.lower.updateMatrix();
  leg.foot.updateMatrix();
  bone[0].copy(leg.upper.matrix);
  bone[1].multiplyMatrices(bone[0], leg.lower.matrix);
  bone[2].multiplyMatrices(bone[1], leg.foot.matrix);
  turn[0].copy(leg.upper.quaternion);
  turn[1].multiplyQuaternions(turn[0], leg.lower.quaternion);
  turn[2].multiplyQuaternions(turn[1], leg.foot.quaternion);
  pivot[0].setFromMatrixPosition(bone[1]);
  pivot[1].setFromMatrixPosition(bone[2]);
}

// A modelled leg's surface, bent with its bones (see the header). Made with
// the leg at rest, in the model's pose.
class Skin {
  readonly mesh: THREE.Mesh;
  private readonly faces: readonly (readonly [number, number, number])[];
  private readonly of: number[]; // each corner's bone (0 upper, 1 lower, 2 foot), or, on a ring at a joint, 3 + the joint (0 the middle joint, 1 the ankle)
  private readonly local: THREE.Vector3[]; // each corner in its bone's axes at rest; at a joint, from the joint, in the leg's axes at rest
  private readonly rest: THREE.Quaternion[]; // each bone's turn at rest, undone
  private readonly corners: THREE.Vector3[]; // each corner, placed
  private readonly position: THREE.BufferAttribute;
  private readonly normal: THREE.BufferAttribute;

  constructor(leg: Leg, surface: Lofted, model: LegModel, shade: Shade, material: THREE.Material, paint?: readonly THREE.Color[]) {
    bonesOf(leg);
    const undo = bone.map((b) => b.clone().invert());
    this.rest = turn.map((q) => q.clone().invert());
    const joints = pivot.map((p) => p.clone());
    this.of = surface.ring.map((r) => (r < model.middle ? 0 : r === model.middle ? 3 : r < model.ankle ? 1 : r === model.ankle ? 4 : 2));
    this.local = surface.points.map((p, i) => {
      const b = this.of[i];
      return b < 3 ? p.clone().applyMatrix4(undo[b]) : p.clone().sub(joints[b - 3]);
    });
    this.corners = surface.points.map((p) => p.clone());
    this.faces = surface.faces;

    // Coloured as the model paints it, or by how far down the leg each face is.
    const along = [0];
    for (let i = 1; i < surface.centers.length; i++) along.push(along[i - 1] + surface.centers[i].distanceTo(surface.centers[i - 1]));
    const length = along[along.length - 1];
    const geometry = loftGeometry(surface, (face, n) => {
      const band = surface.band[face];
      return paint?.[face] ?? shade((along[band] + along[band + 1]) / (2 * length), n);
    });
    this.position = geometry.getAttribute('position') as THREE.BufferAttribute;
    this.normal = geometry.getAttribute('normal') as THREE.BufferAttribute;
    // However it bends, no corner gets further from the joint than its own
    // distance from its bone's origin, plus the bones' lengths down to it.
    const upper = joints[0].length();
    const down = [0, upper, upper + joints[0].distanceTo(joints[1]), upper, upper + joints[0].distanceTo(joints[1])];
    const radius = Math.max(...this.local.map((p, i) => p.length() + down[this.of[i]]));
    geometry.boundingSphere = new THREE.Sphere(new THREE.Vector3(), radius);
    this.mesh = mesh(geometry, material);
  }

  // The foot's corners (the ankle ring's too), in the foot's axes.
  foot(): THREE.Vector3[] {
    return this.local.flatMap((p, i) => (this.of[i] === 2 ? [p] : this.of[i] === 4 ? [p.clone().applyQuaternion(this.rest[2])] : []));
  }

  // Bends the surface to where the leg's bones are now.
  bend(leg: Leg): void {
    bonesOf(leg);
    for (let b = 0; b < 3; b++) turn[b].multiply(this.rest[b]);
    half[0].slerpQuaternions(turn[0], turn[1], 0.5);
    half[1].slerpQuaternions(turn[1], turn[2], 0.5);
    for (let i = 0; i < this.corners.length; i++) {
      const b = this.of[i];
      if (b < 3) this.corners[i].copy(this.local[i]).applyMatrix4(bone[b]);
      else this.corners[i].copy(this.local[i]).applyQuaternion(half[b - 3]).add(pivot[b - 3]);
    }
    for (let f = 0; f < this.faces.length; f++) {
      const face = this.faces[f];
      const p = this.corners[face[0]];
      normal.subVectors(this.corners[face[1]], p).cross(side.subVectors(this.corners[face[2]], p)).normalize();
      for (let k = 0; k < 3; k++) {
        const c = this.corners[face[k]];
        this.position.setXYZ(3 * f + k, c.x, c.y, c.z);
        this.normal.setXYZ(3 * f + k, normal.x, normal.y, normal.z);
      }
    }
    this.position.needsUpdate = true;
    this.normal.needsUpdate = true;
    this.mesh.geometry.boundingBox = null; // measured again when next asked for
  }
}
