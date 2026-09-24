import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Crown } from './Crown';
import { createMaterials, seededRandom, tipClumps, type TreeOptions } from './parts';
import { ellipsoid, fan, grow, wobble } from './skeleton';
import { Trunk } from './Trunk';

// A sugar maple (Acer saccharum), about 15 m tall with a crown 11 m across:
// a straight trunk, branches rising steeply, and a dense crown shaped like an
// upright egg with a fairly smooth outline. Bark is the theme's wood colour,
// leaves its grass colour.
//
// Units are meters, y is up and the origin is on the ground at the foot of the
// trunk (the roots reach a little below it). The tree grows from a seed (see
// TreeOptions and skeleton.ts), so it is the same every time. The spec,
// MapleTree.md, is empty so far: the tree has no behaviour and stands still.

const SEED = 23;
const FOLIAGE_SHADE = 1;
const TRUNK = [
  [0, -0.2, 0],
  [0.05, 1.8, 0.03],
  [0, 3.6, 0.08],
] as const;
const TRUNK_RADIUS = 0.34;
const FLARE = { amount: 0.5, height: 0.4, lobes: 6 };
const LIMBS = { count: 3, spread: 1.6, rise: 2.8, bend: 0.4 }; // rising steeply
const CROWN = { center: new THREE.Vector3(0, 9.3, 0), radii: new THREE.Vector3(5.4, 5.9, 5.4), bottom: 4.2, wobble: 0.07 };
const GROWTH = { points: 1100, step: 0.4, reach: 3.5, kill: 0.95, wander: 0.15, rise: 0.15, pipe: 2.5 };
const CLUMP = { radius: [0.8, 1.15] as const, squash: 0.85 };

export class MapleTree extends THREE.Group {
  readonly trunk: Trunk;
  readonly crown: Crown;

  constructor(options: TreeOptions = {}) {
    super();
    this.name = 'maple tree';
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
