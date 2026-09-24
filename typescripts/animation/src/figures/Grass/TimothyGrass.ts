import * as THREE from 'three';
import { grow, type Kind } from './grow';
import { arch, culms, endDirection, tube, tuft, type Culms, type Tuft } from './habit';
import type { Leaves } from './Leaves';
import { between, crowded, emptyPlan, tone, type GrassOptions, type Profile, type Size } from './parts';
import type { Stems } from './Stems';

// Timothy (Phleum pratense), about 95 cm tall and 50 cm across: a tuft of
// flat, grey-green leaves, and many stiff, upright stems with a leaf or two
// on the way up, each topped by its seed head, a dense, blunt cylinder 7–12
// cm long and under a centimeter thick. Leaves are the theme's grass colour,
// the heads ripening toward straw.
//
// Units are meters, y is up and the origin is on the ground at the middle of
// its foot. It grows from a seed and is sized to a height and width (see
// GrassOptions), so it is the same every time. The spec, TimothyGrass.md, is
// a draft: the grass has no behaviour and stands still.

const LEAVES: Tuft = {
  count: 45,
  base: 0.05,
  length: [0.2, 0.42],
  lean: [0.1, 0.65],
  bend: [0.4, 1.6],
  rows: 12,
  width: [0.006, 0.009],
  fold: 0.15,
};
const CULMS: Culms = {
  count: 11,
  base: 0.035,
  length: [0.62, 0.85],
  lean: [0.03, 0.22],
  bend: [0, 0.12],
  rows: 14,
  radius: 0.0035, // over twice a real stem's, which the painterly filter would wipe out
  leaves: { count: 2, at: [0.15, 0.55], length: [0.1, 0.2], lean: [0.3, 0.6], bend: 0.6, width: 0.006, fold: 0.15 },
};
const HEAD = { length: [0.07, 0.12] as const, radius: 0.0045, bumps: 0.18, row: 0.004, columns: 10 };
// A blunt cylinder, rounded off at both ends.
const HEAD_SHAPE: Profile = (u) => Math.min(1, Math.sqrt(u / 0.08), Math.sqrt((1 - u) / 0.06));

const KIND: Kind = {
  seed: 5,
  size: { height: 0.95, width: 0.5 },
  plan(c, random, crowd) {
    const plan = emptyPlan();
    const leaf = { foot: tone(c.grass, 0.55), tip: c.grass.clone().lerp(c.light, 0.05) };
    plan.leaves.blades = tuft({ ...LEAVES, count: crowded(LEAVES.count, crowd) }, leaf, random);

    const stem = { foot: tone(c.grass, 0.7), tip: c.grass.clone().lerp(c.straw, 0.4) };
    const { stems, leaves } = culms({ ...CULMS, count: crowded(CULMS.count, crowd) }, { stem, leaf }, random);
    plan.stems.tubes.push(...stems);
    plan.stems.blades.push(...leaves);
    for (const { spine } of stems) {
      const head = arch(spine[spine.length - 1], endDirection(spine), between(random, ...HEAD.length), 0, 6);
      const tint = between(random, 0.9, 1.08);
      const paint = { foot: tone(c.straw.clone().lerp(c.grass, 0.3), 0.85 * tint), tip: tone(c.straw, tint) };
      plan.stems.tubes.push(tube(head, HEAD.radius, HEAD_SHAPE, HEAD.bumps, HEAD.row, HEAD.columns, paint));
    }
    return plan;
  },
};

export class TimothyGrass extends THREE.Group {
  static readonly SIZE: Size = KIND.size;
  readonly leaves: Leaves;
  readonly stems: Stems;

  constructor(options: GrassOptions = {}) {
    super();
    this.name = 'timothy grass';
    const parts = grow(options, KIND);
    this.leaves = parts.leaves;
    this.stems = parts.stems;
    this.add(this.leaves, this.stems);
  }
}
