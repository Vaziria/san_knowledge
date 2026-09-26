import * as THREE from 'three';
import { MudPit } from '../../figures/objects/MudPit';
import { between, seededRandom } from '../../figures/Tree/parts';
import type { Theme } from '../../theme';
import type { Boulders } from './Boulders';
import { BANK_WIDTH, groundHeight, LANDING, shoreRadius } from './Ground';

// Mud pits scattered over the lake's land, as the user asked ("create mud
// pit, and add randomly to lake environtment"): five of the mud pit object
// (figures/objects/MudPit.ts), 1.5-4 m across, on the land past the top of
// the bank, mostly 3-15 m from the water's edge, each lying on the uneven
// ground as it lies (groundHeight). The first two are placed in the
// preview cameras' frames, and big: on the west shore, in the penguin's lake
// camera, and on the east shore, in the animals'. Both are about 20 m off
// across the water, where a flat pit seen from under 2 m up is a sliver,
// mostly hidden by the grass and the land's rise. Nearer, the cameras see
// only the clearing, the bank and the water, where no pit may lie.
//
// - Only where the land is nearly flat: its height across the pit changes by
//   at most PITS.flat for each meter across, so a pit never lies on a slope.
// - None in the landing's clearing (the lake meeting's is wider), and none
//   within 2 m of a trunk or a boulder, among the lakeside stones or against
//   another pit.
// - The grass keeps off them (near(), which the sward and the meadow ask, as
//   they ask the boulders).
//
// They are placed by a generator of their own, after the trees, stones and
// boulders, so those keep their places. Units are meters, in the lake's
// coordinates.

const PITS = {
  count: 5,
  width: [1.5, 4] as const, // m
  deep: [0.6, 1] as const, // its depth, times its width
  fromWater: [3, 15] as const, // m past the water's edge, mostly
  nearer: 1.6, // more of them toward the water: the share of the range is random() to this power
  apart: 1.5, // m at least between two pits' edges
  clearing: 0.6, // m kept free outside the landing's clearing
  trunk: 2, // m kept free round a trunk, and round a boulder
  stone: 0.4, // m kept free round a stone
  flat: 0.06, // m the land may rise across each meter of a pit: 3.4°, nearly flat (near the lake the humps rise 13 cm a meter, typically)
  // The first ones, where the preview cameras see them: this far round the
  // lake (radians from +x toward +z) and past the water's edge, as wide as
  // `width`, each if it fits there within `tries`.
  inView: [
    { around: [3.05, 3.45] as const, fromWater: [3, 5.5] as const }, // the west shore, for the penguin's camera
    { around: [-0.25, 0.1] as const, fromWater: [4, 8] as const }, // the east shore, for the animals'
  ],
  inViewWidth: [3, 4] as const,
  tries: 300,
};
const SEED = 29;

interface Placed {
  pit: MudPit;
  x: number;
  z: number;
  radius: number; // m from its middle to its furthest edge
}

export class MudPits extends THREE.Group {
  private readonly placed: Placed[] = [];

  // `stones` are the lakeside stones there are before the pits (Stones.spots).
  constructor(theme: Theme, clear: number, trunks: THREE.Vector3[], boulders: Boulders, stones: { x: number; z: number; radius: number }[]) {
    super();
    this.name = 'mud pits';
    const random = seededRandom(SEED);
    const rocks = stones;
    const ground = (x: number, z: number) => groundHeight(x, z, clear);

    let spot = 0; // the in-view spot now being filled; past the list, anywhere
    let tries = 0;
    for (let attempt = 0; this.placed.length < PITS.count && attempt < 400 * PITS.count; attempt++) {
      // A spot with no room for one (a wide clearing, the lake meeting's) is
      // given up on after PITS.tries.
      if (spot < PITS.inView.length && tries++ >= PITS.tries) {
        spot++;
        tries = 0;
      }
      const view = PITS.inView[spot];
      const inView = view !== undefined;
      const [narrowest, widest] = inView ? PITS.inViewWidth : PITS.width;
      const width = between(random, narrowest, widest);
      const depth = width * between(random, ...PITS.deep);
      const radius = Math.max(width, depth) / 2;
      let x: number;
      let z: number;
      if (inView) {
        const angle = between(random, view.around[0], view.around[1]);
        const out = shoreRadius(angle) + between(random, view.fromWater[0], view.fromWater[1]);
        x = out * Math.cos(angle);
        z = out * Math.sin(angle);
      } else {
        const angle = 2 * Math.PI * random();
        const out = shoreRadius(angle) + PITS.fromWater[0] + (PITS.fromWater[1] - PITS.fromWater[0]) * random() ** PITS.nearer;
        x = out * Math.cos(angle);
        z = out * Math.sin(angle);
      }
      const turn = 2 * Math.PI * random();
      const seed = Math.floor(random() * 1e6);

      // Past the top of the bank, all of it.
      if (Math.hypot(x, z) - radius < shoreRadius(Math.atan2(z, x)) + BANK_WIDTH + 0.3) continue;
      if (Math.hypot(x - LANDING.x, z - LANDING.z) < clear + PITS.clearing + radius) continue;
      if (trunks.some((t) => Math.hypot(x - t.x, z - t.z) < PITS.trunk + radius)) continue;
      if (boulders.near(x, z, PITS.trunk + radius)) continue;
      if (rocks.some((r) => Math.hypot(x - r.x, z - r.z) < r.radius + PITS.stone + radius)) continue;
      if (this.placed.some((p) => Math.hypot(x - p.x, z - p.z) < p.radius + PITS.apart + radius)) continue;
      if (!flat(ground, x, z, radius)) continue;

      const pit = new MudPit({
        theme,
        seed,
        width,
        depth,
        // In the pit's own coordinates: turned and moved to where it lies.
        heightAt: (px, pz) => {
          const c = Math.cos(turn);
          const s = Math.sin(turn);
          return ground(x + c * px + s * pz, z - s * px + c * pz);
        },
      });
      if (inView) {
        spot++;
        tries = 0;
      }
      pit.position.set(x, 0, z);
      pit.rotation.y = turn;
      this.placed.push({ pit, x, z, radius });
      this.add(pit);
    }
  }

  // Whether a point of the lake is within `gap` m of a pit's edge (inside it
  // for a gap under 0), so that grass keeps off them.
  near(x: number, z: number, gap: number): boolean {
    for (const p of this.placed) {
      const dx = x - p.x;
      const dz = z - p.z;
      if (Math.hypot(dx, dz) > 1.3 * p.radius + gap) continue; // its middle may lie a little off its footprint's
      // Into the pit's own coordinates, unturned.
      const c = Math.cos(p.pit.rotation.y);
      const s = Math.sin(p.pit.rotation.y);
      if (p.pit.near(c * dx - s * dz, s * dx + c * dz, gap)) return true;
    }
    return false;
  }

  // The pits' places and sizes, for checking them.
  get spots(): { x: number; z: number; radius: number }[] {
    return this.placed.map(({ x, z, radius }) => ({ x, z, radius }));
  }
}

// Whether the land is nearly flat under a pit: the ground's height at its
// middle and round its edge changes by at most PITS.flat a meter across it.
function flat(ground: (x: number, z: number) => number, x: number, z: number, radius: number): boolean {
  const heights = [ground(x, z)];
  for (let k = 0; k < 12; k++) {
    const angle = (2 * Math.PI * k) / 12;
    heights.push(ground(x + radius * Math.cos(angle), z + radius * Math.sin(angle)));
  }
  return Math.max(...heights) - Math.min(...heights) <= PITS.flat * 2 * radius;
}
