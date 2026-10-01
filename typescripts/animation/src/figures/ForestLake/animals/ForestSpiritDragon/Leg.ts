import * as THREE from 'three';
import { blend, Sculpt, tone, type Palette, type Tone } from '../parts';
import { bendingMeshes, bodyScaleTone, chainOf, claw, cover, hash, leaf, lid, ringLoft, sectionRing, shaded, shellAt, shellOf, tube, type Chain, type Face, type Materials, type Shell } from './parts';

// The forest spirit dragon's legs (ForestSpiritDragon.md), as the overview
// and the claw sheet draw them: sturdy, a heavy shoulder or thigh coming
// out of the body, covered in leaf scales pointing down, with a ring of
// bigger leaves hanging over the ankle; a paw green on top and cream
// beneath, three cream toes and a small inner one, each with an ivory claw,
// the front claws bigger and more curved; the back of each foreleg cream,
// with cream tufts at the elbow and the wrist.
//
// It bends at its elbow or knee and at its ankle: three bones (the upper
// leg, the lower leg, the foot), stiff between the joints. reach() puts its
// ankle where the dragon wants it (two-bone inverse kinematics, the joint
// bending the way it does as built) and turns its foot.
//
// Meters, in the dragon's frame (y up, facing +z, its left at -x). Each
// leg's origin is its root in the body, the shoulder or the hip, where it
// would swing. The left legs are the right ones mirrored.

type V = THREE.Vector3;
const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

// A leg as it stands (its right one, x > 0): its points, the thickness at
// each of its stations down it, its paw and its toes.
export interface LegSpec {
  front: boolean;
  root: V; // in the body: the shoulder or the hip, its origin
  joint: V; // the elbow (pointing back) or the knee (forward)
  ankle: V; // the wrist, or the hock
  paw: V; // the middle of the paw's sole, on the ground
  // At the root, down the upper leg, at the joint, down the lower leg, at
  // the ankle and in the paw: half its width, and how far it reaches in
  // front of and behind its line.
  stations: readonly (readonly [w: number, front: number, back: number])[];
  pawSize: readonly [length: number, width: number, height: number];
  toe: number; // m long
  claw: readonly [length: number, hook: number]; // m, and radians it curls down
  scale: readonly [top: number, bottom: number]; // its leaves' length, at its root and above its ankle
}

export const FRONT_LEG: LegSpec = {
  front: true,
  root: v(0.52, 1.82, 1.2),
  joint: v(0.62, 1.12, 0.96),
  ankle: v(0.66, 0.42, 1.1),
  paw: v(0.68, 0, 1.24),
  stations: [
    [0.42, 0.48, 0.48],
    [0.38, 0.42, 0.42],
    [0.28, 0.28, 0.32],
    [0.24, 0.25, 0.26],
    [0.22, 0.22, 0.22],
    [0.18, 0.17, 0.17],
  ],
  pawSize: [0.52, 0.48, 0.3],
  toe: 0.2,
  claw: [0.18, 0.7],
  scale: [0.55, 0.36],
};

export const HIND_LEG: LegSpec = {
  front: false,
  root: v(0.62, 1.66, -1.1),
  joint: v(0.76, 1.05, -0.68),
  ankle: v(0.8, 0.52, -1.3),
  paw: v(0.8, 0, -1.06),
  stations: [
    [0.44, 0.55, 0.55],
    [0.38, 0.52, 0.52],
    [0.31, 0.32, 0.34],
    [0.25, 0.26, 0.28],
    [0.22, 0.21, 0.23],
    [0.18, 0.17, 0.17],
  ],
  pawSize: [0.56, 0.5, 0.3],
  toe: 0.2,
  claw: [0.15, 0.5],
  scale: [0.62, 0.36],
};

const CORNERS = 12;
const PAW_CORNERS = 10;
// The foreleg's cream back: radians either side of its back.
const BACK = 0.55;
// How a claw starts from its toe: this far below level (radians).
const CLAW_DIP = 0.05;

// The leg's root on the side it is on: where the dragon holds it.
export function legRoot(spec: LegSpec, side: 1 | -1): V {
  return v(spec.root.x * side, spec.root.y, spec.root.z);
}

// The leg's points on the side it is on.
function placed(spec: LegSpec, side: 1 | -1) {
  const m = (p: V) => v(p.x * side, p.y, p.z);
  return { root: m(spec.root), joint: m(spec.joint), ankle: m(spec.ankle), paw: m(spec.paw) };
}

// The leg's surface down from its root to inside its paw: corner 0 of each
// ring at its front, the rest going round its outside first.
export function legShell(spec: LegSpec, side: 1 | -1): Shell {
  const { root, joint, ankle, paw } = placed(spec, side);
  const pawTop = paw.clone().setY(spec.pawSize[2] * 0.62);
  const centres = [root, root.clone().lerp(joint, 0.45), joint, joint.clone().lerp(ankle, 0.5), ankle, pawTop];
  const outward = v(side, 0, 0);
  return shellOf(
    centres.map((at, i) => {
      const t = centres[Math.min(centres.length - 1, i + 1)].clone().sub(centres[Math.max(0, i - 1)]).normalize();
      const x = outward.clone().addScaledVector(t, -outward.dot(t)).normalize();
      const front = new THREE.Vector3().crossVectors(t, v(1, 0, 0)).normalize();
      if (front.z < 0) front.negate();
      const [w, ahead, behind] = spec.stations[i];
      return sectionRing(at, x, front, w, ahead, behind, CORNERS, 2.2);
    }),
  );
}

function angleOf(k: number, count: number): number {
  return (Math.PI * 2 * (k + 0.5)) / count;
}

// How far round its joints the leg's surface blends from one bone to the
// next, as a share of the shorter of the two: stiff between them.
const BLEND = 0.22;

export class Leg extends THREE.Group {
  readonly spec: LegSpec;
  readonly side: 1 | -1;
  readonly chain: Chain;
  // As it stands, in its own space (its root at the origin): its joint, its
  // ankle, and where the middle of its sole is, on the ground.
  readonly joint: V;
  readonly ankle: V;
  readonly paw: V;
  readonly upper: number; // m from the root to the joint
  readonly lower: number; // and from the joint to the ankle
  // Its claws' foremost point, low on the paw, in its own space: as the paw
  // turns its toes down, they drop by this much more than the paw.
  readonly toe: V;
  // The way its joint stands out from the line from its root to its ankle:
  // forward for a knee, back for an elbow.
  private readonly pole: V;

  constructor(spec: LegSpec, side: 1 | -1, palette: Palette, random: () => number, materials: Materials) {
    super();
    this.spec = spec;
    this.side = side;
    const skin = new Sculpt(random, 0.025);
    const sh = legShell(spec, side);
    const face: Face = (a, b, c, inside, t) => skin.facing(a, b, c, inside, t);
    // Its root, sunk in the body, closed.
    ringLoft(
      face,
      sh.rings,
      (r, k) => {
        const a = angleOf(k, CORNERS);
        if (spec.front && r >= 2 && Math.abs(a - Math.PI) < BACK) return blend('plate', 'plateShade', 0.3);
        return blend('scale', 'scaleDark', hash(r, k, 9) < 0.5 ? 0.4 : 0.6);
      },
      lid(sh, false),
    );
    scales(skin, sh, spec, random);
    ankleRing(skin, sh, random);
    if (spec.front) tufts(skin, sh);
    // The paw, its toes and claws ride on the foot's bone, whole: weighed
    // along the leg, a hind paw's toes (ahead of the knee) went with the
    // shin and dipped into the ground as it turned.
    const pawSkin = new Sculpt(random, 0.025);
    paw(pawSkin, spec, side, random);
    const at = placed(spec, side);
    this.chain = chainOf(this, side < 0 ? 'leftLeg' : 'rightLeg', [at.root, at.joint, at.ankle]);
    this.add(
      ...bendingMeshes(this.chain, palette, [
        [skin, materials.skin],
        [pawSkin, materials.skin, 2],
      ], BLEND),
    );
    this.joint = at.joint.clone().sub(at.root);
    this.ankle = at.ankle.clone().sub(at.root);
    this.paw = at.paw.clone().sub(at.root);
    this.upper = this.joint.length();
    this.lower = this.ankle.distanceTo(this.joint);
    const line = this.ankle.clone().normalize();
    this.pole = this.joint.clone().addScaledVector(line, -this.joint.dot(line)).normalize();
    this.toe = new THREE.Vector3(0, 0, -Infinity);
    const meshes = this.children.filter((c) => c instanceof THREE.Mesh);
    const position = meshes[meshes.length - 1].geometry.getAttribute('position');
    const p = new THREE.Vector3();
    for (let i = 0; i < position.count; i++) {
      p.fromBufferAttribute(position, i);
      if (p.y < this.paw.y + 0.1 && p.z > this.toe.z) this.toe.copy(p);
    }
  }

  // How much lower than the paw its claws reach, the paw turned `toes` rad
  // toes down about its ankle: what a swinging paw is raised by, so its
  // claws never dip.
  toesDrop(toes: number): number {
    const ahead = this.toe.z - this.ankle.z;
    const below = this.ankle.y - this.toe.y;
    return Math.max(0, ahead * Math.sin(toes) - below * (1 - Math.cos(toes)));
  }

  // The most its ankle reaches from its root.
  get reachMost(): number {
    return (this.upper + this.lower) * REACH;
  }

  // Puts its ankle at `ankle` (in its own space: its root at the origin, as
  // the body holds it), as near as it reaches, its joint bending the way it
  // does as built, and turns its foot to `foot` (its turn from as built, in
  // the same space).
  reach(ankle: V, foot: THREE.Quaternion): void {
    const { upper, lower } = this;
    const span = THREE.MathUtils.clamp(ankle.length(), Math.abs(upper - lower) * 1.001 + 1e-6, (upper + lower) * REACH);
    const line = _line.copy(ankle).normalize();
    if (!Number.isFinite(line.x)) line.copy(this.ankle).normalize();
    // The joint, out from the line toward the pole.
    const out = _out.copy(this.pole).addScaledVector(line, -this.pole.dot(line)).normalize();
    const along = (upper * upper - lower * lower + span * span) / (2 * span);
    const joint = _joint.copy(line).multiplyScalar(along).addScaledVector(out, Math.sqrt(Math.max(0, upper * upper - along * along)));
    const end = _end.copy(line).multiplyScalar(span);
    // Each bone turned from as built, keeping the leg's plane.
    const restNormal = _n0.crossVectors(this.ankle, this.pole).normalize();
    const normal = _n1.crossVectors(line, out).normalize();
    const thigh = turnBetween(this.joint, restNormal, joint, normal, _q1);
    const shin = turnBetween(_d0.subVectors(this.ankle, this.joint), restNormal, _d1.subVectors(end, joint), normal, _q2);
    const [hip, knee, wrist] = this.chain.bones;
    hip.quaternion.copy(thigh);
    knee.quaternion.copy(thigh).invert().multiply(shin);
    wrist.quaternion.copy(shin).invert().multiply(foot);
  }
}

// Of a leg's length, the most it stretches.
const REACH = 0.998;

const _line = new THREE.Vector3();
const _out = new THREE.Vector3();
const _joint = new THREE.Vector3();
const _end = new THREE.Vector3();
const _n0 = new THREE.Vector3();
const _n1 = new THREE.Vector3();
const _d0 = new THREE.Vector3();
const _d1 = new THREE.Vector3();
const _q1 = new THREE.Quaternion();
const _q2 = new THREE.Quaternion();
const _m0 = new THREE.Matrix4();
const _m1 = new THREE.Matrix4();
const _c = new THREE.Vector3();

// The turn that takes a bone running `from` in a plane facing `fromNormal`
// to running `to` in one facing `toNormal`.
function turnBetween(from: V, fromNormal: V, to: V, toNormal: V, out: THREE.Quaternion): THREE.Quaternion {
  const a = from.clone().normalize();
  const b = to.clone().normalize();
  _m0.makeBasis(a, fromNormal, _c.crossVectors(a, fromNormal));
  _m1.makeBasis(b, toNormal, _c.crossVectors(b, toNormal));
  return out.setFromRotationMatrix(_m1.multiply(_m0.transpose()));
}

// Leaf scales down the leg, pointing down it, smaller toward the ankle; a
// foreleg's cream back left bare.
function scales(skin: Sculpt, sh: Shell, spec: LegSpec, random: () => number): void {
  const [top, bottom] = spec.scale;
  cover(skin, sh, random, {
    from: 0.15,
    to: 3.85,
    half: (u) => (spec.front && u > 1.6 ? Math.PI - BACK - 0.05 : Math.PI),
    length: (u) => THREE.MathUtils.lerp(top, bottom, THREE.MathUtils.clamp((u - 0.5) / 3.3, 0, 1)),
    tone: () => bodyScaleTone(random),
    way: 1,
    width: 0.9,
    along: 0.62,
    across: 0.78,
    droop: 0.2,
    lift: 0.03,
    curl: 0.03,
    ridge: 0.1,
    contrast: 0.4,
  });
}

// A ring of bigger leaves round the ankle, hanging over the top of the paw,
// as the claw sheet's ankle scale ring overlaps it.
function ankleRing(skin: Sculpt, sh: Shell, random: () => number): void {
  const count = 8;
  for (let i = 0; i < count; i++) {
    const at = shellAt(sh, 4.05, (Math.PI * 2 * (i + 0.5)) / count);
    const down = at.along.clone().addScaledVector(at.normal, 0.45);
    const length = 0.24 + 0.04 * random();
    const roles: Tone[] = [tone('scale'), tone('scaleLight'), blend('gold', 'scaleLight', 0.4), tone('scaleDark')];
    leaf(skin, {
      base: at.point,
      along: down,
      normal: at.normal,
      length,
      width: length * 0.55,
      curl: 0.18,
      sink: 0.02,
      top: roles[i % roles.length],
    });
  }
}

// Cream tufts down the back of a foreleg, at the elbow and the wrist,
// pointing back and down, as the overview draws them.
function tufts(skin: Sculpt, sh: Shell): void {
  const list = [
    { u: 1.9, angle: 0, length: 0.3 },
    { u: 2.15, angle: 0.4, length: 0.26 },
    { u: 2.15, angle: -0.4, length: 0.26 },
    { u: 3.7, angle: 0.25, length: 0.22 },
    { u: 3.7, angle: -0.25, length: 0.22 },
  ];
  for (const t of list) {
    const at = shellAt(sh, t.u, Math.PI + t.angle);
    leaf(skin, {
      base: at.point,
      along: at.along.clone().addScaledVector(at.normal, 0.7),
      normal: at.normal,
      length: t.length,
      width: t.length * 0.34,
      curl: 0.25,
      sink: 0.02,
      top: t.length > 0.25 ? tone('plate') : blend('plate', 'gold', 0.3),
    });
  }
}

// How far a claw drops, and reaches forward, from its root: claw() turns
// it down `dip` at its root and `hook` more over its four facets' lengths.
function clawReach(length: number, hook: number, dip: number): { drop: number; ahead: number } {
  let drop = 0;
  let ahead = 0;
  for (let i = 1; i <= 4; i++) {
    const angle = dip + (i * hook) / 3;
    drop += (length / 4) * Math.sin(angle);
    ahead += (length / 4) * Math.cos(angle);
  }
  return { drop, ahead };
}

// The paw: a rounded block on the ground under the ankle, green on top and
// cream beneath; three toes forward, splayed, and a small one on its
// inside, each with an ivory claw whose tip just reaches the ground.
function paw(skin: Sculpt, spec: LegSpec, side: 1 | -1, random: () => number): void {
  const { paw: at } = placed(spec, side);
  const [length, width, height] = spec.pawSize;
  const stations = [
    { z: -0.5, w: 0.55, up: 0.55, down: 0.5, y: 0.52 },
    { z: -0.18, w: 0.92, up: 1, down: 0.95, y: 0.5 },
    { z: 0.18, w: 1, up: 0.9, down: 0.95, y: 0.48 },
    { z: 0.44, w: 0.9, up: 0.66, down: 0.8, y: 0.42 },
  ];
  const rings = stations.map((s) =>
    sectionRing(v(at.x, s.y * height, at.z + s.z * length), v(side, 0, 0), v(0, 1, 0), (s.w * width) / 2, (s.up * height) / 2, (s.down * height) / 2, PAW_CORNERS, 2.5),
  );
  const face: Face = (a, b, c, inside, t) => skin.facing(a, b, c, inside, t);
  ringLoft(
    face,
    rings,
    (r, k) => {
      const a = angleOf(k, PAW_CORNERS);
      const fromTop = Math.min(a, Math.PI * 2 - a);
      if (fromTop < 1.0 && r < 2) return tone('scale');
      if (fromTop > 2.4) return blend('plate', 'plateShade', 0.6);
      return hash(r, k, 13) < 0.5 ? tone('plate') : blend('plate', 'plateShade', 0.25);
    },
    v(at.x, 0.5 * height, at.z - 0.6 * length),
    v(at.x, 0.4 * height, at.z + 0.5 * length),
  );
  // The toes: the claw's drop sets how high each toe's tip stands, so the
  // claw's point just reaches the ground.
  const [clawLength, hook] = spec.claw;
  const { drop } = clawReach(clawLength, hook, CLAW_DIP);
  const front = at.z + 0.42 * length;
  const toes = [
    { across: -0.62, turn: -0.3, size: 0.92 },
    { across: 0, turn: 0, size: 1 },
    { across: 0.62, turn: 0.3, size: 0.92 },
  ];
  for (const t of toes) {
    const way = v(0, 0, 1).applyAxisAngle(v(0, 1, 0), t.turn * side);
    const radius = 0.065 * t.size;
    const tipY = drop + 0.006;
    const root = v(at.x + side * t.across * width * 0.36, Math.max(0.42 * height, tipY + 0.035), front - 0.05);
    const knuckle = root.clone().addScaledVector(way, spec.toe * 0.5).setY((root.y + tipY) / 2 + 0.012);
    const tip = root.clone().addScaledVector(way, spec.toe * t.size).setY(tipY);
    tube(skin, [root, knuckle, tip], [radius, radius * 0.95, radius * 0.85], 6, random, (r) => (r === 1 ? blend('plate', 'plateShade', 0.35) : tone('plate')), tip.clone().addScaledVector(way, radius * 0.6));
    claw(skin, tip.clone().addScaledVector(way, radius * 0.3), way.clone().setY(-Math.tan(CLAW_DIP)), clawLength * t.size, radius * 0.72, hook, random, tone('claw'));
  }
  // The small inner toe, higher on the paw's inside, its claw short.
  const inner = v(at.x - side * width * 0.42, 0.55 * height, at.z + 0.1 * length);
  const innerWay = v(-side * 0.55, -0.35, 0.75).normalize();
  const innerTip = inner.clone().addScaledVector(innerWay, 0.1);
  tube(skin, [inner, innerTip], [0.05, 0.045], 6, random, () => tone('plate'), innerTip.clone().addScaledVector(innerWay, 0.03));
  claw(skin, innerTip, innerWay, 0.1, 0.035, 0.6, random, shaded(tone('claw'), 0.96));
}
