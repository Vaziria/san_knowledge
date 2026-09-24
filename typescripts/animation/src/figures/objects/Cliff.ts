import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { defaultTheme } from '../../theme';
import { between, fit, mesh, randomUnit, seededRandom, sizeOf, wobble, type ObjectOptions, type Size } from './parts';

// A natural rock cliff, about 6 m tall, 10 m wide and 4 m deep. Its face,
// toward the front, is steep and craggy: horizontal layers of rock (strata)
// of different thicknesses, each standing out past or back behind the one
// below it in places, so ledges and overhangs come and go along the face,
// with the bedding plane worn into a groove between them. The face bows in
// and out in bays and buttresses. A few joints (vertical fractures) run
// through some of the layers, staggered a little from layer to layer, and
// the blocks between them stand out or back on their own; a few big cracks
// run down from the top. The top edge is broken (some top blocks lower, some
// gone, notches where the cracks reach it), and behind it the back slopes
// down to the ground. The cliff leans back a little as it rises, and at its
// two ends its layers break off one by one, the higher ones mostly sooner,
// so the ends step down to a low shoulder, and the face curves back toward
// them. A few boulders fallen from the face lie at its foot.
// Rock is the theme's stone colour, drawn with flat faces like the lake's
// stones, darker in the cracks and grooves and under overhangs (vertex
// colours), and each layer a shade lighter or darker than the next, so the
// strata show.
//
// Units are meters, y is up, the face looks toward +z and the origin is on
// the ground at the middle of its footprint, boulders included. It is built
// from a seed and sized to a width, height and depth (see ObjectOptions in
// parts.ts), so it is the same every time. The spec, Cliff.md, is a draft: it
// names no behaviour, so the cliff stands still.

const SIZE: Size = { width: 10, height: 6, depth: 4 };
const SEED = 4;

// The face. Depths into it are for a cliff 4 m deep (DEEP), and shrink with
// a shallower one.
const LAYER = [0.35, 1.3] as const; // m thick
const SETBACK = 0.3; // m a layer's face may stand back behind the mean face, or out past it: ledges and overhangs
const LEDGE = { depth: 0.22, size: 2.5 }; // m: and it wanders out and back along the width, over about 2.5 m
const LAYER_LEAN = 0.2; // m a layer's top may stand out past its bottom, or back
const LEAN_BACK = 0.12; // m the face leans back for every meter up (about 7°)
const BAYS = { depth: 0.6, size: 4 }; // m: the face bows in and out along its width, bays about 4 m apart
const CRAG = { depth: 0.14, size: 0.7 }; // m: the rock's roughness, lumps about 0.7 m apart
const GRIT = { depth: 0.05, size: 0.25 }; // m: and finer roughness over that
const BEVEL = 0.08; // m: a layer's edges are broken off this much
const GROOVE = { depth: 0.18, size: 1.8 }; // m the bedding plane is worn in behind the deeper of the two faces at most; in places not at all
const BEDDING = { height: 0.1, size: 2 }; // m a bedding plane wanders up or down along the width, over about 2 m
const FACE_ROWS = 4; // rows of points up a layer's face, between its edges
const COLUMN = 0.3; // m between columns of points along the width
const JITTER = 0.07; // m each point moves along the width at random, so the facets are uneven
const DEEP = 4; // m: the depth the depths above are for

// Joints: vertical fractures, a few lines across the width. Each runs
// through some of the layers (it goes on into the next layer with
// `onward` odds, or starts there with `start` odds), shifted a little in
// each, and the blocks either side of it stand out or back by up to `step`.
const JOINTS = {
  apart: [1.4, 3.5] as const, // m between joint lines
  start: 0.3,
  onward: 0.7,
  shift: 0.25, // m either way, in each layer
  width: [0.12, 0.22] as const,
  depth: [0.08, 0.16] as const,
  step: 0.15,
};

// Big cracks from the top down: `reach` is the share of the height they run
// down, and `notch` how far they cut down into the top edge.
const CRACKS = {
  count: [2, 4] as const,
  width: [0.25, 0.45] as const,
  depth: [0.25, 0.45] as const,
  reach: [0.35, 0.95] as const,
  notch: [0.2, 0.6] as const,
  apart: 1.2, // m at least between two
  fade: 0.8, // m over which a crack closes at its lower end
  wander: 0.08, // m either way, as it runs down
};

// The top: the top layer's blocks are each their own height (a share of the
// layer's thickness), and some are gone. It runs back `depth` of the cliff's
// depth from the face, rounding down `round` m to its back edge.
const TOP = { block: [0.25, 1] as const, missing: 0.3, rough: 0.15, depth: 0.18, round: 0.25, rows: 3 };
// The back slope, down to the ground: steep at the top and easing out at the
// foot. Its foot wanders up to `foot` of the depth forward.
const BACK = { rows: 9, curve: 1.35, rough: 0.2, foot: 0.1 };
// The ends: over `length` m from each end, the layers above `low` of the
// height stop one by one, the higher ones mostly nearer the middle (each at
// `stop` times its share of the height, at random), each breaking off along
// a rough slope `fall` m long rather than in a sheer cut. The ends also
// narrow to `narrow` of the depth, the face curving back toward them, and
// the end faces lean in `inset` m at the top.
const END = {
  length: [1.8, 3] as const,
  low: 0.12,
  stop: [0.55, 1.25] as const,
  fall: [0.35, 0.9] as const,
  narrow: 0.4,
  inset: 0.15,
};

// Shades, times the stone colour.
const TINT = [0.86, 1.1] as const; // each layer a little lighter or darker
const GROOVE_SHADE = 0.6; // at its deepest
const OVERHANG_SHADE = 0.55; // the underside of a layer that juts out OVERHANG_FULL m or more
const SHADED = 0.75; // the top of a layer under one that juts out that far
const OVERHANG_FULL = 0.3;
const CRACK_SHADE = 0.45; // the bottom of a crack or joint
const BACK_SHADE = 0.95;

// Boulders fallen from the face, lying at its foot within `reach` m of it.
// Small ones are far more common than big ones. Each is a sphere with slices
// taken off, like the lake's stones, bedded `sink` of its height into the
// ground and cut flat there.
const SCREE = {
  count: 9, // for a cliff 10 m wide
  radius: [0.15, 0.6] as const, // m, for a cliff 6 m tall
  reach: 0.9,
  flat: [0.5, 0.8] as const, // height as a share of width
  sink: 0.3,
  tilt: 0.2, // radians either way
  shapes: 5,
  cuts: 8,
  cut: [0.55, 0.85] as const, // where a cut sits, as a share of the radius
  tint: [0.85, 1.05] as const,
  underside: 0.7,
};

interface Joint {
  x: number;
  width: number;
  depth: number;
}

interface Layer {
  bottom: number; // m; the bedding planes wander a little along the width
  top: number;
  setback: number; // m back behind the mean face; below 0, out past it
  lean: number; // m its top stands out past its bottom
  tint: number;
  joints: Joint[]; // left to right
  blocks: number[]; // each block's setback from the layer's face, m
  ends: [left: number, right: number]; // m in from each end of the cliff where the layer is whole; 0: it runs right to the end
  falls: [left: number, right: number]; // m beyond that over which it breaks off
}

interface Crack {
  x: number;
  width: number;
  depth: number;
  bottom: number; // m up where it ends
  notch: number;
}

interface Plan {
  size: Size;
  layers: Layer[];
  cracks: Crack[];
  tops: number[]; // the height of each block of the top layer
  ends: [left: number, right: number]; // m over which each end steps down
  foot: number; // z of the face's foot, before its setbacks
  lean: number; // m back for every meter up
  deep: number; // depths into the face are times this
  seeds: number[];
}

export class Cliff extends THREE.Group {
  static readonly SIZE: Size = SIZE;
  readonly rock: THREE.Mesh;
  readonly scree: THREE.Mesh;

  constructor(options: ObjectOptions = {}) {
    super();
    this.name = 'cliff';
    const theme = options.theme ?? defaultTheme;
    const random = seededRandom(options.seed ?? SEED);
    const size = sizeOf(options, SIZE);
    const plan = planCliff(size, random);
    const rock = rockGeometry(plan, random);
    const scree = screeGeometry(plan, random);
    fit(scree ? [rock, scree] : [rock], size);

    // The same matte stone as the lake's rocks, with flat faces, shaded by
    // the vertex colours.
    const material = new THREE.MeshStandardMaterial({ color: theme.scene.stone, roughness: 0.9, flatShading: true, vertexColors: true });
    this.rock = mesh(rock, material);
    this.scree = mesh(scree ?? new THREE.BufferGeometry(), material);
    this.add(this.rock, this.scree);
  }
}

function planCliff(size: Size, random: () => number): Plan {
  const { width, height, depth } = size;
  const half = width / 2;
  const deep = Math.min(1, depth / DEEP);
  const seeds = Array.from({ length: 4 }, () => Math.floor(random() * 1e6));
  const ends: [number, number] = [
    Math.min(between(random, ...END.length), 0.3 * width),
    Math.min(between(random, ...END.length), 0.3 * width),
  ];

  // Layers, stretched together to fill the height.
  const thickness: number[] = [];
  let total = 0;
  while (total < height) {
    thickness.push(between(random, ...LAYER));
    total += thickness[thickness.length - 1];
  }
  // Joint lines across the width, and whether each is in the layer below.
  const lines: number[] = [];
  for (let x = -half + between(random, ...JOINTS.apart) / 2; x < half - COLUMN; x += between(random, ...JOINTS.apart)) lines.push(x);
  const inBelow = lines.map(() => false);

  const layers: Layer[] = [];
  let bottom = 0;
  for (const t of thickness) {
    const top = bottom + (t * height) / total;
    const joints: Joint[] = [];
    lines.forEach((x, i) => {
      inBelow[i] = random() < (inBelow[i] ? JOINTS.onward : JOINTS.start);
      const at = x + JOINTS.shift * (2 * random() - 1);
      if (inBelow[i] && Math.abs(at) < half - 2 * COLUMN) {
        joints.push({ x: at, width: between(random, ...JOINTS.width), depth: between(random, ...JOINTS.depth) });
      }
    });
    joints.sort((a, b) => a.x - b.x);
    // Higher layers mostly stop sooner at the ends; the lowest run right to
    // them.
    const rise = layers.length === 0 ? 0 : THREE.MathUtils.clamp((top / height - END.low) / (1 - END.low), 0, 1);
    const stop = (length: number) => (rise > 0 ? THREE.MathUtils.clamp(length * rise * between(random, ...END.stop), COLUMN, length) : 0);
    const fall = () => between(random, ...END.fall);
    layers.push({
      bottom,
      top,
      setback: between(random, -SETBACK, SETBACK),
      lean: between(random, -LAYER_LEAN, LAYER_LEAN),
      tint: between(random, ...TINT),
      joints,
      blocks: Array.from({ length: joints.length + 1 }, () => between(random, -JOINTS.step, JOINTS.step)),
      ends: [stop(ends[0]), stop(ends[1])],
      falls: [fall(), fall()],
    });
    bottom = top;
  }

  // Big cracks, in the full-height middle, apart from each other.
  const cracks: Crack[] = [];
  const count = Math.round((between(random, ...CRACKS.count) * width) / SIZE.width);
  for (let attempt = 0; cracks.length < count && attempt < 50; attempt++) {
    const x = between(random, -half + ends[0] + CRACKS.apart / 2, half - ends[1] - CRACKS.apart / 2);
    if (cracks.some((c) => Math.abs(c.x - x) < CRACKS.apart)) continue;
    cracks.push({
      x,
      width: between(random, ...CRACKS.width),
      depth: between(random, ...CRACKS.depth),
      bottom: height * (1 - between(random, ...CRACKS.reach)),
      notch: between(random, ...CRACKS.notch),
    });
  }

  // The top layer's blocks: each its own height, some gone down to the
  // layer below.
  const last = layers[layers.length - 1];
  const tops = last.blocks.map(() =>
    random() < TOP.missing
      ? last.bottom + 3 * BEVEL
      : last.bottom + Math.max(3 * BEVEL, (last.top - last.bottom) * between(random, ...TOP.block)),
  );
  // The highest block reaches the full height.
  tops[tops.indexOf(Math.max(...tops))] = height;

  return {
    size,
    layers,
    cracks,
    tops,
    ends,
    foot: depth / 2 - Math.min(SCREE.reach, 0.25 * depth),
    lean: Math.min(LEAN_BACK, (0.25 * depth) / height),
    deep,
    seeds,
  };
}

// The block of a layer at x: its index, and the joint x is on (within a
// quarter of the joint's width of it), or -1.
function blockAt(joints: Joint[], x: number): { index: number; joint: number } {
  let index = 0;
  while (index < joints.length && joints[index].x <= x) index++;
  let joint = -1;
  if (index > 0 && x - joints[index - 1].x < joints[index - 1].width / 4) joint = index - 1;
  if (index < joints.length && joints[index].x - x < joints[index].width / 4) joint = index;
  return { index, joint };
}

// How far a block stands back from its layer's face at x. On a joint, the
// deeper of the two blocks either side.
function blockSetback(layer: Layer, x: number): number {
  const { index, joint } = blockAt(layer.joints, x);
  return joint < 0 ? layer.blocks[index] : Math.max(layer.blocks[joint], layer.blocks[joint + 1]);
}

// A V-shaped cut `width` wide and `depth` deep, centered on `at`.
function vee(x: number, at: number, width: number, depth: number): number {
  return depth * Math.max(0, 1 - Math.abs(x - at) / (width / 2));
}

// How deep the joints and cracks cut into the face at (x, y), and how much of
// their full depth that is (for the shading).
function cuts(p: Plan, layer: Layer, x: number, y: number): { cut: number; share: number } {
  let cut = 0;
  let share = 0;
  for (const j of layer.joints) {
    const c = vee(x, j.x, j.width, j.depth);
    cut += c;
    share = Math.max(share, c / j.depth);
  }
  p.cracks.forEach((crack, i) => {
    const at = crack.x + CRACKS.wander * wobble(y / 1.5, 9.1 * i, p.seeds[3]);
    const c = vee(x, at, crack.width, crack.depth) * THREE.MathUtils.smoothstep(y, crack.bottom, crack.bottom + CRACKS.fade);
    cut += c;
    share = Math.max(share, c / crack.depth);
  });
  return { cut, share };
}

// The heights of the bedding planes at x: each wanders a little.
function beds(p: Plan, x: number): number[] {
  return p.layers.map((layer, k) => (k === 0 ? 0 : layer.bottom + BEDDING.height * wobble(x / BEDDING.size, 3.7 * k, p.seeds[0])));
}

// The face of layer k at (x, y): its z, before the ends narrow, and its shade.
function face(p: Plan, k: number, beds: number[], x: number, y: number): { z: number; shade: number } {
  const layer = p.layers[k];
  const bottom = beds[k];
  const top = k + 1 < beds.length ? beds[k + 1] : p.size.height;
  const t = THREE.MathUtils.clamp((y - bottom) / (top - bottom), 0, 1);
  const { cut, share } = cuts(p, layer, x, y);
  const back =
    layer.setback +
    LEDGE.depth * wobble(x / LEDGE.size, 11.3 * k, p.seeds[1]) +
    blockSetback(layer, x) +
    cut +
    BAYS.depth * wobble(x / BAYS.size, 0.5, p.seeds[1]) -
    layer.lean * (t - 0.5) -
    CRAG.depth * wobble(x / CRAG.size, y / CRAG.size, p.seeds[2]) -
    GRIT.depth * wobble(x / GRIT.size, y / GRIT.size, p.seeds[3]);
  return { z: p.foot - p.lean * y - p.deep * back, shade: layer.tint * THREE.MathUtils.lerp(1, CRACK_SHADE, share) };
}

// The layer a height is in.
function layerAt(beds: number[], y: number): number {
  let k = 0;
  while (k + 1 < beds.length && beds[k + 1] <= y) k++;
  return k;
}

// How far in from the nearer end x is.
function fromEnd(p: Plan, x: number): number {
  return p.size.width / 2 - Math.abs(x);
}

// The height of the top at x: the height of the top layer's block there (on
// a joint, the lower of the two), notched where a crack reaches it. Toward
// the ends, only as high as the layers that still reach x: the first layer
// that is not whole there breaks off along a rough slope, from its top
// where it is whole down to its bottom `fall` m further out.
function topHeight(p: Plan, bed: number[], x: number): number {
  const { height } = p.size;
  const last = p.layers[p.layers.length - 1];
  const { index, joint } = blockAt(last.joints, x);
  let h = joint < 0 ? p.tops[index] : Math.min(p.tops[joint], p.tops[joint + 1]);
  const side = x < 0 ? 0 : 1;
  const inside = fromEnd(p, x);
  for (const [k, layer] of p.layers.entries()) {
    const whole = layer.ends[side];
    if (inside >= whole) continue;
    const fall = layer.falls[side];
    const left = THREE.MathUtils.clamp((inside - (whole - fall)) / fall, 0, 1); // 1 where whole, 0 where gone
    const rough = 0.25 * wobble(x / 0.4, 13.3 * k, p.seeds[0]) * 4 * left * (1 - left);
    const top = k + 1 < bed.length ? bed[k + 1] : height;
    h = Math.min(h, bed[k] + (top - bed[k]) * THREE.MathUtils.clamp(left + rough, 0, 1));
    break;
  }
  h -= TOP.rough * (0.5 + 0.5 * wobble(x / 1.2, 5.1, p.seeds[3]));
  for (const crack of p.cracks) h -= vee(x, crack.x, crack.width, crack.notch);
  return Math.max(2 * BEVEL, Math.min(h, height));
}

// The kinds of rows of points along a column, from the face's foot up and
// over the top to the back's foot: the face of each layer (its bottom edge,
// FACE_ROWS rows, its top edge), the groove between two layers, the top's
// front edge and its rows, and the back slope's rows.
type Row =
  | { kind: 'face'; layer: number; at: number } // at: 0 its bottom edge, 1 its top edge
  | { kind: 'groove'; layer: number } // below this layer
  | { kind: 'top'; at: number } // 0 its front edge, 1 its back edge
  | { kind: 'back'; at: number }; // 0 just below the top's back edge, 1 its foot

function rowsOf(p: Plan): Row[] {
  const rows: Row[] = [];
  p.layers.forEach((_, k) => {
    if (k > 0) rows.push({ kind: 'groove', layer: k });
    for (let i = 0; i <= FACE_ROWS + 1; i++) rows.push({ kind: 'face', layer: k, at: i / (FACE_ROWS + 1) });
  });
  for (let i = 0; i <= TOP.rows; i++) rows.push({ kind: 'top', at: i / TOP.rows });
  for (let i = 1; i <= BACK.rows; i++) rows.push({ kind: 'back', at: i / BACK.rows });
  return rows;
}

// Where the columns of points stand along the width: every COLUMN m, plus
// three at each joint and crack (its two lips and its bottom) so they are cut
// sharp, two where each layer breaks off at the ends (the top and the foot
// of the break), and one at each end.
function columnsOf(p: Plan): { x: number; loose: boolean }[] {
  const half = p.size.width / 2;
  const sharp: number[] = [];
  for (const layer of p.layers) {
    for (const j of layer.joints) sharp.push(j.x - j.width / 2, j.x, j.x + j.width / 2);
    layer.ends.forEach((whole, side) => {
      const sign = side === 0 ? -1 : 1;
      if (whole > 0) sharp.push(sign * (half - whole));
      if (whole - layer.falls[side] > 0) sharp.push(sign * (half - whole + layer.falls[side]));
    });
  }
  for (const c of p.cracks) sharp.push(c.x - c.width / 2, c.x, c.x + c.width / 2);
  const columns = sharp.filter((x) => Math.abs(x) < half - COLUMN / 2).map((x) => ({ x, loose: false }));
  const n = Math.ceil(p.size.width / COLUMN);
  for (let i = 0; i <= n; i++) {
    const x = -half + (i * p.size.width) / n;
    const end = i === 0 || i === n;
    if (end || columns.every((c) => Math.abs(c.x - x) > COLUMN / 3)) columns.push({ x, loose: !end });
  }
  columns.sort((a, b) => a.x - b.x);
  return columns.filter((c, i) => i === 0 || c.x - columns[i - 1].x > 0.01);
}

// The rock: one surface over the face, the top and the back, a grid of
// columns (along the width) by rows (from the face's foot to the back's),
// closed at both ends. Its bottom stands on the ground and is left open.
function rockGeometry(p: Plan, random: () => number): THREE.BufferGeometry {
  const { height, depth } = p.size;
  const rows = rowsOf(p);
  const columns = columnsOf(p);
  const positions: number[] = [];
  const colors: number[] = [];
  const middle = (p.foot - depth / 2) / 2; // the ends narrow toward it
  const overhang = (out: number) => THREE.MathUtils.clamp(out / OVERHANG_FULL, 0, 1);

  for (const [c, column] of columns.entries()) {
    const x = column.x;
    const bed = beds(p, x);
    const h = topHeight(p, bed, x);
    const clip = h - BEVEL; // the face stops here and turns onto the top
    const edge = face(p, layerAt(bed, clip), bed, x, clip);
    const narrow = THREE.MathUtils.lerp(END.narrow, 1, THREE.MathUtils.smoothstep(fromEnd(p, x), 0, x < 0 ? p.ends[0] : p.ends[1]));
    // The top runs back from the face's edge to the back edge, measured from
    // the mean face (so a crack doesn't push the back in too), and at least
    // a quarter of its depth.
    const topDepth = TOP.depth * depth;
    const front = edge.z - p.deep * BEVEL;
    const mean = p.foot - p.lean * h - p.deep * BAYS.depth * wobble(x / BAYS.size, 0.5, p.seeds[1]);
    const backEdge = Math.min(mean - topDepth, front - topDepth / 4);
    const backFoot = -depth / 2 + BACK.foot * depth * (0.5 + 0.5 * wobble(x / 2, 7.7, p.seeds[0]));
    const endColumn = c === 0 || c === columns.length - 1;
    let lastTop = h;

    for (const row of rows) {
      let y: number;
      let z: number;
      let shade: number;
      if (row.kind === 'face' || row.kind === 'groove') {
        const k = row.layer;
        const top = k + 1 < bed.length ? bed[k + 1] : height;
        if (row.kind === 'groove') {
          y = bed[k];
          const below = face(p, k - 1, bed, x, y - BEVEL);
          const above = face(p, k, bed, x, y + BEVEL);
          // Worn in by up to GROOVE.depth, and not at all where the wobble
          // is below 0, so the grooves come and go.
          const worn = Math.max(0, wobble(x / GROOVE.size, 5.1 * k, p.seeds[2]));
          z = Math.min(below.z, above.z) - p.deep * GROOVE.depth * worn;
          shade = Math.min(below.shade, above.shade) * THREE.MathUtils.lerp(1, GROOVE_SHADE, worn);
        } else {
          const from = k === 0 ? 0 : bed[k] + BEVEL;
          const to = k + 1 < bed.length ? top - BEVEL : height - BEVEL;
          y = THREE.MathUtils.lerp(from, to, row.at);
          if (row.at > 0 && row.at < 1) y += 0.15 * ((to - from) / (FACE_ROWS + 1)) * (2 * random() - 1);
          const here = face(p, k, bed, x, y);
          z = here.z;
          shade = here.shade;
          // The top edge of a layer under one that juts out is in its shade,
          // and the bottom edge of a layer that juts out is its underside,
          // the darker the further it juts out.
          if (row.at === 1 && k + 1 < bed.length) {
            shade *= THREE.MathUtils.lerp(1, SHADED, overhang(face(p, k + 1, bed, x, top + BEVEL).z - z));
          }
          if (row.at === 0 && k > 0) {
            const out = overhang(z - face(p, k - 1, bed, x, bed[k] - BEVEL).z);
            shade = THREE.MathUtils.lerp(shade, p.layers[k].tint * OVERHANG_SHADE, out);
          }
        }
        // Above the top, the face's rows all fold into its edge there.
        if (y >= clip) {
          y = clip;
          z = edge.z;
          shade = edge.shade;
        }
      } else if (row.kind === 'top') {
        const t = row.at;
        y = h - TOP.round * t * t - (t > 0 ? TOP.rough * (0.5 + 0.5 * wobble(x / 0.9, 2 + 3 * t, p.seeds[3])) : 0);
        z = THREE.MathUtils.lerp(front, backEdge, t);
        shade = edge.shade;
        lastTop = y;
      } else {
        const t = row.at;
        y = Math.max(0, lastTop * Math.pow(1 - t, BACK.curve) + BACK.rough * Math.sin(Math.PI * t) * wobble(x / 1.5, 3 * t, p.seeds[2]));
        if (t === 1) y = 0;
        z = THREE.MathUtils.lerp(backEdge, backFoot, t);
        shade = p.layers[layerAt(bed, y)].tint * BACK_SHADE;
      }

      z = middle + (z - middle) * narrow;
      let px = x;
      if (column.loose) px += JITTER * (2 * random() - 1);
      if (endColumn) px -= Math.sign(x) * END.inset * (y / h);
      positions.push(px, y, z);
      colors.push(shade, shade, shade);
    }
  }

  // Two triangles for each cell, wound so their faces point out: along the
  // width x along the rows points out of the face, the top and the back.
  const n = rows.length;
  const at = (c: number, r: number) => c * n + r;
  const indices: number[] = [];
  for (let c = 0; c + 1 < columns.length; c++) {
    for (let r = 0; r + 1 < n; r++) {
      indices.push(at(c, r), at(c + 1, r), at(c, r + 1));
      indices.push(at(c + 1, r), at(c + 1, r + 1), at(c, r + 1));
    }
  }
  closeEnd(positions, 0, n, -1, indices);
  closeEnd(positions, columns.length - 1, n, 1, indices);

  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  geo.setIndex(indices);
  geo.computeVertexNormals(); // for the shadow's offsets; the faces are drawn flat
  return geo;
}

// Closes one end of the rock: the column's outline, seen from the side (z,
// y), cut into triangles facing out (toward -x at the left end, +x at the
// right).
function closeEnd(positions: number[], column: number, rows: number, side: -1 | 1, indices: number[]): void {
  const outline: THREE.Vector2[] = [];
  const ids: number[] = [];
  for (let r = 0; r < rows; r++) {
    const i = column * rows + r;
    const point = new THREE.Vector2(positions[3 * i + 2], positions[3 * i + 1]);
    if (outline.length > 0 && point.distanceTo(outline[outline.length - 1]) < 1e-4) continue;
    outline.push(point);
    ids.push(i);
  }
  const a = new THREE.Vector3();
  const b = new THREE.Vector3();
  const c = new THREE.Vector3();
  const vertex = (v: THREE.Vector3, i: number) => v.fromArray(positions, 3 * i);
  for (const [i, j, k] of THREE.ShapeUtils.triangulateShape(outline, [])) {
    vertex(a, ids[i]);
    vertex(b, ids[j]);
    vertex(c, ids[k]);
    const normal = b.sub(a).cross(c.sub(a));
    if (normal.x * side >= 0) indices.push(ids[i], ids[j], ids[k]);
    else indices.push(ids[i], ids[k], ids[j]);
  }
}

// A unit sphere with random slices taken off, like chipped stone: every
// point beyond a cutting plane is pressed back onto it, leaving a flat face.
function boulderShape(random: () => number): THREE.BufferGeometry {
  const geo = new THREE.IcosahedronGeometry(1, 2);
  geo.deleteAttribute('uv');
  const planes = Array.from({ length: SCREE.cuts }, () => ({ normal: randomUnit(random), depth: between(random, ...SCREE.cut) }));
  const position = geo.getAttribute('position');
  const p = new THREE.Vector3();
  for (let i = 0; i < position.count; i++) {
    p.fromBufferAttribute(position, i);
    for (const plane of planes) {
      const beyond = p.dot(plane.normal) - plane.depth;
      if (beyond > 0) p.addScaledVector(plane.normal, -beyond);
    }
    position.setXYZ(i, p.x, p.y, p.z);
  }
  return geo;
}

// The boulders at the foot of the face, one geometry: each a boulder shape
// turned, tilted and sized on its own, bedded into the ground and cut flat
// there, darker beneath. Only in front of the full-height middle, not the
// stepped ends. None for a cliff too small to have room.
function screeGeometry(p: Plan, random: () => number): THREE.BufferGeometry | null {
  const { width, height, depth } = p.size;
  const shapes = Array.from({ length: SCREE.shapes }, () => boulderShape(random));
  const scale = Math.min(1, height / SIZE.height, depth / SIZE.depth);
  const count = Math.round((SCREE.count * width) / SIZE.width);
  const from = -width / 2 + p.ends[0];
  const to = width / 2 - p.ends[1];
  const reach = depth / 2 - p.foot;
  const parts: THREE.BufferGeometry[] = [];
  const matrix = new THREE.Matrix4();
  const turn = new THREE.Quaternion();
  const euler = new THREE.Euler();
  for (let i = 0; i < count && to > from; i++) {
    const r = scale * (SCREE.radius[0] + (SCREE.radius[1] - SCREE.radius[0]) * random() ** 2);
    const half = r * between(random, ...SCREE.flat);
    const x = between(random, from, to);
    const foot = face(p, 0, beds(p, x), x, 0).z;
    const z = Math.min(foot + 0.5 * r + random() * Math.max(0, reach - 1.5 * r), depth / 2 - r);
    euler.set(between(random, -SCREE.tilt, SCREE.tilt), 2 * Math.PI * random(), between(random, -SCREE.tilt, SCREE.tilt));
    matrix.compose(new THREE.Vector3(x, half * (1 - 2 * SCREE.sink), z), turn.setFromEuler(euler), new THREE.Vector3(r, half, r * between(random, 0.8, 1.2)));
    const geo = shapes[Math.floor(random() * SCREE.shapes)].clone().applyMatrix4(matrix);
    // Cut flat at the ground.
    const position = geo.getAttribute('position');
    for (let v = 0; v < position.count; v++) if (position.getY(v) < 0) position.setY(v, 0);
    geo.computeVertexNormals();
    const tint = between(random, ...SCREE.tint);
    const normal = geo.getAttribute('normal');
    const colors: number[] = [];
    for (let v = 0; v < normal.count; v++) {
      const shade = tint * THREE.MathUtils.lerp(SCREE.underside, 1, 0.5 + 0.5 * normal.getY(v));
      colors.push(shade, shade, shade);
    }
    geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    parts.push(geo);
  }
  for (const shape of shapes) shape.dispose();
  if (parts.length === 0) return null;
  const merged = mergeGeometries(parts);
  for (const part of parts) part.dispose();
  return merged;
}
