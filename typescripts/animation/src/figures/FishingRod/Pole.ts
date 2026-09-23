import * as THREE from 'three';
import { BARK_TILE } from '../../textures/bark';
import { cutFace, LINE_RADIUS, mesh, type RodMaterials } from './parts';

// The pole: a branch cut from a tree (ranting kayu), 2 m long, thick at the
// butt and thin at the tip, with the gentle crooks of real wood and a slight
// droop toward the tip. It is covered in bark and shows pale fresh wood at
// both cut ends. Small details keep it a real branch: two stubs where side
// shoots were cut off, a side twig that snapped and hangs from its splintered
// break, and near the tip a leafy twig and a leaf growing from the pole. A
// few turns of line are lashed round it just below the tip, where the
// fishing line is tied.
//
// It lies along +z, butt at the back, and "up" is +y (the way it is held
// level). Its origin is on its axis at HAND, where the hand holds it.

export const POLE_LENGTH = 2.0;
export const HAND = 0.3; // from the butt
const BUTT_RADIUS = 0.026; // 5.2 cm across: a stout branch, as the user asked
const TIP_RADIUS = 0.009;
const TAPER = 1.2; // above 1 the pole thins quickly near the butt and slowly toward the tip

// The crooks: how far the axis strays from a straight line, sideways (x) and
// up (y), at shares of the length. The tip droops a little under its weight.
const CROOKS: [share: number, x: number, y: number][] = [
  [0, 0, 0],
  [0.2, 0.01, 0.004],
  [0.45, -0.012, 0.006],
  [0.7, 0.01, -0.004],
  [1, -0.015, -0.03],
];

// Side shoots are placed by where along the pole they grow (m from the
// butt), which way round it (radians from straight up, positive toward +x)
// and how far they lean toward the tip (radians from the pole's axis). Their
// thickness is a share of the pole's radius there.

// Stubs where side shoots were cut off, and how far they stick out.
const STUBS = [
  { at: 0.62, around: 1.9, lean: 0.8, length: 0.035 },
  { at: 1.25, around: -1.2, lean: 0.7, length: 0.028 },
];
const STUB_THICKNESS = 0.5;

// A side twig that snapped: the part still growing from the pole, then the
// broken-off part hanging down from the break by a few fibres, with pale
// splinters sticking out of the break.
const BROKEN = {
  at: 0.95,
  around: -2.2,
  lean: 0.75,
  thickness: 0.6,
  length: 0.07, // from the pole's surface to the break
  hanging: 0.09, // the broken-off part
  droop: 1.1, // radians the broken-off part hangs down from the twig's line
};
const SPLINTERS = [
  { length: 0.016, spread: 0.35, turn: 0 },
  { length: 0.011, spread: 0.5, turn: 2.1 },
  { length: 0.02, spread: 0.2, turn: 4.2 },
];

// A leafy twig near the tip, and the leaves: on the twig (at a share of its
// length) or on the pole itself. A twig leaf points `turn` radians to the
// side of the twig's line, and each leaf droops `droop` radians.
const LEAF_TWIG = { at: 1.72, around: 0.7, lean: 0.55, thickness: 0.55, length: 0.1 };
const TWIG_LEAVES = [
  { share: 1, turn: 0, droop: 0.15, length: 0.09 },
  { share: 0.55, turn: 1.3, droop: 0.35, length: 0.08 },
];
const POLE_LEAF = { at: 1.86, around: -0.9, lean: 1.0, length: 0.085, droop: 0.3 };
const LEAF_WIDTH = 0.4; // share of the leaf's length
const LEAF_CURL = 0.18; // the tip curls down by this share of the length
const LEAF_FOLD = 0.2; // each half rises from the midrib by this share of its width
const PETIOLE = { length: 0.012, radius: 0.0012 }; // the leaf's stalk

const LASHING = { at: POLE_LENGTH - 0.03, length: 0.02 }; // turns of line, just below the tip

const ROWS = 90; // along the pole
const COLUMNS = 14; // around it
const UP = new THREE.Vector3(0, 1, 0);

// The pole's axis, from butt to tip.
const axis = new THREE.CatmullRomCurve3(
  CROOKS.map(([share, x, y]) => new THREE.Vector3(x, y, share * POLE_LENGTH - HAND)),
);

// The pole's radius at u, 0 at the butt to 1 at the tip.
function radius(u: number): number {
  return TIP_RADIUS + (BUTT_RADIUS - TIP_RADIUS) * (1 - u) ** TAPER;
}

// u of a point `meters` from the butt, along the axis.
const along = (meters: number) => Math.min(1, meters / axis.getLength());

// Where a side shoot grows: the point on the axis and the direction it grows.
function sprout(at: number, around: number, lean: number) {
  const u = along(at);
  const tangent = axis.getTangentAt(u);
  const sideways = new THREE.Vector3(Math.sin(around), Math.cos(around), 0);
  sideways.addScaledVector(tangent, -sideways.dot(tangent)).normalize();
  const direction = tangent.clone().multiplyScalar(Math.cos(lean)).addScaledVector(sideways, Math.sin(lean));
  return { base: axis.getPointAt(u), direction, radius: radius(u) };
}

// A round, tapering piece of branch from `from` along `direction`.
function branch(from: THREE.Vector3, direction: THREE.Vector3, length: number, r0: number, r1: number, mat: THREE.Material) {
  const geo = new THREE.CylinderGeometry(r1, r0, length, 10);
  geo.translate(0, length / 2, 0); // from its base up
  const piece = mesh(geo, mat);
  piece.position.copy(from);
  piece.quaternion.setFromUnitVectors(UP, direction.clone().normalize());
  return piece;
}

// A leaf `length` long, from its stalk (the origin) along +x, its face
// toward +z: a pointed oval, its tip curling down and its two halves folded
// up a little along the midrib. A grid of points, so it bends smoothly.
function leafGeometry(length: number): THREE.BufferGeometry {
  const rows = 12;
  const columns = 4;
  const positions: number[] = [];
  for (let i = 0; i <= rows; i++) {
    const t = i / rows;
    const half = ((LEAF_WIDTH * length) / 2) * Math.sin(Math.PI * t ** 0.8); // widest a little past the middle
    for (let j = 0; j <= columns; j++) {
      const s = (j / columns) * 2 - 1; // -1 one edge .. 1 the other
      const z = -LEAF_CURL * length * t * t + LEAF_FOLD * half * Math.abs(s);
      positions.push(t * length, s * half, z);
    }
  }
  const at = (i: number, j: number) => i * (columns + 1) + j;
  const indices: number[] = [];
  for (let i = 0; i < rows; i++) {
    for (let j = 0; j < columns; j++) {
      indices.push(at(i, j), at(i + 1, j), at(i, j + 1));
      indices.push(at(i, j + 1), at(i + 1, j), at(i + 1, j + 1));
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setIndex(indices);
  geo.computeVertexNormals();
  return geo;
}

// A leaf on its stalk at `from`, pointing along `direction`, its face turned
// up as far as the direction allows.
function leaf(from: THREE.Vector3, direction: THREE.Vector3, length: number, m: RodMaterials): THREE.Group {
  const x = direction.clone().normalize();
  const z = UP.clone().addScaledVector(x, -UP.dot(x)).normalize();
  const y = z.clone().cross(x);
  const group = new THREE.Group();
  group.position.copy(from);
  group.quaternion.setFromRotationMatrix(new THREE.Matrix4().makeBasis(x, y, z));
  const stalk = branch(new THREE.Vector3(), new THREE.Vector3(1, 0, 0), PETIOLE.length, PETIOLE.radius, PETIOLE.radius, m.bark);
  const blade = mesh(leafGeometry(length), m.leaf);
  blade.position.x = PETIOLE.length;
  group.add(stalk, blade);
  return group;
}

// `direction` turned `turn` radians round `about`, then drooped `droop`
// radians toward straight down.
function aim(direction: THREE.Vector3, about: THREE.Vector3, turn: number, droop: number): THREE.Vector3 {
  const d = direction.clone().applyAxisAngle(about.clone().normalize(), turn);
  return d.lerp(new THREE.Vector3(0, -1, 0), Math.sin(droop) * 0.6).normalize();
}

// Where the line hangs from: under the lashing.
export function lineTie(): THREE.Vector3 {
  const u = along(LASHING.at);
  return axis.getPointAt(u).add(new THREE.Vector3(0, -(radius(u) + 2 * LINE_RADIUS), 0));
}

export class Pole extends THREE.Group {
  constructor(m: RodMaterials) {
    super();
    this.name = 'pole';

    // The bark: rings of points around the axis, each ring turned with the
    // axis (its Frenet frame). Normals point straight out from the axis, and
    // texture coordinates are meters along the pole and one bark tile round
    // it, so the bark wraps without a seam.
    const frames = axis.computeFrenetFrames(ROWS, false);
    const positions: number[] = [];
    const normals: number[] = [];
    const uvs: number[] = [];
    const out = new THREE.Vector3();
    for (let i = 0; i <= ROWS; i++) {
      const u = i / ROWS;
      const center = axis.getPointAt(u);
      const r = radius(u);
      for (let j = 0; j <= COLUMNS; j++) {
        const angle = (j / COLUMNS) * 2 * Math.PI;
        out.copy(frames.normals[i]).multiplyScalar(Math.cos(angle)).addScaledVector(frames.binormals[i], Math.sin(angle));
        positions.push(center.x + r * out.x, center.y + r * out.y, center.z + r * out.z);
        normals.push(out.x, out.y, out.z);
        uvs.push(u * axis.getLength(), (j / COLUMNS) * BARK_TILE);
      }
    }
    // Wound so the faces point out: (around) x (along) is outward for a
    // right-handed tangent, normal, binormal frame.
    const at = (i: number, j: number) => i * (COLUMNS + 1) + j;
    const indices: number[] = [];
    for (let i = 0; i < ROWS; i++) {
      for (let j = 0; j < COLUMNS; j++) {
        indices.push(at(i, j), at(i, j + 1), at(i + 1, j));
        indices.push(at(i, j + 1), at(i + 1, j + 1), at(i + 1, j));
      }
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geo.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3));
    geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
    geo.setIndex(indices);
    this.add(mesh(geo, m.bark));

    // The cut ends.
    this.add(cutFace(radius(0), axis.getPointAt(0), axis.getTangentAt(0).negate(), m.cut));
    this.add(cutFace(radius(1), axis.getPointAt(1), axis.getTangentAt(1), m.cut));

    // The stubs: short shoots growing out of the axis, cut off clean.
    for (const stub of STUBS) {
      const { base, direction, radius: r } = sprout(stub.at, stub.around, stub.lean);
      const thickness = STUB_THICKNESS * r;
      const length = r + stub.length; // from the axis
      this.add(
        branch(base, direction, length, thickness, thickness * 0.8, m.bark),
        cutFace(thickness * 0.8, base.clone().addScaledVector(direction, length), direction, m.cut),
      );
    }

    // The broken twig: grown out to the break, which shows pale wood and
    // splinters, and the broken-off part hanging down from it.
    {
      const { base, direction, radius: r } = sprout(BROKEN.at, BROKEN.around, BROKEN.lean);
      const thickness = BROKEN.thickness * r;
      const length = r + BROKEN.length;
      const at = base.clone().addScaledVector(direction, length);
      this.add(
        branch(base, direction, length, thickness, thickness * 0.85, m.bark),
        cutFace(thickness * 0.85, at, direction, m.cut),
      );
      // Splinters: thin pale slivers standing out of the break, splayed.
      const side = new THREE.Vector3().crossVectors(direction, UP).normalize();
      for (const s of SPLINTERS) {
        const splay = side.clone().applyAxisAngle(direction, s.turn);
        const way = direction.clone().multiplyScalar(Math.cos(s.spread)).addScaledVector(splay, Math.sin(s.spread));
        const sliver = new THREE.ConeGeometry(thickness * 0.3, s.length, 5);
        sliver.translate(0, s.length / 2, 0);
        const piece = mesh(sliver, m.cut);
        piece.position.copy(at).addScaledVector(splay, thickness * 0.4);
        piece.quaternion.setFromUnitVectors(UP, way);
        this.add(piece);
      }
      // The broken-off part hangs from the break's lower edge, still held by
      // a few fibres, tapering to its natural tip.
      const hangs = aim(direction, side, 0, BROKEN.droop);
      const from = at.clone().addScaledVector(hangs, thickness * 0.6);
      this.add(branch(from, hangs, BROKEN.hanging, thickness * 0.8, thickness * 0.35, m.bark));
    }

    // The leafy twig near the tip, its leaves, and a leaf on the pole.
    {
      const { base, direction, radius: r } = sprout(LEAF_TWIG.at, LEAF_TWIG.around, LEAF_TWIG.lean);
      const thickness = LEAF_TWIG.thickness * r;
      const length = r + LEAF_TWIG.length;
      this.add(branch(base, direction, length, thickness, thickness * 0.4, m.bark));
      for (const l of TWIG_LEAVES) {
        const from = base.clone().addScaledVector(direction, r + LEAF_TWIG.length * l.share);
        this.add(leaf(from, aim(direction, UP, l.turn, l.droop), l.length, m));
      }
      const pole = sprout(POLE_LEAF.at, POLE_LEAF.around, POLE_LEAF.lean);
      const from = pole.base.clone().addScaledVector(pole.direction, pole.radius);
      this.add(leaf(from, aim(pole.direction, UP, 0, POLE_LEAF.droop), POLE_LEAF.length, m));
    }

    // The lashing: a band of wound line where the fishing line is tied.
    const u = along(LASHING.at);
    const lashing = mesh(
      new THREE.CylinderGeometry(radius(u) + 2 * LINE_RADIUS, radius(u) + 2 * LINE_RADIUS, LASHING.length, 14),
      m.line,
    );
    lashing.position.copy(axis.getPointAt(u));
    lashing.quaternion.setFromUnitVectors(UP, axis.getTangentAt(u));
    this.add(lashing);
  }
}
