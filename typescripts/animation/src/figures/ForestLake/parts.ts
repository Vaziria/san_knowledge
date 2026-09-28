import * as THREE from 'three';
import { ConvexGeometry } from 'three/addons/geometries/ConvexGeometry.js';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';

// What the forest lake's assets share. The forest lake is built as the user's
// reference images ("Spring - Forest Lake", and the forest lake in all four
// seasons) show it: bright, saturated, low poly, every face flat shaded, with
// chunky bevelled wood, lavender-grey rock, green pines, pink cherry blossom
// and fresh green grass in spring. Its assets take their colours from the
// palette below, picked from those images, not from the theme: the user
// asked for the reference as it is, with the project's own look kept out of
// it. Each season changes what grows (palette()), and in winter snow lies on
// everything (snowOn()).
//
// Every asset is built from BufferGeometry made here: triangles with a
// normal and a colour at each corner (Shape), so a whole asset is one mesh
// per material, however many pieces it has, and the environment can merge
// or instance them.

// sRGB, as picked from the reference: spring's.
export const PALETTE = {
  wood: 0xb86d33, // warm orange-brown planks and rails
  post: 0x9f5b2c, // posts, a little darker and more weathered than the planks
  woodDark: 0x7d4522, // wood in the water or the shade: piles, legs
  woodGroove: 0x5c3117, // cracks and grooves along the grain
  woodEnd: 0xe6b374, // sawn end grain, pale
  woodCore: 0xc4884c, // the middle of the end grain, round the pith
  nail: 0x3b3532,
  iron: 0x45403f, // lantern frames, hoops, brackets
  grassTip: 0xb9e05a, // fresh lime green, the tips of the blades
  grass: 0x80bf38,
  grassFoot: 0x4b8a27, // the blades' feet, in their own shade
  pineLight: 0x96cc49, // a pine's tiers, lit from above
  pine: 0x4f9d39,
  pineDark: 0x245f2a, // their undersides and the shade between tiers
  bark: 0x8e4f24, // trunks: orange-brown, as the reference's
  barkDark: 0x5b3019,
  blossomLight: 0xffcde3, // cherry blossom, lit
  blossom: 0xf799c5,
  blossomDeep: 0xde6399, // in the shade of the crown
  blossomWarm: 0xffb5c9, // where the warm sun catches it
  rockLight: 0xd3cbd4, // lavender-grey rock, lit from above
  rock: 0xa89fae,
  rockDark: 0x736d80, // its undersides and crevices
  moss: 0x8cc23f,
  leafLight: 0x8fd24f, // leaves: a bush's, a broadleaf tree's crown
  leaf: 0x51a63a,
  leafDark: 0x2b6d2b,
  petalPink: 0xf27fb2,
  petalLight: 0xf9c4dd,
  petalWhite: 0xfbf3f6,
  petalLilac: 0xc6a3ec,
  petalOrange: 0xf68a2e,
  petalBlue: 0x72aaeb,
  petalYellow: 0xf8d445,
  flowerHeart: 0xf6be2c, // the flowers' middles
  berry: 0xd8262d, // red berries on a bush or a sprig
  lily: 0x7cc444, // lily pads
  lilyDark: 0x4e982d,
  lotus: 0xf9b3d0, // the water lily's petals, pale at the foot
  lotusTip: 0xef6aa5, // and deep pink at the tips
  reed: 0x6fae3c, // reeds' blades
  cattail: 0x7a4a26, // a cattail's brown head
  capRed: 0xd9362b, // a mushroom's red cap, spotted white
  capTan: 0xc98f55, // a brown mushroom's cap
  mushroomStem: 0xf2e6cf,
  canvas: 0xf0e2bf, // the tent
  canvasShade: 0xd3bf92,
  rope: 0xd9c08d,
  stone: 0xc3beb5, // the ruins' dressed stone
  stoneDark: 0x8f897f,
  flame: 0xffa531,
  flameCore: 0xfff0a0,
  flameEdge: 0xff5a14,
  glass: 0xffc452, // a lantern's glass, lit from inside
  flag: 0xe8542c,
  path: 0xd9a35f, // sandy orange dirt: the paths, a cave's floor
  pathLight: 0xe8bd7e,
  snow: 0xf7fafd, // winter's snow, lit
  snowShade: 0xc6d8ec, // and in its own shade, blue
  ice: 0xcdebf7, // icicles
} as const;

export type Palette = { readonly [K in keyof typeof PALETTE]: number };

// The four seasons the forest lake comes in, as the reference draws them:
// spring fresh green with cherry blossom and flowers; summer a deeper,
// denser green; autumn orange, red and gold with fallen leaves; winter under
// snow, the lake frozen.
export type Season = 'spring' | 'summer' | 'autumn' | 'winter';
export const SEASONS: readonly Season[] = ['spring', 'summer', 'autumn', 'winter'];

// What each season changes in the palette: the grass, the pines, leaves and
// the cherry's crown (in leaf in summer, red in autumn, under snow in
// winter), moss, flowers (autumn's orange and red, winter's red berries),
// lily pads, the tent's canvas and, in winter, the paths.
const SEASON_COLORS: Record<Season, Partial<Palette>> = {
  spring: {},
  summer: {
    grassTip: 0xa4d84a,
    grass: 0x63ae36,
    grassFoot: 0x377c25,
    pineLight: 0x80c142,
    pine: 0x3e8c35,
    pineDark: 0x1d5626,
    leafLight: 0x80ca4a,
    leaf: 0x41993a,
    leafDark: 0x21612a,
    blossomLight: 0x98d65a,
    blossom: 0x52a53c,
    blossomDeep: 0x2a6c2d,
    blossomWarm: 0xaedd62,
    moss: 0x72b33a,
    petalPink: 0xf5a9cb,
    petalLilac: 0xfbf3f6,
    petalOrange: 0xf8d445,
    petalBlue: 0xfbf3f6,
    lily: 0x6cbb3e,
    lilyDark: 0x3f8f2c,
    lotus: 0xfff8fb,
    lotusTip: 0xf7d2e2,
    reed: 0x5c9f35,
  },
  autumn: {
    grassTip: 0xf2c75c,
    grass: 0xd69d3e,
    grassFoot: 0x92652a,
    pineLight: 0x86b347,
    pine: 0x417f36,
    pineDark: 0x1f4f27,
    leafLight: 0xffb64c,
    leaf: 0xea7a2b,
    leafDark: 0xa9441d,
    blossomLight: 0xffa05e,
    blossom: 0xe5542f,
    blossomDeep: 0xa6301f,
    blossomWarm: 0xffbe5c,
    moss: 0xaaa53e,
    petalPink: 0xe9553b,
    petalLight: 0xf7a24a,
    petalWhite: 0xf6e6c6,
    petalLilac: 0xd9472c,
    petalOrange: 0xf5872a,
    petalBlue: 0xf3b83b,
    petalYellow: 0xf7cc3e,
    lily: 0xc8b442,
    lilyDark: 0x9b7c2b,
    reed: 0xc99a45,
    canvas: 0xf2a65a,
    canvasShade: 0xcc742d,
  },
  winter: {
    grassTip: 0xf3f7fa,
    grass: 0xc7d4dc,
    grassFoot: 0x7d8d84,
    pineLight: 0x5e9a50,
    pine: 0x2f6d3d,
    pineDark: 0x173f29,
    leafLight: 0x71926c,
    leaf: 0x405f46,
    leafDark: 0x243a2d,
    blossomLight: 0xffffff,
    blossom: 0xedf3f9,
    blossomDeep: 0xb4c7dc,
    blossomWarm: 0xfbfdff,
    moss: 0xeef4fa,
    petalPink: 0xd8262d,
    petalLight: 0xe8404a,
    petalWhite: 0xc91f2a,
    petalLilac: 0xd8262d,
    petalOrange: 0xe23b35,
    petalBlue: 0xc91f2a,
    petalYellow: 0xe8404a,
    flowerHeart: 0x8a1c1c,
    lily: 0xb9cfd6,
    lilyDark: 0x8fa9b3,
    reed: 0xcdb98a,
    canvas: 0xf4efe4,
    canvasShade: 0xcbc4b6,
    path: 0xdcd3c7,
    pathLight: 0xeee9e2,
  },
};

// The palette in a season: spring's, with what that season changes.
export function palette(season: Season = 'spring'): Palette {
  return { ...PALETTE, ...SEASON_COLORS[season] };
}

// A colour of the palette, as three.js keeps colours (linear).
export function color(hex: number): THREE.Color {
  return new THREE.Color(hex);
}

// The same colour lighter (> 1) or darker (< 1).
export function shade(base: THREE.Color, factor: number): THREE.Color {
  return base.clone().multiplyScalar(factor);
}

// A slightly different colour for another piece of the same stuff: a little
// lighter or darker and a little warmer or cooler, `amount` 0..1 of the most.
export function vary(base: THREE.Color, random: () => number, amount = 1): THREE.Color {
  return base.clone().offsetHSL((random() - 0.5) * 0.02 * amount, (random() - 0.5) * 0.1 * amount, (random() - 0.5) * 0.06 * amount);
}

// Materials, each named, so the environment can merge every mesh that uses
// the same kind into one (merge()). Matte: coloured by the shape's corners
// and lit. Two-sided: for thin things seen from both sides (petals, leaves,
// the tent's canvas, a flag). Glowing: shows its colours as they are, unlit
// (flames, a lantern's glass, the glow in the cave).
export function matte(): THREE.MeshStandardMaterial {
  const material = new THREE.MeshStandardMaterial({ vertexColors: true, roughness: 0.9, metalness: 0 });
  material.name = 'matte';
  vivid(material);
  return material;
}

export function twoSided(): THREE.MeshStandardMaterial {
  const material = new THREE.MeshStandardMaterial({ vertexColors: true, roughness: 0.9, metalness: 0, side: THREE.DoubleSide });
  material.name = 'two-sided';
  vivid(material);
  return material;
}

// As the reference paints its shade: colours kept vivid where the light is
// dim, rather than dulled toward grey (a pink crown's shaded side deep pink,
// not mauve). The lit colour is pushed away from its own grey, the more the
// darker it is, not at all where it is bright. Shared by every lit material
// of the forest lake (Terrain's ground too, VIVID).
export const VIVID = /* glsl */ `
  {
    float lum = dot(outgoingLight, vec3(0.2126, 0.7152, 0.0722));
    outgoingLight = max(vec3(0.0), mix(vec3(lum), outgoingLight, 1.0 + 0.35 * (1.0 - smoothstep(0.08, 0.7, lum))));
  }
  #include <opaque_fragment>`;
function vivid(material: THREE.MeshStandardMaterial): void {
  material.onBeforeCompile = (shader) => {
    shader.fragmentShader = shader.fragmentShader.replace('#include <opaque_fragment>', VIVID);
  };
  material.customProgramCacheKey = () => 'forest-lake-vivid';
}

export function glowing(): THREE.MeshBasicMaterial {
  const material = new THREE.MeshBasicMaterial({ vertexColors: true });
  material.name = 'glowing';
  return material;
}

export function mesh(geometry: THREE.BufferGeometry, material: THREE.Material, shadows = true): THREE.Mesh {
  const m = new THREE.Mesh(geometry, material);
  m.castShadow = shadows;
  m.receiveShadow = shadows;
  return m;
}

// A soft round glow round something that gives off light (a lantern, a
// flame): a sprite that adds its colour to what is behind it, brightest in
// the middle. `size` is its width in meters.
let haloTexture: THREE.DataTexture | null = null;
export function halo(hex: number, size: number, strength = 1): THREE.Sprite {
  if (!haloTexture) {
    const n = 64;
    const data = new Uint8Array(n * n * 4);
    for (let y = 0; y < n; y++) {
      for (let x = 0; x < n; x++) {
        const d = Math.hypot(x + 0.5 - n / 2, y + 0.5 - n / 2) / (n / 2);
        const a = Math.max(0, 1 - d) ** 2.2;
        data.set([255, 255, 255, Math.round(255 * a)], (y * n + x) * 4);
      }
    }
    haloTexture = new THREE.DataTexture(data, n, n);
    // Smoothly filtered: a DataTexture is nearest by default, blocky close up.
    haloTexture.magFilter = THREE.LinearFilter;
    haloTexture.minFilter = THREE.LinearFilter;
    haloTexture.needsUpdate = true;
  }
  const material = new THREE.SpriteMaterial({
    map: haloTexture,
    color: new THREE.Color(hex).multiplyScalar(strength),
    blending: THREE.AdditiveBlending,
    depthWrite: false,
    transparent: true,
  });
  material.name = 'halo';
  const sprite = new THREE.Sprite(material);
  sprite.scale.setScalar(size);
  return sprite;
}

// A repeatable random number generator (mulberry32): the same seed always
// gives the same numbers, in 0..1.
export function seededRandom(seed: number): () => number {
  let a = seed;
  return () => {
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// A number between min and max.
export function between(random: () => number, min: number, max: number): number {
  return min + (max - min) * random();
}

// Smooth value noise in 0..1 over the plane, the same for the same seed, and
// several octaves of it (fbm), for colouring and shaping land.
function hash2(x: number, z: number, seed: number): number {
  let h = Math.imul(x | 0, 374761393) ^ Math.imul(z | 0, 668265263) ^ Math.imul(seed | 0, 982451653);
  h = Math.imul(h ^ (h >>> 13), 1274126177);
  return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
}
export function noise2(x: number, z: number, seed = 0): number {
  const ix = Math.floor(x);
  const iz = Math.floor(z);
  const fx = x - ix;
  const fz = z - iz;
  const sx = fx * fx * (3 - 2 * fx);
  const sz = fz * fz * (3 - 2 * fz);
  const a = hash2(ix, iz, seed);
  const b = hash2(ix + 1, iz, seed);
  const c = hash2(ix, iz + 1, seed);
  const d = hash2(ix + 1, iz + 1, seed);
  return a + (b - a) * sx + (c - a) * sz + (a - b - c + d) * sx * sz;
}
export function fbm2(x: number, z: number, seed = 0, octaves = 4): number {
  let sum = 0;
  let amplitude = 0.5;
  let total = 0;
  for (let i = 0; i < octaves; i++) {
    sum += amplitude * noise2(x, z, seed + i * 17);
    total += amplitude;
    x *= 2.03;
    z *= 2.03;
    amplitude *= 0.5;
  }
  return sum / total;
}

const ab = new THREE.Vector3();
const ac = new THREE.Vector3();
const faceNormal = new THREE.Vector3();

// Triangles being put together into one BufferGeometry: each has its own
// three corners, so each face is flat shaded, with a colour at each corner.
export class Shape {
  private readonly positions: number[] = [];
  private readonly normals: number[] = [];
  private readonly colors: number[] = [];

  get triangles(): number {
    return this.positions.length / 9;
  }

  // A triangle, its corners counter-clockwise seen from the side it faces.
  // One colour for the face, or one per corner, blended across it. Its
  // normal is the face's, or `normal` for all three corners (grass blades
  // lit from above whichever way they face).
  triangle(a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3, ca: THREE.Color, cb = ca, cc = ca, normal?: THREE.Vector3): void {
    const n = normal ?? faceNormal.crossVectors(ab.subVectors(b, a), ac.subVectors(c, a)).normalize();
    for (const [p, k] of [
      [a, ca],
      [b, cb],
      [c, cc],
    ] as const) {
      this.positions.push(p.x, p.y, p.z);
      this.normals.push(n.x, n.y, n.z);
      this.colors.push(k.r, k.g, k.b);
    }
  }

  // A four-cornered face, counter-clockwise, as two triangles: one colour, or
  // one per corner.
  quad(a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3, d: THREE.Vector3, ca: THREE.Color, cb = ca, cc = ca, cd = ca): void {
    this.triangle(a, b, c, ca, cb, cc);
    this.triangle(a, c, d, ca, cc, cd);
  }

  geometry(): THREE.BufferGeometry {
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(this.positions, 3));
    geometry.setAttribute('normal', new THREE.Float32BufferAttribute(this.normals, 3));
    geometry.setAttribute('color', new THREE.Float32BufferAttribute(this.colors, 3));
    geometry.computeBoundingBox();
    geometry.computeBoundingSphere();
    return geometry;
  }
}

// A crack along one face of a bar, where the wood has split along its grain:
// a narrow V cut into the face, deepest in its middle and closing at its
// ends. `face`: 0 back (-z), 1 top (+y), 2 front (+z), 3 bottom (-y). `at`
// is where it runs across the face (0 one edge, 1 the other), `from` and
// `to` where it starts and ends along the bar (0 its start, 1 its end).
export interface Crack {
  face: 0 | 1 | 2 | 3;
  at: number;
  from: number;
  to: number;
  depth: number; // m
  width: number; // m, across the V at the face
}

export interface BarColors {
  side: THREE.Color;
  groove: THREE.Color;
  end: THREE.Color; // sawn end grain
  core: THREE.Color; // the end grain's middle
}

// A squared length of wood, along x from 0 to `length`: `width` deep in z
// and `height` tall in y, its long edges cut off by `chamfer` and the edges
// round each end by `bevel`, so every edge catches the light. Hand cut: it
// bends (`bow`, in y and z at its middle), twists, and each corner strays by
// up to `rough`; the faces between are flat, so it looks hewn rather than
// sawn. The far end may be cut off aslant (`slant`: x per m of y and of z),
// and the near end too (`slantStart`).
// Ends left open (`caps`) are for a buried end.
export interface BarOptions {
  length: number;
  width: number;
  height: number;
  chamfer: number;
  bevel: number;
  segments: number; // lengths it is split into between its ends
  rough?: number;
  bow?: readonly [number, number];
  twist?: number; // radians, start to end
  slant?: readonly [number, number];
  slantStart?: readonly [number, number];
  cracks?: readonly Crack[];
  caps?: readonly [boolean, boolean];
  colors: BarColors;
  random: () => number;
}

// Adds a bar to a shape, placed by `matrix` (its x along the bar). Each side
// face gets its own shade of the side colour; the cut-off edges are lighter,
// as worn edges catch the light; the ends show pale end grain, darker toward
// the middle.
export function addBar(shape: Shape, matrix: THREE.Matrix4, options: BarOptions): void {
  const { length, width, height, chamfer, bevel, segments, random, colors } = options;
  const rough = options.rough ?? 0;
  const [bowY, bowZ] = options.bow ?? [0, 0];
  const twist = options.twist ?? 0;
  const [slantY, slantZ] = options.slant ?? [0, 0];
  const [startY, startZ] = options.slantStart ?? [0, 0];
  const cracks = options.cracks ?? [];
  const [capStart, capEnd] = options.caps ?? [true, true];

  // Where the rings of the bar stand along it, and how far each is inset for
  // the bevel at the ends.
  const stations: { x: number; inset: number }[] = [];
  const b = Math.min(bevel, length / 4);
  if (b > 0 && capStart) stations.push({ x: 0, inset: b });
  const x0 = b > 0 && capStart ? b : 0;
  const x1 = b > 0 && capEnd ? length - b : length;
  for (let i = 0; i <= segments; i++) stations.push({ x: x0 + ((x1 - x0) * i) / segments, inset: 0 });
  if (b > 0 && capEnd) stations.push({ x: length, inset: b });

  // The corners of each ring, as (u, v): v is y, u is -z, so they run
  // counter-clockwise seen from the far end, and the faces they bound face
  // outward. Corner k: 0,1 on the back face (u = a), 2,3 top, 4,5 front,
  // 6,7 bottom.
  const strays = stations.map(() => Array.from({ length: 8 }, () => [(random() - 0.5) * 2 * rough, (random() - 0.5) * 2 * rough]));
  const inward = [
    [-1, 0],
    [0, -1],
    [1, 0],
    [0, 1],
  ];
  // The cracks each face has room for, each with where it runs across the
  // face and its half width there, both as fractions of the face: kept off
  // the face's edges and each other, the same for every ring.
  const spans = [height - 2 * chamfer, width - 2 * chamfer, height - 2 * chamfer, width - 2 * chamfer];
  const faceCracks = [0, 1, 2, 3].map((face) => {
    const fitted: { crack: Crack; at: number; half: number }[] = [];
    let taken = 0;
    for (const crack of cracks.filter((c) => c.face === face).sort((p, q) => p.at - q.at)) {
      const half = crack.width / 2 / Math.max(spans[face], 1e-6);
      const at = Math.min(Math.max(crack.at, half + 0.05), 1 - half - 0.05);
      if (half > 0.3 || at - half < taken + 0.02) continue;
      fitted.push({ crack, at, half });
      taken = at + half;
    }
    return fitted;
  });

  const rings: THREE.Vector3[][] = [];
  const dark: number[][] = []; // per ring point, 0..1 how far into a crack (for its colour)
  stations.forEach(({ x, inset }, r) => {
    const t = length > 0 ? x / length : 0;
    const a = width / 2 - inset;
    const hb = height / 2 - inset;
    const c = inset > 0 ? Math.max(chamfer - inset * 0.6, chamfer * 0.35) : chamfer;
    const corners = [
      [a, -hb + c],
      [a, hb - c],
      [a - c, hb],
      [-a + c, hb],
      [-a, hb - c],
      [-a, -hb + c],
      [-a + c, -hb],
      [a - c, -hb],
    ].map(([u, v], k) => [u + strays[r][k][0] * (inset > 0 ? 0.4 : 1), v + strays[r][k][1] * (inset > 0 ? 0.4 : 1)]);
    const points: number[][] = [];
    const depthOf: number[] = [];
    for (let face = 0; face < 4; face++) {
      const [ua, va] = corners[2 * face];
      const [ub, vb] = corners[2 * face + 1];
      points.push([ua, va]);
      depthOf.push(0);
      for (const { crack, at, half } of faceCracks[face]) {
        const inside = t > crack.from && t < crack.to ? Math.sin((Math.PI * (t - crack.from)) / (crack.to - crack.from)) : 0;
        const depth = crack.depth * Math.sqrt(inside);
        const lerp = (s: number) => [ua + (ub - ua) * s, va + (vb - va) * s];
        const [um, vm] = lerp(at);
        points.push(lerp(at - half), [um + inward[face][0] * depth, vm + inward[face][1] * depth], lerp(at + half));
        depthOf.push(0, depth > 0 ? Math.min(1, depth / Math.max(crack.depth, 1e-6)) : 0, 0);
      }
      points.push([ub, vb]);
      depthOf.push(0);
    }
    // Bent, twisted and, at the far end, cut off square.
    const turn = twist * t;
    const cos = Math.cos(turn);
    const sin = Math.sin(turn);
    const bend = Math.sin(Math.PI * t);
    const far = r >= stations.length - (b > 0 && capEnd ? 2 : 1);
    const near = r < (b > 0 && capStart ? 2 : 1);
    rings.push(
      points.map(([u, v]) => {
        const y = v * cos - u * sin + bowY * bend;
        const z = -(u * cos + v * sin) + bowZ * bend;
        const slant = far ? slantY * y + slantZ * z : near ? startY * y + startZ * z : 0;
        return new THREE.Vector3(x + slant, y, z).applyMatrix4(matrix);
      }),
    );
    dark.push(depthOf);
  });

  // The sides: each face between two rings, in its own shade; the edges cut
  // off lighter; the cracks dark, fading out where they close.
  const count = rings[0].length;
  // Which face each edge of a ring lies on, or -1 for a cut-off corner.
  const edgeFace: number[] = [];
  for (let face = 0; face < 4; face++) {
    const along = 1 + 3 * faceCracks[face].length;
    for (let i = 0; i < along; i++) edgeFace.push(face);
    edgeFace.push(-1); // from the face's last corner to the next face's first
  }
  // Each face its own shade of the wood, drifting a little lighter and
  // darker along the grain.
  const faceShades = [0, 1, 2, 3].map(() => vary(colors.side, random, 1.2));
  const edgeShade = shade(colors.side, 1.35);
  const drift = rings.map(() => 1 + (random() - 0.5) * 0.14);
  for (let r = 0; r + 1 < rings.length; r++) {
    const [p, q] = [rings[r], rings[r + 1]];
    for (let i = 0; i < count; i++) {
      const j = (i + 1) % count;
      const face = edgeFace[i];
      const base = face < 0 ? edgeShade : faceShades[face];
      const tint = (ring: number, k: number) => {
        const c = shade(base, drift[ring]);
        return k > 0 ? c.lerp(colors.groove, k) : c;
      };
      shape.quad(p[i], p[j], q[j], q[i], tint(r, dark[r][i]), tint(r, dark[r][j]), tint(r + 1, dark[r + 1][j]), tint(r + 1, dark[r + 1][i]));
    }
  }

  // The ends: end grain, paler at the rim, darker toward the middle.
  if (capStart) addCap(shape, rings[0], true, colors, random);
  if (capEnd) addCap(shape, rings[rings.length - 1], false, colors, random);
}

// Closes a ring of a loft with sawn end grain: pale at the rim, darker
// toward the middle, with a darker growth ring between when `rings` asks.
// The ring runs counter-clockwise seen from the side the cap faces, or the
// other way when `reverse`.
export function addCap(shape: Shape, ring: THREE.Vector3[], reverse: boolean, colors: Pick<BarColors, 'end' | 'core'>, random: () => number, rings = false): void {
  const middle = ring.reduce((sum, p) => sum.add(p), new THREE.Vector3()).divideScalar(ring.length);
  const core = vary(colors.core, random, 0.5);
  const rim = vary(colors.end, random, 0.5);
  const fan = (outer: THREE.Vector3[], inner: THREE.Vector3[] | null, outerColor: THREE.Color, innerColor: THREE.Color) => {
    for (let i = 0; i < outer.length; i++) {
      const j = (i + 1) % outer.length;
      const [a, b] = reverse ? [outer[j], outer[i]] : [outer[i], outer[j]];
      if (!inner) {
        shape.triangle(middle, a, b, innerColor, outerColor, outerColor);
        continue;
      }
      const [c, d] = reverse ? [inner[i], inner[j]] : [inner[j], inner[i]];
      shape.quad(a, b, c, d, outerColor, outerColor, innerColor, innerColor);
    }
  };
  if (!rings) {
    fan(ring, null, rim, core);
    return;
  }
  // Two bands of growth rings, then the heart.
  const toward = (f: number) => ring.map((p) => p.clone().lerp(middle, f));
  const mid = toward(0.4);
  const inner = toward(0.72);
  fan(ring, mid, rim, shade(rim, 0.92));
  fan(mid, inner, shade(core, 1.08), shade(rim, 0.97));
  fan(inner, null, core, shade(core, 0.85));
}

// A frame for a piece of wood: its x along `along`, its z as near `out` as
// that allows, and its y making them right-handed, with its start at `at`.
export function frame(at: THREE.Vector3, along: THREE.Vector3, out: THREE.Vector3): THREE.Matrix4 {
  const x = along.clone().normalize();
  const z = out.clone().addScaledVector(x, -out.dot(x)).normalize();
  const y = new THREE.Vector3().crossVectors(z, x);
  return new THREE.Matrix4().makeBasis(x, y, z).setPosition(at);
}

// The colours of a piece of wood: its sides `side` (the planks' by default),
// with the grain's grooves and the pale end grain.
export function woodColors(side: number = PALETTE.wood): BarColors {
  return { side: color(side), groove: color(PALETTE.woodGroove), end: color(PALETTE.woodEnd), core: color(PALETTE.woodCore) };
}

// Lines of grain split open along a piece of wood: on each of `faces`, a
// number of them from `count` (min, max), spread evenly across the face so
// they never cross, each starting somewhere from `from` along the piece (a
// post's above the ground) and running a good way along it.
export function grain(random: () => number, faces: readonly Crack['face'][], [least, most]: readonly [number, number], from = 0.02): Crack[] {
  return faces.flatMap((face) => {
    const count = Math.floor(between(random, least, most + 0.999));
    return Array.from({ length: count }, (_, k) => {
      const start = between(random, from, 0.35);
      return {
        face,
        at: (k + 0.5 + between(random, -0.2, 0.2)) / count,
        from: start,
        to: Math.min(0.98, start + between(random, 0.35, 0.75)),
        depth: between(random, 0.003, 0.006),
        width: between(random, 0.006, 0.009),
      };
    });
  });
}

// A squared piece of wood from `from` to `to`: `tall` across it in the way
// nearest `up`, `thick` the other way, its edges cut off and its grain split
// here and there; `options` override any of the bar's (colours, cracks,
// roughness...).
export function addBeam(
  shape: Shape,
  from: THREE.Vector3,
  to: THREE.Vector3,
  up: THREE.Vector3,
  tall: number,
  thick: number,
  random: () => number,
  options: Partial<BarOptions> = {},
): void {
  const along = to.clone().sub(from);
  const out = new THREE.Vector3().crossVectors(along, up);
  if (out.lengthSq() < 1e-10) out.set(0, 0, 1).cross(along);
  if (out.lengthSq() < 1e-10) out.set(1, 0, 0);
  const small = Math.min(tall, thick);
  addBar(shape, frame(from, along, out), {
    length: along.length(),
    width: thick,
    height: tall,
    chamfer: small * 0.2,
    bevel: small * 0.12,
    segments: Math.max(1, Math.round(along.length() / 0.6)),
    rough: small * 0.03,
    cracks: along.length() > 0.4 ? grain(random, [1, 2], [0, 1]) : [],
    colors: woodColors(),
    random,
    ...options,
  });
}

// A round wooden pole or pile from `from` to `to`, `radius` thick, tapering
// to `top` of that at its far end, its bark-less wood darker in `shade` of
// its length nearest `from` (a pile's wet foot). The far end is sawn.
export function addPole(shape: Shape, from: THREE.Vector3, to: THREE.Vector3, radius: number, random: () => number, top = 0.9, side: number = PALETTE.post, sides = 7): void {
  const steps = Math.max(2, Math.round(from.distanceTo(to) / 0.5));
  const path = Array.from({ length: steps + 1 }, (_, i) => from.clone().lerp(to, i / steps));
  const radii = path.map((_, i) => radius * (1 - (1 - top) * (i / steps)));
  const base = color(side);
  const shades = Array.from({ length: sides }, () => vary(base, random, 1.4));
  addTube(shape, path, radii, { sides, rough: 0.05, random, paint: (_, i) => shades[i], caps: ['open', 'grain'] });
}

// Rings joined into a closed surface, each ring's points counter-clockwise
// seen from the end the rings run toward, all rings the same length.
// `paint(r, i)` colours the face between ring r and r + 1 from point i to
// i + 1. The ends are left open.
export function addLoft(shape: Shape, rings: THREE.Vector3[][], paint: (r: number, i: number) => THREE.Color): void {
  const count = rings[0].length;
  for (let r = 0; r + 1 < rings.length; r++) {
    const [p, q] = [rings[r], rings[r + 1]];
    for (let i = 0; i < count; i++) {
      const j = (i + 1) % count;
      shape.quad(p[i], p[j], q[j], q[i], paint(r, i));
    }
  }
}

// A round tapering length along a path (a trunk, a branch, a root, a log, a
// pole): `sides` faces round, `radii` at each point of the path, each ring's
// points straying by up to `rough` of its radius. `paint(r, i)` colours its
// faces; the ends are closed with `caps` (sawn end grain, growth rings and
// all), or with the side colour, or left open.
export interface TubeOptions {
  sides: number;
  rough?: number;
  random: () => number;
  paint: (r: number, i: number) => THREE.Color;
  caps?: readonly [Cap, Cap];
  turn?: number; // radians the first ring is turned about the path
}
export type Cap = 'open' | 'grain' | 'rings' | THREE.Color;

export function addTube(shape: Shape, path: readonly THREE.Vector3[], radii: readonly number[], options: TubeOptions): THREE.Vector3[][] {
  const { sides, random } = options;
  const rough = options.rough ?? 0;
  // A frame carried along the path without twisting (parallel transport).
  const tangents = path.map((_, i) => {
    const a = path[Math.max(0, i - 1)];
    const b = path[Math.min(path.length - 1, i + 1)];
    return b.clone().sub(a).normalize();
  });
  const first = tangents[0];
  let normal = Math.abs(first.y) < 0.9 ? new THREE.Vector3(0, 1, 0) : new THREE.Vector3(1, 0, 0);
  normal = normal.addScaledVector(first, -normal.dot(first)).normalize();
  if (options.turn) normal.applyAxisAngle(first, options.turn);
  const rings: THREE.Vector3[][] = [];
  path.forEach((p, k) => {
    const t = tangents[k];
    if (k > 0) {
      const axis = new THREE.Vector3().crossVectors(tangents[k - 1], t);
      if (axis.lengthSq() > 1e-12) normal.applyAxisAngle(axis.normalize(), Math.acos(THREE.MathUtils.clamp(tangents[k - 1].dot(t), -1, 1)));
      normal.addScaledVector(t, -normal.dot(t)).normalize();
    }
    const binormal = new THREE.Vector3().crossVectors(t, normal);
    const ring: THREE.Vector3[] = [];
    for (let i = 0; i < sides; i++) {
      const angle = (2 * Math.PI * i) / sides;
      const r = radii[k] * (1 + (random() - 0.5) * 2 * rough);
      ring.push(
        p
          .clone()
          .addScaledVector(normal, Math.cos(angle) * r)
          .addScaledVector(binormal, Math.sin(angle) * r),
      );
    }
    rings.push(ring);
  });
  addLoft(shape, rings, options.paint);
  const [start, end] = options.caps ?? ['open', 'open'];
  const cap = (ring: THREE.Vector3[], reverse: boolean, style: Cap) => {
    if (style === 'open') return;
    if (style instanceof THREE.Color) {
      addCap(shape, ring, reverse, { end: style, core: style }, random);
      return;
    }
    addCap(shape, ring, reverse, { end: color(PALETTE.woodEnd), core: color(PALETTE.woodCore) }, random, style === 'rings');
  };
  cap(rings[0], true, start);
  cap(rings[rings.length - 1], false, end);
  return rings;
}

// A shape turned on a lathe round y (a barrel, a lantern's roof, a stone
// drum): `profile` as (radius, height) from its foot up, `sides` faces round,
// placed by `matrix`. `paint(r, i)` colours the face from profile point r to
// r + 1, side i to i + 1; a radius of 0 closes it to a point.
export function addLathe(shape: Shape, profile: readonly (readonly [number, number])[], sides: number, matrix: THREE.Matrix4, paint: (r: number, i: number) => THREE.Color): void {
  const rings = profile.map(([radius, y]) =>
    Array.from({ length: sides }, (_, i) => {
      const angle = (2 * Math.PI * i) / sides;
      return new THREE.Vector3(radius * Math.cos(angle), y, -radius * Math.sin(angle)).applyMatrix4(matrix);
    }),
  );
  // A ring of radius 0 is a point: its faces are triangles.
  for (let r = 0; r + 1 < rings.length; r++) {
    const [p, q] = [rings[r], rings[r + 1]];
    for (let i = 0; i < sides; i++) {
      const j = (i + 1) % sides;
      const c = paint(r, i);
      if (profile[r][0] === 0) shape.triangle(p[i], q[j], q[i], c);
      else if (profile[r + 1][0] === 0) shape.triangle(p[i], p[j], q[i], c);
      else shape.quad(p[i], p[j], q[j], q[i], c);
    }
  }
}

// A convex lump of rock, or anything faceted like it, round the given
// points: the smallest convex shape holding them all, each face coloured by
// `paint` from its outward normal and its middle.
const hullA = new THREE.Vector3();
const hullB = new THREE.Vector3();
const hullC = new THREE.Vector3();
const hullN = new THREE.Vector3();
const hullM = new THREE.Vector3();
export function addHull(shape: Shape, points: THREE.Vector3[], paint: (normal: THREE.Vector3, middle: THREE.Vector3) => THREE.Color): void {
  const hull = new ConvexGeometry(points);
  const position = hull.getAttribute('position');
  for (let i = 0; i < position.count; i += 3) {
    hullA.fromBufferAttribute(position, i);
    hullB.fromBufferAttribute(position, i + 1);
    hullC.fromBufferAttribute(position, i + 2);
    hullN.crossVectors(ab.subVectors(hullB, hullA), ac.subVectors(hullC, hullA)).normalize();
    hullM.copy(hullA).add(hullB).add(hullC).divideScalar(3);
    shape.triangle(hullA, hullB, hullC, paint(hullN, hullM));
  }
  hull.dispose();
}

// Points for a rock: `count` of them scattered over an egg `size` across
// (x, y, z), each pushed in or out by up to `lump`, the bottom cut flat at
// `floor` (a fraction of the height down from the middle, -1 the bottom),
// so it sits on the ground. Their hull (addHull) is the rock.
export function rockPoints(random: () => number, size: THREE.Vector3, count = 16, lump = 0.22, floor = -0.55): THREE.Vector3[] {
  const points: THREE.Vector3[] = [];
  for (let i = 0; i < count; i++) {
    // Spread evenly over the sphere (a spiral), then roughened.
    const y = 1 - (2 * (i + 0.5)) / count;
    const r = Math.sqrt(1 - y * y);
    const angle = i * 2.39996 + random() * 0.6;
    const push = 1 + (random() - 0.5) * 2 * lump;
    points.push(new THREE.Vector3(Math.cos(angle) * r * push * (size.x / 2), Math.max(floor, y * push) * (size.y / 2), Math.sin(angle) * r * push * (size.z / 2)));
  }
  return points;
}

// How the reference paints rock: lavender grey, light where it faces the
// sky, dark underneath, each face its own shade; with `moss`, faces turned
// up past it are mossy green (the season's moss: snow in winter).
export function rockPaint(random: () => number, moss = 2, P: Palette = PALETTE): (normal: THREE.Vector3) => THREE.Color {
  const light = color(P.rockLight);
  const mid = color(P.rock);
  const dark = color(P.rockDark);
  const green = color(P.moss);
  return (normal) => {
    if (normal.y > moss) return vary(green, random, 1.5);
    const up = normal.y;
    const base = up > 0 ? mid.clone().lerp(light, Math.min(1, up * 1.25)) : mid.clone().lerp(dark, Math.min(1, -up * 1.1));
    return vary(base, random, 1.6);
  };
}

// A rock sitting on the ground at `at` (its foot), `size` across, turned by
// `turn` about y: the hull of rockPoints(), bedded a little into the ground,
// painted by rockPaint() (mossy on top past `moss`).
export function addRock(shape: Shape, at: THREE.Vector3, size: THREE.Vector3, turn: number, random: () => number, moss = 2, count = 16, P: Palette = PALETTE): void {
  const floor = -0.55;
  const matrix = new THREE.Matrix4().makeRotationY(turn).setPosition(at.x, at.y - floor * (size.y / 2) - size.y * 0.04, at.z);
  const paint = rockPaint(random, moss, P);
  addHull(
    shape,
    rockPoints(random, size, count, 0.22, floor).map((p) => p.applyMatrix4(matrix)),
    (normal) => paint(normal),
  );
}

// A patch of moss on the ground round `at`: a flat, ragged, bright green
// cushion `radius` across, a little raised in its middle (the season's moss:
// olive in autumn, a drift of snow in winter).
export function addMoss(shape: Shape, at: THREE.Vector3, radius: number, random: () => number, P: Palette = PALETTE): void {
  const green = color(P.moss);
  const middle = at.clone().add(new THREE.Vector3(0, radius * 0.12, 0));
  const count = 9;
  const rim = Array.from({ length: count }, (_, i) => {
    const angle = -(2 * Math.PI * i) / count;
    const r = radius * between(random, 0.6, 1.1);
    return new THREE.Vector3(at.x + Math.cos(angle) * r, at.y + 0.008, at.z + Math.sin(angle) * r);
  });
  // The rim runs counter-clockwise seen from above, so these face up.
  for (let i = 0; i < count; i++) shape.triangle(middle, rim[i], rim[(i + 1) % count], shade(green, 1.08), vary(green, random, 1.5), vary(green, random, 1.5));
}

// A lumpy ball (a cluster of blossom or leaves, a cloud): an icosahedron
// split `detail` times, each corner pushed in or out by up to `lump` of its
// radius, `radii` across in x, y and z, each face coloured by `paint` from
// its outward normal.
export function addBlob(shape: Shape, center: THREE.Vector3, radii: THREE.Vector3, detail: number, lump: number, random: () => number, paint: (normal: THREE.Vector3) => THREE.Color): void {
  const ico = new THREE.IcosahedronGeometry(1, detail);
  const position = ico.getAttribute('position');
  const pushes = new Map<string, number>();
  const corner = (i: number) => {
    const v = new THREE.Vector3().fromBufferAttribute(position, i);
    const key = `${v.x.toFixed(3)},${v.y.toFixed(3)},${v.z.toFixed(3)}`;
    let push = pushes.get(key);
    if (push === undefined) pushes.set(key, (push = 1 + (random() - 0.5) * 2 * lump));
    return v.multiplyScalar(push).multiply(radii).add(center);
  };
  for (let i = 0; i < position.count; i += 3) {
    const a = corner(i);
    const b = corner(i + 1);
    const c = corner(i + 2);
    const n = new THREE.Vector3().crossVectors(ab.subVectors(b, a), ac.subVectors(c, a)).normalize();
    shape.triangle(a, b, c, paint(n));
  }
  ico.dispose();
}

// A flat shape cut out of a board `depth` thick (a sign's arrow, a stone's
// outline): `outline` in x and y, counter-clockwise, its faces facing +z and
// -z, placed by `matrix`.
export function addExtrusion(shape: Shape, outline: readonly THREE.Vector2[], depth: number, matrix: THREE.Matrix4, colors: { face: THREE.Color; back?: THREE.Color; side: THREE.Color }): void {
  const points = THREE.ShapeUtils.isClockWise(outline as THREE.Vector2[]) ? [...outline].reverse() : [...outline];
  const front = points.map((p) => new THREE.Vector3(p.x, p.y, depth / 2).applyMatrix4(matrix));
  const back = points.map((p) => new THREE.Vector3(p.x, p.y, -depth / 2).applyMatrix4(matrix));
  for (const [a, b, c] of THREE.ShapeUtils.triangulateShape(points, [])) {
    // Counter-clockwise in x and y faces +z.
    const ccw = (points[b].x - points[a].x) * (points[c].y - points[a].y) - (points[b].y - points[a].y) * (points[c].x - points[a].x) > 0;
    const [i, j, k] = ccw ? [a, b, c] : [a, c, b];
    shape.triangle(front[i], front[j], front[k], colors.face);
    shape.triangle(back[i], back[k], back[j], colors.back ?? colors.face);
  }
  for (let i = 0; i < points.length; i++) {
    const j = (i + 1) % points.length;
    shape.quad(front[i], back[i], back[j], front[j], colors.side);
  }
}

// A cone of pine needles, one tier of a pine: `radius` round at its skirt,
// `height` tall, with `points` long drooping points round its skirt and a
// shorter one between each two, its sides bulging a little at a shoulder
// on the way up, and a shallow hollow under it. Lightest at its tip, darker
// down to its skirt, darkest underneath.
export function addTier(shape: Shape, base: THREE.Vector3, radius: number, height: number, points: number, droop: number, random: () => number, shouldered = true, P: Palette = PALETTE): void {
  const light = color(P.pineLight);
  const mid = color(P.pine);
  const dark = color(P.pineDark);
  const apex = base.clone().add(new THREE.Vector3((random() - 0.5) * radius * 0.08, height, (random() - 0.5) * radius * 0.08));
  const turn = random() * Math.PI * 2;
  const count = points * 2;
  const rim: THREE.Vector3[] = [];
  const shoulder: THREE.Vector3[] = [];
  for (let i = 0; i < count; i++) {
    const long = i % 2 === 0;
    const angle = turn - (2 * Math.PI * i) / count;
    const r = radius * (long ? between(random, 0.97, 1.06) : between(random, 0.8, 0.87));
    const y = long ? -droop * between(random, 0.8, 1.2) : droop * 0.2;
    rim.push(new THREE.Vector3(base.x + Math.cos(angle) * r, base.y + y, base.z + Math.sin(angle) * r));
    const rs = radius * (shouldered ? 0.6 : 0.5) * (long ? 1.04 : 0.97);
    shoulder.push(new THREE.Vector3(base.x + Math.cos(angle) * rs, base.y + height * 0.42, base.z + Math.sin(angle) * rs));
  }
  // The hollow underneath rises into the cone.
  const under = base.clone().add(new THREE.Vector3(0, height * 0.25, 0));
  const tip = vary(light, random, 1);
  const shoulders = shoulder.map(() => vary(light.clone().lerp(mid, 0.45), random, 1.2));
  const skirts = rim.map((_, i) => vary(i % 2 === 0 ? mid.clone().lerp(dark, 0.55) : mid, random, 1));
  const belly = shade(dark, 0.8);
  for (let i = 0; i < count; i++) {
    const j = (i + 1) % count;
    // The rings run counter-clockwise seen from above, so these face out
    // and up, and the underside down.
    if (shouldered) {
      shape.triangle(apex, shoulder[i], shoulder[j], tip, shoulders[i], shoulders[j]);
      shape.quad(shoulder[i], rim[i], rim[j], shoulder[j], shoulders[i], skirts[i], skirts[j], shoulders[j]);
    } else {
      shape.triangle(apex, rim[i], rim[j], tip, skirts[i], skirts[j]);
    }
    shape.triangle(under, rim[j], rim[i], belly, shade(dark, 0.9), shade(dark, 0.9));
  }
}

const UP = new THREE.Vector3(0, 1, 0);
const lit = new THREE.Vector3();

// A clump of grass springing from round `foot`, `spread` or a little more
// from it (round a post's foot, say): `blades` broad blades fanning out and
// arching over, `height` tall at most, darkest at the foot and lightest at
// the tip. Each blade is folded along its middle, so its two halves take the
// light differently, and drawn on both sides, lit more from above than its
// faces alone would be, so the grass stays bright from any side. `upright`
// 0..1 keeps them standing (a tall grass patch) rather than arching out.
export function addTuft(shape: Shape, foot: THREE.Vector3, blades: number, height: number, random: () => number, spread = 0, upright = 0, P: Palette = PALETTE): void {
  const tip = color(P.grassTip);
  const mid = color(P.grass);
  const low = color(P.grassFoot);
  for (let i = 0; i < blades; i++) {
    const angle = (2 * Math.PI * (i + random() * 0.8)) / blades;
    const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
    const side = new THREE.Vector3(-out.z, 0, out.x);
    const h = height * between(random, 0.5, 1);
    const lean = between(random, 0.25, 0.8) * (1 - upright * 0.75); // how far out the tip is, per m of height
    const w = between(random, 0.03, 0.045) * (1 + upright * 0.4);
    const base = foot.clone().addScaledVector(out, spread + between(random, 0, 0.03));
    base.y -= 0.01; // from just under the ground
    addBlade(shape, base, out, side, h, lean, w, vary(low, random, 1), vary(mid, random, 1.5), vary(tip, random, 1.5));
  }
}

// One blade of grass or a long leaf, from `base` toward `out` (and up):
// folded along its middle, drawn on both sides.
export function addBlade(
  shape: Shape,
  base: THREE.Vector3,
  out: THREE.Vector3,
  side: THREE.Vector3,
  height: number,
  lean: number,
  width: number,
  footColor: THREE.Color,
  midColor: THREE.Color,
  tipColor: THREE.Color,
): void {
  const fold = width * 0.3; // how far its middle stands out of its edges
  const knee = base.clone().addScaledVector(UP, height * 0.55).addScaledVector(out, lean * height * 0.3);
  const top = base.clone().addScaledVector(UP, height).addScaledVector(out, lean * height);
  const [bl, bm, br] = [-1, 0, 1].map((s) => base.clone().addScaledVector(side, (s * width) / 2).addScaledVector(out, s === 0 ? fold : 0));
  const [kl, km, kr] = [-1, 0, 1].map((s) => knee.clone().addScaledVector(side, s * width * 0.36).addScaledVector(out, s === 0 ? fold * 0.7 : 0));
  both(shape, bl, bm, km, footColor, footColor, midColor);
  both(shape, bl, km, kl, footColor, midColor, midColor);
  both(shape, bm, br, kr, footColor, footColor, midColor);
  both(shape, bm, kr, km, footColor, midColor, midColor);
  both(shape, kl, km, top, midColor, midColor, tipColor);
  both(shape, km, kr, top, midColor, midColor, tipColor);
}

// Both sides of a thin triangle, each lit as its face and a good deal from
// above, so thin things stay bright from any side.
export function both(shape: Shape, a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3, ca: THREE.Color, cb: THREE.Color, cc: THREE.Color, lift = 1): void {
  const n = lit.crossVectors(ab.subVectors(b, a), ac.subVectors(c, a)).normalize();
  shape.triangle(a, b, c, ca, cb, cc, n.clone().addScaledVector(UP, lift).normalize());
  shape.triangle(a, c, b, ca, cc, cb, n.negate().addScaledVector(UP, lift).normalize());
}

// A flower head facing `facing`: `petals` petals `length` long round a
// raised middle, each petal a pointed leaf shape, lighter at its tip. Drawn
// on both sides.
export function addFlower(shape: Shape, center: THREE.Vector3, facing: THREE.Vector3, petals: number, length: number, petal: THREE.Color, heart: THREE.Color, random: () => number, cup = 0.25): void {
  const n = facing.clone().normalize();
  const u = new THREE.Vector3().crossVectors(n, Math.abs(n.y) < 0.9 ? UP : new THREE.Vector3(1, 0, 0)).normalize();
  const v = new THREE.Vector3().crossVectors(n, u);
  const turn = random() * Math.PI * 2;
  const tipColor = shade(petal, 1.08);
  const footColor = shade(petal, 0.9);
  for (let i = 0; i < petals; i++) {
    const angle = turn + (2 * Math.PI * i) / petals;
    const dir = u.clone().multiplyScalar(Math.cos(angle)).addScaledVector(v, Math.sin(angle));
    const across = new THREE.Vector3().crossVectors(n, dir);
    const w = ((Math.PI * length) / petals) * 0.95;
    const foot = center.clone().addScaledVector(dir, length * 0.12);
    const mid = center.clone().addScaledVector(dir, length * 0.6).addScaledVector(n, length * cup * 0.4);
    const tip = center.clone().addScaledVector(dir, length).addScaledVector(n, length * cup);
    const left = mid.clone().addScaledVector(across, w / 2);
    const right = mid.clone().addScaledVector(across, -w / 2);
    both(shape, foot, left, tip, footColor, petal, tipColor, 0.4);
    both(shape, foot, tip, right, footColor, tipColor, petal, 0.4);
  }
  // The middle: a small raised dome.
  const r = length * 0.28;
  const top = center.clone().addScaledVector(n, r * 0.7);
  const ring = Array.from({ length: 6 }, (_, i) => {
    const angle = (2 * Math.PI * i) / 6;
    return center.clone().addScaledVector(u, Math.cos(angle) * r).addScaledVector(v, Math.sin(angle) * r).addScaledVector(n, r * 0.15);
  });
  for (let i = 0; i < 6; i++) shape.triangle(top, ring[i], ring[(i + 1) % 6], shade(heart, 1.1), heart, heart);
}

// Many copies of a template, for the environment: for each mesh in it, one
// InstancedMesh drawing it at every placement (the template's own placing
// of the mesh within it kept), each copy tinted by `tints` when given. The
// template's geometries and materials are used, not copied.
export function instances(template: THREE.Object3D, placements: readonly THREE.Matrix4[], tints?: readonly THREE.Color[]): THREE.Group {
  const group = new THREE.Group();
  template.updateMatrixWorld(true);
  const rootInverse = template.matrixWorld.clone().invert();
  template.traverse((object) => {
    if (!(object instanceof THREE.Mesh)) return;
    const local = rootInverse.clone().multiply(object.matrixWorld);
    const batch = new THREE.InstancedMesh(object.geometry, object.material, placements.length);
    const m = new THREE.Matrix4();
    placements.forEach((placement, i) => {
      batch.setMatrixAt(i, m.multiplyMatrices(placement, local));
      if (tints) batch.setColorAt(i, tints[i]);
    });
    batch.castShadow = false;
    batch.receiveShadow = object.receiveShadow;
    batch.computeBoundingSphere();
    group.add(batch);
  });
  return group;
}

// Every mesh under the given objects (their sprites and anything else left
// out), in the world, merged into one mesh for each named material (the
// first of each kind is kept), for scenery that never moves: one draw for
// all the wood and stone of the lake's piers, bridges, tower and ruins.
export function merge(objects: readonly THREE.Object3D[], shadows = false): THREE.Group {
  const kinds = new Map<string, { material: THREE.Material; geometries: THREE.BufferGeometry[] }>();
  for (const root of objects) {
    root.updateMatrixWorld(true);
    root.traverse((object) => {
      if (!(object instanceof THREE.Mesh) || object instanceof THREE.InstancedMesh) return;
      const material = object.material as THREE.Material;
      const kind = material.name || material.uuid;
      let entry = kinds.get(kind);
      if (!entry) kinds.set(kind, (entry = { material, geometries: [] }));
      entry.geometries.push((object.geometry as THREE.BufferGeometry).clone().applyMatrix4(object.matrixWorld));
    });
  }
  const group = new THREE.Group();
  for (const { material, geometries } of kinds.values()) {
    const merged = mergeGeometries(geometries);
    geometries.forEach((g) => g.dispose());
    if (merged) group.add(mesh(merged, material, shadows));
  }
  return group;
}

// A block of dressed stone (a ruin's, a step, a paving slab) or anything
// boxy and chamfered that is painted by the way its faces turn: `size`
// across, centred on `at`, turned by `turn` about y and tipped by `tip`
// about x and z, each corner straying by up to `rough`. Its eight corners are
// cut off by `chamfer`; `paint` colours each face from its outward normal.
export function addBlock(
  shape: Shape,
  at: THREE.Vector3,
  size: THREE.Vector3,
  paint: (normal: THREE.Vector3) => THREE.Color,
  random: () => number,
  { turn = 0, tip = [0, 0], chamfer = 0.04, rough = 0.01 }: { turn?: number; tip?: readonly [number, number]; chamfer?: number; rough?: number } = {},
): void {
  const matrix = new THREE.Matrix4().makeRotationFromEuler(new THREE.Euler(tip[0], turn, tip[1], 'YXZ')).setPosition(at);
  const [a, b, c] = [size.x / 2, size.y / 2, size.z / 2];
  const points: THREE.Vector3[] = [];
  for (const sx of [-1, 1]) {
    for (const sy of [-1, 1]) {
      for (const sz of [-1, 1]) {
        const jitter = () => (random() - 0.5) * 2 * rough;
        points.push(
          new THREE.Vector3(sx * (a - chamfer), sy * b, sz * c),
          new THREE.Vector3(sx * a, sy * (b - chamfer), sz * c),
          new THREE.Vector3(sx * a, sy * b, sz * (c - chamfer)),
        );
        for (let k = points.length - 3; k < points.length; k++) points[k].add(new THREE.Vector3(jitter(), jitter(), jitter()));
      }
    }
  }
  addHull(
    shape,
    points.map((p) => p.applyMatrix4(matrix)),
    (normal) => paint(normal),
  );
}

// How the reference paints dressed stone: light warm grey, lighter on top,
// darker underneath, each face its own shade; faces turned up past `moss`
// are mossy green, now and then.
export function stonePaint(random: () => number, moss = 0.8, mossy = 0.35, P: Palette = PALETTE): (normal: THREE.Vector3) => THREE.Color {
  const light = color(P.stone);
  const dark = color(P.stoneDark);
  const green = color(P.moss);
  return (normal) => {
    if (normal.y > moss && random() < mossy) return vary(green, random, 1.5);
    const base = normal.y >= 0 ? shade(light, 0.94 + normal.y * 0.1) : light.clone().lerp(dark, Math.min(1, -normal.y));
    return vary(base, random, 1.3);
  };
}

// A lantern hanging from `hook` (the underside of an arm): an iron hook, a
// pointed iron roof with a ring on top, a square iron frame round four
// panes of glowing yellow glass, brightest up the middle and at the foot
// where the flame inside is, and a base under them. Its iron goes in `iron`,
// its glass in `glass` (drawn unlit); returns the middle of its glass, for
// its glow.
export function addLantern(iron: Shape, glass: Shape, hook: THREE.Vector3, random: () => number): THREE.Vector3 {
  const up = new THREE.Vector3(0, 1, 0);
  const ironColor = color(PALETTE.iron);
  const irons: BarColors = { side: ironColor, groove: ironColor, end: ironColor, core: ironColor };
  const { x, z } = hook;
  const hookTop = hook.y;
  const roofTop = hookTop - 0.1;
  const glassTop = roofTop - 0.13;
  const glassBottom = glassTop - 0.24;
  const half = 0.085;
  const ring = (center: THREE.Vector3, radius: number, thick: number) => {
    const points = Array.from({ length: 9 }, (_, i) => {
      const angle = (2 * Math.PI * i) / 8;
      return center.clone().add(new THREE.Vector3(Math.cos(angle) * radius, Math.sin(angle) * radius, 0));
    });
    addTube(iron, points, points.map(() => thick), { sides: 4, random, paint: () => ironColor });
  };
  ring(new THREE.Vector3(x, hookTop - 0.035, z), 0.03, 0.007);
  ring(new THREE.Vector3(x, roofTop + 0.005, z), 0.025, 0.006);
  const turn45 = new THREE.Matrix4().makeRotationY(Math.PI / 4).setPosition(x, 0, z);
  addLathe(
    iron,
    [
      [0.15, glassTop],
      [0.14, glassTop + 0.02],
      [0.035, roofTop - 0.02],
      [0, roofTop],
    ],
    4,
    turn45,
    (r) => (r === 1 ? shade(ironColor, 1.25) : ironColor),
  );
  addLathe(
    iron,
    [
      [0, glassBottom - 0.07],
      [0.035, glassBottom - 0.05],
      [0.13, glassBottom - 0.02],
      [0.13, glassBottom + 0.005],
      [0, glassBottom + 0.005],
    ],
    4,
    turn45,
    () => ironColor,
  );
  for (const [cx, cz] of [
    [-half, -half],
    [half, -half],
    [half, half],
    [-half, half],
  ]) {
    addBar(iron, frame(new THREE.Vector3(x + cx, glassBottom, z + cz), up, new THREE.Vector3(0, 0, 1)), {
      length: glassTop - glassBottom,
      width: 0.02,
      height: 0.02,
      chamfer: 0.004,
      bevel: 0,
      segments: 1,
      colors: irons,
      random,
    });
  }
  const bright = color(0xfff3c4);
  const warm = color(PALETTE.glass);
  const deep = color(PALETTE.flame);
  const g = half - 0.004;
  const corners = [
    [-g, g],
    [g, g],
    [g, -g],
    [-g, -g],
  ];
  for (let k = 0; k < 4; k++) {
    // Each pane from corner k to k + 1, round counter-clockwise seen from
    // above, faces out.
    const [ax, az] = corners[k];
    const [bx, bz] = corners[(k + 1) % 4];
    const lowA = new THREE.Vector3(x + ax, glassBottom, z + az);
    const lowB = new THREE.Vector3(x + bx, glassBottom, z + bz);
    const lowM = lowA.clone().lerp(lowB, 0.5);
    const highA = lowA.clone().setY(glassTop);
    const highB = lowB.clone().setY(glassTop);
    const highM = lowM.clone().setY(glassTop);
    glass.quad(lowA, lowM, highM, highA, warm, bright, warm, deep);
    glass.quad(lowM, lowB, highB, highM, bright, warm, deep, warm);
  }
  return new THREE.Vector3(x, (glassTop + glassBottom) / 2, z);
}

// A flame (a torch's, a campfire's): tongues of fire in a cluster, the
// tallest in the middle and the rest leaning out round it, each pale yellow
// at its foot, orange up its middle and red at its tip, round a white-hot
// core, drawn unlit, with a glow round them. It flickers: update()
// stretches and sways each tongue on its own rhythm. `size` is its height
// in meters.
export class Flame extends THREE.Group {
  readonly tongues: THREE.Mesh[] = [];
  readonly glow: THREE.Sprite;
  private readonly phases: number[] = [];
  private readonly leans: [number, number][] = [];
  private readonly glowSize: number;
  private time = 0;

  constructor(size: number, random: () => number, tongues = 5, glow = true) {
    super();
    const material = glowing();
    const pale = color(0xfff3b4);
    const mid = color(PALETTE.flame);
    const tip = color(PALETTE.flameEdge);
    const white = color(0xffffff);
    for (let k = 0; k <= tongues; k++) {
      const core = k === tongues; // the last, small and white-hot, in the middle
      const main = k === 0;
      const h = size * (core ? 0.42 : main ? 1 : between(random, 0.5, 0.78));
      const r = size * (core ? 0.1 : main ? 0.22 : between(random, 0.11, 0.16));
      const profile: [number, number][] = [
        [r * 0.6, 0],
        [r, h * 0.18],
        [r * 0.82, h * 0.42],
        [r * 0.4, h * 0.72],
        [0, h],
      ];
      const tones = core ? [white, white, pale, pale] : [pale, pale.clone().lerp(mid, 0.5), mid, mid.clone().lerp(tip, 0.7)];
      const shape = new Shape();
      addLathe(shape, profile, 6, new THREE.Matrix4(), (ring) => tones[Math.min(ring, 3)]);
      const bottom = new THREE.Vector3(0, 0, 0);
      const foot = Array.from({ length: 6 }, (_, i) => {
        const angle = (2 * Math.PI * i) / 6;
        return new THREE.Vector3(r * 0.6 * Math.cos(angle), 0, -r * 0.6 * Math.sin(angle));
      });
      for (let i = 0; i < 6; i++) shape.triangle(bottom, foot[(i + 1) % 6], foot[i], tones[0]);
      const tongue = mesh(shape.geometry(), material, false);
      // Spun about its own axis first, then leant: so the spin doesn't
      // carry the lean round.
      tongue.rotation.order = 'ZXY';
      // The side tongues stand round the middle, leaning out.
      const angle = ((k - 1) / Math.max(1, tongues - 1)) * Math.PI * 2 + random() * 0.6;
      const out = main || core ? 0 : size * 0.13;
      const lean = main || core ? 0 : between(random, 0.18, 0.38);
      tongue.position.set(Math.cos(angle) * out, 0, Math.sin(angle) * out);
      this.leans.push([lean * Math.sin(angle), -lean * Math.cos(angle)]);
      this.phases.push(random() * Math.PI * 2);
      this.tongues.push(tongue);
      this.add(tongue);
    }
    this.glowSize = size * 3.2;
    this.glow = halo(PALETTE.flame, this.glowSize, 0.9);
    this.glow.position.y = size * 0.4;
    this.glow.visible = glow;
    this.add(this.glow);
    this.update(0);
  }

  update(delta: number): void {
    this.time += delta;
    const t = this.time;
    this.tongues.forEach((tongue, k) => {
      const p = this.phases[k];
      const [lx, lz] = this.leans[k];
      const stretch = 1 + 0.18 * Math.sin(t * (9 + k * 1.7) + p) + 0.09 * Math.sin(t * (17 + k) + p * 2);
      tongue.scale.set(1 - 0.08 * Math.sin(t * 11 + p), stretch, 1 + 0.08 * Math.sin(t * 13 + p));
      tongue.rotation.set(lx + 0.08 * Math.sin(t * 5 + p), t * (0.7 + 0.3 * k), lz + 0.08 * Math.sin(t * 6.3 + p));
    });
    const flicker = 1 + 0.1 * Math.sin(t * 10) + 0.06 * Math.sin(t * 23 + 1);
    this.glow.material.opacity = Math.min(1, 0.8 * flicker);
    this.glow.scale.setScalar(this.glowSize * (0.95 + 0.05 * flicker));
  }
}

function smooth01(a: number, b: number, x: number): number {
  const t = Math.max(0, Math.min(1, (x - a) / (b - a)));
  return t * t * (3 - 2 * t);
}

// Snow lying on an asset in winter, as the reference's winter draws it:
// every face of its lit meshes (and of anything under it) turned up to the
// sky past `from` is whitened, wholly past `to`: white where it faces
// straight up, blue-white on the slopes, with a face here and there left
// half bare. What glows, the water and the flag keep their colours. Its
// placing within `root` is allowed for, so what leans or lies on its side
// is snowed on by its own up.
const snowA = new THREE.Vector3();
const snowB = new THREE.Vector3();
const snowC = new THREE.Vector3();
const snowN = new THREE.Vector3();
const snowTint = new THREE.Color();
export function snowOn(root: THREE.Object3D, random: () => number, from = 0.3, to = 0.62): void {
  const snow = color(PALETTE.snow);
  const blue = color(PALETTE.snowShade);
  root.updateMatrixWorld(true);
  const rootInverse = root.matrixWorld.clone().invert();
  root.traverse((object) => {
    if (!(object instanceof THREE.Mesh) || object instanceof THREE.InstancedMesh) return;
    const material = object.material as THREE.Material;
    if (!(material instanceof THREE.MeshStandardMaterial) || material.name === 'flag') return;
    const geometry = object.geometry as THREE.BufferGeometry;
    const position = geometry.getAttribute('position');
    const tint = geometry.getAttribute('color');
    if (geometry.index || !tint) return;
    const normalMatrix = new THREE.Matrix3().getNormalMatrix(rootInverse.clone().multiply(object.matrixWorld));
    for (let i = 0; i + 2 < position.count; i += 3) {
      snowA.fromBufferAttribute(position, i);
      snowB.fromBufferAttribute(position, i + 1);
      snowC.fromBufferAttribute(position, i + 2);
      snowN.crossVectors(ab.subVectors(snowB, snowA), ac.subVectors(snowC, snowA)).applyMatrix3(normalMatrix).normalize();
      const cover = smooth01(from, to, snowN.y) * (random() < 0.08 ? 0.45 : 1);
      if (cover <= 0) continue;
      const white = blue.clone().lerp(snow, smooth01(from, 0.92, snowN.y));
      for (let k = 0; k < 3; k++) {
        snowTint.fromBufferAttribute(tint as THREE.BufferAttribute, i + k).lerp(white, cover);
        tint.setXYZ(i + k, snowTint.r, snowTint.g, snowTint.b);
      }
    }
    tint.needsUpdate = true;
  });
}

// An icicle hanging from `top`: a thin, four-sided spike `length` long,
// pale blue ice, whiter at its root.
export function addIcicle(shape: Shape, top: THREE.Vector3, length: number, radius: number, random: () => number): void {
  const ice = color(PALETTE.ice);
  const root = color(PALETTE.snow);
  const turn = random() * Math.PI;
  // From its tip up, so its faces face out.
  addLathe(
    shape,
    [
      [0, -length],
      [radius * 0.55, -length * 0.45],
      [radius, 0],
    ],
    4,
    new THREE.Matrix4().makeRotationY(turn).setPosition(top),
    (r) => (r === 1 ? vary(root, random, 0.5) : vary(ice, random, 0.8)),
  );
  // Closed at the top, facing up.
  const ring = Array.from({ length: 4 }, (_, i) => {
    const a = turn + (2 * Math.PI * i) / 4;
    return new THREE.Vector3(top.x + Math.cos(a) * radius, top.y, top.z - Math.sin(a) * radius);
  });
  shape.triangle(ring[0], ring[1], ring[2], root);
  shape.triangle(ring[0], ring[2], ring[3], root);
}

// A berry, or anything small and round (a bead of dew, an ember): eight
// faces round `at`, `radius` across, lit from above.
export function addBerry(shape: Shape, at: THREE.Vector3, radius: number, tone: THREE.Color): void {
  const top = at.clone().add(new THREE.Vector3(0, radius, 0));
  const bottom = at.clone().add(new THREE.Vector3(0, -radius, 0));
  const ring = [
    new THREE.Vector3(radius, 0, 0),
    new THREE.Vector3(0, 0, -radius),
    new THREE.Vector3(-radius, 0, 0),
    new THREE.Vector3(0, 0, radius),
  ].map((p) => p.add(at));
  const lit = shade(tone, 1.25);
  const dim = shade(tone, 0.75);
  for (let i = 0; i < 4; i++) {
    const j = (i + 1) % 4;
    shape.triangle(top, ring[i], ring[j], lit, tone, tone);
    shape.triangle(bottom, ring[j], ring[i], dim, tone, tone);
  }
}

// The colours of autumn's fallen leaves, as the reference scatters them:
// orange, red, gold and rust.
export const FALLEN_LEAVES = [0xf5872a, 0xe5542f, 0xf7c43e, 0xd0482a, 0xffa94d, 0xb8431f] as const;

// Leaves fallen round `at` in autumn, `count` of them within `radius`, on
// ground at `at`'s height, each its own colour of FALLEN_LEAVES.
export function addFallenLeaves(shape: Shape, at: THREE.Vector3, radius: number, count: number, random: () => number): void {
  for (let k = 0; k < count; k++) {
    const a = random() * Math.PI * 2;
    const d = radius * Math.sqrt(random());
    const tone = vary(color(FALLEN_LEAVES[Math.floor(random() * FALLEN_LEAVES.length)]), random, 1.2);
    addLeaf(shape, new THREE.Vector3(at.x + Math.cos(a) * d, at.y, at.z + Math.sin(a) * d), between(random, 0.09, 0.14), random() * Math.PI * 2, tone, random, 0.012 + k * 0.0004);
  }
}

// A mushroom standing at `at`: a pale stem `height` tall, and a domed cap
// `cap` across, its gills pale underneath; a red cap is spotted white, as
// the reference's are, a tan one plain. It leans by `lean` (radians) toward
// `turn`.
export function addMushroom(shape: Shape, at: THREE.Vector3, height: number, cap: number, kind: 'red' | 'tan', random: () => number, turn = 0, lean = 0): void {
  const matrix = new THREE.Matrix4().makeRotationFromEuler(new THREE.Euler(0, turn, lean, 'YXZ')).setPosition(at);
  const stem = color(PALETTE.mushroomStem);
  const top = color(kind === 'red' ? PALETTE.capRed : PALETTE.capTan);
  const gills = shade(stem, 0.86);
  const r = cap * 0.2;
  addLathe(
    shape,
    [
      [r * 1.15, -0.01],
      [r, height * 0.5],
      [r * 0.85, height],
    ],
    6,
    matrix,
    (_, i) => shade(stem, i % 2 ? 1 : 0.9),
  );
  const rim = cap / 2;
  addLathe(
    shape,
    [
      [r * 0.8, height],
      [rim, height + rim * 0.15],
      [rim * 0.86, height + rim * 0.5],
      [rim * 0.48, height + rim * 0.78],
      [0, height + rim * 0.86],
    ],
    7,
    matrix,
    (ring, i) => (ring === 0 ? gills : vary(shade(top, ring === 3 ? 1.12 : i % 2 ? 1 : 0.92), random, 0.6)),
  );
  if (kind === 'red') {
    const white = color(0xfbf6ee);
    for (let k = 0; k < 5; k++) {
      const a = random() * Math.PI * 2;
      const f = between(random, 0.25, 0.7);
      const y = height + rim * (0.86 - 0.7 * f * f);
      addBerry(shape, new THREE.Vector3(Math.cos(a) * rim * f, y, -Math.sin(a) * rim * f).applyMatrix4(matrix), rim * 0.12, white);
    }
  }
}

// A leaf lying flat at `at` (on the ground, a log, the water), `size`
// across, turned by `turn`: five pointed lobes round its middle, as a
// maple's, its lobes in `tone` and its middle a shade darker; facing up.
export function addLeaf(shape: Shape, at: THREE.Vector3, size: number, turn: number, tone: THREE.Color, random: () => number, lift = 0.012): void {
  const middle = at.clone().add(new THREE.Vector3(0, lift + size * 0.04, 0));
  const outline: THREE.Vector3[] = [];
  for (let i = 0; i < 10; i++) {
    // Counter-clockwise seen from above (decreasing angle): lobe tips and
    // the notches between them; the stalk's lobe (i = 0) shortest.
    const a = turn - (2 * Math.PI * i) / 10;
    const tip = i % 2 === 0;
    const r = (size / 2) * (tip ? (i === 0 ? 0.55 : between(random, 0.85, 1)) : 0.42);
    outline.push(new THREE.Vector3(at.x + Math.cos(a) * r, at.y + lift, at.z + Math.sin(a) * r));
  }
  const core = shade(tone, 0.82);
  for (let i = 0; i < 10; i++) shape.triangle(middle, outline[i], outline[(i + 1) % 10], core, tone, tone);
}

// Bakes a light into a geometry's corner colours, for a place the sun
// doesn't reach (inside a cave): each corner's colour times `ambient` plus
// `light` as much as the corner faces `at`, fading with distance past
// `reach`. Drawn with glowing(), it looks lit by that light alone.
export function prelight(geometry: THREE.BufferGeometry, at: THREE.Vector3, light: THREE.Color, reach: number, ambient: THREE.Color): void {
  const position = geometry.getAttribute('position');
  const normal = geometry.getAttribute('normal');
  const tint = geometry.getAttribute('color');
  const p = new THREE.Vector3();
  const n = new THREE.Vector3();
  const c = new THREE.Color();
  for (let i = 0; i < position.count; i++) {
    p.fromBufferAttribute(position, i);
    n.fromBufferAttribute(normal, i);
    const toward = at.clone().sub(p);
    const distance = toward.length();
    const facing = Math.max(0, n.dot(toward.divideScalar(Math.max(distance, 1e-6)))) * 0.8 + 0.2;
    const fall = 1 / (1 + (distance / reach) ** 2);
    c.fromBufferAttribute(tint as THREE.BufferAttribute, i);
    const r = c.r * (ambient.r + light.r * facing * fall);
    const g = c.g * (ambient.g + light.g * facing * fall);
    const b = c.b * (ambient.b + light.b * facing * fall);
    tint.setXYZ(i, r, g, b);
  }
  tint.needsUpdate = true;
}
