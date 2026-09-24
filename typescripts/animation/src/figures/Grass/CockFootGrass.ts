import * as THREE from 'three';
import { grow, type Kind } from './grow';
import { along, arch, culms, endDirection, splay, tube, tuft, type Culms, type Tuft } from './habit';
import type { Leaves } from './Leaves';
import { between, crowded, emptyPlan, STEM, tone, type GrassOptions, type Size } from './parts';
import type { Stems } from './Stems';

// Cock's-foot (Dactylis glomerata), about 90 cm tall and 70 cm across: a
// coarse tussock of broad, keeled (V-folded), blue-green leaves that arch
// over, and stems rising out of it, each topped by its seed head (a
// panicle): two or three stiff branches spread from its foot like a bird's
// toes, each ending in a dense cluster of spikelets, and more clusters crowd
// one side of the axis above them. Leaves are the theme's grass colour,
// lightened a little; the clusters are green ripening toward straw, with a
// touch of the flower colour, as real ones are tinged purple.
//
// Units are meters, y is up and the origin is on the ground at the middle of
// its foot. It grows from a seed and is sized to a height and width (see
// GrassOptions), so it is the same every time. The spec, CockFootGrass.md,
// is a draft: the grass has no behaviour and stands still.

const LEAVES: Tuft = {
  count: 60,
  base: 0.06,
  length: [0.25, 0.5],
  lean: [0.15, 0.8],
  bend: [0.6, 1.8],
  rows: 12,
  width: [0.008, 0.013],
  fold: 0.35,
};
const CULMS: Culms = {
  count: 7,
  base: 0.04,
  length: [0.6, 0.8],
  lean: [0.05, 0.3],
  bend: [0, 0.25],
  rows: 14,
  radius: 0.0035, // over twice a real stem's, which the painterly filter would wipe out
  leaves: { count: 2, at: [0.15, 0.5], length: [0.12, 0.22], lean: [0.35, 0.7], bend: 0.8, width: 0.008, fold: 0.35 },
};
const PANICLE = {
  length: [0.08, 0.13] as const, // m, its axis above the toes
  bend: 0.15,
  radius: 0.0025,
  toes: [2, 3] as const, // branches spreading from its foot
  toeLength: [0.035, 0.07] as const, // m
  toeAngle: [0.6, 1] as const, // rad off the axis
  clusters: [3, 5] as const, // on the axis above the toes, the last at its tip
  cluster: new THREE.Vector3(0.01, 0.017, 0.01), // m, the radii of a cluster
  offset: 0.005, // m a cluster sits to one side of the axis
};

const KIND: Kind = {
  seed: 17,
  size: { height: 0.9, width: 0.7 },
  plan(c, random, crowd) {
    const plan = emptyPlan();
    const blueGreen = c.grass.clone().lerp(c.light, 0.08);
    const leaf = { foot: tone(c.grass, 0.55), tip: blueGreen };
    plan.leaves.blades = tuft({ ...LEAVES, count: crowded(LEAVES.count, crowd) }, leaf, random);

    const stem = { foot: tone(c.grass, 0.7), tip: c.grass.clone().lerp(c.straw, 0.4) };
    const { stems, leaves } = culms({ ...CULMS, count: crowded(CULMS.count, crowd) }, { stem, leaf }, random);
    plan.stems.tubes.push(...stems);
    plan.stems.blades.push(...leaves);

    const clusterColor = c.straw.clone().lerp(c.grass, 0.4).lerp(c.flower, 0.15);
    const cluster = (center: THREE.Vector3, direction: THREE.Vector3, scale: number) =>
      plan.stems.blobs.push({
        center,
        size: PANICLE.cluster.clone().multiplyScalar(scale),
        turn: along(direction),
        color: tone(clusterColor, between(random, 0.9, 1.08)),
      });
    for (const { spine } of stems) {
      const foot = spine[spine.length - 1];
      const up = endDirection(spine);
      const axis = arch(foot, up, between(random, ...PANICLE.length), PANICLE.bend, 6);
      plan.stems.tubes.push(tube(axis, PANICLE.radius, STEM, 0, 0.02, 5, stem));

      // The toes, each with its cluster over its outer half.
      const toes = Math.round(between(random, ...PANICLE.toes));
      for (let k = 0; k < toes; k++) {
        const toe = arch(foot, splay(up, between(random, ...PANICLE.toeAngle), random), between(random, ...PANICLE.toeLength), 0.2, 3);
        plan.stems.tubes.push(tube(toe, 0.8 * PANICLE.radius, STEM, 0, 0.02, 5, stem));
        const out = endDirection(toe);
        cluster(toe[toe.length - 1].clone().addScaledVector(out, -0.5 * PANICLE.cluster.y), out, between(random, 0.8, 1.15));
      }

      // Clusters up one side of the axis, the one at the tip the biggest.
      const side = splay(up, Math.PI / 2, random).multiplyScalar(PANICLE.offset);
      const count = Math.round(between(random, ...PANICLE.clusters));
      for (let k = 0; k < count; k++) {
        const u = 0.3 + (0.7 * (k + 1)) / count;
        const i = Math.round(u * (axis.length - 1));
        const last = k === count - 1;
        const at = axis[i].clone().add(last ? side.clone().multiplyScalar(0.3) : side);
        cluster(at, up, last ? 1.15 : between(random, 0.7, 0.95));
      }
    }
    return plan;
  },
};

export class CockFootGrass extends THREE.Group {
  static readonly SIZE: Size = KIND.size;
  readonly leaves: Leaves;
  readonly stems: Stems;

  constructor(options: GrassOptions = {}) {
    super();
    this.name = "cock's-foot grass";
    const parts = grow(options, KIND);
    this.leaves = parts.leaves;
    this.stems = parts.stems;
    this.add(this.leaves, this.stems);
  }
}
