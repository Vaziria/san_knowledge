import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { BARK_TILE } from '../../textures/bark';
import { bark, defaultTheme, wood, type Theme } from '../../theme';
import { grainUVs } from '../Boat/parts';
import { between, mesh, seededRandom } from './parts';

// A dock, as the user asked for the lake meeting ("in lake meeting add dock
// in the lake"): a plain village jetty (dermaga kayu), traditional rather
// than modern. A walkway of wooden planks laid across three beams, with
// narrow gaps between them, running out from its landward end over the
// water. The beams rest on cross-beams, each on a pair of log posts with
// their bark on, about every 2 m, which stand on whatever lies under them:
// the lakebed under the water, the bank near the shore. Nothing else: no
// railing, ladder, lamps or boats.
//
// It is a little uneven, as a hand-built jetty is: the planks are not all as
// wide, sit a little askew and a few millimeters high or low, and the posts
// lean a little, each from its seed. The planks, beams and cross-beams are
// wood with its grain along their length (wood() in theme.ts), the posts
// bark (bark()), both in the theme's colours.
//
// It is 3 m wide, wider than most village jetties, so that the lake
// meeting's animals fit on it: they keep their radius and a hand's width
// more from the water, and the deer and the wolf are 1.7 m long.
//
// Its landward end can come down to the ground (`foot`): the first RAMP m of
// the deck then slope from there to its height, so that an animal walks on
// from the land with no step, whatever the land's humps (at the lake they
// stood up to 26 cm off a flat deck).
//
// Units are meters, y is up. The origin is at the middle of its landward
// end, on the still water's level (y = 0), and it runs out along +z, the top
// of its deck `height` above the water. A post stands where `bedAt` says the
// bed is, in the dock's own coordinates; where the bed comes up to a
// cross-beam (on land) the beam rests on it and has no posts. The spec,
// Dock.md, is a draft: it names no behaviour, so it stands still.

export interface DockOptions {
  theme?: Theme;
  seed?: number; // another seed, another dock of the same kind
  width?: number; // m across its deck, along x
  length?: number; // m from its landward end out, along +z
  height?: number; // m from the still water up to the top of its deck
  foot?: number; // m up the top of its deck is at its landward end, rising to `height` over RAMP m; `height` when left out
  // The height of what lies under it, in its own coordinates: the posts
  // stand on it. A flat bed 1 m down when left out.
  bedAt?: (x: number, z: number) => number;
}

export const DOCK_SIZE = { width: 3, length: 6.5, height: 0.36 };
const SEED = 1;
const PLANK = { width: [0.19, 0.25] as const, gap: 0.012, thick: 0.035, overhang: 0.05, askew: 0.012, rise: 0.004, end: 0.02 };
const BEAM = { width: 0.1, height: 0.13, inset: 0.3 }; // three, lengthwise; the outer two inset from the sides
const CROSS = { width: 0.12, height: 0.12, past: 0.08 }; // on each pair of posts, reaching past them
const POST = { radius: [0.065, 0.085] as const, every: 2, inset: 0.22, sunk: 0.3, lean: 0.03, shift: 0.03, sides: 9, rows: 4 };
const ENDS = { start: 0.35, end: 0.25 }; // m in from each end the first and last pairs of posts stand
const RESTS = 0.06; // m: a cross-beam this close to the bed rests on it, with no posts
const FLAT_BED = -1;
export const RAMP = 1.2; // m of deck from its landward end that slope from its foot to its height

// Where along it (m from its landward end) each pair of posts stands.
function stationsAlong(length: number): number[] {
  const count = Math.max(2, Math.round((length - ENDS.start - ENDS.end) / POST.every) + 1);
  return Array.from({ length: count }, (_, k) => THREE.MathUtils.lerp(ENDS.start, length - ENDS.end, k / (count - 1)));
}

export class Dock extends THREE.Group {
  static readonly SIZE = DOCK_SIZE;
  readonly width: number;
  readonly length: number;
  readonly height: number;

  // Where its posts stand, in its own coordinates (before each is shifted a
  // few centimeters from its seed), for keeping stones out of their way.
  static postSpots(width = DOCK_SIZE.width, length = DOCK_SIZE.length): { x: number; z: number }[] {
    return stationsAlong(length).flatMap((z) => [-1, 1].map((side) => ({ x: side * (width / 2 - POST.inset), z })));
  }

  // How high the underside of its planks and of its beams is, for a deck
  // `height` up: what stands under it must stay below them.
  static undersides(height: number): { planks: number; beams: number } {
    return { planks: height - PLANK.thick, beams: height - PLANK.thick - BEAM.height };
  }

  // The height of the top of its deck `z` m from its landward end: `foot`
  // there, rising to `height` over RAMP m.
  static topAt(z: number, height: number, foot = height): number {
    return z < RAMP ? THREE.MathUtils.lerp(foot, height, Math.max(0, z) / RAMP) : height;
  }

  constructor(options: DockOptions = {}) {
    super();
    this.name = 'dock';
    const theme = options.theme ?? defaultTheme;
    const width = (this.width = options.width ?? DOCK_SIZE.width);
    const length = (this.length = options.length ?? DOCK_SIZE.length);
    const height = (this.height = options.height ?? DOCK_SIZE.height);
    const foot = options.foot ?? height;
    const bedAt = options.bedAt ?? (() => FLAT_BED);
    const rand = seededRandom(options.seed ?? SEED);
    const topAt = (z: number) => Dock.topAt(z, height, foot);
    const slope = Math.atan2(height - foot, RAMP); // radians the ramp rises

    const woodParts: THREE.BufferGeometry[] = [];
    const posts: THREE.BufferGeometry[] = [];

    // The planks, across the dock, from its landward end out; those on the
    // ramp tilted with it.
    for (let z = 0; z < length - 0.08; ) {
      const w = Math.min(between(rand, ...PLANK.width), length - z);
      const across = width + 2 * PLANK.overhang + (rand() * 2 - 1) * PLANK.end;
      const plank = new THREE.BoxGeometry(across, PLANK.thick, w - PLANK.gap);
      grainUVs(plank);
      const middle = z + w / 2;
      plank.applyMatrix4(
        new THREE.Matrix4().compose(
          new THREE.Vector3((rand() * 2 - 1) * PLANK.end, topAt(middle) - PLANK.thick / 2 + (rand() * 2 - 1) * PLANK.rise, middle),
          new THREE.Quaternion().setFromEuler(new THREE.Euler(middle < RAMP ? -slope : 0, (rand() * 2 - 1) * PLANK.askew, (rand() * 2 - 1) * PLANK.askew)),
          new THREE.Vector3(1, 1, 1),
        ),
      );
      woodParts.push(plank);
      z += w;
    }

    // The beams under them, along the dock: up the ramp, then level.
    const rampLength = Math.hypot(RAMP, height - foot);
    for (const x of [-(width / 2 - BEAM.inset), 0, width / 2 - BEAM.inset]) {
      const up = new THREE.BoxGeometry(BEAM.width, BEAM.height, rampLength);
      grainUVs(up);
      up.applyMatrix4(
        new THREE.Matrix4().compose(
          new THREE.Vector3(x, (foot + height) / 2 - PLANK.thick - BEAM.height / 2, RAMP / 2 + 0.05),
          new THREE.Quaternion().setFromEuler(new THREE.Euler(-slope, 0, 0)),
          new THREE.Vector3(1, 1, 1),
        ),
      );
      const level = new THREE.BoxGeometry(BEAM.width, BEAM.height, length - RAMP - 0.05);
      grainUVs(level);
      level.translate(x, height - PLANK.thick - BEAM.height / 2, (RAMP + length) / 2 - 0.025);
      woodParts.push(up, level);
    }

    // Pairs of posts about every POST.every m, a cross-beam on each pair.
    for (const z of stationsAlong(length)) {
      const crossBottom = topAt(z) - PLANK.thick - BEAM.height - CROSS.height;
      const cross = new THREE.BoxGeometry(width - 2 * POST.inset + 2 * CROSS.past + 2 * POST.radius[1], CROSS.height, CROSS.width);
      grainUVs(cross);
      cross.translate(0, crossBottom + CROSS.height / 2, z);
      woodParts.push(cross);
      for (const side of [-1, 1]) {
        const x = side * (width / 2 - POST.inset) + (rand() * 2 - 1) * POST.shift;
        const at = z + (rand() * 2 - 1) * POST.shift;
        const bed = bedAt(x, at);
        const radius = between(rand, ...POST.radius);
        const lean = new THREE.Vector2((rand() * 2 - 1) * POST.lean, (rand() * 2 - 1) * POST.lean);
        if (crossBottom - bed < RESTS) continue; // the cross-beam rests on the bank
        posts.push(post(new THREE.Vector3(x, bed - POST.sunk, at), crossBottom + 0.02, radius, lean));
      }
    }

    this.add(mesh(mergeGeometries(woodParts), wood(theme)));
    if (posts.length) this.add(mesh(mergeGeometries(posts), bark(theme)));
    for (const geo of [...woodParts, ...posts]) geo.dispose();
  }
}

// A log post from `foot` up to the height `top`, leaning by `lean` (m out
// over its length, in x and z), a little thinner at the top. An open tube:
// its foot is in the bed and its top under a cross-beam. Texture coordinates
// in meters along it and a whole number of bark tiles round it, so the bark
// wraps without a seam.
function post(foot: THREE.Vector3, top: number, radius: number, lean: THREE.Vector2): THREE.BufferGeometry {
  const length = top - foot.y;
  const tiles = Math.max(1, Math.round((2 * Math.PI * radius) / BARK_TILE));
  const positions: number[] = [];
  const uvs: number[] = [];
  const indices: number[] = [];
  const { sides, rows } = POST;
  for (let i = 0; i <= rows; i++) {
    const t = i / rows;
    const r = radius * (1 - 0.12 * t);
    const cx = foot.x + lean.x * t;
    const cz = foot.z + lean.y * t;
    for (let j = 0; j <= sides; j++) {
      const a = (j / sides) * 2 * Math.PI;
      positions.push(cx + r * Math.cos(a), foot.y + t * length, cz + r * Math.sin(a));
      uvs.push(t * length, (j / sides) * tiles * BARK_TILE);
    }
  }
  const ring = sides + 1;
  for (let i = 0; i < rows; i++) {
    for (let j = 0; j < sides; j++) {
      const a = i * ring + j;
      indices.push(a, a + ring, a + 1, a + 1, a + ring, a + ring + 1);
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
  geo.setIndex(indices);
  geo.computeVertexNormals();
  return geo;
}
