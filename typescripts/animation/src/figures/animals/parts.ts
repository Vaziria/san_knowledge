import * as THREE from 'three';
import { mergeVertices } from 'three/addons/utils/BufferGeometryUtils.js';
import { gloss, surface, type Theme } from '../../theme';
import { randomUnit, seededRandom } from '../Tree/parts';

// Helpers shared by the animals and their parts.

export interface AnimalOptions {
  theme?: Theme;
}

// How an animal's surfaces are drawn: smooth (the default), or faceted, low
// poly like folded paper (the bear, from the user's reference picture). A
// faceted part is built from far fewer rows and columns, its corners nudged
// a little at random so the faces are uneven triangles rather than a
// regular grid, and each face is flat and of one colour.
export type Look = 'smooth' | 'faceted';

const FACETS = {
  share: 0.4, // of a smooth solid's rows and columns
  rows: 4, // at least
  columns: 7, // at least, round a section
  jitter: 0.16, // how far a corner moves, as a share of its edges' mean length
};

// Materials, built once per animal from the theme. The coat is one material
// whose colours are the vertex colours (white in the theme's finish, times
// each vertex's colour), since a coat shades from back to belly and down the
// legs (a fox's black stockings, the white tip of its tail). Eyes are glossy
// on any material, with a catchlight, dark unless the animal's eyes have a
// colour of their own (a cat's green); noses, hooves and claws are the dark.
// The look travels with the materials, so every part draws itself the same
// way.
export function createMaterials(theme: Theme, look: Look = 'smooth', eye: THREE.ColorRepresentation = theme.colors.dark) {
  const coat = surface(theme, 0xffffff);
  coat.vertexColors = true;
  return {
    coat,
    eye: gloss(eye),
    shine: gloss(theme.colors.light),
    dark: surface(theme, theme.colors.dark),
    look,
  };
}

export type AnimalMaterials = ReturnType<typeof createMaterials>;

// The theme's colours an animal mixes its coat from, as colours. Beyond the
// coat's roles, the scene's colours are here too, for animals whose own
// colours are nearest one of them (a bluebird's back the sky's zenith, a
// frog's skin the grass).
export function palette(theme: Theme) {
  const c = theme.colors;
  const s = theme.scene;
  return {
    fur: new THREE.Color(c.fur),
    trim: new THREE.Color(c.trim),
    light: new THREE.Color(c.light),
    dark: new THREE.Color(c.dark),
    wood: new THREE.Color(c.wood),
    grass: new THREE.Color(s.grass),
    body: new THREE.Color(c.body),
    glow: new THREE.Color(c.glow),
    metal: new THREE.Color(c.metal),
    water: new THREE.Color(s.water),
    zenith: new THREE.Color(s.zenith),
    flower: new THREE.Color(s.flower),
    autumn: new THREE.Color(s.autumn),
    sand: new THREE.Color(s.sand),
    stone: new THREE.Color(s.stone),
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
// smooth all round; faceted, it has fewer rows and columns and flat faces.
export function solid(axis: THREE.Vector3[], section: (u: number) => Section, shade: Shade, rows: number, columns = 18, look: Look = 'smooth'): THREE.BufferGeometry {
  if (look === 'faceted') {
    rows = Math.max(FACETS.rows, Math.round(rows * FACETS.share));
    columns = Math.max(FACETS.columns, Math.round(columns * FACETS.share));
  }
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

  // Faceted, every other ring is turned half a column, so the faces between
  // rings are triangles in a zigzag rather than quads in bands.
  const faceted = look === 'faceted';
  const offset = (i: number) => (faceted && i % 2 === 0 ? 0.5 : 0);

  push(curve.getPointAt(0), 0, frame(0).clone().negate());
  for (let i = 1; i < rows; i++) {
    const u = i / rows;
    const center = curve.getPointAt(u);
    frame(u);
    const { width, top, bottom } = section(u);
    for (let j = 0; j < columns; j++) {
      const angle = ((j + offset(i)) / columns) * 2 * Math.PI - Math.PI / 2; // from the bottom round the side and over the top
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
      if (offset(i) > offset(i + 1)) {
        // This ring is turned ahead of the next: split the other way, so
        // each face's lone corner sits between the two across from it.
        indices.push(ring(i, j), ring(i, j + 1), ring(i + 1, j + 1));
        indices.push(ring(i, j), ring(i + 1, j + 1), ring(i + 1, j));
      } else {
        indices.push(ring(i, j), ring(i, j + 1), ring(i + 1, j));
        indices.push(ring(i, j + 1), ring(i + 1, j + 1), ring(i + 1, j));
      }
    }
  }
  for (let j = 0; j < columns; j++) indices.push(ring(rows - 1, j), ring(rows - 1, j + 1), last);

  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  geo.setIndex(indices);
  if (look === 'faceted') return facet(geo);
  geo.computeVertexNormals();
  return geo;
}

// Turns an indexed geometry with vertex colours into flat faces: each corner
// is nudged by up to FACETS.jitter of the mean length of its edges, then the
// faces are split apart, each takes the mean colour of its corners, and each
// gets its own normal. The nudges come from a generator seeded by the shape
// itself, so a part has the same facets on every build.
export function facet(geo: THREE.BufferGeometry): THREE.BufferGeometry {
  const position = geo.getAttribute('position');
  const index = geo.getIndex()!;
  const edges = new Float32Array(position.count);
  const count = new Uint16Array(position.count);
  const a = new THREE.Vector3();
  const b = new THREE.Vector3();
  for (let i = 0; i < index.count; i += 3) {
    for (let k = 0; k < 3; k++) {
      const p = index.getX(i + k);
      const q = index.getX(i + ((k + 1) % 3));
      const length = a.fromBufferAttribute(position, p).distanceTo(b.fromBufferAttribute(position, q));
      edges[p] += length;
      edges[q] += length;
      count[p]++;
      count[q]++;
    }
  }
  let seed = position.count;
  for (let i = 0; i < position.count; i += 5) seed = (Math.imul(seed, 31) + Math.round(position.getX(i) * 1e4 + position.getY(i) * 3e4 + position.getZ(i) * 7e4)) | 0;
  const random = seededRandom(seed);
  for (let i = 0; i < position.count; i++) {
    const nudge = randomUnit(random).multiplyScalar(FACETS.jitter * random() * (count[i] ? edges[i] / count[i] : 0));
    position.setXYZ(i, position.getX(i) + nudge.x, position.getY(i) + nudge.y, position.getZ(i) + nudge.z);
  }

  const flat = geo.toNonIndexed();
  geo.dispose();
  const color = flat.getAttribute('color');
  if (color) {
    for (let i = 0; i < color.count; i += 3) {
      for (let c = 0; c < 3; c++) {
        const mean = (color.getComponent(i, c) + color.getComponent(i + 1, c) + color.getComponent(i + 2, c)) / 3;
        for (let k = 0; k < 3; k++) color.setComponent(i + k, c, mean);
      }
    }
  }
  flat.computeVertexNormals();
  return flat;
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

// The unit spheres every blob starts from, made once: indexed, so the
// normals come out smooth. The faceted one is an icosphere of 80 faces,
// whose triangles are already uneven in size.
const spheres: Partial<Record<Look, THREE.BufferGeometry>> = {};

function unitSphere(look: Look): THREE.BufferGeometry {
  let sphere = spheres[look];
  if (!sphere) {
    const faces = look === 'faceted' ? new THREE.IcosahedronGeometry(1, 1) : new THREE.SphereGeometry(1, 24, 16);
    faces.deleteAttribute('normal');
    faces.deleteAttribute('uv');
    sphere = spheres[look] = mergeVertices(faces);
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
  look: Look = 'smooth',
): THREE.BufferGeometry {
  const geo = unitSphere(look).clone();
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
  if (look === 'faceted') return facet(geo);
  geo.computeVertexNormals();
  return geo;
}

// A part lofted by hand, low poly like folded paper (the wolf's body, legs
// and tail, from the user's model): a tube through a ring of corners at each
// section (LOFT_SIDES, or `sides`), its ends closed flat or to a point. A
// section is its middle, on the part's y-z plane, and its half-width (along
// x) and half-height; each ring stands square to the path through the
// middles (along the line from the section before to the one after), its
// first corner on top. So the part is the same on both sides of x = 0, and a
// left and right pair can share it.
export type LoftSection = readonly [y: number, z: number, width: number, height: number];
export type LoftEnd = 'flat' | readonly [y: number, z: number]; // closed flat, or to that point
export interface Loft {
  sections: readonly LoftSection[];
  start?: LoftEnd; // open when left out
  end?: LoftEnd;
  sides?: number; // corners round each ring (LOFT_SIDES when left out; the user's bird has 7, its antlers 5)
}

export const LOFT_SIDES = 6;

// A loft's surface: its rings' corners (ring by ring, then the points its
// ends close to) and its triangles, each wound anticlockwise seen from
// outside. A face's band is the pair of rings it lies between (band i between
// ring i and i + 1); an end takes the band next to it. Its hub is the middle
// the user's models colour it from (their colorFor(band, centre, ring)): a
// band's face, the middle of its two rings; an end's, the middle of the ring
// it closes.
export interface Lofted {
  centers: THREE.Vector3[]; // each section's middle
  points: THREE.Vector3[];
  ring: number[]; // each point's ring (an end's point: the ring it closes)
  faces: (readonly [number, number, number])[];
  band: number[];
  hub: THREE.Vector3[];
}

const LOFT_X = new THREE.Vector3(1, 0, 0);

// A loft's numbers times `scale` (from a model's units to meters), its other
// fields kept.
export function scaleLoft<T extends Loft>(loft: T, scale: number): T {
  const end = (e: LoftEnd | undefined): LoftEnd | undefined => (e === undefined || e === 'flat' ? e : [e[0] * scale, e[1] * scale]);
  return {
    ...loft,
    sections: loft.sections.map(([y, z, width, height]): LoftSection => [y * scale, z * scale, width * scale, height * scale]),
    start: end(loft.start),
    end: end(loft.end),
  };
}

// The surface of a loft, exactly as the user's model builds it: each face is
// turned to face away from the path, a band's from the middle of its two
// rings and an end's from the ring next to it.
export function lofted(loft: Loft): Lofted {
  const { sections } = loft;
  const sides = loft.sides ?? LOFT_SIDES;
  const centers = sections.map(([y, z]) => new THREE.Vector3(0, y, z));
  const points: THREE.Vector3[] = [];
  const ring: number[] = [];
  const tangent = new THREE.Vector3();
  const up = new THREE.Vector3();
  sections.forEach(([, , width, height], i) => {
    tangent.subVectors(centers[Math.min(i + 1, centers.length - 1)], centers[Math.max(i - 1, 0)]).normalize();
    up.crossVectors(tangent, LOFT_X).normalize();
    for (let k = 0; k < sides; k++) {
      const angle = Math.PI / 2 + (2 * Math.PI * k) / sides;
      points.push(centers[i].clone().addScaledVector(LOFT_X, Math.cos(angle) * width).addScaledVector(up, Math.sin(angle) * height));
      ring.push(i);
    }
  });
  const corner = (i: number, k: number) => i * sides + (k % sides);

  const faces: (readonly [number, number, number])[] = [];
  const band: number[] = [];
  const hub: THREE.Vector3[] = [];
  const n = new THREE.Vector3();
  const e = new THREE.Vector3();
  const add = (a: number, b: number, c: number, inside: THREE.Vector3, of: number, from: THREE.Vector3) => {
    const [p, q, r] = [points[a], points[b], points[c]];
    n.subVectors(q, p).cross(e.subVectors(r, p));
    e.copy(p).add(q).add(r).divideScalar(3).sub(inside);
    faces.push(n.dot(e) < 0 ? [a, c, b] : [a, b, c]);
    band.push(of);
    hub.push(from);
  };
  for (let i = 0; i < sections.length - 1; i++) {
    const middle = new THREE.Vector3().addVectors(centers[i], centers[i + 1]).multiplyScalar(0.5);
    for (let k = 0; k < sides; k++) {
      const [a, b, c, d] = [corner(i, k), corner(i, k + 1), corner(i + 1, k), corner(i + 1, k + 1)];
      add(a, b, d, middle, i, middle);
      add(a, d, c, middle, i, middle);
    }
  }
  const close = (end: LoftEnd | undefined, i: number, next: number, of: number) => {
    if (!end) return;
    points.push(end === 'flat' ? centers[i].clone() : new THREE.Vector3(0, end[0], end[1]));
    ring.push(i);
    for (let k = 0; k < sides; k++) add(corner(i, k), corner(i, k + 1), points.length - 1, centers[next], of, centers[i]);
  };
  const last = sections.length - 1;
  close(loft.start, 0, 1, 0);
  close(loft.end, last, last - 1, last - 1);
  return { centers, points, ring, faces, band, hub };
}

// How the user's models colour a lofted part (their colorFor): by the band a
// face is in, where its centre is and its hub (see Lofted), in the loft's own
// units and axes. Returns what `paint` gives for each face, in order.
export type Paint<T> = (band: number, centre: THREE.Vector3, hub: THREE.Vector3) => T;

export function paintLoft<T>(surface: Lofted, paint: Paint<T>): T[] {
  const centre = new THREE.Vector3();
  return surface.faces.map((face, f) => {
    centre.set(0, 0, 0);
    for (const i of face) centre.add(surface.points[i]);
    return paint(surface.band[f], centre.divideScalar(3), surface.hub[f]);
  });
}

// A loft's surface as flat faces (polygons()), each coloured by `color`,
// given which face it is and its normal.
export function loftGeometry(surface: Lofted, color: (face: number, normal: THREE.Vector3) => THREE.Color): THREE.BufferGeometry {
  return polygons(
    surface.faces.map((face) => face.map((i) => surface.points[i])),
    (normal, face) => color(face, normal),
  );
}

// Flat faces from polygons, for a surface modelled by hand face by face (the
// wolf's head, and its lofted parts): each polygon lists its corners
// anticlockwise seen from outside, so its front faces out. It is cut into
// triangles fanning from its first corner, so it must be convex, and drawn
// flat: one normal for all of it (Newell's, so a face whose corners are not
// quite in a plane still shades as one) and, with `color`, one colour, given
// that normal and which face it is.
export function polygons(faces: THREE.Vector3[][], color?: (normal: THREE.Vector3, face: number) => THREE.Color): THREE.BufferGeometry {
  const positions: number[] = [];
  const normals: number[] = [];
  const colors: number[] = [];
  const n = new THREE.Vector3();
  for (const [f, face] of faces.entries()) {
    n.set(0, 0, 0);
    face.forEach((p, i) => {
      const q = face[(i + 1) % face.length];
      n.x += (p.y - q.y) * (p.z + q.z);
      n.y += (p.z - q.z) * (p.x + q.x);
      n.z += (p.x - q.x) * (p.y + q.y);
    });
    n.normalize();
    const c = color?.(n, f);
    for (let i = 1; i < face.length - 1; i++) {
      for (const p of [face[0], face[i], face[i + 1]]) {
        positions.push(p.x, p.y, p.z);
        normals.push(n.x, n.y, n.z);
        if (c) colors.push(c.r, c.g, c.b);
      }
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3));
  if (color) geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  return geo;
}
