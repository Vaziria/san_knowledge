import * as THREE from 'three';
import { SOIL_MEAN, soilTexture } from '../../textures/soil';
import { defaultTheme } from '../../theme';
import { tuftGeometry, type TuftShape } from '../Lake/Sward';
import { sway, type Wind } from '../wind';
import {
  between,
  footprintOf,
  landNormal,
  LIFT,
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

// Grass terrain: grassy ground for an environment's land (./parts.ts), as
// on the lake's land beyond its bank. A lawn a shade darker than the grass,
// mottled lighter and darker in patches about a meter across, the soil's
// grit barely showing through; and tufts of pointed blades growing all over
// it, the sward's three shapes (upright and broad, spreading, fine and
// floppy), each sized, turned and shaded at random, 7-26 cm tall with most
// of them short, leaning a little with the slope. Toward its edge the tufts
// thin out and the lawn sinks under the land, so it meets the land around
// it without a line. The colour is the theme's grass (theme.scene.grass).
//
// Units are meters, y is up, and the origin is the middle of its footprint;
// the land under it is `heightAt` (flat by default). Built from a seed, so
// the same seed grows the same grass. The spec, GrassTerrain.md, is a draft:
// it names no behaviour. In a wind (`wind`, which the environment owns and
// moves) the tufts bend, their tips 7 cm at 25 cm up in a full wind, as the
// lake's sward does. Tufts take shadows but cast none.

export interface GrassTerrainOptions extends TerrainOptions {
  wind?: Wind; // the tufts bend in it; they stand still without it
  height?: readonly [number, number]; // m: the shortest and tallest tufts; most are short
  density?: number; // tufts a square meter, where it is all grass
  keepOff?: (x: number, z: number) => boolean; // no tuft grows where this says true (under a stone, at a trunk)
}

const SIZE: Footprint = { width: 10, depth: 8 };
const SEED = 1;
const CELL = 0.25; // m between the lawn's points
// The lawn under the tufts, times the grass colour: darker than the lake's
// (0.75), so the gaps between the tufts show and the painterly filter keeps
// them as strokes; with the lake's shades the tufts merged into the lawn
// from more than a few meters off.
const LAWN = 0.6;
const MOTTLE = { amount: 0.12, size: 1.2 }; // lighter and darker by up to `amount`, in patches `size` m across
const SOIL = 0.3; // how much of the soil texture shows through, as on the lake's lawn
const BUMP = 0.012; // m, the soil's bump height, as on the lake's ground

const SHAPES: TuftShape[] = [
  { blades: 6, width: 0.1, spread: 0.35, lean: 0.3 }, // upright, broad-bladed
  { blades: 5, width: 0.08, spread: 0.7, lean: 0.45 }, // spreading
  { blades: 8, width: 0.05, spread: 0.55, lean: 0.6 }, // fine, floppy
];
const HEIGHT = [0.07, 0.26] as const; // m, as the lake's sward
const SHORT = 1.3; // the power on a random number that makes most tufts short
const DENSITY = 70; // a square meter: as thick as the sward round the lake's landing
const FOLLOW = 0.7; // how far a tuft leans with the land's slope
const TINT = [0.8, 1.15] as const; // darkest and lightest tuft, times the grass colour (the sward's are 0.72-1.05)
const BEND = { sway: 0.07, reach: 0.25 };

const UP = new THREE.Vector3(0, 1, 0);

export class GrassTerrain extends THREE.Group implements Terrain {
  static readonly SIZE = SIZE;

  readonly width: number;
  readonly depth: number;
  readonly lawn: THREE.Mesh;
  readonly tufts: THREE.InstancedMesh[] = [];
  private readonly land: (x: number, z: number) => number;
  private readonly mask: (x: number, z: number) => number;

  constructor(options: GrassTerrainOptions = {}) {
    super();
    this.name = 'grass terrain';
    const theme = options.theme ?? defaultTheme;
    const size = footprintOf(options, SIZE);
    this.width = size.width;
    this.depth = size.depth;
    const seed = options.seed ?? SEED;
    const land = (this.land = options.heightAt ?? (() => 0));
    const mask = (this.mask = options.mask ?? ovalMask(size, seed));
    const random = seededRandom(seed);

    // The lawn: the grass colour a shade darker, mottled.
    const grass = new THREE.Color(theme.scene.grass);
    const lawn = surfaceGeometry(size, CELL, mask, land, (x, z, cover, out) => {
      out.lift = lift(cover);
      out.color.copy(grass).multiplyScalar(LAWN * (1 + MOTTLE.amount * mottle(x, z, MOTTLE.size, seed)));
      out.soil = SOIL;
    });
    this.lawn = new THREE.Mesh(lawn, surfaceMaterial({ texture: soilTexture(), mean: SOIL_MEAN, bump: BUMP, roughness: 1 }));
    this.lawn.name = 'lawn';
    this.lawn.receiveShadow = true;
    this.add(this.lawn);

    // The tufts: spots picked evenly over the footprint, kept as often as
    // the ground there is grass (squared, so they thin out well inside the
    // edge, before the lawn sinks), each stood on the lawn.
    const [shortest, tallest] = options.height ?? HEIGHT;
    const spots = Math.round((options.density ?? DENSITY) * size.width * size.depth);
    const placed: THREE.Matrix4[][] = SHAPES.map(() => []);
    const shades: number[][] = SHAPES.map(() => []);
    const lean = new THREE.Quaternion();
    const turn = new THREE.Quaternion();
    for (let k = 0; k < spots; k++) {
      const x = size.width * (random() - 0.5);
      const z = size.depth * (random() - 0.5);
      const keep = random();
      const height = THREE.MathUtils.lerp(shortest, tallest, random() ** SHORT);
      const shape = Math.floor(random() * SHAPES.length);
      const angle = 2 * Math.PI * random();
      const shade = between(random, ...TINT);
      const cover = mask(x, z);
      if (keep >= cover * cover || options.keepOff?.(x, z)) continue;
      lean.setFromUnitVectors(UP, landNormal(land, x, z)).slerp(new THREE.Quaternion(), 1 - FOLLOW);
      turn.setFromAxisAngle(UP, angle);
      placed[shape].push(
        new THREE.Matrix4().compose(
          new THREE.Vector3(x, land(x, z) + Math.max(0, lift(cover)), z),
          lean.clone().multiply(turn),
          new THREE.Vector3(height, height, height),
        ),
      );
      shades[shape].push(shade);
    }

    const material = new THREE.MeshStandardMaterial({ color: theme.scene.grass, roughness: 0.9, vertexColors: true });
    if (options.wind) sway(material, options.wind, BEND);
    const color = new THREE.Color();
    SHAPES.forEach((shape, s) => {
      const tufts = new THREE.InstancedMesh(tuftGeometry(shape, random), material, placed[s].length);
      tufts.name = 'tufts';
      placed[s].forEach((matrix, i) => {
        tufts.setMatrixAt(i, matrix);
        tufts.setColorAt(i, color.setScalar(shades[s][i]));
      });
      tufts.receiveShadow = true;
      this.tufts.push(tufts);
      this.add(tufts);
    });
  }

  // The lawn's height at a point (its own coordinates), under the tufts.
  heightAt(x: number, z: number): number {
    return this.land(x, z) + lift(this.mask(x, z));
  }

  covers(x: number, z: number): number {
    return this.mask(x, z);
  }

  // Nothing moves on its own: the tufts bend in the wind on the graphics card.
  update(): void {}
}

// How far the lawn lies above the land, for how much of the ground is grass.
function lift(cover: number): number {
  return THREE.MathUtils.lerp(-SINK, LIFT, cover);
}
