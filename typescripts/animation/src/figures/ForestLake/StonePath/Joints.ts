import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { between, matte, noise2, type Season } from '../parts';
import { fadeOf, fillColor, flat, footprint, onStone, planted, strip, type Laid, type Planted, type Plants, type Stretch } from './parts';

// What fills between the stone path's stones (StonePath.md), as the sheet
// draws each variation: the ground's fill, a strip over its ground the
// colour of its joints (grass in a regular path's, moss in a mossy one's,
// brown dirt in a dirty or ruined one's; gold in autumn; snow in winter),
// darker toward the middle where the stones shade it; and what grows or
// lies in the joints: small tufts of grass, moss cushions and small
// flowers (more in spring), none on the stones; in autumn fallen leaves,
// on the stones too; in winter nothing but the snow.

// Copies to a square meter of the path, and what they are: small tufts,
// moss, flowers, (in autumn) leaves.
const DENSITY: Record<Stretch['variation'], { per: number; small: number; moss: number; flowers: number }> = {
  regular: { per: 14, small: 0.88, moss: 0, flowers: 0.12 },
  mossy: { per: 5, small: 0.3, moss: 0.55, flowers: 0.15 },
  dirty: { per: 1.2, small: 0.9, moss: 0, flowers: 0.1 },
  ruined: { per: 3, small: 0.6, moss: 0.25, flowers: 0.15 },
  stairs: { per: 0, small: 0, moss: 0, flowers: 0 },
};
const LEAVES = 1.6; // autumn's leaves to a square meter, half of them on the stones

export class Joints extends THREE.Group {
  constructor(stretches: readonly { stretch: Stretch; stones: readonly Laid[] }[], plants: Plants, season: Season, light: (x: number, z: number) => number, random: () => number) {
    super();
    this.name = 'joints';
    const winter = season === 'winter';

    // The fill, one mesh for every stretch; in winter snow, a little higher
    // in the middle. Toward an end that fades (where it meets a path of
    // another kind) it breaks up into patches, its sides first, and is gone
    // before its stones are: single stones on the dirt, then dirt.
    const fills = stretches.map(({ stretch }) => {
      const half = (along: number) => (stretch.widthAt?.(along) ?? stretch.width) / 2 + 0.08;
      const fading = fadeOf(stretch.course.length, stretch.fade);
      const keep = stretch.fade
        ? (along: number, t: number) => {
            const fade = Math.min(1, fading(along) ** 0.55 * 1.3);
            return fade <= 0 ? 1 : noise2(along * 2.4, t * 4.1, 83) * (1 - 0.45 * fade * Math.abs(2 * t - 1)) - fade * 0.9;
          }
        : undefined;
      return strip(
        stretch.course,
        (along) => -half(along),
        half,
        (x, z, t) => fillColor(stretch.variation, season, x, z, Math.abs(2 * t - 1)),
        (t) => (winter ? 0.008 + 0.012 * (1 - Math.abs(2 * t - 1)) : 0.008),
        0.18,
        10,
        keep,
      );
    });
    const merged = fills.length ? mergeGeometries(fills) : null;
    fills.forEach((g) => g.dispose());
    if (merged) this.add(flat(merged));

    // What grows and lies in the joints.
    const shapes = [...plants.small, ...plants.flowers, ...plants.moss, ...plants.leaves];
    const first = { small: 0, flowers: plants.small.length, moss: plants.small.length + plants.flowers.length, leaves: plants.small.length + plants.flowers.length + plants.moss.length };
    const copies: Planted[] = [];
    const flowering = season === 'spring' ? 1 : season === 'summer' ? 0.4 : 0;
    for (const { stretch, stones } of stretches) {
      const feet = stones.map(footprint);
      const { course, width, variation } = stretch;
      const area = course.length * width;
      const kind = DENSITY[variation];
      const count = winter ? 0 : Math.round(area * kind.per);
      for (let k = 0; k < count; k++) {
        const u = random() * course.length;
        const v = between(random, -0.5, 0.5) * (stretch.widthAt?.(u) ?? width);
        const at = course.at(u, v);
        if (onStone(feet, at.x, at.z, 0.015)) continue;
        const roll = random();
        const flowers = kind.flowers * flowering;
        let shape: number;
        if (roll < kind.moss) shape = first.moss + Math.floor(random() * plants.moss.length);
        else if (roll < kind.moss + flowers) shape = first.flowers + Math.floor(random() * plants.flowers.length);
        else shape = first.small + Math.floor(random() * plants.small.length);
        copies.push({ shape, x: at.x, y: at.y, z: at.z, turn: random() * Math.PI * 2, scale: between(random, 0.6, 1.15) });
      }
      // Autumn's leaves, on the stones and between them.
      if (season === 'autumn') {
        for (let k = 0; k < Math.round(area * LEAVES); k++) {
          const u = random() * course.length;
          const at = course.at(u, between(random, -0.55, 0.55) * (stretch.widthAt?.(u) ?? width));
          const stone = onStone(feet, at.x, at.z, -0.04);
          copies.push({ shape: first.leaves + Math.floor(random() * plants.leaves.length), x: at.x, y: stone ? stone.top : at.y, z: at.z, turn: random() * Math.PI * 2, scale: between(random, 0.8, 1.2) });
        }
      }
    }
    for (const batch of planted(shapes, copies, matte(), light, random)) this.add(batch);
  }
}
