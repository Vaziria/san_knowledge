import * as THREE from 'three';
import { SAND_MEAN, sandTexture } from '../../textures/sand';
import { defaultTheme } from '../../theme';
import {
  footprintOf,
  mottle,
  ovalMask,
  seededRandom,
  SINK,
  surfaceGeometry,
  surfaceMaterial,
  type Footprint,
  type Terrain,
  type TerrainOptions,
} from './parts';

// Sand terrain: dry, loose sand for an environment's land (./parts.ts), as
// on a beach or a sandbank. It lies a few centimeters deep on the land,
// heaped a little higher and lower in soft drifts, and thins out toward its
// edge until it sinks under the land it meets. Its top is covered in the
// ripples the wind raises on dry sand, long wavy crests about 17 cm apart
// and about a centimeter high, high in some patches and all but gone in
// others, over fine grain (textures/sand.ts): the sun picks them out and the
// heavier, darker grains gather in their troughs. It is matte, and mottled a
// little lighter and darker in patches a couple of meters across.
//
// The colour is the theme's sand (theme.scene.sand). Units are meters, y is
// up, and the origin is the middle of its footprint; the land under it is
// `heightAt` (flat by default). Built from a seed, which also turns the
// ripples to face another way unless `ripples` says which way the wind blew.
// The spec, SandTerrain.md, is a draft: it names no behaviour, so it lies
// still.

export interface SandTerrainOptions extends TerrainOptions {
  // The way the wind blew that raised the ripples, in radians from +x toward
  // +z: the crests run across it. From the seed by default.
  ripples?: number;
}

const SIZE: Footprint = { width: 10, depth: 8 };
const SEED = 1;
const CELL = 0.2; // m between the sand's points
const DEEP = 0.03; // m of sand on the land where it is all sand
const DRIFTS = { height: 0.012, size: 1.4, seed: 3 }; // m higher or lower in drifts about `size` m across
const MOTTLE = { amount: 0.05, size: 2.2, seed: 5 }; // lighter and darker by up to `amount`
// How much of the texture's light and dark shows in the sand's colour; its
// bumps show in full (BUMP is divided by it, as the material shrinks them
// with it). The texture's crests stand about half a unit above its troughs
// in linear light where the ripples are highest, so 2.4 cm a unit makes
// them 1.2 cm high.
const SHADE = 0.55;
const BUMP = 0.024; // m a unit of the texture's brightness
const ROUGHNESS = 1; // dry sand: no sheen

export class SandTerrain extends THREE.Group implements Terrain {
  static readonly SIZE = SIZE;

  readonly width: number;
  readonly depth: number;
  readonly sand: THREE.Mesh;
  private readonly land: (x: number, z: number) => number;
  private readonly mask: (x: number, z: number) => number;
  private readonly seed: number;

  constructor(options: SandTerrainOptions = {}) {
    super();
    this.name = 'sand terrain';
    const theme = options.theme ?? defaultTheme;
    const size = footprintOf(options, SIZE);
    this.width = size.width;
    this.depth = size.depth;
    const seed = (this.seed = options.seed ?? SEED);
    const land = (this.land = options.heightAt ?? (() => 0));
    const mask = (this.mask = options.mask ?? ovalMask(size, seed));
    const wind = options.ripples ?? Math.PI * seededRandom(seed)();
    const along = { x: Math.cos(wind), z: Math.sin(wind) };

    const sand = new THREE.Color(theme.scene.sand);
    const geo = surfaceGeometry(size, CELL, mask, land, (x, z, cover, out) => {
      out.lift = this.lift(x, z, cover);
      out.color.copy(sand).multiplyScalar(1 + MOTTLE.amount * mottle(x, z, MOTTLE.size, MOTTLE.seed + seed));
      out.soil = SHADE;
      // u the way the wind blew, v along the crests.
      out.u = x * along.x + z * along.z;
      out.v = z * along.x - x * along.z;
    });
    this.sand = new THREE.Mesh(geo, surfaceMaterial({ texture: sandTexture(), mean: SAND_MEAN, bump: BUMP / SHADE, roughness: ROUGHNESS }));
    this.sand.name = 'sand';
    this.sand.receiveShadow = true; // it casts none: its drifts are a few centimeters high
    this.add(this.sand);
  }

  // The sand's height at a point (its own coordinates).
  heightAt(x: number, z: number): number {
    return this.land(x, z) + this.lift(x, z, this.mask(x, z));
  }

  covers(x: number, z: number): number {
    return this.mask(x, z);
  }

  // Nothing moves.
  update(): void {}

  // How deep the sand lies on the land at a point: DEEP give or take its
  // drifts where it is all sand, thinning to under the land at its edge.
  private lift(x: number, z: number, cover: number): number {
    const drift = DRIFTS.height * mottle(x, z, DRIFTS.size, DRIFTS.seed + this.seed);
    return THREE.MathUtils.lerp(-SINK, DEEP + drift, cover);
  }
}
