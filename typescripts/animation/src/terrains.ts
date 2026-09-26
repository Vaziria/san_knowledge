import * as THREE from 'three';
import { GrassTerrain } from './environtments/terrains/GrassTerrain';
import { Land } from './environtments/terrains/Land';
import { MudTerrain } from './environtments/terrains/MudTerrain';
import type { Footprint, Terrain } from './environtments/terrains/parts';
import { SandTerrain } from './environtments/terrains/SandTerrain';
import { SwampTerrain } from './environtments/terrains/SwampTerrain';
import { WaterTerrain } from './environtments/terrains/WaterTerrain';
import { Sky } from './environtments/Sky';
import { createWind, type Wind } from './environtments/wind';
import type { Environment, Preview } from './previews';
import type { Theme } from './theme';

// The terrains (src/environtments/terrains/), the kinds of ground an
// environment's land is put together from, each shown on its own as a figure
// is: the panel's Terrains tab or ?terrain=<name> picks one, and picking a
// figure shows the figure again. A terrain is shown lying on bare land
// (Land.ts) under the sky, in a light breeze that bends its grass and
// ruffles its water, seen whole from a little above; nothing stands on it.
// It brings its own land, as a story brings its environment, so the
// Environments tab rests while it is shown. The tab lists them in this order.
export interface TerrainPreview {
  environment: (theme: Theme) => Environment;
  create: (theme: Theme, environment: Environment) => Preview;
}

const LAND_RADIUS = 40; // m: the bare land's end, fading into the sky
const BREEZE = 0.35; // the wind's strength, 0 still to 1 strong

// The land under a terrain: low humps, so the terrain is seen lying over
// uneven ground. Each is a wave running across it, `height` m either way and
// `length` m from crest to crest.
const ROLLS = [
  { height: 0.06, length: 7, angle: 0.5, phase: 0.8 },
  { height: 0.045, length: 4.3, angle: 2.1, phase: 2.3 },
  { height: 0.025, length: 2.6, angle: 3.7, phase: 4.4 },
];

function rolling(x: number, z: number): number {
  let y = 0;
  for (const w of ROLLS) y += w.height * Math.sin(((2 * Math.PI) / w.length) * (x * Math.cos(w.angle) + z * Math.sin(w.angle)) + w.phase);
  return y;
}

// The water terrain's land: a pond's hollow, `deep` m deep in the middle
// below the water level (y = 0), its shore an uneven oval about `width` by
// `depth` m, and a bank rising `rise` m above the water over `bank` m past
// the shore, then the humps as elsewhere. The water terrain's footprint
// reaches past the top of the bank all round, so the land there hides its
// sheet's edge.
const POND = { width: 6.4, depth: 4.8, deep: 1, bank: 0.8, rise: 0.3, humps: 1.5 };

function pond(x: number, z: number): number {
  const u = x / (POND.width / 2);
  const v = z / (POND.depth / 2);
  const angle = Math.atan2(v, u);
  const shore = 1 + 0.08 * Math.sin(2 * angle + 0.7) + 0.05 * Math.sin(3 * angle + 2.1);
  const f = Math.hypot(u, v) / shore; // 1 on the shore
  if (f <= 1) return -POND.deep * (1 - f * f);
  const past = (f - 1) * shore * Math.hypot((POND.width / 2) * Math.cos(angle), (POND.depth / 2) * Math.sin(angle)); // m, about
  const bank = POND.rise * Math.sin((Math.PI / 2) * Math.min(1, past / POND.bank));
  return bank + rolling(x, z) * THREE.MathUtils.smoothstep(past, POND.bank, POND.bank + POND.humps);
}

// The swamp terrain's land: a broad, shallow hollow, `deep` m below the water
// level (y = 0) in the middle, its shore an uneven oval about `width` by
// `depth` m, rising gently past the shore toward `top` m above the water;
// humps across its floor (HUMPS) rise out of the water as islands, most of
// them toward the shore, where it is shallow, and wind the shore, dying away
// past it; further out the land's humps as elsewhere. The swamp's footprint
// reaches well past the shore all round, where the land stands above the
// water and hides its edge.
//
// It is smooth everywhere, a bell (a Gaussian) turned upside down, with no
// crease at the shore: the bare land's mesh (Land.ts, rings 25 cm apart) cuts
// straight across a crease or a tight hollow, and there it came up through
// the swamp's ground. So the humps are no shorter than 2.6 m from crest to
// crest, and fade in and out slowly.
const BASIN = { width: 6.6, depth: 5, deep: 0.18, top: 0.45, humps: [0.9, 1.5] as const, rolling: [1.6, 2.6] as const };
const HUMPS = [
  { height: 0.06, length: 3.1, angle: 0.3, phase: 1.1 },
  { height: 0.05, length: 2.6, angle: 1.9, phase: 2.9 },
  { height: 0.04, length: 4.2, angle: 3.2, phase: 0.4 },
];
// How wide the bell is, so that it crosses the water level on the shore.
const BELL = 1 / Math.sqrt(Math.log((BASIN.top + BASIN.deep) / BASIN.top));

function basin(x: number, z: number): number {
  const u = x / (BASIN.width / 2);
  const v = z / (BASIN.depth / 2);
  const angle = Math.atan2(v, u);
  const shore = 1 + 0.1 * Math.sin(2 * angle + 1.9) + 0.06 * Math.sin(3 * angle + 0.4);
  const f = Math.hypot(u, v) / shore; // 0 in the middle, 1 on the shore (before the humps)
  let humps = 0;
  for (const w of HUMPS) humps += w.height * Math.sin(((2 * Math.PI) / w.length) * (x * Math.cos(w.angle) + z * Math.sin(w.angle)) + w.phase);
  const bell = BASIN.top - (BASIN.top + BASIN.deep) * Math.exp(-((f / BELL) ** 2));
  return (
    bell +
    humps * (1 - THREE.MathUtils.smoothstep(f, ...BASIN.humps)) +
    rolling(x, z) * THREE.MathUtils.smoothstep(f, ...BASIN.rolling)
  );
}

export const terrains: Record<string, TerrainPreview> = {
  'grass-terrain': onLand(GrassTerrain.SIZE, (theme, wind) => new GrassTerrain({ theme, wind, heightAt: rolling })),
  'mud-terrain': onLand(MudTerrain.SIZE, (theme) => new MudTerrain({ theme, heightAt: rolling })),
  // Walked over on the still water, as on the lake.
  'water-terrain': onLand(WaterTerrain.SIZE, (theme, wind) => new WaterTerrain({ theme, wind, heightAt: pond }), pond, (x, z) =>
    Math.max(pond(x, z), 0),
  ),
  'sand-terrain': onLand(SandTerrain.SIZE, (theme) => new SandTerrain({ theme, heightAt: rolling })),
  // Walked over on the still water, as the water terrain is.
  'swamp-terrain': onLand(SwampTerrain.SIZE, (theme, wind) => new SwampTerrain({ theme, wind, heightAt: basin }), basin, (x, z) =>
    Math.max(basin(x, z), 0),
  ),
};

// A terrain lying on bare land shaped by `land`, with its own breeze and
// sky: the environment it is shown in, and the view of it. The camera
// stands off its front-right, 28° up, far enough back to take in the whole
// terrain and its edge, and looks right of its middle, which puts the
// terrain left of the middle of the screen, clear of the panel; zoom in or
// walk for a closer look. `walk` is where the walking camera stands.
function onLand(
  size: Footprint,
  build: (theme: Theme, wind: Wind) => Terrain,
  land: (x: number, z: number) => number = rolling,
  walk: (x: number, z: number) => number = land,
): TerrainPreview {
  const reach = Math.max(size.width, size.depth);
  const target = new THREE.Vector3(0.28 * size.width, 0, 0);
  const camera = new THREE.Vector3(0.5, 0.5, 0.8).normalize().multiplyScalar(0.85 * reach + 1.5).add(target);
  return {
    environment: (theme) => {
      const wind = createWind();
      wind.strength.value = BREEZE;
      const terrain = build(theme, wind);
      const sky = new Sky({ theme, wind });
      const scenery = new THREE.Group();
      scenery.add(new Land({ theme, radius: LAND_RADIUS, heightAt: land }), terrain, sky);
      return {
        scenery,
        land: new THREE.Vector3(0, walk(0, 0), 0),
        update: (delta) => {
          wind.time.value += delta;
          terrain.update(delta);
          sky.update(delta);
        },
        ground: { heightAt: walk, reach: LAND_RADIUS - 1 },
      };
    },
    // Nothing stands on it: the terrain is the environment's. The sun's
    // shadow covers its footprint.
    create: () => ({
      figure: new THREE.Group(),
      camera: [camera.x, camera.y, camera.z],
      target: [target.x, target.y, target.z],
      bounds: new THREE.Box3(new THREE.Vector3(-size.width / 2, -0.2, -size.depth / 2), new THREE.Vector3(size.width / 2, 0.3, size.depth / 2)),
      update() {},
    }),
  };
}
