import * as THREE from 'three';
import type { ForestLake } from '../../environtments/ForestLake/ForestLake';
import type { Deck } from '../../environtments/Lake/DockSite';
import { Land, type Circle } from '../lake_meeting/Land';
import { finish, type Stepwise } from '../../stepwise';
import { triangles, type Square } from './ForestSight';

// The forest lake's land as the meeting's animals roam it, on the lake
// meeting's Land.ts, in the meeting's coordinates (its origin the landing,
// `offset` in the lake's):
//
// - The ground: the land, the piers' and bridges' decks, the island's steps
//   and the cave's floor (ForestLake.landAt), a pier's landward end eased up
//   from the bank onto its deck over RAMP m, so there is no step. The paths'
//   steps (stone stairs, log steps) are left out of it (their treads' edges
//   would count as too steep), and an animal stands on them up a ramp
//   (standAt).
// - Water: the lake, the rivers and the stream (Terrain.waterAt), and their
//   banks up to WET m over them.
// - Out to `reach` m round the lake's middle, and only what can be walked to
//   from the landing: not the island.
// - What stands on it: the trees' trunks and the rocks (Scatter.obstacles),
//   and the footprints of the rest, found from their meshes: whatever of the
//   landmarks (the tent, the fences, the lamps, the piers' posts, the
//   bridges' rails, the ruins, the cave's rock), the cliffs, the bushes, the
//   logs and the stumps is between LOW and HIGH m over the ground under it,
//   each FOOTPRINT m square of it a circle to keep off.
// - The piers on the shore (not the island's jetty) are decks: the room on
//   them is measured exactly, and now and then an animal walks out on one
//   (Roam.ts).

const RAMP = 0.6; // m of a pier's deck over which it rises from the bank
const WET = 0.12; // m over the water: a bank lower than this is its wet edge
const LOW = 0.06; // m over the ground: lower than this is walked over (a flat stone, a root)
const HIGH = 1.3; // m over the ground: higher than this is walked under (a crown, the cave's roof)
const FOOTPRINT = 0.25; // m, the squares footprints are found in

export interface ForestGround {
  land: Land;
  // The trunks, rocks and footprints, for the cameras to see round.
  obstacles: Circle[];
  // The height to stand at, over the landing's, at a point.
  standAt(x: number, z: number): number;
}

export function forestGround(lake: ForestLake, offset: THREE.Vector3, middle: THREE.Vector2, reach: number): ForestGround {
  return finish(forestGroundBuild(lake, offset, middle, reach));
}

// The same, a piece at a time, each yield a place to stop and draw a frame:
// the meeting measures the next season's land while it plays.
export function* forestGroundBuild(lake: ForestLake, offset: THREE.Vector3, middle: THREE.Vector2, reach: number): Stepwise<ForestGround> {
  const terrain = lake.terrain;
  const piers = lake.landmarks.piers
    .filter((pier) => pier.name !== 'jetty')
    .map((pier) => {
      const dx = Math.cos(pier.heading);
      const dz = Math.sin(pier.heading);
      return { ...pier, dx, dz, foot: terrain.heightAt(pier.x, pier.z) };
    });
  // In the lake's coordinates.
  const stand = (x: number, z: number) => {
    for (const pier of piers) {
      const along = (x - pier.x) * pier.dx + (z - pier.z) * pier.dz;
      const across = (z - pier.z) * pier.dx - (x - pier.x) * pier.dz;
      if (Math.abs(across) <= pier.width / 2 + 0.05 && along >= -0.05 && along < RAMP) {
        return Math.max(terrain.heightAt(x, z), THREE.MathUtils.lerp(pier.foot, pier.height, Math.max(0, along) / RAMP));
      }
    }
    return lake.landAt(x, z, false);
  };

  // What stands on the land.
  const square: Square = { x0: middle.x - reach, z0: middle.y - reach, size: 2 * reach };
  const obstacles: Circle[] = [];
  for (const o of lake.scatter.obstacles) {
    if (o.x < square.x0 || o.z < square.z0 || o.x > square.x0 + square.size || o.z > square.z0 + square.size) continue;
    obstacles.push({ x: o.x - offset.x, z: o.z - offset.z, radius: o.radius });
  }
  const across = Math.ceil(square.size / FOOTPRINT);
  const ground = new Float32Array(across * across).fill(NaN);
  const taken = new Uint8Array(across * across);
  lake.updateMatrixWorld(true);
  const toLake = lake.matrixWorld.clone().invert();
  const mark = (a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3) => {
    const longest = Math.max(a.distanceTo(b), b.distanceTo(c), c.distanceTo(a));
    const n = Math.max(1, Math.ceil(longest / (FOOTPRINT / 2)));
    for (let p = 0; p <= n; p++) {
      for (let q = 0; q <= n - p; q++) {
        const u = p / n;
        const v = q / n;
        const y = a.y + (b.y - a.y) * u + (c.y - a.y) * v;
        const i = Math.floor((a.x + (b.x - a.x) * u + (c.x - a.x) * v - square.x0) / FOOTPRINT);
        const j = Math.floor((a.z + (b.z - a.z) * u + (c.z - a.z) * v - square.z0) / FOOTPRINT);
        if (i < 0 || j < 0 || i >= across || j >= across) continue;
        const k = j * across + i;
        if (taken[k]) continue;
        if (Number.isNaN(ground[k])) ground[k] = stand(square.x0 + (i + 0.5) * FOOTPRINT, square.z0 + (j + 0.5) * FOOTPRINT);
        if (y > ground[k] + LOW && y < ground[k] + HIGH) taken[k] = 1;
      }
    }
  };
  for (const root of [...lake.landmarks.solid, ...lake.scatter.low, lake.cliffs]) yield* triangles(root, toLake, square, 0, mark);
  const radius = (FOOTPRINT * Math.SQRT2) / 2;
  for (let j = 0; j < across; j++) {
    for (let i = 0; i < across; i++) {
      if (taken[j * across + i]) obstacles.push({ x: square.x0 + (i + 0.5) * FOOTPRINT - offset.x, z: square.z0 + (j + 0.5) * FOOTPRINT - offset.z, radius });
    }
  }

  const decks: Deck[] = piers.map((pier) => ({
    x: pier.x - offset.x,
    z: pier.z - offset.z,
    heading: Math.atan2(pier.dx, pier.dz),
    width: pier.width,
    length: pier.length,
    height: pier.height - offset.y,
    foot: pier.foot - offset.y,
  }));
  const land = yield* Land.build({
    ground: (x, z) => stand(x + offset.x, z + offset.z),
    wet: (x, z, height) => {
      const water = terrain.waterAt(x + offset.x, z + offset.z);
      return !Number.isNaN(water) && height < water + WET;
    },
    middle: new THREE.Vector2(middle.x - offset.x, middle.y - offset.z),
    reach,
    obstacles,
    decks,
    home: new THREE.Vector2(0, 0),
  });
  // Where to stand: over a stone path's stairs (its treads left out of the
  // land, whose edges would count as ground too steep to stand on) up a
  // ramp through the middles of their risers (StonePath.rampAt).
  const standAt = (x: number, z: number) => {
    const ramp = lake.rampAt(x, z, stand);
    return Number.isNaN(ramp) ? stand(x, z) : ramp;
  };
  return { land, obstacles, standAt: (x, z) => standAt(x + offset.x, z + offset.z) - offset.y };
}
