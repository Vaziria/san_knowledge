import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { BARK_TILE } from '../../textures/bark';
import { endGrain } from '../../textures/endGrain';
import { noise } from '../../textures/noise';
import { bark, defaultTheme, surface, type Theme } from '../../theme';
import { between, fit, mesh, seededRandom, sizeOf, wobble, type ObjectOptions, type Size } from './parts';

// A log: a length of tree trunk, felled, cut to length and lying on its side
// on the ground, about 2.4 m long and half a meter thick. Like a real trunk
// it is neither quite straight nor quite round: it bows a little along its
// length, thins a little from its thick end (at -x) to its thin end, and its
// girth swells and dips. Both ends are sawn across, a little off square, and
// show the end grain (textures/endGrain.ts): growth rings round the pith,
// which is a little off the middle, darker heartwood, a few drying cracks and
// a rim of bark. A few short stubs stick out of its upper half where
// branches were cut off, each from a swelling in the bark, leaning toward the
// thin end the way the branches grew, and sawn across too. It is bedded a
// little into the ground and cut flat there, so it lies along its length
// rather than resting on a line; a log floating in water (`round`, the lake
// meeting's) is round all the way, since a flat face under it would show
// through the water. The bark is the theme's wood colour through
// the bark texture (bark() in theme.ts), in faint lighter and darker patches
// and darker toward the ground (vertex colours); the cut faces are the plain
// wood colour, paler, like fresh wood.
//
// Units are meters, y is up, the log lies along x with its side to the front
// (+z), and the origin is on the ground at the middle of its footprint. It is
// built from a seed and sized to a width (its length), height and depth (see
// ObjectOptions in parts.ts), so it is the same every time. The spec, Log.md,
// is a draft: it names no behaviour, so the log lies still.

const SIZE: Size = { width: 2.4, height: 0.48, depth: 0.52 };
const SEED = 1;
const LONG = 2.4; // m: the bow and the stubs below are for a log this long, and in proportion for another

// The trunk. Shares are of its radius.
const TAPER = [0.05, 0.12] as const; // the thin end's radius is this share less than the thick end's
// The axis bows out sideways by what the depth leaves once the trunk and the
// stubs have taken theirs, up to BOW m, so the trunk stays round (see
// planLog). Along with the bow it bends in a gentle S, by up to KINK of the
// bow's main curve.
const BOW = 0.045;
const KINK = 0.4;
const HUMP = 0.025; // it rises and dips along the log by up to this share
const GIRTH = { depth: 0.03, along: 0.9 }; // its section swells and dips round it by about `depth` (an oval, three lobes, four), changing over about `along` m
const SINK = 0.08; // bedded this share into the ground all along and cut flat there; more than HUMP and GIRTH, so no part lifts off

// Stubs where branches were cut off, on its upper half only: underneath,
// where it lies, they broke off when it fell. Each leaves the trunk `around`
// radians from straight up, toward the front and the back in turn (branches
// spiral round a trunk), and leans `lean` radians from straight out toward
// the thin end. Its radius, and how far it sticks out of the bark, are shares
// of the trunk's radius. The bark swells round its foot by `collar` of its
// radius, over about `spread` of its radii.
const STUBS = {
  count: [1.5, 3.5] as const, // on a log LONG m long
  around: [0.6, 0.95] as const,
  radius: [0.12, 0.22] as const,
  out: [0.1, 0.28] as const,
  lean: [0.4, 0.8] as const,
  apart: 0.4, // m at least between two, along the log
  ends: 0.35, // m at least from either end
  collar: 0.5,
  spread: 1.7,
  taper: 0.9, // its cut end's radius, times its foot's
};

// The sawn ends: each cut up to `tilt` radians off square either way, with
// the pith up to `pith` of the radius off the axis.
const CUT = { tilt: 0.07, pith: 0.1 };

// Shades of the bark, times the wood colour through the bark texture.
const FOOT = { shade: 0.72, height: 0.4 }; // at the ground, fading out over `height` of the log's height
const TONE = { depth: 0.07, size: 0.5 }; // lighter and darker patches about `size` m across

const ROW = 0.03; // m between rings of points along the log
const COLUMNS = 64; // points round it
const CUT_RINGS = 8; // rings of points on a sawn end, from the pith out
const STUB_ROWS = 4;
const STUB_COLUMNS = 16;
const STUB_RINGS = 3;

interface Stub {
  x: number; // m along the log from its middle, where it leaves the trunk
  around: number; // radians round the trunk from straight up, toward the front (+z)
  radius: number;
  out: number;
  lean: number;
  turn: number; // radians its end grain is turned
}

interface End {
  side: -1 | 1; // the thick end at -x, the thin end at +x
  normal: THREE.Vector3; // out of the log, square to the cut
  pith: [up: number, across: number]; // how far the pith is off the axis, as shares of the radius
  turn: number; // radians the end grain is turned round the pith
}

interface Plan {
  length: number;
  ry: number; // m: the trunk's radius up and down at its thick end, before its girth swells and dips
  rz: number; // and front to back
  taper: number;
  bow: { width: number; kink: number; phase: number; low: number; range: number }; // see bowAt()
  hump: number; // m
  humpPhase: number;
  seed: number; // for the girth and the bark's patches
  sink: number; // share of its radius bedded into the ground: SINK, or none for a round log
  stubs: Stub[];
  ends: [End, End];
}

export interface LogOptions extends ObjectOptions {
  // Round underneath, neither bedded nor cut flat: a log floating in water,
  // whose flat underside would show through it (the lake meeting's; the
  // lake's Logs.ts). Its box is its full girth, from y = 0 up.
  round?: boolean;
}

export class Log extends THREE.Group {
  static readonly SIZE: Size = SIZE;
  readonly bark: THREE.Mesh; // the trunk and its stubs
  readonly cuts: THREE.Mesh; // every sawn face: both ends and the stubs'
  // Its trunk at its middle, stubs aside: its axis's height over the log's
  // bottom, and its radius up and down. The lake floats a round log by them
  // (Lake/Logs.ts).
  readonly axis: number;
  readonly radius: number;

  constructor(options: LogOptions = {}) {
    super();
    this.name = 'log';
    const theme = options.theme ?? defaultTheme;
    const random = seededRandom(options.seed ?? SEED);
    const size = sizeOf(options, SIZE);
    const plan = planLog(size, random, options.round ? 0 : SINK);
    const stubs = plan.stubs.map((stub) => stubGeometry(plan, stub));
    const barkParts = [trunkGeometry(plan), ...stubs.map((s) => s.bark)];
    const cutParts = [...plan.ends.map((end) => endGeometry(plan, end)), ...stubs.map((s) => s.cut)];
    const barkGeo = mergeGeometries(barkParts);
    const cutGeo = mergeGeometries(cutParts);
    for (const part of [...barkParts, ...cutParts]) part.dispose();
    if (!options.round) {
      bed(barkGeo);
      bed(cutGeo);
    }
    const stretch = fit([barkGeo, cutGeo], size);
    this.axis = axisAt(plan, 0).applyMatrix4(stretch).y;
    this.radius = plan.ry * taperAt(plan, 0) * stretch.elements[5];

    const m = createMaterials(theme);
    this.bark = mesh(barkGeo, m.bark);
    this.cuts = mesh(cutGeo, m.cut);
    this.add(this.bark, this.cuts);
  }
}

// Bark (bark() in theme.ts), shaded by the vertex colours, and the sawn faces
// in the plain wood colour through the end grain.
function createMaterials(theme: Theme) {
  const barkSurface = bark(theme);
  barkSurface.vertexColors = true;
  const cut = surface(theme, theme.colors.wood);
  cut.map = endGrain();
  return { bark: barkSurface, cut };
}

function planLog(size: Size, random: () => number, sink: number): Plan {
  const length = size.width;
  const scale = length / LONG;
  const bend = (random() < 0.5 ? -1 : 1) as -1 | 1;
  const kink = KINK * (2 * random() - 1);
  const phase = 2 * Math.PI * random();
  const humpShare = HUMP * (2 * random() - 1);
  const humpPhase = 2 * Math.PI * random();
  const taper = between(random, ...TAPER);
  const seed = Math.floor(random() * 1e6);

  const stubs: Stub[] = [];
  const count = Math.round(between(random, ...STUBS.count) * scale);
  const room = length / 2 - STUBS.ends;
  const side = random() < 0.5 ? -1 : 1;
  for (let attempt = 0; stubs.length < count && room > 0 && attempt < 50; attempt++) {
    const x = between(random, -room, room);
    if (stubs.some((s) => Math.abs(s.x - x) < STUBS.apart)) continue;
    stubs.push({
      x,
      around: (stubs.length % 2 === 0 ? side : -side) * between(random, ...STUBS.around),
      radius: between(random, ...STUBS.radius),
      out: between(random, ...STUBS.out),
      lean: between(random, ...STUBS.lean),
      turn: 2 * Math.PI * random(),
    });
  }

  const tilt = () => Math.tan(CUT.tilt) * (2 * random() - 1);
  const ends = ([-1, 1] as const).map((side): End => {
    const normal = new THREE.Vector3(side, tilt(), tilt()).normalize();
    const angle = 2 * Math.PI * random();
    const off = CUT.pith * Math.sqrt(random());
    return { side, normal, pith: [off * Math.cos(angle), off * Math.sin(angle)], turn: 2 * Math.PI * random() };
  }) as [End, End];

  // The trunk's radius is what the height leaves once the bedding, the hump
  // and the stubs reaching up have taken theirs. Across, the stubs reaching
  // to the front and the back take theirs, and the bow takes up what the
  // depth leaves, so the trunk stays round. Past BOW, or when the stubs leave
  // too little, the trunk is wider or narrower across than up. fit() makes
  // up the last centimeters (the girth's swellings, the stubs' collars).
  const reach = stubReach(stubs);
  const ry = size.height / (2 - sink + Math.abs(humpShare) + reach.up);
  const across = 2 + reach.front + reach.back;
  const width = THREE.MathUtils.clamp(size.depth - ry * across, 0, BOW * scale);
  const rz = (size.depth - width) / across;
  // The bow's curve, measured, so it spans exactly `width`.
  let low = Infinity;
  let high = -Infinity;
  for (let i = 0; i <= 64; i++) {
    const z = curve(kink, phase, i / 64);
    low = Math.min(low, z);
    high = Math.max(high, z);
  }
  const bow = { width: bend * width, kink, phase, low, range: high - low };
  return { length, ry, rz, taper, bow, hump: humpShare * ry, humpPhase, seed, sink, stubs, ends };
}

// How far the stubs reach past the trunk's round, up, to the front and to the
// back, as shares of its radius: each stub's cut end is a disk square to the
// stub, its middle as far out as the stub reaches from the top of its collar.
function stubReach(stubs: Stub[]): { up: number; front: number; back: number } {
  let up = 0;
  let front = 0;
  let back = 0;
  for (const s of stubs) {
    const middle = 1 + STUBS.collar * s.radius + s.out * Math.cos(s.lean);
    const rim = s.radius * STUBS.taper;
    const out = Math.cos(s.lean); // how much the stub points straight out of the trunk
    const cos = Math.cos(s.around);
    const sin = Math.sin(s.around);
    up = Math.max(up, middle * cos + rim * Math.sqrt(1 - (out * cos) ** 2) - 1);
    const across = middle * Math.abs(sin) + rim * Math.sqrt(1 - (out * sin) ** 2) - 1;
    if (sin > 0) front = Math.max(front, across);
    else back = Math.max(back, across);
  }
  return { up, front, back };
}

// The bow's shape at u, 0 at the thick end to 1 at the thin end: one curve
// along the whole log and an S-bend on it.
function curve(kink: number, phase: number, u: number): number {
  return Math.sin(Math.PI * u) + kink * Math.sin(2 * Math.PI * u + phase);
}

// How far the axis bows sideways at u: from 0 at its lowest to the bow's
// width at its highest, centred.
function bowAt(p: Plan, u: number): number {
  const { width, kink, phase, low, range } = p.bow;
  return width * ((curve(kink, phase, u) - low) / range - 0.5);
}

// The trunk's radius at x, as a share of its thick end's.
function taperAt(p: Plan, x: number): number {
  return 1 - p.taper * (x / p.length + 0.5);
}

// The axis at x: bowed sideways, rising and dipping a little, and as high as
// the trunk's radius there less its bedding, so it lies bedded all along
// however it tapers.
function axisAt(p: Plan, x: number, target = new THREE.Vector3()): THREE.Vector3 {
  const u = x / p.length + 0.5;
  return target.set(
    x,
    p.ry * taperAt(p, x) * (1 - p.sink) + p.hump * Math.sin(Math.PI * u + p.humpPhase),
    bowAt(p, u),
  );
}

// How far the trunk's section swells (above 0) or dips at x, at `around`, as
// a share of its radius: an oval, three lobes and four, each turning and
// changing slowly along the log.
function girth(p: Plan, x: number, around: number): number {
  let sum = 0;
  for (let m = 2; m <= 4; m++) {
    sum += wobble(x / GIRTH.along, 3 * m, p.seed) * Math.cos(m * around) + wobble(x / GIRTH.along, 3 * m + 1.5, p.seed) * Math.sin(m * around);
  }
  return (GIRTH.depth / Math.sqrt(3)) * sum;
}

// `a` less `b`, both angles, the short way round: -π..π.
function angleBetween(a: number, b: number): number {
  return ((((a - b + Math.PI) % (2 * Math.PI)) + 2 * Math.PI) % (2 * Math.PI)) - Math.PI;
}

// How far the bark swells round the stubs' feet at x, at `around`, as a share
// of the trunk's radius.
function collars(p: Plan, x: number, around: number): number {
  const r = (p.ry + p.rz) / 2;
  let sum = 0;
  for (const s of p.stubs) {
    const along = (x - s.x) / r;
    const round = angleBetween(around, s.around);
    const spread = STUBS.spread * s.radius;
    sum += STUBS.collar * s.radius * Math.exp(-(along * along + round * round) / (spread * spread));
  }
  return sum;
}

// The trunk's surface at x, at `around` radians round it from straight up,
// toward the front (+z).
function trunkAt(p: Plan, x: number, around: number, target: THREE.Vector3): THREE.Vector3 {
  const axis = axisAt(p, x, target);
  const k = taperAt(p, x) * (1 + girth(p, x, around)) + collars(p, x, around);
  return target.set(x, axis.y + p.ry * k * Math.cos(around), axis.z + p.rz * k * Math.sin(around));
}

// Where along the log the trunk's surface meets an end's cut, at `around`:
// the cut is a plane through the axis at that end, square to its normal.
function cutAt(p: Plan, end: End, around: number): number {
  const center = axisAt(p, (end.side * p.length) / 2);
  const point = new THREE.Vector3();
  let x = center.x;
  for (let i = 0; i < 4; i++) {
    trunkAt(p, x, around, point);
    x -= point.sub(center).dot(end.normal) / end.normal.x;
  }
  return x;
}

// The bark's shade at a point: faint lighter and darker patches, and darker
// toward the ground.
function shade(p: Plan, x: number, around: number, y: number): number {
  const cells = Math.max(1, Math.round((Math.PI * (p.ry + p.rz)) / TONE.size));
  const tone = 1 + TONE.depth * (2 * noise((around / (2 * Math.PI)) * cells, x / TONE.size, cells, p.seed + 1) - 1);
  const height = p.ry * (2 - p.sink);
  return tone * THREE.MathUtils.lerp(FOOT.shade, 1, THREE.MathUtils.smoothstep(y, 0, FOOT.height * height));
}

// The trunk's bark: rings of points along the log, from one sawn end to the
// other, each column running along the trunk at its own angle round it.
// Normals come from the surface's slopes, so the swellings are lit.
// Texture coordinates are meters along the log and whole bark tiles round
// it, so the bark wraps without a seam (textures/bark.ts).
function trunkGeometry(p: Plan): THREE.BufferGeometry {
  const rows = Math.max(8, Math.ceil(p.length / ROW));
  const tiles = Math.max(1, Math.round((Math.PI * (p.ry + p.rz)) / BARK_TILE));
  const positions: number[] = [];
  const normals: number[] = [];
  const uvs: number[] = [];
  const colors: number[] = [];
  const point = new THREE.Vector3();
  const ahead = new THREE.Vector3();
  const behind = new THREE.Vector3();
  const left = new THREE.Vector3();
  const right = new THREE.Vector3();
  const h = 1e-3;
  for (let j = 0; j <= COLUMNS; j++) {
    const around = (j / COLUMNS) * 2 * Math.PI;
    const from = cutAt(p, p.ends[0], around);
    const to = cutAt(p, p.ends[1], around);
    for (let i = 0; i <= rows; i++) {
      const x = THREE.MathUtils.lerp(from, to, i / rows);
      trunkAt(p, x, around, point);
      trunkAt(p, x + h, around, ahead);
      trunkAt(p, x - h, around, behind);
      trunkAt(p, x, around + h, left);
      trunkAt(p, x, around - h, right);
      // (round) x (along) points out of the trunk.
      const normal = left.sub(right).cross(ahead.sub(behind)).normalize();
      positions.push(point.x, point.y, point.z);
      normals.push(normal.x, normal.y, normal.z);
      uvs.push(x, (j / COLUMNS) * tiles * BARK_TILE);
      const c = shade(p, x, around, point.y);
      colors.push(c, c, c);
    }
  }
  return tube(positions, normals, uvs, colors, COLUMNS, rows);
}

// Two triangles for each cell of a grid of `columns` + 1 columns of `rows` +
// 1 points, wound so their faces point out when the columns run round it and
// the rows along it the right way ((round) x (along) outward).
function tube(positions: number[], normals: number[], uvs: number[], colors: number[], columns: number, rows: number): THREE.BufferGeometry {
  const at = (j: number, i: number) => j * (rows + 1) + i;
  const indices: number[] = [];
  for (let j = 0; j < columns; j++) {
    for (let i = 0; i < rows; i++) {
      indices.push(at(j, i), at(j + 1, i), at(j, i + 1));
      indices.push(at(j + 1, i), at(j + 1, i + 1), at(j, i + 1));
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3));
  geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  geo.setIndex(indices);
  return geo;
}

// A sawn end of the log: from its pith out to where the cut meets the bark.
function endGeometry(p: Plan, end: End): THREE.BufferGeometry {
  const x = (end.side * p.length) / 2;
  const center = axisAt(p, x);
  const t = taperAt(p, x);
  const up = new THREE.Vector3(0, 1, 0).addScaledVector(end.normal, -end.normal.y).normalize();
  const across = new THREE.Vector3().crossVectors(end.normal, up);
  const pith = center.addScaledVector(up, end.pith[0] * p.ry * t).addScaledVector(across, end.pith[1] * p.rz * t);
  const outline = Array.from({ length: COLUMNS }, (_, j) => {
    const around = (j / COLUMNS) * 2 * Math.PI;
    return trunkAt(p, cutAt(p, end, around), around, new THREE.Vector3());
  });
  return cutGeometry(pith, outline, end.normal, end.turn, CUT_RINGS);
}

// A stub: a short, round piece of branch in bark, from inside the trunk out
// through the top of its collar, leaning toward the thin end, and its sawn
// end.
function stubGeometry(p: Plan, s: Stub): { bark: THREE.BufferGeometry; cut: THREE.BufferGeometry } {
  const center = axisAt(p, s.x);
  const exit = trunkAt(p, s.x, s.around, new THREE.Vector3());
  const outward = exit.clone().sub(center).normalize();
  const direction = outward.multiplyScalar(Math.cos(s.lean)).add(new THREE.Vector3(Math.sin(s.lean), 0, 0)).normalize();
  const trunk = ((p.ry + p.rz) / 2) * taperAt(p, s.x); // the trunk's radius there
  const radius = s.radius * trunk;
  const inside = 0.5 * exit.distanceTo(center); // it starts this far inside the trunk, along its own line
  const start = exit.clone().addScaledVector(direction, -inside);
  const length = inside + s.out * trunk;
  // Round the stub: e1, e2 and its direction are right-handed.
  const e1 = new THREE.Vector3().crossVectors(direction, new THREE.Vector3(1, 0, 0)).normalize();
  const e2 = new THREE.Vector3().crossVectors(direction, e1);
  const tiles = Math.max(1, Math.round((2 * Math.PI * radius) / BARK_TILE));

  const positions: number[] = [];
  const normals: number[] = [];
  const uvs: number[] = [];
  const colors: number[] = [];
  const offset = new THREE.Vector3();
  const point = new THREE.Vector3();
  const rim: THREE.Vector3[] = [];
  for (let j = 0; j <= STUB_COLUMNS; j++) {
    const angle = (j / STUB_COLUMNS) * 2 * Math.PI;
    offset.copy(e1).multiplyScalar(Math.cos(angle)).addScaledVector(e2, Math.sin(angle));
    for (let i = 0; i <= STUB_ROWS; i++) {
      const t = i / STUB_ROWS;
      point.copy(start).addScaledVector(direction, t * length).addScaledVector(offset, radius * THREE.MathUtils.lerp(1, STUBS.taper, t));
      positions.push(point.x, point.y, point.z);
      normals.push(offset.x, offset.y, offset.z);
      uvs.push(t * length, (j / STUB_COLUMNS) * tiles * BARK_TILE);
      const c = shade(p, s.x, s.around, point.y);
      colors.push(c, c, c);
      if (i === STUB_ROWS && j < STUB_COLUMNS) rim.push(point.clone());
    }
  }
  const end = start.addScaledVector(direction, length);
  return {
    bark: tube(positions, normals, uvs, colors, STUB_COLUMNS, STUB_ROWS),
    cut: cutGeometry(end, rim, direction, s.turn, STUB_RINGS),
  };
}

// A sawn face, from the pith out to its outline (points round it in turn), in
// rings, facing `normal`. Its texture coordinates put each point at its share
// of the way from the pith to the outline, in its direction from the pith
// turned by `turn`, as the end grain wants (textures/endGrain.ts): the rings
// follow the outline, wherever the pith is.
function cutGeometry(pith: THREE.Vector3, outline: THREE.Vector3[], normal: THREE.Vector3, turn: number, rings: number): THREE.BufferGeometry {
  const e1 = new THREE.Vector3(0, 1, 0).addScaledVector(normal, -normal.y);
  if (e1.lengthSq() < 1e-6) e1.set(1, 0, 0).addScaledVector(normal, -normal.x);
  e1.normalize();
  const e2 = new THREE.Vector3().crossVectors(normal, e1);
  const positions = [pith.x, pith.y, pith.z];
  const uvs = [0.5, 0.5];
  const offset = new THREE.Vector3();
  for (let k = 1; k <= rings; k++) {
    const share = k / rings;
    for (const point of outline) {
      offset.subVectors(point, pith);
      const angle = Math.atan2(offset.dot(e2), offset.dot(e1)) + turn;
      positions.push(pith.x + share * offset.x, pith.y + share * offset.y, pith.z + share * offset.z);
      uvs.push(0.5 + 0.5 * share * Math.cos(angle), 0.5 + 0.5 * share * Math.sin(angle));
    }
  }
  const n = outline.length;
  const at = (k: number, j: number) => 1 + (k - 1) * n + (j % n);
  const indices: number[] = [];
  for (let j = 0; j < n; j++) indices.push(0, at(1, j), at(1, j + 1));
  for (let k = 1; k < rings; k++) {
    for (let j = 0; j < n; j++) {
      indices.push(at(k, j), at(k + 1, j), at(k, j + 1));
      indices.push(at(k, j + 1), at(k + 1, j), at(k + 1, j + 1));
    }
  }
  // The outline may run either way round: wind the faces toward `normal`.
  const a = new THREE.Vector3().fromArray(positions, 0);
  const b = new THREE.Vector3().fromArray(positions, 3 * at(1, 0));
  const c = new THREE.Vector3().fromArray(positions, 3 * at(1, 1));
  if (b.sub(a).cross(c.sub(a)).dot(normal) < 0) {
    for (let i = 0; i < indices.length; i += 3) [indices[i + 1], indices[i + 2]] = [indices[i + 2], indices[i + 1]];
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(positions.map((_, i) => normal.getComponent(i % 3)), 3));
  geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
  geo.setIndex(indices);
  return geo;
}

// Cut flat at the ground: whatever lies bedded below it is pressed up onto
// it.
function bed(geo: THREE.BufferGeometry): void {
  const position = geo.getAttribute('position');
  for (let i = 0; i < position.count; i++) if (position.getY(i) < 0) position.setY(i, 0);
}
