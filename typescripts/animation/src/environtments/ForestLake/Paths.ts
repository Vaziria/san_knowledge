import { noise2 } from '../../figures/ForestLake/parts';
import { alongLine, VARIATIONS, type Stretch, type Variation } from '../../figures/ForestLake/StonePath/parts';
import { LANDING, Line, SPOTS, type Point } from './layout';
import { smoothstep, type Terrain } from './Terrain';

// The forest lake's paths (ForestLake.md, item 6), as they change along
// their way (task 20): each path a run of stretches 8–30 m long, no two
// neighbours alike, each of a kind that suits where it goes; the stone path
// (figures/ForestLake/StonePath) in its variations, and sandy dirt, two
// worn tracks, stepping stones, gravel and log steps (Wayside.ts). A path
// is wider where paths meet and by the camp, the landing and the gate,
// narrower in the forest and on the heights; its edges are uneven. Its line
// stays where layout.ts has it, so the fences, lamp posts, sign posts and
// the gate still stand by it. And trails, to places no path reached.

export type PathKind = Variation | 'dirt' | 'tracks' | 'stepping' | 'gravel' | 'logs';
const STONE = new Set<PathKind>(VARIATIONS);

// The kinds each path is made of, by its place in PATHS, in order from its
// start: its stretches take them in turn. Change them here.
export const PATH_KINDS: readonly { kinds: readonly PathKind[]; path: string }[] = [
  { kinds: ['dirty', 'regular', 'dirt', 'regular'], path: 'the main path, from the camp along the south shore past the landing to the bridge: stone mostly, dirty stone by the camp' },
  { kinds: ['mossy', 'dirt'], path: 'on from the bridge along the east shore to the cave: mossy stone, and dirt in the forest shade' },
  { kinds: ['ruined', 'gravel'], path: 'from the camp to the ruins: ruined stone, then gravel' },
  { kinds: ['tracks'], path: "the camp's spur to the pier: dirt worn into two tracks" },
  { kinds: ['stepping', 'gravel'], path: 'along the heights from the watch tower: stepping stones and gravel' },
  { kinds: ['regular', 'tracks', 'regular'], path: 'the way in, from the gate through the forest: stone, and two tracks under the trees' },
];
// And over the table, wherever a path climbs more steeply than STEEP (1 in
// 5, over a meter along it) it is steps: stone stairs on a path NARROW m
// wide or more, log steps on a narrower one; steeper than CLIFF (the
// heights path's end, down the cliff over the cave) nothing is laid.
const STEEP = 0.2;
const CLIFF = 0.8;
const NARROW = 2;
const LEAST = 8; // m: a stretch is at least this long, as far as the path allows
const BLEND = 1.8; // m over which a stone path breaks up into single stones where it meets another kind
const WIDER = 0.3; // how much wider or narrower a path is at most than its width in PATHS

// The trails, to places no path reached: trodden dirt or stepping stones.
// What stood on one is taken out once everything is placed (Scatter's
// `cleared`), so nothing else moves.
export const TRAILS: readonly { name: string; points: Point[]; width: number; kind: 'dirt' | 'stepping' }[] = [
  {
    name: "to the spaceship wreck's ramp, from the forest path's end by the camp: east of the camp's fence, then west between the wreck's debris",
    points: [
      [-25.8, 17.4],
      [-26, 22.4],
      [-26.5, 23.35],
      [-27.6, 23.68],
      [-29.3, 23.95],
      [-31.1, 24.2],
      [-32.6, 24.25],
      [-34.4, 24.55],
      [-35.4, 25.3],
      [-35.9, 26.4],
      [-36, 27.7],
    ],
    width: 0.8,
    kind: 'dirt',
  },
  {
    name: "along the heights, from the watch tower's path round the tower's east side to the footbridge over the main river, meeting its deck between the rails",
    points: [
      [29, -23],
      [30.6, -23.9],
      [30.9, -27.6],
      [28.6, -29.6],
      [22.4, -32.2],
      [14.65, -33.4],
      [8.8, -34.1],
      [6.7, -34.25],
    ],
    width: 0.7,
    kind: 'stepping',
  },
];
const TRAIL_ROOM = 0.35; // m more than half a trail's width cleared of grass and flowers either side of it
const TRAIL_HEAD = 1.5; // m from a trail's middle cleared of what stands (trunks, bushes, rocks), so a walker's head is out of the trees' lowest boughs

// ---------------------------------------------------------------- clearing

export interface PathClearing {
  // Under the paths as task 19 laid its stone path: the scatter's flat
  // stones there go, and grass, flowers and leaves (Scatter's \`bare\`); what
  // stands in a path stays.
  covers(x: number, z: number): boolean;
  // On a trail: everything goes; what stands (not `bare`) further out, by
  // TRAIL_HEAD.
  clears(x: number, z: number, bare: boolean): boolean;
  trails: Line[];
}

// What the paths and trails clear of the scatter, known before it is
// placed.
export function pathClearing(terrain: Terrain): PathClearing {
  const band = grid();
  for (const { course, width } of task19Band(terrain)) {
    for (let d = 0; d <= course.length; d += SAMPLE) {
      const at = course.at(d, 0);
      band.add(at.x, at.z, width / 2 + 0.08);
    }
  }
  const trails = TRAILS.map((trail) => new Line(trail.points, 6, 0.8));
  const cleared = grid();
  const head = grid();
  trails.forEach((line, i) => {
    for (let d = 0; d <= line.length; d += 0.25) {
      const at = line.at(d);
      cleared.add(at.x, at.z, TRAILS[i].width / 2 + TRAIL_ROOM);
      head.add(at.x, at.z, TRAIL_HEAD);
    }
  });
  return { covers: band.within, clears: (x, z, bare) => (bare ? cleared : head).within(x, z), trails };
}

// The stretches task 19 laid its stone path along, by its rules (its
// variation by path, dirty round the camp, stairs where steep, none down a
// cliff), sampled as it sampled them: the band the scatter is cleared
// under stays exactly its.
const BAND_KINDS = ['regular', 'mossy', 'ruined', 'dirty', 'mossy', 'regular'] as const;
function task19Band(terrain: Terrain): { course: ReturnType<typeof alongLine>; width: number }[] {
  const ground = (x: number, z: number) => terrain.heightAt(x, z);
  const out: { course: ReturnType<typeof alongLine>; width: number }[] = [];
  terrain.paths.forEach(({ line, width }, i) => {
    const base = BAND_KINDS[i] ?? 'regular';
    const length = line.length;
    const kinds: (string | null)[] = [];
    for (let d = 0; d <= length; d += SAMPLE) {
      const slope = slopeAt(line, d, ground);
      const p = line.at(d);
      const camp = Math.hypot(p.x - SPOTS.camp.x, p.z - SPOTS.camp.z) < SPOTS.camp.radius + 1.5;
      kinds.push(slope > CLIFF ? null : slope > STEEP ? 'stairs' : camp ? 'dirty' : base);
    }
    const runs: { kind: string | null; from: number; to: number }[] = [];
    kinds.forEach((kind, k) => {
      const at = k * SAMPLE;
      const last = runs[runs.length - 1];
      if (last && last.kind === kind) last.to = Math.min(length, at + SAMPLE);
      else runs.push({ kind, from: at, to: Math.min(length, at + SAMPLE) });
    });
    for (let k = runs.length - 1; k > 0; k--) {
      const run = runs[k];
      if (run.kind !== 'stairs' && run.kind !== null && run.to - run.from < 1 && runs[k - 1].kind !== null) {
        runs[k - 1].to = run.to;
        runs.splice(k, 1);
      }
    }
    for (const run of runs) if (run.kind !== null && run.to - run.from >= 0.3) out.push({ course: alongLine(line.points, run.from, run.to, ground), width });
  });
  return out;
}

// Where along a path stones and dirt are laid: all of it but where it runs
// down a cliff (task 19's samples, a meter along it).
function laidRanges(line: Line, ground: (x: number, z: number) => number): [number, number][] {
  const ranges: [number, number][] = [];
  const length = line.length;
  let from: number | null = null;
  for (let d = 0; d <= length + 1e-6; d += SAMPLE) {
    const cliff = slopeAt(line, d, ground) > CLIFF;
    if (!cliff && from === null) from = d;
    if ((cliff || d + SAMPLE > length) && from !== null) {
      ranges.push([from, cliff ? d : length]);
      from = null;
    }
  }
  return ranges;
}
const SAMPLE = 0.25;

// How steep a line climbs at a point, over a meter along it.
function slopeAt(line: Line, d: number, ground: (x: number, z: number) => number): number {
  const length = line.length;
  const a = line.at(Math.max(0, d - 0.5));
  const b = line.at(Math.min(length, d + 0.5));
  const run = Math.max(0.1, Math.min(length, d + 0.5) - Math.max(0, d - 0.5));
  return Math.abs(ground(b.x, b.z) - ground(a.x, a.z)) / run;
}

// Points with a reach round each, CELL m to a cell, and whether a point is
// within one's reach.
function grid(): { add(x: number, z: number, reach: number): void; within(x: number, z: number): boolean } {
  const CELL = 2;
  const cells = new Map<string, { x: number; z: number; reach: number }[]>();
  return {
    add(x, z, reach) {
      const key = `${Math.floor(x / CELL)},${Math.floor(z / CELL)}`;
      let list = cells.get(key);
      if (!list) cells.set(key, (list = []));
      list.push({ x, z, reach });
    },
    within(x, z) {
      const i = Math.floor(x / CELL);
      const j = Math.floor(z / CELL);
      for (let di = -1; di <= 1; di++) {
        for (let dj = -1; dj <= 1; dj++) {
          for (const p of cells.get(`${i + di},${j + dj}`) ?? []) if (Math.hypot(x - p.x, z - p.z) < p.reach) return true;
        }
      }
      return false;
    },
  };
}

// ---------------------------------------------------------------- the plan

// A path planned: its line, how wide it is along it, its stretches (each
// of a kind, from and to m along it), where it is steps, and where nothing
// is laid.
export interface PlannedPath {
  name: string;
  trail: boolean;
  line: Line;
  base: number; // m, its width in PATHS (or the trail's)
  widthAt(along: number): number;
  stretches: { kind: PathKind; from: number; to: number }[];
  steps: { kind: 'stairs' | 'logs'; from: number; to: number }[];
  bare: [number, number][]; // where nothing is laid (down a cliff)
}

export interface PathPlan {
  paths: PlannedPath[];
  // The stone path's stretches (StonePath), and every other kind's
  // (Wayside), each with its course, width along it and kind.
  stone: Stretch[];
  other: { kind: Exclude<PathKind, Variation>; course: ReturnType<typeof alongLine>; width: number; widthAt: (along: number) => number; forest: (along: number) => number; path: PlannedPath; from: number }[];
  // How much of the paths' sandy dirt shows on the ground at a point
  // (Terrain.meshing), 0..1.
  paint(x: number, z: number): number;
}

// The plan, once the scatter is placed (its trunks tell the forest's
// stretches) and the landmarks stand (a path stays clear of their posts).
export function planPaths(terrain: Terrain, clearing: PathClearing, trunks: readonly { x: number; z: number }[], posts: readonly { x: number; z: number; radius: number }[]): PathPlan {
  const ground = (x: number, z: number) => terrain.heightAt(x, z);
  // How much of the forest a point is in: the trunks within 5 m.
  const cells = new Map<string, { x: number; z: number }[]>();
  for (const t of trunks) {
    const key = `${Math.floor(t.x / 5)},${Math.floor(t.z / 5)}`;
    let list = cells.get(key);
    if (!list) cells.set(key, (list = []));
    list.push(t);
  }
  const forestAt = (x: number, z: number) => {
    let n = 0;
    const i = Math.floor(x / 5);
    const j = Math.floor(z / 5);
    for (let di = -1; di <= 1; di++) for (let dj = -1; dj <= 1; dj++) for (const t of cells.get(`${i + di},${j + dj}`) ?? []) if (Math.hypot(t.x - x, t.z - z) < 5) n++;
    return smoothstep(2, 7, n);
  };
  const small = posts.filter((p) => p.radius <= 1.2);
  const lines = terrain.paths.map((p) => p.line);
  const paths: PlannedPath[] = [];

  terrain.paths.forEach(({ line, width }, i) => {
    const length = line.length;
    // Where it meets another path: its ends near another's line, and where
    // another's end comes near it.
    const meets: { x: number; z: number }[] = [];
    for (const d of [0, length]) {
      const at = line.at(d);
      if (lines.some((other, k) => k !== i && other.nearest(at.x, at.z).distance < 3)) meets.push({ x: at.x, z: at.z });
    }
    lines.forEach((other, k) => {
      if (k === i) return;
      for (const d of [0, other.length]) {
        const at = other.at(d);
        if (line.nearest(at.x, at.z).distance < 3) meets.push({ x: at.x, z: at.z });
      }
    });
    const gate = i === SPOTS.gate.path ? line.at(SPOTS.gate.along) : null;
    const places = [...meets, { x: SPOTS.camp.x, z: SPOTS.camp.z, reach: SPOTS.camp.radius + 4 }, { x: LANDING[0], z: LANDING[1], reach: 6 }, ...(gate ? [{ x: gate.x, z: gate.z, reach: 3 }] : [])];
    const samples: number[] = [];
    for (let d = 0; d <= length + 1e-6; d += 0.5) {
      const at = line.at(d);
      const wander = (noise2(d * 0.11, i * 7.3, 91) - 0.5) * 2;
      const wide = Math.max(0, ...places.map((p) => smoothstep('reach' in p ? (p.reach as number) : 4, 0, Math.hypot(at.x - p.x, at.z - p.z))));
      const narrow = Math.max(forestAt(at.x, at.z), smoothstep(4, 7, ground(at.x, at.z)));
      const s = Math.max(-1, Math.min(1, 0.55 * wander + 1.1 * wide - 0.9 * narrow));
      let w = width * (1 + WIDER * s);
      // Clear of the posts along it (lamp posts, fences, sign posts, the
      // gate's): its edge 0.3 m short of each.
      for (const p of small) {
        const d2 = Math.hypot(at.x - p.x, at.z - p.z);
        if (d2 < 3) w = Math.min(w, Math.max(width * (1 - WIDER), 2 * (d2 - 0.3)));
      }
      samples.push(w);
    }
    const widthAt = sampled(samples, 0.5);
    paths.push({ name: PATH_KINDS[i]?.path ?? `path ${i}`, trail: false, line, base: width, widthAt, stretches: stretchesOf(length, PATH_KINDS[i]?.kinds ?? ['dirt'], i), steps: stepsAlong(line, ground, width >= NARROW ? 'stairs' : 'logs'), bare: bareOf(line, ground) });
  });
  clearing.trails.forEach((line, i) => {
    const trail = TRAILS[i];
    paths.push({ name: trail.name, trail: true, line, base: trail.width, widthAt: () => trail.width, stretches: [{ kind: trail.kind, from: 0, to: line.length }], steps: [], bare: [] });
  });

  // Each stretch laid, split where the path is steps or bare.
  const stone: Stretch[] = [];
  const other: PathPlan['other'] = [];
  for (const path of paths) {
    const { line, stretches } = path;
    const forest = (along: number) => {
      const at = line.at(along);
      return forestAt(at.x, at.z);
    };
    stretches.forEach((stretch, k) => {
      const before = stretches[k - 1];
      const after = stretches[k + 1];
      const pieces = cut(stretch.from, stretch.to, path);
      for (const piece of pieces) {
        const course = alongLine(line.points, piece.from, piece.to, ground);
        const widthAt = (along: number) => path.widthAt(piece.from + along);
        const kind = piece.steps ?? stretch.kind;
        if (STONE.has(kind)) {
          // A stone path breaks up into single stones where it meets
          // another kind.
          const fadeStart = before && before.kind !== stretch.kind && Math.abs(piece.from - stretch.from) < 1e-6 && !piece.steps ? BLEND : 0;
          const fadeEnd = after && after.kind !== stretch.kind && Math.abs(piece.to - stretch.to) < 1e-6 && !piece.steps ? BLEND : 0;
          const wear = stretch.kind === 'ruined' ? (along: number) => (piece.from + along) / line.length : undefined;
          stone.push({ course, width: path.base, widthAt, variation: kind as Variation, wear, fade: [fadeStart, fadeEnd] });
        } else {
          other.push({ kind: kind as Exclude<PathKind, Variation>, course, width: path.base, widthAt, forest: (along) => forest(piece.from + along), path, from: piece.from });
        }
      }
    });
  }

  // The ground's sandy dirt under each path, by its kind.
  const paint = (x: number, z: number) => {
    let most = 0;
    for (const path of paths) {
      const near = path.line.nearest(x, z);
      if (near.segment < 0 || near.distance > path.base * (1 + WIDER) + 1) continue;
      const along = near.along;
      if (path.bare.some(([a, b]) => along > a && along < b)) continue;
      // Uneven edges: the grass reaching in and bare earth reaching out.
      const half = path.widthAt(along) / 2 + (noise2(x * 0.9, z * 0.9, 17) - 0.5) * 0.4 + (noise2(x * 2.3, z * 2.3, 19) - 0.5) * 0.16;
      const kind = kindAt(path, along);
      most = Math.max(most, amountOf(kind.kind, near.distance, half, path.trail) * (1 - kind.blend) + (kind.next ? amountOf(kind.next, near.distance, half, path.trail) * kind.blend : 0));
    }
    return most;
  };
  return { paths, stone, other, paint };
}

// A path's stretches: `kinds` in turn, each LEAST m long at least (as far as
// the path's length allows every kind a stretch), the rest shared out at
// random, seeded by the path.
function stretchesOf(length: number, kinds: readonly PathKind[], seed: number): { kind: PathKind; from: number; to: number }[] {
  let n = Math.max(1, Math.min(kinds.length, Math.floor(length / LEAST)));
  // Let a path just short of room for all its kinds keep them (the path to
  // the ruins, 15.8 m, ruined then gravel).
  if (n < kinds.length && length / kinds.length >= LEAST - 0.5) n = kinds.length;
  const least = Math.min(LEAST, length / n);
  const extra = length - least * n;
  const shares = Array.from({ length: n }, (_, k) => 0.5 + noise2(k * 3.1, seed * 5.7, 97));
  const total = shares.reduce((a, b) => a + b, 0);
  const out: { kind: PathKind; from: number; to: number }[] = [];
  let at = 0;
  for (let k = 0; k < n; k++) {
    const to = k === n - 1 ? length : at + least + (extra * shares[k]) / total;
    out.push({ kind: kinds[k % kinds.length], from: at, to });
    at = to;
  }
  return out;
}

// Where a path is steps (steeper than STEEP), of `kind`.
function stepsAlong(line: Line, ground: (x: number, z: number) => number, kind: 'stairs' | 'logs'): { kind: 'stairs' | 'logs'; from: number; to: number }[] {
  const out: { kind: 'stairs' | 'logs'; from: number; to: number }[] = [];
  let from: number | null = null;
  for (let d = 0; d <= line.length + 1e-6; d += SAMPLE) {
    const slope = slopeAt(line, d, ground);
    const steep = slope > STEEP && slope <= CLIFF;
    if (steep && from === null) from = d;
    if ((!steep || d + SAMPLE > line.length) && from !== null) {
      const to = steep ? line.length : d;
      if (to - from >= 0.3) out.push({ kind, from, to: Math.min(line.length, to + SAMPLE) });
      from = null;
    }
  }
  return out;
}

// Where nothing is laid on a path: down a cliff.
function bareOf(line: Line, ground: (x: number, z: number) => number): [number, number][] {
  const laid = laidRanges(line, ground);
  const bare: [number, number][] = [];
  let at = 0;
  for (const [a, b] of laid) {
    if (a - at > 0.2) bare.push([at, a]);
    at = b;
  }
  if (line.length - at > 0.2) bare.push([at, line.length]);
  return bare;
}

// A stretch cut into the pieces laid: its own kind, or steps, and none
// where it is bare.
function cut(from: number, to: number, path: PlannedPath): { from: number; to: number; steps?: 'stairs' | 'logs' }[] {
  const marks = new Set<number>([from, to]);
  for (const s of path.steps) for (const m of [s.from, s.to]) if (m > from && m < to) marks.add(m);
  for (const [a, b] of path.bare) for (const m of [a, b]) if (m > from && m < to) marks.add(m);
  const sorted = [...marks].sort((a, b) => a - b);
  const pieces: { from: number; to: number; steps?: 'stairs' | 'logs' }[] = [];
  for (let k = 0; k + 1 < sorted.length; k++) {
    const [a, b] = [sorted[k], sorted[k + 1]];
    const middle = (a + b) / 2;
    if (b - a < 0.3 || path.bare.some(([p, q]) => middle > p && middle < q)) continue;
    const steps = path.steps.find((s) => middle > s.from && middle < s.to)?.kind;
    pieces.push({ from: a, to: b, steps });
  }
  return pieces;
}

// The kind at a point along a path, and within BLEND of the next stretch
// how far it is blended into that one.
function kindAt(path: PlannedPath, along: number): { kind: PathKind; next: PathKind | null; blend: number } {
  const k = Math.max(0, path.stretches.findIndex((s) => along <= s.to));
  const stretch = path.stretches[k] ?? path.stretches[path.stretches.length - 1];
  const next = path.stretches[k + 1];
  if (next && next.kind !== stretch.kind && stretch.to - along < BLEND) return { kind: stretch.kind, next: next.kind, blend: (1 - (stretch.to - along) / BLEND) * 0.5 };
  const before = path.stretches[k - 1];
  if (before && before.kind !== stretch.kind && along - stretch.from < BLEND) return { kind: stretch.kind, next: before.kind, blend: (1 - (along - stretch.from) / BLEND) * 0.5 };
  return { kind: stretch.kind, next: null, blend: 0 };
}

// How much sandy dirt shows at `distance` m from a path's middle of `kind`,
// `half` m to its edge: all of it across a stone path, dirt, gravel or log
// steps; two worn bands with grass up the middle for tracks; a faint worn
// line for stepping stones in the grass; a trodden line for a trail.
function amountOf(kind: PathKind, distance: number, half: number, trail: boolean): number {
  const edge = (h: number) => smoothstep(h + 0.3, h - 0.25, distance);
  if (kind === 'tracks') {
    const band = Math.abs(distance - half * 0.5);
    return Math.max(smoothstep(half * 0.3 + 0.1, half * 0.3 - 0.12, band), 0.12 * edge(half));
  }
  if (kind === 'stepping') return (trail ? 0.3 : 0.25) * smoothstep(0.5, 0.15, distance);
  if (trail) return 0.85 * smoothstep(half + 0.2, half - 0.15, distance);
  return edge(half);
}

// A function of the distance along a path from samples every `step` m.
function sampled(samples: readonly number[], step: number): (along: number) => number {
  return (along) => {
    const u = Math.max(0, along / step);
    const k = Math.min(samples.length - 1, Math.floor(u));
    const f = Math.min(1, u - k);
    return samples[k] + (samples[Math.min(samples.length - 1, k + 1)] - samples[k]) * f;
  };
}
