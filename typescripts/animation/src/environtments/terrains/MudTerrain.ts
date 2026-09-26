import * as THREE from 'three';
import { SOIL_MEAN, soilTexture } from '../../textures/soil';
import { defaultTheme } from '../../theme';
import {
  between,
  footprintOf,
  LIFT,
  mottle,
  ovalMask,
  seededRandom,
  SINK,
  surfaceGeometry,
  surfaceMaterial,
  wobble,
  type Footprint,
  type Terrain,
  type TerrainOptions,
} from './parts';

// Mud terrain: a stretch of wet, muddy ground for an environment's land
// (./parts.ts). Where it is wettest the mud is dark, smooth and has a sheen,
// with shallow puddles of standing water lying in it; in drier patches, and
// toward its edge, it dries paler and rougher, the soil's grit and cracks
// showing (textures/soil.ts), until it is the bare land's colour where it
// meets the land. It is the mud pit's mud (figures/objects/MudPit.ts) as open
// ground: level with the land, with no rim round it.
//
// The mud is the bare land's colour (theme.scene.floor), darker as it gets
// wetter, and the puddles the water's (theme.scene.water), with a glint like
// the lake's. Units are meters, y is up, and the origin is the middle of its
// footprint; the land under it is `heightAt` (flat by default). Built from a
// seed. The spec, MudTerrain.md, is a draft: it names no behaviour, so it
// lies still.

export interface MudTerrainOptions extends TerrainOptions {
  puddles?: number; // how many; one for about every 8 m² of mud by default
}

const SIZE: Footprint = { width: 8, depth: 6 };
const SEED = 1;
const CELL = 0.15; // m between the mud's points: fine enough for its lumps

// How wet the mud is: patches `size` m across, wetter where the noise is
// above `from` and wet through above `to`, and drier toward the edge, dry
// where the terrain's cover falls to `edge[0]`.
const WETNESS = { size: 2.6, from: 0.2, to: 0.5, edge: [0.25, 0.9] as const, seed: 5 }; // wet through over about half to four fifths of it
// Its colour, times the land's: dark where wet, paler where dry, as the mud
// pit's; mottled by up to `mottle` either way in patches `size` m across.
const TONE = { wet: 0.42, dry: 0.95, mottle: 0.06, size: 0.9, seed: 9 };
const SOIL = { wet: 0.15, dry: 1 }; // how much of the soil texture shows: little in the smooth wet mud
const BUMP = 0.012; // m, the soil's bump height, as on the lake's ground
const ROUGHNESS = { dry: 0.92, wet: 0.4 }; // the wet mud has a sheen
const LUMPS = { wet: 0.003, dry: 0.01, size: 0.35, seed: 7 }; // m the mud rises and dips, smooth where wet; m from lump to lump

// Puddles lie in the wettest mud, each a thin sheet of water over a dip in
// the mud, apart from the others and clear of the edge.
const PUDDLES = {
  every: 8, // m² of mud for each, by default
  radius: [0.18, 0.5] as const, // m
  wettest: 0.85, // how wet the mud must be under one
  apart: 0.25, // m between two
  above: 0.004, // m the water lies above the mud's level
  dip: 0.008, // m the mud dips under a puddle, so no lump pokes through; it stays 4 mm above the land, which would flicker through the water at its level
  opacity: 0.85,
  roughness: 0.1, // a clear glint, like the lake's calm water
};
const PUDDLE_RINGS = 5;
const PUDDLE_COLUMNS = 28;

interface Puddle {
  x: number;
  z: number;
  radius: number;
  seed: number;
}

export class MudTerrain extends THREE.Group implements Terrain {
  static readonly SIZE = SIZE;

  readonly width: number;
  readonly depth: number;
  readonly mud: THREE.Mesh;
  readonly puddles: THREE.Mesh[] = [];
  private readonly land: (x: number, z: number) => number;
  private readonly mask: (x: number, z: number) => number;
  private readonly lift: (x: number, z: number) => number;

  constructor(options: MudTerrainOptions = {}) {
    super();
    this.name = 'mud terrain';
    const theme = options.theme ?? defaultTheme;
    const size = footprintOf(options, SIZE);
    this.width = size.width;
    this.depth = size.depth;
    const seed = options.seed ?? SEED;
    const land = (this.land = options.heightAt ?? (() => 0));
    const mask = (this.mask = options.mask ?? ovalMask(size, seed));
    const random = seededRandom(seed);

    // How wet the mud is at a point, 0 dry to 1.
    const wetness = (x: number, z: number, cover: number) => {
      const w = WETNESS;
      const patches = 0.5 + 0.5 * mottle(x, z, w.size, w.seed + seed);
      return THREE.MathUtils.smoothstep(patches, w.from, w.to) * THREE.MathUtils.smoothstep(cover, w.edge[0], w.edge[1]);
    };

    // The puddles, in the wettest mud, tried at random spots until there
    // are enough or none will fit.
    const puddles: Puddle[] = [];
    const count = options.puddles ?? Math.round((Math.PI / 4) * size.width * size.depth / PUDDLES.every);
    for (let attempt = 0; puddles.length < count && attempt < 60 * count; attempt++) {
      const radius = between(random, ...PUDDLES.radius);
      const x = (size.width / 2 - radius) * (2 * random() - 1);
      const z = (size.depth / 2 - radius) * (2 * random() - 1);
      const edge = Math.floor(random() * 1e6); // its own uneven edge
      const around = [0, 1, 2, 3, 4, 5].map((k) => [x + 1.4 * radius * Math.cos(k), z + 1.4 * radius * Math.sin(k)]);
      if ([[x, z], ...around].some(([px, pz]) => wetness(px, pz, mask(px, pz)) < PUDDLES.wettest)) continue;
      if (puddles.some((p) => Math.hypot(p.x - x, p.z - z) < 1.35 * (p.radius + radius) + PUDDLES.apart)) continue;
      puddles.push({ x, z, radius, seed: edge });
    }
    // How much a point is under a puddle: 1 inside, 0 a little way out.
    const underPuddle = (x: number, z: number) =>
      Math.max(0, ...puddles.map((p) => 1 - THREE.MathUtils.smoothstep(Math.hypot(x - p.x, z - p.z), p.radius * 1.05, p.radius * 1.35)));

    // How far the mud lies above the land: LIFT where it is all mud, with
    // its lumps, down to the land's level under a puddle, and sinking under
    // the land at its edge.
    this.lift = (x, z) => {
      const cover = mask(x, z);
      const wet = wetness(x, z, cover);
      const lumps = THREE.MathUtils.lerp(LUMPS.dry, LUMPS.wet, wet) * wobble(x / LUMPS.size, z / LUMPS.size, LUMPS.seed + seed);
      const mud = THREE.MathUtils.lerp(-SINK, LIFT + lumps, cover);
      const under = underPuddle(x, z);
      return mud * (1 - under) + (LIFT - PUDDLES.dip) * under;
    };

    const floor = new THREE.Color(theme.scene.floor);
    const geo = surfaceGeometry(size, CELL, mask, land, (x, z, cover, out) => {
      const wet = wetness(x, z, cover);
      out.lift = this.lift(x, z);
      const tone = THREE.MathUtils.lerp(TONE.dry, TONE.wet, wet) * (1 + TONE.mottle * mottle(x, z, TONE.size, TONE.seed + seed));
      out.color.copy(floor).multiplyScalar(tone);
      out.soil = THREE.MathUtils.lerp(SOIL.dry, SOIL.wet, wet);
      out.wet = wet;
    });
    const material = surfaceMaterial({ texture: soilTexture(), mean: SOIL_MEAN, bump: BUMP, roughness: ROUGHNESS.dry, wetRoughness: ROUGHNESS.wet });
    this.mud = new THREE.Mesh(geo, material);
    this.mud.name = 'mud';
    this.mud.receiveShadow = true; // low and lumpy: it casts none worth drawing
    this.add(this.mud);

    // The puddles: thin sheets of water on the land's height, a little
    // above the mud's level.
    const water = new THREE.MeshStandardMaterial({ color: theme.scene.water, roughness: PUDDLES.roughness, transparent: true, opacity: PUDDLES.opacity });
    for (const p of puddles) {
      const sheet = new THREE.Mesh(puddleGeometry(p, land), water);
      sheet.name = 'puddle';
      sheet.receiveShadow = true;
      this.puddles.push(sheet);
      this.add(sheet);
    }
  }

  // The mud's height at a point (its own coordinates).
  heightAt(x: number, z: number): number {
    return this.land(x, z) + this.lift(x, z);
  }

  covers(x: number, z: number): number {
    return this.mask(x, z);
  }

  // Nothing moves.
  update(): void {}
}

// A puddle's sheet: rings out to its edge, which swells and dips a little,
// as the mud pit's puddles are.
function puddleGeometry(p: Puddle, land: (x: number, z: number) => number): THREE.BufferGeometry {
  const positions: number[] = [];
  for (let i = 0; i <= PUDDLE_RINGS; i++) {
    for (let j = 0; j < PUDDLE_COLUMNS; j++) {
      const angle = (2 * Math.PI * j) / PUDDLE_COLUMNS;
      const edge = 1 + 0.18 * wobble(2 * Math.cos(angle), 2 * Math.sin(angle), p.seed);
      const r = (p.radius * edge * i) / PUDDLE_RINGS;
      const x = p.x + r * Math.cos(angle);
      const z = p.z + r * Math.sin(angle);
      positions.push(x, land(x, z) + LIFT + PUDDLES.above, z);
    }
  }
  const indices: number[] = [];
  for (let i = 0; i < PUDDLE_RINGS; i++) {
    for (let j = 0; j < PUDDLE_COLUMNS; j++) {
      const a = i * PUDDLE_COLUMNS + j;
      const b = i * PUDDLE_COLUMNS + ((j + 1) % PUDDLE_COLUMNS);
      if (i > 0) indices.push(a, b, a + PUDDLE_COLUMNS);
      indices.push(b, b + PUDDLE_COLUMNS, a + PUDDLE_COLUMNS);
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setIndex(indices);
  geo.computeVertexNormals();
  return geo;
}
