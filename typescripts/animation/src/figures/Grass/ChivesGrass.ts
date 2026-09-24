import * as THREE from 'three';
import { grow, type Kind } from './grow';
import { along, endDirection, sprouts, tube, type Sprouts } from './habit';
import type { Leaves } from './Leaves';
import { between, crowded, emptyPlan, randomUnit, STEM, tone, type GrassOptions, type Profile, type Size } from './parts';
import type { Stems } from './Stems';

// Chives (Allium schoenoprasum), about 45 cm tall and 30 cm across: not a
// true grass but a grass-like herb, a clump of hollow, round leaves that
// stand almost upright and taper to a point, and a few stiff flower stalks
// (scapes) as tall as the leaves, each topped by a round pom-pom of small
// florets, 2.5–3 cm across. Leaves are the theme's grass colour, the flowers
// its flower colour.
//
// Units are meters, y is up and the origin is on the ground at the middle of
// its foot. It grows from a seed and is sized to a height and width (see
// GrassOptions), so it is the same every time. The spec, ChivesGrass.md, is
// a draft: the herb has no behaviour and stands still.

const LEAVES: Sprouts & { radius: number } = {
  count: 40,
  base: 0.03,
  length: [0.22, 0.38],
  lean: [0.03, 0.35],
  bend: [0.1, 0.8],
  rows: 10,
  radius: 0.003, // a little thicker than a real leaf, which the painterly filter would thin out
};
// A round leaf, tapering to a point over its last third.
const LEAF_SHAPE: Profile = (u) => (u < 0.65 ? 1 : Math.pow((1 - u) / 0.35, 0.7));
const SCAPES: Sprouts & { radius: number } = {
  count: 7,
  base: 0.02,
  length: [0.3, 0.4],
  lean: [0.02, 0.22],
  bend: [0, 0.12],
  rows: 10,
  radius: 0.0022,
};
const FLOWER = {
  radius: [0.012, 0.016] as const, // m
  florets: 20,
  core: 0.7, // the ball the florets sit on, as a share of the flower's radius
  floret: new THREE.Vector3(0.34, 0.5, 0.34), // its radii, as a share of the flower's radius
  below: -0.35, // florets point no further down than this (the y of their direction), clear of the stalk
};

const KIND: Kind = {
  seed: 23,
  size: { height: 0.45, width: 0.3 },
  plan(c, random, crowd) {
    const plan = emptyPlan();
    for (const { spine } of sprouts({ ...LEAVES, count: crowded(LEAVES.count, crowd) }, random)) {
      const tint = between(random, 0.9, 1.08);
      const paint = { foot: tone(c.grass, 0.6 * tint), tip: tone(c.grass, 0.95 * tint) };
      plan.leaves.tubes.push(tube(spine, LEAVES.radius, LEAF_SHAPE, 0, 0.02, 7, paint));
    }

    const stalk = { foot: tone(c.grass, 0.65), tip: tone(c.grass, 0.95) };
    for (const { spine } of sprouts({ ...SCAPES, count: crowded(SCAPES.count, crowd) }, random, -0.01)) {
      plan.stems.tubes.push(tube(spine, SCAPES.radius, STEM, 0, 0.03, 6, stalk));
      // The flower: a ball of florets round a core, above the stalk's tip.
      const r = between(random, ...FLOWER.radius);
      const center = spine[spine.length - 1].clone().addScaledVector(endDirection(spine), 0.6 * r);
      plan.stems.blobs.push({ center, size: new THREE.Vector3().setScalar(FLOWER.core * r), color: tone(c.flower, 0.8) });
      for (let k = 0; k < FLOWER.florets; k++) {
        let way = randomUnit(random);
        for (let tries = 0; way.y < FLOWER.below && tries < 10; tries++) way = randomUnit(random);
        plan.stems.blobs.push({
          center: center.clone().addScaledVector(way, FLOWER.core * r),
          size: FLOWER.floret.clone().multiplyScalar(r),
          turn: along(way),
          color: tone(c.flower, between(random, 0.9, 1.1)),
        });
      }
    }
    return plan;
  },
};

export class ChivesGrass extends THREE.Group {
  static readonly SIZE: Size = KIND.size;
  readonly leaves: Leaves;
  readonly stems: Stems;

  constructor(options: GrassOptions = {}) {
    super();
    this.name = 'chives';
    const parts = grow(options, KIND);
    this.leaves = parts.leaves;
    this.stems = parts.stems;
    this.add(this.leaves, this.stems);
  }
}
