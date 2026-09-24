import * as THREE from 'three';
import type { Limb } from '../Tree/skeleton';
import { grow, type Kind } from './grow';
import { arch, blade, heading, splay, type Range } from './habit';
import type { Leaves } from './Leaves';
import { between, crowded, emptyPlan, NEEDLE, tone, type GrassOptions, type Size } from './parts';
import type { Stems } from './Stems';

// Rosemary (Salvia rosmarinus), about 80 cm tall and 70 cm across: not a
// grass but an evergreen herb, a small bush of woody stems that lean out
// from its foot and turn back up, some branching, bare and barky low down
// and above that thick with narrow, leathery leaves like needles, 1–3 cm
// long, in pairs, each pair turned a quarter from the last, dark green above
// and pale beneath, the edges rolled under. The leaves shorten and lie closer
// to the stem toward its tip. Stems are bark in the theme's wood colour;
// leaves are its grass colour, darkened, their undersides toward its light
// colour. It has no flowers.
//
// Units are meters, y is up and the origin is on the ground at the middle of
// its foot. It grows from a seed and is sized to a height and width (see
// GrassOptions), so it is the same every time. The spec, RosemaryGrass.md,
// is a draft: the herb has no behaviour and stands still.

const STEMS = {
  count: 14,
  base: 0.05, // m round the foot
  length: [0.45, 0.8] as const, // m
  lean: [0.05, 0.55] as const, // rad from upright at the foot, the outer ones more
  rise: [0.3, 0.7] as const, // the share of that lean it turns back up by its tip
  rows: 10,
  radius: [0.005, 0.0015] as const, // m, at its foot and its tip
};
const BRANCHES = {
  count: [1, 3] as const, // on each stem
  at: [0.25, 0.75] as const, // how far up the stem
  length: [0.12, 0.28] as const, // m
  angle: [0.35, 0.65] as const, // rad off the stem
  rise: 0.3, // rad it turns back up by its tip
  rows: 6,
  radius: [0.003, 0.0012] as const,
};
const NEEDLES = {
  bare: 0.2, // share of a stem that is bare wood at its foot
  branchBare: 0.05, // and of a branch
  spacing: 0.0075, // m between pairs
  length: [0.032, 0.015] as const, // m, low on the leafy part and at the tip
  angle: [1.1, 0.5] as const, // rad off the stem, low and at the tip
  width: 0.005, // a little wider than a real leaf, which the painterly filter would thin out
  fold: -0.3, // edges rolled under
  bend: 0.25,
  rows: 3,
};
const UNDERSIDE = 0.35; // how far the leaves' undersides are toward the light colour

const UP = new THREE.Vector3(0, 1, 0);
const X = new THREE.Vector3(1, 0, 0);

const KIND: Kind = {
  seed: 31,
  size: { height: 0.8, width: 0.7 },
  plan(c, random, crowd) {
    const plan = emptyPlan();
    const top = { foot: tone(c.grass, 0.55), tip: tone(c.grass, 0.7) };
    const paint = { ...top, back: [top.foot.clone().lerp(c.light, UNDERSIDE), top.tip.clone().lerp(c.light, UNDERSIDE)] as [THREE.Color, THREE.Color] };

    const limb = (points: THREE.Vector3[], radius: Range): Limb => ({
      points,
      radii: points.map((_, i) => THREE.MathUtils.lerp(radius[0], radius[1], i / (points.length - 1))),
    });

    // Pairs of leaves up a stem from where it stops being bare, each pair a
    // quarter turn from the last.
    const needles = (spine: THREE.Vector3[], radius: Range, bare: number) => {
      const curve = new THREE.CatmullRomCurve3(spine);
      const count = Math.floor((curve.getLength() * (1 - bare)) / NEEDLES.spacing);
      for (let k = 0; k < count; k++) {
        const tipward = (k + 0.5) / count;
        const u = bare + (1 - bare) * tipward;
        const at = curve.getPointAt(u);
        const along = curve.getTangentAt(u);
        const a = new THREE.Vector3().crossVectors(along, Math.abs(along.y) < 0.9 ? UP : X).normalize();
        const b = new THREE.Vector3().crossVectors(along, a);
        const turn = (k * Math.PI) / 2 + between(random, -0.25, 0.25);
        for (const half of [0, Math.PI]) {
          const out = a.clone().multiplyScalar(Math.cos(turn + half)).addScaledVector(b, Math.sin(turn + half));
          const angle = THREE.MathUtils.lerp(NEEDLES.angle[0], NEEDLES.angle[1], tipward) * between(random, 0.85, 1.15);
          const length = THREE.MathUtils.lerp(NEEDLES.length[0], NEEDLES.length[1], tipward * tipward) * between(random, 0.85, 1.15);
          const direction = along.clone().multiplyScalar(Math.cos(angle)).addScaledVector(out, Math.sin(angle));
          const start = at.clone().addScaledVector(out, 0.8 * THREE.MathUtils.lerp(radius[0], radius[1], u));
          const spine = arch(start, direction, length, NEEDLES.bend, NEEDLES.rows);
          const side = new THREE.Vector3().crossVectors(out, along).normalize();
          plan.leaves.blades.push(blade(spine, side, NEEDLES.width, NEEDLES.fold, NEEDLE, paint, random));
        }
      }
    };

    for (let k = 0; k < crowded(STEMS.count, crowd); k++) {
      const out = Math.sqrt(random());
      const round = 2 * Math.PI * random();
      const foot = new THREE.Vector3(STEMS.base * out * Math.cos(round), -0.01, STEMS.base * out * Math.sin(round));
      const lean = THREE.MathUtils.lerp(STEMS.lean[0], STEMS.lean[1], 0.4 * out + 0.6 * random());
      const azimuth = round + between(random, -0.5, 0.5);
      const spine = arch(foot, heading(lean, azimuth), between(random, ...STEMS.length), -between(random, ...STEMS.rise) * lean, STEMS.rows);
      plan.wood.push(limb(spine, STEMS.radius));
      needles(spine, STEMS.radius, NEEDLES.bare);

      const branches = Math.round(between(random, ...BRANCHES.count));
      for (let n = 0; n < branches; n++) {
        const i = Math.round(between(random, ...BRANCHES.at) * (spine.length - 2));
        const way = spine[i + 1].clone().sub(spine[i]).normalize();
        // Off the stem, but upward: never back down into the bush.
        let direction = splay(way, between(random, ...BRANCHES.angle), random);
        for (let tries = 0; direction.y < 0.4 && tries < 8; tries++) direction = splay(way, between(random, ...BRANCHES.angle), random);
        const branch = arch(spine[i], direction, between(random, ...BRANCHES.length), -BRANCHES.rise, BRANCHES.rows);
        plan.wood.push(limb(branch, BRANCHES.radius));
        needles(branch, BRANCHES.radius, NEEDLES.branchBare);
      }
    }
    return plan;
  },
};

export class RosemaryGrass extends THREE.Group {
  static readonly SIZE: Size = KIND.size;
  readonly leaves: Leaves;
  readonly stems: Stems;

  constructor(options: GrassOptions = {}) {
    super();
    this.name = 'rosemary';
    const parts = grow(options, KIND);
    this.leaves = parts.leaves;
    this.stems = parts.stems;
    this.add(this.leaves, this.stems);
  }
}
