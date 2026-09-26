import * as THREE from 'three';
import { SOIL_MEAN, soilTexture } from '../../textures/soil';
import { defaultTheme, type Theme } from '../../theme';
import { between, seededRandom, wobble, type Size } from './parts';

// A mud pit, as the user asked ("create mud pit, and add randomly to lake
// environtment"): a shallow hollow of wet mud, 1.5-4 m across, its outline
// uneven rather than round. In the middle the mud is dark and wet, smoother
// and with a sheen, with one or two shallow puddles of standing water; toward
// its edge it dries paler and rougher, the soil's grit and cracks showing
// (textures/soil.ts); and a low, trodden rim is pushed up round it. Nothing
// else: no footprints, bubbles or sticks.
//
// It never goes below the ground: it reads as a hollow by its raised rim,
// with its mud a little above the ground's level. A real hole would need the
// ground cut open, and at the lake the water is a sheet under the land that
// shows in any hollow dug below it. Given the ground's height (`heightAt`,
// in the pit's own coordinates), its rim and mud lie on uneven land; its
// outer edge tucks just under the ground, so there is no seam to see.
//
// It is in the lake's own style, not faceted: part of the ground, like the
// bank. The mud is the bare land's colour (scene.floor), darker toward the
// middle as it gets wetter, and the puddles the water's (scene.water), with
// a glint like the lake's.
//
// Units are meters, y is up, and the origin is on the ground at the middle of
// its footprint. It is built from a seed and sized to a width (x) and depth
// (z), the whole footprint to its outer edge, exactly; it has no height of
// its own beyond its rim. The spec, MudPit.md, is a draft: it names no
// behaviour, so it lies still.

export interface MudPitOptions {
  theme?: Theme;
  seed?: number; // another seed, another pit of the same kind
  width?: number; // m across its footprint, along x
  depth?: number; // m along z
  // The ground's height at a point, in the pit's own coordinates (from its
  // origin); flat when left out.
  heightAt?: (x: number, z: number) => number;
}

const SIZE: Pick<Size, 'width' | 'depth'> = { width: 3, depth: 2.2 };
const SEED = 1;

// Across the pit, shares of the way from its middle to its outline:
// the mud to WET, the rim from RIM_FROM up to its crest at 1, and down to
// the outer edge at FOOT, where it meets the ground.
const WET = 0.3; // the mud is wettest within this, drying out to DRY
const DRY = 0.8;
const RIM_FROM = 0.8;
const FOOT = 1.35;
// Rings closer together over the rim, where the surface bends; each pit is
// then 600 points and builds in about a millisecond.
const RINGS = [0, 0.12, 0.25, 0.38, 0.51, 0.63, 0.74, 0.82, 0.88, 0.94, 1, 1.07, 1.16, 1.25, FOOT];
const COLUMNS = 40; // round the outline
const OUTLINE = { lumps: 3, amount: 0.14, seed: 3 }; // how uneven the outline is: its radius swells and dips by up to `amount`

const LIFT = 0.012; // m the mud lies above the ground, so the ground never shows through it
const SINK = 0.015; // m its outer edge lies below the ground
const RIM = { height: 0.07, uneven: 0.4, seed: 5 }; // m the rim's crest stands above the ground, a 3 m pit's, trodden up and down by `uneven` of it
const LUMPS = { wet: 0.003, dry: 0.01, size: 0.35, seed: 7 }; // m the mud rises and dips, smooth in the wet middle; and m from one lump to the next

// The mud's colour, times the floor's: dark where wet, paler where dry, the
// rim a shade darker, trodden. Mottled by `mottle` either way.
const TONE = { wet: 0.42, dry: 0.95, rim: 0.8, mottle: 0.06, seed: 9 };
// How much of the soil texture shows (its grit, pebbles and cracks): little
// in the smooth wet mud, all of it where it has dried.
const SOIL = { wet: 0.15, dry: 1, rim: 0.85 };
const BUMP = 0.012; // m, the soil's bump height, as on the lake's ground
const ROUGHNESS = { dry: 0.92, wet: 0.4 }; // the wet mud has a sheen

// One or two puddles in the middle, each a shallow sheet of water over a
// dip in the mud.
const PUDDLES = {
  count: [1, 2.6] as const, // 1 or 2
  radius: [0.15, 0.3] as const, // shares of the pit's smaller half size
  within: 0.42, // share of the way out their middles lie
  above: 0.004, // m the water lies above the mud's level
  dip: 0.012, // m the mud dips under a puddle, so none pokes through
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

export class MudPit extends THREE.Group {
  static readonly SIZE = SIZE;

  readonly mud: THREE.Mesh;
  readonly puddles: THREE.Mesh[] = [];
  private readonly reach: number[] = []; // m from the pit's middle (shiftX, shiftZ) to its outer edge, at each column
  private readonly scaleX: number;
  private readonly scaleZ: number;
  private readonly shiftX: number;
  private readonly shiftZ: number;

  constructor(options: MudPitOptions = {}) {
    super();
    this.name = 'mud pit';
    const theme = options.theme ?? defaultTheme;
    const width = options.width ?? SIZE.width;
    const depth = options.depth ?? SIZE.depth;
    const heightAt = options.heightAt ?? (() => 0);
    const seed = options.seed ?? SEED;
    const random = seededRandom(seed);
    const phase = 100 * random();

    // The outline: an ellipse swelling and dipping round it. Stretched and
    // moved so the whole footprint, to the outer edge, is exactly width by
    // depth, centred on the origin, as the other objects are built to size.
    const around = (j: number) => (2 * Math.PI * j) / COLUMNS;
    const swell = (j: number) => 1 + OUTLINE.amount * wobble(phase + OUTLINE.lumps * Math.cos(around(j)), OUTLINE.lumps * Math.sin(around(j)), OUTLINE.seed + seed);
    const raw = Array.from({ length: COLUMNS }, (_, j) => ({ x: FOOT * swell(j) * Math.cos(around(j)), z: FOOT * swell(j) * Math.sin(around(j)) }));
    const xs = raw.map((p) => p.x);
    const zs = raw.map((p) => p.z);
    this.scaleX = width / (Math.max(...xs) - Math.min(...xs));
    this.scaleZ = depth / (Math.max(...zs) - Math.min(...zs));
    this.shiftX = -((Math.max(...xs) + Math.min(...xs)) / 2) * this.scaleX;
    this.shiftZ = -((Math.max(...zs) + Math.min(...zs)) / 2) * this.scaleZ;
    // A point `share` of the way out to the outline, round at column j.
    const point = (share: number, j: number) => {
      const s = (share / FOOT) * swell(j);
      return new THREE.Vector2(this.shiftX + FOOT * s * Math.cos(around(j)) * this.scaleX, this.shiftZ + FOOT * s * Math.sin(around(j)) * this.scaleZ);
    };
    const middle = new THREE.Vector2(this.shiftX, this.shiftZ);
    for (let j = 0; j < COLUMNS; j++) this.reach.push(point(FOOT, j).sub(middle).length());
    const half = Math.min(width, depth) / 2 / FOOT; // m from the middle to the rim's crest, at the narrow side
    const scale = Math.min(width, depth) / 2.6; // the pit's size, 1 for a middling one

    // The puddles, in the middle, apart.
    const puddles: Puddle[] = [];
    const count = Math.floor(between(random, ...PUDDLES.count));
    for (let attempt = 0; puddles.length < count && attempt < 20; attempt++) {
      const angle = 2 * Math.PI * random();
      const out = PUDDLES.within * Math.sqrt(random());
      const radius = half * between(random, ...PUDDLES.radius);
      const x = this.shiftX + out * Math.cos(angle) * (width / 2 / FOOT);
      const z = this.shiftZ + out * Math.sin(angle) * (depth / 2 / FOOT);
      if (puddles.some((p) => Math.hypot(p.x - x, p.z - z) < p.radius + radius + 0.1)) continue;
      puddles.push({ x, z, radius, seed: Math.floor(random() * 1e6) });
    }
    // How much a point is under a puddle: 1 inside, 0 a little way out.
    const underPuddle = (x: number, z: number) =>
      Math.max(0, ...puddles.map((p) => 1 - THREE.MathUtils.smoothstep(Math.hypot(x - p.x, z - p.z), p.radius * 1.05, p.radius * 1.35)));

    // The mud's surface: a ring of points for each share of the way out,
    // with the ground's height under each, the mud's lumps, the rim, and
    // the dips under the puddles.
    const floor = new THREE.Color(theme.scene.floor);
    const positions: number[] = [];
    const colors: number[] = [];
    const uvs: number[] = [];
    const soils: number[] = [];
    const color = new THREE.Color();
    const rimHeight = RIM.height * Math.sqrt(scale);
    for (const share of RINGS) {
      for (let j = 0; j < COLUMNS; j++) {
        const p = point(share, j);
        const wet = 1 - THREE.MathUtils.smoothstep(share, WET, DRY);
        const lump = THREE.MathUtils.lerp(LUMPS.dry, LUMPS.wet, wet) * wobble(p.x / LUMPS.size, p.y / LUMPS.size, LUMPS.seed + seed);
        const crest = rimHeight * (1 + RIM.uneven * wobble(phase + 2 * Math.cos(around(j)), 2 * Math.sin(around(j)), RIM.seed + seed));
        let y: number;
        if (share <= RIM_FROM) y = LIFT + lump;
        else if (share <= 1) y = THREE.MathUtils.lerp(LIFT + lump, crest, THREE.MathUtils.smoothstep(share, RIM_FROM, 1));
        else y = THREE.MathUtils.lerp(crest, -SINK, THREE.MathUtils.smoothstep(share, 1, FOOT));
        const under = underPuddle(p.x, p.y);
        y = y * (1 - under) + (LIFT - PUDDLES.dip) * under;
        positions.push(p.x, heightAt(p.x, p.y) + y, p.y);

        const rim = THREE.MathUtils.smoothstep(share, RIM_FROM, 1);
        const tone = THREE.MathUtils.lerp(THREE.MathUtils.lerp(TONE.dry, TONE.wet, wet), TONE.rim, rim) * (1 + TONE.mottle * wobble(p.x, p.y, TONE.seed + seed));
        color.copy(floor).multiplyScalar(tone);
        colors.push(color.r, color.g, color.b);
        uvs.push(p.x, p.y); // meters, as the soil texture wants
        soils.push(THREE.MathUtils.lerp(THREE.MathUtils.lerp(SOIL.dry, SOIL.wet, wet), SOIL.rim, rim));
      }
    }
    const indices: number[] = [];
    for (let i = 0; i < RINGS.length - 1; i++) {
      for (let j = 0; j < COLUMNS; j++) {
        const a = i * COLUMNS + j;
        const b = i * COLUMNS + ((j + 1) % COLUMNS);
        const c = a + COLUMNS;
        const d = b + COLUMNS;
        // Wound so the faces look up: (around) x (out) points up.
        if (i > 0) indices.push(a, b, c); // the middle ring is one point, many times over
        indices.push(b, d, c);
      }
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
    geo.setAttribute('soil', new THREE.Float32BufferAttribute(soils, 1));
    geo.setIndex(indices);
    geo.computeVertexNormals();

    const soil = soilTexture();
    const material = new THREE.MeshStandardMaterial({ vertexColors: true, roughness: ROUGHNESS.dry, map: soil, bumpMap: soil, bumpScale: BUMP });
    wetMud(material);
    this.mud = new THREE.Mesh(geo, material);
    this.mud.name = 'mud';
    this.mud.receiveShadow = true; // flat and low: it casts none worth drawing
    this.add(this.mud);

    // The puddles: thin sheets of water lying on the ground's height, a
    // little above the mud's level, so they are level to the eye.
    const water = new THREE.MeshStandardMaterial({ color: theme.scene.water, roughness: PUDDLES.roughness, transparent: true, opacity: PUDDLES.opacity });
    for (const p of puddles) {
      const sheet = new THREE.Mesh(puddleGeometry(p, heightAt), water);
      sheet.name = 'puddle';
      sheet.receiveShadow = true;
      this.puddles.push(sheet);
      this.add(sheet);
    }
  }

  // Whether a point (in the pit's own coordinates) is within `gap` m of the
  // pit's outer edge (inside it for a gap under 0), so that grass keeps off.
  near(x: number, z: number, gap: number): boolean {
    // The outer edge's distance from the pit's middle the way the point lies,
    // between two columns. The outline is stretched along x and z, so the
    // way is found where it was drawn round, before the stretch.
    const angle = THREE.MathUtils.euclideanModulo(Math.atan2((z - this.shiftZ) / this.scaleZ, (x - this.shiftX) / this.scaleX), 2 * Math.PI);
    const j = (angle / (2 * Math.PI)) * COLUMNS;
    const k = Math.floor(j) % COLUMNS;
    const reach = THREE.MathUtils.lerp(this.reach[k], this.reach[(k + 1) % COLUMNS], j - Math.floor(j));
    return Math.hypot(x - this.shiftX, z - this.shiftZ) < reach + gap;
  }
}

// A puddle's sheet: rings out to its edge, which swells and dips a little.
function puddleGeometry(p: Puddle, heightAt: (x: number, z: number) => number): THREE.BufferGeometry {
  const positions: number[] = [];
  for (let i = 0; i <= PUDDLE_RINGS; i++) {
    for (let j = 0; j < PUDDLE_COLUMNS; j++) {
      const angle = (2 * Math.PI * j) / PUDDLE_COLUMNS;
      const edge = 1 + 0.18 * wobble(2 * Math.cos(angle), 2 * Math.sin(angle), p.seed);
      const r = (p.radius * edge * i) / PUDDLE_RINGS;
      const x = p.x + r * Math.cos(angle);
      const z = p.z + r * Math.sin(angle);
      positions.push(x, heightAt(x, z) + LIFT + PUDDLES.above, z);
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

// The soil texture showing as much as each vertex's `soil` says, as on the
// lake's ground (Ground.ts showSoil): its colour mixed from its plain
// average toward the texture and divided by that average, so the mud keeps
// its colours on average, and its bumps shrinking with it. The wet mud,
// where little soil shows, is smoother to the light too: its roughness
// falls toward ROUGHNESS.wet, which gives it a sheen.
function wetMud(material: THREE.MeshStandardMaterial): void {
  const wet = (ROUGHNESS.wet / ROUGHNESS.dry).toFixed(4);
  material.onBeforeCompile = (shader) => {
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', '#include <common>\nattribute float soil;\nvarying float vSoil;')
      .replace('#include <begin_vertex>', '#include <begin_vertex>\nvSoil = soil;');
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', `#include <common>\nvarying float vSoil;\n#define SOIL_MEAN ${SOIL_MEAN.toFixed(4)}`)
      .replace('#include <map_fragment>', 'diffuseColor.rgb *= mix(vec3(SOIL_MEAN), texture2D(map, vMapUv).rgb, vSoil) / SOIL_MEAN;')
      .replace('#include <bumpmap_pars_fragment>', THREE.ShaderChunk.bumpmap_pars_fragment.replaceAll('bumpScale *', 'bumpScale * vSoil *'))
      .replace('#include <roughnessmap_fragment>', `#include <roughnessmap_fragment>\nroughnessFactor *= mix(${wet}, 1.0, vSoil);`);
  };
  material.customProgramCacheKey = () => 'mud';
}
