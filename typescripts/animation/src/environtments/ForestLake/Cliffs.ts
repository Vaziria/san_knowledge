import * as THREE from 'three';
import { addHull, addIcicle, addRock, between, matte, mesh, palette, rockPaint, seededRandom, Shape, snowOn, type Palette, type Season } from '../../figures/ForestLake/parts';
import type { Froth } from './Water';
import type { Shade, Terrain } from './Terrain';

// A gap in the cliffs: no slabs within `radius` of it lower than `above`
// (a waterfall's lip, the cave's mouth).
export interface Gap {
  x: number;
  z: number;
  radius: number;
  above?: number;
}

// The forest lake's cliffs (ForestLake.md), as the reference's "Cliff
// (Terrain)" draws them: faces of tall, faceted grey columns of rock stacked
// up from the land (or the lake) to the heights' edge all along the rim,
// the tops of the columns capped with moss (the season's: snow in winter),
// with boulders heaped at their feet and standing in the waterfalls' pools.
// In winter snow lies on every ledge and icicles hang from the cliffs' top
// edge.
export class Cliffs extends THREE.Group {
  readonly rock: THREE.Mesh;
  // The tops of columns standing out of the cliff face below its top, and
  // along its top edge, for bushes and grass to grow on.
  readonly ledges: { x: number; y: number; z: number; top: boolean }[] = [];
  // Where foam gathers round the boulders standing in the water, and the
  // boulders' shadows and footprints, for the water and the land.
  readonly froth: Froth[] = [];
  readonly shades: Shade[] = [];
  readonly boulders: { x: number; z: number; radius: number }[] = [];

  constructor(terrain: Terrain, gaps: readonly Gap[], pools: readonly { x: number; z: number; width: number }[], season: Season = 'spring') {
    super();
    const random = seededRandom(401);
    const shape = new Shape();
    const P = cliffRock(palette(season));
    const paint = rockPaint(random, 0.62, P);
    const icicles = new Shape();
    const rim = terrain.rim;
    const up = new THREE.Vector3(0, 1, 0);

    let s = 0;
    while (s < rim.length) {
      const at = rim.at(s);
      const width = between(random, 1.7, 2.8);
      s += width * 0.74;
      const level = rim.value(rim.nearest(at.x, at.z));
      // Out toward the lake is the rim's right.
      const out = new THREE.Vector3(-at.dz, 0, at.dx);
      const foot = terrain.heightAt(at.x + out.x * 2.4, at.z + out.z * 2.4);
      const top = level;
      if (top - foot < 1.1) continue;
      const blocked = (y: number) => gaps.some((g) => Math.hypot(at.x - g.x, at.z - g.z) < g.radius && y < (g.above ?? Infinity));
      // Stacked from below the land up to just over the heights' edge.
      let y = foot - 0.6;
      while (y < top - 0.25) {
        const tall = Math.min(top + 0.2 - y, between(random, 2.6, 4.4));
        if (tall < 0.8) break;
        if (!blocked(y)) {
          const deep = between(random, 1.8, 2.8);
          const along = new THREE.Vector3(at.dx, 0, at.dz);
          const stand = between(random, -0.25, 0.35);
          const center = new THREE.Vector3(at.x, 0, at.z)
            .addScaledVector(out, 0.55 - deep / 2 + stand)
            .addScaledVector(along, between(random, -0.4, 0.4));
          center.y = y + tall / 2;
          const turn = new THREE.Matrix4().makeBasis(along, up, out).setPosition(center);
          addHull(
            shape,
            slabPoints(random, width, tall, deep).map((p) => p.applyMatrix4(turn)),
            (normal) => paint(normal),
          );
          // Its top: a ledge, where it stands out of the face below the
          // cliff's top, or the top edge.
          const topY = y + tall;
          const edge = topY > top - 0.3;
          if (edge || stand > 0.1) {
            const ledge = center.clone().addScaledVector(out, deep / 2 - 0.45);
            this.ledges.push({ x: ledge.x, y: topY - 0.05, z: ledge.z, top: edge });
          }
          // In winter, icicles hanging from the top edge down the face.
          if (season === 'winter' && edge && random() < 0.55) {
            const lip = center.clone().addScaledVector(out, deep / 2 + 0.02);
            for (let k = 0; k < 3; k++) {
              const across = between(random, -0.4, 0.4) * width;
              addIcicle(icicles, lip.clone().addScaledVector(along, across).setY(topY - 0.12), between(random, 0.3, 0.9), between(random, 0.05, 0.09), random);
            }
          }
        }
        y += tall * between(random, 0.78, 0.92);
      }
      // Boulders heaped at the cliff's foot.
      if (random() < 0.45 && !blocked(foot)) {
        const size = between(random, 0.8, 1.9);
        const x = at.x + out.x * between(random, 2.2, 3.4);
        const z = at.z + out.z * between(random, 2.2, 3.4);
        this.boulder(shape, terrain, x, z, size, random);
      }
    }

    // Over a gap that only keeps the cliffs' foot open (the cave's mouth),
    // the notch left in the heights above it is roofed with slabs, down to
    // `above` - 0.75 (clear of its hollow's roof), from where the rim is
    // nearest it back into the heights.
    for (const gap of gaps) {
      if (gap.above === undefined) continue;
      const near = rim.nearest(gap.x, gap.z);
      const at = rim.at(near.along);
      const level = rim.value(near);
      const inward = new THREE.Vector3(at.dz, 0, -at.dx);
      const along = new THREE.Vector3(at.dx, 0, at.dz);
      const bottom = gap.above - 0.75;
      for (const offset of [-1.6, 0, 1.6]) {
        for (const back of [1.2, 3.4]) {
          const tall = level + 0.25 - bottom;
          const center = new THREE.Vector3(at.x, bottom + tall / 2, at.z).addScaledVector(along, offset).addScaledVector(inward, back);
          const turn = new THREE.Matrix4().makeBasis(along, up, inward.clone().negate()).setPosition(center);
          addHull(
            shape,
            slabPoints(random, 2.4, tall, 2.4).map((p) => p.applyMatrix4(turn)),
            (normal) => paint(normal),
          );
        }
      }
    }

    // Boulders standing in each waterfall's pool, the water frothing round
    // them.
    for (const pool of pools) {
      const count = Math.round(4 + pool.width * 1.5);
      for (let k = 0; k < count; k++) {
        const angle = between(random, -Math.PI, 0) + between(random, -0.3, 0.3);
        const d = between(random, 1.2, 2.6) + pool.width * 0.4;
        this.boulder(shape, terrain, pool.x + Math.cos(angle) * d * 1.2, pool.z - Math.sin(angle) * d * 0.7 + 1.2, between(random, 0.6, 1.3), random);
      }
    }

    this.rock = mesh(shape.geometry(), matte(), false);
    this.rock.receiveShadow = true;
    this.add(this.rock);
    if (season === 'winter') {
      snowOn(this.rock, random, 0.4, 0.72);
      this.add(mesh(icicles.geometry(), matte(), false));
    }
  }

  private boulder(shape: Shape, terrain: Terrain, x: number, z: number, size: number, random: () => number): void {
    const ground = terrain.heightAt(x, z);
    addRock(shape, new THREE.Vector3(x, ground - size * 0.15, z), new THREE.Vector3(size, size * 0.8, size * 0.9), random() * Math.PI, random, 0.7, 16, cliffRock(palette(terrain.season)));
    this.boulders.push({ x, z, radius: size * 0.5 });
    this.shades.push({ x, y: ground + size * 0.3, z, radius: size * 0.5, foot: size * 0.9 });
    if (ground < 0.1) this.froth.push({ x, z, radius: size * 0.5 + 0.7, strength: 0.75 });
  }
}

// The cliffs' rock: the palette's, a shade darker and warmer, as the
// reference's cliffs stand darker than its boulders.
function cliffRock(P: Palette): Palette {
  return { ...P, rockLight: 0xc8c1c6, rock: 0x98909c, rockDark: 0x5f5968 };
}

// Points for a slab of cliff rock `width` along the cliff, `tall` and
// `deep`: its corners roughened by up to a tenth of its size, and a point
// pushed out of the middle of each face, so its hull (addHull) is a
// faceted, chunky slab.
function slabPoints(random: () => number, width: number, tall: number, deep: number): THREE.Vector3[] {
  const points: THREE.Vector3[] = [];
  const jitter = (size: number) => (random() - 0.5) * 0.2 * size;
  for (const sx of [-1, 1]) {
    for (const sy of [-1, 1]) {
      for (const sz of [-1, 1]) {
        // The top corners toward the lake are cut back, as broken rock is.
        const cut = sy > 0 && sz > 0 ? between(random, 0.75, 0.92) : 1;
        points.push(new THREE.Vector3((sx * width) / 2 + jitter(width), (sy * tall) / 2 + jitter(tall), ((sz * deep) / 2) * cut + jitter(deep)));
      }
    }
  }
  for (const [x, y, z] of [
    [0, 0, 1],
    [0, 0, -1],
    [1, 0, 0],
    [-1, 0, 0],
    [0, 1, 0],
  ]) {
    const push = between(random, 0.02, 0.12);
    points.push(new THREE.Vector3(x * (width / 2) * (1 + push) + jitter(width) * (1 - Math.abs(x)), y * (tall / 2) * (1 + push * 0.5) + jitter(tall) * (1 - Math.abs(y)), z * (deep / 2) * (1 + push) + jitter(deep) * (1 - Math.abs(z))));
  }
  return points;
}
