import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Crown } from './Crown';
import { createMaterials, seededRandom, tipClumps, type TreeOptions } from './parts';
import { ellipsoid, fan, grow, wobble } from './skeleton';
import { Trunk } from './Trunk';

// An English oak (Quercus robur), about 13 m tall with a crown 16 m across: a
// short, thick trunk swelling into buttress roots at the ground, splitting low
// into a few massive limbs that spread wide, crooked wood, and a broad, lumpy
// dome of leaves. Bark is the theme's wood colour, leaves its grass colour.
//
// Units are meters, y is up and the origin is on the ground at the foot of the
// trunk (the roots reach a little below it). The tree grows from a seed (see
// TreeOptions and skeleton.ts), so it is the same every time. The spec,
// OakTree.md, is empty so far: the tree has no behaviour and stands still.

const SEED = 11;
const FOLIAGE_SHADE = 0.95;
const TRUNK = [
  [0, -0.2, 0],
  [0.08, 1.3, 0.05],
  [0.2, 2.6, -0.08],
] as const;
const TRUNK_RADIUS = 0.5;
const FLARE = { amount: 0.8, height: 0.45, lobes: 5 };
const LIMBS = { count: 4, spread: 4.2, rise: 2.4, bend: 0.8 }; // spreading wide first, then rising
const CROWN = { center: new THREE.Vector3(0, 7.4, 0), radii: new THREE.Vector3(7.8, 4.6, 7.8), bottom: 3.4, wobble: 0.12 };
const GROWTH = { points: 900, step: 0.45, reach: 4.5, kill: 1.2, wander: 0.3, rise: 0.04, pipe: 2.4 };
const CLUMP = { radius: [0.9, 1.35] as const, squash: 0.8 };

export class OakTree extends THREE.Group {
  readonly trunk: Trunk;
  readonly crown: Crown;

  constructor(options: TreeOptions = {}) {
    super();
    this.name = 'oak tree';
    const m = createMaterials(options.theme ?? defaultTheme, FOLIAGE_SHADE);
    const random = seededRandom(options.seed ?? SEED);

    const trunk = TRUNK.map(([x, y, z]) => new THREE.Vector3(x, y, z));
    const lumps = wobble(CROWN.wobble, random);
    const reach = CROWN.radii.clone().multiplyScalar(1 + CROWN.wobble);
    const skeleton = grow(
      {
        trunk,
        limbs: fan(trunk[trunk.length - 1], LIMBS.count, LIMBS.spread, LIMBS.rise, LIMBS.bend, random),
        crown: (p) => p.y > CROWN.bottom && ellipsoid(p, CROWN.center, CROWN.radii) < lumps(p),
        box: new THREE.Box3(CROWN.center.clone().sub(reach), CROWN.center.clone().add(reach)),
        points: GROWTH.points,
        step: GROWTH.step,
        reach: GROWTH.reach,
        kill: GROWTH.kill,
        wander: GROWTH.wander,
        tropism: () => new THREE.Vector3(0, GROWTH.rise, 0),
        radius: TRUNK_RADIUS,
        pipe: GROWTH.pipe,
      },
      random,
    );

    this.trunk = new Trunk(m, skeleton.limbs, FLARE);
    this.crown = new Crown(m, tipClumps(skeleton.tips, CLUMP.radius, CLUMP.squash, random), random);
    this.add(this.trunk, this.crown);
  }
}
