import * as THREE from 'three';
import { mergeVertices } from 'three/addons/utils/BufferGeometryUtils.js';
import { gloss, surface, type Theme } from '../../theme';

// Helpers shared by the animals and their parts.

export interface AnimalOptions {
  theme?: Theme;
}

// Materials, built once per animal from the theme. The coat is one material
// whose colours are the vertex colours (white in the theme's finish, times
// each vertex's colour), since a coat shades from back to belly and down the
// legs (a fox's black stockings, the white tip of its tail). Eyes are glossy
// on any material, with a catchlight; noses, hooves and claws are the dark.
export function createMaterials(theme: Theme) {
  const coat = surface(theme, 0xffffff);
  coat.vertexColors = true;
  return {
    coat,
    eye: gloss(theme.colors.dark),
    shine: gloss(theme.colors.light),
    dark: surface(theme, theme.colors.dark),
  };
}

export type AnimalMaterials = ReturnType<typeof createMaterials>;

// The theme's colours an animal mixes its coat from, as colours.
export function palette(theme: Theme) {
  const c = theme.colors;
  return {
    fur: new THREE.Color(c.fur),
    trim: new THREE.Color(c.trim),
    light: new THREE.Color(c.light),
    dark: new THREE.Color(c.dark),
    wood: new THREE.Color(c.wood),
    grass: new THREE.Color(theme.scene.grass),
  };
}

export type Palette = ReturnType<typeof palette>;

// a mixed toward b by t.
export function mix(a: THREE.Color, b: THREE.Color, t: number): THREE.Color {
  return a.clone().lerp(b, t);
}

// -1 for the animal's left (-x), 1 for its right (+x).
export type Side = -1 | 1;

export function mesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

// The colour of a coat at a point of a part: `u` runs 0..1 along the part,
// `out` is the way the surface faces there, in the part's own axes (so
// out.y < 0 is its underside).
export type Shade = (u: number, out: THREE.Vector3) => THREE.Color;

// A plain colour.
export function plain(color: THREE.Color): Shade {
  return () => color;
}

// The coat on top and the belly colour underneath, blended over `soft` of
// the way round, below `line` (the y of the facing direction where the belly
// begins; lower keeps it further under).
export function twoTone(coat: THREE.Color, belly: THREE.Color, line = -0.3, soft = 0.35): Shade {
  const c = new THREE.Color();
  return (_u, out) => c.copy(coat).lerp(belly, THREE.MathUtils.smoothstep(line - out.y, 0, soft));
}

// A section across a solid: its half-width, and how far its top and bottom
// are above and below the axis, in meters.
export interface Section {
  width: number;
  top: number;
  bottom: number;
}

const UP = new THREE.Vector3(0, 1, 0);
const AHEAD = new THREE.Vector3(0, 0, 1);

// A smooth solid along a curve through `axis` (a torso, a neck, a leg, a
// tail), its section at each point along it (u from 0 to 1) given by
// `section`, and closed at both ends to a point on the axis. Sections are
// ellipses, wider than tall or taller than wide, squarer on top or below
// (top and bottom differ): their width lies level and their height as near
// to up as the curve allows (for a curve running straight up or down,
// toward +z). One vertex ring per row and no seam, so the normals come out
// smooth all round.
export function solid(axis: THREE.Vector3[], section: (u: number) => Section, shade: Shade, rows: number, columns = 18): THREE.BufferGeometry {
  const curve = new THREE.CatmullRomCurve3(axis, false, 'centripetal');
  const positions: number[] = [];
  const colors: number[] = [];
  const out = new THREE.Vector3();
  const n = new THREE.Vector3();
  const s = new THREE.Vector3();
  const push = (p: THREE.Vector3, u: number, facing: THREE.Vector3) => {
    positions.push(p.x, p.y, p.z);
    const color = shade(u, facing);
    colors.push(color.r, color.g, color.b);
  };

  // The frame at a point: t along the curve, n toward up, s level to the side.
  const frame = (u: number) => {
    const t = curve.getTangentAt(u);
    const reference = Math.abs(t.dot(UP)) > 0.95 ? AHEAD : UP;
    n.copy(reference).addScaledVector(t, -reference.dot(t)).normalize();
    s.crossVectors(n, t);
    return t;
  };

  push(curve.getPointAt(0), 0, frame(0).clone().negate());
  for (let i = 1; i < rows; i++) {
    const u = i / rows;
    const center = curve.getPointAt(u);
    frame(u);
    const { width, top, bottom } = section(u);
    for (let j = 0; j < columns; j++) {
      const angle = (j / columns) * 2 * Math.PI - Math.PI / 2; // from the bottom round the side and over the top
      const up = Math.sin(angle);
      const p = center.clone().addScaledVector(s, width * Math.cos(angle)).addScaledVector(n, (up > 0 ? top : bottom) * up);
      out.copy(s).multiplyScalar(Math.cos(angle)).addScaledVector(n, up);
      push(p, u, out);
    }
  }
  push(curve.getPointAt(1), 1, curve.getTangentAt(1));

  // Wound so the faces point out: (around) x (along) is outward.
  const ring = (i: number, j: number) => 1 + (i - 1) * columns + (j % columns);
  const last = positions.length / 3 - 1;
  const indices: number[] = [];
  for (let j = 0; j < columns; j++) indices.push(0, ring(1, j + 1), ring(1, j));
  for (let i = 1; i < rows - 1; i++) {
    for (let j = 0; j < columns; j++) {
      indices.push(ring(i, j), ring(i, j + 1), ring(i + 1, j));
      indices.push(ring(i, j + 1), ring(i + 1, j + 1), ring(i + 1, j));
    }
  }
  for (let j = 0; j < columns; j++) indices.push(ring(rows - 1, j), ring(rows - 1, j + 1), last);

  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  geo.setIndex(indices);
  geo.computeVertexNormals();
  return geo;
}

// A rounded end-to-end profile for a solid: full in the middle, closing
// quickly at both ends (bluntness 8 is a capsule, 2 an egg).
export function capsule(bluntness = 8): (u: number) => number {
  return (u) => Math.pow(Math.max(0, 1 - Math.pow(Math.abs(2 * u - 1), bluntness)), 1 / bluntness);
}

// How much a blob narrows toward its top or bottom (its own y): 0.3 makes
// that end 30% narrower, 0.95 almost a point (a pointed ear).
export interface Taper {
  top?: number;
  bottom?: number;
}

// The unit sphere every blob starts from, made once: indexed, so the
// normals come out smooth.
let sphere: THREE.BufferGeometry | null = null;

function unitSphere(): THREE.BufferGeometry {
  if (!sphere) {
    const faces = new THREE.SphereGeometry(1, 24, 16);
    faces.deleteAttribute('normal');
    faces.deleteAttribute('uv');
    sphere = mergeVertices(faces);
    faces.dispose();
  }
  return sphere;
}

// An ellipsoid with its radii along its own axes, narrowed toward its top or
// bottom, turned and put at `center`, coloured by `shade` (u from 0 at its
// bottom to 1 at its top, `out` the way it faces before turning).
export function blob(
  center: THREE.Vector3,
  radii: THREE.Vector3,
  shade: Shade,
  taper: Taper = {},
  turn?: THREE.Euler,
): THREE.BufferGeometry {
  const geo = unitSphere().clone();
  const position = geo.getAttribute('position');
  const colors: number[] = [];
  const p = new THREE.Vector3();
  const out = new THREE.Vector3();
  for (let i = 0; i < position.count; i++) {
    p.fromBufferAttribute(position, i);
    const f = 1 - (taper.top ?? 0) * Math.max(0, p.y) - (taper.bottom ?? 0) * Math.max(0, -p.y);
    out.set(p.x / radii.x, p.y / radii.y, p.z / radii.z).normalize();
    const color = shade(0.5 + 0.5 * p.y, out);
    colors.push(color.r, color.g, color.b);
    position.setXYZ(i, p.x * f * radii.x, p.y * radii.y, p.z * f * radii.z);
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  if (turn) geo.applyMatrix4(new THREE.Matrix4().makeRotationFromEuler(turn));
  geo.translate(center.x, center.y, center.z);
  geo.computeVertexNormals();
  return geo;
}
