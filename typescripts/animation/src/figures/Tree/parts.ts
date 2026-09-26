import * as THREE from 'three';
import { mergeVertices } from 'three/addons/utils/BufferGeometryUtils.js';
import { BARK_TILE } from '../../textures/bark';
import { bark, surface, type Theme } from '../../theme';
import type { Tip } from './skeleton';

// Helpers shared by the trees and their parts.

export interface TreeOptions {
  theme?: Theme;
  // The seed the tree grows from: the same seed always grows the same tree,
  // another seed another tree of the same kind. Each kind has its own default.
  seed?: number;
  // Leaves turned for autumn: in the theme's autumn colour instead of its
  // grass colour, some clumps browner, paler or still green (see
  // createMaterials).
  autumn?: boolean;
}

// Materials for every part, built once per tree from the theme: bark in the
// theme's wood colour (see bark() in theme.ts) and leaves or needles in its
// grass colour (its foliage colour) times `shade`, since conifers are darker
// than broadleaves. The foliage is shaded per vertex (undersides and the
// inside of the crown darker), so its material takes those shades from the
// vertex colours.
//
// In autumn the leaves are the theme's autumn colour instead, and each clump
// takes one of AUTUMN_TINTS at random (`leafTints`, which the crown puts in
// its vertex colours): the autumn colour itself, turned part of the way
// toward the wood colour (browner) or the grass colour (still green), or
// lightened (faded), so a crown is mottled like a real one in autumn. Each
// tint comes up `weight` times as often. Faded leaves are the autumn colour
// lightened, not mixed toward the theme's light colour, which in a dark theme
// turned them nearly white.
const AUTUMN_TINTS: { toward?: 'wood' | 'grass'; share?: number; light?: number; weight: number }[] = [
  { weight: 3 },
  { toward: 'wood', share: 0.4, weight: 2 },
  { light: 1.25, weight: 1 },
  { toward: 'grass', share: 0.45, weight: 1 },
];

export function createMaterials(theme: Theme, shade: number, autumn = false) {
  const leaves = new THREE.Color(autumn ? theme.scene.autumn : theme.scene.grass).multiplyScalar(shade);
  const foliage = surface(theme, autumn ? 0xffffff : leaves);
  foliage.vertexColors = true;
  const others = { wood: theme.colors.wood, grass: theme.scene.grass };
  const leafTints = autumn
    ? AUTUMN_TINTS.flatMap((t) => {
        const tint = leaves.clone().multiplyScalar(t.light ?? 1);
        if (t.toward) tint.lerp(new THREE.Color(others[t.toward]).multiplyScalar(shade), t.share ?? 0);
        return Array<THREE.Color>(t.weight).fill(tint);
      })
    : undefined;
  return { bark: bark(theme), foliage, leafTints };
}

export type TreeMaterials = ReturnType<typeof createMaterials>;

export function mesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

// A repeatable random number generator (mulberry32): the same seed always
// gives the same numbers, in 0..1.
export function seededRandom(seed: number): () => number {
  let a = seed;
  return () => {
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// A random direction, every way equally likely.
export function randomUnit(random: () => number): THREE.Vector3 {
  const y = 2 * random() - 1;
  const angle = 2 * Math.PI * random();
  const r = Math.sqrt(1 - y * y);
  return new THREE.Vector3(r * Math.cos(angle), y, r * Math.sin(angle));
}

// A number between min and max.
export function between(random: () => number, min: number, max: number): number {
  return min + (max - min) * random();
}

// Roots: the foot of a trunk swells by up to `amount` times its radius, in
// `lobes` buttresses round it, fading out over about `height` meters up.
export interface Flare {
  amount: number;
  height: number;
  lobes: number;
}

const ROW_LENGTH = 0.25; // m of limb between rings of vertices
const BUTTRESS = 0.45; // how much of the flare the buttresses carry, the rest swells evenly

// A limb: a round tube along a smooth curve through `points`, its radius
// running through `radii` (one for each point), closed to a point at its end.
// Texture coordinates are meters along the limb and whole bark tiles round
// it, so the bark wraps without a seam (textures/bark.ts). `flare` swells a
// trunk's foot into roots.
export function limbGeometry(points: THREE.Vector3[], radii: number[], flare?: Flare): THREE.BufferGeometry {
  const curve = new THREE.CatmullRomCurve3(points, false, 'centripetal');
  const length = curve.getLength();
  const segments = points.length - 1;
  const rows = Math.max(2 * segments, Math.ceil(length / ROW_LENGTH), flare ? 24 : 0);
  const columns = Math.round(THREE.MathUtils.clamp(6 + 24 * radii[0], 6, 18));
  const tiles = Math.max(1, Math.round((2 * Math.PI * radii[0]) / BARK_TILE));
  const frames = curve.computeFrenetFrames(rows, false);

  // The radius at u (0..1 along the limb), between the radii at its points.
  const radius = (u: number) => {
    const at = u * segments;
    const i = Math.min(segments - 1, Math.floor(at));
    return THREE.MathUtils.lerp(radii[i], radii[i + 1], at - i);
  };

  const positions: number[] = [];
  const normals: number[] = [];
  const uvs: number[] = [];
  const out = new THREE.Vector3();
  for (let i = 0; i <= rows; i++) {
    const u = i / rows;
    const center = curve.getPointAt(u);
    const r = radius(u);
    for (let j = 0; j <= columns; j++) {
      const angle = (j / columns) * 2 * Math.PI;
      out.copy(frames.normals[i]).multiplyScalar(Math.cos(angle)).addScaledVector(frames.binormals[i], Math.sin(angle));
      let swell = 1;
      if (flare) {
        const lobe = 1 - BUTTRESS + BUTTRESS * Math.cos(flare.lobes * angle);
        swell += flare.amount * Math.exp(-Math.max(0, center.y) / flare.height) * lobe;
      }
      positions.push(center.x + r * swell * out.x, center.y + r * swell * out.y, center.z + r * swell * out.z);
      normals.push(out.x, out.y, out.z);
      uvs.push(u * length, (j / columns) * tiles * BARK_TILE);
    }
  }
  // The end, closed to a point.
  const end = curve.getPointAt(1);
  const tangent = curve.getTangentAt(1);
  for (let j = 0; j <= columns; j++) {
    positions.push(end.x, end.y, end.z);
    normals.push(tangent.x, tangent.y, tangent.z);
    uvs.push(length, (j / columns) * tiles * BARK_TILE);
  }

  // Wound so the faces point out: (around) x (along) is outward for the
  // curve's right-handed tangent, normal, binormal frames.
  const at = (i: number, j: number) => i * (columns + 1) + j;
  const indices: number[] = [];
  for (let i = 0; i <= rows; i++) {
    for (let j = 0; j < columns; j++) {
      indices.push(at(i, j), at(i, j + 1), at(i + 1, j));
      indices.push(at(i, j + 1), at(i + 1, j + 1), at(i + 1, j));
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3));
  geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
  geo.setIndex(indices);
  return geo;
}

// A clump of leaves or needles: a lumpy blob, the unit of every crown.
export interface Clump {
  center: THREE.Vector3;
  size: THREE.Vector3; // its radii along its own x, y and z
  turn?: THREE.Quaternion; // unturned, its own y is up
}

const CLUMP_DETAIL = 2; // subdivisions of the icosahedron: 320 faces (1 gives 80, enough for a small clump)
const LUMPS = 5; // waves that dent and swell the sphere
const LUMP_DEPTH = 0.12; // how far they move its surface, as a share of its radius
const UNDERSIDE = 0.72; // an underside's shade, times the foliage colour
const CLUMP_TINT = [0.9, 1.04] as const; // each clump is a little lighter or darker

// The unit sphere every clump starts from, made once for each detail:
// indexed, so the normals come out smooth.
const spheres = new Map<number, THREE.BufferGeometry>();

function unitSphere(detail: number): THREE.BufferGeometry {
  let sphere = spheres.get(detail);
  if (!sphere) {
    const faces = new THREE.IcosahedronGeometry(1, detail);
    faces.deleteAttribute('normal');
    faces.deleteAttribute('uv');
    sphere = mergeVertices(faces);
    faces.dispose();
    spheres.set(detail, sphere);
  }
  return sphere;
}

// One clump: a sphere dented and swelled by a few waves running across it in
// random directions, stretched to its size and turned. Its vertex colours
// shade the underside darker than the top. `detail` is the sphere's
// subdivisions, fewer for a small clump (the grasses' spikelets and florets).
export function clumpGeometry(clump: Clump, random: () => number, detail = CLUMP_DETAIL): THREE.BufferGeometry {
  const geo = unitSphere(detail).clone();

  const waves = Array.from({ length: LUMPS }, () => ({
    direction: randomUnit(random),
    frequency: between(random, 2, 5),
    phase: 2 * Math.PI * random(),
  }));
  const position = geo.getAttribute('position');
  const p = new THREE.Vector3();
  for (let i = 0; i < position.count; i++) {
    p.fromBufferAttribute(position, i);
    let r = 1;
    for (const w of waves) r += (LUMP_DEPTH / Math.sqrt(LUMPS)) * Math.sin(w.frequency * p.dot(w.direction) + w.phase);
    position.setXYZ(i, p.x * r, p.y * r, p.z * r);
  }
  geo.computeVertexNormals();
  geo.applyMatrix4(new THREE.Matrix4().compose(clump.center, clump.turn ?? new THREE.Quaternion(), clump.size));

  const tint = between(random, ...CLUMP_TINT);
  const normal = geo.getAttribute('normal');
  const colors: number[] = [];
  for (let i = 0; i < normal.count; i++) {
    const shade = tint * THREE.MathUtils.lerp(UNDERSIDE, 1, 0.5 + 0.5 * normal.getY(i));
    colors.push(shade, shade, shade);
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  return geo;
}

const TIP_REACH = 0.2; // a tip's clump sits this far past the tip, so it hides the twig's end

// A clump of leaves at each twig tip, its radius between the two given, and
// `squash` times as tall as it is wide: flat for a pine's tufts.
export function tipClumps(tips: Tip[], radius: readonly [number, number], squash: number, random: () => number): Clump[] {
  return tips.map((tip) => {
    const r = between(random, ...radius);
    return {
      center: tip.position.clone().addScaledVector(tip.direction, TIP_REACH),
      size: new THREE.Vector3(r, r * squash, r),
      turn: new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), 2 * Math.PI * random()),
    };
  });
}
