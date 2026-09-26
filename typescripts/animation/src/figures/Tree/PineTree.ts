import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Crown } from './Crown';
import { between, createMaterials, seededRandom, tipClumps, type TreeOptions } from './parts';
import { ellipsoid, grow } from './skeleton';
import { Trunk } from './Trunk';

// A Scots pine (Pinus sylvestris), about 18 m tall: a tall, straight, bare
// trunk with a slight crook, and only at the top an open, irregular crown of
// flat tufts of needles on branches that reach out from the trunk, in clouds
// with gaps between them. Needles are a dark green. Bark is the theme's wood
// colour, needles its grass colour.
//
// Units are meters, y is up and the origin is on the ground at the foot of the
// trunk (the roots reach a little below it). The tree grows from a seed (see
// TreeOptions and skeleton.ts), so it is the same every time. The spec,
// PineTree.md, is empty so far: the tree has no behaviour and stands still.

const SEED = 61;
const FOLIAGE_SHADE = 0.78;
const TRUNK = [
  [0, -0.2, 0],
  [0.15, 5, 0.1],
  [-0.1, 10, 0.2],
  [0.1, 15, 0],
] as const;
const TRUNK_RADIUS = 0.3;
const FLARE = { amount: 0.35, height: 0.4, lobes: 5 };
// The crown is a few clouds of needles round the top of the trunk, and one
// at the very top.
const CLOUDS = {
  count: 7,
  height: [10.8, 16.4] as const, // m, their middles
  out: [1.2, 2.6] as const, // m from the trunk
  width: [2, 2.8] as const, // m, their radius across
  depth: [0.8, 1.1] as const, // m, their radius up and down
};
const TOP_CLOUD = { center: new THREE.Vector3(0, 17, 0), radii: new THREE.Vector3(1.8, 1, 1.8) };
const GROWTH = { points: 700, step: 0.4, reach: 3, kill: 0.9, wander: 0.25, rise: 0.02, pipe: 2.5 };
const CLUMP = { radius: [0.85, 1.2] as const, squash: 0.5 };

export class PineTree extends THREE.Group {
  readonly trunk: Trunk;
  readonly crown: Crown;

  constructor(options: TreeOptions = {}) {
    super();
    this.name = 'pine tree';
    const m = createMaterials(options.theme ?? defaultTheme, FOLIAGE_SHADE, options.autumn);
    const random = seededRandom(options.seed ?? SEED);

    const clouds = [TOP_CLOUD];
    for (let i = 0; i < CLOUDS.count; i++) {
      const angle = 2 * Math.PI * random();
      const out = between(random, ...CLOUDS.out);
      const width = between(random, ...CLOUDS.width);
      clouds.push({
        center: new THREE.Vector3(out * Math.cos(angle), between(random, ...CLOUDS.height), out * Math.sin(angle)),
        radii: new THREE.Vector3(width, between(random, ...CLOUDS.depth), width),
      });
    }
    const box = new THREE.Box3();
    for (const c of clouds) box.union(new THREE.Box3(c.center.clone().sub(c.radii), c.center.clone().add(c.radii)));

    const skeleton = grow(
      {
        trunk: TRUNK.map(([x, y, z]) => new THREE.Vector3(x, y, z)),
        crown: (p) => clouds.some((c) => ellipsoid(p, c.center, c.radii) < 1),
        box,
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
