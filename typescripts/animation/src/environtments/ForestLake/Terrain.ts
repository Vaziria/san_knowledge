import * as THREE from 'three';
import { fbm2, noise2, palette, type Season } from '../../figures/ForestLake/parts';
import { finish, type Stepwise } from '../../stepwise';
import { SUN } from '../../sun';
import { CASCADE, CAVE_SCALE, CLEARING, FURROW, INLAND, INLAND_MOST, ISLAND, ISLAND_TOP, LAKE, LANDING, Line, MAIN_LIP, MAIN_RIVER, Outline, PATHS, RIM, SMALL_LIP, SMALL_RIVER, SPOTS, STREAM, WRECK_SCAR, WRECK_SITE, wreckFrame } from './layout';

// Rows of the grid sampled, coloured or lit between the stops of a build a
// piece at a time (ForestLake.build).
const ROWS = 8;

export function smoothstep(a: number, b: number, x: number): number {
  const t = Math.max(0, Math.min(1, (x - a) / (b - a)));
  return t * t * (3 - 2 * t);
}

// Something standing on the land that shades it: a ball (a crown of
// needles or blossom, a rock, a building roughly) whose shadow falls on the
// ground away from the sun, and whose foot darkens the ground round it.
export interface Shade {
  x: number;
  y: number;
  z: number;
  radius: number;
  foot?: number; // radius of the darkening round its foot on the ground, if it stands there
}

const SUN_DIRECTION = SUN.clone().normalize();
// The light baked into the ground (tasks/lighting.md: "shadows are too
// weak", "add stronger contact shadows"): what is left of it in a shadow
// (0.6 before), and how much darker the ground is right at the foot of a
// trunk, a rock or a bush, fading out by its `foot` (0.28 before, and
// fading faster).
const SHADED = 0.48;
const CONTACT = 0.48;

// The ground's colours in each season, as the reference paints it: fresh
// green grass in spring, deeper green in summer, gold and orange in autumn
// (with fallen leaves drawn over it, `leaves`), snow in winter, the paths
// trodden snow and the rock showing only where it is steep. Sand and the
// bed are the lake's floor, seen through the water; under winter's ice
// they are hidden.
interface GroundColors {
  grass: number;
  grassLight: number;
  grassDark: number;
  pathDark: number;
  rock: number;
  steep: number; // how much of steep land is bare rock, 0..1
  sand: number;
  bed: number;
  shoreline: number;
  pebbles: number;
  camp: number;
  leaves: number; // fallen leaves drawn over it, 0..1
  snow: number; // snow's glitter, 0..1
}
const GROUND: Record<Season, GroundColors> = {
  spring: { grass: 0x70ad3c, grassLight: 0x92c64c, grassDark: 0x4e8f2f, pathDark: 0xbd8446, rock: 0xa89fae, steep: 1, sand: 0xd9c690, bed: 0x2f7f96, shoreline: 0x8e8a55, pebbles: 0x9a98a0, camp: 0xc99a5e, leaves: 0, snow: 0 },
  summer: { grass: 0x5b9e36, grassLight: 0x7bb946, grassDark: 0x3d832d, pathDark: 0xb87c40, rock: 0xa39aa8, steep: 1, sand: 0xd4c28b, bed: 0x2a7494, shoreline: 0x7a7d48, pebbles: 0x97959d, camp: 0xc4935a, leaves: 0, snow: 0 },
  autumn: { grass: 0xc9913f, grassLight: 0xdcae55, grassDark: 0x9a6630, pathDark: 0xb07a45, rock: 0xa59ba6, steep: 1, sand: 0xd6c08a, bed: 0x2b6f8c, shoreline: 0x8a6f45, pebbles: 0x9c948f, camp: 0xc28e55, leaves: 1, snow: 0 },
  winter: { grass: 0xeaf1f7, grassLight: 0xfbfdff, grassDark: 0xcfdcea, pathDark: 0xc4b6a6, rock: 0x9f9aab, steep: 0.55, sand: 0xc9d9e3, bed: 0x6f97b3, shoreline: 0xdde8f1, pebbles: 0xb7bcc6, camp: 0xd6ccbf, leaves: 0, snow: 1 },
};

// A river over the heights: its line, from its source down to its lip.
export interface River {
  line: Line;
  lip: { x: number; z: number; level: number; width: number; rise: number };
}

// The forest lake's land (ForestLake.md): its height everywhere, from the
// map's shapes (layout.ts), sampled into a grid 0.6 m apart round the lake
// and further apart outward, out to 340 m; the water over it (the lake's,
// the rivers', the stream's); where the paths run; and the ground mesh,
// coloured grass, dirt, sand and rock, with the sun's shadows and the
// darkness in its hollows baked into its colours.
export class Terrain {
  readonly lake = new Outline(LAKE, 14, 0.8);
  readonly island = new Outline(ISLAND, 6, 0.6);
  readonly rim = new Line(RIM, 80, 1.5);
  readonly rivers: River[] = [
    { line: new Line(MAIN_RIVER, 8, 1), lip: MAIN_LIP },
    { line: new Line(SMALL_RIVER, 8, 1), lip: SMALL_LIP },
  ];
  readonly stream = new Line(STREAM, 9, 1);
  readonly paths = PATHS.map(({ points, width }) => ({ line: new Line(points, 4, 0.8), width }));
  // Level places: the landing's clearing, the camp, the tower's and the
  // cave's feet, each held at its middle's height.
  private readonly pads: { x: number; z: number; radius: number; height: number }[];
  readonly xs: Float64Array;
  readonly zs: Float64Array;
  readonly heights: Float32Array;
  // The cave's floor: its pad's height.
  readonly caveFloor: number;
  // The spaceship wreck's site: the height it is levelled to.
  readonly wreckLevel: number;

  // Its heights sampled at once; or, `later`, only once sample() has run to
  // its end (the forest lake built a piece at a time, ForestLake.build).
  constructor(
    readonly season: Season = 'spring',
    later = false,
  ) {
    this.pads = [
      { x: LANDING[0], z: LANDING[1], radius: CLEARING },
      { x: SPOTS.camp.x, z: SPOTS.camp.z, radius: SPOTS.camp.radius },
      { x: SPOTS.tower.x, z: SPOTS.tower.z, radius: 3.4 },
      { x: SPOTS.cave.x - 2.2, z: SPOTS.cave.z, radius: 3.2 },
    ].map((pad) => ({ ...pad, height: this.shape(pad.x, pad.z, false) }));
    this.caveFloor = this.pads[3].height;
    this.wreckLevel = this.shape(SPOTS.wreck.x, SPOTS.wreck.z, false);
    this.xs = axis(-68, 68, 0.6, 340); // as wide as the wider lake and what stands round it (task 21)
    this.zs = axis(-82, 66, 0.6, 340);
    this.heights = new Float32Array(this.xs.length * this.zs.length);
    if (!later) finish(this.sample());
  }

  // The land's height at each grid point, ROWS rows of the grid at a time.
  *sample(): Stepwise<void> {
    for (let j = 0; j < this.zs.length; j++) {
      for (let i = 0; i < this.xs.length; i++) this.heights[j * this.xs.length + i] = this.shape(this.xs[i], this.zs[j], true);
      if (j % ROWS === ROWS - 1) yield;
    }
  }

  // A river's water level `along` m from its source.
  riverLevel(river: River, along: number): number {
    return river.lip.level + (river.line.length - along) * river.lip.rise;
  }

  // The stream's water level `along` m from the lake.
  streamLevel(along: number): number {
    if (along <= CASCADE.from) return 0;
    if (along <= CASCADE.to) return -CASCADE.drop * smoothstep(CASCADE.from, CASCADE.to, along);
    return -CASCADE.drop - (along - CASCADE.to) * CASCADE.fall;
  }

  // The ground's height, worked out from the shapes: slow, for building.
  shape(x: number, z: number, pads: boolean): number {
    // The lowland: gentle rolls a little over the water, rising into hills
    // away from the lake.
    const r = Math.hypot((x - 1) * 0.85, z + 6);
    let h = 0.55 + (fbm2(x * 0.045, z * 0.045, 7) - 0.5) * 0.7;
    h += smoothstep(42, 125, r) * 17 * (0.65 + 0.7 * fbm2(x * 0.018, z * 0.018, 3));
    // The ruins' terrace, in the west.
    const ruins = Math.hypot(x - SPOTS.ruins.x, z - SPOTS.ruins.z);
    h += (1.35 - h) * smoothstep(10.5, 6.5, ruins);

    // The lake: its bed deepening away from the shore, the land sloping
    // down into it.
    const shore = this.lake.distance(x, z);
    if (shore < 3) {
      const bed = -Math.min(2.9, 0.3 + 0.42 * Math.max(0, -shore)) + (noise2(x * 0.3, z * 0.3, 9) - 0.5) * 0.25;
      h = bed + (h - bed) * smoothstep(-0.9, 1.2, shore);
    }
    // The island, rising steeply out of it: a plinth of rock (its ring of
    // boulders, Landmarks) with a level grassy top.
    const island = this.island.distance(x, z);
    if (island < 3) {
      const top = ISLAND_TOP + (fbm2(x * 0.4, z * 0.4, 13) - 0.5) * 0.18;
      // Straight down into deep water, no shelf round it.
      h = Math.max(h, top - smoothstep(-0.9, 0.9, island) * (top + 1.3) - smoothstep(0.9, 3, island) * 1.6);
    }

    // The heights: the rim's level on its left, a cliff at its edge, rising
    // a little further in, and fading back into the hills far in.
    const edge = this.rim.nearest(x, z);
    if (edge.segment >= 0) {
      const inward = edge.distance * edge.side;
      const level = this.rim.value(edge) + Math.min(INLAND_MOST, Math.max(0, inward) * INLAND) + (fbm2(x * 0.08, z * 0.08, 11) - 0.5) * 0.5;
      const t = smoothstep(-1.3, 0.5, inward) * (1 - smoothstep(50, 78, inward));
      h = Math.max(h, h + (level - h) * t);
    }

    // The rivers, cut into the heights, and the stream, into the land.
    for (const river of this.rivers) {
      const near = river.line.nearest(x, z);
      if (near.segment < 0) continue;
      const bed = this.riverLevel(river, near.along) - 0.65;
      h = Math.min(h, bed + Math.max(0, near.distance - river.lip.width * 0.36) * 1.1);
    }
    const stream = this.stream.nearest(x, z);
    if (stream.segment >= 0) {
      const bed = this.streamLevel(stream.along) - 0.5;
      h = Math.min(h, bed + Math.max(0, stream.distance - 1.25) * 1.15);
    }

    // Level places, and a trench cut into the cliff for the cave's hollow,
    // below its floor: land can't arch over a hollow, so under the cave's
    // hill of rock (which covers the trench, with the boulders round its
    // mouth and the slabs roofing the heights over it) there is none. The
    // cave faces west: across it is z, into it is +x.
    if (pads) {
      for (const pad of this.pads) {
        const d = Math.hypot(x - pad.x, z - pad.z);
        if (d < pad.radius + 2.5) h += (pad.height - h) * smoothstep(pad.radius + 2.5, pad.radius, d);
      }
      const across = z - SPOTS.cave.z;
      const inward = x - SPOTS.cave.x;
      if (Math.abs(across) < 2.2 * CAVE_SCALE && inward > 0.15 - 0.7 * (CAVE_SCALE - 1) && inward < 0.7 + 4.2 * CAVE_SCALE) h = Math.min(h, this.caveFloor - 0.3);
      // The wreck's site, levelled round it, and the furrow it ploughed
      // behind its tail, a low ridge of earth thrown up along each side.
      const [u, v] = wreckFrame(x, z);
      const e = Math.hypot(u / WRECK_SITE.along, v / WRECK_SITE.across);
      if (e < 1.35) h += (this.wreckLevel - h) * smoothstep(1.35, 1, e);
      h += this.furrowAt(u, v);
    }
    return h;
  }

  // How much the wreck's furrow lowers (or its ridges raise) the ground at a
  // point of the wreck's frame.
  private furrowAt(u: number, v: number): number {
    if (u < FURROW.from - 3 || u > FURROW.to || Math.abs(v) > FURROW.width + 1.4) return 0;
    const along = smoothstep(FURROW.from - 3, FURROW.from + 1, u) * (1 - smoothstep(FURROW.to - 7, FURROW.to, u));
    const w = FURROW.width * (1 - 0.35 * Math.max(0, (u - FURROW.from) / (FURROW.to - FURROW.from)));
    const a = Math.abs(v);
    const dig = FURROW.depth * (1 - smoothstep(w * 0.35, w, a));
    const ridge = 0.16 * smoothstep(w * 0.75, w, a) * (1 - smoothstep(w + 0.2, w + 1.2, a));
    return along * (ridge - dig);
  }

  // How much of a point is the earth the wreck churned up: under and round
  // it, and down its furrow; 0 off it, soft at the edges.
  scarAt(x: number, z: number): number {
    const [u, v] = wreckFrame(x, z);
    const ragged = (noise2(x * 0.45, z * 0.45, 41) - 0.5) * 0.3;
    const site = smoothstep(1.05, 0.8, Math.hypot(u / WRECK_SCAR.along, v / WRECK_SCAR.across) + ragged);
    const w = FURROW.width * (1 - 0.35 * Math.max(0, (u - FURROW.from) / (FURROW.to - FURROW.from)));
    const furrow = u > FURROW.from - 2 && u < FURROW.to ? smoothstep(w + 0.9, w * 0.6, Math.abs(v) + ragged) * (1 - smoothstep(FURROW.to - 5, FURROW.to, u)) : 0;
    return Math.max(site, furrow);
  }

  // The ground's height from the grid, between its points: fast.
  heightAt(x: number, z: number): number {
    const [i, u] = locate(this.xs, x);
    const [j, v] = locate(this.zs, z);
    const n = this.xs.length;
    const h = this.heights;
    const a = h[j * n + i];
    const b = h[j * n + i + 1];
    const c = h[(j + 1) * n + i];
    const d = h[(j + 1) * n + i + 1];
    return a + (b - a) * u + (c - a) * v + (a - b - c + d) * u * v;
  }

  // The ground's colour at a point as meshed (its paint and the light baked
  // into it), between its grid points; the grass's before it is meshed.
  colorAt(x: number, z: number, target: THREE.Color): THREE.Color {
    const colors = this.colors;
    if (!colors) return target.set(GROUND[this.season].grass);
    const [i, u] = locate(this.xs, x);
    const [j, v] = locate(this.zs, z);
    const n = this.xs.length;
    const corners = [j * n + i, j * n + i + 1, (j + 1) * n + i, (j + 1) * n + i + 1];
    const weights = [(1 - u) * (1 - v), u * (1 - v), (1 - u) * v, u * v];
    let r = 0;
    let g = 0;
    let b = 0;
    corners.forEach((k, q) => {
      r += colors[k * 3] * weights[q];
      g += colors[k * 3 + 1] * weights[q];
      b += colors[k * 3 + 2] * weights[q];
    });
    return target.setRGB(r, g, b);
  }

  // The water's surface over a point, or NaN where there is none: the
  // lake's (y 0), a river's or the stream's. Over the island's ground, or
  // any ground higher than the water, the caller finds the ground higher.
  waterAt(x: number, z: number): number {
    if (this.lake.inside(x, z)) return 0;
    const stream = this.stream.nearest(x, z);
    if (stream.distance < 1.7) return this.streamLevel(stream.along);
    for (const river of this.rivers) {
      const near = river.line.nearest(x, z);
      // Not past its ends: past its lip is the waterfall's.
      const end = near.segment === river.line.points.length - 2 && near.t >= 1;
      if (near.distance < river.lip.width * 0.52 && !end && !(near.segment === 0 && near.t <= 0)) return this.riverLevel(river, near.along);
    }
    return NaN;
  }

  // How much of a point is path: 1 on one, 0 off them all, soft at the
  // edges.
  pathAt(x: number, z: number): number {
    let most = 0;
    for (const { line, width } of this.paths) {
      const near = line.nearest(x, z);
      if (near.segment < 0) continue;
      const edge = width / 2 + (noise2(x * 0.9, z * 0.9, 17) - 0.5) * 0.35;
      most = Math.max(most, smoothstep(edge + 0.3, edge - 0.25, near.distance));
    }
    return most;
  }

  // The ground mesh: coloured, with the shadows of `shades` and of the
  // land itself baked in.
  mesh(shades: readonly Shade[], paths?: (x: number, z: number) => number): THREE.Mesh {
    return finish(this.meshing(shades, paths));
  }

  // The same, ROWS rows of the grid at a time.
  // `paths`: how much of the paths' sandy dirt shows at a point, in place
  // of pathAt's even band (the paths as they change along their way,
  // Paths.ts).
  *meshing(shades: readonly Shade[], paths?: (x: number, z: number) => number): Stepwise<THREE.Mesh> {
    const { xs, zs, heights } = this;
    const n = xs.length;
    const m = zs.length;
    const light = (this.baked = yield* this.bakeLight(shades));
    const positions = new Float32Array(n * m * 3);
    const colors = (this.colors = new Float32Array(n * m * 3));
    const c = new THREE.Color();
    const G = GROUND[this.season];
    const P = palette(this.season);
    const grass = new THREE.Color(G.grass);
    const grassLight = new THREE.Color(G.grassLight);
    const grassDark = new THREE.Color(G.grassDark);
    const path = new THREE.Color(P.path);
    const pathLight = new THREE.Color(P.pathLight);
    const pathDark = new THREE.Color(G.pathDark);
    const rock = new THREE.Color(G.rock);
    const sand = new THREE.Color(G.sand);
    const bed = new THREE.Color(G.bed); // deep down, blue like the water over it
    const shoreline = new THREE.Color(G.shoreline);
    const pebbles = new THREE.Color(G.pebbles);
    const camp = new THREE.Color(G.camp);
    // Where fallen leaves or snow's glitter are drawn: the open ground.
    const cover = new Float32Array(n * m);
    const tmp = new THREE.Color();
    for (let j = 0; j < m; j++) {
      for (let i = 0; i < n; i++) {
        const k = j * n + i;
        const x = xs[i];
        const z = zs[j];
        const h = heights[k];
        positions.set([x, h, z], k * 3);
        // Grass in patches of lighter and darker green, broad and smaller
        // ones over each other, finely mottled: deeper darks and a second
        // scale since tasks/lighting.md ("ground is very uniformly bright").
        const patch = fbm2(x * 0.05, z * 0.05, 21);
        c.copy(grass).lerp(grassLight, smoothstep(0.5, 0.74, patch)).lerp(grassDark, smoothstep(0.5, 0.26, patch) * 0.95);
        c.multiplyScalar(0.86 + 0.2 * fbm2(x * 0.16, z * 0.16, 27));
        c.multiplyScalar(0.93 + 0.14 * noise2(x * 0.4, z * 0.4, 5));
        // Rock where the land is steep.
        const steep = this.steepness(i, j);
        const bare = smoothstep(0.55, 1.1, steep) * G.steep;
        c.lerp(tmp.copy(rock).multiplyScalar(0.85 + 0.25 * noise2(x * 0.5, z * 0.5, 3)), bare);
        let open = 1 - bare;
        // Paths, and the camp's trodden ground.
        const onPath = paths ? paths(x, z) : this.pathAt(x, z);
        if (onPath > 0) {
          const grit = noise2(x * 0.7, z * 0.7, 29);
          tmp.copy(path).lerp(pathLight, smoothstep(0.55, 0.8, grit)).lerp(pathDark, smoothstep(0.35, 0.1, grit) * 0.7);
          c.lerp(tmp, onPath);
          open *= 1 - onPath * 0.7;
        }
        const campAt = Math.hypot(x - SPOTS.camp.x, z - SPOTS.camp.z);
        if (campAt < SPOTS.camp.radius + 1) c.lerp(camp, smoothstep(SPOTS.camp.radius + 1, SPOTS.camp.radius - 1.5, campAt) * (0.55 + 0.35 * noise2(x * 0.6, z * 0.6, 31)));
        // The earth the wreck churned up, darker in its furrow.
        const scar = Math.abs(x - SPOTS.wreck.x) < 45 && Math.abs(z - SPOTS.wreck.z) < 20 ? this.scarAt(x, z) : 0;
        if (scar > 0) {
          const grit = noise2(x * 0.8, z * 0.8, 43);
          tmp.copy(pathDark).lerp(path, smoothstep(0.45, 0.8, grit) * 0.6).multiplyScalar(0.86 + 0.12 * noise2(x * 2.1, z * 2.1, 47));
          c.lerp(tmp, scar * 0.9);
          open *= 1 - scar * 0.8;
        }
        // Where there is water over it: sand in the shallows turning green
        // with depth; a darker, muddy band at the waterline; stones in the
        // rivers' and the stream's beds.
        const water = this.waterAt(x, z);
        if (!Number.isNaN(water)) {
          const depth = water - h;
          const channel = !this.lake.inside(x, z);
          const under = channel ? tmp.copy(pebbles).multiplyScalar(0.85 + 0.3 * noise2(x * 1.3, z * 1.3, 2)) : tmp.copy(sand).lerp(bed, smoothstep(0.3, 2.4, depth));
          c.lerp(under, smoothstep(-0.25, 0.05, depth));
          open *= 1 - smoothstep(-0.25, 0.05, depth);
        }
        if (h > -0.35 && h < 0.35 && this.lake.distance(x, z) < 1.2) c.lerp(shoreline, (1 - Math.abs(h) / 0.35) * 0.35);
        c.multiplyScalar(light[k]);
        colors.set([c.r, c.g, c.b], k * 3);
        cover[k] = open;
      }
      if (j % ROWS === ROWS - 1) yield;
    }
    // Two triangles to each cell, split one way or the other at random, so
    // no grain of diagonals runs across the land.
    const index: number[] = [];
    for (let j = 0; j + 1 < m; j++) {
      for (let i = 0; i + 1 < n; i++) {
        const a = j * n + i;
        const b = a + 1;
        const d = a + n;
        const e = d + 1;
        if (((i * 7919) ^ (j * 104729)) & 1) index.push(a, d, b, b, d, e);
        else index.push(a, d, e, a, e, b);
      }
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));
    geometry.setAttribute('cover', new THREE.BufferAttribute(cover, 1));
    geometry.setIndex(index);
    geometry.computeVertexNormals();
    geometry.computeBoundingSphere();
    const material = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.95, metalness: 0 });
    material.name = 'ground';
    // The grain of grass and dirt, painted per pixel over the vertex
    // colours: fine streaks and specks, lighter and darker, and a coarser
    // mottling, fixed to the land; softer on snow. In autumn fallen leaves,
    // orange, red and gold, lie thick on the open ground; in winter the snow
    // glitters here and there.
    const leaves = G.leaves.toFixed(2);
    const glitter = G.snow.toFixed(2);
    const grain = (G.snow > 0 ? 0.35 : 1).toFixed(2);
    material.onBeforeCompile = (shader) => {
      shader.vertexShader = shader.vertexShader
        .replace('#include <common>', '#include <common>\nvarying vec2 vGround;\nattribute float cover;\nvarying float vCover;')
        .replace('#include <begin_vertex>', '#include <begin_vertex>\nvGround = position.xz;\nvCover = cover;');
      shader.fragmentShader = shader.fragmentShader
        .replace(
          '#include <common>',
          `#include <common>
          varying vec2 vGround;
          varying float vCover;
          float groundHash(vec2 p) { return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453); }
          float groundNoise(vec2 p) {
            vec2 i = floor(p);
            vec2 f = fract(p);
            vec2 u = f * f * (3.0 - 2.0 * f);
            return mix(mix(groundHash(i), groundHash(i + vec2(1.0, 0.0)), u.x), mix(groundHash(i + vec2(0.0, 1.0)), groundHash(i + vec2(1.0, 1.0)), u.x), u.y);
          }`,
        )
        .replace(
          '#include <color_fragment>',
          `#include <color_fragment>
          float streaks = groundNoise(vGround * vec2(10.0, 3.2)) * 0.5 + groundNoise(vGround * vec2(3.2, 10.0)) * 0.5;
          float specks = groundNoise(vGround * 24.0);
          float mottle = groundNoise(vGround * 0.8) + groundNoise(vGround * 0.23) - 1.0;
          diffuseColor.rgb *= 1.0 + ${grain} * (-0.12 + 0.2 * streaks + 0.1 * (specks - 0.5) + 0.14 * mottle);
          // Fallen leaves: in cells about 18 cm across, some holding a
          // leaf, lit as the ground under it is.
          if (${leaves} > 0.0) {
            vec2 q = vGround * 5.5;
            vec2 cell = floor(q);
            float pick = groundHash(cell);
            vec2 off = vec2(groundHash(cell + 3.7), groundHash(cell + 9.1)) - 0.5;
            vec2 d = fract(q) - 0.5 - off * 0.45;
            float r = length(d * vec2(1.0, 1.35));
            float size = 0.62 + 0.5 * groundHash(cell + 5.3);
            float lobes = (0.3 + 0.06 * cos(atan(d.y, d.x) * 5.0 + pick * 20.0)) * size;
            float leaf = step(r, lobes) * step(0.5, pick) * smoothstep(0.35, 0.8, vCover) * ${leaves};
            vec3 tone = pick > 0.86 ? vec3(0.62, 0.07, 0.03) : pick > 0.72 ? vec3(0.88, 0.25, 0.03) : pick > 0.58 ? vec3(0.93, 0.55, 0.07) : vec3(0.95, 0.72, 0.12);
            float lit = clamp(max(vColor.r, max(vColor.g, vColor.b)) * 1.4, 0.5, 1.0);
            diffuseColor.rgb = mix(diffuseColor.rgb, tone * lit, leaf);
          }
          // Snow's glitter.
          if (${glitter} > 0.0) {
            float sparkle = step(0.985, groundHash(floor(vGround * 30.0))) * vCover * ${glitter};
            diffuseColor.rgb += vec3(0.35, 0.38, 0.42) * sparkle;
          }`,
        );
    };
    material.customProgramCacheKey = () => `forest-lake-ground-${this.season}`;
    const ground = new THREE.Mesh(geometry, material);
    ground.receiveShadow = true;
    return ground;
  }

  // How steep the land is at a grid point: the rise over the run to its
  // neighbours, as a slope (1 is 45°).
  private steepness(i: number, j: number): number {
    const { xs, zs, heights } = this;
    const n = xs.length;
    const i0 = Math.max(0, i - 1);
    const i1 = Math.min(n - 1, i + 1);
    const j0 = Math.max(0, j - 1);
    const j1 = Math.min(zs.length - 1, j + 1);
    const dx = (heights[j * n + i1] - heights[j * n + i0]) / (xs[i1] - xs[i0]);
    const dz = (heights[j1 * n + i] - heights[j0 * n + i]) / (zs[j1] - zs[j0]);
    return Math.hypot(dx, dz);
  }

  // How much light reaches each grid point, about 0.2 to 1: the sun's, unless
  // the land itself or one of `shades` is between it and the sun, and less
  // in hollows and at the feet of cliffs and things standing on it.
  private *bakeLight(shades: readonly Shade[]): Stepwise<Float32Array> {
    const { xs, zs, heights } = this;
    const n = xs.length;
    const m = zs.length;
    const sun = new Float32Array(n * m).fill(1);
    const ambient = new Float32Array(n * m).fill(1);
    const s = SUN_DIRECTION;
    // The land's own shadows, marching toward the sun.
    for (let j = 0; j < m; j++) {
      for (let i = 0; i < n; i++) {
        const k = j * n + i;
        const x = xs[i];
        const z = zs[j];
        if (Math.abs(x) > 140 || Math.abs(z + 8) > 140) continue;
        const y = heights[k] + 0.05;
        let dark = 0;
        for (let t = 0.8; t < 60; t += t < 10 ? 0.6 : 1.5) {
          const over = this.heightAt(x + s.x * t, z + s.z * t) - (y + s.y * t);
          if (over > 0) dark = Math.max(dark, smoothstep(0, 0.6, over));
          if (dark >= 1) break;
        }
        sun[k] = 1 - dark;
        // Hollows and cliff feet: how much of the sky the land round it
        // hides, from eight directions at two distances.
        let hidden = 0;
        for (let a = 0; a < 8; a++) {
          const dx = Math.cos((a * Math.PI) / 4);
          const dz = Math.sin((a * Math.PI) / 4);
          for (const d of [1.5, 4.5]) hidden += Math.max(0, Math.min(1, (this.heightAt(x + dx * d, z + dz * d) - y) / d - 0.15));
        }
        ambient[k] = 1 - Math.min(0.35, hidden * 0.045);
      }
      if (j % ROWS === ROWS - 1) yield;
    }
    // The shadows of the things standing on it.
    for (const shade of shades) {
      const ground = this.heightAt(shade.x, shade.z);
      const rise = Math.max(0, shade.y - ground);
      // Where its shadow's middle falls, and how far its shadow reaches.
      const cx = shade.x - (s.x / s.y) * rise;
      const cz = shade.z - (s.z / s.y) * rise;
      const reach = shade.radius / s.y + 0.5;
      const [i0] = locate(xs, Math.min(cx, shade.x) - reach);
      const [i1] = locate(xs, Math.max(cx, shade.x) + reach);
      const [j0] = locate(zs, Math.min(cz, shade.z) - reach);
      const [j1] = locate(zs, Math.max(cz, shade.z) + reach);
      for (let j = j0; j <= j1 + 1 && j < m; j++) {
        for (let i = i0; i <= i1 + 1 && i < n; i++) {
          const k = j * n + i;
          // From the ground point toward the sun: how near it passes the
          // ball's middle.
          const vx = shade.x - xs[i];
          const vy = shade.y - heights[k];
          const vz = shade.z - zs[j];
          const t = vx * s.x + vy * s.y + vz * s.z;
          if (t > 0) {
            const miss = Math.sqrt(Math.max(0, vx * vx + vy * vy + vz * vz - t * t));
            const cover = 1 - smoothstep(shade.radius * 0.7, shade.radius * 1.05, miss);
            sun[k] = Math.min(sun[k], 1 - cover * 0.85);
          }
          if (shade.foot) {
            const d = Math.hypot(xs[i] - shade.x, zs[j] - shade.z);
            if (d < shade.foot) ambient[k] *= 1 - CONTACT * (1 - d / shade.foot) ** 1.6;
          }
        }
      }
    }
    const light = new Float32Array(n * m);
    for (let k = 0; k < light.length; k++) light[k] = (SHADED + (1 - SHADED) * sun[k]) * ambient[k];
    return light;
  }

  private baked: Float32Array | null = null;
  private colors: Float32Array | null = null; // the ground's, as meshed (colorAt)

  // How much light reaches the ground at a point (about 0.2 to 1), as baked into
  // its colours: for tinting what stands there. Known once the mesh is
  // built; 1 before.
  lightAt(x: number, z: number): number {
    if (!this.baked) return 1;
    const [i, u] = locate(this.xs, x);
    const [j, v] = locate(this.zs, z);
    const n = this.xs.length;
    const l = this.baked;
    const a = l[j * n + i];
    const b = l[j * n + i + 1];
    const c = l[(j + 1) * n + i];
    const d = l[(j + 1) * n + i + 1];
    return a + (b - a) * u + (c - a) * v + (a - b - c + d) * u * v;
  }
}

// Grid lines from `from` to `to` every `step` m, then further apart outward
// both ways, each step 10% longer than the last up to 14 m, out to `far`.
function axis(from: number, to: number, step: number, far: number): Float64Array {
  const middle: number[] = [];
  const count = Math.round((to - from) / step);
  for (let i = 0; i <= count; i++) middle.push(from + i * step);
  const after: number[] = [];
  let s = step;
  let v = middle[middle.length - 1];
  while (v < far) {
    s = Math.min(14, s * 1.1);
    v += s;
    after.push(v);
  }
  const before: number[] = [];
  s = step;
  v = middle[0];
  while (v > -far) {
    s = Math.min(14, s * 1.1);
    v -= s;
    before.push(v);
  }
  return new Float64Array([...before.reverse(), ...middle, ...after]);
}

// Which cell of an axis a value falls in, and how far across it (0..1),
// clamped to the axis.
function locate(axis: Float64Array, value: number): [number, number] {
  const last = axis.length - 2;
  if (value <= axis[0]) return [0, 0];
  if (value >= axis[last + 1]) return [last, 1];
  let lo = 0;
  let hi = last + 1;
  while (hi - lo > 1) {
    const mid = (lo + hi) >> 1;
    if (axis[mid] <= value) lo = mid;
    else hi = mid;
  }
  return [lo, (value - axis[lo]) / (axis[lo + 1] - axis[lo])];
}
