import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { between, color, matte, noise2, palette, type Season } from '../parts';
import { flat, footprint, onStone, planted, strip, type Laid, type Planted, type Plants, type Stretch } from './parts';

// The stone path's verge (StonePath.md), along both its sides, as the sheet
// draws it: tufts of grass, taller than in the joints, and small flowers
// (more in spring); gold grass and fallen leaves in autumn; in winter snow
// banked up along each side, a few dry tufts standing out of it.

const EVERY = 0.3; // m along a side between the verge's plants (the forest lake grows its own tufts along its paths too)
const OUT = 0.34; // m out from the path's edge they grow, at most
const BANK = { out: 0.6, over: 0.07, height: 0.11 }; // winter's snow bank: m out past the edge, m in over the stones' edge, m high

export class Verge extends THREE.Group {
  constructor(stretches: readonly { stretch: Stretch; stones: readonly Laid[] }[], plants: Plants, season: Season, light: (x: number, z: number) => number, random: () => number) {
    super();
    this.name = 'verge';
    const winter = season === 'winter';
    const shapes = [...plants.tall, ...plants.small, ...plants.flowers, ...plants.leaves];
    const first = { tall: 0, small: plants.tall.length, flowers: plants.tall.length + plants.small.length, leaves: plants.tall.length + plants.small.length + plants.flowers.length };
    const flowering = season === 'spring' ? 0.16 : season === 'summer' ? 0.06 : 0;
    const copies: Planted[] = [];
    const banks: THREE.BufferGeometry[] = [];
    for (const { stretch, stones } of stretches) {
      const { course, width } = stretch;
      const feet = stones.map(footprint);
      for (const side of [-1, 1]) {
        for (let u = random() * EVERY; u < course.length; u += EVERY * between(random, 0.6, 1.4)) {
          if (winter && random() > 0.25) continue;
          const v = side * ((stretch.widthAt?.(u) ?? width) / 2 + between(random, -0.05, OUT));
          const at = course.at(u, v);
          if (onStone(feet, at.x, at.z, 0.02)) continue;
          const roll = random();
          const leafy = season === 'autumn' ? 0.2 : 0;
          let shape: number;
          if (roll < leafy) shape = first.leaves + Math.floor(random() * plants.leaves.length);
          else if (roll < leafy + flowering) shape = first.flowers + Math.floor(random() * plants.flowers.length);
          else if (roll < leafy + flowering + 0.45) shape = first.tall + Math.floor(random() * plants.tall.length);
          else shape = first.small + Math.floor(random() * plants.small.length);
          copies.push({ shape, x: at.x, y: at.y, z: at.z, turn: random() * Math.PI * 2, scale: between(random, 0.75, 1.3) });
        }
        // Winter's snow, banked up along the side: highest a little out
        // from the edge, lumpy along it.
        if (winter) {
          const P = palette(season);
          const snow = color(P.snow);
          const blue = color(P.snowShade);
          const half = (along: number) => (stretch.widthAt?.(along) ?? width) / 2;
          const inner = (along: number) => half(along) - BANK.over;
          const outer = (along: number) => half(along) + BANK.out;
          banks.push(
            strip(
              course,
              side < 0 ? (along) => -outer(along) : inner,
              side < 0 ? (along) => -inner(along) : outer,
              (x, z, t) => blue.clone().lerp(snow, 0.55 + 0.45 * (1 - Math.abs((side < 0 ? 1 - t : t) - 0.45) * 1.6) * (0.8 + 0.2 * noise2(x * 3, z * 3, 83))),
              (t, along) => {
                const out = side < 0 ? 1 - t : t; // 0 at the path, 1 out
                const bump = Math.sin(Math.PI * Math.min(1, out * 1.15)) * (out < 0.08 ? out / 0.08 : 1);
                return 0.01 + BANK.height * bump * (0.7 + 0.6 * noise2(along * 1.3, side * 7.1, 87));
              },
              0.2,
              5,
            ),
          );
        }
      }
    }
    for (const batch of planted(shapes, copies, matte(), light, random)) this.add(batch);
    if (banks.length) {
      const merged = mergeGeometries(banks);
      banks.forEach((g) => g.dispose());
      if (merged) this.add(flat(merged));
    }
  }
}
