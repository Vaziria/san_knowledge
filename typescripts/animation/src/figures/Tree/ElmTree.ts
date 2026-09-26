import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Crown } from './Crown';
import { createMaterials, seededRandom, tipClumps, type TreeOptions } from './parts';
import { ellipsoid, fan, grow, wobble } from './skeleton';
import { Trunk } from './Trunk';

// An American elm (Ulmus americana), about 18 m tall with a crown 17 m across:
// the vase shape it is known for. The trunk splits low into several stems that
// rise steeply and fan out, and the leaves spread over them like an umbrella
// whose rim hangs down, leaving the space under it open. Bark is the theme's
// wood colour, leaves its grass colour.
//
// Units are meters, y is up and the origin is on the ground at the foot of the
// trunk (the roots reach a little below it). The tree grows from a seed (see
// TreeOptions and skeleton.ts), so it is the same every time. The spec,
// ElmTree.md, is empty so far: the tree has no behaviour and stands still.

const SEED = 41;
const FOLIAGE_SHADE = 0.95;
const TRUNK = [
  [0, -0.2, 0],
  [0.05, 1.5, 0],
  [0.1, 2.9, 0.05],
] as const;
const TRUNK_RADIUS = 0.42;
const FLARE = { amount: 0.7, height: 0.45, lobes: 5 };
const STEMS = { count: 5, spread: 4.8, rise: 7.5, bend: 0.3 }; // rising steeply first, spreading at the top
// The umbrella: an ellipsoid of leaves, open underneath.
const CROWN = { center: new THREE.Vector3(0, 13.4, 0), radii: new THREE.Vector3(8.4, 5.5, 8.4), bottom: 8.6, wobble: 0.08 };
const HOLLOW = { radius: 5.5, height: 13.2 }; // no leaves this close round the trunk below this height
const GROWTH = { points: 1000, step: 0.45, reach: 3.5, kill: 1, wander: 0.2, rise: 0.08, droop: 0.1, pipe: 2.5 };
const DROOP_FROM = 6; // m from the trunk: further out, branches bend down, making the rim hang
const CLUMP = { radius: [0.95, 1.3] as const, squash: 0.75 };

export class ElmTree extends THREE.Group {
  readonly trunk: Trunk;
  readonly crown: Crown;

  constructor(options: TreeOptions = {}) {
    super();
    this.name = 'elm tree';
    const m = createMaterials(options.theme ?? defaultTheme, FOLIAGE_SHADE, options.autumn);
    const random = seededRandom(options.seed ?? SEED);

    const trunk = TRUNK.map(([x, y, z]) => new THREE.Vector3(x, y, z));
    const lumps = wobble(CROWN.wobble, random);
    const reach = CROWN.radii.clone().multiplyScalar(1 + CROWN.wobble);
    const skeleton = grow(
      {
        trunk,
        limbs: fan(trunk[trunk.length - 1], STEMS.count, STEMS.spread, STEMS.rise, STEMS.bend, random),
        crown: (p) =>
          p.y > CROWN.bottom &&
          ellipsoid(p, CROWN.center, CROWN.radii) < lumps(p) &&
          !(p.y < HOLLOW.height && Math.hypot(p.x, p.z) < HOLLOW.radius),
        box: new THREE.Box3(CROWN.center.clone().sub(reach), CROWN.center.clone().add(reach)),
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
