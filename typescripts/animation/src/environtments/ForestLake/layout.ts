// The forest lake's map: where its lake, island, cliffs, rivers, paths and
// landmarks are, as the reference's overview lays them out, looking north
// (-z) from the south shore: the lake in the middle, fed by a tall
// waterfall from the cliffs to the north and a smaller one to the
// northwest, a rocky island with a cherry tree in it, the cave in the
// cliffs to the east with the watch tower on the heights above, the ruins
// to the west, the camp to the southwest, and the lake running out to the
// southeast down a cascade under a bridge. Meters; x east, z south; the
// lake's water at y 0.
//
// Shapes answer distance queries fast: each keeps its segments in a coarse
// grid of cells, each cell listing the segments within `reach` of it, so a
// query looks only at a few; beyond `reach` a shape only says "far".

export type Point = readonly [number, number];

// A smooth curve through the given points (Catmull-Rom), sampled about
// every `step` m along x and z, closed round to its start when `closed`.
// Points may carry more numbers after x and z (a height); they are carried
// along the curve too.
export function smooth(points: readonly (readonly number[])[], step: number, closed: boolean): number[][] {
  const out: number[][] = [];
  const n = points.length;
  const at = (i: number) => points[closed ? ((i % n) + n) % n : Math.max(0, Math.min(n - 1, i))];
  const spans = closed ? n : n - 1;
  for (let i = 0; i < spans; i++) {
    const [p0, p1, p2, p3] = [at(i - 1), at(i), at(i + 1), at(i + 2)];
    const length = Math.hypot(p2[0] - p1[0], p2[1] - p1[1]);
    const steps = Math.max(1, Math.ceil(length / step));
    for (let k = 0; k < steps; k++) {
      const t = k / steps;
      const t2 = t * t;
      const t3 = t2 * t;
      out.push(p1.map((_, c) => 0.5 * (2 * p1[c] + (-p0[c] + p2[c]) * t + (2 * p0[c] - 5 * p1[c] + 4 * p2[c] - p3[c]) * t2 + (-p0[c] + 3 * p1[c] - 3 * p2[c] + p3[c]) * t3)));
    }
  }
  if (!closed) out.push([...points[n - 1]]);
  return out;
}

// The nearest point of a set of segments.
export interface Nearest {
  distance: number;
  along: number; // how far along the line (from its start) it is
  segment: number; // which segment, -1 when none is within reach
  t: number; // how far along that segment, 0..1
  side: number; // which side of the line the query point is: 1 its left (seen from above, facing along it), -1 its right
}

class SegmentGrid {
  private readonly cells = new Map<number, number[]>();
  private readonly starts: number[] = []; // distance along the line at each segment's start
  readonly total: number;

  constructor(
    private readonly points: readonly (readonly number[])[],
    segments: number,
    private readonly reach: number,
    private readonly cell = 8,
  ) {
    let along = 0;
    for (let i = 0; i < segments; i++) {
      this.starts.push(along);
      const [a, b] = this.ends(i);
      along += Math.hypot(b[0] - a[0], b[1] - a[1]);
      const x0 = Math.floor((Math.min(a[0], b[0]) - reach) / cell);
      const x1 = Math.floor((Math.max(a[0], b[0]) + reach) / cell);
      const z0 = Math.floor((Math.min(a[1], b[1]) - reach) / cell);
      const z1 = Math.floor((Math.max(a[1], b[1]) + reach) / cell);
      for (let cx = x0; cx <= x1; cx++) {
        for (let cz = z0; cz <= z1; cz++) {
          const key = cx * 100003 + cz;
          let list = this.cells.get(key);
          if (!list) this.cells.set(key, (list = []));
          list.push(i);
        }
      }
    }
    this.total = along;
  }

  ends(i: number): [readonly number[], readonly number[]] {
    return [this.points[i], this.points[(i + 1) % this.points.length]];
  }

  nearest(x: number, z: number): Nearest {
    const list = this.cells.get(Math.floor(x / this.cell) * 100003 + Math.floor(z / this.cell));
    const found: Nearest = { distance: this.reach, along: 0, segment: -1, t: 0, side: 1 };
    if (!list) return found;
    for (const i of list) {
      const [a, b] = this.ends(i);
      const dx = b[0] - a[0];
      const dz = b[1] - a[1];
      const length2 = dx * dx + dz * dz;
      const t = length2 > 0 ? Math.max(0, Math.min(1, ((x - a[0]) * dx + (z - a[1]) * dz) / length2)) : 0;
      const d = Math.hypot(x - a[0] - dx * t, z - a[1] - dz * t);
      if (d < found.distance) {
        found.distance = d;
        found.along = this.starts[i] + t * Math.sqrt(length2);
        found.segment = i;
        found.t = t;
        // Seen from above (x right, z down the page), a point to the left
        // of the way the segment runs has a negative cross product.
        found.side = dx * (z - a[1]) - dz * (x - a[0]) < 0 ? 1 : -1;
      }
    }
    return found;
  }
}

// An open line (a path, a river, a cliff's edge), its points' extra numbers
// (a height) carried along it.
export class Line {
  readonly points: number[][];
  private readonly grid: SegmentGrid;

  constructor(points: readonly (readonly number[])[], reach: number, step = 1) {
    this.points = smooth(points, step, false);
    this.grid = new SegmentGrid(this.points, this.points.length - 1, reach);
  }

  get length(): number {
    return this.grid.total;
  }

  nearest(x: number, z: number): Nearest {
    return this.grid.nearest(x, z);
  }

  // The extra number `index` (2 for the first after x and z) at the nearest
  // point found.
  value(found: Nearest, index = 2): number {
    const i = Math.max(0, found.segment);
    const a = this.points[i][index];
    const b = this.points[Math.min(this.points.length - 1, i + 1)][index];
    return a + (b - a) * found.t;
  }

  // The point `along` m from its start, and the way the line runs there.
  at(along: number): { x: number; z: number; dx: number; dz: number } {
    let rest = Math.max(0, along);
    for (let i = 0; i + 1 < this.points.length; i++) {
      const [a, b] = [this.points[i], this.points[i + 1]];
      const length = Math.hypot(b[0] - a[0], b[1] - a[1]);
      if (rest <= length || i + 2 === this.points.length) {
        const t = length > 0 ? Math.min(1, rest / length) : 0;
        return { x: a[0] + (b[0] - a[0]) * t, z: a[1] + (b[1] - a[1]) * t, dx: (b[0] - a[0]) / (length || 1), dz: (b[1] - a[1]) / (length || 1) };
      }
      rest -= length;
    }
    const [p] = this.points;
    return { x: p[0], z: p[1], dx: 1, dz: 0 };
  }
}

// A closed outline (the lake's shore, the island's). Its signed distance is
// negative inside. Whether a point is inside is looked up in a raster of
// it filled once, 0.25 m to a cell.
export class Outline {
  readonly points: number[][];
  private readonly grid: SegmentGrid;
  private readonly raster: Uint8Array;
  private readonly x0: number;
  private readonly z0: number;
  private readonly columns: number;
  private readonly rows: number;
  private static readonly CELL = 0.25;

  constructor(points: readonly Point[], reach: number, step = 1) {
    this.points = smooth(points, step, true);
    this.grid = new SegmentGrid(this.points, this.points.length, reach);
    const xs = this.points.map((p) => p[0]);
    const zs = this.points.map((p) => p[1]);
    const c = Outline.CELL;
    this.x0 = Math.min(...xs) - c;
    this.z0 = Math.min(...zs) - c;
    this.columns = Math.ceil((Math.max(...xs) + c - this.x0) / c) + 1;
    this.rows = Math.ceil((Math.max(...zs) + c - this.z0) / c) + 1;
    this.raster = new Uint8Array(this.columns * this.rows);
    // Each row filled between the outline's crossings of it.
    for (let r = 0; r < this.rows; r++) {
      const z = this.z0 + (r + 0.5) * c;
      const crossings: number[] = [];
      for (let i = 0; i < this.points.length; i++) {
        const [a, b] = [this.points[i], this.points[(i + 1) % this.points.length]];
        if (a[1] <= z !== b[1] <= z) crossings.push(a[0] + ((z - a[1]) / (b[1] - a[1])) * (b[0] - a[0]));
      }
      crossings.sort((p, q) => p - q);
      for (let k = 0; k + 1 < crossings.length; k += 2) {
        const from = Math.max(0, Math.ceil((crossings[k] - this.x0) / c - 0.5));
        const to = Math.min(this.columns - 1, Math.floor((crossings[k + 1] - this.x0) / c - 0.5));
        for (let col = from; col <= to; col++) this.raster[r * this.columns + col] = 1;
      }
    }
  }

  inside(x: number, z: number): boolean {
    const col = Math.floor((x - this.x0) / Outline.CELL);
    const row = Math.floor((z - this.z0) / Outline.CELL);
    if (col < 0 || row < 0 || col >= this.columns || row >= this.rows) return false;
    return this.raster[row * this.columns + col] === 1;
  }

  // Signed distance, negative inside; at most `reach` either way.
  distance(x: number, z: number): number {
    const d = this.grid.nearest(x, z).distance;
    return this.inside(x, z) ? -d : d;
  }
}

// ---------------------------------------------------------------------------
// The map.

// The lake's shore, round from the northwest bay under the small waterfall,
// east along the foot of the cliffs (reaching them under the main
// waterfall), down the east shore to the outlet in the southeast, and back
// west along the south shore.
export const LAKE: Point[] = [
  [-20, -14],
  [-19.4, -20.5],
  [-17.2, -23.7],
  [-12, -24.4],
  [-4.5, -25.6],
  [0.2, -28.6],
  [3, -29.9],
  [6, -28.6],
  [10, -24.5],
  [15, -18],
  [18, -10],
  [19.5, -2],
  [18.6, 5.5],
  [16.5, 10.5],
  [13.5, 13.2],
  [8, 12.2],
  [2, 11.4],
  [-4.5, 11],
  [-11, 8.5],
  [-16, 3.8],
  [-19.2, -3],
  [-20.3, -10],
];

// The island, off the middle of the lake toward its east shore.
export const ISLAND: Point[] = [
  [6.8, -8],
  [10.2, -6.8],
  [11.4, -3.2],
  [10.2, 0.6],
  [6.6, 1.9],
  [3.2, 0.7],
  [1.8, -2.8],
  [3.4, -6.6],
];
export const ISLAND_TOP = 1.2; // m, the island's grass over the water, on its plinth of rock

// The edge of the heights round the north and east of the lake: the top of
// the cliffs, from where they rise out of the western hills, round behind
// both waterfalls and the watch tower, down the east side past the cave, to
// where they sink into the land by the outlet. The heights lie on its left
// (it runs clockwise round the lake); each point's third number is the
// heights' level there, falling to the land's at both ends.
export const RIM: readonly (readonly [number, number, number])[] = [
  [-42, -15, 0.6],
  [-33, -17.5, 3.4],
  [-26, -20.2, 5.6],
  [-18.8, -24.6, 6.2],
  [-12, -27.2, 7.4],
  [-4, -29.8, 9.6],
  [3, -31, 10],
  [9.5, -29.8, 10.2],
  [14.5, -26.2, 10.6],
  [19, -20.5, 11],
  [22, -13.5, 10],
  [23.9, -6.5, 8],
  [24.7, 0.5, 7],
  [24.3, 6.5, 6],
  [23.6, 10.8, 2.6],
  [23.2, 15.5, 0.6],
];
// How far the heights rise further inland: m per m in from the rim, at most
// INLAND_MOST.
export const INLAND = 0.02;
export const INLAND_MOST = 2.2;

// The rivers over the heights, each from upstream down to its lip, where it
// falls into the lake, with its water's level at its lip, and how much it
// rises per m upstream.
export const MAIN_RIVER: Point[] = [
  [1, -80],
  [-2.5, -64],
  [1.5, -50],
  [4.2, -41],
  [2.6, -35],
  [3, -31.4],
];
// Where it falls: its last point, just inside the rim.
export const MAIN_LIP = { x: 3, z: -31.4, level: 9.6, width: 4.2, rise: 0.012 };
export const SMALL_RIVER: Point[] = [
  [-33, -58],
  [-29, -44],
  [-24, -34],
  [-19.8, -28.4],
  [-18.0, -25.4],
];
export const SMALL_LIP = { x: -18.0, z: -25.4, level: 5.8, width: 2.6, rise: 0.012 };

// The stream the lake runs out by, from the outlet down the cascade under
// the bridge and away southeast, with its water's level along it: the
// lake's (0) until the cascade, 1.9 m lower after it, then falling gently.
export const STREAM: Point[] = [
  [14.2, 11.6],
  [16.5, 14.6],
  [18.4, 17.6],
  [20, 20.6],
  [22.2, 25],
  [25, 31],
  [28.5, 38],
  [33, 46],
  [38.5, 55],
  [45, 66],
  [53, 80],
  [62, 96],
];
export const CASCADE = { from: 4.9, to: 6.9, drop: 1.9, fall: 0.012 }; // m along the stream; `fall` m per m after it

// The paths: the forest path from the camp along the south shore, past the
// landing, over the bridge and north to the cave; a branch from the camp
// north to the ruins, with a spur to the pier; one along the heights from
// the watch tower toward the cliffs over the cave; and the way in from the
// south, through the forest entrance's gate, joining the forest path east
// of the landing.
export const PATHS: { points: Point[]; width: number }[] = [
  {
    width: 2.3,
    points: [
      [-19.8, 17.2],
      [-15.5, 17.4],
      [-8, 17.6],
      [-1.5, 18.3],
      [5, 17.4],
      [10.5, 16.2],
      [13.8, 15.35],
    ],
  },
  {
    width: 2.1,
    points: [
      [20.5, 12.45],
      [23, 8.6],
      [22.2, 4.2],
      [21, 0.2],
      [20.9, -3.3],
    ],
  },
  {
    width: 1.9,
    points: [
      [-24.4, 11.4],
      [-24.2, 5],
      [-25.6, -0.5],
      [-27.2, -3.8],
    ],
  },
  {
    width: 1.6,
    points: [
      [-24.3, 7.2],
      [-19.5, 7.9],
      [-15, 9.1],
    ],
  },
  {
    width: 1.8,
    points: [
      [22.6, -23.2],
      [26.2, -18.5],
      [28.4, -11],
      [28.4, -5],
    ],
  },
  {
    width: 2.1,
    points: [
      [5.6, 17.3],
      [7.2, 22],
      [8.3, 27],
      [8.6, 33],
      [8, 40],
      [6.8, 48],
    ],
  },
];

// Where figures stand: the south shore, a clearing between the path and
// the water, looking north across the lake to the island and the waterfall.
export const LANDING: Point = [-2, 14.4];
export const CLEARING = 3.4; // m round it kept level and clear

// Where the landmarks stand. A heading is the way something runs out (a
// pier over the water), in radians from +x toward +z.
export const SPOTS = {
  pier: { x: -12.8, z: 8.5, heading: -0.93, length: 7 },
  eastPier: { x: 19.8, z: 2.2, heading: Math.PI, length: 5.5 },
  jetty: { x: 6.4, z: 2.3, heading: Math.PI / 2, length: 3 }, // from the island's steps, just off its plinth
  boat: { x: -11.8, z: 4.0, heading: -0.93 },
  bridge: { from: [13.8, 15.35] as Point, to: [20.5, 12.45] as Point },
  footbridge: { x: 3, z: -34.2 }, // over the main river above its lip
  tower: { x: 21.2, z: -25.4 },
  cave: { x: 22.7, z: -3.3 }, // its mouth, facing west
  ruins: { x: -31, z: -4.5 },
  camp: { x: -24.4, z: 16.4, radius: 5.4 },
  tent: { x: -27.6, z: 17.6 },
  campfire: { x: -23.6, z: 15.2 },
  gate: { path: 5, along: 8.2 }, // the forest entrance's gate, on the way in from the south
} as const;

// How much bigger than their assets' own size the cave's mouth and the
// ruins stand here, as big as the reference's overview draws them.
export const CAVE_SCALE = 1.3;
export const RUINS_SCALE = 1.25;
