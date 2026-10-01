import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { addTube, between, color, matte, mesh, noise2, seededRandom, shade, Shape, vary, type Season } from '../../figures/ForestLake/parts';
import { FIRST, flat, Flights, FLAGS, CHIPS, groundUp, planted, plantGeometries, stepsOf, stoneGeometries, strip, type Laid, type Planted, type Step } from '../../figures/ForestLake/StonePath/parts';
import { Stones } from '../../figures/ForestLake/StonePath/Stones';
import type { PathPlan } from './Paths';

// What lies along the forest lake's paths besides the stone path (task 20;
// Paths.ts): the stretches of other kinds, and what is found along the way.
// - Stepping stones: single flagstones in the grass, a stride apart.
// - Gravel: pebbles packed in the dirt, loose ones spilling onto the grass.
// - Log steps, on the steep stretches of the narrow paths: a log across the
//   path at each step, the earth level behind it; their treads are ground.
// - Two worn tracks: grass up the middle between them.
// - Along the way: roots across the forest stretches, puddles in dips (ice
//   in winter), moss along the shaded edges, a few stones kicked onto the
//   grass, the grass reaching in over the edges; in autumn leaves drifted
//   along the edges; in winter a trodden track down the middle of the snow,
//   with footprints.
// Everything but the log steps lies at most 5 cm over the ground, so the
// forest lake meeting's animals walk over it. Drawn instanced; the stones
// are the stone path's shapes (StonePath/parts.ts).

const STRIDE = 0.62; // m between stepping stones
const GRAVEL = 26; // pebbles to a square meter of gravel
const SPILL = 0.45; // m past a gravel path's edge its loose pebbles reach
const ROOTS = 2.6; // m between roots across a forest stretch, about
const KICKED = 5; // m of path to each stone kicked onto the grass
const STEP_DEPTH_LOG = 0.085; // m, a log step's log's radius
const PUDDLE = 0.015; // m a dip along a path's middle is under the ground 1.5 m either way for a puddle to lie in it

// The ground the log steps' earth is made to match (ForestLake): its
// height, its colour as meshed, and its material.
export interface WaysideGround {
  heightAt(x: number, z: number): number;
  colorAt(x: number, z: number, target: THREE.Color): THREE.Color;
  material: THREE.Material;
}

export class Wayside extends THREE.Group {
  readonly flights: Flights; // the log steps
  readonly puddles: number;

  constructor(plan: PathPlan, season: Season, light: (x: number, z: number) => number, ground: WaysideGround, seed = 23) {
    super();
    this.name = 'wayside';
    const random = seededRandom(seed * 7907 + 5);
    const winter = season === 'winter';
    const autumn = season === 'autumn';
    const laid: Laid[] = [];
    const plants = plantGeometries(season, seed);
    const plantShapes = [...plants.small, ...plants.tall, ...plants.moss, ...plants.leaves];
    const first = { small: 0, tall: plants.small.length, moss: plants.small.length + plants.tall.length, leaves: plants.small.length + plants.tall.length + plants.moss.length };
    const copies: Planted[] = [];
    const flights: Step[][] = [];
    const logs: { step: Step; width: number }[] = [];
    const tracks: THREE.BufferGeometry[] = [];
    const prints: Planted[] = [];
    const roots = new Shape();
    const puddles: THREE.BufferGeometry[] = [];

    for (const piece of plan.other) {
      const { kind, course, widthAt } = piece;
      const at = (u: number, v: number) => course.at(u, v);
      const stone = (u: number, v: number, template: number, size: number, sink = 0.022, stretch = 1) => {
        const p = at(u, v);
        const thick = THREE.MathUtils.clamp(0.4 + (size - 0.3) * 0.5, 0.4, 0.6);
        laid.push({ template, x: p.x, y: p.y + sink - 0.062 * thick, z: p.z, turn: random() * Math.PI * 2, up: groundUp(course.ground, p.x, p.z), width: size, depth: size * stretch, thick });
      };
      const chip = (u: number, v: number, size: number) => {
        const p = at(u, v);
        laid.push({ template: FIRST.chip + Math.floor(random() * CHIPS), x: p.x, y: p.y - size * 0.15, z: p.z, turn: random() * Math.PI * 2, up: groundUp(course.ground, p.x, p.z), width: size, depth: size * between(random, 0.8, 1.1), thick: Math.min(size, 0.07) });
      };
      const plant = (shape: number, u: number, v: number, scale: number) => {
        const p = at(u, v);
        copies.push({ shape, x: p.x, y: p.y, z: p.z, turn: random() * Math.PI * 2, scale });
      };

      if (kind === 'stepping') {
        // A stride apart, a little to one side and the other.
        let side = random() < 0.5 ? -1 : 1;
        for (let u = between(random, 0.1, 0.4); u < course.length - 0.15; u += STRIDE * between(random, 0.85, 1.15)) {
          const v = side * between(random, 0.04, 0.14);
          side = -side;
          stone(u, v, FIRST.flag + Math.floor(random() * FLAGS), between(random, 0.36, 0.48), between(random, 0.015, 0.03), between(random, 0.8, 1.05));
        }
      } else if (kind === 'gravel') {
        const area = course.length * piece.width;
        for (let k = 0; k < area * GRAVEL; k++) {
          const u = random() * course.length;
          const half = widthAt(u) / 2;
          // Packed across it, loose ones spilling past its edges.
          const spill = random() < 0.18;
          const v = spill ? (random() < 0.5 ? -1 : 1) * (half + SPILL * random() ** 1.6) : between(random, -half, half);
          chip(u, v, spill ? between(random, 0.035, 0.07) : between(random, 0.03, 0.065));
        }
      } else if (kind === 'logs') {
        const steps = stepsOf(course, piece.width, widthAt);
        flights.push(steps);
        for (const step of steps) logs.push({ step, width: step.width });
      } else if (kind === 'tracks') {
        // Grass up the middle, between the tracks.
        for (let u = random() * 0.4; u < course.length; u += between(random, 0.2, 0.45)) plant(first.small + Math.floor(random() * plants.small.length), u, between(random, -0.12, 0.12), between(random, 0.6, 1.1));
      }

      // Along the way.
      const every = (m: number, fn: (u: number) => void) => {
        for (let u = random() * m; u < course.length; u += m * between(random, 0.6, 1.4)) fn(u);
      };
      // The grass reaching in over the edges, now and then a tuft in it.
      if (kind !== 'stepping' && !winter) {
        every(0.9, (u) => {
          const half = widthAt(u) / 2;
          plant(first.small + Math.floor(random() * plants.small.length), u, (random() < 0.5 ? -1 : 1) * (half - between(random, 0, 0.25)), between(random, 0.7, 1.2));
        });
        every(6, (u) => plant(first.small + Math.floor(random() * plants.small.length), u, between(random, -0.3, 0.3) * widthAt(u), between(random, 0.6, 0.9)));
      }
      // Roots across the forest stretches.
      if (kind !== 'logs') {
        every(ROOTS, (u) => {
          if (piece.forest(u) < 0.5) return;
          const p = at(u, between(random, -0.2, 0.2));
          root(roots, p, p.heading + Math.PI / 2 + between(random, -0.45, 0.45), widthAt(u) + between(random, 0.3, 0.8), course.ground, light, random);
        });
      }
      // Moss along the shaded edges; in autumn leaves drifted along them.
      every(0.7, (u) => {
        const side = random() < 0.5 ? -1 : 1;
        const v = side * (widthAt(u) / 2 + between(random, -0.1, 0.25));
        const p = at(u, v);
        if (!winter && light(p.x, p.z) < 0.84 && random() < 0.7) plant(first.moss + Math.floor(random() * plants.moss.length), u, v, between(random, 0.7, 1.3));
        if (autumn) plant(first.leaves + Math.floor(random() * plants.leaves.length), u, v, between(random, 0.8, 1.3));
      });
      // A few stones kicked onto the grass.
      every(KICKED, (u) => chip(u, (random() < 0.5 ? -1 : 1) * (widthAt(u) / 2 + between(random, 0.15, 0.7)), between(random, 0.06, 0.13)));
      // Puddles in the dips along its middle.
      for (let u = 1.5; u < course.length - 1.5; u += 0.5) {
        const here = at(u, 0).y;
        const around = Math.min(at(u - 1.5, 0).y, at(u + 1.5, 0).y);
        if (around - here > PUDDLE && here <= at(u - 0.5, 0).y && here <= at(u + 0.5, 0).y) {
          puddles.push(puddle(at(u, between(random, -0.15, 0.15)), Math.min(widthAt(u) * 0.4, between(random, 0.3, 0.55)), winter, random));
          u += 2;
        }
      }
      // In winter, a trodden track down the middle of the snow, with
      // footprints along it.
      if (winter && kind !== 'logs') {
        const packed = color(0xb9c1cd);
        tracks.push(strip(course, -0.26, 0.26, (x, z, t) => packed.clone().multiplyScalar(0.94 + 0.1 * noise2(x * 3, z * 3, 61) - 0.05 * Math.abs(2 * t - 1)), () => 0.012, 0.3, 3));
        let left = true;
        for (let u = random() * 0.3; u < course.length; u += 0.36) {
          const p = at(u, left ? -0.09 : 0.09);
          prints.push({ shape: left ? 0 : 1, x: p.x, y: p.y + 0.014, z: p.z, turn: p.heading + between(random, -0.12, 0.12), scale: between(random, 0.9, 1.1) });
          left = !left;
        }
      }
    }

    // Stepping stones, pebbles, kicked stones.
    this.add(new Stones(laid, stoneGeometries(season, seed), light, random));
    // Plants.
    for (const batch of planted(plantShapes, copies, matte(), light, random)) this.add(batch);
    // Roots.
    if (roots.triangles > 0) this.add(flat(roots.geometry()));
    // Log steps: a log across each, the earth level behind it.
    this.flights = new Flights(flights);
    if (logs.length) this.add(logSteps(logs, light, random, ground));
    // Puddles.
    this.puddles = puddles.length;
    if (puddles.length) {
      const merged = mergeGeometries(puddles);
      puddles.forEach((g) => g.dispose());
      if (merged) {
        const material = new THREE.MeshStandardMaterial({ vertexColors: true, roughness: winter ? 0.35 : 0.12, metalness: winter ? 0 : 0.15 });
        material.name = 'puddle';
        const m = mesh(merged, material, false);
        m.receiveShadow = true;
        this.add(m);
      }
    }
    // Winter's trodden track and footprints.
    if (tracks.length) {
      const merged = mergeGeometries(tracks);
      tracks.forEach((g) => g.dispose());
      if (merged) this.add(flat(merged));
      for (const batch of planted(footprintGeometries(), prints, matte(), light, random, 0.04)) this.add(batch);
    }
  }

  // A log step's tread under a point (the walking camera); NaN off them.
  stepAt(x: number, z: number): number {
    return this.flights.stepAt(x, z);
  }

  // A ramp over the log steps, for walking up them over a grid (the forest
  // lake meeting's animals, as StonePath.rampAt).
  rampAt(x: number, z: number, ground: (x: number, z: number) => number): number {
    return this.flights.rampAt(x, z, ground);
  }
}

// A puddle lying in a dip: a flat, ragged pool `radius` across, the sky's
// blue-grey on muddy water; in winter ice, pale and white at its rim.
function puddle(at: { x: number; y: number; z: number }, radius: number, winter: boolean, random: () => number): THREE.BufferGeometry {
  const shape = new Shape();
  const middle = new THREE.Vector3(at.x, at.y + 0.012, at.z);
  const count = 10;
  const water = color(winter ? 0xdde9f2 : 0x9dbcd0);
  const rim = color(winter ? 0xf2f6fa : 0x6d6152);
  const stretch = between(random, 0.6, 1);
  const turn = random() * Math.PI;
  const edge = Array.from({ length: count }, (_, i) => {
    const a = -(2 * Math.PI * i) / count;
    const r = radius * between(random, 0.75, 1.1);
    const x = Math.cos(a) * r;
    const z = Math.sin(a) * r * stretch;
    return new THREE.Vector3(at.x + x * Math.cos(turn) - z * Math.sin(turn), at.y + 0.008, at.z + x * Math.sin(turn) + z * Math.cos(turn));
  });
  for (let i = 0; i < count; i++) shape.triangle(middle, edge[i], edge[(i + 1) % count], vary(water, random, 0.4), rim, rim);
  return shape.geometry();
}

// A root across the path, `length` m long through `p`, turned `turn`:
// bending with the ground under it, a little proud of it in the middle,
// its ends going into it; at most 5 cm over it.
function root(shape: Shape, p: { x: number; z: number }, turn: number, length: number, ground: (x: number, z: number) => number, light: (x: number, z: number) => number, random: () => number): void {
  const bark = color(0x6e4428).multiplyScalar(light(p.x, p.z));
  const dx = Math.sin(turn);
  const dz = Math.cos(turn);
  const COUNT = Math.max(6, Math.round(length / 0.25));
  const wander = between(random, 0, Math.PI * 2);
  const points: THREE.Vector3[] = [];
  for (let k = 0; k <= COUNT; k++) {
    const t = k / COUNT - 0.5;
    const side = 0.04 * Math.sin(wander + k * 1.3);
    const x = p.x + dx * t * length - dz * side;
    const z = p.z + dz * t * length + dx * side;
    const end = 1 - (2 * t) ** 4;
    points.push(new THREE.Vector3(x, ground(x, z) - 0.012 + end * (0.018 + 0.008 * Math.sin(wander + k)), z));
  }
  const radii = points.map((_, k) => 0.026 * (1 - Math.abs(k / COUNT - 0.4) * 0.8));
  addTube(shape, points, radii, { sides: 5, random, paint: () => vary(bark, random, 1.2) });
}

// Footprints in the snow: a left and a right boot's, pressed in, darker.
function footprintGeometries(): THREE.BufferGeometry[] {
  const trodden = color(0x9aa6b6);
  return [1, -1].map((side) => {
    const shape = new Shape();
    const count = 8;
    const middle = new THREE.Vector3(0, 0.001, 0);
    const ring = Array.from({ length: count }, (_, i) => {
      const a = -(2 * Math.PI * i) / count;
      const toe = Math.sin(a) > 0 ? 1 : 0.8;
      return new THREE.Vector3(Math.cos(a) * 0.045 * toe + side * 0.005, 0.001, Math.sin(a) * 0.12);
    });
    for (let i = 0; i < count; i++) shape.triangle(middle, ring[i], ring[(i + 1) % count], shade(trodden, 0.92), trodden, trodden);
    return shape.geometry();
  });
}

// The log steps: at each step a log lying across its lower edge, half in
// the ground, and the earth level behind it at its top.
function logSteps(logs: readonly { step: Step; width: number }[], light: (x: number, z: number) => number, random: () => number, ground: WaysideGround): THREE.Group {
  const group = new THREE.Group();
  const log = new Shape();
  const bark = color(0x7a4b2a);
  addTube(log, [new THREE.Vector3(-0.5, 0, 0), new THREE.Vector3(0.5, 0, 0)], [STEP_DEPTH_LOG, STEP_DEPTH_LOG * 0.92], { sides: 7, random, paint: () => vary(bark, random, 1.3), caps: ['rings', 'rings'] });
  const treads = new Shape();
  const logMatrices: THREE.Matrix4[] = [];
  const tints: THREE.Color[] = [];
  for (const { step, width } of logs) {
    const s = Math.sin(step.heading);
    const c = Math.cos(step.heading);
    // Its log along its lower edge (toward the start of the course).
    const x = step.x - s * (step.depth / 2);
    const z = step.z - c * (step.depth / 2);
    const q = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), step.heading);
    logMatrices.push(new THREE.Matrix4().compose(new THREE.Vector3(x, step.top - STEP_DEPTH_LOG * 0.9, z), q, new THREE.Vector3(width + 0.25, 1, 1)));
    const l = light(x, z) * between(random, 0.92, 1.05);
    tints.push(new THREE.Color(l, l, l));
    // The earth tread behind it, level: the ground's own colours as meshed
    // there (the path's dirt, grass at its edges), in cells so they follow
    // it; its sides and front down to the ground, the cut earth darker.
    const corner = (u: number, v: number) => new THREE.Vector3(step.x + s * u + c * v, step.top - 0.012, step.z + c * u - s * v);
    const tone = (p: THREE.Vector3, dark = 1) => ground.colorAt(p.x, p.z, new THREE.Color()).multiplyScalar(dark);
    const down = (p: THREE.Vector3) => new THREE.Vector3(p.x, Math.min(p.y, ground.heightAt(p.x, p.z) - 0.02), p.z);
    const ACROSS = 4;
    const cells: THREE.Vector3[][] = [];
    for (let i = 0; i <= 2; i++) {
      const row: THREE.Vector3[] = [];
      for (let j = 0; j <= ACROSS; j++) row.push(corner((i / 2 - 0.5) * step.depth, (j / ACROSS - 0.5) * width));
      cells.push(row);
    }
    for (let i = 0; i < 2; i++) {
      for (let j = 0; j < ACROSS; j++) {
        const [a, b, e, d] = [cells[i][j], cells[i][j + 1], cells[i + 1][j + 1], cells[i + 1][j]];
        treads.quad(a, d, e, b, tone(a), tone(d), tone(e), tone(b));
      }
    }
    const faces: [THREE.Vector3, THREE.Vector3][] = [
      ...cells.slice(0, 2).map((row, i) => [row[0], cells[i + 1][0]] as [THREE.Vector3, THREE.Vector3]),
      ...cells.slice(0, 2).map((row, i) => [cells[i + 1][ACROSS], row[ACROSS]] as [THREE.Vector3, THREE.Vector3]),
      ...cells[0].slice(0, ACROSS).map((p, j) => [cells[0][j + 1], p] as [THREE.Vector3, THREE.Vector3]),
    ];
    for (const [p, q] of faces) {
      const [pd, qd] = [down(p), down(q)];
      const [tp, tq] = [tone(p, 0.72), tone(q, 0.72)];
      treads.quad(p, q, qd, pd, tp, tq, tq, tp);
      treads.quad(p, pd, qd, q, tp, tp, tq, tq);
    }
  }
  const batch = new THREE.InstancedMesh(log.geometry(), matte(), logMatrices.length);
  logMatrices.forEach((m, i) => {
    batch.setMatrixAt(i, m);
    batch.setColorAt(i, tints[i]);
  });
  batch.castShadow = false;
  batch.receiveShadow = true;
  batch.computeBoundingSphere();
  // Drawn as the ground is (its grain, autumn's leaves on it), on a path.
  const earth = treads.geometry();
  earth.setAttribute('cover', new THREE.BufferAttribute(new Float32Array(earth.getAttribute('position').count).fill(0.3), 1));
  const tread = new THREE.Mesh(earth, ground.material);
  tread.receiveShadow = true;
  group.add(batch, tread);
  return group;
}
