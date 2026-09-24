import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { defaultTheme, surface } from '../../theme';
import { clumpGeometry, type Clump } from '../Tree/parts';
import { between, fit, mesh, randomUnit, seededRandom, sizeOf, type ObjectOptions, type Size } from './parts';

// A puffy fair-weather cloud (cumulus): a heap of lumpy, rounded clumps on a
// flat base, narrowing as it rises into a few towers, with smaller billows
// all over its top and sides like a cauliflower. It is far smaller than a
// real cloud (those are hundreds of meters across), so that it fits the
// scenes: about 8 m wide, 4 m tall and 5 m deep. It is the theme's light
// neutral (white), shaded greyer toward its flat underside (vertex colours,
// as the trees' crowns are).
//
// Units are meters, y is up, the front faces +z and the origin is at the
// middle of its flat base. It does not float by itself: whoever shows it
// lifts it (the preview does). It is built from a seed and sized to a width,
// height and depth (see ObjectOptions in parts.ts), so it is the same every
// time. The spec, Cloud.md, is a draft: it names no behaviour, so the cloud
// stays still.

const SIZE: Size = { width: 8, height: 4, depth: 5 };
const SEED = 2;

// The heap is built in tiers, one above the other, each a ring of big clumps
// round an ellipse. The lowest ring runs round the edge of the footprint and
// the rings shrink as they rise, to a few towers at the top. A clump's
// radius is `share` of the smallest of the half-width, half-depth and height,
// and `top` of that in the top tier; it is `squash` times as tall as it is
// wide, flatter at the bottom.
const CLUMP = { share: [0.4, 0.52] as const, top: 0.7, squash: [0.7, 0.9] as const };
const LIFT = 0.45; // the lowest tier's middles, times their half-height above the base: they are cut off flat below
const STEP = 0.75; // m between tiers, times the clump radius
const RING = 1.25; // m between clumps round a ring, times their radius
const TOP_RING = 0.25; // the top tier's ring, as a share of the lowest's
const TOWERS = 2; // clumps at least in the top tier
const WANDER = 0.12; // clumps stray from their ring by up to this share of their radius, and differ in size by as much
// Billows: small clumps on the surface of the big ones, sunk `sink` of their
// radius into it, on the top and sides only (their direction from the clump
// under them at least `rise` up). About `each` of them for every big clump.
const BILLOWS = { each: 2, radius: [0.28, 0.5] as const, sink: 0.35, rise: -0.15 };
// Subdivisions of a clump's sphere, by its radius: 720 faces from 1.2 m, 500
// from 0.6 m, 320 from 0.3 m and 180 below.
const DETAIL: [radius: number, detail: number][] = [
  [1.2, 5],
  [0.6, 4],
  [0.3, 3],
  [0, 2],
];
const FLOOR = 0.02; // m: each clump is cut off at its own height up to this, so the cut faces never lie in one plane

// Shades, times the light neutral.
const UNDERSIDE = 0.8; // a face turned straight down, times one turned up
const BASE_SHADE = 0.72; // the base, times the top
const TINT = [0.95, 1.03] as const; // each clump a little lighter or darker

export class Cloud extends THREE.Group {
  static readonly SIZE: Size = SIZE;
  readonly body: THREE.Mesh;

  constructor(options: ObjectOptions = {}) {
    super();
    this.name = 'cloud';
    const theme = options.theme ?? defaultTheme;
    const random = seededRandom(options.seed ?? SEED);
    const size = sizeOf(options, SIZE);
    const geo = cloudGeometry(size, random);
    fit([geo], size);

    // White in the theme's finish, times the vertex colours. It casts a
    // shadow but takes none of its own: its clumps shadowing each other drew
    // a dark crease round every one, like a heap of stones, while a cloud's
    // light is soft. The vertex colours shade it instead.
    const material = surface(theme, theme.colors.light);
    material.vertexColors = true;
    this.body = mesh(geo, material);
    this.body.receiveShadow = false;
    this.add(this.body);
  }
}

// The big clumps, tier by tier from the base to the top.
function heap(size: Size, random: () => number): Clump[] {
  const a = size.width / 2;
  const c = size.depth / 2;
  const radius = between(random, ...CLUMP.share) * Math.min(a, c, size.height);
  const topRadius = CLUMP.top * radius;
  const lowest = LIFT * radius * CLUMP.squash[0];
  const highest = size.height - topRadius * CLUMP.squash[1];
  const tiers = Math.max(2, Math.ceil((highest - lowest) / (STEP * radius)) + 1);
  const clumps: Clump[] = [];
  for (let i = 0; i < tiers; i++) {
    const t = i / (tiers - 1);
    const y = THREE.MathUtils.lerp(lowest, highest, t);
    const r = THREE.MathUtils.lerp(radius, topRadius, t);
    const squash = THREE.MathUtils.lerp(CLUMP.squash[0], CLUMP.squash[1], t);
    const shrink = THREE.MathUtils.lerp(1, TOP_RING, t);
    const ringX = shrink * Math.max(0, a - r);
    const ringZ = shrink * Math.max(0, c - r);
    // Round the ellipse's edge (Ramanujan's perimeter), RING radii apart.
    const perimeter = Math.PI * (3 * (ringX + ringZ) - Math.sqrt((3 * ringX + ringZ) * (ringX + 3 * ringZ)));
    const count = Math.max(i === tiers - 1 ? TOWERS : 1, Math.round(perimeter / (RING * r)));
    const start = 2 * Math.PI * random();
    const add = (x: number, z: number) => {
      const k = r * between(random, 1 - WANDER, 1 + WANDER);
      clumps.push({
        center: new THREE.Vector3(x + WANDER * r * (2 * random() - 1), y, z + WANDER * r * (2 * random() - 1)),
        size: new THREE.Vector3(k, k * squash, k),
        turn: new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), 2 * Math.PI * random()),
      });
    };
    for (let j = 0; j < count; j++) {
      const angle = start + (2 * Math.PI * (j + 0.3 * (2 * random() - 1))) / count;
      add(ringX * Math.cos(angle), ringZ * Math.sin(angle));
    }
    // A clump in the middle too where the ring is wide enough to leave a hole.
    if (count > 1 && Math.min(ringX, ringZ) > 0.8 * r) add(0, 0);
  }
  return clumps;
}

// Small billows on the surface of the big clumps: each on a clump, in a
// random direction up or to the side, sunk a little into it. Billows whose
// spot lies deep inside another clump would never show, so they are left out.
function billows(big: Clump[], count: number, radius: number, random: () => number): Clump[] {
  const inside = (p: THREE.Vector3, c: Clump) => {
    const d = p.clone().sub(c.center);
    return Math.hypot(d.x / c.size.x, d.y / c.size.y, d.z / c.size.z);
  };
  const result: Clump[] = [];
  for (let attempt = 0; result.length < count && attempt < 40 * count; attempt++) {
    const on = big[Math.floor(random() * big.length)];
    const direction = randomUnit(random);
    if (direction.y < BILLOWS.rise) continue;
    const r = radius * between(random, ...BILLOWS.radius);
    // The point on the clump's surface that way, and the billow's middle a
    // little inside it.
    const reach = 1 / Math.hypot(direction.x / on.size.x, direction.y / on.size.y, direction.z / on.size.z);
    const spot = on.center.clone().addScaledVector(direction, reach);
    if (big.some((c) => c !== on && inside(spot, c) < 0.85)) continue;
    result.push({
      center: spot.addScaledVector(direction, -BILLOWS.sink * r),
      size: new THREE.Vector3(r, r, r),
      turn: new THREE.Quaternion().setFromAxisAngle(randomUnit(random), 2 * Math.PI * random()),
    });
  }
  return result;
}

// The whole cloud as one geometry: every clump lumpy (see clumpGeometry in
// Tree/parts.ts), cut flat at the base, and shaded darker on its underside
// and toward the base.
function cloudGeometry(size: Size, random: () => number): THREE.BufferGeometry {
  const big = heap(size, random);
  const radius = Math.max(...big.map((c) => c.size.x));
  const small = billows(big, Math.round(BILLOWS.each * big.length), radius, random);

  const detail = (c: Clump) => DETAIL.find(([r]) => c.size.x >= r)![1];
  const parts = [...big, ...small].map((c) => clumpGeometry(c, random, detail(c)));
  const top = Math.max(...big.map((c) => c.center.y + c.size.y));
  for (const geo of parts) {
    const floor = FLOOR * random();
    const position = geo.getAttribute('position');
    for (let i = 0; i < position.count; i++) if (position.getY(i) < floor) position.setY(i, floor);
    geo.computeVertexNormals();
    const tint = between(random, ...TINT);
    const normal = geo.getAttribute('normal');
    const color = geo.getAttribute('color');
    for (let i = 0; i < position.count; i++) {
      const height = THREE.MathUtils.clamp(position.getY(i) / top, 0, 1);
      const shade = tint * THREE.MathUtils.lerp(UNDERSIDE, 1, 0.5 + 0.5 * normal.getY(i)) * THREE.MathUtils.lerp(BASE_SHADE, 1, Math.sqrt(height));
      color.setXYZ(i, shade, shade, shade);
    }
  }
  const geo = mergeGeometries(parts);
  for (const part of parts) part.dispose();
  return geo;
}
