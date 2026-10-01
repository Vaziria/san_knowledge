import * as THREE from 'three';
import type { ForestLake } from '../../environtments/ForestLake/ForestLake';
import { finish, type Stepwise } from '../../stepwise';
import type { Seeing } from '../lake_meeting/Sight';

// What a camera at the forest lake meeting must keep out of, and what it
// can't see through (the lake meeting's Follow.ts and HostView.ts, and the
// turns' close-ups), measured once from the forest lake's own meshes over a
// square HALF m each way round the meeting's middle, as the lake meeting's
// Sight.ts does its lake; for the next season, a piece at a time while the
// meeting plays (build()):
//
// - The floor: the ground mesh's own height, sampled on a FLOOR m grid from
//   its triangles, or the water's now (the lake's waves, its ice in winter,
//   the rivers and the stream), whichever is higher, and WAVES m more over
//   the water.
// - What is solid (Landmarks.solid, Scatter.solid, the cliffs): the cubes,
//   CELL m, that their surfaces pass through, from DOWN m under the ground
//   to UP m over it, kept as bits. A camera in a marked cube, or a line of
//   sight through one, is blocked: a trunk, a crown, a bush, a rock, a log,
//   the tent, a fence, a pier, the cave's rock, a cliff. A rock under PEBBLE
//   across (a path's pebbles) is neither. Outside the square nothing is
//   known, so a camera may not be there, nor see through it: filming the
//   dragon in the meadow at the square's south edge, the quiet camera stood
//   in a trunk past it.
//
// Positions are in the meeting's coordinates; `offset` is where its origin
// is in the lake's.

const CELL = 0.25; // m, the cubes of what is solid
const DOWN = 1; // m under the ground at a column's middle its lowest cube is
const UP = 28; // m over it its highest: the island's great crystal tree (22 m, task 21), its crown out over the lake's bed too, the tallest pines, and the cliffs over their feet
const FLOOR = 0.25; // m between the floor's samples
const WATER = 0.5; // m between the samples of the rivers' and the stream's water
const WAVES = 0.08; // m a camera keeps over the water's height now
const STEP = 0.1; // m between the points a line of sight is checked at, at most
const OVER = 0.005; // m a line of sight must pass over the ground or the water
export const PEBBLE = 0.3; // m across: a rock smaller than this is neither solid nor in the way
// Measured a piece at a time (build()): it stops after this many triangles,
// a skipped copy counting as COPY of them, or this many rows of a grid.
const TRIANGLES = 4000;
const COPY = 4;
const ROWS = 16;

export interface ForestSightOptions {
  lake: ForestLake;
  offset: THREE.Vector3; // where the meeting's origin is in the lake's coordinates
  middle: THREE.Vector2; // the middle of the square it measures, in the lake's coordinates
  half: number; // m from the middle to the square's sides
}

// A square of the lake: its lowest corner and how far it reaches.
export interface Square {
  x0: number;
  z0: number;
  size: number; // m across
}

export class ForestSight implements Seeing {
  private readonly lake: ForestLake;
  private readonly offset: THREE.Vector3;
  private readonly square: Square;
  private readonly columns: number; // cubes across
  private readonly layers = Math.ceil((DOWN + UP) / CELL); // cubes up
  private readonly base: Float32Array; // each column's lowest cube's bottom
  private readonly solid: Uint32Array; // a bit for each cube
  private readonly points: number; // floor samples across
  private readonly heights: Float32Array; // the ground mesh's height at each sample, NaN off it
  private readonly waterPoints: number;
  private readonly waters: Float32Array; // the rivers' and the stream's level at each sample, NaN where there is none

  // Measured at once; or, `later`, only once measure() has run to its end.
  constructor({ lake, offset, middle, half }: ForestSightOptions, later = false) {
    this.lake = lake;
    this.offset = offset.clone();
    const square = (this.square = { x0: middle.x - half, z0: middle.y - half, size: 2 * half });
    const n = (this.points = Math.round(square.size / FLOOR) + 1);
    this.heights = new Float32Array(n * n).fill(NaN);
    const w = (this.waterPoints = Math.round(square.size / WATER) + 1);
    this.waters = new Float32Array(w * w);
    const columns = (this.columns = Math.ceil(square.size / CELL));
    this.base = new Float32Array(columns * columns);
    this.solid = new Uint32Array(Math.ceil((columns * columns * this.layers) / 32));
    if (!later) finish(this.measure());
  }

  // Measured a piece at a time, each yield a place to stop and draw a frame:
  // the forest lake meeting measures the next season's sight while it plays.
  static *build(options: ForestSightOptions): Stepwise<ForestSight> {
    const sight = new ForestSight(options, true);
    yield* sight.measure();
    return sight;
  }

  private *measure(): Stepwise<void> {
    const { lake, square } = this;
    const terrain = lake.terrain;

    // In the lake's own coordinates, wherever the stage has moved it.
    lake.updateMatrixWorld(true);
    const toLake = lake.matrixWorld.clone().invert();

    // The floor.
    yield* triangles(lake.ground, toLake, square, 0, (a, b, c) => this.floorUnder(a, b, c));
    const w = this.waterPoints;
    for (let j = 0; j < w; j++) {
      for (let i = 0; i < w; i++) {
        const x = square.x0 + i * WATER;
        const z = square.z0 + j * WATER;
        this.waters[j * w + i] = terrain.lake.inside(x, z) ? NaN : terrain.waterAt(x, z);
      }
      if (j % ROWS === 0) yield;
    }

    // What is solid.
    const columns = this.columns;
    for (let j = 0; j < columns; j++) {
      for (let i = 0; i < columns; i++) this.base[j * columns + i] = terrain.heightAt(square.x0 + (i + 0.5) * CELL, square.z0 + (j + 0.5) * CELL) - DOWN;
      if (j % ROWS === 0) yield;
    }
    for (const root of [...lake.landmarks.solid, ...lake.scatter.solid, lake.cliffs]) yield* triangles(root, toLake, square, PEBBLE, (a, b, c) => this.markSolid(a, b, c));
  }

  // The height a camera must keep over, at a point: the ground, or the water
  // now and WAVES m more.
  floor(x: number, z: number): number {
    return this.top(x, z, WAVES);
  }

  // Whether a point is in something solid, or outside the square, where
  // nothing is known.
  blocked(point: THREE.Vector3): boolean {
    const x = point.x + this.offset.x;
    const z = point.z + this.offset.z;
    const i = Math.floor((x - this.square.x0) / CELL);
    const j = Math.floor((z - this.square.z0) / CELL);
    if (i < 0 || j < 0 || i >= this.columns || j >= this.columns) return true;
    const column = j * this.columns + i;
    const k = Math.floor((point.y + this.offset.y - this.base[column]) / CELL);
    if (k < 0 || k >= this.layers) return false;
    const bit = column * this.layers + k;
    return (this.solid[bit >>> 5] & (1 << (bit & 31))) !== 0;
  }

  // Whether nothing stands between two points: nothing solid, and neither the
  // ground nor the water in the way. The first `skip` m from `from` aren't
  // checked (the animal looked at stands there).
  sees(from: THREE.Vector3, to: THREE.Vector3, skip = 0): boolean {
    const length = from.distanceTo(to);
    const steps = Math.max(2, Math.ceil(length / STEP));
    const point = new THREE.Vector3();
    for (let s = 1; s < steps; s++) {
      if ((s / steps) * length < skip) continue;
      point.lerpVectors(from, to, s / steps);
      if (point.y < this.top(point.x, point.z, 0) + OVER || this.blocked(point)) return false;
    }
    return true;
  }

  // The top of the ground or of the water now at a point, with `waves` m more
  // over the water.
  private top(x: number, z: number, waves: number): number {
    const lx = x + this.offset.x;
    const lz = z + this.offset.z;
    const terrain = this.lake.terrain;
    let y = Math.max(this.meshHeight(lx, lz), terrain.heightAt(lx, lz));
    if (terrain.lake.inside(lx, lz)) y = Math.max(y, this.lake.surfaceAt(lx, lz) + waves);
    else {
      const water = this.riverAt(lx, lz);
      if (!Number.isNaN(water)) y = Math.max(y, water + waves);
    }
    return y - this.offset.y;
  }

  // The rivers' or the stream's level near a point: the highest of the
  // samples round it, so a camera by the bank keeps over the water.
  private riverAt(x: number, z: number): number {
    const w = this.waterPoints;
    const i = Math.floor((x - this.square.x0) / WATER);
    const j = Math.floor((z - this.square.z0) / WATER);
    let most = NaN;
    for (const [di, dj] of [
      [0, 0],
      [1, 0],
      [0, 1],
      [1, 1],
    ]) {
      const ni = i + di;
      const nj = j + dj;
      if (ni < 0 || nj < 0 || ni >= w || nj >= w) continue;
      const level = this.waters[nj * w + ni];
      if (!Number.isNaN(level) && !(level <= most)) most = level;
    }
    return most;
  }

  // The ground mesh's height at a point of the lake, between its samples, or
  // -Infinity off them.
  private meshHeight(x: number, z: number): number {
    const u = (x - this.square.x0) / FLOOR;
    const v = (z - this.square.z0) / FLOOR;
    const i = Math.floor(u);
    const j = Math.floor(v);
    const n = this.points;
    if (i < 0 || j < 0 || i >= n - 1 || j >= n - 1) return -Infinity;
    const h = this.heights;
    const a = h[j * n + i];
    const b = h[j * n + i + 1];
    const c = h[(j + 1) * n + i];
    const d = h[(j + 1) * n + i + 1];
    if (Number.isNaN(a + b + c + d)) return -Infinity;
    const fu = u - i;
    const fv = v - j;
    return (a * (1 - fu) + b * fu) * (1 - fv) + (c * (1 - fu) + d * fu) * fv;
  }

  // The floor's samples under a triangle of the ground mesh, at its height
  // there.
  private floorUnder(a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3): void {
    const { points: n, square } = this;
    const area = (b.x - a.x) * (c.z - a.z) - (c.x - a.x) * (b.z - a.z);
    if (Math.abs(area) < 1e-12) return;
    const i0 = Math.max(0, Math.ceil((Math.min(a.x, b.x, c.x) - square.x0) / FLOOR));
    const i1 = Math.min(n - 1, Math.floor((Math.max(a.x, b.x, c.x) - square.x0) / FLOOR));
    const j0 = Math.max(0, Math.ceil((Math.min(a.z, b.z, c.z) - square.z0) / FLOOR));
    const j1 = Math.min(n - 1, Math.floor((Math.max(a.z, b.z, c.z) - square.z0) / FLOOR));
    for (let j = j0; j <= j1; j++) {
      const z = square.z0 + j * FLOOR;
      for (let i = i0; i <= i1; i++) {
        const x = square.x0 + i * FLOOR;
        const wb = ((x - a.x) * (c.z - a.z) - (c.x - a.x) * (z - a.z)) / area;
        const wc = ((b.x - a.x) * (z - a.z) - (x - a.x) * (b.z - a.z)) / area;
        const wa = 1 - wb - wc;
        if (wa < -1e-6 || wb < -1e-6 || wc < -1e-6) continue;
        const y = wa * a.y + wb * b.y + wc * c.y;
        const k = j * n + i;
        if (!(this.heights[k] >= y)) this.heights[k] = y;
      }
    }
  }

  // Marks the cubes a triangle passes through, sampled every half cube.
  private markSolid(a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3): void {
    const longest = Math.max(a.distanceTo(b), b.distanceTo(c), c.distanceTo(a));
    const n = Math.max(1, Math.ceil(longest / (CELL / 2)));
    const { columns, layers, square } = this;
    for (let p = 0; p <= n; p++) {
      for (let q = 0; q <= n - p; q++) {
        const u = p / n;
        const v = q / n;
        const i = Math.floor((a.x + (b.x - a.x) * u + (c.x - a.x) * v - square.x0) / CELL);
        const j = Math.floor((a.z + (b.z - a.z) * u + (c.z - a.z) * v - square.z0) / CELL);
        if (i < 0 || j < 0 || i >= columns || j >= columns) continue;
        const column = j * columns + i;
        const k = Math.floor((a.y + (b.y - a.y) * u + (c.y - a.y) * v - this.base[column]) / CELL);
        if (k < 0 || k >= layers) continue;
        const bit = column * layers + k;
        this.solid[bit >>> 5] |= 1 << (bit & 31);
      }
    }
  }
}

// Every triangle of the meshes under a root that may reach into a square of
// the lake, in the coordinates `toFrame` takes the world's to: an instanced
// mesh's once for each copy, but for copies wholly outside the square or
// smaller than `least` m across (a pebble).
export function trianglesIn(root: THREE.Object3D, toFrame: THREE.Matrix4, square: Square, least: number, visit: (a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3) => void): void {
  finish(triangles(root, toFrame, square, least, visit));
}

// The same, a piece at a time: it yields every TRIANGLES triangles looked
// at, a copy it skips counting as COPY of them.
export function* triangles(root: THREE.Object3D, toFrame: THREE.Matrix4, square: Square, least: number, visit: (a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3) => void): Stepwise<void> {
  const a = new THREE.Vector3();
  const b = new THREE.Vector3();
  const c = new THREE.Vector3();
  const matrix = new THREE.Matrix4();
  const instance = new THREE.Matrix4();
  const sphere = new THREE.Sphere();
  const box = new THREE.Box3();
  const [x0, z0, x1, z1] = [square.x0, square.z0, square.x0 + square.size, square.z0 + square.size];
  const meshes: THREE.Mesh[] = [];
  root.updateMatrixWorld(true);
  root.traverse((object) => {
    if (object instanceof THREE.Mesh) meshes.push(object);
  });
  let looked = 0;
  for (const object of meshes) {
    const geometry: THREE.BufferGeometry = object.geometry;
    const position = geometry.getAttribute('position');
    if (!position) continue;
    if (!geometry.boundingSphere) geometry.computeBoundingSphere();
    if (!geometry.boundingBox) geometry.computeBoundingBox();
    const index = geometry.getIndex();
    const count = index ? index.count : position.count;
    const instanced = object instanceof THREE.InstancedMesh;
    for (let n = 0; n < (instanced ? object.count : 1); n++) {
      if (looked >= TRIANGLES) {
        looked = 0;
        yield;
      }
      matrix.multiplyMatrices(toFrame, object.matrixWorld);
      if (instanced) {
        object.getMatrixAt(n, instance);
        matrix.multiply(instance);
        sphere.copy(geometry.boundingSphere!).applyMatrix4(matrix);
        looked += COPY;
        if (2 * sphere.radius < least) continue;
        const r = sphere.radius;
        if (sphere.center.x + r < x0 || sphere.center.x - r > x1 || sphere.center.z + r < z0 || sphere.center.z - r > z1) continue;
      } else {
        box.copy(geometry.boundingBox!).applyMatrix4(matrix);
        if (box.max.x < x0 || box.min.x > x1 || box.max.z < z0 || box.min.z > z1) continue;
      }
      for (let t = 0; t + 2 < count; t += 3) {
        if (++looked >= TRIANGLES) {
          looked = 0;
          yield;
        }
        a.fromBufferAttribute(position, index ? index.getX(t) : t).applyMatrix4(matrix);
        b.fromBufferAttribute(position, index ? index.getX(t + 1) : t + 1).applyMatrix4(matrix);
        c.fromBufferAttribute(position, index ? index.getX(t + 2) : t + 2).applyMatrix4(matrix);
        if (Math.max(a.x, b.x, c.x) < x0 || Math.min(a.x, b.x, c.x) > x1 || Math.max(a.z, b.z, c.z) < z0 || Math.min(a.z, b.z, c.z) > z1) continue;
        visit(a, b, c);
      }
    }
  }
}
