import * as THREE from 'three';
import { grow, type Kind } from './grow';
import { along, arch, culms, endDirection, splay, tube, tuft, type Culms, type Tuft } from './habit';
import type { Leaves } from './Leaves';
import { between, crowded, emptyPlan, STEM, THREAD, tone, type GrassOptions, type Size } from './parts';
import type { Stems } from './Stems';

// Red fescue (Festuca rubra), about 55 cm tall and 45 cm across: a dense,
// fine tuft of thread-like leaves, folded along their length, that arch over
// in a soft mop, reddish at their feet where the sheaths are (the "red" in
// its name), and slender stems whose loose seed heads (panicles) nod over,
// a few short branches each carrying small spikelets tinged red-purple.
// Leaves are the theme's grass colour, toward its wood colour at their feet;
// the spikelets are green toward wood, with a touch of the flower colour.
//
// Units are meters, y is up and the origin is on the ground at the middle of
// its foot. It grows from a seed and is sized to a height and width (see
// GrassOptions), so it is the same every time. The spec, RedFescueGrass.md,
// is a draft: the grass has no behaviour and stands still.

const LEAVES: Tuft = {
  count: 150,
  base: 0.05,
  length: [0.12, 0.3],
  lean: [0.1, 0.9],
  bend: [0.6, 1.8],
  rows: 10,
  width: [0.0028, 0.0035], // three times a real leaf's, which the painterly filter would wipe out
  fold: 0.6,
  profile: THREAD,
};
const CULMS: Culms = {
  count: 9,
  base: 0.035,
  length: [0.3, 0.45],
  lean: [0.08, 0.35],
  bend: [0.1, 0.4],
  rows: 12,
  radius: 0.0025, // over twice a real stem's
  leaves: { count: 1, at: [0.1, 0.35], length: [0.05, 0.1], lean: [0.2, 0.4], bend: 0.3, width: 0.003, fold: 0.6 },
};
const PANICLE = {
  length: [0.06, 0.1] as const, // m, its axis
  bend: 0.8, // rad it nods over by its tip
  radius: 0.0018,
  branches: [5, 7] as const,
  branchLength: [0.012, 0.03] as const, // m, the lowest the longest
  angle: [0.25, 0.55] as const, // rad off the axis
  spikelets: [2, 3] as const, // on each branch
  spikelet: new THREE.Vector3(0.004, 0.009, 0.004), // m, its radii
};

const KIND: Kind = {
  seed: 19,
  size: { height: 0.55, width: 0.45 },
  plan(c, random, crowd) {
    const plan = emptyPlan();
    const sheath = tone(c.grass, 0.6).lerp(c.wood, 0.5);
    const leaf = { foot: sheath, tip: tone(c.grass, 0.97) };
    plan.leaves.blades = tuft({ ...LEAVES, count: crowded(LEAVES.count, crowd) }, leaf, random);

    const stem = { foot: sheath, tip: c.grass.clone().lerp(c.straw, 0.4) };
    const { stems, leaves } = culms({ ...CULMS, count: crowded(CULMS.count, crowd) }, { stem, leaf }, random);
    plan.stems.tubes.push(...stems);
    plan.stems.blades.push(...leaves);

    const spikeletColor = c.grass.clone().lerp(c.wood, 0.45).lerp(c.flower, 0.2);
    const spikelet = (center: THREE.Vector3, direction: THREE.Vector3) =>
      plan.stems.blobs.push({
        center,
        size: PANICLE.spikelet.clone().multiplyScalar(between(random, 0.85, 1.15)),
        turn: along(direction),
        color: tone(spikeletColor, between(random, 0.9, 1.08)),
      });
    for (const { spine } of stems) {
      const axis = arch(spine[spine.length - 1], endDirection(spine), between(random, ...PANICLE.length), PANICLE.bend, 8);
      plan.stems.tubes.push(tube(axis, PANICLE.radius, STEM, 0, 0.015, 5, stem));
      spikelet(axis[axis.length - 1], endDirection(axis));

      // Branches up the axis, shorter higher up, with spikelets along each.
      const count = Math.round(between(random, ...PANICLE.branches));
      for (let k = 0; k < count; k++) {
        const u = (0.85 * k) / count;
        const i = Math.round(u * (axis.length - 2));
        const way = axis[i + 1].clone().sub(axis[i]).normalize();
        const length = THREE.MathUtils.lerp(PANICLE.branchLength[1], PANICLE.branchLength[0], u) * between(random, 0.8, 1.2);
        const branch = arch(axis[i], splay(way, between(random, ...PANICLE.angle), random), length, 0.3, 3);
        plan.stems.tubes.push(tube(branch, 0.7 * PANICLE.radius, STEM, 0, 0.01, 4, stem));
        const on = Math.round(between(random, ...PANICLE.spikelets));
        for (let s = 0; s < on; s++) {
          const at = branch[branch.length - 1 - s];
          spikelet(at.clone(), endDirection(branch));
        }
      }
    }
    return plan;
  },
};

export class RedFescueGrass extends THREE.Group {
  static readonly SIZE: Size = KIND.size;
  readonly leaves: Leaves;
  readonly stems: Stems;

  constructor(options: GrassOptions = {}) {
    super();
    this.name = 'red fescue grass';
    const parts = grow(options, KIND);
    this.leaves = parts.leaves;
    this.stems = parts.stems;
    this.add(this.leaves, this.stems);
  }
}
