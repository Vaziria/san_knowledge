import * as THREE from 'three';
import { seededRandom } from '../parts';

// What the forest lake's animals share (ForestLake/animals/). They are built
// as the user's reference sheets draw them ("create animal figure from this
// reference, create as is, dont let old style influence this design, use
// buffer geometry for better result"): low poly, every face flat, in the
// sheets' own colours rather than the theme's, like the forest lake's other
// assets (../parts.ts).
//
// Each animal is sculpted here from BufferGeometry: flat triangles, each
// with a tone (a colour role of the animal's palette, or a mix of two, a
// little lighter or darker) and, for what bends, the bones it follows. A
// palette maps the roles to colours, one palette for each of the sheet's
// colour variations, so an animal is recoloured by painting its faces again
// from another palette (paint()), without building it again.

export { seededRandom };

// ---------------------------------------------------------------- colours

// Colour roles to sRGB hex, as picked from a reference sheet.
export type Palette = Readonly<Record<string, number>>;

// A face's colour: its role, mixed `mix` of the way toward the role `to`,
// times `shade` (over 1 lighter, under 1 darker).
export interface Tone {
  role: string;
  to?: string;
  mix?: number;
  shade?: number;
}

export function tone(role: string, shade = 1): Tone {
  return { role, shade };
}

export function blend(role: string, to: string, mix: number, shade = 1): Tone {
  return { role, to, mix, shade };
}

const linear = new WeakMap<Palette, Map<string, THREE.Color>>();

function roleColor(palette: Palette, role: string): THREE.Color {
  let cache = linear.get(palette);
  if (!cache) linear.set(palette, (cache = new Map()));
  let c = cache.get(role);
  if (!c) {
    const hex = palette[role];
    if (hex === undefined) throw new Error(`the palette has no colour for "${role}"`);
    cache.set(role, (c = new THREE.Color(hex)));
  }
  return c;
}

// A tone's colour in a palette, as three.js keeps colours (linear).
export function toneColor(t: Tone, palette: Palette, target = new THREE.Color()): THREE.Color {
  target.copy(roleColor(palette, t.role));
  if (t.to && t.mix) target.lerp(roleColor(palette, t.to), t.mix);
  return target.multiplyScalar(t.shade ?? 1);
}

// Paints a sculpted geometry's faces from a palette (another colour
// variation of the same animal).
export function paint(geometry: THREE.BufferGeometry, palette: Palette): void {
  const tones = geometry.userData.tones as Tone[] | undefined;
  if (!tones) return;
  const colors = geometry.getAttribute('color') as THREE.BufferAttribute;
  const c = new THREE.Color();
  for (let i = 0; i < tones.length; i++) {
    toneColor(tones[i], palette, c);
    colors.setXYZ(i, c.r, c.g, c.b);
  }
  colors.needsUpdate = true;
}

// ---------------------------------------------------------------- materials

// Matte fur, feathers and scales: coloured by the faces' tones, flat shaded
// on the graphics card (so a face that bends with its bones stays flat).
export function coat(): THREE.MeshStandardMaterial {
  const material = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.86, metalness: 0 });
  material.name = 'coat';
  return material;
}

// Wet and shiny: eyes, a nose, hooves, a beak.
export function gloss(roughness = 0.28): THREE.MeshStandardMaterial {
  const material = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness, metalness: 0 });
  material.name = 'gloss';
  return material;
}

// A thin sheet seen from both sides (fins, a butterfly's wings), opaque.
export function thin(roughness = 0.8): THREE.MeshStandardMaterial {
  const material = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness, metalness: 0, side: THREE.DoubleSide });
  material.shadowSide = THREE.DoubleSide;
  material.name = 'thin';
  return material;
}

// A see-through sheet (an insect's wings): lit, both sides, writing no depth.
export function sheer(opacity: number): THREE.MeshStandardMaterial {
  const material = new THREE.MeshStandardMaterial({
    vertexColors: true,
    flatShading: true,
    roughness: 0.35,
    metalness: 0,
    side: THREE.DoubleSide,
    transparent: true,
    opacity,
    depthWrite: false,
  });
  material.name = 'sheer';
  return material;
}

// Gives off light: shows its tones as they are, unlit (a firefly's lantern).
export function glowing(): THREE.MeshBasicMaterial {
  const material = new THREE.MeshBasicMaterial({ vertexColors: true });
  material.name = 'glowing';
  return material;
}

// ---------------------------------------------------------------- sculpting

// Which bones a corner follows: bone a, bone b, and the share of b (0: all a).
export type Weights = readonly [a: number, b: number, share: number];

export const RIGID: Weights = [0, 0, 0];

const e1 = new THREE.Vector3();
const e2 = new THREE.Vector3();
const fn = new THREE.Vector3();
const fc = new THREE.Vector3();

// Flat triangles being put together into one BufferGeometry, each with its
// own three corners (so each face is flat), a tone and each corner's bones.
// Faces are given a small random shade of their own (`jitter`), as each
// facet of the sheets catches the light a little differently.
export class Sculpt {
  private readonly positions: number[] = [];
  private readonly normals: number[] = [];
  private readonly tones: Tone[] = [];
  private readonly skin: number[] = [];
  // The bones a corner follows unless its face says otherwise.
  weights: Weights = RIGID;

  constructor(
    private readonly random: () => number,
    private readonly jitter = 0.03,
  ) {}

  get triangles(): number {
    return this.tones.length / 3;
  }

  // A face, counter-clockwise seen from the side it faces.
  triangle(a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3, t: Tone, w?: readonly [Weights, Weights, Weights]): void {
    fn.crossVectors(e1.subVectors(b, a), e2.subVectors(c, a));
    const length = fn.length();
    if (length < 1e-14) return; // no area
    fn.divideScalar(length);
    const face: Tone = { ...t, shade: (t.shade ?? 1) * (1 + (this.random() * 2 - 1) * this.jitter) };
    const corners = [a, b, c];
    for (let i = 0; i < 3; i++) {
      const p = corners[i];
      this.positions.push(p.x, p.y, p.z);
      this.normals.push(fn.x, fn.y, fn.z);
      this.tones.push(face);
      const k = w?.[i] ?? this.weights;
      this.skin.push(k[0], k[1], k[2]);
    }
  }

  // A face turned to face away from `inside`, whichever way its corners run.
  facing(a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3, inside: THREE.Vector3, t: Tone, w?: readonly [Weights, Weights, Weights]): void {
    fn.crossVectors(e1.subVectors(b, a), e2.subVectors(c, a));
    fc.copy(a).add(b).add(c).divideScalar(3).sub(inside);
    if (fn.dot(fc) < 0) this.triangle(a, c, b, t, w && [w[0], w[2], w[1]]);
    else this.triangle(a, b, c, t, w);
  }

  // A face turned toward `toward` (a direction), for thin sheets.
  toward(a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3, toward: THREE.Vector3, t: Tone, w?: readonly [Weights, Weights, Weights]): void {
    fn.crossVectors(e1.subVectors(b, a), e2.subVectors(c, a));
    if (fn.dot(toward) < 0) this.triangle(a, c, b, t, w && [w[0], w[2], w[1]]);
    else this.triangle(a, b, c, t, w);
  }

  // How far along a ray from `from` (going `way`, a unit vector) it first
  // meets a face, from either side; Infinity when it meets none. For seating
  // things on a surface (an eye on a face).
  cast(from: THREE.Vector3, way: THREE.Vector3): number {
    let best = Infinity;
    const p = this.positions;
    const pv = new THREE.Vector3();
    const tv = new THREE.Vector3();
    const qv = new THREE.Vector3();
    for (let i = 0; i < p.length; i += 9) {
      e1.set(p[i + 3] - p[i], p[i + 4] - p[i + 1], p[i + 5] - p[i + 2]);
      e2.set(p[i + 6] - p[i], p[i + 7] - p[i + 1], p[i + 8] - p[i + 2]);
      pv.crossVectors(way, e2);
      const det = e1.dot(pv);
      if (Math.abs(det) < 1e-14) continue;
      tv.set(from.x - p[i], from.y - p[i + 1], from.z - p[i + 2]);
      const u = tv.dot(pv) / det;
      if (u < 0 || u > 1) continue;
      qv.crossVectors(tv, e1);
      const v = way.dot(qv) / det;
      if (v < 0 || u + v > 1) continue;
      const t = e2.dot(qv) / det;
      if (t > 0 && t < best) best = t;
    }
    return best;
  }

  // Where a ray from `from` going `way` meets the surface, or `from` itself
  // when it meets none.
  onto(from: THREE.Vector3, way: THREE.Vector3): THREE.Vector3 {
    const w = way.clone().normalize();
    const t = this.cast(from, w);
    return Number.isFinite(t) ? from.clone().addScaledVector(w, t) : from.clone();
  }

  // The geometry, painted from `palette`, with the bones' weights when
  // `skinned`.
  geometry(palette: Palette, skinned = false): THREE.BufferGeometry {
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(this.positions, 3));
    geometry.setAttribute('normal', new THREE.Float32BufferAttribute(this.normals, 3));
    geometry.setAttribute('color', new THREE.Float32BufferAttribute(new Float32Array(this.tones.length * 3), 3));
    geometry.userData.tones = this.tones;
    paint(geometry, palette);
    if (skinned) {
      const count = this.tones.length;
      const index = new Uint16Array(count * 4);
      const weight = new Float32Array(count * 4);
      for (let i = 0; i < count; i++) {
        index[4 * i] = this.skin[3 * i];
        index[4 * i + 1] = this.skin[3 * i + 1];
        weight[4 * i] = 1 - this.skin[3 * i + 2];
        weight[4 * i + 1] = this.skin[3 * i + 2];
      }
      geometry.setAttribute('skinIndex', new THREE.Uint16BufferAttribute(index, 4));
      geometry.setAttribute('skinWeight', new THREE.Float32BufferAttribute(weight, 4));
    }
    geometry.computeBoundingBox();
    geometry.computeBoundingSphere();
    return geometry;
  }
}

// ---------------------------------------------------------------- lofts

// A cross-section of a loft: centred on `at`, `w` to either side and `up`
// above and `down` below its centre, rounded as a superellipse (`n` 2 an
// ellipse, under 2 toward a diamond, over 2 toward a box). Its sideways axis
// is the loft's `side`, square to the loft's path, or its own `x` (tube()).
export interface Ring {
  at: THREE.Vector3;
  w: number;
  up: number;
  down?: number;
  n?: number;
  x?: THREE.Vector3;
  // The way the ring's plane faces, when it should not stand square to the
  // path: +z keeps a body's slices upright where its neck rises steeply,
  // so neighbouring rings never cut through each other.
  normal?: THREE.Vector3;
  bone?: Weights;
}

// How a loft ends: open, closed flat, or closed to a point this far past its
// last ring along the path.
export type Cap = 'open' | 'flat' | number;

export interface LoftOptions {
  sides: number;
  // Where corner 0 stands, radians round from the top: 0 puts a ridge along
  // the top, half a side (Math.PI / sides) a flat face.
  turn?: number;
  side?: THREE.Vector3; // what the rings' sideways axis runs along; +x by default
  // For the left of a pair (its side -x): the rings' top kept where the
  // right one's is, so each angle is the mirror of the right one's.
  mirror?: boolean;
  // The tone of the face from ring r to r + 1 whose middle is `angle` round
  // from the top (toward +x at π/2); r is -1 for the start's cap and the
  // last ring's index for the end's.
  paint: (ring: number, angle: number, corner: number) => Tone;
  // How far out each corner stands, times the ring's size (fur, a keel).
  shape?: (ring: number, angle: number, corner: number) => number;
  start?: Cap;
  end?: Cap;
}

export interface Lofted {
  rings: THREE.Vector3[][];
  frames: { x: THREE.Vector3; y: THREE.Vector3; t: THREE.Vector3 }[];
}

const TAU = Math.PI * 2;

function wrap(angle: number): number {
  return ((angle % TAU) + TAU) % TAU;
}

// A closed surface through its rings: each ring square to the path through
// their centres, joined by flat faces, alternately split one way and the
// other, as the sheets' facets run both ways.
export function loft(sculpt: Sculpt, rings: readonly Ring[], options: LoftOptions): Lofted {
  const { sides, paint: painter } = options;
  const turn = options.turn ?? 0;
  const side = options.side ?? new THREE.Vector3(1, 0, 0);
  const count = rings.length;
  const frames = rings.map((ring, r) => {
    const a = rings[Math.max(0, r - 1)].at;
    const b = rings[Math.min(count - 1, r + 1)].at;
    const t = ring.normal ? ring.normal.clone().normalize() : b.clone().sub(a).normalize();
    const along = ring.x ?? side;
    const x = along.clone().addScaledVector(t, -along.dot(t)).normalize();
    const y = new THREE.Vector3().crossVectors(t, x);
    if (options.mirror) y.negate();
    return { x, y, t };
  });
  const points = rings.map((ring, r) => {
    const { x, y } = frames[r];
    const e = 2 / (ring.n ?? 2);
    return Array.from({ length: sides }, (_, k) => {
      const angle = turn + (TAU * k) / sides;
      const s = Math.sin(angle);
      const c = Math.cos(angle);
      const sx = Math.sign(s) * Math.abs(s) ** e;
      const sy = Math.sign(c) * Math.abs(c) ** e;
      const h = c >= 0 ? ring.up : (ring.down ?? ring.up);
      const f = options.shape?.(r, wrap(angle), k) ?? 1;
      return ring.at
        .clone()
        .addScaledVector(x, ring.w * sx * f)
        .addScaledVector(y, h * sy * f);
    });
  });
  const weightsOf = (r: number) => rings[r].bone ?? sculpt.weights;
  const middle = new THREE.Vector3();
  for (let r = 0; r + 1 < count; r++) {
    middle.copy(rings[r].at).add(rings[r + 1].at).multiplyScalar(0.5);
    const [wa, wb] = [weightsOf(r), weightsOf(r + 1)];
    for (let k = 0; k < sides; k++) {
      const k1 = (k + 1) % sides;
      const a = points[r][k];
      const b = points[r][k1];
      const c = points[r + 1][k1];
      const d = points[r + 1][k];
      const t = painter(r, wrap(turn + (TAU * (k + 0.5)) / sides), k);
      if ((r + k) % 2 === 0) {
        sculpt.facing(a, b, c, middle, t, [wa, wa, wb]);
        sculpt.facing(a, c, d, middle, t, [wa, wb, wb]);
      } else {
        sculpt.facing(a, b, d, middle, t, [wa, wa, wb]);
        sculpt.facing(b, c, d, middle, t, [wa, wb, wb]);
      }
    }
  }
  const cap = (r: number, style: Cap | undefined, outward: THREE.Vector3, tones: number) => {
    if (style === undefined || style === 'open') return;
    const centre = rings[r].at;
    const tip = style === 'flat' ? centre.clone() : centre.clone().addScaledVector(outward, style);
    const inside = centre.clone().addScaledVector(outward, -Math.max(1e-4, rings[r].w * 0.1));
    const w = weightsOf(r);
    for (let k = 0; k < sides; k++) {
      const k1 = (k + 1) % sides;
      sculpt.facing(points[r][k], points[r][k1], tip, inside, painter(tones, wrap(turn + (TAU * (k + 0.5)) / sides), k), [w, w, w]);
    }
  };
  cap(0, options.start, frames[0].t.clone().negate(), -1);
  cap(count - 1, options.end, frames[count - 1].t, count - 1);
  return { rings: points, frames };
}

// Rings along a bending path whose sideways axes are carried along it without
// twisting (a tail curling up, an antler's tine), each `radii[i]` round (or
// [w, up, down]), starting from `side` (+x by default).
export function tubeRings(path: readonly THREE.Vector3[], radii: readonly (number | readonly [number, number, number?])[], side = new THREE.Vector3(1, 0, 0), bones?: readonly Weights[]): Ring[] {
  const tangents = path.map((_, i) => path[Math.min(path.length - 1, i + 1)].clone().sub(path[Math.max(0, i - 1)]).normalize());
  let x = side.clone().addScaledVector(tangents[0], -side.dot(tangents[0]));
  if (x.lengthSq() < 1e-10) x = new THREE.Vector3(0, 0, 1).addScaledVector(tangents[0], -tangents[0].z);
  x.normalize();
  return path.map((at, i) => {
    if (i > 0) {
      const axis = new THREE.Vector3().crossVectors(tangents[i - 1], tangents[i]);
      if (axis.lengthSq() > 1e-12) x.applyAxisAngle(axis.normalize(), Math.acos(THREE.MathUtils.clamp(tangents[i - 1].dot(tangents[i]), -1, 1)));
      x.addScaledVector(tangents[i], -x.dot(tangents[i])).normalize();
    }
    const r = radii[i];
    const [w, up, down] = typeof r === 'number' ? [r, r, r] : [r[0], r[1], r[2] ?? r[1]];
    return { at: at.clone(), w, up, down, x: x.clone(), bone: bones?.[i] };
  });
}

// A faceted egg `radii` across its own axes (x, y, z of `basis`), its poles
// along z: `rings` rings between them, `sides` round. `paint(latitude,
// angle)` colours it, latitude -1 at the back pole to 1 at the front.
export function ellipsoid(
  sculpt: Sculpt,
  centre: THREE.Vector3,
  radii: THREE.Vector3,
  basis: THREE.Matrix4,
  rings: number,
  sides: number,
  painter: (latitude: number, angle: number) => Tone,
  turn = 0,
): Lofted {
  const x = new THREE.Vector3();
  const y = new THREE.Vector3();
  const z = new THREE.Vector3();
  basis.extractBasis(x, y, z);
  const list: Ring[] = [];
  const lat: number[] = [];
  for (let i = 1; i <= rings; i++) {
    const phi = -Math.PI / 2 + (Math.PI * i) / (rings + 1);
    const c = Math.cos(phi);
    list.push({ at: centre.clone().addScaledVector(z, Math.sin(phi) * radii.z), w: radii.x * c, up: radii.y * c, x: x.clone() });
    lat.push(Math.sin(phi));
  }
  const tip = radii.z * (1 - Math.sin(Math.PI / 2 - Math.PI / (rings + 1)));
  return loft(sculpt, list, {
    sides,
    turn,
    side: x,
    start: tip,
    end: tip,
    paint: (r, angle) => painter(r < 0 ? -1 : r >= list.length - 1 ? 1 : (lat[r] + lat[r + 1]) / 2, angle),
  });
}

// An ear: a pointed flap from its `base` on the head to its `tip`, `width`
// across (along `across`), its hollow facing `facing`, cupped `cup` of its
// width deep. Outside `outer`, the hollow `inner`, with a rim of `outer`
// round the hollow.
export function ear(
  sculpt: Sculpt,
  base: THREE.Vector3,
  tip: THREE.Vector3,
  across: THREE.Vector3,
  facing: THREE.Vector3,
  width: number,
  thick: number,
  cup: number,
  outer: Tone,
  inner: Tone,
  rim: Tone = outer, // the front edges round the hollow (a fox's orange, round white fur, with black backs)
): void {
  const up = tip.clone().sub(base).normalize();
  // Its hollow faces `facing` as nearly as the ear's length allows, and its
  // width runs square to both, the way `across` points.
  const f = facing.clone().addScaledVector(up, -facing.dot(up)).normalize();
  const a = new THREE.Vector3().crossVectors(up, f).normalize();
  if (a.dot(across) < 0) a.negate();
  const left = base.clone().addScaledVector(a, -width / 2);
  const right = base.clone().addScaledVector(a, width / 2);
  const back = base.clone().addScaledVector(f, -thick).addScaledVector(up, thick * 0.3);
  // The hollow: a little way up the ear, pushed back into it, with a rim
  // round it on either side.
  const hollow = base.clone().lerp(tip, 0.3).addScaledVector(f, -width * cup);
  const rimL = left.clone().lerp(tip, 0.22).addScaledVector(a, width * 0.12);
  const rimR = right.clone().lerp(tip, 0.22).addScaledVector(a, -width * 0.12);
  const backward = f.clone().negate();
  const outL = a.clone().negate().add(f.clone().multiplyScalar(0.2));
  const outR = a.clone().add(f.clone().multiplyScalar(0.2));
  // The back, two faces from the rims to the back.
  sculpt.toward(right, back, tip, backward.clone().add(a), outer);
  sculpt.toward(back, left, tip, backward.clone().sub(a), outer);
  // The front: the rims round the hollow, and the hollow.
  sculpt.toward(left, rimL, tip, outL.add(f), rim);
  sculpt.toward(rimR, right, tip, outR.add(f), rim);
  sculpt.toward(rimL, hollow, tip, f, inner);
  sculpt.toward(hollow, rimR, tip, f, inner);
  sculpt.toward(left, hollow, rimL, f, inner);
  sculpt.toward(hollow, right, rimR, f, inner);
  sculpt.toward(left, right, hollow, f.clone().add(up), inner);
}

// A shingle of fur (a wolf's ruff, a boar's bristles): from its root `a`-`b`
// on the body back to its `tip`, its middle raised by `lift`, closed
// underneath in `under`.
export function spike(sculpt: Sculpt, a: THREE.Vector3, b: THREE.Vector3, tip: THREE.Vector3, lift: THREE.Vector3, top: Tone, under: Tone, w?: readonly [Weights, Weights, Weights]): void {
  const m = a.clone().add(b).multiplyScalar(0.5).add(lift);
  const inside = a.clone().add(b).multiplyScalar(0.5).addScaledVector(lift, -1).lerp(tip, 0.3);
  const wm = w ? w[0] : undefined;
  sculpt.facing(a, m, tip, inside, top, w && [w[0], wm!, w[2]]);
  sculpt.facing(m, b, tip, inside, top, w && [wm!, w[1], w[2]]);
  sculpt.facing(a, tip, b, m, under, w && [w[0], w[2], w[1]]);
}

// A round eye on a head, looking along `look` (out of the head), `radius`
// round: a low dome of `iris` round a `pupil`, with a white glint up and
// forward of its middle, as the sheets paint their eyes. `lid` rings it.
export interface EyeTones {
  lid?: Tone;
  iris: Tone;
  pupil: Tone;
  shine: Tone;
}
export function eye(
  sculpt: Sculpt,
  centre: THREE.Vector3,
  look: THREE.Vector3,
  up: THREE.Vector3,
  radius: number,
  tones: EyeTones,
  sides = 8,
  pupil = 0.55,
  bulge = 0.5,
  stretch = 1, // wider than tall: an almond eye
  slant = 0, // radians its outer corner rises
): void {
  const z = look.clone().normalize();
  const x = new THREE.Vector3().crossVectors(up, z).normalize();
  const y = new THREE.Vector3().crossVectors(z, x);
  if (slant) {
    x.applyAxisAngle(z, slant);
    y.applyAxisAngle(z, slant);
  }
  const ring = (r: number, out: number, spin = 0) =>
    Array.from({ length: sides }, (_, k) => {
      const angle = (TAU * (k + spin)) / sides;
      return centre
        .clone()
        .addScaledVector(x, Math.cos(angle) * r * stretch)
        .addScaledVector(y, Math.sin(angle) * r)
        .addScaledVector(z, out);
    });
  const base = ring(radius, -radius * 0.25);
  const iris = ring(radius * 0.92, radius * bulge * 0.35, 0.5);
  const inner = ring(radius * pupil, radius * bulge * 0.75);
  const apex = centre.clone().addScaledVector(z, radius * bulge);
  const inside = centre.clone().addScaledVector(z, -radius);
  for (let k = 0; k < sides; k++) {
    const k1 = (k + 1) % sides;
    const lid = tones.lid ?? tones.iris;
    sculpt.facing(base[k], base[k1], iris[k], inside, lid);
    sculpt.facing(base[k1], iris[k1], iris[k], inside, lid);
    sculpt.facing(iris[k], iris[k1], inner[k1], inside, tones.iris);
    sculpt.facing(iris[k], inner[k1], inner[k], inside, tones.iris);
    sculpt.facing(inner[k], inner[k1], apex, inside, tones.pupil);
  }
  // The glint: a small facet standing just off the dome, up and forward.
  const g = centre
    .clone()
    .addScaledVector(y, radius * 0.38)
    .addScaledVector(x, radius * 0.22)
    .addScaledVector(z, radius * bulge * 0.95 + radius * 0.02);
  const s = radius * 0.2;
  const g1 = g.clone().addScaledVector(y, s);
  const g2 = g.clone().addScaledVector(x, -s * 0.9).addScaledVector(y, -s * 0.5);
  const g3 = g.clone().addScaledVector(x, s * 0.9).addScaledVector(y, -s * 0.5);
  sculpt.toward(g1, g2, g3, z, tones.shine);
}

// A flat piece seen from both sides (a fin, a wing): its outline in order,
// fanned from its first corner, or from `from` when given, each triangle
// painted by `painter` from its middle.
export function sheet(sculpt: Sculpt, outline: readonly THREE.Vector3[], facing: THREE.Vector3, painter: (middle: THREE.Vector3, index: number) => Tone, from?: THREE.Vector3, w?: Weights): void {
  const hub = from ?? outline[0];
  const start = from ? 0 : 1;
  const end = from ? outline.length : outline.length - 1;
  const middle = new THREE.Vector3();
  for (let i = start; i < end; i++) {
    const a = outline[i];
    const b = outline[(i + 1) % outline.length];
    middle.copy(hub).add(a).add(b).divideScalar(3);
    sculpt.toward(hub, a, b, facing, painter(middle, i), w && [w, w, w]);
  }
}

// Now and then a facet a little toward another role, the same for the same
// face each time (ring r, corner k): a coat patched with darker facets, as
// the sheets' coats are. `share` of the faces are patched, by up to
// `amount` of the way.
export function patchy(role: string, toward: string, r: number, k: number, amount = 0.35, share = 0.3): Tone {
  const h = Math.sin(r * 12.9898 + k * 78.233) * 43758.5453;
  const f = h - Math.floor(h);
  return f < share ? blend(role, toward, amount * (0.5 + f / share / 2)) : tone(role);
}

// A dog's paw (a wolf's, a fox's) on a foot's joint `at`, its sole on the
// ground (y 0): a rounded block, its toes in front, `k` m to a unit of its
// shape (about a pixel of the wolf's sheet at 1), on the `side` of the body.
export function paw(s: Sculpt, at: THREE.Vector3, side: number, k: number, top: Tone, toes: Tone, pads: Tone): void {
  const rings = [
    { z: -8, y: 8, w: 8, up: 5, down: 7 },
    { z: 0, y: 7, w: 10.5, up: 7, down: 7 },
    { z: 9, y: 6, w: 11, up: 6, down: 6 },
    { z: 16, y: 4.5, w: 10, up: 4, down: 4.5 },
  ].map((r) => ({ at: new THREE.Vector3(at.x, r.y * k, at.z + r.z * k), w: r.w * k, up: r.up * k, down: r.down * k, n: 2.6 }));
  loft(s, rings, {
    sides: 8,
    turn: Math.PI / 8,
    side: new THREE.Vector3(side, 0, 0),
    mirror: side < 0,
    start: 'flat',
    end: 3 * k,
    paint: (r, angle) => {
      if (Math.abs(angle - Math.PI) < 0.8) return pads;
      if (r >= 2 && Math.min(angle, 2 * Math.PI - angle) < 0.5) return toes;
      return top;
    },
  });
}

// ---------------------------------------------------------------- bones

// A bone at `at` (in the rig's space, standing as built), under `parent`
// whose own place in the rig is `parentAt`.
export function bone(name: string, parent: THREE.Object3D, at: THREE.Vector3, parentAt: THREE.Vector3 = new THREE.Vector3()): THREE.Bone {
  const b = new THREE.Bone();
  b.name = name;
  b.position.copy(at).sub(parentAt);
  parent.add(b);
  return b;
}

// Weighs every corner of a geometry to the bones of a chain of joints, by
// where along the chain it lies: `bones[i]` sits at `joints[i]` and runs to
// the next joint (the last one on past the chain's end). 'linear' blends
// from each joint's bone to the next's all the way between them, so the
// chain bends smoothly all along (a spine, a tail); a number blends only
// that share of each bone either side of each joint, so it stays stiff
// between them (a leg).
export function weighAlong(geometry: THREE.BufferGeometry, joints: readonly THREE.Vector3[], bones: readonly number[], blend: number | 'linear'): void {
  const position = geometry.getAttribute('position');
  const count = position.count;
  const index = new Uint16Array(count * 4);
  const weight = new Float32Array(count * 4);
  const lengths = joints.slice(1).map((j, i) => j.distanceTo(joints[i]));
  const starts: number[] = [0];
  for (const l of lengths) starts.push(starts[starts.length - 1] + l);
  const p = new THREE.Vector3();
  const d = new THREE.Vector3();
  const q = new THREE.Vector3();
  for (let v = 0; v < count; v++) {
    p.fromBufferAttribute(position, v);
    // Where it lies along the chain: the nearest point on it, reaching on
    // past both ends.
    let best = Infinity;
    let s = 0;
    for (let i = 0; i < lengths.length; i++) {
      d.subVectors(joints[i + 1], joints[i]);
      const t = q.subVectors(p, joints[i]).dot(d) / Math.max(1e-12, d.lengthSq());
      const lo = i === 0 ? -Infinity : 0;
      const hi = i === lengths.length - 1 ? Infinity : 1;
      const tc = THREE.MathUtils.clamp(t, Math.max(lo, 0), Math.min(hi, 1));
      const distance = q.copy(joints[i]).addScaledVector(d, tc).distanceTo(p);
      if (distance < best - 1e-9) {
        best = distance;
        s = starts[i] + THREE.MathUtils.clamp(t, lo, hi) * lengths[i];
      }
    }
    let a = bones[0];
    let b = bones[0];
    let share = 0;
    const last = joints.length - 1;
    if (blend === 'linear') {
      if (s >= starts[last]) a = b = bones[last];
      else if (s > 0) {
        let i = 0;
        while (i < last - 1 && s >= starts[i + 1]) i++;
        a = bones[i];
        b = bones[i + 1];
        share = (s - starts[i]) / lengths[i];
      }
    } else {
      let i = 0;
      while (i < last && s >= starts[i + 1]) i++;
      a = b = bones[i];
      for (let j = 1; j <= last; j++) {
        const r = blend * Math.min(lengths[j - 1], lengths[j] ?? lengths[j - 1]);
        if (Math.abs(s - starts[j]) < r) {
          a = bones[j - 1];
          b = bones[j];
          share = (s - (starts[j] - r)) / (2 * r);
        }
      }
    }
    index[4 * v] = a;
    index[4 * v + 1] = b;
    weight[4 * v] = 1 - share;
    weight[4 * v + 1] = share;
  }
  geometry.setAttribute('skinIndex', new THREE.Uint16BufferAttribute(index, 4));
  geometry.setAttribute('skinWeight', new THREE.Float32BufferAttribute(weight, 4));
}

// A mesh that bends with the skeleton's bones: bound as it stands, at the
// rig's origin, so its corners are in the rig's space.
export function skinnedMesh(geometry: THREE.BufferGeometry, material: THREE.Material, skeleton: THREE.Skeleton): THREE.SkinnedMesh {
  const mesh = new THREE.SkinnedMesh(geometry, material);
  mesh.castShadow = true;
  mesh.receiveShadow = true;
  mesh.bind(skeleton, new THREE.Matrix4());
  // Its bones swing it well past the pose it was bound in; it is small, so it
  // is always drawn rather than culled by a stale bound.
  mesh.frustumCulled = false;
  return mesh;
}

export function rigidMesh(geometry: THREE.BufferGeometry, material: THREE.Material, shadows = true): THREE.Mesh {
  const mesh = new THREE.Mesh(geometry, material);
  mesh.castShadow = shadows;
  mesh.receiveShadow = shadows;
  return mesh;
}

// ---------------------------------------------------------------- motion

// Eases `value` toward `target` at `rate` a second, the same at any frame
// rate.
export function ease(value: number, target: number, rate: number, delta: number): number {
  return value + (target - value) * (1 - Math.exp(-rate * delta));
}

export function smooth(t: number): number {
  const u = THREE.MathUtils.clamp(t, 0, 1);
  return u * u * (3 - 2 * u);
}

// Up and back down once over 0..1, gently at both ends.
export function bump(t: number): number {
  return t <= 0 || t >= 1 ? 0 : Math.sin(Math.PI * t) ** 2;
}

// A direction in a leg's plane (y, z), by its angle from straight down,
// toward +z as it grows.
export function downAngle(y: number, z: number): number {
  return Math.atan2(z, -y);
}

// Two bones in a plane reaching from a root to a target (both (y, z)), `a`
// then `b` long, the joint between them ahead of the line to the target
// (`bend` 1, a knee) or behind it (-1, an elbow). Returns both bones' angles
// from straight down (downAngle()), and whether the target was out of reach
// (it then reaches toward it as far as it can).
export function reach(rootY: number, rootZ: number, targetY: number, targetZ: number, a: number, b: number, bend: 1 | -1): { upper: number; lower: number; short: boolean } {
  const dy = targetY - rootY;
  const dz = targetZ - rootZ;
  const d = Math.hypot(dy, dz);
  const most = (a + b) * 0.9999;
  const least = Math.abs(a - b) * 1.0001 + 1e-9;
  const span = THREE.MathUtils.clamp(d, least, most);
  const line = downAngle(dy, dz);
  const cos = (a * a + span * span - b * b) / (2 * a * span);
  const upper = line + bend * Math.acos(THREE.MathUtils.clamp(cos, -1, 1));
  // The joint, and the lower bone from it to the target (or toward it).
  const jy = rootY - a * Math.cos(upper);
  const jz = rootZ + a * Math.sin(upper);
  const ty = rootY + (dy / Math.max(d, 1e-9)) * span;
  const tz = rootZ + (dz / Math.max(d, 1e-9)) * span;
  return { upper, lower: downAngle(ty - jy, tz - jz), short: d > most };
}
