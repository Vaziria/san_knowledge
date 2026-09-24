import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Crown } from './Crown';
import { between, createMaterials, seededRandom, type Clump, type TreeOptions } from './parts';
import type { Limb } from './skeleton';
import { Trunk } from './Trunk';

// A Norway spruce (Picea abies), 20 m tall and 7 m across at the bottom: a
// narrow cone, dense to the ground. A straight trunk runs to the very top, and
// round it whorls of branches, a few every 60 cm, grow shorter toward
// the top; they reach out, droop and turn up at their tips, the lowest ones
// sweeping down like a skirt. Each branch carries a drooping pad of needles.
// Needles are a dark green. Bark is the theme's wood colour, needles its grass
// colour.
//
// Unlike the other trees, a spruce does not grow its branches toward the
// light (skeleton.ts): its whorls are regular enough to build directly.
//
// Units are meters, y is up and the origin is on the ground at the foot of the
// trunk (the roots reach a little below it). The tree is built from a seed
// (see TreeOptions), so it is the same every time. The spec, SpruceTree.md, is
// empty so far: the tree has no behaviour and stands still.

const SEED = 83;
const FOLIAGE_SHADE = 0.7;
const HEIGHT = 20;
const TRUNK_RADIUS = 0.32;
const TOP_RADIUS = 0.03;
const CROOK = 0.08; // m the trunk strays from straight
const FLARE = { amount: 0.4, height: 0.35, lobes: 6 };

const WHORLS = { lowest: 0.9, highest: 19.2, spacing: 0.6, branches: [4, 6] as const };
const BRANCH = {
  bottom: 3.6, // m long at the bottom, making the cone
  top: 0.3, // m long at the top
  jitter: 0.15, // each is up to this share longer or shorter
  droop: [0.55, 0.2] as const, // how far it sinks, as a share of its length, at the bottom and at the top
  upturn: 0.2, // how far its tip turns up again, as a share of its length
  radius: { base: 0.02, perMeter: 0.02, tip: 0.008 },
};
// A branch's pad of needles runs from PAD.from to past its tip, as shares of
// its length: long and narrow, so the whorls stay ragged rather than closing
// into discs.
const PAD = { from: 0.15, to: 1.08, width: { base: 0.22, perMeter: 0.14 }, depth: { base: 0.25, perMeter: 0.08 } };
const LEADER_CLUMP = new THREE.Vector3(0.25, 0.8, 0.25); // the tuft at the very top
const LEADER_DROP = 0.5; // m below the top that the tuft is centered

const UP = new THREE.Vector3(0, 1, 0);

export class SpruceTree extends THREE.Group {
  readonly trunk: Trunk;
  readonly crown: Crown;

  constructor(options: TreeOptions = {}) {
    super();
    this.name = 'spruce tree';
    const m = createMaterials(options.theme ?? defaultTheme, FOLIAGE_SHADE);
    const random = seededRandom(options.seed ?? SEED);

    // The trunk, tapering evenly to the top, with a slight crook.
    const crook = [0, 1].map(() => 2 * Math.PI * random());
    const axis = (y: number) =>
      new THREE.Vector3(CROOK * Math.sin(y * 0.3 + crook[0]), y, CROOK * Math.sin(y * 0.23 + crook[1]));
    const trunkPoints = Array.from({ length: 11 }, (_, i) => axis(-0.2 + (i / 10) * (HEIGHT + 0.2)));
    const limbs: Limb[] = [
      { points: trunkPoints, radii: trunkPoints.map((p) => THREE.MathUtils.lerp(TRUNK_RADIUS, TOP_RADIUS, Math.max(0, p.y) / HEIGHT)) },
    ];
    const clumps: Clump[] = [{ center: axis(HEIGHT - LEADER_DROP), size: LEADER_CLUMP }];

    // The whorls, from the bottom up, each turned from the last.
    let turn = 0;
    for (let y = WHORLS.lowest; y <= WHORLS.highest; y += WHORLS.spacing) {
      const h = (y - WHORLS.lowest) / (WHORLS.highest - WHORLS.lowest); // 0 at the lowest whorl, 1 at the highest
      const count = Math.round(between(random, WHORLS.branches[0], WHORLS.branches[1] + 0.49));
      turn += 2.4; // about the golden angle, so branches of whorls in turn don't line up
      for (let i = 0; i < count; i++) {
        const angle = turn + ((i + between(random, -0.2, 0.2)) / count) * 2 * Math.PI;
        const length = THREE.MathUtils.lerp(BRANCH.bottom, BRANCH.top, h) * (1 + between(random, -1, 1) * BRANCH.jitter);
        const droop = THREE.MathUtils.lerp(BRANCH.droop[0], BRANCH.droop[1], h);
        const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
        const base = axis(y);
        // Out along `out`, sinking by `droop` and turning up at the tip.
        const at = (t: number) =>
          base
            .clone()
            .addScaledVector(out, t * length)
            .addScaledVector(UP, length * (-droop * t + BRANCH.upturn * t * t * t));
        const points = [0, 0.33, 0.66, 1].map(at);
        const r = BRANCH.radius.base + BRANCH.radius.perMeter * length;
        limbs.push({ points, radii: [r, r * 0.7, r * 0.4, BRANCH.radius.tip] });

        // The pad of needles along it, lying along the branch as it droops.
        const from = at(PAD.from);
        const to = at(PAD.to);
        const along = to.clone().sub(from).normalize();
        const side = new THREE.Vector3().crossVectors(along, UP).normalize();
        const up = new THREE.Vector3().crossVectors(side, along);
        clumps.push({
          center: from.clone().lerp(to, 0.5),
          size: new THREE.Vector3(
            (from.distanceTo(to) / 2) * 1.1,
            PAD.depth.base + PAD.depth.perMeter * length,
            PAD.width.base + PAD.width.perMeter * length,
          ),
          turn: new THREE.Quaternion().setFromRotationMatrix(new THREE.Matrix4().makeBasis(along, up, side)),
        });
      }
    }

    this.trunk = new Trunk(m, limbs, FLARE);
    this.crown = new Crown(m, clumps, random);
    this.add(this.trunk, this.crown);
  }
}
