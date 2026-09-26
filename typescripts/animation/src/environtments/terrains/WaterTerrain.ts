import * as THREE from 'three';
import { SOIL_MEAN, soilTexture } from '../../textures/soil';
import { defaultTheme } from '../../theme';
import { waterSurface } from '../water';
import type { Wind } from '../wind';
import {
  footprintOf,
  LIFT,
  ovalMask,
  SINK,
  surfaceGeometry,
  surfaceMaterial,
  type Footprint,
  type Terrain,
  type TerrainOptions,
} from './parts';

// Water terrain: water for an environment's land (./parts.ts), filling the
// land's hollows up to its surface, as the lake's water does. The
// environment shapes the hollow (`heightAt`, a lake's bed or a pond's); the
// terrain lays a sheet of water over it at its origin's level and covers the
// bed under it, so the land around that rises above the water hides the
// sheet's edge, and the water meets the land in a crisp line where the bank
// slopes up out of it.
//
// The water is a see-through sheet, clearer where it is shallow, so the bed
// shows at its edge and the water's colour deepens toward the middle. It
// moves in small waves: a few sine waves
// crossing each other, each travelling at the speed deep water gives its
// length, 1-3 cm high, as on the lake. The same sum moves the sheet and
// answers heightAt(), so anything riding the waves follows what is drawn.
// In a wind (`wind`, which the environment owns and moves) the waves grow
// higher, a short ripple runs downwind across them and the glint dulls. The
// sheet is kept out of hulls marked dry (../water.ts).
//
// The bed is the bare land's colour (theme.scene.floor), fading toward the
// water's (theme.scene.water) with depth, so deep water looks deeper, with
// the soil's grit showing in the shallows and fading out further down; the
// bank is darker and smoother where the water has wet it, just above the
// waterline.
//
// Units are meters, y is up, and the origin is the middle of its footprint
// on the still water's surface (y = 0 is the water level), so the land must
// lie below 0 where there is water and rise above it all round, inside the
// footprint. The spec, WaterTerrain.md, is a draft: it names no behaviour;
// on its own the water moves in its waves (update).

export interface WaterTerrainOptions extends TerrainOptions {
  wind?: Wind; // raises its waves and roughens it; calm without it
}

const SIZE: Footprint = { width: 10, depth: 8 };
const SEED = 1;
const SHEET_CELL = 0.16; // m between the sheet's points: 6 to the shortest wave
const BED_CELL = 0.2; // m between the bed's points
const GRAVITY = 9.81;
// How much of the bed the water hides: `shallow` at its edge, where the bed
// shows through, growing to `deep` (a little more than the lake's 0.72, all
// over) `over` m down. At one depth all over, a pond seen from above was a
// flat oval of one colour.
const CLEAR = { shallow: 0.4, deep: 0.85, over: 0.9 };
const ROUGHNESS = 0.12; // low: a calm surface with a clear sun glint
const WIND_WAVES = 1.2; // in a full wind the waves are this much higher again
const RIPPLE = { length: 0.9, height: 0.02 }; // m: the short wave a full wind raises, running downwind
const WIND_ROUGHNESS = 0.25; // how much rougher the surface is in a full wind
const EAST = new THREE.Vector2(1, 0); // the way the ripple would run, in still air (it has no height then)

// [length (m), height from trough to crest (m), direction it travels
// (radians from +x toward +z), phase], as the lake's.
const WAVES: [length: number, height: number, direction: number, phase: number][] = [
  [3.2, 0.03, 0.5, 0],
  [1.9, 0.02, 1.4, 1.7],
  [1.1, 0.012, -0.5, 4.2],
];

// The bed: how deep the water is where the bed has faded DEEP_TINT of the
// way toward the water's colour; how much of the soil texture shows, all of
// it in the shallows fading to `deep` over the first `over` m down; and the
// wet bank, `band` m up from the water, darker (`tone`) and smoother.
const DEEP = 1.4; // m
const DEEP_TINT = 0.85;
const SOIL = { deep: 0.15, over: 0.5 };
const WET = { band: 0.12, tone: 0.62 };
const BED_ROUGHNESS = { dry: 0.9, wet: 0.45 };
const BUMP = 0.012; // m, the soil's bump height, as on the lake's ground

interface Wave {
  kx: number; // wave number along x and z (radians per meter)
  kz: number;
  omega: number; // radians per second
  amplitude: number;
  phase: number;
}

const calm: Wave[] = WAVES.map(([length, height, direction, phase]) => {
  const k = (2 * Math.PI) / length;
  return { kx: k * Math.cos(direction), kz: k * Math.sin(direction), omega: Math.sqrt(GRAVITY * k), amplitude: height / 2, phase };
});

export class WaterTerrain extends THREE.Group implements Terrain {
  static readonly SIZE = SIZE;

  readonly width: number;
  readonly depth: number;
  readonly bed: THREE.Mesh;
  readonly sheet: THREE.Mesh;
  private readonly mask: (x: number, z: number) => number;
  private readonly wind: Wind | null;
  private readonly material: THREE.MeshStandardMaterial;
  private time = 0; // seconds of wave motion so far
  private current: Wave[] = calm; // the waves as they are now: the calm ones, higher in wind, and the ripple

  constructor(options: WaterTerrainOptions = {}) {
    super();
    this.name = 'water terrain';
    const theme = options.theme ?? defaultTheme;
    const size = footprintOf(options, SIZE);
    this.width = size.width;
    this.depth = size.depth;
    const seed = options.seed ?? SEED;
    const land = options.heightAt ?? (() => 0);
    const mask = (this.mask = options.mask ?? ovalMask(size, seed));
    this.wind = options.wind ?? null;

    // The bed, and the bank up out of the water.
    const floor = new THREE.Color(theme.scene.floor);
    const water = new THREE.Color(theme.scene.water);
    const bed = surfaceGeometry(size, BED_CELL, mask, land, (x, z, cover, out) => {
      const y = land(x, z);
      const wet = 1 - THREE.MathUtils.smoothstep(y, 0, WET.band);
      out.lift = THREE.MathUtils.lerp(-SINK, LIFT, cover);
      out.color
        .copy(floor)
        .multiplyScalar(THREE.MathUtils.lerp(1, WET.tone, wet))
        .lerp(water, DEEP_TINT * THREE.MathUtils.clamp(-y / DEEP, 0, 1));
      out.soil = THREE.MathUtils.lerp(1, SOIL.deep, THREE.MathUtils.smoothstep(-y, 0, SOIL.over));
      out.wet = wet;
    });
    this.bed = new THREE.Mesh(
      bed,
      surfaceMaterial({ texture: soilTexture(), mean: SOIL_MEAN, bump: BUMP, roughness: BED_ROUGHNESS.dry, wetRoughness: BED_ROUGHNESS.wet }),
    );
    this.bed.name = 'bed';
    this.bed.receiveShadow = true;

    // The sheet: the whole footprint the mask covers, at the water level;
    // the land rising above it hides it. Each point is as clear as the
    // water is shallow there (its alpha).
    const material = (this.material = new THREE.MeshStandardMaterial({
      color: theme.scene.water,
      roughness: ROUGHNESS,
      transparent: true,
      vertexColors: true,
    }));
    waterSurface(material);
    this.sheet = new THREE.Mesh(sheetGeometry(size, mask, land), material);
    this.sheet.name = 'water';
    this.sheet.receiveShadow = true;
    this.add(this.bed, this.sheet);
    this.shape();
  }

  // The water's height at a point (its own coordinates), now.
  heightAt(x: number, z: number): number {
    let y = 0;
    for (const w of this.current) y += w.amplitude * Math.sin(w.kx * x + w.kz * z - w.omega * this.time + w.phase);
    return y;
  }

  covers(x: number, z: number): number {
    return this.mask(x, z);
  }

  // Moves the waves on by delta seconds, as high as the wind makes them now.
  update(delta: number): void {
    this.time += delta;
    const strength = this.wind?.strength.value ?? 0;
    const direction = this.wind?.direction.value ?? EAST;
    const higher = 1 + WIND_WAVES * strength;
    const k = (2 * Math.PI) / RIPPLE.length;
    this.current = [
      ...calm.map((w) => ({ ...w, amplitude: w.amplitude * higher })),
      { kx: k * direction.x, kz: k * direction.y, omega: Math.sqrt(GRAVITY * k), amplitude: (RIPPLE.height / 2) * strength, phase: 0 },
    ];
    this.material.roughness = ROUGHNESS + WIND_ROUGHNESS * strength;
    this.shape();
  }

  // Sets every point of the sheet to the waves' height and its normal to
  // their slope: for y = f(x, z), the normal is (-df/dx, 1, -df/dz).
  private shape(): void {
    const geometry = this.sheet.geometry;
    const position = geometry.attributes.position;
    const normal = geometry.attributes.normal;
    const n = new THREE.Vector3();
    for (let i = 0; i < position.count; i++) {
      const x = position.getX(i);
      const z = position.getZ(i);
      let y = 0;
      let dx = 0;
      let dz = 0;
      for (const w of this.current) {
        const a = w.kx * x + w.kz * z - w.omega * this.time + w.phase;
        y += w.amplitude * Math.sin(a);
        const slope = w.amplitude * Math.cos(a);
        dx += slope * w.kx;
        dz += slope * w.kz;
      }
      position.setY(i, y);
      n.set(-dx, 1, -dz).normalize();
      normal.setXYZ(i, n.x, n.y, n.z);
    }
    position.needsUpdate = true;
    normal.needsUpdate = true;
  }
}

// The sheet: a flat grid over the footprint at the water level, leaving out
// the cells the mask leaves out entirely, white with an alpha for how much of
// the bed it hides at each point, by the depth of the water there. Its
// heights and normals are set by the waves (shape()).
function sheetGeometry(size: Footprint, mask: (x: number, z: number) => number, land: (x: number, z: number) => number): THREE.BufferGeometry {
  const geo = surfaceGeometry(size, SHEET_CELL, mask, () => 0, () => {});
  const position = geo.attributes.position;
  const colors = new Float32Array(position.count * 4).fill(1);
  for (let i = 0; i < position.count; i++) {
    const deep = -land(position.getX(i), position.getZ(i));
    colors[i * 4 + 3] = THREE.MathUtils.lerp(CLEAR.shallow, CLEAR.deep, THREE.MathUtils.smoothstep(deep, 0, CLEAR.over));
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 4));
  return geo;
}
