import * as THREE from 'three';
import { gloss } from '../../theme';
import type { Dress, ModelFace, Point } from './Head';
import { lofted, mesh, paintLoft, polygons, type Loft, type LoftSection, type Paint } from './parts';
import { sheets } from './Wing';

// What the user's "Low-poly animal park" shares (2026-09-26): one three.js
// page that built nine animals from the same few helpers, the crocodile, ox,
// antelope, tiger, coyote, raccoon, monkey, pig and squirrel, since split
// into their nine specs' "# Shape Reference". Each animal there is only its
// numbers and a few colour rules (its <animal>Model.ts here); this file is
// the page's helpers, doing what the page's do:
// - stretchedHead(), the page's buildHead(): one head for every animal but
//   the monkey, the wolf's head from its first model (the points of
//   wolfModel.ts) stretched by a few numbers: the whole head wider and
//   taller, the snout alone longer, wider and dropped, the ears bigger, moved
//   or with their tips elsewhere, or none. Each face belongs to a region
//   ("muzzle", "nose", "earIn" …), which the animal colours, a region it
//   leaves out taking the colour of the one it falls back to;
// - eyeDisc(), a flat round eye, a ring round a dark middle;
// - pair(), a part and its mirror image (addPair);
// - ridge(), a row of little upright spikes along the top of a loft (the
//   crocodile's scutes).
// Faces are given in the units of the part they go on (a head's, the
// body's), with a colour named as the animal's page names it, and placed and
// drawn by solid(), sheet() and decal().
//
// Beyond the page, as for the wolf: the page draws every face from both
// sides, so the heads' ears faced in and their backs were open round the
// neck. Here the ears' faces are turned to face out and their bases are
// closed, and the back of the head is closed to the nape, where the neck
// ends, which the head turns about.

// The page's repeatable "random" number from a seed (the squirrel's tail's
// darker tufts), and its stripes: true where a wave through `v` is above
// `width` (the tiger's).
export function rand(n: number): number {
  const v = Math.sin(n * 91.7 + 3.1) * 43758.55;
  return v - Math.floor(v);
}
export function stripe(v: number, width = 0.5): boolean {
  return Math.sin(v) > width;
}

// A head's regions, which the animal colours (its `colors`).
export type Region =
  | 'top'
  | 'side'
  | 'brow'
  | 'cheek'
  | 'mask'
  | 'eye'
  | 'bridge'
  | 'cheekLow'
  | 'muzzle'
  | 'snoutTop'
  | 'nose'
  | 'chin'
  | 'throat'
  | 'earIn'
  | 'earOut'
  | 'earBack';

// The wolf's head from its first model, its right half and middle.
const BASE_V = {
  C_back: [0, 1.45, -1.2],
  C_forehead: [0, 1.7, 0.05],
  C_brow: [0, 0.55, 0.9],
  C_stop: [0, 0.35, 1.1],
  C_snoutTip: [0, 0.12, 2.4],
  C_noseBot: [0, -0.42, 2.5],
  C_chin: [0, -0.72, 1.2],
  C_throat: [0, -0.7, 0.25],
  topPlate: [0.45, 1.62, 0.1],
  temple: [0.78, 1.2, 0.3],
  brow: [0.4, 0.55, 0.85],
  browOuter: [0.82, 0.78, 0.6],
  eyeIn: [0.38, 0.32, 0.93],
  eyeOut: [0.74, 0.38, 0.68],
  eyeLow: [0.52, 0.1, 0.84],
  eyeDeep: [0.56, 0.3, 0.55],
  cheek: [1.22, 0.6, 0.0],
  cheekLow: [1.0, -0.2, 0.3],
  jaw: [0.6, -0.45, 0.9],
  back: [0.9, 1.0, -1.0],
  backLow: [1.1, -0.1, -0.7],
  snoutBase: [0.3, 0.3, 1.15],
  snoutTip: [0.25, 0.05, 2.35],
  noseBot: [0.22, -0.4, 2.45],
  snoutBot: [0.3, -0.6, 1.3],
  earFront: [0.5, 1.55, 0.15],
  earOut: [1.25, 0.95, 0.05],
  earBack: [0.75, 1.4, -0.55],
  earTip: [1.1, 2.55, -0.2],
  earInner: [0.9, 1.45, -0.15],
} as const satisfies Record<string, Point>;

type Base = keyof typeof BASE_V;
export type Corner = Base | 'C_nape';

// Its faces, on the right side (mirrored to the left), each in its region.
const BASE_F: readonly (readonly [Base, Base, Base, Region])[] = [
  ['C_back', 'C_forehead', 'topPlate', 'top'],
  ['C_back', 'topPlate', 'back', 'top'],
  ['C_forehead', 'brow', 'topPlate', 'top'],
  ['C_forehead', 'C_brow', 'brow', 'top'],
  ['topPlate', 'brow', 'temple', 'top'],
  ['temple', 'brow', 'browOuter', 'top'],
  ['topPlate', 'temple', 'back', 'top'],
  ['back', 'temple', 'cheek', 'side'],
  ['temple', 'browOuter', 'cheek', 'side'],
  ['brow', 'eyeOut', 'browOuter', 'brow'],
  ['brow', 'eyeIn', 'eyeOut', 'brow'],
  ['C_brow', 'eyeIn', 'brow', 'top'],
  ['browOuter', 'eyeOut', 'cheek', 'cheek'],
  ['eyeOut', 'cheekLow', 'cheek', 'cheek'],
  ['eyeOut', 'eyeLow', 'cheekLow', 'mask'],
  ['eyeIn', 'snoutBase', 'eyeLow', 'mask'],
  ['eyeIn', 'eyeLow', 'eyeDeep', 'eye'],
  ['eyeLow', 'eyeOut', 'eyeDeep', 'eye'],
  ['eyeOut', 'eyeIn', 'eyeDeep', 'eye'],
  ['C_brow', 'snoutBase', 'eyeIn', 'bridge'],
  ['C_brow', 'C_stop', 'snoutBase', 'bridge'],
  ['eyeLow', 'jaw', 'cheekLow', 'cheekLow'],
  ['eyeLow', 'snoutBase', 'jaw', 'muzzle'],
  ['snoutBase', 'snoutBot', 'jaw', 'muzzle'],
  ['C_stop', 'snoutTip', 'snoutBase', 'snoutTop'],
  ['C_stop', 'C_snoutTip', 'snoutTip', 'snoutTop'],
  ['snoutBase', 'noseBot', 'snoutBot', 'muzzle'],
  ['snoutBase', 'snoutTip', 'noseBot', 'muzzle'],
  ['C_snoutTip', 'noseBot', 'snoutTip', 'nose'],
  ['C_snoutTip', 'C_noseBot', 'noseBot', 'nose'],
  ['C_noseBot', 'snoutBot', 'noseBot', 'chin'],
  ['C_noseBot', 'C_chin', 'snoutBot', 'chin'],
  ['C_chin', 'jaw', 'snoutBot', 'chin'],
  ['C_chin', 'C_throat', 'jaw', 'throat'],
  ['C_throat', 'backLow', 'jaw', 'throat'],
  ['jaw', 'backLow', 'cheekLow', 'cheekLow'],
  ['cheek', 'cheekLow', 'backLow', 'side'],
  ['cheek', 'backLow', 'back', 'side'],
  ['earFront', 'earTip', 'earInner', 'earIn'],
  ['earInner', 'earTip', 'earOut', 'earIn'],
  ['earFront', 'earInner', 'earOut', 'earOut'],
  ['earOut', 'earTip', 'earBack', 'earBack'],
  ['earBack', 'earTip', 'earFront', 'earBack'],
];

// The region a region takes its colour from when the animal doesn't colour it.
const FALLBACK: Partial<Record<Region, Region>> = {
  side: 'top',
  brow: 'top',
  cheek: 'side',
  mask: 'cheek',
  bridge: 'top',
  cheekLow: 'cheek',
  muzzle: 'cheekLow',
  snoutTop: 'bridge',
  chin: 'muzzle',
  throat: 'chin',
  earIn: 'top',
  earOut: 'top',
  earBack: 'top',
  nose: 'top',
  eye: 'top',
};

const EARS: readonly Base[] = ['earFront', 'earOut', 'earBack', 'earTip', 'earInner'];

// How an animal stretches the head, as its page gives it: the whole head `w`
// times as wide and `h` as tall; the snout (what is ahead of 1 unit)
// `snout` times as long, `snoutW` as wide at its tip and dropped `snoutDrop`
// at the nose; the ears `scale` times as big about their base, moved by
// `offset`, their tips put at `tip`, or not there (`show: false`); and its
// colours, by region, each a colour the animal names. `faceColor` colours a
// face itself, given its region and its middle (the tiger's stripes), or
// leaves it to its region (null).
export interface Stretch<Color extends string = string> {
  w?: number;
  h?: number;
  snout?: number;
  snoutW?: number;
  snoutDrop?: number;
  ear?: { scale?: number; show?: boolean; offset?: Point; tip?: Point };
  colors: { top: Color } & { [R in Region]?: Color };
  faceColor?: (region: Region, centre: Point) => Color | null;
}

// Where the head of a model turns, in the head's units: the middle of the
// body's last section, where the neck ends, with the head at `at`, `scale`
// times the body's units.
export function napeOf(body: Loft, at: Point, scale: number): Point {
  const [y, z] = body.sections[body.sections.length - 1];
  return [0, (y - at[1]) / scale, (z - at[2]) / scale];
}

// The head, stretched as the animal's page stretches it, as a HeadModel's
// points and faces (Head.ts), closed round the nape; its mouth (where a held
// figure goes) is under the muzzle, halfway from the nose's bottom to the
// chin.
export function stretchedHead(o: Stretch, nape: Point) {
  const w = o.w ?? 1;
  const h = o.h ?? 1;
  const sl = o.snout ?? 1;
  const sw = o.snoutW ?? 1;
  const drop = o.snoutDrop ?? 0;
  // The page's stretch: the whole head by w and h, the snout by snout,
  // snoutW and snoutDrop.
  const morph = ([x, y, z]: Point): [number, number, number] => {
    const t = Math.min(1, Math.max(0, (z - 0.8) / 0.6));
    const nx = x * w * (1 + (sw - 1) * t);
    let ny = y * h;
    let nz = z;
    if (z > 1.0) {
      ny += (drop * (z - 1.0)) / 1.5;
      nz = 1.0 + (z - 1.0) * sl;
    }
    return [nx, ny, nz];
  };
  const V = {} as Record<Base, [number, number, number]>;
  for (const k of Object.keys(BASE_V) as Base[]) V[k] = morph(BASE_V[k]);

  // The ears: scaled about their base, moved, and their tips put elsewhere;
  // the dent in the front of each goes where the page puts it.
  const ear = { scale: 1, show: true, offset: [0, 0, 0] as Point, ...o.ear };
  const base = (['earFront', 'earOut', 'earBack'] as const).map((k) => V[k]);
  const c = [0, 1, 2].map((j) => (base[0][j] + base[1][j] + base[2][j]) / 3);
  for (const k of ['earFront', 'earOut', 'earBack', 'earTip'] as const) V[k] = V[k].map((v, j) => c[j] + (v - c[j]) * ear.scale + ear.offset[j]) as [number, number, number];
  if (ear.tip) V.earTip = [...ear.tip];
  const bc = [0, 1, 2].map((j) => (V.earFront[j] + V.earOut[j] + V.earBack[j]) / 3);
  V.earInner = bc.map((v, j) => v + (V.earTip[j] - v) * 0.35 + (j === 2 ? -0.1 : 0)) as [number, number, number];

  const colorOf = (region: Region, centre: Point): string => {
    const custom = o.faceColor?.(region, centre);
    if (custom) return custom;
    let r = region;
    while (!o.colors[r] && FALLBACK[r]) r = FALLBACK[r]!;
    return o.colors[r] ?? o.colors.top;
  };

  const side: ModelFace<Corner>[] = [];
  for (const [a, b, cc, region] of BASE_F) {
    const isEar = region.startsWith('ear');
    if (isEar && !ear.show) continue;
    const centre = [0, 1, 2].map((j) => (V[a][j] + V[b][j] + V[cc][j]) / 3) as unknown as Point;
    const color = colorOf(region, centre);
    // An ear's faces the other way round from the page's, so they face out.
    side.push(isEar ? [color, cc, b, a] : [color, a, b, cc]);
  }
  const top = o.colors.top;
  if (ear.show) side.push([top, 'earFront', 'earBack', 'earOut']); // the ear's base, closed
  // The back of the head, closed from its rim to the nape.
  side.push([top, 'C_nape', 'C_back', 'back'], [top, 'C_nape', 'back', 'backLow'], [top, 'C_nape', 'backLow', 'C_throat']);

  const points: Partial<Record<Corner, Point>> = { C_nape: nape };
  for (const k of Object.keys(V) as Base[]) if (ear.show || !EARS.includes(k)) points[k] = V[k];
  const mouth: Point = [0, (V.C_noseBot[1] + V.C_chin[1]) / 2, (V.C_noseBot[2] + V.C_chin[2]) / 2];
  return { points: points as Record<string, Point>, side, middle: [] as ModelFace[], mouth, pitch: 0 };
}

// A face of something a page adds (horns, teeth, eyes): its corners, in the
// units of the part it goes on, and the colour its page names.
export interface Face {
  corners: Point[];
  color: string;
}

// A loft's faces, coloured by `paint` (a page's colorFor).
export function loftFaces(loft: Loft, paint: Paint<string>): Face[] {
  const surface = lofted(loft);
  const colors = paintLoft(surface, paint);
  return surface.faces.map((face, f) => ({ corners: face.map((i) => surface.points[i].toArray()), color: colors[f] }));
}

// A part and its mirror image, as the page's addPair() places them: the
// right one turned by `turn` (x, y and z, in that order) and put at `at`,
// the left one the same at -x, mirrored, its faces the other way round so
// they still face out.
export function pair(faces: readonly Face[], at: Point, turn: Point = [0, 0, 0]): Face[] {
  const matrix = new THREE.Matrix4().makeRotationFromEuler(new THREE.Euler(...turn)).setPosition(...at);
  const right = faces.map(({ corners, color }) => ({ corners: corners.map((p) => new THREE.Vector3(...p).applyMatrix4(matrix).toArray() as Point), color }));
  const left = right.map(({ corners, color }) => ({ corners: corners.map(([x, y, z]): Point => [-x, y, z]).reverse(), color }));
  return [...right, ...left];
}

// A flat round eye, as the page's eyeDisc() makes it: a disc facing +x, a
// ring of six corners in the `iris` colour round a middle in the `pupil`
// colour, raised a little (the ring's inside 0.02 out, the middle 0.03).
export function eyeDisc(radius: number, iris: string, pupil: string): Face[] {
  const N = 6;
  const ring = (r: number, x: number): Point[] =>
    [...Array(N).keys()].map((k) => [x, Math.cos((2 * Math.PI * k) / N) * r, Math.sin((2 * Math.PI * k) / N) * r]);
  const outer = ring(radius, 0);
  const inner = ring(radius * 0.5, 0.02);
  const faces: Face[] = [];
  for (let k = 0; k < N; k++) {
    const k2 = (k + 1) % N;
    faces.push({ corners: [outer[k], outer[k2], inner[k2]], color: iris });
    faces.push({ corners: [outer[k], inner[k2], inner[k]], color: iris });
    faces.push({ corners: [[0.03, 0, 0], inner[k], inner[k2]], color: pupil });
  }
  return faces;
}

// A row of `count` little upright spikes along the top of a loft's sections
// (the crocodile's scutes), `height` tall, in `rows` across its top (each a
// share of four times the section's half-width out from the middle).
export function ridge(sections: readonly LoftSection[], count: number, height: number, color: string, rows: readonly number[] = [-0.12, 0.12]): Face[] {
  const at = (u: number) => {
    const f = u * (sections.length - 1);
    const i = Math.min(Math.floor(f), sections.length - 2);
    const k = f - i;
    return [0, 1, 2, 3].map((j) => sections[i][j] + (sections[i + 1][j] - sections[i][j]) * k);
  };
  const faces: Face[] = [];
  for (let j = 0; j < count; j++) {
    const [y, z, rx, ry] = at((j + 0.5) / count);
    for (const off of rows) {
      const x = off * rx * 4;
      faces.push({ corners: [[x, y + ry * 0.85, z + 0.13], [x, y + ry * 0.85, z - 0.13], [x, y + ry + height, z - 0.04]], color });
    }
  }
  return faces;
}

// Faces placed on the part (`point` takes them to meters), in the colours
// the animal names.
function geometry({ colors, point }: Dress, faces: readonly Face[]): THREE.BufferGeometry {
  const colorOf = (name: string) => {
    const color = colors[name];
    if (!color) throw new Error(`a face names the colour "${name}", which the animal hasn't`);
    return color;
  };
  return polygons(
    faces.map((f) => f.corners.map((p) => point(p))),
    (_n, i) => colorOf(faces[i].color),
  );
}

// A closed surface (a horn, a skull, a bump), its faces facing out: drawn on
// one side, in the coat.
export function solid(dress: Dress, faces: readonly Face[], name: string): THREE.Object3D {
  const part = mesh(geometry(dress, faces), dress.m.coat);
  part.name = name;
  return part;
}

// Thin flat pieces seen from both sides (teeth, spikes, a brow, a monkey's
// ears): a thin sheet (rules.md, Building shapes 9), its front casting the
// shadow from both sides.
export function sheet(dress: Dress, faces: readonly Face[], name: string): THREE.Object3D {
  const group = new THREE.Group();
  group.name = name;
  group.add(...sheets(dress.m)(geometry(dress, faces)));
  return group;
}

// Flat pieces lying on a surface (eyes, nostrils), drawn from both sides as
// the page draws them and casting no shadow, which would only fall on the
// surface they lie on. Faces in a `glossy` colour (an eye's middle) are
// glossy, as eyes are.
export function decal(dress: Dress, faces: readonly Face[], name: string, glossy?: string): THREE.Object3D {
  const both = [...faces, ...faces.map(({ corners, color }) => ({ corners: [...corners].reverse(), color }))];
  const group = new THREE.Group();
  group.name = name;
  const matte = both.filter((f) => f.color !== glossy);
  const shiny = both.filter((f) => f.color === glossy);
  const parts = [mesh(geometry(dress, matte), dress.m.coat)];
  if (glossy && shiny.length) parts.push(mesh(polygons(shiny.map((f) => f.corners.map((p) => dress.point(p)))), gloss(dress.colors[glossy])));
  for (const part of parts) {
    part.castShadow = false;
    group.add(part);
  }
  return group;
}
