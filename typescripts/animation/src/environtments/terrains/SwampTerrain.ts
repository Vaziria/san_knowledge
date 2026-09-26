import * as THREE from 'three';
import { SOIL_MEAN, soilTexture } from '../../textures/soil';
import { defaultTheme } from '../../theme';
import { tuftGeometry, type TuftShape } from '../Lake/Sward';
import { waterSurface } from '../water';
import { sway, type Wind } from '../wind';
import {
  between,
  footprintOf,
  landNormal,
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

// Swamp terrain: waterlogged ground for an environment's land (./parts.ts),
// as the user asked ("add swamps terrain"). Still, murky water stands in the
// land's low places, dark with peat and green with algae, with duckweed
// floating on it in patches, thickest in the shallows. Hummocks of peat
// stand out of it, each topped by a tussock of sedge, and stands of cattails
// grow in the shallows and along the shores, their brown heads on stems
// among their long leaves. Round the water the ground is dark, wet mud, and
// higher up peat and moss with short grass and patches of bare mud, until it
// meets the bare land at its edge.
//
// As for the water terrain (WaterTerrain.ts), the environment shapes the
// land: the water lies at the terrain's origin (y = 0) wherever the ground is
// lower, so the land must lie below 0 where there is to be water and rise
// above it all round, inside the footprint, which hides the water's edge. On
// land above 0 all over it is only its wet ground, sedges and reeds.
//
// Colours come from the theme, all but the plants' well darkened: the mud
// from the bare land's (theme.scene.floor) turned toward the wood's brown
// (theme.colors.wood), the peat and moss from the grass (theme.scene.grass),
// the water the theme's water turned toward the grass, the duckweed, sedges
// and reeds the grass, and dry blades and the cattails' heads the wood
// (straw, and dark brown).
// Units are meters, y is up, and the origin is the middle of its footprint on
// the still water. Built from a seed. The spec, SwampTerrain.md, is a draft:
// it names no behaviour. Its water moves in small waves (update) and its
// plants bend in a wind (`wind`, which the environment owns and moves).
// Plants take shadows but cast none.

export interface SwampTerrainOptions extends TerrainOptions {
  wind?: Wind; // its plants bend in it and its water ripples; still without it
  reeds?: number; // stands of cattails, where the water is shallow; about one for every 6 m² by default
  duckweed?: number; // how much of its open water duckweed covers, 0 to 1; about a third by default
  keepOff?: (x: number, z: number) => boolean; // no sedge or reed grows where this says true (at a trunk, under a stone)
  rim?: THREE.ColorRepresentation; // the colour its ground fades to at its edge, the land's there: the bare land's, a shade darker, by default; a lawn's on the lake (Lake/Swamps.ts)
}

const SIZE: Footprint = { width: 10, depth: 8 };
const SEED = 1;
const GROUND_CELL = 0.15; // m between the ground's points: fine enough for a hummock's sides
const SHEET_CELL = 0.16; // m between the water's points: 5 to its ripple
const GRAVITY = 9.81;

// The water lies still, sheltered by the sedges and reeds: waves a third as
// high as a pond's (WaterTerrain.ts). [length (m), height from trough to
// crest (m), direction it travels (radians from +x toward +z), phase].
const WAVES: [length: number, height: number, direction: number, phase: number][] = [
  [3.4, 0.008, 0.6, 0],
  [2.1, 0.006, 1.7, 1.3],
  [1.3, 0.004, -0.4, 3.9],
];
const WIND_WAVES = 0.8; // in a full wind the waves are this much higher again
const RIPPLE = { length: 0.8, height: 0.006 }; // m: the short wave a full wind raises, running downwind
const EAST = new THREE.Vector2(1, 0); // the way the ripple would run, in still air (it has no height then)
// The waves die away in the shallows, flat at the water's edge, so the water
// meets the ground in a crisp line and never washes over ground just above
// it, which would flicker (Environments rule 7): full from `full` m deep.
const SHALLOWS = { full: 0.1 };

// The water: the theme's water turned toward the grass (the peat and algae
// that make it murky) and darkened well, dark as a swamp's: turned halfway
// and half as bright, it was a pale grey-green. It hides much more of the
// bed than a pond's: `shallow` of it at the water's edge, up to `deep` from
// `over` m down.
const MURK = { grass: 0.7, tone: 0.17 };
const CLEAR = { shallow: 0.6, deep: 0.95, over: 0.3 };
const ROUGHNESS = 0.16; // still water, with a glint
const WIND_ROUGHNESS = 0.2; // how much rougher it is in a full wind
// And at night and in the heaviest shower (setNight), as the lake's water
// (Lake/Water.ts): the moon's sharp glint read as a lamp on it.
const NIGHT_ROUGHNESS = 0.2;
const RAIN_ROUGHNESS = 0.3;
// Duckweed floats in patches about `size` m across, drifted into the
// shallows (`shore`: how much likelier it is in the first 15 cm of depth),
// soft-edged (`soft`); the grass colour made brighter (`tone`), and matte.
const DUCKWEED = { share: 0.3, size: 0.8, shore: 0.25, soft: 0.04, tone: 1.1, roughness: 0.9, seed: 11 };
const HIDDEN = 0.01; // m above the still water: the water is left out where the ground stands this high all round a cell (it is flat there)

// The ground: mud, the bare land's colour turned halfway toward the wood's
// brown (EARTH), dark and wet (with a sheen) within `band` m above the
// water; above that peat and moss, the grass colour darkened, mottled, with
// patches of bare, damp mud (about `bare` of it); under the water the bed,
// fading toward the water's murk with depth; and toward its edge the bare
// land's colour, a shade darker, where it sinks under the land. The first
// swamp's mud was the land's colour alone (× 0.3 wet, × 0.5 damp) and its
// moss that colour turned 62% toward the grass (× 0.66): pale grey mud and
// grey-green moss, little darker than the land round it. With a third of the
// wood's brown, the bare patches looked mauve beside the moss.
const EARTH = { wood: 0.5 };
const MUD = { wet: 0.11, damp: 0.2, band: 0.1 };
const MOSS = { tone: 0.38, mottle: 0.12, size: 1.1, bare: 0.3, seed: 13 };
// The hummocks are peat grown over with moss, darker than the moss round
// them (`tone`), and never bare: when bare mud was the land's light colour
// darkened, their bare tops looked like pale stones.
const PEAT_MOUND = { tone: 0.75, from: 0.01 }; // m a hummock raises the ground where its colour starts
// m the peat lies on the land where it is all swamp: thicker than the other
// terrains' LIFT, as deep as the sand, so the bare land's mesh, which cuts
// across the land's hollows in straight lines, stays under it.
const PEAT = 0.03;
const BED = { tint: 0.8, deep: 0.5 }; // the bed has faded `tint` of the way to the murk `deep` m down
const SOIL = { bed: 0.1, wet: 0.2, damp: 0.6, moss: 0.35 }; // how much of the soil texture shows
// The wet mud has a sheen, duller than the mud terrain's (0.4): with the
// sun behind the preview camera, a brighter one lit the shore up in a pale
// rim like a beach's.
const GROUND_ROUGHNESS = { dry: 0.95, wet: 0.55 };
const BUMP = 0.012; // m, the soil's bump height, as on the lake's ground
const LUMPS = { height: 0.008, size: 0.3, seed: 7 }; // m the peat rises and dips above the wet band; the water's edge is smooth, so no lump pokes through the water
const RIM = 0.9; // toward its edge, times the land's colour

// Hummocks: mounds of peat standing out of the shallow water and at its
// edge, each topped by a tussock of sedge. One for every `every` m² of ground
// from `from` m under the water to `to` m above it, `radius` m round their
// foot, uneven (`wobble`), their tops `rise` m above the water, or above the
// land where it stands out of the water.
const HUMMOCKS = {
  every: 2.2,
  from: -0.22,
  to: 0.06,
  radius: [0.25, 0.5] as const,
  wobble: 0.15,
  rise: [0.07, 0.16] as const,
  apart: 0.15, // m between two
  inside: 0.9, // how much the ground must be swamp where one stands
};

// Sedges and grass, each shape one instanced mesh: a tussock of long arching
// blades on each hummock; upright clumps on the wet ground by the water, in
// the shallowest water and among the tussocks; and short grass on the peat
// above the wet band, the sward's fine, floppy tuft.
const SEDGES: TuftShape[] = [
  { blades: 16, width: 0.04, spread: 0.5, lean: 0.75 }, // a tussock: a fountain of arching blades
  { blades: 11, width: 0.05, spread: 0.32, lean: 0.45 }, // an upright clump
  { blades: 8, width: 0.05, spread: 0.55, lean: 0.6 }, // short grass
];
const TUSSOCK = { height: [0.45, 0.8] as const, wide: 1.4, shape: 0 }; // m tall, from the smallest hummock to the largest; this much wider than tall
const CLUMPS = { density: 2.2, height: [0.25, 0.5] as const, from: -0.04, to: 0.3, shape: 1 }; // a square meter; m; from `from` m under the water to `to` above it, thinning upward
const GRASS = { density: 18, height: [0.08, 0.22] as const, shape: 2 }; // a square meter of peat
const SHORT = 1.3; // the power on a random number that makes most of them short
const SEDGE_TINT = [0.75, 1.05] as const; // darkest and lightest, times the grass colour
const SEDGE_DRY = { share: 0.35, amount: [0.2, 0.5] as const }; // blades gone toward straw, and how far
const SEDGE_BEND = { sway: 0.12, reach: 0.6 };
const FOLLOW = 0.5; // how far a plant leans with the ground's slope

// Reeds: stands of cattails in the water from `shallowest` m above its level
// (at its edge) to `deepest` m under it, the shallows likelier (a stand
// deeper than `open` m is kept only now and then), each stand `clumps`
// clumps within `radius` m. A clump is long, strap-like leaves round its
// foot and, on most, one or two stems carrying a brown head with a thin
// spike above it. Clumps are built KINDS ways, each shared by its instances,
// `NOMINAL` m tall and scaled to their height, their widths by its square
// root.
const REEDS = {
  every: 4.5, // m² of the footprint's oval for each stand, by default
  clumps: [4, 11] as const,
  radius: [0.3, 0.8] as const,
  shallowest: 0.05,
  deepest: 0.35,
  open: 0.15,
  height: [1.3, 1.9] as const, // m
  inside: 0.85, // how much the ground must be swamp at a stand's middle; its clumps take 0.6
  tint: [0.85, 1.08] as const, // each clump's shade
  tilt: 0.06, // radians a clump leans, at most
};
const KINDS = 4;
const NOMINAL = 1.6; // m
const LEAF = {
  count: [4, 7] as const,
  length: [0.65, 1] as const, // times NOMINAL
  width: 0.028, // m: broad, as a cattail's, and drawn a little broader than real so it survives the painterly filter
  segments: 5,
  out: 0.05, // how far a leaf leans out, as a share of its length
  arch: [0.1, 0.35] as const, // how far it arches over near its tip
  foot: 0.025, // m: the leaves rise from a circle this wide
  shade: 0.6, // its foot, times its tip's colour
  dry: { share: 0.25, amount: [0.15, 0.45] as const }, // leaves browning toward straw from their tips
};
const STEM = { chance: 0.65, second: 0.35, height: [1, 1.12] as const, radius: [0.005, 0.0035] as const, sides: 5, foot: 0.015, tilt: 0.06, tone: 0.9 };
// A cattail's head: a brown cylinder rounded at its ends, drawn thicker
// than real (2.5 cm across is typical) so it still reads from meters away,
// with a spike of straw above it.
const HEAD = { radius: 0.022, length: 0.2, sides: 8, spike: 0.09, spikeRadius: 0.003, tone: 0.42, spikeTone: 0.9 };
const REED_BEND = { sway: 0.22, reach: 1.6 };

const UP = new THREE.Vector3(0, 1, 0);

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

export interface Hummock {
  x: number;
  z: number;
  radius: number;
  height: number; // m it raises the ground at its middle
  seed: number; // its own uneven outline
}

interface Placed {
  matrix: THREE.Matrix4;
  color: THREE.Color;
}

export class SwampTerrain extends THREE.Group implements Terrain {
  static readonly SIZE = SIZE;

  readonly width: number;
  readonly depth: number;
  readonly ground: THREE.Mesh;
  readonly water: THREE.Mesh;
  readonly sedges: THREE.InstancedMesh[] = [];
  readonly reeds: THREE.InstancedMesh[] = [];
  readonly hummocks: readonly Hummock[];
  private readonly land: (x: number, z: number) => number;
  private readonly mask: (x: number, z: number) => number;
  private readonly lift: (x: number, z: number) => number;
  private readonly wind: Wind | null;
  private readonly waterMaterial: THREE.MeshStandardMaterial;
  private readonly swell: Float32Array; // how much of the waves each point of the water takes: none at the water's edge
  private time = 0; // seconds of wave motion so far
  private current: Wave[] = calm; // the waves as they are now: the calm ones, higher in wind, and the ripple
  private night = 0; // how far into the night it is, and how hard it rains (setNight)
  private rain = 0;

  constructor(options: SwampTerrainOptions = {}) {
    super();
    this.name = 'swamp terrain';
    const theme = options.theme ?? defaultTheme;
    const size = footprintOf(options, SIZE);
    this.width = size.width;
    this.depth = size.depth;
    const seed = options.seed ?? SEED;
    const land = (this.land = options.heightAt ?? (() => 0));
    const mask = (this.mask = options.mask ?? ovalMask(size, seed));
    this.wind = options.wind ?? null;
    const random = seededRandom(seed);
    const keepOff = options.keepOff ?? (() => false);

    const hummocks = (this.hummocks = placeHummocks(size, land, mask, keepOff, random));

    // How far the ground lies above the land: PEAT where it is all swamp,
    // with the peat's lumps above the wet band and the hummocks' mounds, and
    // sinking under the land at its edge.
    this.lift = (x, z) => {
      const cover = mask(x, z);
      const base = land(x, z) + PEAT;
      const lumps = LUMPS.height * wobble(x / LUMPS.size, z / LUMPS.size, LUMPS.seed + seed) * THREE.MathUtils.smoothstep(base, MUD.band, MUD.band + 0.1);
      return THREE.MathUtils.lerp(-SINK, PEAT + lumps, cover) + moundAt(hummocks, x, z) * cover * cover;
    };

    // The ground: the bed under the water, the mud at its edge, the peat above.
    const floor = new THREE.Color(theme.scene.floor);
    const grass = new THREE.Color(theme.scene.grass);
    const straw = new THREE.Color(theme.colors.wood);
    const murk = new THREE.Color(theme.scene.water).lerp(grass, MURK.grass).multiplyScalar(MURK.tone);
    const earth = floor.clone().lerp(straw, EARTH.wood);
    const mudWet = earth.clone().multiplyScalar(MUD.wet);
    const mudDamp = earth.clone().multiplyScalar(MUD.damp);
    const moss = grass.clone().multiplyScalar(MOSS.tone);
    const rim = options.rim !== undefined ? new THREE.Color(options.rim) : floor.clone().multiplyScalar(RIM);
    const ground = surfaceGeometry(size, GROUND_CELL, mask, land, (x, z, cover, out) => {
      out.lift = this.lift(x, z);
      const y = land(x, z) + out.lift; // m over the still water
      if (y < 0) {
        out.color.copy(mudWet).lerp(murk, BED.tint * Math.min(1, -y / BED.deep));
        out.soil = SOIL.bed;
        out.wet = 1;
      } else {
        const wet = 1 - THREE.MathUtils.smoothstep(y, 0, MUD.band);
        const mound = THREE.MathUtils.smoothstep(moundAt(hummocks, x, z), PEAT_MOUND.from, 3 * PEAT_MOUND.from);
        const bare = (1 - mound) * THREE.MathUtils.smoothstep(0.5 + 0.5 * mottle(x, z, MOSS.size, MOSS.seed + seed + 1), 1 - MOSS.bare - 0.05, 1 - MOSS.bare + 0.05);
        out.color
          .copy(moss)
          .multiplyScalar((1 + MOSS.mottle * mottle(x, z, MOSS.size, MOSS.seed + seed)) * THREE.MathUtils.lerp(1, PEAT_MOUND.tone, mound))
          .lerp(mudDamp, bare)
          .lerp(mudWet, wet * (1 - mound));
        out.soil = THREE.MathUtils.lerp(THREE.MathUtils.lerp(SOIL.moss, SOIL.damp, bare), SOIL.wet, wet);
        out.wet = Math.max(wet, 0.3 * bare);
      }
      out.color.lerp(rim, 1 - THREE.MathUtils.smoothstep(cover, 0.15, 0.85));
    });
    this.ground = new THREE.Mesh(
      ground,
      surfaceMaterial({ texture: soilTexture(), mean: SOIL_MEAN, bump: BUMP, roughness: GROUND_ROUGHNESS.dry, wetRoughness: GROUND_ROUGHNESS.wet }),
    );
    this.ground.name = 'ground';
    this.ground.receiveShadow = true; // it casts none worth drawing
    this.add(this.ground);

    // The water: a sheet at the still water's level, murkier where it is
    // deeper (its alpha), with the duckweed painted on it. The duckweed is
    // matte where the water glints (its `scum` attribute).
    const material = (this.waterMaterial = new THREE.MeshStandardMaterial({ roughness: ROUGHNESS, transparent: true, vertexColors: true }));
    waterSurface(material);
    material.onBeforeCompile = (shader) => {
      shader.vertexShader = shader.vertexShader
        .replace('#include <common>', '#include <common>\nattribute float scum;\nvarying float vScum;')
        .replace('#include <begin_vertex>', '#include <begin_vertex>\nvScum = scum;');
      shader.fragmentShader = shader.fragmentShader
        .replace('#include <common>', '#include <common>\nvarying float vScum;')
        .replace('#include <roughnessmap_fragment>', `#include <roughnessmap_fragment>\nroughnessFactor = mix(roughnessFactor, ${DUCKWEED.roughness.toFixed(2)}, vScum);`);
    };
    material.customProgramCacheKey = () => 'swamp-water';
    const duckweed = grass.clone().multiplyScalar(DUCKWEED.tone);
    this.water = new THREE.Mesh(
      sheetGeometry(size, (x, z) => (this.flooded(x, z) ? mask(x, z) : 0), (x, z) => this.groundAt(x, z), murk, duckweed, options.duckweed ?? DUCKWEED.share, seed),
      material,
    );
    this.water.name = 'water';
    const sheet = this.water.geometry.attributes.position;
    this.swell = new Float32Array(sheet.count);
    for (let i = 0; i < sheet.count; i++) this.swell[i] = swellAt(-this.groundAt(sheet.getX(i), sheet.getZ(i)));
    this.water.receiveShadow = true;
    this.add(this.water);

    // The sedges and the grass.
    const green = grass;
    const sedges: Placed[][] = SEDGES.map(() => []);
    const sedge = (shape: number, x: number, z: number, height: number, wide: number) => {
      const tint = between(random, ...SEDGE_TINT);
      const dry = random() < SEDGE_DRY.share ? between(random, ...SEDGE_DRY.amount) : 0;
      const spin = 2 * Math.PI * random();
      const lean = new THREE.Quaternion().setFromUnitVectors(UP, landNormal((px, pz) => this.groundAt(px, pz), x, z)).slerp(new THREE.Quaternion(), 1 - FOLLOW);
      sedges[shape].push({
        matrix: new THREE.Matrix4().compose(
          new THREE.Vector3(x, this.groundAt(x, z), z),
          lean.multiply(new THREE.Quaternion().setFromAxisAngle(UP, spin)),
          new THREE.Vector3(height * wide, height, height * wide),
        ),
        color: green.clone().lerp(straw, dry).multiplyScalar(tint),
      });
    };
    // A tussock on each hummock, bigger on a bigger one.
    for (const h of hummocks) {
      const t = (h.radius - HUMMOCKS.radius[0]) / (HUMMOCKS.radius[1] - HUMMOCKS.radius[0]);
      sedge(TUSSOCK.shape, h.x, h.z, THREE.MathUtils.lerp(...TUSSOCK.height, t) * between(random, 0.9, 1.1), TUSSOCK.wide);
    }
    // Clumps by the water, thinning as the ground rises; short grass on the
    // peat. Spots are picked evenly over the footprint and kept as often as
    // the ground there is swamp (squared, so they thin out before its edge).
    const area = size.width * size.depth;
    for (let k = 0; k < Math.round(CLUMPS.density * area); k++) {
      const x = size.width * (random() - 0.5);
      const z = size.depth * (random() - 0.5);
      const keep = random();
      const height = THREE.MathUtils.lerp(...CLUMPS.height, random() ** SHORT);
      const cover = mask(x, z);
      const y = this.groundAt(x, z);
      if (y < CLUMPS.from || y > CLUMPS.to || moundAt(hummocks, x, z) > 0.02 || keepOff(x, z)) continue;
      if (keep >= cover * cover * (1 - THREE.MathUtils.smoothstep(y, MUD.band / 2, CLUMPS.to))) continue;
      sedge(CLUMPS.shape, x, z, height, 1);
    }
    for (let k = 0; k < Math.round(GRASS.density * area); k++) {
      const x = size.width * (random() - 0.5);
      const z = size.depth * (random() - 0.5);
      const keep = random();
      const height = THREE.MathUtils.lerp(...GRASS.height, random() ** SHORT);
      const cover = mask(x, z);
      if (keep >= cover * cover || this.groundAt(x, z) < MUD.band || keepOff(x, z)) continue;
      sedge(GRASS.shape, x, z, height, 1);
    }
    const sedgeMaterial = new THREE.MeshStandardMaterial({ roughness: 0.9, vertexColors: true });
    if (this.wind) sway(sedgeMaterial, this.wind, SEDGE_BEND);
    SEDGES.forEach((shape, s) => this.sedges.push(this.instanced('sedges', tuftGeometry(shape, random), sedgeMaterial, sedges[s])));

    // The reeds, in stands in the shallow water and at its edge.
    const palette = {
      leaf: green,
      straw,
      stem: green.clone().multiplyScalar(STEM.tone),
      head: straw.clone().multiplyScalar(HEAD.tone),
      spike: straw.clone().multiplyScalar(HEAD.spikeTone),
    };
    const kinds = Array.from({ length: KINDS }, () => reedGeometry(random, palette));
    const reeds: Placed[][] = kinds.map(() => []);
    const shallow = (x: number, z: number) => {
      const y = this.groundAt(x, z);
      return y <= REEDS.shallowest && y >= -REEDS.deepest;
    };
    const stands = options.reeds ?? Math.round(((Math.PI / 4) * area) / REEDS.every);
    for (let placed = 0, attempt = 0; placed < stands && attempt < 60 * stands; attempt++) {
      const cx = size.width * (random() - 0.5);
      const cz = size.depth * (random() - 0.5);
      const keep = random();
      if (mask(cx, cz) < REEDS.inside || !shallow(cx, cz) || moundAt(hummocks, cx, cz) > 0) continue;
      if (-this.groundAt(cx, cz) > REEDS.open && keep > 0.25) continue;
      placed++;
      const radius = between(random, ...REEDS.radius);
      const count = Math.round(between(random, ...REEDS.clumps));
      for (let k = 0; k < count; k++) {
        const r = radius * Math.sqrt(random());
        const angle = 2 * Math.PI * random();
        const x = cx + r * Math.cos(angle);
        const z = cz + r * Math.sin(angle);
        const kind = Math.floor(random() * KINDS);
        const scale = between(random, ...REEDS.height) / NOMINAL;
        const tint = between(random, ...REEDS.tint);
        const spin = 2 * Math.PI * random();
        const tilt = REEDS.tilt * random();
        const way = 2 * Math.PI * random();
        if (mask(x, z) < 0.6 || !shallow(x, z) || moundAt(hummocks, x, z) > 0 || keepOff(x, z)) continue;
        const lean = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(Math.cos(way), 0, Math.sin(way)), tilt);
        const root = Math.sqrt(scale);
        reeds[kind].push({
          matrix: new THREE.Matrix4().compose(
            new THREE.Vector3(x, this.groundAt(x, z), z),
            lean.multiply(new THREE.Quaternion().setFromAxisAngle(UP, spin)),
            new THREE.Vector3(root, scale, root),
          ),
          color: new THREE.Color().setScalar(tint),
        });
      }
    }
    const reedMaterial = new THREE.MeshStandardMaterial({ roughness: 0.85, vertexColors: true });
    if (this.wind) sway(reedMaterial, this.wind, REED_BEND);
    kinds.forEach((geo, k) => this.reeds.push(this.instanced('reeds', geo, reedMaterial, reeds[k])));

    this.shape();
  }

  // The ground's height at a point (its own coordinates): the bed where the
  // water covers it.
  groundAt(x: number, z: number): number {
    return this.land(x, z) + this.lift(x, z);
  }

  // Its top at a point: the water's surface now where the water covers the
  // ground, otherwise the ground.
  heightAt(x: number, z: number): number {
    const ground = this.groundAt(x, z);
    return this.flooded(x, z) ? Math.max(ground, this.waterAt(x, z)) : ground;
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
    this.waterMaterial.roughness = ROUGHNESS + WIND_ROUGHNESS * strength + NIGHT_ROUGHNESS * this.night + RAIN_ROUGHNESS * this.rain;
    this.shape();
  }

  // How far into the night it is (0 day, 1 night) and how hard it rains (0
  // to 1), from an environment with day and night and weather (the lake):
  // both dull its water's glint. Call before update() each frame.
  setNight(night: number, rain = 0): void {
    this.night = night;
    this.rain = rain;
  }

  // Whether there is water at a point: where the land lies below the water's
  // level, inside the outline. Not where the land is at it or above, even
  // where the ground sinks under the land at its edge: the land shows there.
  private flooded(x: number, z: number): boolean {
    return this.land(x, z) < 0 && this.mask(x, z) > 0;
  }

  // The water's height at a point, now: the waves, as much of them as the
  // depth there lets through.
  private waterAt(x: number, z: number): number {
    let y = 0;
    for (const w of this.current) y += w.amplitude * Math.sin(w.kx * x + w.kz * z - w.omega * this.time + w.phase);
    return y * swellAt(-this.groundAt(x, z));
  }

  // Sets every point of the water to the waves' height and its normal to
  // their slope: for y = f(x, z), the normal is (-df/dx, 1, -df/dz). Each
  // point takes its share of them (swell), and its slope with it; the slope
  // of that share itself is left out, as it changes slowly.
  private shape(): void {
    const geometry = this.water.geometry;
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
      const swell = this.swell[i];
      position.setY(i, y * swell);
      n.set(-dx * swell, 1, -dz * swell).normalize();
      normal.setXYZ(i, n.x, n.y, n.z);
    }
    position.needsUpdate = true;
    normal.needsUpdate = true;
  }

  // One instanced mesh of a plant's shape, placed and shaded.
  private instanced(name: string, geometry: THREE.BufferGeometry, material: THREE.Material, placed: Placed[]): THREE.InstancedMesh {
    const mesh = new THREE.InstancedMesh(geometry, material, placed.length);
    mesh.name = name;
    placed.forEach(({ matrix, color }, i) => {
      mesh.setMatrixAt(i, matrix);
      mesh.setColorAt(i, color);
    });
    mesh.receiveShadow = true;
    this.add(mesh);
    return mesh;
  }
}

// The hummocks, at random spots from a little under the water to a little
// above it, apart from each other: as many as the ground there has room for.
// The spots are tried at random until there are enough or none will fit.
function placeHummocks(
  size: Footprint,
  land: (x: number, z: number) => number,
  mask: (x: number, z: number) => number,
  keepOff: (x: number, z: number) => boolean,
  random: () => number,
): Hummock[] {
  const fits = (x: number, z: number) => {
    const y = land(x, z) + PEAT;
    return mask(x, z) >= HUMMOCKS.inside && y >= HUMMOCKS.from && y <= HUMMOCKS.to && !keepOff(x, z);
  };
  // How much ground they may stand on, from a grid over the footprint.
  const STEP = 0.25;
  let cells = 0;
  for (let x = -size.width / 2; x < size.width / 2; x += STEP) for (let z = -size.depth / 2; z < size.depth / 2; z += STEP) if (fits(x, z)) cells++;
  const count = Math.round((cells * STEP * STEP) / HUMMOCKS.every);
  const hummocks: Hummock[] = [];
  for (let attempt = 0; hummocks.length < count && attempt < 80 * count; attempt++) {
    const radius = between(random, ...HUMMOCKS.radius);
    const x = (size.width / 2 - radius) * (2 * random() - 1);
    const z = (size.depth / 2 - radius) * (2 * random() - 1);
    const rise = between(random, ...HUMMOCKS.rise);
    const edge = Math.floor(random() * 1e6);
    if (!fits(x, z)) continue;
    if (hummocks.some((h) => Math.hypot(h.x - x, h.z - z) < (h.radius + radius) * (1 + HUMMOCKS.wobble) + HUMMOCKS.apart)) continue;
    const base = land(x, z) + PEAT;
    hummocks.push({ x, z, radius, height: Math.max(base, 0) + rise - base, seed: edge });
  }
  return hummocks;
}

// How much of the waves water `depth` m deep takes: none at its edge, all of
// them from SHALLOWS.full down.
function swellAt(depth: number): number {
  return THREE.MathUtils.smoothstep(depth, 0, SHALLOWS.full);
}

// How much the hummocks raise the ground at a point: a smooth mound over
// each one's uneven outline, highest at its middle.
function moundAt(hummocks: readonly Hummock[], x: number, z: number): number {
  let most = 0;
  for (const h of hummocks) {
    const dx = x - h.x;
    const dz = z - h.z;
    const d = Math.hypot(dx, dz);
    if (d >= h.radius * (1 + HUMMOCKS.wobble)) continue;
    const angle = Math.atan2(dz, dx);
    const t = d / (h.radius * (1 + HUMMOCKS.wobble * wobble(2 * Math.cos(angle), 2 * Math.sin(angle), h.seed)));
    if (t < 1) most = Math.max(most, h.height * (1 - t * t) ** 2);
  }
  return most;
}

// The water's sheet: a grid over the footprint at the still water's level,
// leaving out the cells with no water (`water`: the swamp's mask where the
// land is under the water, 0 elsewhere) and those where the ground stands
// above the water all round, coloured with the murk and the duckweed, and
// with an alpha for how much of the bed it hides, by the depth of the water
// there. Its heights and normals are set by the waves (shape()).
function sheetGeometry(
  size: Footprint,
  water: (x: number, z: number) => number,
  groundAt: (x: number, z: number) => number,
  murk: THREE.Color,
  duckweed: THREE.Color,
  share: number,
  seed: number,
): THREE.BufferGeometry {
  const geo = surfaceGeometry(size, SHEET_CELL, (x, z) => (groundAt(x, z) >= HIDDEN ? 0 : water(x, z)), () => 0, () => {});
  const position = geo.attributes.position;
  const count = position.count;
  const depths = new Float32Array(count);
  const drift = new Float32Array(count); // how likely duckweed is at each point: its patches, and more in the shallows
  for (let i = 0; i < count; i++) {
    const x = position.getX(i);
    const z = position.getZ(i);
    depths[i] = -groundAt(x, z);
    drift[i] = 0.5 + 0.5 * mottle(x, z, DUCKWEED.size, DUCKWEED.seed + seed) + DUCKWEED.shore * (1 - THREE.MathUtils.smoothstep(depths[i], 0.02, 0.15));
  }
  // The duckweed covers `share` of the open water: the points whose drift is
  // above that share's mark.
  const open = [...drift].filter((_, i) => depths[i] > 0).sort((a, b) => a - b);
  const mark = share <= 0 || open.length === 0 ? Infinity : share >= 1 ? -Infinity : open[Math.floor((1 - share) * (open.length - 1))];
  const colors = new Float32Array(count * 4);
  const scum = new Float32Array(count);
  const color = new THREE.Color();
  for (let i = 0; i < count; i++) {
    const weed = THREE.MathUtils.smoothstep(drift[i], mark - DUCKWEED.soft, mark + DUCKWEED.soft);
    const hides = THREE.MathUtils.lerp(CLEAR.shallow, CLEAR.deep, THREE.MathUtils.smoothstep(depths[i], 0, CLEAR.over));
    color.copy(murk).lerp(duckweed, weed);
    colors.set([color.r, color.g, color.b, THREE.MathUtils.lerp(hides, 1, weed)], i * 4);
    scum[i] = weed;
  }
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 4));
  geo.setAttribute('scum', new THREE.Float32BufferAttribute(scum, 1));
  return geo;
}

// What a reed clump is built from: points, their normals and colours, and
// the triangles between them.
interface Build {
  positions: number[];
  normals: number[];
  colors: number[];
  indices: number[];
}

interface ReedPalette {
  leaf: THREE.Color;
  straw: THREE.Color;
  stem: THREE.Color;
  head: THREE.Color;
  spike: THREE.Color;
}

// A clump of cattails, NOMINAL m tall: its leaves round its foot and, on
// most, one or two stems with their heads.
function reedGeometry(random: () => number, palette: ReedPalette): THREE.BufferGeometry {
  const build: Build = { positions: [], normals: [], colors: [], indices: [] };
  const leaves = Math.round(between(random, ...LEAF.count));
  for (let i = 0; i < leaves; i++) addLeaf(build, random, (2 * Math.PI * (i + between(random, -0.3, 0.3))) / leaves, palette);
  if (random() < STEM.chance) {
    const stems = random() < STEM.second ? 2 : 1;
    for (let i = 0; i < stems; i++) addCattail(build, random, palette);
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(build.positions, 3));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(build.normals, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(build.colors, 3));
  geo.setIndex(build.indices);
  return geo;
}

// A long, strap-like leaf rising from the foot, leaning out a little and
// arching over near its tip, as broad as it is all the way up until it
// narrows to a point in its last quarter. Both sides are front faces with
// normals pointing up, so it is lit like the sedges from either side, as
// the tufts' blades are (Lake/Sward.ts). Its foot is darker, and a dry leaf
// browns toward straw from its tip.
function addLeaf(build: Build, random: () => number, turn: number, palette: ReedPalette): void {
  const length = NOMINAL * between(random, ...LEAF.length);
  const width = LEAF.width * between(random, 0.85, 1.15);
  const arch = between(random, ...LEAF.arch);
  const dry = random() < LEAF.dry.share ? between(random, ...LEAF.dry.amount) : 0;
  const orient = new THREE.Matrix4().makeRotationY(-turn).setPosition(LEAF.foot * Math.cos(turn), 0, LEAF.foot * Math.sin(turn));
  const first = build.positions.length / 3;
  const p = new THREE.Vector3();
  const color = new THREE.Color();
  for (let i = 0; i <= LEAF.segments; i++) {
    const t = i / LEAF.segments;
    const half = (width / 2) * Math.min(1, (1 - t) / 0.25);
    const x = length * (LEAF.out * t + arch * t * t * t);
    const y = length * t * (1 - 0.4 * arch * t * t);
    color.copy(palette.leaf).lerp(palette.straw, dry * t).multiplyScalar(THREE.MathUtils.lerp(LEAF.shade, 1, t));
    for (const side of i < LEAF.segments ? [-half, half] : [0]) {
      p.set(x, y, side).applyMatrix4(orient);
      build.positions.push(p.x, p.y, p.z);
      build.normals.push(0, 1, 0);
      build.colors.push(color.r, color.g, color.b);
    }
  }
  const front: number[] = [];
  for (let i = 0; i < LEAF.segments - 1; i++) {
    const a = first + 2 * i;
    front.push(a, a + 1, a + 2, a + 1, a + 3, a + 2);
  }
  const last = first + 2 * (LEAF.segments - 1);
  front.push(last, last + 1, first + 2 * LEAF.segments);
  build.indices.push(...front);
  for (let i = 0; i < front.length; i += 3) build.indices.push(front[i], front[i + 2], front[i + 1]);
}

// A cattail: a stem rising from near the foot, leaning a little, with its
// brown head near the top and a spike of straw above it.
function addCattail(build: Build, random: () => number, palette: ReedPalette): void {
  const turn = 2 * Math.PI * random();
  const foot = new THREE.Vector3(STEM.foot * Math.cos(turn), 0, STEM.foot * Math.sin(turn));
  const height = NOMINAL * between(random, ...STEM.height);
  const tilt = STEM.tilt * random();
  const way = 2 * Math.PI * random();
  const axis = new THREE.Vector3(Math.sin(tilt) * Math.cos(way), Math.cos(tilt), Math.sin(tilt) * Math.sin(way));
  const head = height - HEAD.spike - HEAD.length; // m up the stem the head starts
  const L = HEAD.length;
  const r = HEAD.radius;
  addLathe(build, foot, axis, [[0, STEM.radius[0]], [head, STEM.radius[1]]], STEM.sides, palette.stem);
  addLathe(
    build,
    foot.clone().addScaledVector(axis, head),
    axis,
    [[0, 0], [0.01, 0.75 * r], [0.03, r], [L - 0.03, r], [L - 0.01, 0.75 * r], [L, 0]],
    HEAD.sides,
    palette.head,
  );
  addLathe(build, foot.clone().addScaledVector(axis, head + L), axis, [[0, HEAD.spikeRadius], [HEAD.spike, 0]], 4, palette.spike);
}

// A round body along `axis` from `base`: rings of `sides` points at the
// profile's distances along it, of its radii, facing out.
function addLathe(build: Build, base: THREE.Vector3, axis: THREE.Vector3, profile: [along: number, radius: number][], sides: number, color: THREE.Color): void {
  const across = new THREE.Vector3().crossVectors(axis, Math.abs(axis.y) < 0.9 ? UP : new THREE.Vector3(1, 0, 0)).normalize();
  const other = new THREE.Vector3().crossVectors(axis, across);
  const first = build.positions.length / 3;
  const out = new THREE.Vector3();
  const n = new THREE.Vector3();
  profile.forEach(([along, radius], i) => {
    // How fast the radius narrows along the axis here, which tips the normal.
    const [a0, r0] = profile[Math.max(0, i - 1)];
    const [a1, r1] = profile[Math.min(profile.length - 1, i + 1)];
    const narrowing = (r1 - r0) / Math.max(1e-6, a1 - a0);
    for (let j = 0; j < sides; j++) {
      const angle = (2 * Math.PI * j) / sides;
      out.copy(across).multiplyScalar(Math.cos(angle)).addScaledVector(other, Math.sin(angle));
      const p = base.clone().addScaledVector(axis, along).addScaledVector(out, radius);
      n.copy(out).addScaledVector(axis, -narrowing).normalize();
      build.positions.push(p.x, p.y, p.z);
      build.normals.push(n.x, n.y, n.z);
      build.colors.push(color.r, color.g, color.b);
    }
  });
  for (let i = 0; i < profile.length - 1; i++) {
    for (let j = 0; j < sides; j++) {
      const a = first + i * sides + j;
      const b = first + i * sides + ((j + 1) % sides);
      build.indices.push(a, b, a + sides, b, b + sides, a + sides);
    }
  }
}
