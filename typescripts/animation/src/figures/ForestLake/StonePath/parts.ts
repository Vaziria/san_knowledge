import * as THREE from 'three';
import { addBlock, addFlower, addLeaf, addRock, addTube, addTuft, between, color, FALLEN_LEAVES, matte, mesh, noise2, palette, PALETTE, seededRandom, shade, Shape, vary, type Palette, type Season } from '../parts';

// What the stone path's parts share (StonePath.md): its variations, the
// sheet's stone colours, the stones' shapes, the ground's fill, the plants
// in its joints and along its sides, where it runs (a Course), and how each
// variation lays its stones. Every shape is flat-shaded triangles with a
// colour at each corner, as the forest lake's assets are (../parts.ts): the
// sheet's albedo is the corners' colours, its normal map the facets, and
// its ambient occlusion darkness baked into the colours at the joints and
// round each stone's foot.

// The sheet's five, in its Variation row's order.
export const VARIATIONS = ['regular', 'mossy', 'dirty', 'ruined', 'stairs'] as const;
export type Variation = (typeof VARIATIONS)[number];

// sRGB, picked from the sheet: grey stone with a cool tint, paler on top,
// darker at its edges and foot; the brown dirt of the dirty and ruined
// paths.
export const STONE = {
  top: 0xb3b2b8,
  face: 0x94939b,
  edge: 0x6e6d76,
  foot: 0x47464f,
  dirt: 0x8a6a48,
  dirtDark: 0x5f4630,
  dirtLight: 0xa6845c,
} as const;

// Where a path runs: the point `along` m from its start and `across` m to
// its left, on its ground, and the way it runs there (radians round y, as a
// figure's heading: 0 toward +z).
export interface Course {
  readonly length: number;
  at(along: number, across: number): Spot;
  ground(x: number, z: number): number;
}

export interface Spot {
  x: number;
  y: number;
  z: number;
  heading: number;
}

// Straight along +z from the origin, on `ground` (flat at y 0 by default).
export function straight(length: number, ground: (x: number, z: number) => number = () => 0): Course {
  return {
    length,
    ground,
    at: (along, across) => ({ x: across, y: ground(across, along), z: along, heading: 0 }),
  };
}

// Along a line of points ([x, z], a meter or so apart), from `from` m to
// `to` m along it, on `ground`.
export function alongLine(points: readonly (readonly number[])[], from: number, to: number, ground: (x: number, z: number) => number): Course {
  const lengths = [0];
  for (let i = 1; i < points.length; i++) lengths.push(lengths[i - 1] + Math.hypot(points[i][0] - points[i - 1][0], points[i][1] - points[i - 1][1]));
  const total = lengths[lengths.length - 1];
  const point = (d: number) => {
    const t = THREE.MathUtils.clamp(d, 0, total);
    let i = 1;
    while (i < lengths.length - 1 && lengths[i] < t) i++;
    const [a, b] = [points[i - 1], points[i]];
    const f = (t - lengths[i - 1]) / Math.max(1e-6, lengths[i] - lengths[i - 1]);
    return { x: a[0] + (b[0] - a[0]) * f, z: a[1] + (b[1] - a[1]) * f };
  };
  return {
    length: to - from,
    ground,
    at: (along, across) => {
      const d = from + along;
      // The way it runs, over a meter, so it turns smoothly round a bend.
      const p = point(d);
      const back = point(d - 0.5);
      const ahead = point(d + 0.5);
      const heading = Math.atan2(ahead.x - back.x, ahead.z - back.z);
      const x = p.x + Math.cos(heading) * across;
      const z = p.z - Math.sin(heading) * across;
      return { x, y: ground(x, z), z, heading };
    },
  };
}

// ---------------------------------------------------------------- stones

// A flagstone, as the sheet's components and wireframe draw one, 1 m each
// way before it is scaled to the place it is laid in: a slab whose top has
// `corners` corners (a square's, each corner a little off, some clipped
// into two), a little domed and faceted (a ring of facets round a middle),
// its edge bevelled and its sides tapering out to its foot, 0.2 m thick.
// 7 triangles a corner. Pale on top, darker down its bevel and sides,
// darkest at its foot. With `moss`, a patch of the season's moss on its
// top; with `broken`, a piece of one: cut off straight across.
export function flagstone(random: () => number, corners: number, P: Palette, { moss = false, broken = false } = {}): THREE.BufferGeometry {
  const shape = new Shape();
  let outline = squarish(random, corners);
  if (broken) outline = cut(outline, random);
  // In order round it (x, z), so the faces built below face out.
  outline.sort((p, q) => Math.atan2(p.y, p.x) - Math.atan2(q.y, q.x));
  const middle = outline.reduce((sum, p) => sum.add(p), new THREE.Vector2()).divideScalar(outline.length);
  const ring = (f: number, y: number, jitter = 0) =>
    outline.map((p) => new THREE.Vector3(middle.x + (p.x - middle.x) * f + between(random, -jitter, jitter), y + between(random, -jitter, jitter) * 0.3, middle.y + (p.y - middle.y) * f + between(random, -jitter, jitter)));
  const TOP = 0.04;
  const center = new THREE.Vector3(middle.x, TOP + 0.022, middle.y);
  const inner = ring(0.5, TOP + 0.015, 0.03);
  const top = ring(0.86, TOP, 0.01);
  const edge = ring(1, TOP - 0.035);
  const foot = ring(1.06, TOP - 0.2);

  const light = color(STONE.top);
  const face = color(STONE.face);
  const dark = color(STONE.edge);
  const low = color(STONE.foot);
  const green = color(P.moss);
  // Moss on a patch of its top, off its middle.
  const patch = new THREE.Vector2(middle.x + between(random, -0.2, 0.2), middle.y + between(random, -0.2, 0.2));
  const mossAt = (p: THREE.Vector3) => (moss ? Math.hypot(p.x - patch.x, p.z - patch.y) < between(random, 0.16, 0.3) : false);
  const topColor = (p: THREE.Vector3, base: THREE.Color) => (mossAt(p) ? vary(green, random, 1.6) : vary(base, random, 1.4));
  const n = outline.length;
  for (let i = 0; i < n; i++) {
    const j = (i + 1) % n;
    const facet = vary(light, random, 1.2).multiplyScalar(between(random, 0.86, 1.06));
    shape.triangle(center, inner[j], inner[i], topColor(center, facet), topColor(inner[j], facet), topColor(inner[i], facet));
    shape.quad(inner[i], inner[j], top[j], top[i], topColor(inner[i], light), topColor(inner[j], light), topColor(top[j], shade(light, 0.97)), topColor(top[i], shade(light, 0.97)));
    // The bevel, then the sides down to its foot, darker the lower.
    const bevel = shade(face, between(random, 0.88, 1.02));
    shape.quad(top[i], top[j], edge[j], edge[i], vary(bevel, random), vary(bevel, random), vary(dark, random), vary(dark, random));
    shape.quad(edge[i], edge[j], foot[j], foot[i], vary(dark, random), vary(dark, random), low, low);
  }
  return shape.geometry();
}

// A square 1 m across, round its middle, each corner well off it, and
// `corners` - 4 more corners: a corner clipped into two, or a side bowed
// out or in at a corner of its own. A flagstone's top, filling most of the
// place it is laid in, as irregular as the sheet's.
function squarish(random: () => number, corners: number): THREE.Vector2[] {
  const square = [
    [0.5, 0.5],
    [-0.5, 0.5],
    [-0.5, -0.5],
    [0.5, -0.5],
  ].map(([x, z]) => new THREE.Vector2(x + between(random, -0.07, 0.03) * Math.sign(x), z + between(random, -0.07, 0.03) * Math.sign(z)));
  const extra = new Map<number, 'clip' | 'bow'>();
  while (extra.size < Math.min(4, corners - 4)) extra.set(Math.floor(random() * 4), random() < 0.55 ? 'clip' : 'bow');
  const outline: THREE.Vector2[] = [];
  square.forEach((corner, i) => {
    const after = square[(i + 1) % 4];
    const how = extra.get(i);
    if (how === 'clip') {
      const before = square[(i + 3) % 4];
      outline.push(corner.clone().lerp(before, between(random, 0.06, 0.24)), corner.clone().lerp(after, between(random, 0.06, 0.24)));
    } else {
      outline.push(corner);
    }
    if (how === 'bow') {
      // A corner along its side to the next, pushed out or in.
      const middle = corner.clone().lerp(after, between(random, 0.3, 0.7));
      outline.push(middle.multiplyScalar(1 + between(random, -0.08, 0.1)));
    }
  });
  return outline;
}

// A piece of a stone broken off: the outline cut by a straight line across
// it, a little off its middle, the bigger side kept.
function cut(outline: THREE.Vector2[], random: () => number): THREE.Vector2[] {
  const a = random() * Math.PI;
  const normal = new THREE.Vector2(Math.cos(a), Math.sin(a));
  const offset = between(random, 0.05, 0.16);
  const side = (p: THREE.Vector2) => p.dot(normal) - offset;
  const kept: THREE.Vector2[] = [];
  for (let i = 0; i < outline.length; i++) {
    const p = outline[i];
    const q = outline[(i + 1) % outline.length];
    if (side(p) <= 0) kept.push(p);
    if (side(p) <= 0 !== side(q) <= 0) kept.push(p.clone().lerp(q, side(p) / (side(p) - side(q))));
  }
  return kept.length >= 3 ? kept : outline;
}

// A chip of stone, rubble or a pebble, 1 m across before it is scaled: a
// faceted lump in the sheet's greys.
export function chip(random: () => number, P: Palette): THREE.BufferGeometry {
  const shape = new Shape();
  addRock(shape, new THREE.Vector3(), new THREE.Vector3(1, between(random, 0.45, 0.65), between(random, 0.7, 0.95)), random() * Math.PI, random, 2, 11, { ...P, rockLight: STONE.top, rock: STONE.face, rockDark: STONE.edge });
  return shape.geometry();
}

// A stair's block, 1 m each way before it is scaled to its step: oblong,
// dressed, chamfered, pale on its tread and darker down its riser, its top
// at y 0.
export function block(random: () => number): THREE.BufferGeometry {
  const shape = new Shape();
  const light = color(STONE.top);
  const face = color(STONE.face);
  const dark = color(STONE.edge);
  addBlock(shape, new THREE.Vector3(0, -0.5, 0), new THREE.Vector3(1, 1, 1), (normal) => (normal.y > 0.5 ? vary(light, random, 1.3) : normal.y < -0.5 ? dark : vary(face.clone().lerp(dark, 0.3), random, 1.3)), random, { chamfer: 0.08, rough: 0.025 });
  return shape.geometry();
}

// ---------------------------------------------------------------- plants

// A tuft of grass for the joints (small) or the verge (taller); the
// season's grass (gold in autumn, pale and dry in winter).
export function tuft(random: () => number, P: Palette, height: number, blades: number): THREE.BufferGeometry {
  const shape = new Shape();
  addTuft(shape, new THREE.Vector3(), blades, height, random, 0.01, 0.25, P);
  return shape.geometry();
}

// A small flower on its stem, for the joints and the verge.
export function flower(random: () => number, P: Palette, petal: number): THREE.BufferGeometry {
  const shape = new Shape();
  const head = new THREE.Vector3(between(random, -0.02, 0.02), between(random, 0.1, 0.17), between(random, -0.02, 0.02));
  addTube(shape, [new THREE.Vector3(0, -0.01, 0), head], [0.005, 0.004], { sides: 3, random, paint: () => color(P.grass) });
  addFlower(shape, head, new THREE.Vector3(head.x * 4, 1, head.z * 4), 5, between(random, 0.035, 0.05), vary(color(petal), random, 0.6), color(P.flowerHeart), random, 0.2);
  addTuft(shape, new THREE.Vector3(), 3, 0.09, random, 0, 0.2, P);
  return shape.geometry();
}

// A low cushion of moss with a sprig or two: the mossy path's joints.
export function mossClump(random: () => number, P: Palette): THREE.BufferGeometry {
  const shape = new Shape();
  const green = color(P.moss);
  const count = 9;
  const middle = new THREE.Vector3(0, 0.035, 0);
  const rim = Array.from({ length: count }, (_, i) => {
    const a = -(2 * Math.PI * i) / count;
    const r = 0.14 * between(random, 0.65, 1.1);
    return new THREE.Vector3(Math.cos(a) * r, 0.005, Math.sin(a) * r);
  });
  for (let i = 0; i < count; i++) shape.triangle(middle, rim[i], rim[(i + 1) % count], shade(green, 1.1), vary(green, random, 1.5), vary(green, random, 1.5));
  addTuft(shape, new THREE.Vector3(0.03, 0, 0), 4, 0.07, random, 0, 0.3, P);
  return shape.geometry();
}

// A few fallen leaves, for autumn: on the stones and between them.
export function leaves(random: () => number): THREE.BufferGeometry {
  const shape = new Shape();
  for (let k = 0; k < 4; k++) {
    const a = random() * Math.PI * 2;
    const d = 0.12 * Math.sqrt(random());
    const tone = vary(color(FALLEN_LEAVES[Math.floor(random() * FALLEN_LEAVES.length)]), random, 1.2);
    addLeaf(shape, new THREE.Vector3(Math.cos(a) * d, 0, Math.sin(a) * d), between(random, 0.08, 0.12), random() * Math.PI * 2, tone, random, 0.004 + k * 0.002);
  }
  return shape.geometry();
}

// ---------------------------------------------------------------- laying

// A stone laid: which of its shapes (stoneGeometries), where its origin is,
// the way it is turned about its `up`, its up (the ground's, tipped a
// little), how wide (across the path) and deep (along it) it is, and how
// thick (a flagstone: a share of the 0.2 m it is built; a stair's block:
// m from its tread down, its origin its tread's middle).
export interface Laid {
  template: number;
  x: number;
  y: number;
  z: number;
  turn: number;
  up: THREE.Vector3;
  width: number;
  depth: number;
  thick: number;
}

// Its stones' shapes, one set per season: FLAGS flagstones (their shapes as
// the components panel's), MOSSY of them with moss on, BROKEN pieces of
// broken ones, CHIPS chips and pebbles, and BLOCKS stairs' blocks.
export const FLAGS = 6;
export const MOSSY = 2;
export const BROKEN = 3;
export const CHIPS = 3;
export const BLOCKS = 2;
export const FIRST = { flag: 0, mossy: FLAGS, broken: FLAGS + MOSSY, chip: FLAGS + MOSSY + BROKEN, block: FLAGS + MOSSY + BROKEN + CHIPS } as const;

export function stoneGeometries(season: Season, seed: number): THREE.BufferGeometry[] {
  const P = palette(season);
  const random = seededRandom(seed * 131 + 7);
  const shapes: THREE.BufferGeometry[] = [];
  for (let k = 0; k < FLAGS; k++) shapes.push(flagstone(random, 5 + (k % 4), P));
  for (let k = 0; k < MOSSY; k++) shapes.push(flagstone(random, 6 + k, P, { moss: true }));
  for (let k = 0; k < BROKEN; k++) shapes.push(flagstone(random, 5 + k, P, { broken: true }));
  for (let k = 0; k < CHIPS; k++) shapes.push(chip(random, P));
  for (let k = 0; k < BLOCKS; k++) shapes.push(block(random));
  // In winter a little snow on their tops: some of the facets turned to
  // the sky, the stones' grey showing between, as the sheet's winter path.
  if (season === 'winter') for (const g of shapes) dust(g, random, 0.35);
  return shapes;
}

// Snow on `share` of a geometry's faces turned up (a flagstone's facets),
// mostly white, a little of the stone showing through.
function dust(geometry: THREE.BufferGeometry, random: () => number, share: number): void {
  const snow = color(PALETTE.snow);
  const blue = color(PALETTE.snowShade);
  const normal = geometry.getAttribute('normal');
  const tint = geometry.getAttribute('color') as THREE.BufferAttribute;
  const c = new THREE.Color();
  for (let i = 0; i + 2 < normal.count; i += 3) {
    if (normal.getY(i) < 0.8 || random() > share) continue;
    const white = blue.clone().lerp(snow, between(random, 0.5, 1));
    const cover = between(random, 0.7, 0.95);
    for (let k = 0; k < 3; k++) {
      c.fromBufferAttribute(tint, i + k).lerp(white, cover);
      tint.setXYZ(i + k, c.r, c.g, c.b);
    }
  }
  tint.needsUpdate = true;
}

// How each variation lays its stones, in rows across the path, as the
// sheet's strips are laid: how deep a row is and how wide a stone (m), the
// joint between them (m), how much of its place a stone fills (a dirty
// path's are spaced out in the dirt), how often one is missing, how far one
// tips (radians), how often one is mossy or broken, and how many chips lie
// in the joints to each stone (rubble, pebbles).
const LAYING: Record<Exclude<Variation, 'stairs'>, { rows: [number, number]; widths: [number, number]; joint: [number, number]; fill: [number, number]; missing: number; tip: number; mossy: number; broken: number; chips: number }> = {
  regular: { rows: [0.42, 0.66], widths: [0.42, 0.78], joint: [0.025, 0.05], fill: [1, 1], missing: 0, tip: 0.02, mossy: 0.08, broken: 0, chips: 0.08 },
  mossy: { rows: [0.36, 0.58], widths: [0.34, 0.66], joint: [0.08, 0.13], fill: [0.96, 1], missing: 0.03, tip: 0.03, mossy: 0.35, broken: 0, chips: 0.12 },
  dirty: { rows: [0.32, 0.5], widths: [0.32, 0.52], joint: [0.04, 0.08], fill: [0.66, 0.86], missing: 0.05, tip: 0.04, mossy: 0, broken: 0.05, chips: 0.9 },
  ruined: { rows: [0.38, 0.62], widths: [0.36, 0.68], joint: [0.04, 0.09], fill: [0.9, 1], missing: 0.1, tip: 0.13, mossy: 0.1, broken: 0.35, chips: 1.2 },
};

export const TOP_MOST = 0.05; // m over the ground a flagstone's top is at most
const FLAG_TOP = 0.062; // its top's middle over its origin, before it is scaled (flagstone())
const Y = new THREE.Vector3(0, 1, 0);

// The ground's up at a point: its normal.
export function groundUp(ground: (x: number, z: number) => number, x: number, z: number, e = 0.2): THREE.Vector3 {
  const dx = (ground(x + e, z) - ground(x - e, z)) / (2 * e);
  const dz = (ground(x, z + e) - ground(x, z - e)) / (2 * e);
  return new THREE.Vector3(-dx, 1, -dz).normalize();
}

// Lays a stretch of a path: its flagstones in rows across it, each row's
// joints off the last one's, or, for stairs, its steps, two blocks to each.
// `wear` (0..1) is how broken a ruined path is at each point along it (more
// broken toward the ruins); `shape`, how wide it is along it and how far
// from each end it breaks up into single stones, then none (where it meets
// a path of another kind).
export function layStones(course: Course, width: number, variation: Variation, random: () => number, wear: (along: number) => number = () => 1, shape: Shaping = {}): Laid[] {
  const widthAt = shape.widthAt ?? (() => width);
  if (variation === 'stairs') return laySteps(course, width, random, widthAt);
  const kind = LAYING[variation];
  const ruined = variation === 'ruined';
  const laid: Laid[] = [];
  const fading = fadeOf(course.length, shape.fade);
  let along = between(random, 0, 0.04);
  while (along < course.length - 0.15) {
    const row = Math.min(between(random, kind.rows[0], kind.rows[1]), course.length - along);
    const joint = between(random, kind.joint[0], kind.joint[1]);
    const rowWidth = widthAt(along + row / 2);
    // The row's places across it, from one side to the other, the last
    // taking what is left.
    const places: number[] = [];
    let left = rowWidth;
    while (left > 0.02) {
      let w = between(random, kind.widths[0], kind.widths[1]);
      if (left - w < kind.widths[0] * 0.7) w = left;
      places.push(Math.min(w, left));
      left -= w;
    }
    let across = -rowWidth / 2;
    for (const place of places) {
      const v = across + place / 2;
      across += place;
      const u = along + row / 2;
      const worn = ruined ? 0.4 + wear(u) : 1;
      if (random() < kind.missing * worn || place < 0.12) continue;
      const fade = fading(u);
      if (fade > 0 && random() < fade) continue;
      const fill = between(random, kind.fill[0], kind.fill[1]);
      const w = (place - joint) * fill * between(random, 0.96, 1);
      // Not all of a row's stones as deep, so the rows don't show.
      const d = (row - joint) * fill * between(random, 0.86, 1);
      // Spaced out, a stone sits anywhere in its place.
      const du = (row - joint - d) * (random() - 0.5);
      const dv = (place - joint - w) * (random() - 0.5);
      const at = course.at(u + du, v + dv);
      const broken = random() < kind.broken * worn;
      const mossy = !broken && random() < kind.mossy;
      const template = broken ? FIRST.broken + Math.floor(random() * BROKEN) : mossy ? FIRST.mossy + Math.floor(random() * MOSSY) : FIRST.flag + Math.floor(random() * FLAGS);
      const thick = THREE.MathUtils.clamp(0.4 + (Math.max(w, d) - 0.3) * 0.5, 0.4, 0.6); // 8–12 cm
      // Lying on the ground's slope, tipped a little (a ruined one more);
      // its top's highest point at most TOP_MOST over the ground under it.
      const tilt = between(random, 0, kind.tip) * worn;
      const toward = random() * Math.PI * 2;
      const up = groundUp(course.ground, at.x, at.z).add(new THREE.Vector3(Math.cos(toward) * tilt, 0, Math.sin(toward) * tilt)).normalize();
      const rise = (Math.hypot(w, d) / 2) * Math.sin(tilt);
      const lift = Math.min(TOP_MOST - rise - 0.004, between(random, 0.025, 0.042) + (ruined ? between(random, 0, 0.008) * wear(u) : 0));
      // Turned with the row, either way round, a little askew.
      const turn = at.heading + (random() < 0.5 ? 0 : Math.PI) + between(random, -0.09, 0.09);
      laid.push({ template, x: at.x, y: at.y + lift - FLAG_TOP * thick, z: at.z, turn, up, width: w, depth: d, thick });
    }
    along += row;
  }
  return laid;
}

// Chips of stone lying in the joints: a dirty path's pebbles, a ruined
// one's rubble, a few in the others; kept off the stones themselves.
export function layChips(course: Course, width: number, variation: Variation, stones: readonly Laid[], random: () => number, wear: (along: number) => number = () => 1, shape: Shaping = {}): Laid[] {
  if (variation === 'stairs') return [];
  const kind = LAYING[variation];
  const feet = stones.map(footprint);
  const chips: Laid[] = [];
  const count = Math.round(kind.chips * stones.length);
  const widthAt = shape.widthAt ?? (() => width);
  for (let k = 0; k < count * 4 && chips.length < count; k++) {
    const u = random() * course.length;
    const v = between(random, -0.5, 0.5) * widthAt(u) * (variation === 'ruined' ? 1.15 : 1);
    if (variation === 'ruined' && random() > 0.35 + 0.65 * wear(u)) continue;
    const at = course.at(u, v);
    const size = variation === 'ruined' ? between(random, 0.06, 0.2) : between(random, 0.04, 0.1);
    if (onStone(feet, at.x, at.z, size * 0.3)) continue;
    // Low: a chip stands no higher than a flagstone (TOP_MOST).
    chips.push({ template: FIRST.chip + Math.floor(random() * CHIPS), x: at.x, y: at.y - size * 0.12, z: at.z, turn: random() * Math.PI * 2, up: groundUp(course.ground, at.x, at.z), width: size, depth: size, thick: Math.min(size, 0.075) });
  }
  return chips;
}

// A step: where its middle is, the way up the path it faces, how wide and
// deep it is, and its tread's height.
export interface Step {
  x: number;
  z: number;
  heading: number;
  width: number;
  depth: number;
  top: number;
}

// The steps of a stair laid along a course, each about STEP_RISE high and
// as deep as the ground's slope takes to rise that (STEP_DEPTH at most and
// least), its tread level at the ground's height at its higher end, along
// the path's middle (a bank rising at its side doesn't lift it).
export const STEP_RISE = 0.15;
const STEP_DEPTH = [0.34, 0.8] as const;
export function stepsOf(course: Course, width: number, widthAt: (along: number) => number = () => width): Step[] {
  const steps: Step[] = [];
  let along = 0;
  while (along < course.length - 0.2) {
    const a = course.at(along, 0);
    const b = course.at(Math.min(course.length, along + 0.5), 0);
    const slope = Math.abs(b.y - a.y) / 0.5;
    const depth = Math.min(course.length - along, THREE.MathUtils.clamp(STEP_RISE / Math.max(slope, 1e-3), STEP_DEPTH[0], STEP_DEPTH[1]));
    const middle = course.at(along + depth / 2, 0);
    let top = -Infinity;
    for (const u of [along, along + depth / 2, along + depth]) top = Math.max(top, course.at(u, 0).y);
    steps.push({ x: middle.x, z: middle.z, heading: middle.heading, width: widthAt(along + depth / 2), depth, top: top + 0.02 });
    along += depth;
  }
  return steps;
}

// Each step's two blocks, side by side across it, reaching down into the
// ground under their lowest corner.
function laySteps(course: Course, width: number, random: () => number, widthAt: (along: number) => number): Laid[] {
  const laid: Laid[] = [];
  for (const step of stepsOf(course, width, widthAt)) {
    const c = Math.cos(step.heading);
    const s = Math.sin(step.heading);
    for (const side of [-1, 1]) {
      const w = step.width / 2 - between(random, 0.03, 0.06);
      const across = side * (step.width / 4 + between(random, -0.02, 0.02));
      const x = step.x + c * across;
      const z = step.z - s * across;
      const depth = step.depth - between(random, 0.02, 0.05);
      let low = Infinity;
      for (const [a, b] of [
        [-1, -1],
        [-1, 1],
        [1, -1],
        [1, 1],
      ]) {
        const u = (a * depth) / 2;
        const v = (b * w) / 2;
        low = Math.min(low, course.ground(x + s * u + c * v, z + c * u - s * v));
      }
      laid.push({ template: FIRST.block + Math.floor(random() * BLOCKS), x, y: step.top, z, turn: step.heading + between(random, -0.03, 0.03), up: Y.clone(), width: w, depth, thick: step.top - low + 0.1 });
    }
  }
  return laid;
}

// The matrix a stone is drawn with: its shape's frame, placed.
export function matrixOf(laid: Laid): THREE.Matrix4 {
  const q = new THREE.Quaternion().setFromUnitVectors(Y, laid.up).multiply(new THREE.Quaternion().setFromAxisAngle(Y, laid.turn));
  return new THREE.Matrix4().compose(new THREE.Vector3(laid.x, laid.y, laid.z), q, new THREE.Vector3(laid.width, laid.thick, laid.depth));
}

// A stone's footprint on the ground (the rectangle it is laid in, turned
// with it), for keeping plants and chips off it, and its top's height.
export interface Footprint {
  x: number;
  z: number;
  turn: number;
  width: number;
  depth: number;
  top: number;
}

export function footprint(laid: Laid): Footprint {
  const block = laid.template >= FIRST.block;
  return { x: laid.x, z: laid.z, turn: laid.turn, width: laid.width, depth: laid.depth, top: block ? laid.y : laid.y + FLAG_TOP * laid.thick };
}

// A stretch of a path: where it runs, how wide it is, its variation, and
// for a ruined one how broken it is along it (0..1).
export interface Stretch extends Shaping {
  course: Course;
  width: number;
  variation: Variation;
  wear?: (along: number) => number;
}

// How a stretch is shaped beyond its width: how wide it is along it (its
// width by default), and over how many meters from its start and its end
// its stones break up into single stones, then none, where it meets a path
// of another kind (none by default).
export interface Shaping {
  widthAt?: (along: number) => number;
  fade?: readonly [number, number];
}

// How much of a stretch's stones are gone at a point along it (0..1),
// breaking up toward an end that fades.
export function fadeOf(length: number, fade: Shaping['fade']): (along: number) => number {
  if (!fade) return () => 0;
  const [start, end] = fade;
  return (along) => Math.max(start > 0 ? 1 - along / start : 0, end > 0 ? 1 - (length - along) / end : 0, 0) ** 1.4;
}

// Stairs' flights, each its steps in order up its course: the tread under a
// point, or a ramp over them for walking on a grid (StonePath.stepAt,
// rampAt; the forest lake's log steps).
export class Flights {
  readonly steps: readonly Step[];
  private readonly flights: readonly (readonly Step[])[];
  private readonly box = new THREE.Box2();

  constructor(flights: readonly (readonly Step[])[]) {
    this.flights = flights;
    this.steps = flights.flat();
    for (const step of this.steps) this.box.expandByPoint(new THREE.Vector2(step.x, step.z));
    this.box.expandByScalar(1);
  }

  // The tread's height under a point; NaN off the steps.
  stepAt(x: number, z: number): number {
    const on = this.onStep(x, z);
    return on ? on.flight[on.index].top : NaN;
  }

  // A ramp over the steps instead: the height of a slope through the
  // middles of their risers, within half a riser of each tread, easing down
  // to `ground` over RAMP_EDGE m at their sides and over the first and
  // last steps, so it has no edge. NaN off them.
  rampAt(x: number, z: number, ground: (x: number, z: number) => number): number {
    const on = this.onStep(x, z);
    if (!on) return NaN;
    const { flight, index, along, across } = on;
    const step = flight[index];
    const before = index > 0 ? (flight[index - 1].top + step.top) / 2 : step.top;
    const after = index < flight.length - 1 ? (flight[index + 1].top + step.top) / 2 : step.top;
    const ramp = before + (after - before) * (along / step.depth + 0.5);
    let weight = THREE.MathUtils.smoothstep((step.width / 2 - Math.abs(across)) / RAMP_EDGE, 0, 1);
    if (index === 0) weight *= THREE.MathUtils.clamp(along / step.depth + 0.5, 0, 1);
    if (index === flight.length - 1) weight *= THREE.MathUtils.clamp(0.5 - along / step.depth, 0, 1);
    const below = ground(x, z);
    return below + Math.max(0, ramp - below) * weight;
  }

  // Which step a point is on, how far along it from its middle (toward the
  // next step) and across it.
  private onStep(x: number, z: number): { flight: readonly Step[]; index: number; along: number; across: number } | null {
    if (!this.steps.length || !this.box.containsPoint(new THREE.Vector2(x, z))) return null;
    for (const flight of this.flights) {
      for (let index = 0; index < flight.length; index++) {
        const step = flight[index];
        const dx = x - step.x;
        const dz = z - step.z;
        const along = dx * Math.sin(step.heading) + dz * Math.cos(step.heading);
        const across = dx * Math.cos(step.heading) - dz * Math.sin(step.heading);
        if (Math.abs(along) <= step.depth / 2 && Math.abs(across) <= step.width / 2) return { flight, index, along, across };
      }
    }
    return null;
  }
}

const RAMP_EDGE = 0.35; // m over which a ramp over the steps eases down to the ground at its sides (Flights.rampAt)

// ---------------------------------------------------------------- the ground's fill

// The colour of what fills a path's joints at a point: grass in a regular
// path's joints, moss in a mossy one's, brown dirt in a dirty or ruined
// one's (with grass through a ruined one), grass under stairs; the season's
// (gold in autumn, snow in winter). Darker in the middle of the path, where
// the stones shade it (the sheet's ambient occlusion).
export function fillColor(variation: Variation, season: Season, x: number, z: number, edge: number): THREE.Color {
  const P = palette(season);
  const n = noise2(x * 1.7, z * 1.7, 71);
  const m = noise2(x * 5.3, z * 5.3, 73);
  let c: THREE.Color;
  if (season === 'winter') c = color(P.snow).lerp(color(P.snowShade), 0.35 + 0.35 * n);
  else if (variation === 'dirty' || (variation === 'ruined' && n < 0.55)) c = color(STONE.dirt).lerp(color(m > 0.5 ? STONE.dirtLight : STONE.dirtDark), Math.abs(m - 0.5) * 1.4);
  else if (variation === 'mossy') c = color(P.moss).lerp(color(P.grassFoot), 0.3 + 0.4 * m);
  else c = color(P.grass).lerp(color(P.grassFoot), 0.35 + 0.5 * m);
  // In shade toward the middle, where the stones stand close, lighter out
  // at the edges (the sheet's ambient occlusion).
  return c.multiplyScalar(season === 'winter' ? 0.9 + 0.1 * edge : 0.6 + 0.32 * edge + 0.08 * n);
}

// A strip of ground over the course, from `from` m to `to` m across it
// (to its left), `rise(t, along)` m over its ground, coloured by
// `paint(x, z, t)`, `t` running 0 to 1 across it: the fill between the
// stones, or winter's snow banked along a side.
// `keep`: where it is laid, by how far along it and across it (0..1) a
// point is: none where it is below 0, cut along a smooth edge where it
// crosses 0 (all of it by default).
export function strip(course: Course, from: number | ((along: number) => number), to: number | ((along: number) => number), paint: (x: number, z: number, t: number) => THREE.Color, rise: (t: number, along: number) => number = () => 0.008, step = 0.25, across = 6, keep?: (along: number, t: number) => number): THREE.BufferGeometry {
  const fromAt = typeof from === 'number' ? () => from : from;
  const toAt = typeof to === 'number' ? () => to : to;
  const shape = new Shape();
  const rows: THREE.Vector3[][] = [];
  const colors: THREE.Color[][] = [];
  const kept: number[][] = [];
  const count = Math.max(1, Math.ceil(course.length / step));
  for (let i = 0; i <= count; i++) {
    const along = (i / count) * course.length;
    const row: THREE.Vector3[] = [];
    const tints: THREE.Color[] = [];
    const keeps: number[] = [];
    for (let j = 0; j <= across; j++) {
      const t = j / across;
      const at = course.at(along, fromAt(along) + (toAt(along) - fromAt(along)) * t);
      row.push(new THREE.Vector3(at.x, at.y + rise(t, along), at.z));
      tints.push(paint(at.x, at.z, t));
      keeps.push(keep ? keep(along, t) : 1);
    }
    rows.push(row);
    colors.push(tints);
    kept.push(keeps);
  }
  for (let i = 0; i < count; i++) {
    for (let j = 0; j < across; j++) {
      const corners: [number, number][] = [
        [i, j],
        [i + 1, j],
        [i + 1, j + 1],
        [i, j + 1],
      ];
      const k = corners.map(([a, b]) => kept[a][b]);
      if (k.every((v) => v < 0)) continue;
      const [p, c] = [corners.map(([a, b]) => rows[a][b]), corners.map(([a, b]) => colors[a][b])];
      if (k.every((v) => v >= 0)) shape.quad(p[0], p[1], p[2], p[3], c[0], c[1], c[2], c[3]);
      else for (const [a, b, d] of [[0, 1, 2], [0, 2, 3]]) clipped(shape, [p[a], p[b], p[d]], [c[a], c[b], c[d]], [k[a], k[b], k[d]]);
    }
  }
  const geometry = shape.geometry();
  // Face up, whichever way it bends (a tight bend can fold a face over).
  upward(geometry);
  return geometry;
}

// The part of a triangle where `keep` (at its corners, linear across it)
// is 0 or more, as triangles.
function clipped(shape: Shape, points: THREE.Vector3[], colors: THREE.Color[], keep: number[]): void {
  const p: THREE.Vector3[] = [];
  const c: THREE.Color[] = [];
  for (let n = 0; n < 3; n++) {
    const m = (n + 1) % 3;
    if (keep[n] >= 0) {
      p.push(points[n]);
      c.push(colors[n]);
    }
    if (keep[n] >= 0 !== keep[m] >= 0) {
      const f = keep[n] / (keep[n] - keep[m]);
      p.push(points[n].clone().lerp(points[m], f));
      c.push(colors[n].clone().lerp(colors[m], f));
    }
  }
  for (let n = 1; n + 1 < p.length; n++) shape.triangle(p[0], p[n], p[n + 1], c[0], c[n], c[n + 1]);
}

// Turns every triangle of a geometry that faces down to face up.
function upward(geometry: THREE.BufferGeometry): void {
  const position = geometry.getAttribute('position') as THREE.BufferAttribute;
  const normal = geometry.getAttribute('normal') as THREE.BufferAttribute;
  const color = geometry.getAttribute('color') as THREE.BufferAttribute;
  for (let i = 0; i < position.count; i += 3) {
    if (normal.getY(i) >= 0) continue;
    for (const attribute of [position, normal, color]) {
      const [x, y, z] = [attribute.getX(i + 1), attribute.getY(i + 1), attribute.getZ(i + 1)];
      attribute.setXYZ(i + 1, attribute.getX(i + 2), attribute.getY(i + 2), attribute.getZ(i + 2));
      attribute.setXYZ(i + 2, x, y, z);
    }
    for (let k = 0; k < 3; k++) normal.setXYZ(i + k, -normal.getX(i + k), -normal.getY(i + k), -normal.getZ(i + k));
  }
}

// A mesh of the path's own (the fill, a snow bank), lit, casting no shadow.
export function flat(geometry: THREE.BufferGeometry): THREE.Mesh {
  const m = mesh(geometry, matte(), false);
  m.receiveShadow = true;
  return m;
}

// Colour for the seasons' flowers in a path's joints and verge.
export function flowerColors(P: Palette, season: Season): number[] {
  return season === 'summer' ? [P.petalWhite, P.petalYellow] : [P.petalPink, P.petalLilac, P.petalYellow, P.petalWhite];
}

// ---------------------------------------------------------------- planting

// The plants and leaves the path's joints and verge are dressed with, one
// set per season: small tufts for the joints, taller ones for the verge,
// small flowers, moss cushions and autumn's leaves.
export interface Plants {
  small: THREE.BufferGeometry[];
  tall: THREE.BufferGeometry[];
  flowers: THREE.BufferGeometry[];
  moss: THREE.BufferGeometry[];
  leaves: THREE.BufferGeometry[];
}

export function plantGeometries(season: Season, seed: number): Plants {
  const P = palette(season);
  const random = seededRandom(seed * 137 + 3);
  return {
    small: [tuft(random, P, 0.1, 5), tuft(random, P, 0.14, 6)],
    tall: [tuft(random, P, 0.26, 6), tuft(random, P, 0.34, 7)],
    flowers: flowerColors(P, season).map((petal) => flower(random, P, petal)),
    moss: [mossClump(random, P), mossClump(random, P)],
    leaves: [leaves(random), leaves(random)],
  };
}

// A plant set on the ground: which of a list of shapes, where, turned, how
// big.
export interface Planted {
  shape: number;
  x: number;
  y: number;
  z: number;
  turn: number;
  scale: number;
}

// Each shape's copies drawn in one batch, tinted by the light on the ground
// under each (`light`) and a little at random.
export function planted(shapes: readonly THREE.BufferGeometry[], copies: readonly Planted[], material: THREE.Material, light: (x: number, z: number) => number, random: () => number, tint = 0.08): THREE.InstancedMesh[] {
  const batches: THREE.InstancedMesh[] = [];
  const m = new THREE.Matrix4();
  const q = new THREE.Quaternion();
  shapes.forEach((shape, k) => {
    const list = copies.filter((c) => c.shape === k);
    if (!list.length) return;
    const batch = new THREE.InstancedMesh(shape, material, list.length);
    list.forEach((c, i) => {
      q.setFromAxisAngle(Y, c.turn);
      batch.setMatrixAt(i, m.compose(new THREE.Vector3(c.x, c.y, c.z), q, new THREE.Vector3(c.scale, c.scale, c.scale)));
      batch.setColorAt(i, tinted(light(c.x, c.z), random, tint));
    });
    batch.castShadow = false;
    batch.receiveShadow = true;
    batch.computeBoundingSphere();
    batches.push(batch);
  });
  return batches;
}

// A copy's tint: the light on the ground under it, a little lighter or
// darker at random.
export function tinted(light: number, random: () => number, amount = 0.08): THREE.Color {
  const t = light * (1 + (random() - 0.5) * 2 * amount);
  return new THREE.Color(t, t, t);
}

// Whether a point is on one of the stones laid (their footprints), with
// `room` m more round each (less, below 0).
export function onStone(feet: readonly Footprint[], x: number, z: number, room = 0): Footprint | null {
  for (const f of feet) {
    const dx = x - f.x;
    const dz = z - f.z;
    if (Math.abs(dx) > f.width + f.depth || Math.abs(dz) > f.width + f.depth) continue;
    const along = dx * Math.sin(f.turn) + dz * Math.cos(f.turn);
    const across = dx * Math.cos(f.turn) - dz * Math.sin(f.turn);
    if (Math.abs(along) < f.depth / 2 + room && Math.abs(across) < f.width / 2 + room) return f;
  }
  return null;
}
