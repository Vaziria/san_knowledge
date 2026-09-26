import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Crown } from './Crown';
import { between, createMaterials, seededRandom, tipClumps, type TreeOptions } from './parts';
import { fan, grow, wobble } from './skeleton';
import { Trunk } from './Trunk';

// A cedar of Lebanon (Cedrus libani), about 15 m tall with a crown 18 m
// across: a massive trunk that splits into a few leaders, and its foliage in
// broad, flat, level layers like shelves, one above the other, narrowing
// toward the top. Needles are a dark green. Bark is the theme's wood colour,
// needles its grass colour.
//
// Units are meters, y is up and the origin is on the ground at the foot of the
// trunk (the roots reach a little below it). The tree grows from a seed (see
// TreeOptions and skeleton.ts), so it is the same every time. The spec,
// CedarTree.md, is empty so far: the tree has no behaviour and stands still.

const SEED = 71;
const FOLIAGE_SHADE = 0.8;
const TRUNK = [
  [0, -0.2, 0],
  [0.1, 1.5, 0],
  [0.2, 2.8, 0.1],
] as const;
const TRUNK_RADIUS = 0.62;
const FLARE = { amount: 0.8, height: 0.5, lobes: 6 };
const LEADERS = { count: 4, spread: 2.6, rise: 6.5, bend: 0.35 };
// The layers of foliage: their height and radius, in m. Each is off center by
// up to OFFSET and its edge is uneven.
const LAYERS = [
  { y: 4.8, radius: 8.8 },
  { y: 7.4, radius: 7.4 },
  { y: 9.9, radius: 5.8 },
  { y: 12.3, radius: 4.2 },
  { y: 14.4, radius: 2.5 },
];
const LAYER_DEPTH = 0.5; // m above and below a layer's height
const OFFSET = 0.8;
const WOBBLE = 0.15;
const GROWTH = { points: 1000, step: 0.45, reach: 4, kill: 1, wander: 0.2, pipe: 2.4 };
const CLUMP = { radius: [1.1, 1.5] as const, squash: 0.32 };

export class CedarTree extends THREE.Group {
  readonly trunk: Trunk;
  readonly crown: Crown;

  constructor(options: TreeOptions = {}) {
    super();
    this.name = 'cedar tree';
    const m = createMaterials(options.theme ?? defaultTheme, FOLIAGE_SHADE, options.autumn);
    const random = seededRandom(options.seed ?? SEED);

    const trunk = TRUNK.map(([x, y, z]) => new THREE.Vector3(x, y, z));
    const layers = LAYERS.map((l) => ({
      ...l,
      x: between(random, -1, 1) * OFFSET,
      z: between(random, -1, 1) * OFFSET,
      edge: wobble(WOBBLE, random),
    }));
    const widest = LAYERS[0].radius * (1 + WOBBLE) + OFFSET;
    const skeleton = grow(
      {
        trunk,
        limbs: fan(trunk[trunk.length - 1], LEADERS.count, LEADERS.spread, LEADERS.rise, LEADERS.bend, random),
        crown: (p) =>
          layers.some((l) => Math.abs(p.y - l.y) < LAYER_DEPTH && Math.hypot(p.x - l.x, p.z - l.z) < l.radius * l.edge(p)),
        box: new THREE.Box3(
          new THREE.Vector3(-widest, LAYERS[0].y - LAYER_DEPTH, -widest),
          new THREE.Vector3(widest, LAYERS[LAYERS.length - 1].y + LAYER_DEPTH, widest),
        ),
        points: GROWTH.points,
        step: GROWTH.step,
        reach: GROWTH.reach,
        kill: GROWTH.kill,
        wander: GROWTH.wander,
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
