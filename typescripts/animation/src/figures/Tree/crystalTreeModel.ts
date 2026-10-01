import * as THREE from 'three';

// The user's crystal tree (CrystalTree.md, its "# Shape Reference": the
// three.js page "Crystal Tree · Three.js BufferGeometry"), built as its page
// builds it: its numbers, its shapes and its draws of the random generator
// kept as the user wrote them, in the same order, so a seed grows the same
// tree as on the page (the page's own is 7). Only the colours are taken out,
// as the roles of `CrystalColors`: MODEL_COLORS are the page's own, and the
// figure (CrystalTree.ts) fills them from the theme.
//
// Units are meters (a grass blade is 12-25 cm, a rock 16-32 cm across), y is
// up, and the origin is on the ground in the middle of the ground disk, by
// the foot of the trunk. The page's three r128 shows colours as given, so
// every colour here is worked in display (sRGB) values, as the page works
// them, and turned linear only as it goes into a geometry's vertex colours.
//
// Where the page makes one mesh of all its earth (the trunk, roots and
// branches, then the ground) and one of all its crystals (the tree's, then
// the ground's), this makes two of each, the same triangles, so the tree and
// its ground are parts of their own.
//
// Standing on other ground (`CrystalGround`, the forest lake's island,
// task 21): its ground crystals stand on that ground, and its roots run on
// out over it. Those roots are drawn after all of the page's draws, so the
// tree above them is still the page's.

type V3 = THREE.Vector3;
const V = THREE.Vector3;
const C = THREE.Color;

// The page's colours, by what they colour.
export interface CrystalColors {
  barkLow: THREE.Color; // the bark at the foot of the trunk, and the roots
  barkHigh: THREE.Color; // toward which the bark lightens up the trunk and out along the branches
  crystalBlue: THREE.Color; // a crystal cluster's colour at one end of its hue (the page's t = 0)
  crystalViolet: THREE.Color; // and at the other (t = 1)
  tipBlue: THREE.Color; // the pale tips of a blue cluster's shards
  tipViolet: THREE.Color; // and a violet one's
  lilac: THREE.Color; // toward which each shard of the tree's clusters is tinted a little
  stone: THREE.Color; // the rocks
  grassLow: THREE.Color; // the ground disk's faces, each between these two
  grassHigh: THREE.Color;
  soil: THREE.Color; // the disk's edge
  blade: THREE.Color; // the tufts of grass
}

export const MODEL_COLORS: CrystalColors = {
  barkLow: new C(0.45, 0.24, 0.1),
  barkHigh: new C(0.85, 0.5, 0.22),
  crystalBlue: new C(0.28, 0.52, 1.0),
  crystalViolet: new C(0.68, 0.38, 1.0),
  tipBlue: new C(0.66, 0.8, 0.98),
  tipViolet: new C(0.76, 0.8, 0.98),
  lilac: new C(0.7, 0.45, 1),
  stone: new C(0.33, 0.33, 0.36),
  grassLow: new C(0.45, 0.62, 0.18),
  grassHigh: new C(0.68, 0.74, 0.25),
  soil: new C(0.28, 0.2, 0.12),
  blade: new C(0.35, 0.6, 0.15),
};

// A crystal hanging on a string: its geometry is built round its pivot, the
// top of the string, and hangs below it.
export interface Pendant {
  at: V3; // the pivot, in the tree's space
  length: number; // m of string, down to the top of the crystal
  geometry: THREE.BufferGeometry;
  phase: number; // its swing's, rad
  speed: number; // its swing's, rad a second
}

// Other ground for the tree to stand on, in the model's own units (the
// page's meters), round the foot of its trunk.
export interface CrystalGround {
  // The ground's height at a point; NaN where nothing may stand (a ground
  // crystal there is left out).
  at(x: number, z: number): number;
  // How far out its roots run, following the ground (down over an edge), at
  // most; each stops where the ground falls below `low`.
  reach: number;
  low: number;
}

export interface CrystalTreeGeometry {
  wood: THREE.BufferGeometry; // the trunk, roots and branches
  crystals: THREE.BufferGeometry; // the clusters on the branches and on top of the crown
  ground: THREE.BufferGeometry; // the grassy disk, its soil edge, the rocks and the tufts of grass
  groundCrystals: THREE.BufferGeometry; // the crystals growing from the ground
  pendants: Pendant[];
  sparkles: Float32Array; // x, y and z of each of the 260 sparkles
  triangles: number; // as the page counts them: every mesh but the sparkles
  // Each root's way out from the trunk (radians from +x toward +z) and how
  // far it runs.
  roots: { angle: number; reach: number }[];
}

// The page's seeded random generator (mulberry32): the same seed always gives
// the same numbers, in 0..1.
function mulberry32(seed: number): () => number {
  return function () {
    seed |= 0;
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// A display value turned linear, as three.js keeps colours.
function linear(c: number): number {
  return c < 0.04045 ? c * 0.0773993808 : Math.pow(c * 0.9478672986 + 0.0521327014, 2.4);
}

// Collects triangles for one BufferGeometry, not indexed, so every face gets
// a flat normal of its own (the page's Builder).
class Builder {
  private readonly p: number[] = [];
  private readonly c: number[] = [];

  get triangles(): number {
    return this.p.length / 9;
  }

  tri(a: V3, b: V3, c: V3, ca: THREE.Color, cb?: THREE.Color, cc?: THREE.Color): void {
    cb = cb || ca;
    cc = cc || ca;
    this.p.push(a.x, a.y, a.z, b.x, b.y, b.z, c.x, c.y, c.z);
    this.c.push(ca.r, ca.g, ca.b, cb.r, cb.g, cb.b, cc.r, cc.g, cc.b);
  }

  quad(a: V3, b: V3, c: V3, d: V3, col: THREE.Color): void {
    this.tri(a, b, c, col);
    this.tri(a, c, d, col);
  }

  build(): THREE.BufferGeometry {
    const g = new THREE.BufferGeometry();
    g.setAttribute('position', new THREE.Float32BufferAttribute(this.p, 3));
    g.setAttribute('color', new THREE.Float32BufferAttribute(this.c.map(linear), 3));
    g.computeVertexNormals(); // not indexed: flat facets
    g.computeBoundingSphere();
    return g;
  }
}

// Builds the tree from `seed`, in `colors` (display values), on its own
// ground disk or on `on`.
export function buildCrystalTree(seed: number, colors: CrystalColors, on?: CrystalGround): CrystalTreeGeometry {
  const R = mulberry32(seed);
  const rand = (a: number, b: number) => a + (b - a) * R();

  // Orthonormal basis round a direction.
  function basis(dir: V3) {
    const d = dir.clone().normalize();
    const ref = Math.abs(d.y) < 0.9 ? new V(0, 1, 0) : new V(1, 0, 0);
    const u = new V().crossVectors(d, ref).normalize();
    const v = new V().crossVectors(d, u).normalize();
    return { d, u, v };
  }
  const add = (p: V3, dir: V3, s: number) => p.clone().add(dir.clone().multiplyScalar(s));
  const jitter = (col: THREE.Color, a: number, b: number) => new C(col.r, col.g, col.b).multiplyScalar(rand(a, b));

  // A low poly tube along a list of points (the trunk, a branch, a root),
  // its rings twisting a little, closed by a point.
  function addTube(b: Builder, pts: V3[], radii: number[], sides: number, colorAt: (t: number) => THREE.Color): void {
    const rings: V3[][] = [];
    let prevU: V3 | null = null;
    let lastT: V3 | null = null;
    for (let i = 0; i < pts.length; i++) {
      const t = (i < pts.length - 1 ? pts[i + 1].clone().sub(pts[i]) : pts[i].clone().sub(pts[i - 1])).normalize();
      lastT = t;
      const u: V3 = prevU ? prevU.clone().sub(t.clone().multiplyScalar(prevU.dot(t))).normalize() : basis(t).u;
      prevU = u;
      const v = new V().crossVectors(t, u).normalize();
      const ring: V3[] = [];
      for (let s = 0; s < sides; s++) {
        const a = (s / sides) * Math.PI * 2 + i * 0.35; // a slight twist
        const r = radii[i] * rand(0.88, 1.12);
        ring.push(add(add(pts[i], u, Math.cos(a) * r), v, Math.sin(a) * r));
      }
      rings.push(ring);
    }
    for (let i = 0; i < rings.length - 1; i++) {
      for (let s = 0; s < sides; s++) {
        const n = (s + 1) % sides;
        const col = jitter(colorAt(i / (rings.length - 1)), 0.78, 1.15);
        b.quad(rings[i][s], rings[i][n], rings[i + 1][n], rings[i + 1][s], col);
      }
    }
    // A pointed end cap.
    const last = rings[rings.length - 1];
    const tip = add(pts[pts.length - 1], lastT!, radii[radii.length - 1] * 1.5);
    for (let s = 0; s < sides; s++) b.tri(last[s], last[(s + 1) % sides], tip, jitter(colorAt(1), 0.8, 1.1));
  }

  // One crystal shard: a stretched five-sided bipyramid.
  function addShard(b: Builder, origin: V3, dir: V3, L: number, w: number, cBase: THREE.Color, cTip: THREE.Color, cDeep: THREE.Color): void {
    const { d, u, v } = basis(dir);
    const sides = 5;
    const ringC = add(origin, d, L * 0.28);
    const tip = add(origin, d, L);
    const back = add(origin, d, -L * 0.1);
    const rot = R() * Math.PI * 2;
    const ring: V3[] = [];
    for (let s = 0; s < sides; s++) {
      const a = rot + (s / sides) * Math.PI * 2;
      const r = w * rand(0.8, 1.2);
      ring.push(add(add(ringC, u, Math.cos(a) * r), v, Math.sin(a) * r * 0.75));
    }
    for (let s = 0; s < sides; s++) {
      const n = (s + 1) % sides;
      const shade = rand(0.85, 1.15);
      const cb = cBase.clone().multiplyScalar(shade);
      b.tri(ring[s], ring[n], tip, cb, cb, cTip);
      b.tri(ring[n], ring[s], back, cb, cb, cDeep);
    }
  }

  // A cluster's colours: somewhere between blue and violet.
  function crystalPalette() {
    const t = R(); // 0 blue, 1 violet
    const base = colors.crystalBlue.clone().lerp(colors.crystalViolet, t);
    const tip = colors.tipBlue.clone().lerp(colors.tipViolet, t);
    const deep = base.clone().multiplyScalar(0.45);
    return { base, tip, deep };
  }

  // A star-like bunch of shards: one "leaf".
  function addCluster(b: Builder, center: V3, size: number, outward: V3): void {
    const pal = crystalPalette();
    const n = Math.floor(rand(10, 15));
    for (let i = 0; i < n; i++) {
      const d = new V(rand(-1, 1), rand(-0.55, 1), rand(-1, 1)).normalize().add(outward.clone().multiplyScalar(0.55)).normalize();
      const L = size * rand(0.55, 1.1);
      const base = pal.base.clone().lerp(colors.lilac, rand(0, 0.25));
      addShard(b, add(center, d, -0.04), d, L, L * rand(0.16, 0.24), base, pal.tip, pal.deep);
    }
  }

  // A hanging crystal, built round its own pivot so it can swing.
  function pendantGeometry(len: number): { geometry: THREE.BufferGeometry; triangles: number } {
    const b = new Builder();
    const pal = crystalPalette();
    const sides = 6;
    const r = rand(0.07, 0.1);
    const mid = -len - 0.12;
    const top = new V(0, -len, 0);
    const bottom = new V(0, mid - rand(0.28, 0.4), 0);
    const ring: V3[] = [];
    for (let s = 0; s < sides; s++) {
      const a = (s / sides) * Math.PI * 2;
      ring.push(new V(Math.cos(a) * r, mid, Math.sin(a) * r));
    }
    for (let s = 0; s < sides; s++) {
      const n = (s + 1) % sides;
      const cb = pal.base.clone().multiplyScalar(rand(0.85, 1.15));
      b.tri(ring[n], ring[s], top, cb, cb, pal.tip);
      b.tri(ring[s], ring[n], bottom, cb, cb, pal.tip);
    }
    return { geometry: b.build(), triangles: b.triangles };
  }

  // A low poly rock.
  function addRock(b: Builder, pos: V3, size: number): void {
    const sides = 7;
    const gray = colors.stone;
    const r0: V3[] = [];
    const r1: V3[] = [];
    for (let s = 0; s < sides; s++) {
      const a = (s / sides) * Math.PI * 2 + rand(-0.2, 0.2);
      r0.push(new V(pos.x + Math.cos(a) * size * rand(0.85, 1.1), pos.y, pos.z + Math.sin(a) * size * rand(0.85, 1.1)));
      r1.push(new V(pos.x + Math.cos(a + 0.3) * size * rand(0.55, 0.8), pos.y + size * rand(0.5, 0.75), pos.z + Math.sin(a + 0.3) * size * rand(0.55, 0.8)));
    }
    const top = new V(pos.x + rand(-0.1, 0.1) * size, pos.y + size * rand(0.8, 1.0), pos.z);
    for (let s = 0; s < sides; s++) {
      const n = (s + 1) % sides;
      b.quad(r0[s], r0[n], r1[n], r1[s], jitter(gray, 0.8, 1.25));
      b.tri(r1[s], r1[n], top, jitter(gray, 0.95, 1.4));
    }
  }

  // The grassy ground disk, with its soil edge.
  function addGround(b: Builder): void {
    const segs = 30;
    const radii = [0, 1.1, 2.2, 3.1];
    const rings: V3[][] = [];
    radii.forEach((r, k) => {
      const ring: V3[] = [];
      for (let s = 0; s < segs; s++) {
        const a = (s / segs) * Math.PI * 2;
        const rr = k === 0 ? 0 : r * rand(0.96, 1.04);
        ring.push(new V(Math.cos(a) * rr, k === radii.length - 1 ? 0 : rand(0.0, 0.06), Math.sin(a) * rr * 0.92));
      }
      rings.push(ring);
    });
    const g1 = colors.grassLow;
    const g2 = colors.grassHigh;
    for (let k = 0; k < rings.length - 1; k++) {
      for (let s = 0; s < segs; s++) {
        const n = (s + 1) % segs;
        const col = g1.clone().lerp(g2, R()).multiplyScalar(rand(0.85, 1.1));
        if (k === 0) b.tri(rings[0][0], rings[1][n], rings[1][s], col);
        else b.quad(rings[k][s], rings[k][n], rings[k + 1][n], rings[k + 1][s], col);
      }
    }
    // The soil edge.
    const outer = rings[rings.length - 1];
    const soil = colors.soil;
    for (let s = 0; s < segs; s++) {
      const n = (s + 1) % segs;
      const lo = (p: V3) => new V(p.x * 0.97, -0.16, p.z * 0.97);
      b.quad(outer[s], lo(outer[s]), lo(outer[n]), outer[n], jitter(soil, 0.8, 1.2));
    }
  }

  // A tuft of five blades of grass.
  function addGrass(b: Builder, pos: V3): void {
    const green = colors.blade;
    for (let i = 0; i < 5; i++) {
      const a = rand(0, Math.PI * 2);
      const h = rand(0.12, 0.25);
      const base1 = new V(pos.x + Math.cos(a) * 0.03, pos.y, pos.z + Math.sin(a) * 0.03);
      const base2 = new V(pos.x - Math.cos(a) * 0.03, pos.y, pos.z - Math.sin(a) * 0.03);
      const tip = new V(pos.x + rand(-0.08, 0.08), pos.y + h, pos.z + rand(-0.08, 0.08));
      b.tri(base1, base2, tip, jitter(green, 0.8, 1.3));
    }
  }

  // The whole tree (the page's buildTree).
  const wood = new Builder();
  const ground = new Builder();
  const crystals = new Builder();
  const groundCrystals = new Builder();
  const clusterSpots: { pos: V3; out: V3; size: number }[] = [];

  const bark = (t: number) => colors.barkLow.clone().lerp(colors.barkHigh, 0.35 + 0.65 * t);

  // The trunk: an S-curve.
  const trunkPts: V3[] = [];
  const trunkR: number[] = [];
  const N = 11;
  for (let i = 0; i < N; i++) {
    const t = i / (N - 1);
    trunkPts.push(new V(0.35 * Math.sin(t * 3.2 + 0.3) - 0.1, t * 3.0, 0.25 * Math.sin(t * 2.4 + 1.2)));
    trunkR.push(0.24 + 0.4 * Math.pow(1 - t, 4) - 0.05 * t);
  }
  addTube(wood, trunkPts, trunkR, 8, bark);
  const trunkTop = trunkPts[N - 1];

  // The roots.
  const roots: { angle: number; reach: number }[] = [];
  for (let i = 0; i < 5; i++) {
    const a = (i / 5) * Math.PI * 2 + rand(-0.3, 0.3);
    roots.push({ angle: a, reach: 0.85 });
    const o = trunkPts[1].clone();
    const pts = [o, new V(o.x + Math.cos(a) * 0.45, 0.12, o.z + Math.sin(a) * 0.45), new V(o.x + Math.cos(a) * 0.85, 0.02, o.z + Math.sin(a) * 0.85)];
    addTube(wood, pts, [0.2, 0.1, 0.04], 5, () => colors.barkLow);
  }

  // The branches, forking twice (recursive).
  function branch(start: V3, dir: V3, len: number, r0: number, depth: number): void {
    const segs = 5;
    const pts: V3[] = [];
    const radii: number[] = [];
    const p = start.clone();
    const d = dir.clone().normalize();
    for (let i = 0; i <= segs; i++) {
      pts.push(p.clone());
      radii.push(r0 * (1 - (0.55 * i) / segs));
      d.add(new V(rand(-0.22, 0.22), rand(-0.08, 0.14), rand(-0.22, 0.22))).normalize();
      p.add(d.clone().multiplyScalar(len / segs));
    }
    addTube(wood, pts, radii, depth === 0 ? 6 : 5, bark);
    const end = pts[segs];
    const outward = new V(end.x, 0.4, end.z).normalize();
    if (depth < 2) {
      for (let k = 0; k < 2; k++) {
        const side = new V(-d.z, 0, d.x).multiplyScalar(k === 0 ? 0.8 : -0.8);
        const nd = d.clone().add(side).add(new V(0, 0.35, 0)).normalize();
        const from = pts[segs - k];
        branch(from, nd, len * 0.62, radii[segs - k] * 0.95, depth + 1);
      }
      if (depth === 1) clusterSpots.push({ pos: end, out: outward, size: 0.7 });
    } else {
      clusterSpots.push({ pos: end, out: outward, size: rand(0.7, 0.95) });
    }
  }
  const mains = 5;
  for (let i = 0; i < mains; i++) {
    const a = (i / mains) * Math.PI * 2 + rand(-0.3, 0.3);
    const dir = new V(Math.cos(a) * 0.9, rand(0.55, 0.85), Math.sin(a) * 0.9);
    const startIdx = i % 2 === 0 ? N - 1 : N - 3;
    branch(trunkPts[startIdx], dir, rand(1.3, 1.65), trunkR[startIdx] * 0.8, 0);
  }
  // A cluster on top of the crown.
  clusterSpots.push({ pos: add(trunkTop, new V(0, 1, 0), 1.6), out: new V(0, 1, 0), size: 0.9 });

  clusterSpots.forEach((s) => addCluster(crystals, s.pos, s.size, s.out));

  // The ground, its rocks, crystals and grass.
  addGround(ground);
  for (let i = 0; i < 6; i++) {
    const a = rand(0, Math.PI * 2);
    const r = rand(0.8, 1.9);
    addRock(ground, new V(Math.cos(a) * r, 0.02, Math.sin(a) * r), rand(0.16, 0.32));
  }
  const offGround = new Builder(); // where `on` has no ground: drawn as the page draws them, then left out
  for (let i = 0; i < 8; i++) {
    const a = (i / 8) * Math.PI * 2 + rand(-0.3, 0.3);
    const r = rand(0.9, 2.5);
    const c = new V(Math.cos(a) * r, 0.02, Math.sin(a) * r);
    const y = on ? on.at(c.x, c.z) : 0.02;
    if (on && !Number.isNaN(y)) c.y = y;
    const pal = crystalPalette();
    const n = Math.floor(rand(3, 6));
    for (let k = 0; k < n; k++) {
      const d = new V(rand(-0.6, 0.6), 1, rand(-0.6, 0.6)).normalize();
      const L = rand(0.22, 0.5);
      addShard(Number.isNaN(y) ? offGround : groundCrystals, c, d, L, L * 0.22, pal.base, pal.tip, pal.deep);
    }
  }
  for (let i = 0; i < 14; i++) {
    const a = rand(0, Math.PI * 2);
    const r = rand(0.6, 2.8);
    addGrass(ground, new V(Math.cos(a) * r, 0.02, Math.sin(a) * r));
  }

  // Crystals hanging from the outer clusters, two of every three.
  const pendants: Pendant[] = [];
  let pendantTriangles = 0;
  const outer = clusterSpots.filter((s) => Math.hypot(s.pos.x, s.pos.z) > 1.1);
  outer.forEach((s, i) => {
    if (i % 3 === 2) return;
    const length = rand(0.55, 1.3);
    const at = s.pos.clone().add(new V(rand(-0.2, 0.2), -0.25, rand(-0.2, 0.2)));
    const gem = pendantGeometry(length);
    pendantTriangles += gem.triangles;
    pendants.push({ at, length, geometry: gem.geometry, phase: rand(0, Math.PI * 2), speed: rand(0.8, 1.4) });
  });

  // Sparkles round it all.
  const count = 260;
  const sparkles = new Float32Array(count * 3);
  for (let i = 0; i < count; i++) {
    const a = rand(0, Math.PI * 2);
    const r = Math.sqrt(R()) * 3.2;
    sparkles[i * 3] = Math.cos(a) * r;
    sparkles[i * 3 + 1] = rand(0.3, 5.8);
    sparkles[i * 3 + 2] = Math.sin(a) * r;
  }

  // On other ground, each root runs on from the trunk out over it, as
  // thick as the page's where it leaves the trunk and tapering, half sunk in
  // the ground, down over an edge, until the ground falls below `on.low`.
  if (on) {
    const STEP = 0.2;
    for (const root of roots) {
      const o = trunkPts[1].clone();
      const pts = [o];
      const radii = [0.2];
      const out = new V(Math.cos(root.angle), 0, Math.sin(root.angle));
      const side = new V(-out.z, 0, out.x);
      const wander = rand(0, Math.PI * 2);
      for (let d = 0.45; d <= on.reach; d += STEP) {
        const r = 0.2 - 0.165 * Math.min(1, d / on.reach) ** 0.7;
        const p = new V(o.x, 0, o.z).add(out.clone().multiplyScalar(d)).add(side.clone().multiplyScalar(0.12 * Math.sin(wander + d * 2.3) * Math.min(1, d)));
        const g = on.at(p.x, p.z);
        if (Number.isNaN(g) || g < on.low) break;
        p.y = g + r * 0.3;
        pts.push(p);
        radii.push(r);
        root.reach = d;
      }
      if (pts.length > 2) addTube(wood, pts, radii, 5, (t) => colors.barkLow.clone().lerp(colors.barkHigh, 0.25 * (1 - t)));
    }
  }

  return {
    wood: wood.build(),
    crystals: crystals.build(),
    ground: ground.build(),
    groundCrystals: groundCrystals.build(),
    pendants,
    sparkles,
    triangles: wood.triangles + ground.triangles + crystals.triangles + groundCrystals.triangles + pendantTriangles,
    roots,
  };
}
