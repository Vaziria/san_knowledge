import * as THREE from 'three';
import { grow, type Kind } from './grow';
import { across, arch, blade, heading, tube } from './habit';
import type { Leaves } from './Leaves';
import { between, BLADE, crowded, emptyPlan, STEM, tone, type GrassOptions, type Profile, type Size } from './parts';
import type { Stems } from './Stems';

// Bermuda grass (Cynodon dactylon, rumput grinting), about 20 cm tall and 90
// cm across: a low mat. Runners (stolons) creep out over the ground from the
// middle, wandering a little, and at every node put out a pair of short,
// narrow leaves that spread almost flat, and now and then a short upright
// shoot with its leaves in two rows. More shoots stand in the middle, where
// the plant started, and from there a few thin flower stems rise, each
// ending in four or five slender spikes that spread from its tip like the
// ribs of an umbrella. Leaves are the theme's grass colour, the runners
// toward its wood colour, and the spikes green ripening toward straw with a
// touch of the flower colour, as real ones are tinged purple.
//
// Units are meters, y is up and the origin is on the ground at the middle of
// its foot. It grows from a seed and is sized to a height and width (see
// GrassOptions), so it is the same every time. The spec, BermudaGrass.md, is
// a draft: the grass has no behaviour and stands still.

const RUNNERS = {
  count: 12,
  length: [0.26, 0.44] as const, // m
  node: 0.035, // m from one node to the next
  wander: 0.35, // rad it may turn at each node
  radius: 0.0025, // over twice a real runner's, which the painterly filter would wipe out
  y: 0.003, // m: its axis, so it lies half sunk in the ground
  branching: 0.3, // the chance that a node starts a side runner
  side: [0.08, 0.18] as const, // m, a side runner's length
};
const NODE_LEAF = { length: [0.03, 0.06] as const, lean: [1, 1.35] as const, bend: 0.4, width: 0.0055, fold: 0.2 };
const SHOOT = {
  chance: 0.6, // that a node puts one up
  height: [0.035, 0.09] as const, // m
  lean: [0.1, 0.5] as const,
  radius: 0.0018,
  leaves: [4, 6] as const,
  leafLength: [0.03, 0.065] as const, // m, the lowest; they shorten upward
  leafLean: [0.75, 1.15] as const, // rad from upright
  bend: 0.3,
};
const MIDDLE = { count: 24, base: 0.04, height: [0.06, 0.13] as const }; // upright shoots in the middle
const SPIKES = {
  count: 5,
  base: 0.03, // m round the middle
  height: [0.13, 0.18] as const, // m, the stem to its spikes
  lean: [0.02, 0.2] as const,
  radius: 0.0028, // over twice a real stem's
  fingers: [4, 5] as const,
  length: [0.03, 0.05] as const, // m, a finger
  spread: [0.45, 0.85] as const, // rad from upright
  fingerRadius: 0.0026,
  bumps: 0.25,
};
// A finger: slender, thinner at its foot where it leaves the stem, closing to a point.
const FINGER: Profile = (u) => Math.min(1, Math.sqrt(0.4 + u / 0.08), Math.sqrt((1 - u) / 0.15));

const KIND: Kind = {
  seed: 29,
  size: { height: 0.2, width: 0.9 },
  plan(c, random, crowd) {
    const plan = emptyPlan();
    const leaf = { foot: tone(c.grass, 0.6), tip: tone(c.grass, 0.95) };
    const shootPaint = { foot: tone(c.grass, 0.6), tip: tone(c.grass, 0.8) };
    const runnerPaint = { foot: tone(c.grass, 0.6).lerp(c.wood, 0.25), tip: tone(c.grass, 0.75).lerp(c.wood, 0.2) };

    const leafAt = (at: THREE.Vector3, azimuth: number, length: number, lean: number, bend: number) =>
      plan.leaves.blades.push(blade(arch(at, heading(lean, azimuth), length, bend, 4), across(azimuth), NODE_LEAF.width, NODE_LEAF.fold, BLADE, leaf, random));

    // An upright shoot, its leaves in two rows up it, shortening upward.
    const shoot = (foot: THREE.Vector3, height: number, lean: number) => {
      const spine = arch(foot, heading(lean, 2 * Math.PI * random()), height, -0.2, 5);
      plan.leaves.tubes.push(tube(spine, SHOOT.radius, STEM, 0, 0.02, 5, shootPaint));
      const count = Math.round(between(random, ...SHOOT.leaves));
      const first = 2 * Math.PI * random();
      for (let k = 0; k < count; k++) {
        const at = spine[Math.round(((k + 0.5) / count) * (spine.length - 1))];
        const length = between(random, ...SHOOT.leafLength) * THREE.MathUtils.lerp(1, 0.6, k / count);
        leafAt(at, first + k * Math.PI + between(random, -0.3, 0.3), length, between(random, ...SHOOT.leafLean), SHOOT.bend);
      }
    };

    // A runner, node by node, with its leaves and shoots, and now and then a
    // side runner of its own.
    const runner = (start: THREE.Vector3, azimuth: number, length: number, side: boolean) => {
      const nodes = [start.clone()];
      const steps = Math.max(2, Math.round(length / RUNNERS.node));
      for (let k = 0; k < steps; k++) {
        azimuth += between(random, -RUNNERS.wander, RUNNERS.wander);
        const last = nodes[nodes.length - 1];
        const node = new THREE.Vector3(last.x + RUNNERS.node * Math.cos(azimuth), RUNNERS.y, last.z + RUNNERS.node * Math.sin(azimuth));
        nodes.push(node);
        for (const s of [-1, 1]) {
          const way = azimuth + s * (Math.PI / 2 + between(random, -0.6, 0.3));
          leafAt(node, way, between(random, ...NODE_LEAF.length), between(random, ...NODE_LEAF.lean), NODE_LEAF.bend);
        }
        if (random() < SHOOT.chance) shoot(node, between(random, ...SHOOT.height), between(random, ...SHOOT.lean));
        if (!side && k > 0 && random() < RUNNERS.branching) {
          const turn = (random() < 0.5 ? -1 : 1) * between(random, 0.6, 1.1);
          runner(node, azimuth + turn, between(random, ...RUNNERS.side), true);
        }
      }
      plan.leaves.tubes.push(tube(nodes, RUNNERS.radius, STEM, 0, 0.04, 5, runnerPaint));
    };

    const runners = crowded(RUNNERS.count, crowd);
    for (let r = 0; r < runners; r++) {
      const azimuth = (2 * Math.PI * (r + between(random, -0.3, 0.3))) / runners;
      const start = new THREE.Vector3(0.02 * Math.cos(azimuth), RUNNERS.y, 0.02 * Math.sin(azimuth));
      runner(start, azimuth, between(random, ...RUNNERS.length), false);
    }
    const spot = (radius: number, y: number) => {
      const out = radius * Math.sqrt(random());
      const round = 2 * Math.PI * random();
      return new THREE.Vector3(out * Math.cos(round), y, out * Math.sin(round));
    };
    for (let k = 0; k < crowded(MIDDLE.count, crowd); k++) {
      shoot(spot(MIDDLE.base, 0), between(random, ...MIDDLE.height), between(random, 0.05, 0.4));
    }

    // Flower stems, and the spikes at their tips.
    const stem = { foot: tone(c.grass, 0.65), tip: c.grass.clone().lerp(c.straw, 0.3) };
    const spikes = c.grass.clone().lerp(c.straw, 0.3).lerp(c.flower, 0.25);
    for (let k = 0; k < crowded(SPIKES.count, crowd); k++) {
      const spine = arch(spot(SPIKES.base, -0.01), heading(between(random, ...SPIKES.lean), 2 * Math.PI * random()), between(random, ...SPIKES.height), 0.05, 8);
      plan.stems.tubes.push(tube(spine, SPIKES.radius, STEM, 0, 0.03, 5, stem));
      const top = spine[spine.length - 1];
      const fingers = Math.round(between(random, ...SPIKES.fingers));
      const first = 2 * Math.PI * random();
      for (let f = 0; f < fingers; f++) {
        const azimuth = first + (2 * Math.PI * (f + between(random, -0.2, 0.2))) / fingers;
        const finger = arch(top, heading(between(random, ...SPIKES.spread), azimuth), between(random, ...SPIKES.length), 0.25, 6);
        const tint = between(random, 0.9, 1.08);
        const paint = { foot: tone(spikes, 0.85 * tint), tip: tone(spikes, tint) };
        plan.stems.tubes.push(tube(finger, SPIKES.fingerRadius, FINGER, SPIKES.bumps, 0.003, 6, paint));
      }
    }
    return plan;
  },
};

export class BermudaGrass extends THREE.Group {
  static readonly SIZE: Size = KIND.size;
  readonly leaves: Leaves;
  readonly stems: Stems;

  constructor(options: GrassOptions = {}) {
    super();
    this.name = 'bermuda grass';
    const parts = grow(options, KIND);
    this.leaves = parts.leaves;
    this.stems = parts.stems;
    this.add(this.leaves, this.stems);
  }
}
