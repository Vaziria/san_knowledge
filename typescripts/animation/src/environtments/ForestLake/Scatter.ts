import * as THREE from 'three';
import { BroadleafTree, type AutumnTone } from '../../figures/ForestLake/BroadleafTree';
import { Bush } from '../../figures/ForestLake/Bush';
import { CherryTree } from '../../figures/ForestLake/CherryTree';
import { DeadTree } from '../../figures/ForestLake/DeadTree';
import { FallenLeaves } from '../../figures/ForestLake/FallenLeaves';
import { FlowerCluster } from '../../figures/ForestLake/FlowerCluster';
import { ForestPine, type PineTone } from '../../figures/ForestLake/ForestPine';
import { GrassPatch } from '../../figures/ForestLake/GrassPatch';
import { LargeRock } from '../../figures/ForestLake/LargeRock';
import { LilyPad } from '../../figures/ForestLake/LilyPad';
import { MossyLog } from '../../figures/ForestLake/MossyLog';
import { Mushrooms } from '../../figures/ForestLake/Mushrooms';
import { addBlade, addFlower, addRock, addTube, addTuft, between, color, instances, matte, mesh, palette, seededRandom, Shape, snowOn, vary, type Palette, type Season } from '../../figures/ForestLake/parts';
import { Reeds } from '../../figures/ForestLake/Reeds';
import { SmallRock } from '../../figures/ForestLake/SmallRock';
import { Stump } from '../../figures/ForestLake/Stump';
import { LANDING } from './layout';
import { smoothstep, type Shade, type Terrain } from './Terrain';
import type { Froth } from './Water';

// A place kept clear: nothing taller than grass within `radius` of it
// (grass and flowers too, when `bare`).
export interface Keep {
  x: number;
  z: number;
  radius: number;
  bare?: boolean;
}

interface Placed {
  x: number;
  y: number;
  z: number;
  scale: number;
  turn: number;
  tilt?: number; // radians it leans, for a rock
}

// Where something may grow on a cliff: a column's top standing out of the
// face, or along the cliffs' top edge (Cliffs.ledges).
export interface Ledge {
  x: number;
  y: number;
  z: number;
  top: boolean;
}

// What grows and lies about on the forest lake's land and water
// (ForestLake.md), placed by seeded rules so it is the same on every load
// and in every season, as the reference's seasons show the one valley:
// a mixed forest round the lake and on the heights, pines, round broadleaf
// trees and cherries, thick right down to the shore and thinning only on
// the far hills; bushes at the forest's edges, along the paths and on the
// cliffs' ledges and top; tall grass and flower clusters in the meadows,
// thickest round the landing; a ring of boulders round the lake's shore
// and the island, rocks along the stream and in the forest, pebbles and
// flat stones along the paths; reeds and lily pads in the shallows; logs,
// stumps and mushrooms in the forest. Each season swaps what it is: in
// autumn the broadleaf trees orange, red and gold, some pines golden
// larches, leaves fallen thick on the ground and floating on the water; in
// winter everything under snow, bare trees among the pines, the lily pads
// gone under the ice. Each kind is a few variants of its asset, drawn
// instanced, each copy turned, sized and tinted its own way.
export class Scatter extends THREE.Group {
  readonly shades: Shade[] = [];
  readonly froth: Froth[] = [];
  // Trunks and rocks, for anything walking to go round.
  readonly obstacles: { x: number; z: number; radius: number }[] = [];
  // Once built: what a camera can't be in or see through (the trees,
  // bushes, rocks, logs and stumps), and of those what `obstacles` leaves
  // out (the bushes, logs and stumps), for the forest lake meeting to find
  // their footprints (forest_lake_meeting/).
  readonly solid: THREE.Object3D[] = [];
  readonly low: THREE.Object3D[] = [];
  // The cherry and broadleaf trees nearest the landing, for their falling
  // petals or leaves.
  readonly shedding: { x: number; y: number; z: number; height: number }[] = [];
  private readonly kinds: { templates: THREE.Object3D[]; placed: Placed[][]; tint: number; hue: number; shaded: boolean; solid?: boolean; low?: boolean }[] = [];

  constructor(
    private readonly terrain: Terrain,
    keep: readonly Keep[],
    readonly season: Season = 'spring',
    ledges: readonly Ledge[] = [],
  ) {
    super();
    const random = seededRandom(601);
    const winter = season === 'winter';
    const autumn = season === 'autumn';
    const P = palette(season);

    // Distances that rule where things go.
    const pathGap = (x: number, z: number) => {
      let gap = Infinity;
      for (const { line, width } of terrain.paths) {
        const near = line.nearest(x, z);
        if (near.segment >= 0) gap = Math.min(gap, near.distance - width / 2);
      }
      return gap;
    };
    const kept = (x: number, z: number, bare: boolean) => keep.some((k) => (bare ? k.bare : true) && Math.hypot(x - k.x, z - k.z) < k.radius);
    // The landing's views kept open: south, behind the preview cameras, and
    // north, down to the water, which they look across.
    const inView = (x: number, z: number) =>
      (z > LANDING[1] && z < LANDING[1] + 48 && Math.abs(x - LANDING[0]) < 3 + (z - LANDING[1]) * 0.4) ||
      (z <= LANDING[1] && z > LANDING[1] - 8 && Math.abs(x - LANDING[0]) < 2.5 + (LANDING[1] - z) * 1.1);
    const dry = (x: number, z: number, margin = 0.15) => {
      const water = terrain.waterAt(x, z);
      return Number.isNaN(water) || terrain.heightAt(x, z) > water + margin;
    };
    const cliffBand = (x: number, z: number) => {
      const edge = terrain.rim.nearest(x, z);
      if (edge.segment < 0) return false;
      const inward = edge.distance * edge.side;
      return inward > -3.4 && inward < 1.4 && terrain.rim.value(edge) - terrain.heightAt(x, z) > -0.5 && terrain.rim.value(edge) > 1.2;
    };
    const ground = (x: number, z: number) => terrain.heightAt(x, z);
    const grid = (spacing: number, from: [number, number], to: [number, number], each: (x: number, z: number) => void) => {
      for (let z = from[1]; z < to[1]; z += spacing) {
        for (let x = from[0]; x < to[0]; x += spacing) each(x + between(random, -0.45, 0.45) * spacing, z + between(random, -0.45, 0.45) * spacing);
      }
    };
    const near = (x: number, z: number) => Math.hypot(x - LANDING[0], z - LANDING[1]);

    // The trees: a mixed forest of pines and round broadleaf trees, thick
    // down to the shore, with cherries near the water and the paths, and in
    // winter bare trees among them.
    const pines: Placed[][] = [[], [], []];
    const farPines: Placed[] = [];
    // Near the landing in full, further out simpler (the second three).
    const broadleaves: Placed[][] = [[], [], [], [], [], []];
    const cherries: Placed[][] = [[], [], []];
    const bare: Placed[][] = [[], []];
    const tree = (x: number, z: number, far: boolean) => {
      if (!dry(x, z, 0.3) || kept(x, z, false) || inView(x, z) || cliffBand(x, z) || pathGap(x, z) < 1.6) return;
      const shore = terrain.lake.distance(x, z);
      if (shore < 2.2) return;
      // Thick in the forest and right down to the shore, thinner far out.
      if (random() > smoothstep(2.2, 7, shore) * (far ? 0.72 : 0.93)) return;
      const y = ground(x, z);
      const turn = random() * Math.PI * 2;
      const roll = random();
      if (!far && roll < (shore < 14 ? 0.13 : 0.04)) {
        const scale = between(random, 0.8, 1.2);
        cherries[Math.floor(random() * 3)].push({ x, y, z, scale, turn });
        this.shades.push({ x, y: y + 5.5 * scale * 0.68, z, radius: 5.5 * scale * 0.4, foot: 1.6 * scale });
        this.obstacles.push({ x, z, radius: 0.35 * scale });
        if (near(x, z) < 26) this.shedding.push({ x, y, z, height: 5.5 * scale });
        return;
      }
      const leafy = far ? 0.12 : autumn ? 0.52 : 0.34;
      if (roll < leafy + 0.13) {
        const scale = between(random, 0.8, 1.25) * (far ? 1.15 : 1);
        if (winter && random() < 0.35) {
          bare[Math.floor(random() * 2)].push({ x, y, z, scale, turn });
          this.obstacles.push({ x, z, radius: 0.3 * scale });
          return;
        }
        broadleaves[Math.floor(random() * 3) + (near(x, z) < 30 ? 0 : 3)].push({ x, y, z, scale, turn });
        this.shades.push({ x, y: y + 6 * scale * 0.66, z, radius: 6 * scale * 0.36, foot: 1.6 * scale });
        this.obstacles.push({ x, z, radius: 0.35 * scale });
        if (autumn && near(x, z) < 26) this.shedding.push({ x, y, z, height: 6 * scale });
        return;
      }
      const scale = between(random, 0.7, 1.45) * (far ? 1.1 : 1);
      (far ? farPines : pines[Math.floor(random() * 3)]).push({ x, y, z, scale, turn });
      const h = 8 * scale;
      this.shades.push({ x, y: y + h * 0.42, z, radius: h * 0.25, foot: 1.3 * scale }, { x, y: y + h * 0.7, z, radius: h * 0.15 });
      this.obstacles.push({ x, z, radius: 0.4 * scale });
    };
    grid(3.7, [-82, -100], [82, 82], (x, z) => {
      if (Math.hypot(x - 1, z + 6) < 84) tree(x, z, false);
    });
    grid(7.2, [-235, -250], [235, 235], (x, z) => {
      const r = Math.hypot(x - 1, z + 6);
      if (r >= 84 && r < 235) tree(x, z, true);
    });

    // Bushes at the forest's edges and along the paths, and on the cliffs'
    // ledges and top edge.
    const bushes: Placed[][] = [[], [], [], [], [], []];
    grid(4.2, [-70, -80], [70, 64], (x, z) => {
      if (!dry(x, z, 0.2) || kept(x, z, false) || cliffBand(x, z) || pathGap(x, z) < 0.6 || inView(x, z)) return;
      const shore = terrain.lake.distance(x, z);
      const edge = shore > 2.2 && shore < 16 ? 0.46 : 0.16;
      if (random() > edge) return;
      const scale = between(random, 0.65, 1.35);
      bushes[Math.floor(random() * 3) + (near(x, z) < 30 ? 0 : 3)].push({ x, y: ground(x, z), z, scale, turn: random() * Math.PI * 2 });
      this.shades.push({ x, y: ground(x, z) + 0.45 * scale, z, radius: 0.55 * scale, foot: 0.9 * scale });
    });

    // Tall grass and flowers in the meadows, thickest round the landing;
    // fewer flowers in autumn, and in winter dry grass and sprigs of
    // berries standing out of the snow.
    const grass: Placed[][] = [[], [], []];
    const flowers: Placed[][] = [[], [], [], []];
    const grassy = winter ? 0.35 : 1;
    const flowery = winter ? 0.22 : autumn ? 0.55 : 1;
    grid(1.35, [-62, -70], [62, 60], (x, z) => {
      if (!dry(x, z, 0.08) || kept(x, z, true) || cliffBand(x, z) || pathGap(x, z) < 0.05) return;
      const landing = near(x, z);
      if (landing < 1.9) return; // the figure's own spot stays open
      const close = smoothstep(14, 4, landing);
      const r = Math.hypot(x - 1, z + 6);
      const meadow = 1 - smoothstep(35, 62, r);
      const roll = random();
      const y = ground(x, z);
      const tall = (0.2 + close * 0.3) * meadow * grassy;
      if (roll < tall) {
        grass[Math.floor(random() * 3)].push({ x, y, z, scale: between(random, 0.45, 1.0), turn: random() * Math.PI * 2 });
      } else if (roll < tall + (0.22 + close * 0.05 + (pathGap(x, z) < 1.2 ? 0.2 : 0)) * meadow * flowery) {
        flowers[Math.floor(random() * 4)].push({ x, y, z, scale: between(random, 0.75, 1.2), turn: random() * Math.PI * 2 });
      }
    });

    // Ground cover: low tufts of grass and single flowers, thick in the
    // meadows, along the paths and round the landing (in winter a few
    // tufts, no flowers).
    const tufts: Placed[][] = [[], []];
    const singles: Placed[][] = [[], [], [], []];
    grid(0.95, [-52, -60], [52, 48], (x, z) => {
      if (!dry(x, z, 0.06) || kept(x, z, true) || cliffBand(x, z) || pathGap(x, z) < -0.2) return;
      const landing = near(x, z);
      if (landing < 1.4) return;
      const r = Math.hypot(x - 1, z + 6);
      const meadow = 1 - smoothstep(30, 50, r);
      const close = smoothstep(16, 3, landing);
      const edge = pathGap(x, z) < 1.2 ? 1 : 0;
      const roll = random();
      const y = ground(x, z);
      const low = (0.26 + close * 0.3) * meadow * grassy;
      if (roll < low) tufts[Math.floor(random() * 2)].push({ x, y, z, scale: between(random, 0.7, 1.3), turn: random() * Math.PI * 2 });
      else if (!winter && roll < low + (0.1 + close * 0.12 + edge * 0.18) * meadow * flowery) singles[Math.floor(random() * 4)].push({ x, y, z, scale: between(random, 0.8, 1.25), turn: random() * Math.PI * 2 });
    });

    // Rocks: a ring of boulders round the lake's shore, half in the water,
    // lower in the landing's view; a ring round the island's plinth; along
    // the stream's banks; in the forest now and then; pebbles and flat
    // stones along the paths.
    const small: Placed[][] = [[], [], []];
    const large: Placed[][] = [[], []];
    const flat: Placed[] = [];
    const rock = (x: number, z: number, size: number, big: boolean, sink = 0.12) => {
      if (kept(x, z, false)) return;
      const y = ground(x, z) - size * sink;
      (big ? large : small)[Math.floor(random() * (big ? 2 : 3))].push({ x, y, z, scale: size / (big ? 2.4 : 0.75), turn: random() * Math.PI * 2, tilt: between(random, -0.15, 0.15) });
      this.obstacles.push({ x, z, radius: size * 0.45 });
      if (size > 0.7) this.shades.push({ x, y: y + size * 0.4, z, radius: size * 0.45, foot: size * 0.8 });
      const water = terrain.waterAt(x, z);
      if (!Number.isNaN(water) && ground(x, z) < water + 0.1) this.froth.push({ x, z, radius: size * 0.5 + 0.5, strength: 0.55 });
    };
    const lake = terrain.lake.points;
    for (let i = 0; i < lake.length; i++) {
      const [x, z] = lake[i];
      // Low ones only in the landing's view of the lake.
      const low = inView(x, z);
      // On the bank, just up from the water, their feet in it.
      const up = terrain.lake.distance(x + 0.05, z) - terrain.lake.distance(x - 0.05, z);
      const upZ = terrain.lake.distance(x, z + 0.05) - terrain.lake.distance(x, z - 0.05);
      const n = new THREE.Vector2(up, upZ).normalize();
      const d = between(random, -0.1, 0.55);
      if (random() < (low ? 0.3 : 0.85)) rock(x + n.x * d, z + n.y * d, between(random, low ? 0.3 : 0.7, low ? 0.48 : 1.45), false, 0.14);
      if (!low && random() < 0.08) rock(x + n.x * 0.5, z + n.y * 0.5, between(random, 1.4, 2.3), true, 0.14);
    }
    for (const [x, z] of terrain.island.points) {
      // Just outside its edge, half in the water, big enough to wall it.
      const out = terrain.island.distance(x + 0.01, z) - terrain.island.distance(x - 0.01, z);
      const outZ = terrain.island.distance(x, z + 0.01) - terrain.island.distance(x, z - 0.01);
      const n = new THREE.Vector2(out, outZ).normalize();
      const d = between(random, 0.05, 0.35);
      rock(x + n.x * d, z + n.y * d, between(random, 0.95, 1.4), false, 0.3);
    }
    const stream = terrain.stream;
    for (let along = 3; along < stream.length; along += 1.8) {
      const at = stream.at(along);
      for (const side of [-1, 1]) {
        if (random() > 0.55) continue;
        const off = side * between(random, 1.6, 2.6);
        rock(at.x - at.dz * off, at.z + at.dx * off, between(random, 0.4, 1.1), false);
      }
    }
    grid(7, [-75, -85], [75, 70], (x, z) => {
      if (!dry(x, z) || cliffBand(x, z) || pathGap(x, z) < 0.8 || inView(x, z) || terrain.lake.distance(x, z) < 4) return;
      if (random() < 0.16) rock(x, z, between(random, 0.4, 1.0), false);
      else if (random() < 0.05) rock(x, z, between(random, 1.4, 2.6), true);
    });
    for (const { line, width } of terrain.paths) {
      for (let along = 0; along < line.length; along += 0.7) {
        const at = line.at(along);
        const roll = random();
        if (roll < 0.3) {
          const off = between(random, -0.5, 0.5) * width;
          const x = at.x - at.dz * off;
          const z = at.z + at.dx * off;
          small[Math.floor(random() * 3)].push({ x, y: ground(x, z) - 0.02, z, scale: between(random, 0.12, 0.28), turn: random() * Math.PI * 2, tilt: 0 });
        } else if (roll < 0.42) {
          // A flat stone set in the path.
          const off = between(random, -0.3, 0.3) * width;
          const x = at.x - at.dz * off;
          const z = at.z + at.dx * off;
          if (!kept(x, z, true)) flat.push({ x, y: ground(x, z) - 0.03, z, scale: between(random, 0.8, 1.3), turn: random() * Math.PI * 2 });
        }
      }
    }

    // In the shallows: reeds in clumps along the shore and the stream, and
    // lily pads further out (yellowing in autumn, none in winter's ice).
    const reeds: Placed[][] = [[], []];
    grid(1.6, [-24, -32], [26, 22], (x, z) => {
      if (kept(x, z, false) || inView(x, z)) return;
      const water = terrain.waterAt(x, z);
      if (Number.isNaN(water)) return;
      const depth = water - ground(x, z);
      if (depth < 0.04 || depth > 0.5) return;
      if (random() > (winter ? 0.07 : 0.2)) return;
      reeds[Math.floor(random() * 2)].push({ x, y: water, z, scale: between(random, 0.75, 1.15), turn: random() * Math.PI * 2 });
    });
    const lilies: Placed[][] = [[], [], []];
    if (!winter) {
      grid(1.7, [-22, -30], [22, 16], (x, z) => {
        if (!terrain.lake.inside(x, z) || kept(x, z, false)) return;
        const depth = -ground(x, z);
        if (depth < 0.2 || depth > 1.8) return;
        if (random() > 0.45) return;
        lilies[Math.floor(random() * 3)].push({ x, y: 0.005, z, scale: between(random, 0.7, 1.15), turn: random() * Math.PI * 2 });
      });
    }

    // Logs and stumps in the forest, and mushrooms among them.
    const logs: Placed[][] = [[], []];
    const stumps: Placed[][] = [[], []];
    const mushrooms: Placed[][] = [[], []];
    const fungi = winter ? 0 : autumn ? 0.16 : 0.07;
    grid(9, [-70, -80], [70, 64], (x, z) => {
      if (!dry(x, z) || kept(x, z, false) || cliffBand(x, z) || pathGap(x, z) < 1 || inView(x, z) || terrain.lake.distance(x, z) < 3) return;
      const roll = random();
      if (roll < 0.1) logs[Math.floor(random() * 2)].push({ x, y: ground(x, z) - 0.04, z, scale: between(random, 0.8, 1.3), turn: random() * Math.PI * 2 });
      else if (roll < 0.2) stumps[Math.floor(random() * 2)].push({ x, y: ground(x, z) - 0.03, z, scale: between(random, 0.7, 1.2), turn: random() * Math.PI * 2 });
    });
    grid(5, [-60, -70], [60, 56], (x, z) => {
      if (!dry(x, z) || kept(x, z, false) || cliffBand(x, z) || pathGap(x, z) < 0.5 || inView(x, z) || terrain.lake.distance(x, z) < 2.5) return;
      if (random() < fungi) mushrooms[Math.floor(random() * 2)].push({ x, y: ground(x, z) - 0.01, z, scale: between(random, 0.8, 1.4), turn: random() * Math.PI * 2 });
    });

    // In autumn, fallen leaves in drifts on the ground, thickest under the
    // trees and along the paths, and floating on the lake by its shore.
    const fallen: Placed[][] = [[], []];
    if (autumn) {
      grid(2.3, [-58, -66], [58, 54], (x, z) => {
        if (kept(x, z, true) || cliffBand(x, z) || near(x, z) < 1.6) return;
        const water = terrain.waterAt(x, z);
        const wet = !Number.isNaN(water) && ground(x, z) < water;
        if (wet && (terrain.lake.distance(x, z) < -4 || !terrain.lake.inside(x, z))) return;
        if (random() > (wet ? 0.2 : 0.42)) return;
        fallen[Math.floor(random() * 2)].push({ x, y: wet ? 0.004 : ground(x, z), z, scale: between(random, 0.8, 1.5), turn: random() * Math.PI * 2 });
      });
    }

    // On the cliffs: bushes along the top edge, grass and small bushes on
    // the ledges.
    for (const ledge of ledges) {
      if (kept(ledge.x, ledge.z, false)) continue;
      const roll = random();
      if (roll < (ledge.top ? 0.3 : 0.18)) bushes[Math.floor(random() * 3) + (near(ledge.x, ledge.z) < 30 ? 0 : 3)].push({ x: ledge.x, y: ledge.y - 0.05, z: ledge.z, scale: between(random, 0.55, 1.0), turn: random() * Math.PI * 2 });
      else if (roll < (ledge.top ? 0.55 : 0.5)) grass[Math.floor(random() * 3)].push({ x: ledge.x, y: ledge.y, z: ledge.z, scale: between(random, 0.5, 0.9) * grassy, turn: random() * Math.PI * 2 });
    }

    // The variants of each kind, and their copies.
    const pineTones: PineTone[] = autumn ? ['green', 'dark', 'gold'] : ['green', 'dark', 'light'];
    const autumnTones: AutumnTone[] = ['orange', 'red', 'yellow'];
    const tuftTemplates = [lowTuft(11, P), lowTuft(12, P)];
    // Snow on the tufts, in winter.
    if (winter) for (const template of tuftTemplates) snowOn(template, random, 0.25, 0.6);
    this.kinds.push(
      { templates: pineTones.map((tone, k) => new ForestPine({ seed: 1 + k, height: 8, tiers: k === 1 ? 5 : 4, tone, season })), placed: pines, tint: 0.1, hue: 0.06, shaded: true, solid: true },
      { templates: [new ForestPine({ seed: 4, height: 8, tiers: 3, simple: true, tone: 'dark', season })], placed: [farPines], tint: 0.12, hue: 0.06, shaded: true, solid: true },
      { templates: [0, 1, 2, 3, 4, 5].map((k) => new BroadleafTree({ seed: 11 + (k % 3), height: 6, season, tone: autumnTones[k % 3], simple: k >= 3 })), placed: broadleaves, tint: 0.08, hue: 0.08, shaded: true, solid: true },
      { templates: [0, 1, 2].map((k) => new CherryTree({ seed: 1 + k, season })), placed: cherries, tint: 0.05, hue: 0.02, shaded: true, solid: true },
      { templates: [new DeadTree({ seed: 1 }), new DeadTree({ seed: 2, height: 4.2 })], placed: bare, tint: 0.06, hue: 0, shaded: true, solid: true },
      { templates: [0, 1, 2, 3, 4, 5].map((k) => new Bush({ seed: 1 + (k % 3), flowers: k % 3 !== 1, season, simple: k >= 3 })), placed: bushes, tint: 0.08, hue: 0.05, shaded: true, solid: true, low: true },
      { templates: [new GrassPatch({ seed: 1, height: 0.5, season }), new GrassPatch({ seed: 2, height: 0.42, season }), new GrassPatch({ seed: 3, height: 0.36, season })], placed: grass, tint: 0.08, hue: 0.03, shaded: true },
      {
        templates: [
          new FlowerCluster({ seed: 1, kind: 'pink', season }),
          new FlowerCluster({ seed: 2, kind: 'mixed', season }),
          new FlowerCluster({ seed: 3, kind: 'pink', flowers: 4, season }),
          new FlowerCluster({ seed: 4, kind: 'mixed', flowers: 5, season }),
        ],
        placed: flowers,
        tint: 0.04,
        hue: 0,
        shaded: true,
      },
      { templates: [new SmallRock({ seed: 1, dressing: false, season }), new SmallRock({ seed: 2, dressing: false, season }), new SmallRock({ seed: 3, dressing: false, season })], placed: small, tint: 0.06, hue: 0.02, shaded: true, solid: true },
      { templates: [new LargeRock({ seed: 1, dressing: false, moss: true, season }), new LargeRock({ seed: 2, dressing: false, season })], placed: large, tint: 0.05, hue: 0.02, shaded: true, solid: true },
      { templates: [flatStone(31, season)], placed: [flat], tint: 0.06, hue: 0, shaded: true },
      { templates: [new LilyPad({ seed: 1, season, flower: !autumn }), new LilyPad({ seed: 2, flower: false, season }), new LilyPad({ seed: 3, pads: 3, season, flower: !autumn })], placed: lilies, tint: 0.05, hue: 0.03, shaded: false },
      { templates: [new Reeds({ seed: 1, season }), new Reeds({ seed: 2, season, height: 0.9, cattails: 3 })], placed: reeds, tint: 0.06, hue: 0.03, shaded: true },
      { templates: [new MossyLog({ seed: 1, dressing: false, season }), new MossyLog({ seed: 2, dressing: false, mushrooms: false, length: 2.2, season })], placed: logs, tint: 0.05, hue: 0, shaded: true, solid: true, low: true },
      { templates: tuftTemplates, placed: tufts, tint: 0.1, hue: 0.03, shaded: true },
      { templates: [single(21, P.petalPink, P), single(22, P.petalWhite, P), single(23, P.petalLilac, P), single(24, P.petalYellow, P)], placed: singles, tint: 0.05, hue: 0, shaded: true },
      { templates: [new Stump({ seed: 1, dressing: false, season }), new Stump({ seed: 2, dressing: false, mushrooms: false, radius: 0.28, season })], placed: stumps, tint: 0.05, hue: 0, shaded: true, solid: true, low: true },
      { templates: [new Mushrooms({ seed: 1, season }), new Mushrooms({ seed: 2, count: 3, season })], placed: mushrooms, tint: 0.05, hue: 0, shaded: true },
      { templates: [new FallenLeaves({ seed: 1 }), new FallenLeaves({ seed: 2, radius: 0.6, count: 18 })], placed: fallen, tint: 0.08, hue: 0.04, shaded: true },
    );
  }

  // How many copies of everything there are.
  get count(): number {
    return this.kinds.reduce((sum, kind) => sum + kind.placed.reduce((s, list) => s + list.length, 0), 0);
  }

  // Builds the instanced meshes, each copy tinted by how much light
  // reaches the ground under it (Terrain.lightAt(), known once the ground
  // mesh is built) and a little at random, in brightness and in hue.
  build(): void {
    const random = seededRandom(602);
    const m = new THREE.Matrix4();
    const q = new THREE.Quaternion();
    for (const kind of this.kinds) {
      kind.templates.forEach((template, i) => {
        const placed = kind.placed[i];
        if (!placed?.length) return;
        const matrices = placed.map((p) => {
          q.setFromEuler(new THREE.Euler(p.tilt ?? 0, p.turn, (p.tilt ?? 0) * 0.7, 'YXZ'));
          return m.clone().compose(new THREE.Vector3(p.x, p.y, p.z), q, new THREE.Vector3(p.scale, p.scale, p.scale));
        });
        const tints = placed.map((p) => {
          const light = kind.shaded ? this.terrain.lightAt(p.x, p.z) : 1;
          const t = light * (1 + (random() - 0.5) * 2 * kind.tint);
          const warm = (random() - 0.5) * 2 * kind.hue;
          return new THREE.Color(t * (1 + warm), t, t * (1 - warm));
        });
        const group = instances(template, matrices, tints);
        this.add(group);
        if (kind.solid) this.solid.push(group);
        if (kind.low) this.low.push(group);
      });
    }
  }
}

// A low tuft of grass, 20 cm tall: ground cover.
function lowTuft(seed: number, P: Palette): THREE.Group {
  const random = seededRandom(seed);
  const shape = new Shape();
  addTuft(shape, new THREE.Vector3(), 7, 0.2, random, 0.01, 0.3, P);
  const group = new THREE.Group();
  group.add(mesh(shape.geometry(), matte(), false));
  return group;
}

// One flower on its stem with a leaf or two at its foot: ground cover.
function single(seed: number, petal: number, P: Palette): THREE.Group {
  const random = seededRandom(seed);
  const shape = new Shape();
  const head = new THREE.Vector3(between(random, -0.03, 0.03), between(random, 0.16, 0.26), between(random, -0.03, 0.03));
  addTube(shape, [new THREE.Vector3(0, -0.01, 0), head], [0.006, 0.005], { sides: 3, random, paint: () => color(P.grass) });
  addFlower(shape, head, new THREE.Vector3(head.x * 4, 1, head.z * 4), 6, between(random, 0.05, 0.065), vary(color(petal), random, 0.6), color(P.flowerHeart), random, 0.15);
  for (let k = 0; k < 2; k++) {
    const a = random() * Math.PI * 2;
    const out = new THREE.Vector3(Math.cos(a), 0, Math.sin(a));
    addBlade(shape, new THREE.Vector3(0, -0.01, 0), out, new THREE.Vector3(-out.z, 0, out.x), 0.12, 0.7, 0.035, color(P.grassFoot), color(P.grass), color(P.grassTip));
  }
  const group = new THREE.Group();
  group.add(mesh(shape.geometry(), matte(), false));
  return group;
}

// A flat stone set in a path, 45 cm across, its top just over the dirt:
// the reference's stone path.
function flatStone(seed: number, season: Season): THREE.Group {
  const random = seededRandom(seed);
  const shape = new Shape();
  addRock(shape, new THREE.Vector3(), new THREE.Vector3(0.48, 0.12, 0.42), 0, random, 2, 12, palette(season));
  const group = new THREE.Group();
  group.add(mesh(shape.geometry(), matte(), false));
  if (season === 'winter') snowOn(group, random, 0.5, 0.9);
  return group;
}

