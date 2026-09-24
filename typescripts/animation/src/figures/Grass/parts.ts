import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { bark, surface, type Theme } from '../../theme';
import { clumpGeometry, limbGeometry } from '../Tree/parts';
import type { Limb } from '../Tree/skeleton';

export { between, randomUnit, seededRandom } from '../Tree/parts';

// Helpers shared by the grasses (and the grass-like herbs) and their parts.
// A plant is first planned as lines and blobs in meters: leaf blades along
// their midribs, tubes along their axes (stems, a chive's leaves, a seed
// head), blobs (a cluster of spikelets, a floret) and woody stems. fit() then
// sizes the plan to the height and width asked for, and the parts turn it
// into meshes.

export interface GrassOptions {
  theme?: Theme;
  // The seed the plant grows from: the same seed always grows the same plant,
  // another seed another plant of the same kind. Each kind has its own default.
  seed?: number;
  // m from the ground to the plant's highest point. Each kind has its own
  // default, its usual size.
  height?: number;
  // m across, at the widest. Left out, the plant keeps its kind's shape: its
  // default width times height / default height.
  width?: number;
  // How full the plant is: its leaves and stems (and runners) times this,
  // 1 by default. Plants seen only from afar, such as a meadow's, need fewer.
  detail?: number;
}

export interface Size {
  height: number;
  width: number;
}

// The size a plant is built at, from the options and its kind's default, and
// how crowded it is: a plant wider than its kind's default grows more leaves
// and stems (crowd > 1), not only longer ones, and a narrower one fewer; and
// fewer again for a lower detail.
export function sizeOf(options: GrassOptions, kind: Size): Size & { crowd: number } {
  const height = options.height ?? kind.height;
  const width = options.width ?? (kind.width * height) / kind.height;
  return { height, width, crowd: THREE.MathUtils.clamp(width / kind.width, 0.4, 4) * (options.detail ?? 1) };
}

// `count` things, more or fewer as the plant is crowded, at least one.
export function crowded(count: number, crowd: number): number {
  return Math.max(1, Math.round(count * crowd));
}

// Materials, built once per plant from the theme. Everything green (and the
// seed heads and flowers) is one material whose colours are the vertex
// colours, since a leaf fades from its foot to its tip and a seed head is
// straw where its stem is green: white in the theme's finish, times each
// vertex's colour. Woody stems (rosemary) are bark (see bark() in theme.ts).
export function createMaterials(theme: Theme) {
  const plant = surface(theme, 0xffffff);
  plant.vertexColors = true;
  return { plant, bark: bark(theme) };
}

export type GrassMaterials = ReturnType<typeof createMaterials>;

// The colours a plant is painted in, all from the theme: leaves in its grass
// colour (its foliage colour), ripening seed heads toward its wood colour
// (straw), and flowers in its flower colour.
export function palette(theme: Theme) {
  const grass = new THREE.Color(theme.scene.grass);
  const wood = new THREE.Color(theme.colors.wood);
  return {
    grass,
    wood,
    straw: grass.clone().lerp(wood, STRAW),
    light: new THREE.Color(theme.colors.light),
    flower: new THREE.Color(theme.scene.flower),
  };
}

export type Palette = ReturnType<typeof palette>;

const STRAW = 0.4; // how far a seed head has ripened from green toward the wood colour

// A colour, lighter (k > 1) or darker.
export function tone(color: THREE.Color, k: number): THREE.Color {
  return color.clone().multiplyScalar(k);
}

export function mesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

// A width or radius along a blade or tube, as a share of its widest, for u
// from 0 at its foot to 1 at its tip.
export type Profile = (u: number) => number;

// A grass blade: narrower at the sheath round its foot, parallel-sided for
// most of its length, then tapering to a point.
export const BLADE: Profile = (u) => (u < 0.12 ? 0.6 + (0.4 * u) / 0.12 : u < 0.6 ? 1 : Math.pow((1 - u) / 0.4, 0.8));
// A thread-like leaf (fescue): the same width almost to the end.
export const THREAD: Profile = (u) => (u < 0.75 ? 1 : Math.pow((1 - u) / 0.25, 0.7));
// A short, narrow leaf with a blunt foot (a rosemary needle).
export const NEEDLE: Profile = (u) => (u < 0.2 ? 0.55 + (0.45 * u) / 0.2 : u < 0.65 ? 1 : Math.pow((1 - u) / 0.35, 0.6));
// A stem: round all along, its tip closed.
export const STEM: Profile = () => 1;

// A leaf blade: a ribbon along its midrib (`spine`, foot to tip), `width`
// across where it is widest and shaped by `profile`. `side` points across
// the blade, from its left edge to its right; the leaf's upper face is
// side x (the way it grows). `fold` raises both edges by that share of the
// half-width over the midrib, so the blade is a V in cross-section (a keeled
// leaf); a negative fold rolls the edges under. Its colour fades from `foot`
// to `tip`; its back is `back` if given (a rosemary needle's pale underside).
export interface Blade {
  spine: THREE.Vector3[];
  side: THREE.Vector3;
  width: number;
  profile: Profile;
  fold: number;
  foot: THREE.Color;
  tip: THREE.Color;
  back?: [foot: THREE.Color, tip: THREE.Color];
}

// A tube along `spine`, `radius` at its thickest and shaped by `profile`,
// closed at both ends: a stem, a chive's hollow leaf or a seed head. `bumps`
// roughens its surface by up to that share of its radius, for the spikelets
// of a seed head. `row` is the m between its rings.
export interface Tube {
  spine: THREE.Vector3[];
  radius: number;
  profile: Profile;
  bumps: number;
  row: number;
  columns: number;
  foot: THREE.Color;
  tip: THREE.Color;
}

// A lumpy blob (see clumpGeometry in Tree/parts.ts): a cluster of spikelets,
// a floret. `size` is its radii along its own axes; unturned, its own y is up.
export interface Blob {
  center: THREE.Vector3;
  size: THREE.Vector3;
  turn?: THREE.Quaternion;
  color: THREE.Color;
}

export interface Layer {
  blades: Blade[];
  tubes: Tube[];
  blobs: Blob[];
}

// A plant, planned: its leaves, its stems with what they carry, and its wood
// (woody stems in bark, each a limb as in the trees).
export interface Plan {
  leaves: Layer;
  stems: Layer;
  wood: Limb[];
}

export function emptyPlan(): Plan {
  return { leaves: { blades: [], tubes: [], blobs: [] }, stems: { blades: [], tubes: [], blobs: [] }, wood: [] };
}

// Sizes a plan, in place, so that its highest point is `size.height` above
// the ground and it is `size.width` across at the widest: heights stretch by
// one factor, and spreads by another. The foot of the plant stays at the
// origin. Widths, radii and blobs change by the square root of the two
// factors' mean, so a plant twice the size has leaves about 1.4 times as
// broad. Since blobs grow by that and not by the height, the factors are
// found in a few passes, measuring the sized plant each time.
export function fit(plan: Plan, size: Size): void {
  const layers = [plan.leaves, plan.stems];
  const blobs = layers.flatMap((layer) => layer.blobs);
  // Every point once, although parts may share one (a spikelet on the tip
  // of its stem).
  const points = new Set<THREE.Vector3>(blobs.map((b) => b.center));
  for (const layer of layers) {
    for (const b of layer.blades) for (const p of b.spine) points.add(p);
    for (const t of layer.tubes) for (const p of t.spine) points.add(p);
  }
  for (const limb of plan.wood) for (const p of limb.points) points.add(p);
  // How far each blob reaches from its center along x, y and z, turned as it is.
  const reach = blobs.map((b) => {
    const e = new THREE.Matrix4().makeRotationFromQuaternion(b.turn ?? new THREE.Quaternion()).elements;
    return new THREE.Vector3(...([0, 1, 2] as const).map((j) => Math.hypot(e[j] * b.size.x, e[4 + j] * b.size.y, e[8 + j] * b.size.z)));
  });

  const girthOf = (sx: number, sy: number) => Math.sqrt((sx + sy) / 2);
  const measure = (scale: THREE.Vector3) => {
    const box = new THREE.Box3();
    const q = new THREE.Vector3();
    for (const p of points) box.expandByPoint(q.copy(p).multiply(scale));
    const girth = girthOf(scale.x, scale.y);
    blobs.forEach((b, i) => {
      const c = b.center.clone().multiply(scale);
      box.expandByPoint(q.copy(c).addScaledVector(reach[i], girth));
      box.expandByPoint(q.copy(c).addScaledVector(reach[i], -girth));
    });
    return { height: box.max.y, across: Math.max(box.max.x - box.min.x, box.max.z - box.min.z) };
  };
  const scale = new THREE.Vector3(1, 1, 1);
  for (let pass = 0; pass < FIT_PASSES; pass++) {
    const now = measure(scale);
    scale.x *= size.width / Math.max(now.across, 1e-3);
    scale.y *= size.height / Math.max(now.height, 1e-3);
    scale.z = scale.x;
  }

  const girth = girthOf(scale.x, scale.y);
  for (const p of points) p.multiply(scale);
  for (const layer of layers) {
    for (const b of layer.blades) b.width *= girth;
    for (const t of layer.tubes) {
      t.radius *= girth;
      t.row *= girth;
    }
    for (const b of layer.blobs) b.size.multiplyScalar(girth);
  }
  for (const limb of plan.wood) for (let i = 0; i < limb.radii.length; i++) limb.radii[i] *= girth;
}

const FIT_PASSES = 4;

// Coarsens a plan, in place, for a plant seen from afar (a detail under 1):
// blades keep every other point along their midribs, and tubes get rings
// twice as far apart and fewer sides.
export function coarsen(plan: Plan): void {
  for (const layer of [plan.leaves, plan.stems]) {
    for (const b of layer.blades) {
      if (b.spine.length > 3) b.spine = b.spine.filter((_, i) => i % 2 === 0 || i === b.spine.length - 1);
    }
    for (const t of layer.tubes) {
      t.row *= 2;
      t.columns = Math.max(4, Math.round(0.6 * t.columns));
    }
  }
}

// A layer as one geometry, or null when it is empty.
export function layerGeometry(layer: Layer, random: () => number): THREE.BufferGeometry | null {
  const parts = [
    ...layer.blades.map(bladeGeometry),
    ...layer.tubes.map((t) => tubeGeometry(t, random)),
    ...layer.blobs.map((b) => blobGeometry(b, random)),
  ];
  if (parts.length === 0) return null;
  const geo = mergeGeometries(parts);
  for (const part of parts) part.dispose();
  return geo;
}

// Woody stems as one geometry in bark, or null when there are none.
export function woodGeometry(limbs: Limb[]): THREE.BufferGeometry | null {
  if (limbs.length === 0) return null;
  const parts = limbs.map((limb) => limbGeometry(limb.points, limb.radii));
  const geo = mergeGeometries(parts);
  for (const part of parts) part.dispose();
  return geo;
}

const UP = new THREE.Vector3(0, 1, 0);
const LIFT = 1; // how far a blade's normals bend toward straight up, so both its faces are lit much like the ground

// A blade: three vertices across each row (left edge, midrib, right edge),
// drawn twice, the front and the back each with its own vertices, normals
// and colours, since a thin leaf is seen from both sides. Both sets of
// normals lean up (LIFT), so a leaf seen from behind is lit like the tuft
// round it instead of dark.
function bladeGeometry(b: Blade): THREE.BufferGeometry {
  const rows = b.spine.length - 1;
  const positions: number[] = [];
  const normals: number[] = [];
  const colors: number[] = [];
  const tangent = new THREE.Vector3();
  const side = new THREE.Vector3();
  const up = new THREE.Vector3();
  const edge = new THREE.Vector3();
  const n = new THREE.Vector3();
  const color = new THREE.Color();
  const back = b.back ?? [b.foot, b.tip];

  for (const face of [1, -1]) {
    const [foot, tip] = face > 0 ? [b.foot, b.tip] : back;
    for (let i = 0; i <= rows; i++) {
      const u = i / rows;
      const p = b.spine[i];
      tangent.subVectors(b.spine[Math.min(rows, i + 1)], b.spine[Math.max(0, i - 1)]).normalize();
      side.copy(b.side).addScaledVector(tangent, -b.side.dot(tangent)).normalize();
      up.crossVectors(side, tangent); // the leaf's upper face
      const half = (b.width / 2) * b.profile(u);
      color.copy(foot).lerp(tip, Math.pow(u, 0.6));
      for (const across of [-1, 0, 1]) {
        edge.copy(p).addScaledVector(side, across * half).addScaledVector(up, Math.abs(across) * b.fold * half);
        positions.push(edge.x, edge.y, edge.z);
        // The two halves of the V lean toward each other.
        n.copy(up).addScaledVector(side, -across * b.fold).normalize().multiplyScalar(face).addScaledVector(UP, LIFT).normalize();
        normals.push(n.x, n.y, n.z);
        colors.push(color.r, color.g, color.b);
      }
    }
  }

  // Wound so the front faces point along the upper face (side x tangent),
  // the back's the other way.
  const at = (i: number, j: number) => 3 * i + j;
  const back0 = 3 * (rows + 1);
  const indices: number[] = [];
  for (let i = 0; i < rows; i++) {
    for (let j = 0; j < 2; j++) {
      const [a, c, d, e] = [at(i, j), at(i, j + 1), at(i + 1, j), at(i + 1, j + 1)];
      indices.push(a, c, d, c, e, d);
      indices.push(back0 + a, back0 + d, back0 + c, back0 + c, back0 + d, back0 + e);
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  geo.setIndex(indices);
  return geo;
}

// A tube: rings of vertices along a smooth curve through the spine, closed
// at each end to a point on its axis. Its frames are carried along the curve
// (parallel transport), so it never twists.
function tubeGeometry(t: Tube, random: () => number): THREE.BufferGeometry {
  const curve = new THREE.CatmullRomCurve3(t.spine, false, 'centripetal');
  const length = curve.getLength();
  const rows = Math.max(t.spine.length - 1, Math.ceil(length / t.row), 2);
  const columns = t.columns;
  const radius = (u: number) => t.radius * t.profile(THREE.MathUtils.clamp(u, 0, 1));

  const positions: number[] = [];
  const normals: number[] = [];
  const colors: number[] = [];
  const tangent = curve.getTangentAt(0);
  const normal = new THREE.Vector3().crossVectors(tangent, Math.abs(tangent.y) < 0.9 ? UP : new THREE.Vector3(1, 0, 0)).normalize();
  const binormal = new THREE.Vector3();
  const out = new THREE.Vector3();
  const n = new THREE.Vector3();
  const color = new THREE.Color();
  const push = (p: THREE.Vector3, v: THREE.Vector3, u: number) => {
    positions.push(p.x, p.y, p.z);
    normals.push(v.x, v.y, v.z);
    color.copy(t.foot).lerp(t.tip, u);
    colors.push(color.r, color.g, color.b);
  };

  // The ends, points on the axis.
  const start = curve.getPointAt(0);
  push(start, tangent.clone().negate(), 0);
  const du = 0.5 / rows;
  for (let i = 0; i <= rows; i++) {
    const u = i / rows;
    const center = curve.getPointAt(u);
    tangent.copy(curve.getTangentAt(u));
    normal.addScaledVector(tangent, -normal.dot(tangent)).normalize();
    binormal.crossVectors(tangent, normal);
    const r = radius(u);
    // How fast the radius changes along the tube tilts its normals.
    const slope = (radius(u + du) - radius(u - du)) / (2 * du * length);
    const bumps = Array.from({ length: columns }, () => 1 + t.bumps * (2 * random() - 1));
    for (let j = 0; j <= columns; j++) {
      const angle = (j / columns) * 2 * Math.PI;
      out.copy(normal).multiplyScalar(Math.cos(angle)).addScaledVector(binormal, Math.sin(angle));
      n.copy(out).addScaledVector(tangent, -slope).normalize();
      push(center.clone().addScaledVector(out, r * bumps[j % columns]), n, u);
    }
  }
  const end = curve.getPointAt(1);
  push(end, curve.getTangentAt(1), 1);

  // Wound so the faces point out: (around) x (along) is outward.
  const ring = (i: number, j: number) => 1 + i * (columns + 1) + j;
  const last = positions.length / 3 - 1;
  const indices: number[] = [];
  for (let j = 0; j < columns; j++) indices.push(0, ring(0, j + 1), ring(0, j));
  for (let i = 0; i < rows; i++) {
    for (let j = 0; j < columns; j++) {
      indices.push(ring(i, j), ring(i, j + 1), ring(i + 1, j));
      indices.push(ring(i, j + 1), ring(i + 1, j + 1), ring(i + 1, j));
    }
  }
  for (let j = 0; j < columns; j++) indices.push(ring(rows, j), ring(rows, j + 1), last);

  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  geo.setIndex(indices);
  return geo;
}

const SMALL_BLOB = 0.02; // m: a blob smaller than this each way is drawn with 80 faces, not 320
const SMALL_DETAIL = 1;

// A blob: a tree's leaf clump (shaded darker underneath) in the blob's colour.
function blobGeometry(b: Blob, random: () => number): THREE.BufferGeometry {
  const small = Math.max(b.size.x, b.size.y, b.size.z) < SMALL_BLOB;
  const geo = clumpGeometry({ center: b.center, size: b.size, turn: b.turn }, random, small ? SMALL_DETAIL : undefined);
  const color = geo.getAttribute('color');
  for (let i = 0; i < color.count; i++) {
    color.setXYZ(i, color.getX(i) * b.color.r, color.getY(i) * b.color.g, color.getZ(i) * b.color.b);
  }
  return geo;
}
