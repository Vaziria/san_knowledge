import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Crown } from './Crown';
import { createMaterials, seededRandom, tipClumps, type TreeOptions } from './parts';
import { ellipsoid, fan, grow, wobble } from './skeleton';
import { Trunk } from './Trunk';

// A horse chestnut (Aesculus hippocastanum), about 16 m tall with a crown 13 m
// across: a stout trunk and a tall, dense dome of big leaves whose lower
// branches sweep down and out, low over the ground at the rim, leaving the
// trunk clear beneath. Its foliage is a darker green than the maple's. Bark is
// the theme's wood colour, leaves its grass colour.
//
// Units are meters, y is up and the origin is on the ground at the foot of the
// trunk (the roots reach a little below it). The tree grows from a seed (see
// TreeOptions and skeleton.ts), so it is the same every time. The spec,
// ChestnutTree.md, is empty so far: the tree has no behaviour and stands still.

const SEED = 31;
const FOLIAGE_SHADE = 0.88;
const TRUNK = [
  [0, -0.2, 0],
  [-0.05, 1.6, 0.05],
  [0.05, 3.0, 0],
] as const;
const TRUNK_RADIUS = 0.45;
const FLARE = { amount: 0.6, height: 0.45, lobes: 5 };
const LIMBS = { count: 4, spread: 2.6, rise: 3.0, bend: 0.55 };
// The dome: an ellipsoid whose lower half is deeper than its upper half, so
// its sides come down steeply, hollowed from below by a bowl that is high
// round the trunk and low at the rim.
const CROWN = { center: new THREE.Vector3(0, 9.5, 0), width: 7, top: 6.3, below: 7.5, wobble: 0.08 };
const HOLLOW = { middle: 6.2, rim: 3.0 }; // the bowl's height round the trunk and at the crown's radius, m
const GROWTH = { points: 1200, step: 0.45, reach: 4, kill: 1.1, wander: 0.2, rise: 0.06, droop: 0.08, pipe: 2.5 };
const DROOP_FROM = 4; // m from the trunk: further out, branches bend down instead of up
const CLUMP = { radius: [1.05, 1.45] as const, squash: 0.85 };

export class ChestnutTree extends THREE.Group {
  readonly trunk: Trunk;
  readonly crown: Crown;

  constructor(options: TreeOptions = {}) {
    super();
    this.name = 'chestnut tree';
    const m = createMaterials(options.theme ?? defaultTheme, FOLIAGE_SHADE, options.autumn);
    const random = seededRandom(options.seed ?? SEED);

    const trunk = TRUNK.map(([x, y, z]) => new THREE.Vector3(x, y, z));
    const lumps = wobble(CROWN.wobble, random);
    const upper = new THREE.Vector3(CROWN.width, CROWN.top, CROWN.width);
    const lower = new THREE.Vector3(CROWN.width, CROWN.below, CROWN.width);
    const hollow = (r: number) => HOLLOW.middle - (HOLLOW.middle - HOLLOW.rim) * (r / CROWN.width) ** 2;
    const reach = CROWN.width * (1 + CROWN.wobble);
    const skeleton = grow(
      {
        trunk,
        limbs: fan(trunk[trunk.length - 1], LIMBS.count, LIMBS.spread, LIMBS.rise, LIMBS.bend, random),
        crown: (p) =>
          ellipsoid(p, CROWN.center, p.y > CROWN.center.y ? upper : lower) < lumps(p) && p.y > hollow(Math.hypot(p.x, p.z)),
        box: new THREE.Box3(
          new THREE.Vector3(-reach, HOLLOW.rim, -reach),
          new THREE.Vector3(reach, CROWN.center.y + CROWN.top * (1 + CROWN.wobble), reach),
        ),
        points: GROWTH.points,
        step: GROWTH.step,
        reach: GROWTH.reach,
        kill: GROWTH.kill,
        wander: GROWTH.wander,
        tropism: (p) => new THREE.Vector3(0, Math.hypot(p.x, p.z) > DROOP_FROM ? -GROWTH.droop : GROWTH.rise, 0),
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
