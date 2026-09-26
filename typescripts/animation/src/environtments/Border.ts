import * as THREE from 'three';
import { Cliff } from '../figures/objects/Cliff';
import { puffMaterial } from '../figures/objects/Fog';
import { between, seededRandom } from '../figures/Tree/parts';
import type { Theme } from '../theme';
import { hazeEdge } from './haze';
import { tuftGeometry, type TuftShape } from './Lake/Sward';
import { sway, type Wind } from './wind';

// The border of an environment's land, where it ends against the sky: high
// cliffs along parts of it, a bank of mist drifting round it, and tall grass
// growing thick up to it, so the line where the land stops never shows and
// the land seems to go on, as the user asked ("random grass and fog, so
// border map line not visible and more naturally", "add high clif too"). The
// lake and the grass environment both have one, round their land's edge.
//
// - Cliffs (the cliff object, figures/objects/Cliff.ts): 9-24 m high and
//   18-36 m wide, each facing the middle with its back just past the land's
//   end, with open stretches of 6-22 m between them. The first stands behind
//   the far side (-z), where the preview cameras look. Each is built 6 m
//   tall and scaled up, so a big cliff has the default cliff's faces, not
//   thousands more, and its rock is a little hazed toward the sky's colour,
//   as far things are.
// - Mist: a few hundred big, soft puffs (the fog object's, turned to face
//   the camera) lying low along the edge, most just past it and some further
//   in, in the sky's colour lightened a little. It drifts slowly round the
//   border, each puff circling, bobbing, swelling and thinning on its own,
//   and follows the ground's rises and hollows, fading out where it meets
//   the ground. A ring needs no ends, so it loops forever.
// - Tall grass: thousands of tufts 0.45-1.6 m tall (the sward's tuft shape,
//   with more and finer blades), thickest at the edge and thinning inward,
//   some of them dry, never inside a cliff. They sway in the wind, if the
//   environment has one.
//
// Given the sky's colour as it changes (`sky`: the lake's, which darkens at
// night), the cliffs' haze, the mist and the grass's fade follow it, and the
// mist is lightened less at night, so it doesn't glow against the dark sky.
//
// Placed by a seeded random generator, so it is the same on every load.
// Units are meters and the origin is the land's middle; `heightAt` gives the
// ground's height, so everything stands on uneven land.

export interface BorderOptions {
  theme: Theme;
  radius: number; // m from the middle to where the land ends
  heightAt?: (x: number, z: number) => number; // the ground's height; flat (0) when left out
  wind?: Wind; // sways the grass, if given
  // The sky's colour at the horizon now, if it changes (the lake at night);
  // the theme's background when left out.
  sky?: THREE.Color;
  seed?: number;
}

const CLIFF = {
  width: [18, 36] as const, // m along the border
  height: [9, 24] as const,
  depth: [9, 14] as const, // m from its face back
  gap: [6, 22] as const, // m of open border between two cliffs
  behind: 1, // m its back stands past the land's end
  sink: 0.3, // m below the lowest ground under it, so no face floats
  build: 6, // m tall it is built, then scaled up
  haze: 0.18, // how far its rock is lightened toward the sky's colour
  far: -Math.PI / 2, // the first stands here, round from +x toward +z: behind the far side
};

const MIST = {
  puffs: 560,
  out: 2, // m past the land's end the furthest puffs lie
  in: 14, // m in from there the nearest lie; most lie near the edge
  above: [0, 3.5] as const, // m a puff's middle floats above its lowest, most of them low
  radius: [1.6, 3.6] as const, // m, half a puff's height as seen
  stretch: [1.6, 2.8] as const, // a puff is this many times as wide as it is tall
  low: 0.3, // its middle is at least this share of its height above the ground
  opacity: [0.25, 0.5] as const, // of a puff at the ground
  thinTop: 0.5, // at the top of the mist a puff is this share as thick
  drift: 0.15, // m/s round the border
  swirl: { radius: [0.3, 1] as const, period: [20, 45] as const }, // m, s: each circles round its place
  bob: { height: [0.1, 0.4] as const, period: [9, 20] as const },
  grow: { amount: 0.15, period: [10, 24] as const },
  fade: { amount: 0.4, period: [7, 16] as const },
  sway: { angle: 0.08, period: [15, 30] as const },
  lighten: 0.35, // from the sky's colour toward the theme's light neutral
  nightLighten: 0.12, // the same at night
  shade: [0.96, 1.03] as const,
  groundFade: 0.4, // m above the ground over which it fades in
};

// Two shapes of tall grass: upright and fine, and looser and broader.
const TUFTS: TuftShape[] = [
  { blades: 11, width: 0.045, spread: 0.28, lean: 0.35 },
  { blades: 8, width: 0.06, spread: 0.45, lean: 0.5 },
];
const GRASS = {
  tufts: 4500,
  out: 0.3, // m past the land's end the furthest grow
  in: 10, // m in from there the nearest grow; most grow near the edge
  height: [0.45, 1.6] as const, // m
  tint: [0.7, 1.05] as const, // darkest and lightest, times the grass colour
  dry: { share: 0.25, amount: [0.2, 0.5] as const }, // tufts gone toward the wood colour (straw), and how far
  follow: 0.6, // how far a tuft leans with the ground's slope
  clear: 0.3, // m kept outside a cliff's footprint
  bend: { sway: 0.2, reach: 1.2 },
  haze: 0.6, // how far it fades into the sky at the land's end, where the ground fades all the way
};

const SEED = 17;
const UP = new THREE.Vector3(0, 1, 0);

// A cliff's footprint on the ground: its middle, the way its face looks (a
// unit vector toward the land's middle) and its half width and half depth.
interface Footprint {
  x: number;
  z: number;
  facing: THREE.Vector2;
  halfWidth: number;
  halfDepth: number;
}

// A cliff's rock material, its colour before it was hazed, and how far it is
// hazed toward the sky.
interface Rock {
  material: THREE.MeshStandardMaterial;
  color: THREE.Color;
  haze: number;
}

export class Border extends THREE.Group {
  readonly cliffs = new THREE.Group();
  readonly mist: Mist;
  readonly grass = new THREE.Group();
  private readonly sky: THREE.Color | null;
  private readonly light: THREE.Color;
  private readonly rocks: Rock[] = [];

  constructor(options: BorderOptions) {
    super();
    this.name = 'border';
    const { theme, radius } = options;
    const heightAt = options.heightAt ?? (() => 0);
    const random = seededRandom(options.seed ?? SEED);
    this.cliffs.name = 'cliffs';
    this.grass.name = 'tall grass';

    this.sky = options.sky ?? null;
    this.light = new THREE.Color(theme.colors.light);
    const sky = options.sky ?? new THREE.Color(theme.scene.background);
    const footprints = placeCliffs(this.cliffs, theme, radius, heightAt, random, sky, this.rocks);
    this.mist = new Mist(theme, radius, heightAt, random);
    growGrass(this.grass, theme, radius, heightAt, footprints, options.wind, random, sky);
    this.add(this.cliffs, this.mist, this.grass);
  }

  // Moves the mist on by delta seconds, and follows the sky's colour, `night`
  // being how far into the night it is (Lake/DayNight.ts); call once per
  // frame.
  update(delta: number, night = 0): void {
    this.mist.update(delta);
    if (!this.sky) return;
    for (const rock of this.rocks) tintRock(rock, this.sky);
    this.mist.tint(this.sky, this.light, THREE.MathUtils.lerp(MIST.lighten, MIST.nightLighten, night));
  }
}

// Cliffs round the border, one after another with open stretches between,
// starting with one centered behind the far side.
function placeCliffs(
  group: THREE.Group,
  theme: Theme,
  radius: number,
  heightAt: (x: number, z: number) => number,
  random: () => number,
  sky: THREE.Color,
  rocks: Rock[],
): Footprint[] {
  const footprints: Footprint[] = [];
  const width = () => between(random, ...CLIFF.width);
  let w = width();
  let angle = CLIFF.far; // of the cliff's middle
  let used = 0; // radians round the border, from the first cliff's middle
  let first = 0; // radians the first cliff takes on its near side
  for (;;) {
    const height = between(random, ...CLIFF.height);
    const depth = between(random, ...CLIFF.depth);
    const out = radius + CLIFF.behind - depth / 2; // of its footprint's middle
    const half = w / 2 / out;
    if (footprints.length === 0) first = half;
    const x = out * Math.cos(angle);
    const z = out * Math.sin(angle);
    const facing = new THREE.Vector2(-Math.cos(angle), -Math.sin(angle));
    const footprint = { x, z, facing, halfWidth: w / 2, halfDepth: depth / 2 };

    const scale = height / CLIFF.build;
    const cliff = new Cliff({ theme, seed: Math.floor(random() * 1e6), width: w / scale, height: CLIFF.build, depth: depth / scale });
    cliff.scale.setScalar(scale);
    cliff.rotation.y = Math.atan2(facing.x, facing.y); // its face (+z) toward the middle
    cliff.position.set(x, lowestUnder(footprint, heightAt) - CLIFF.sink, z);
    const meshes = new Map<THREE.MeshStandardMaterial, number>();
    cliff.traverse((object) => {
      if (!(object instanceof THREE.Mesh)) return;
      object.castShadow = false; // far outside the sun's shadow, which fits the figure
      object.receiveShadow = false;
      const material = object.material as THREE.MeshStandardMaterial;
      meshes.set(material, (meshes.get(material) ?? 0) + 1);
    });
    // Hazed by CLIFF.haze for each mesh, as it was when it was hazed mesh by
    // mesh: the rock and scree share a material, which goes twice as far.
    for (const [material, count] of meshes) {
      const rock = { material, color: material.color.clone(), haze: 1 - (1 - CLIFF.haze) ** count };
      tintRock(rock, sky);
      rocks.push(rock);
    }
    group.add(cliff);
    footprints.push(footprint);

    // The next, past an open stretch, if the border has room for it before
    // the first cliff comes round again.
    const gap = between(random, ...CLIFF.gap);
    const next = width();
    const step = (w / 2 + gap) / out + next / 2 / (radius + CLIFF.behind - CLIFF.depth[1] / 2);
    const minGap = CLIFF.gap[0] / out;
    if (used + step + next / 2 / out + minGap + first > 2 * Math.PI) break;
    used += step;
    angle += step;
    w = next;
  }
  return footprints;
}

// Hazes a cliff's rock toward the sky's colour as it is now.
function tintRock(rock: Rock, sky: THREE.Color): void {
  rock.material.color.copy(rock.color).lerp(sky, rock.haze);
}

// The lowest ground under a footprint, from a few points over it.
function lowestUnder(f: Footprint, heightAt: (x: number, z: number) => number): number {
  let lowest = Infinity;
  const side = new THREE.Vector2(-f.facing.y, f.facing.x);
  for (let i = -2; i <= 2; i++) {
    for (let j = -1; j <= 1; j++) {
      const along = (i / 2) * f.halfWidth;
      const across = j * f.halfDepth;
      lowest = Math.min(lowest, heightAt(f.x + side.x * along + f.facing.x * across, f.z + side.y * along + f.facing.y * across));
    }
  }
  return lowest;
}

// Whether a point is inside a footprint, grown by `margin`.
function inside(f: Footprint, x: number, z: number, margin: number): boolean {
  const dx = x - f.x;
  const dz = z - f.z;
  const across = dx * f.facing.x + dz * f.facing.y;
  const along = -dx * f.facing.y + dz * f.facing.x;
  return Math.abs(along) < f.halfWidth + margin && Math.abs(across) < f.halfDepth + margin;
}

// Tall grass along the border: thickest at the edge, thinning inward, never
// inside a cliff. Each shape is one instanced mesh.
function growGrass(
  group: THREE.Group,
  theme: Theme,
  radius: number,
  heightAt: (x: number, z: number) => number,
  footprints: Footprint[],
  wind: Wind | undefined,
  random: () => number,
  sky: THREE.Color,
): void {
  const green = new THREE.Color(theme.scene.grass);
  const straw = new THREE.Color(theme.colors.wood);
  const material = new THREE.MeshStandardMaterial({ roughness: 0.9, vertexColors: true });
  if (wind) sway(material, wind, GRASS.bend);
  hazeEdge(material, sky, radius, GRASS.haze);
  const placed: { matrix: THREE.Matrix4; color: THREE.Color }[][] = TUFTS.map(() => []);
  const lean = new THREE.Quaternion();
  const turn = new THREE.Quaternion();
  for (let k = 0; k < GRASS.tufts; k++) {
    const angle = 2 * Math.PI * random();
    const r = radius + GRASS.out - GRASS.in * random() ** 1.8;
    const x = r * Math.cos(angle);
    const z = r * Math.sin(angle);
    const shape = Math.floor(random() * TUFTS.length);
    const height = between(random, ...GRASS.height);
    const tint = between(random, ...GRASS.tint);
    const dry = random() < GRASS.dry.share ? between(random, ...GRASS.dry.amount) : 0;
    const spin = 2 * Math.PI * random();
    if (footprints.some((f) => inside(f, x, z, GRASS.clear))) continue;
    lean.setFromUnitVectors(UP, groundNormal(heightAt, x, z)).slerp(new THREE.Quaternion(), 1 - GRASS.follow);
    turn.setFromAxisAngle(UP, spin);
    placed[shape].push({
      matrix: new THREE.Matrix4().compose(new THREE.Vector3(x, heightAt(x, z), z), lean.clone().multiply(turn), new THREE.Vector3(height, height, height)),
      color: green.clone().lerp(straw, dry).multiplyScalar(tint),
    });
  }
  TUFTS.forEach((shape, s) => {
    const tufts = new THREE.InstancedMesh(tuftGeometry(shape, random), material, placed[s].length);
    placed[s].forEach(({ matrix, color }, i) => {
      tufts.setMatrixAt(i, matrix);
      tufts.setColorAt(i, color);
    });
    tufts.receiveShadow = true;
    group.add(tufts);
  });
}

// The way the ground faces at a point.
function groundNormal(heightAt: (x: number, z: number) => number, x: number, z: number): THREE.Vector3 {
  const e = 0.1;
  const dx = (heightAt(x + e, z) - heightAt(x - e, z)) / (2 * e);
  const dz = (heightAt(x, z + e) - heightAt(x, z - e)) / (2 * e);
  return new THREE.Vector3(-dx, 1, -dz).normalize();
}

interface Puff {
  angle: number; // round the middle, from +x toward +z, when the mist is shown
  r: number; // m from the middle
  above: number; // m its middle floats above the ground
  radius: number;
  stretch: number;
  opacity: number;
  swirl: { radius: number; rate: number; phase: number };
  bob: { height: number; rate: number; phase: number };
  grow: { rate: number; phase: number };
  fade: { rate: number; phase: number };
  sway: { rate: number; phase: number };
}

// Reused by Mist.update().
const matrix = new THREE.Matrix4();
const place = new THREE.Vector3();
const unturned = new THREE.Quaternion();
const size = new THREE.Vector3();

// The mist round the border, drifting slowly round it.
class Mist extends THREE.Group {
  private readonly puffs: THREE.InstancedMesh;
  private readonly plan: Puff[] = [];
  private readonly opacity: THREE.InstancedBufferAttribute;
  private readonly turn: THREE.InstancedBufferAttribute;
  private readonly ground: THREE.InstancedBufferAttribute;
  private time = 0;

  constructor(
    theme: Theme,
    radius: number,
    private readonly heightAt: (x: number, z: number) => number,
    random: () => number,
  ) {
    super();
    this.name = 'mist';
    const rate = (period: readonly [number, number]) => (2 * Math.PI) / between(random, ...period);
    const top = MIST.above[1];
    for (let i = 0; i < MIST.puffs; i++) {
      const puffRadius = between(random, ...MIST.radius);
      const above = MIST.low * puffRadius + between(random, ...MIST.above) * random();
      this.plan.push({
        angle: 2 * Math.PI * random(),
        r: radius + MIST.out - MIST.in * random() ** 2,
        above,
        radius: puffRadius,
        stretch: between(random, ...MIST.stretch),
        opacity: between(random, ...MIST.opacity) * THREE.MathUtils.lerp(1, MIST.thinTop, Math.min(1, above / top)),
        swirl: { radius: between(random, ...MIST.swirl.radius), rate: rate(MIST.swirl.period) * (random() < 0.5 ? -1 : 1), phase: 2 * Math.PI * random() },
        bob: { height: between(random, ...MIST.bob.height), rate: rate(MIST.bob.period), phase: 2 * Math.PI * random() },
        grow: { rate: rate(MIST.grow.period), phase: 2 * Math.PI * random() },
        fade: { rate: rate(MIST.fade.period), phase: 2 * Math.PI * random() },
        sway: { rate: rate(MIST.sway.period), phase: 2 * Math.PI * random() },
      });
    }

    const count = this.plan.length;
    const geo = new THREE.PlaneGeometry(2, 2); // corners at ±1: a puff's radius is its size
    const attribute = () => new THREE.InstancedBufferAttribute(new Float32Array(count), 1).setUsage(THREE.DynamicDrawUsage);
    this.opacity = attribute();
    this.turn = attribute();
    this.ground = attribute();
    geo.setAttribute('puffOpacity', this.opacity);
    geo.setAttribute('puffTurn', this.turn);
    geo.setAttribute('puffGround', this.ground);

    const color = new THREE.Color(theme.scene.background).lerp(new THREE.Color(theme.colors.light), MIST.lighten);
    this.puffs = new THREE.InstancedMesh(geo, puffMaterial(color, { ground: true, fade: MIST.groundFade }), count);
    this.puffs.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    const shade = new THREE.Color();
    for (let i = 0; i < count; i++) this.puffs.setColorAt(i, shade.setScalar(between(random, ...MIST.shade)));
    this.puffs.castShadow = false;
    this.puffs.receiveShadow = false;
    // Its bounds are the ring the puffs stay in, not their flat patches.
    const reach = radius + MIST.out + MIST.swirl.radius[1] + MIST.radius[1] * MIST.stretch[1];
    this.puffs.boundingSphere = new THREE.Sphere(new THREE.Vector3(), reach);
    this.puffs.boundingBox = new THREE.Box3(new THREE.Vector3(-reach, -reach, -reach), new THREE.Vector3(reach, reach, reach));
    this.add(this.puffs);
    this.update(0);
  }

  // Its colour: the sky's, lightened toward the theme's light neutral.
  tint(sky: THREE.Color, light: THREE.Color, lighten: number): void {
    (this.puffs.material as THREE.MeshBasicMaterial).color.copy(sky).lerp(light, lighten);
  }

  // Each puff drifts round the border, circles, rises and sinks, swells,
  // thins and tilts as it goes, over the ground's rises and hollows.
  update(delta: number): void {
    this.time += delta;
    const t = this.time;
    this.plan.forEach((p, i) => {
      const swirl = p.swirl.phase + p.swirl.rate * t;
      const angle = p.angle + (MIST.drift * t + p.swirl.radius * Math.cos(swirl)) / p.r;
      const r = p.r + p.swirl.radius * Math.sin(swirl);
      const x = r * Math.cos(angle);
      const z = r * Math.sin(angle);
      const ground = this.heightAt(x, z);
      const y = ground + p.above + p.bob.height * Math.sin(p.bob.phase + p.bob.rate * t);
      const radius = p.radius * (1 + MIST.grow.amount * Math.sin(p.grow.phase + p.grow.rate * t));
      const thin = 1 - MIST.fade.amount * (0.5 + 0.5 * Math.sin(p.fade.phase + p.fade.rate * t));
      this.opacity.setX(i, p.opacity * thin);
      this.turn.setX(i, MIST.sway.angle * Math.sin(p.sway.phase + p.sway.rate * t));
      this.ground.setX(i, ground);
      this.puffs.setMatrixAt(i, matrix.compose(place.set(x, y, z), unturned, size.set(p.stretch * radius, radius, radius)));
    });
    this.opacity.needsUpdate = true;
    this.turn.needsUpdate = true;
    this.ground.needsUpdate = true;
    this.puffs.instanceMatrix.needsUpdate = true;
  }
}
