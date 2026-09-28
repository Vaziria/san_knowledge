import * as THREE from 'three';
import { addBar, addTuft, between, color, frame, grain, matte, mesh, PALETTE, palette, seededRandom, Shape, snowOn, woodColors, type BarColors, type Season } from './parts';

export interface FenceOptions {
  // Where its posts stand, (x, z) in order along it: a section of two rails
  // runs between each two. By default three posts in a row along x, 1.15 m
  // apart, as the asset sheet draws it.
  posts?: readonly (readonly [number, number])[];
  // The ground's height under a point, for a fence on uneven land: each post
  // stands on it and the rails follow. Level at 0 by default.
  groundAt?: (x: number, z: number) => number;
  // Which side of the posts the rails are nailed to: the right of the way
  // the posts run (1, the default: +z for posts running along +x), or the
  // left (-1).
  side?: 1 | -1;
  tufts?: boolean; // grass round the posts' feet and along the fence, as the reference has; true by default
  season?: Season; // spring by default; its grass the season's, snow on it in winter
  seed?: number;
}

const POST = 0.15; // m, square
const BURIED = 0.2; // m of each post under the ground
const RAIL_TALL = 0.14;
const RAIL_THICK = 0.055;
const RAIL_HEIGHTS = [0.38, 0.73]; // the rails' middles over the ground
const OVERHANG = POST / 2 + 0.11; // how far the rails run past the end posts' middles

// The forest lake's wooden fence (Fence.md), as its asset sheet draws it:
// chunky square posts with their tops cut off, and two thick planks nailed
// across their fronts, running well past the end posts. Rustic: the posts
// lean a little, the planks sag and tilt a little, the grain shows in dark
// lines along the wood, and grass grows round the posts' feet. In winter
// snow lies along its rails and on its posts' tops.
export class Fence extends THREE.Group {
  static readonly HEIGHT = 0.95; // m, the posts over the ground, give or take 3 cm

  readonly posts: THREE.Mesh;
  readonly rails: THREE.Mesh; // with their nails
  readonly tufts: THREE.Mesh;

  constructor(options: FenceOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 3);
    const points = options.posts ?? [
      [-1.15, 0],
      [0, 0],
      [1.15, 0],
    ];
    const groundAt = options.groundAt ?? (() => 0);
    const side = options.side ?? 1;
    const P = palette(options.season);

    // Each section's way along the ground, and the way its rails face.
    const sections = points.slice(1).map(([x, z], i) => {
      const along = new THREE.Vector3(x - points[i][0], 0, z - points[i][1]).normalize();
      const out = new THREE.Vector3(-along.z, 0, along.x).multiplyScalar(side);
      return { along, out };
    });

    // The posts, each facing halfway between its sections, leaning a little.
    const postShape = new Shape();
    const postColors = woodColors(PALETTE.post);
    const standing = points.map(([x, z], i) => {
      const out = new THREE.Vector3()
        .add(sections[i - 1]?.out ?? new THREE.Vector3())
        .add(sections[i]?.out ?? new THREE.Vector3())
        .normalize();
      const foot = new THREE.Vector3(x, groundAt(x, z), z);
      const axis = new THREE.Vector3(between(random, -0.03, 0.03), 1, between(random, -0.03, 0.03)).normalize();
      const length = BURIED + Fence.HEIGHT + between(random, -0.03, 0.03);
      addBar(postShape, frame(foot.clone().addScaledVector(axis, -BURIED / axis.y), axis, out), {
        length,
        width: POST,
        height: POST,
        chamfer: 0.022,
        bevel: 0.016,
        segments: 4,
        rough: 0.004,
        twist: between(random, -0.06, 0.06),
        slant: [between(random, -0.06, 0.06), between(random, -0.06, 0.06)],
        cracks: grain(random, [0, 1, 2, 3], [1, 2], BURIED / length + 0.05),
        caps: [false, true],
        colors: postColors,
        random,
      });
      // Where the post's middle is at a height over its foot.
      const at = (height: number) => foot.clone().addScaledVector(axis, height / axis.y);
      return { at };
    });

    // The rails: two to a section, across the posts' fronts, meeting halfway
    // across a post between sections, with a nail into each post.
    const railShape = new Shape();
    const railColors = woodColors(PALETTE.wood);
    const nailColors: BarColors = {
      side: color(PALETTE.nail),
      groove: color(PALETTE.nail),
      end: color(PALETTE.nail).multiplyScalar(1.5),
      core: color(PALETTE.nail),
    };
    sections.forEach(({ along, out }, i) => {
      const first = i === 0;
      const last = i === sections.length - 1;
      for (const height of RAIL_HEIGHTS) {
        const heights = [height + between(random, -0.02, 0.02), height + between(random, -0.02, 0.02)];
        const front = POST / 2 + RAIL_THICK / 2 + 0.001;
        const start = standing[i].at(heights[0]).addScaledVector(out, front).addScaledVector(along, first ? -OVERHANG : 0.001);
        const end = standing[i + 1].at(heights[1]).addScaledVector(out, front).addScaledVector(along, last ? OVERHANG : -0.001);
        const span = end.clone().sub(start);
        addBar(railShape, frame(start, span, out), {
          length: span.length(),
          width: RAIL_THICK,
          height: RAIL_TALL,
          chamfer: 0.013,
          bevel: 0.009,
          segments: 5,
          rough: 0.003,
          bow: [between(random, -0.009, 0.003), between(random, -0.004, 0.004)],
          twist: between(random, -0.04, 0.04),
          cracks: [...grain(random, [2], [2, 3], 0.02), ...grain(random, [1], [1, 2], 0.02)],
          colors: railColors,
          random,
        });
        // A nail at each end: in the post's middle past an end post, a
        // little into the rail at a post it shares with the next section.
        [
          { post: standing[i], at: heights[0], into: first ? 0 : 0.04 },
          { post: standing[i + 1], at: heights[1], into: last ? 0 : -0.04 },
        ].forEach(({ post, at, into }) => {
          const head = post
            .at(at + between(random, -0.02, 0.02))
            .addScaledVector(out, POST / 2 + RAIL_THICK - 0.003)
            .addScaledVector(along, into + between(random, -0.015, 0.015));
          addBar(railShape, frame(head, out, new THREE.Vector3(0, 1, 0)), {
            length: 0.009,
            width: 0.019,
            height: 0.019,
            chamfer: 0.005,
            bevel: 0.003,
            segments: 1,
            caps: [false, true],
            colors: nailColors,
            random,
          });
        });
      }
    });

    // A clump of grass round each post's foot, and now and then a tuft along
    // the fence.
    const tuftShape = new Shape();
    if (options.tufts ?? true) {
      points.forEach(([x, z]) => {
        const foot = new THREE.Vector3(x, groundAt(x, z), z);
        addTuft(tuftShape, foot, Math.round(between(random, 12, 17)), between(random, 0.17, 0.27), random, 0.09, 0, P);
      });
      sections.forEach(({ along, out }, i) => {
        if (random() < 0.5) return;
        const [x0, z0] = points[i];
        const [x1, z1] = points[i + 1];
        const t = between(random, 0.3, 0.7) * Math.hypot(x1 - x0, z1 - z0);
        const across = between(random, -0.1, 0.12);
        const x = x0 + along.x * t + out.x * across;
        const z = z0 + along.z * t + out.z * across;
        addTuft(tuftShape, new THREE.Vector3(x, groundAt(x, z), z), Math.round(between(random, 6, 9)), between(random, 0.1, 0.17), random, 0, 0, P);
      });
    }

    const wood = matte();
    this.posts = mesh(postShape.geometry(), wood);
    this.rails = mesh(railShape.geometry(), wood);
    this.tufts = mesh(tuftShape.geometry(), matte());
    this.add(this.posts, this.rails, this.tufts);
    if (options.season === 'winter') snowOn(this, random);
  }
}
