import * as THREE from 'three';
import { LAND_RADIUS } from '../../environtments/Lake/Ground';
import type { Lake } from '../../environtments/Lake/Lake';

// What a camera at the lake meeting must keep out of, and what it can't see
// through (Follow.ts): the lake's ground and water, and what stands on its
// land, measured once from the lake's own meshes.
//
// - The floor: the ground mesh's own height, sampled on a FLOOR m grid from
//   its triangles, or the water's height now, whichever is higher. Between
//   its points the mesh is flat, and lies up to 1.4 cm off groundAt()'s curve
//   near the water and up to 7 cm out among the hills, so a camera kept just
//   over the curve could be under the mesh. Over the water a camera keeps
//   WAVES m more, since the waves rise and fall while it follows.
// - The trees and boulders: a grid of CELL m cubes up to TOP m, each marked if
//   one of their surfaces passes through it (their triangles sampled every
//   half cube). A line of sight through a marked cube is blocked, whether by
//   a trunk, a crown or a boulder, and so is a camera in one. Raycasting the
//   trees' 667 000 triangles took about a millisecond a ray, and 10–20 ms for
//   one through a crown: too slow to check every frame.
// - The stones: each an upright cylinder round its bounds, since a quarter-
//   meter cube round a pebble would stand in the way of a frog's close-up.
// - The logs floating in the water: each its box, where it has drifted to
//   now (Logs.contains()).
//
// Positions are in the meeting's coordinates; `offset` is where its origin is
// in the lake's.

const FLOOR = 0.2; // m between the floor's samples
const CELL = 0.25; // m, the cubes of what is solid
const BOTTOM = -1; // m, the lowest cube, from the still water
const TOP = 9; // and the highest
const WAVES = 0.08; // m a camera keeps over the water's height now
const BUCKET = 1; // m, the squares the stones are sorted into
const STEP = 0.1; // m between the points a line of sight is checked at, at most
const OVER = 0.005; // m a line of sight must pass over the ground or the water

interface Rock {
  x: number;
  z: number;
  radius: number;
  bottom: number;
  top: number;
}

export interface SightOptions {
  lake: Lake | null;
  offset: THREE.Vector3; // where the meeting's origin is in the lake's coordinates
  ground(x: number, z: number): number; // without a lake, the floor, in the meeting's coordinates
}

export class Sight {
  private readonly offset: THREE.Vector3;
  private readonly lake: Lake | null;
  private readonly ground: (x: number, z: number) => number;
  private readonly half = LAND_RADIUS; // m from the lake's middle the grids reach
  private readonly points = Math.round((2 * LAND_RADIUS) / FLOOR) + 1; // floor samples across
  private readonly heights: Float32Array; // the ground mesh's height at each sample, NaN off it
  private readonly size = Math.ceil((2 * LAND_RADIUS) / CELL); // cubes across
  private readonly layers = Math.ceil((TOP - BOTTOM) / CELL); // cubes up
  private readonly solid: Uint8Array;
  private readonly rocks = new Map<number, Rock[]>(); // by bucket

  constructor({ lake, offset, ground }: SightOptions) {
    this.offset = offset.clone();
    this.lake = lake;
    this.ground = ground;
    this.heights = new Float32Array(lake ? this.points * this.points : 0).fill(NaN);
    this.solid = new Uint8Array(lake ? this.size * this.size * this.layers : 0);
    if (!lake) return;

    // In the lake's own coordinates, wherever the stage has moved it.
    lake.updateMatrixWorld(true);
    const toLake = lake.matrixWorld.clone().invert();
    eachTriangle(lake.ground, toLake, (a, b, c) => this.floorUnder(a, b, c));
    // The dock too (task 09): no camera in its deck or posts, nor seeing through them.
    for (const group of [lake.trees, lake.boulders, ...(lake.dock ? [lake.dock] : [])]) eachTriangle(group, toLake, (a, b, c) => this.markSolid(a, b, c));
    const matrix = new THREE.Matrix4();
    const box = new THREE.Box3();
    const at = new THREE.Vector3();
    const size = new THREE.Vector3();
    lake.stones.traverse((object) => {
      if (!(object instanceof THREE.InstancedMesh)) return;
      const geometry = object.geometry;
      if (!geometry.boundingBox) geometry.computeBoundingBox();
      for (let i = 0; i < object.count; i++) {
        object.getMatrixAt(i, matrix);
        box.copy(geometry.boundingBox!).applyMatrix4(matrix.premultiply(object.matrixWorld).premultiply(toLake));
        box.getCenter(at);
        box.getSize(size);
        const rock = { x: at.x, z: at.z, radius: Math.max(size.x, size.z) / 2, bottom: box.min.y, top: box.max.y };
        const bucket = key(Math.floor(at.x / BUCKET), Math.floor(at.z / BUCKET));
        const list = this.rocks.get(bucket);
        if (list) list.push(rock);
        else this.rocks.set(bucket, [rock]);
      }
    });
  }

  // The height a camera must keep over, at a point: the ground, or the water
  // now and WAVES m more.
  floor(x: number, z: number): number {
    return this.top(x, z, WAVES);
  }

  // Whether a point is in something solid: a tree, a boulder, a stone, the
  // dock or a floating log.
  blocked(point: THREE.Vector3): boolean {
    if (!this.lake) return false;
    const x = point.x + this.offset.x;
    const y = point.y + this.offset.y;
    const z = point.z + this.offset.z;
    const i = Math.floor((x + this.half) / CELL);
    const j = Math.floor((z + this.half) / CELL);
    const k = Math.floor((y - BOTTOM) / CELL);
    if (i >= 0 && j >= 0 && k >= 0 && i < this.size && j < this.size && k < this.layers && this.solid[(k * this.size + j) * this.size + i]) return true;
    const bi = Math.floor(x / BUCKET);
    const bj = Math.floor(z / BUCKET);
    for (let dj = -1; dj <= 1; dj++) {
      for (let di = -1; di <= 1; di++) {
        for (const rock of this.rocks.get(key(bi + di, bj + dj)) ?? []) {
          if (y >= rock.bottom && y <= rock.top && Math.hypot(x - rock.x, z - rock.z) < rock.radius) return true;
        }
      }
    }
    // The floating logs (task 11), where they have drifted to now.
    return this.lake.logs?.contains(x, y, z) ?? false;
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
    const lake = this.lake;
    if (!lake) return this.ground(x, z);
    const lx = x + this.offset.x;
    const lz = z + this.offset.z;
    const bed = lake.groundAt(lx, lz);
    let y = Math.max(this.meshHeight(lx, lz), bed);
    if (bed < 0) y = Math.max(y, lake.surfaceAt(lx, lz) + waves);
    return y - this.offset.y;
  }

  // The ground mesh's height at a point of the lake, between its samples, or
  // -Infinity off it.
  private meshHeight(x: number, z: number): number {
    const u = (x + this.half) / FLOOR;
    const v = (z + this.half) / FLOOR;
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
    const { points: n, half } = this;
    const area = (b.x - a.x) * (c.z - a.z) - (c.x - a.x) * (b.z - a.z);
    if (Math.abs(area) < 1e-12) return;
    const i0 = Math.max(0, Math.ceil((Math.min(a.x, b.x, c.x) + half) / FLOOR));
    const i1 = Math.min(n - 1, Math.floor((Math.max(a.x, b.x, c.x) + half) / FLOOR));
    const j0 = Math.max(0, Math.ceil((Math.min(a.z, b.z, c.z) + half) / FLOOR));
    const j1 = Math.min(n - 1, Math.floor((Math.max(a.z, b.z, c.z) + half) / FLOOR));
    for (let j = j0; j <= j1; j++) {
      const z = j * FLOOR - half;
      for (let i = i0; i <= i1; i++) {
        const x = i * FLOOR - half;
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
    if (Math.max(a.y, b.y, c.y) < BOTTOM || Math.min(a.y, b.y, c.y) > TOP) return;
    const longest = Math.max(a.distanceTo(b), b.distanceTo(c), c.distanceTo(a));
    const n = Math.max(1, Math.ceil(longest / (CELL / 2)));
    const { size, layers, half } = this;
    for (let p = 0; p <= n; p++) {
      for (let q = 0; q <= n - p; q++) {
        const u = p / n;
        const v = q / n;
        const i = Math.floor((a.x + (b.x - a.x) * u + (c.x - a.x) * v + half) / CELL);
        const j = Math.floor((a.z + (b.z - a.z) * u + (c.z - a.z) * v + half) / CELL);
        const k = Math.floor((a.y + (b.y - a.y) * u + (c.y - a.y) * v - BOTTOM) / CELL);
        if (i < 0 || j < 0 || k < 0 || i >= size || j >= size || k >= layers) continue;
        this.solid[(k * size + j) * size + i] = 1;
      }
    }
  }
}

function key(i: number, j: number): number {
  return (i + 1000) * 2001 + (j + 1000);
}

// Every triangle of the meshes under a root, in the coordinates `toFrame`
// takes the world's to (an instanced mesh's once for each instance).
function eachTriangle(root: THREE.Object3D, toFrame: THREE.Matrix4, visit: (a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3) => void): void {
  const a = new THREE.Vector3();
  const b = new THREE.Vector3();
  const c = new THREE.Vector3();
  const matrix = new THREE.Matrix4();
  const instance = new THREE.Matrix4();
  root.traverse((object) => {
    if (!(object instanceof THREE.Mesh)) return;
    const geometry: THREE.BufferGeometry = object.geometry;
    const position = geometry.getAttribute('position');
    if (!position) return;
    const index = geometry.getIndex();
    const count = index ? index.count : position.count;
    const instanced = object instanceof THREE.InstancedMesh;
    for (let n = 0; n < (instanced ? object.count : 1); n++) {
      matrix.multiplyMatrices(toFrame, object.matrixWorld);
      if (instanced) {
        object.getMatrixAt(n, instance);
        matrix.multiply(instance);
      }
      for (let t = 0; t + 2 < count; t += 3) {
        a.fromBufferAttribute(position, index ? index.getX(t) : t).applyMatrix4(matrix);
        b.fromBufferAttribute(position, index ? index.getX(t + 1) : t + 1).applyMatrix4(matrix);
        c.fromBufferAttribute(position, index ? index.getX(t + 2) : t + 2).applyMatrix4(matrix);
        visit(a, b, c);
      }
    }
  });
}
