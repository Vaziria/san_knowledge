import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Crown } from './Crown';
import { between, createMaterials, seededRandom, tipClumps, type TreeOptions } from './parts';
import { ellipsoid, fan, grow, wobble } from './skeleton';
import { Trunk } from './Trunk';

// A weeping willow (Salix babylonica), about 10 m tall and 13 m across: a
// short, thick, leaning trunk splitting into a few limbs that arch up and out,
// the outer branches bending down, and their leaves hanging from them in long
// cascades, a curtain that stops a meter or two above the ground. Its leaves
// are a fresh, light green. Bark is the theme's wood colour, leaves its grass
// colour.
//
// A cascade is a column of tall, narrow clumps, one below the other, each a
// little smaller than the one above, bowing outward as it falls. They are
// clumps like every other tree's leaves, so the willow is drawn in the same
// hand as the rest.
//
// Units are meters, y is up and the origin is on the ground at the foot of the
// trunk (the roots reach a little below it). The tree grows from a seed (see
// TreeOptions and skeleton.ts), so it is the same every time. The spec,
// WillowTree.md, is empty so far: the tree has no behaviour and stands still.

const SEED = 53;
const FOLIAGE_SHADE = 1.02;
const TRUNK = [
  [0, -0.2, 0],
  [0.15, 1.2, 0.08],
  [0.4, 2.4, 0.2],
] as const;
const TRUNK_RADIUS = 0.48;
const FLARE = { amount: 0.7, height: 0.4, lobes: 6 };
const LIMBS = { count: 4, spread: 2.8, rise: 3.2, bend: 0.55 };
// A low, wide dome: the outer branches arch over and down toward its rim.
const CROWN = { center: new THREE.Vector3(0.4, 5.9, 0.2), radii: new THREE.Vector3(6, 4.2, 6), bottom: 4.2, wobble: 0.1 };
const GROWTH = { points: 800, step: 0.4, reach: 3.5, kill: 1, wander: 0.2, rise: 0.06, droop: 0.25, pipe: 2.5 };
const DROOP_FROM = 2.5; // m from the crown's middle: further out, branches bend down
const CLUMP = { radius: [0.75, 1] as const, squash: 0.8 }; // at the twig tips, rounding off the top

// The cascades hang from a share of the branch nodes out beyond `from`,
// flaring out as they fall, so the curtain spreads like a skirt.
const CASCADES = {
  from: 3, // m from the crown's middle
  above: 3.5, // m: only from nodes higher than this
  share: 0.32, // of the nodes there: the gaps between cascades show the shade inside
  scatter: 0.3, // m round the node that a cascade may hang from
  width: [0.5, 0.75] as const, // m, the top clump's radius across
  height: [0.8, 1.1] as const, // m, its radius up and down
  thickness: 0.5, // its radius front to back, as a share of its width
  jitter: 0.15, // m each clump may sit to either side, so a cascade never hangs in a straight line
  overlap: 0.45, // from one clump's middle to the next, as a share of their two heights: under 1, they overlap
  shrink: 0.9, // each clump is this much the size of the one above
  most: 9, // clumps in a cascade: shrinking, a long one would otherwise never reach its hem
  hem: [0.6, 3] as const, // m above the ground where a cascade ends at the lowest
  flare: 1.6, // m it swings outward on its way down
  fall: 4, // m over which it swings out
};

export class WillowTree extends THREE.Group {
  readonly trunk: Trunk;
  readonly crown: Crown;

  constructor(options: TreeOptions = {}) {
    super();
    this.name = 'willow tree';
    const m = createMaterials(options.theme ?? defaultTheme, FOLIAGE_SHADE);
    const random = seededRandom(options.seed ?? SEED);

    const trunk = TRUNK.map(([x, y, z]) => new THREE.Vector3(x, y, z));
    const lumps = wobble(CROWN.wobble, random);
    const reach = CROWN.radii.clone().multiplyScalar(1 + CROWN.wobble);
    const out = (p: THREE.Vector3) => Math.hypot(p.x - CROWN.center.x, p.z - CROWN.center.z);
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
        tropism: (p) => new THREE.Vector3(0, out(p) > DROOP_FROM ? -GROWTH.droop : GROWTH.rise, 0),
        radius: TRUNK_RADIUS,
        pipe: GROWTH.pipe,
      },
      random,
    );

    const clumps = tipClumps(skeleton.tips, CLUMP.radius, CLUMP.squash, random);
    for (const node of skeleton.nodes) {
      if (out(node.position) < CASCADES.from || node.position.y < CASCADES.above || random() > CASCADES.share) continue;
      const top = node.position
        .clone()
        .add(new THREE.Vector3(between(random, -1, 1), 0, between(random, -1, 1)).multiplyScalar(CASCADES.scatter));
      const facing = new THREE.Vector3(top.x - CROWN.center.x, 0, top.z - CROWN.center.z).normalize();
      const side = new THREE.Vector3(-facing.z, 0, facing.x);
      const turn = new THREE.Quaternion().setFromUnitVectors(new THREE.Vector3(0, 0, 1), facing);
      const hem = between(random, ...CASCADES.hem);
      let width = between(random, ...CASCADES.width);
      let height = between(random, ...CASCADES.height);
      // From the top down, until the next clump would reach below the hem.
      for (let y = top.y - 0.5 * height, k = 0; y - height > hem && k < CASCADES.most; k++) {
        const drop = Math.min(1, (top.y - y) / CASCADES.fall);
        const center = top
          .clone()
          .addScaledVector(facing, CASCADES.flare * Math.sin(0.5 * Math.PI * drop))
          .addScaledVector(side, between(random, -1, 1) * CASCADES.jitter)
          .setY(y);
        clumps.push({ center, size: new THREE.Vector3(width, height, width * CASCADES.thickness), turn });
        const next = height * CASCADES.shrink;
        y -= (height + next) * CASCADES.overlap;
        width *= CASCADES.shrink;
        height = next;
      }
    }

    this.trunk = new Trunk(m, skeleton.limbs, FLARE);
    this.crown = new Crown(m, clumps, random);
    this.add(this.trunk, this.crown);
  }
}
