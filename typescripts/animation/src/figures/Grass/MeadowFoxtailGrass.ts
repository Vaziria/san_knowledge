import * as THREE from 'three';
import { grow, type Kind } from './grow';
import { arch, culms, endDirection, tube, tuft, type Culms, type Tuft } from './habit';
import type { Leaves } from './Leaves';
import { between, crowded, emptyPlan, tone, type GrassOptions, type Profile, type Size } from './parts';
import type { Stems } from './Stems';

// Meadow foxtail (Alopecurus pratensis), about 80 cm tall and 50 cm across: a
// tuft of soft, arching leaves, and stems that lean out and nod a little at
// the top, each with a seed head like a fox's tail: 4–8 cm long, soft and
// silky, fullest below its middle and tapering to both ends. Beside timothy
// it is shorter, its heads shorter, fuller, softer and paler. Leaves are the
// theme's grass colour, the heads a pale straw.
//
// Units are meters, y is up and the origin is on the ground at the middle of
// its foot. It grows from a seed and is sized to a height and width (see
// GrassOptions), so it is the same every time. The spec,
// MeadowFoxtailGrass.md, is a draft: the grass has no behaviour and stands
// still.

const LEAVES: Tuft = {
  count: 45,
  base: 0.05,
  length: [0.15, 0.34],
  lean: [0.2, 0.8],
  bend: [0.6, 1.7],
  rows: 12,
  width: [0.005, 0.008],
  fold: 0.2,
};
const CULMS: Culms = {
  count: 9,
  base: 0.035,
  length: [0.5, 0.72],
  lean: [0.05, 0.32],
  bend: [0.1, 0.4],
  rows: 14,
  radius: 0.0032, // over twice a real stem's, which the painterly filter would wipe out
  leaves: { count: 2, at: [0.1, 0.5], length: [0.08, 0.16], lean: [0.3, 0.55], bend: 0.6, width: 0.0055, fold: 0.2 },
};
const HEAD = { length: [0.045, 0.08] as const, bend: 0.15, radius: 0.0078, bumps: 0.35, row: 0.004, columns: 10 };
// Soft, fullest below its middle and tapering to both ends.
const HEAD_SHAPE: Profile = (u) => Math.pow(Math.sin(Math.PI * Math.pow(u, 0.8)), 0.5);

const KIND: Kind = {
  seed: 13,
  size: { height: 0.8, width: 0.5 },
  plan(c, random, crowd) {
    const plan = emptyPlan();
    const leaf = { foot: tone(c.grass, 0.55), tip: tone(c.grass, 1.02) };
    plan.leaves.blades = tuft({ ...LEAVES, count: crowded(LEAVES.count, crowd) }, leaf, random);

    const stem = { foot: tone(c.grass, 0.7), tip: c.grass.clone().lerp(c.straw, 0.3) };
    const { stems, leaves } = culms({ ...CULMS, count: crowded(CULMS.count, crowd) }, { stem, leaf }, random);
    plan.stems.tubes.push(...stems);
    plan.stems.blades.push(...leaves);
    const silk = c.straw.clone().lerp(c.light, 0.18);
    for (const { spine } of stems) {
      const head = arch(spine[spine.length - 1], endDirection(spine), between(random, ...HEAD.length), HEAD.bend, 6);
      const tint = between(random, 0.92, 1.06);
      const paint = { foot: tone(silk, 0.85 * tint), tip: tone(silk, tint) };
      plan.stems.tubes.push(tube(head, HEAD.radius, HEAD_SHAPE, HEAD.bumps, HEAD.row, HEAD.columns, paint));
    }
    return plan;
  },
};

export class MeadowFoxtailGrass extends THREE.Group {
  static readonly SIZE: Size = KIND.size;
  readonly leaves: Leaves;
  readonly stems: Stems;

  constructor(options: GrassOptions = {}) {
    super();
    this.name = 'meadow foxtail grass';
    const parts = grow(options, KIND);
    this.leaves = parts.leaves;
    this.stems = parts.stems;
    this.add(this.leaves, this.stems);
  }
}
