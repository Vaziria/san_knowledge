import * as THREE from 'three';
import { between, seededRandom } from '../../figures/Grass/parts';
import type { Theme } from '../../theme';
import { sway, type Wind } from '../wind';
import { BANK_WIDTH, groundHeight, shoreRadius } from './Ground';

// The sward: short grass covering the uneven land round the lake, the ground
// the meadow's plants (Meadow.ts) grow out of. It is tufts of pointed blades,
// of a few shapes (upright and broad, spreading, fine and floppy), each tuft
// sized, turned and shaded at random, 7–26 cm tall with most of them short,
// leaning a little with the slope. They are thickest near the water and round
// `keepClear` (where standing figures go) and thin out up the land. Round
// keepClear they are mown short, 2–4.5 cm like the grass environment's lawn,
// below a keyboard's keys. The colour is the theme's grass colour; the land
// under them is a lawn a shade darker (Ground.ts). They are placed by a seeded
// random generator, so the lake is the same on every load.
//
// Each shape is one instanced mesh. A blade's two sides are both front faces
// with normals pointing up, so it is lit like the ground from either side, as
// in the grass environment. Tufts take shadows but cast none. They bend in
// the wind (../wind.ts), their tips 7 cm at 25 cm up in a full wind.

const SHAPES = [
  { blades: 6, width: 0.1, spread: 0.35, lean: 0.3 }, // upright, broad-bladed
  { blades: 5, width: 0.08, spread: 0.7, lean: 0.45 }, // spreading
  { blades: 8, width: 0.05, spread: 0.55, lean: 0.6 }, // fine, floppy
];
const TUFTS = 9000; // over the land
const AROUND = 12000; // more round keepClear, thicker toward it
const REACH = 14; // m past the top of the bank the sward covers
const FALL = 5; // m over which it thins to about a third, going up the land
const FROM = 0.05; // m past the top of the bank where it starts
const HEIGHT = [0.07, 0.26] as const; // m, more of them short
const MOWN = { height: [0.02, 0.045] as const, radius: 2.2, over: 1.2 }; // m round keepClear, and over which it grows back
const AROUND_RADIUS = 7; // m round keepClear the extra tufts cover
const FOLLOW = 0.7; // how far a tuft leans with the ground's slope
const TINT = [0.72, 1.05] as const; // darkest and lightest tuft, times the grass colour
const TRUNK = 0.4; // m kept free round a tree's trunk
const BEND = { sway: 0.07, reach: 0.25 };
const SEED = 5;

// One blade's shape, 1 unit tall before it leans.
const SEGMENTS = 2; // enough for a blade a few centimeters long, and 40% fewer triangles than 3
const BASE_SHADE = 0.65; // the base is darker than the tip, which has the grass colour
const FOOT = 0.04; // the blades rise from a circle this wide, as a share of the height

const UP = new THREE.Vector3(0, 1, 0);

// A tuft, 1 unit tall: `blades` pointed blades round a small foot, each
// leaning out a little and bending over toward its tip, of widths and heights
// a little unlike.
function tuftGeometry(shape: (typeof SHAPES)[number], random: () => number): THREE.BufferGeometry {
  const positions: number[] = [];
  const colors: number[] = [];
  const indices: number[] = [];
  const p = new THREE.Vector3();
  for (let b = 0; b < shape.blades; b++) {
    const turn = (2 * Math.PI * (b + between(random, -0.3, 0.3))) / shape.blades;
    const height = between(random, 0.6, 1);
    const width = shape.width * between(random, 0.8, 1.2);
    const out = shape.spread * between(random, 0.5, 1);
    const bend = shape.lean * between(random, 0.6, 1.3);
    const first = positions.length / 3;
    const orient = new THREE.Matrix4().makeRotationY(-turn).setPosition(FOOT * Math.cos(turn), 0, FOOT * Math.sin(turn));
    for (let i = 0; i <= SEGMENTS; i++) {
      const t = i / SEGMENTS;
      const half = (width / 2) * (1 - t);
      const x = height * (out * t + bend * t * t); // leaning out along its own +x, the way it faces
      const y = height * t * (1 - 0.3 * bend * t);
      const shade = THREE.MathUtils.lerp(BASE_SHADE, 1, t);
      for (const side of i < SEGMENTS ? [-half, half] : [0]) {
        p.set(x, y, side).applyMatrix4(orient);
        positions.push(p.x, p.y, p.z);
        colors.push(shade, shade, shade);
      }
    }
    // Two triangles between each pair of levels and one up to the tip, then
    // the same wound the other way, so both sides are front faces.
    const front: number[] = [];
    for (let i = 0; i < SEGMENTS - 1; i++) {
      const a = first + 2 * i;
      front.push(a, a + 1, a + 2, a + 1, a + 3, a + 2);
    }
    const last = first + 2 * (SEGMENTS - 1);
    front.push(last, last + 1, first + 2 * SEGMENTS);
    indices.push(...front);
    for (let i = 0; i < front.length; i += 3) indices.push(front[i], front[i + 2], front[i + 1]);
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(positions.map((_, i) => (i % 3 === 1 ? 1 : 0)), 3));
  geo.setIndex(indices);
  return geo;
}

// The way the ground faces at a point.
function groundNormal(x: number, z: number): THREE.Vector3 {
  const e = 0.05;
  const dx = (groundHeight(x + e, z) - groundHeight(x - e, z)) / (2 * e);
  const dz = (groundHeight(x, z + e) - groundHeight(x, z - e)) / (2 * e);
  return new THREE.Vector3(-dx, 1, -dz).normalize();
}

// How far a point is past the top of the bank.
function pastBank(x: number, z: number): number {
  return Math.hypot(x, z) - shoreRadius(Math.atan2(z, x)) - BANK_WIDTH;
}

export class Sward extends THREE.Group {
  constructor(theme: Theme, keepClear: THREE.Vector3, trunks: THREE.Vector3[], wind: Wind) {
    super();
    this.name = 'sward';
    const random = seededRandom(SEED);

    // Spots: over the land, thinning out up it (the distance past the bank
    // is drawn from a falling exponential, cut off at REACH), and more round
    // keepClear, thicker toward it, though not on its mown middle.
    const spots: THREE.Vector2[] = [];
    const cut = 1 - Math.exp(-REACH / FALL);
    for (let k = 0; k < TUFTS; k++) {
      const angle = 2 * Math.PI * random();
      const past = FROM - FALL * Math.log(1 - cut * random());
      const r = shoreRadius(angle) + BANK_WIDTH + past;
      spots.push(new THREE.Vector2(r * Math.cos(angle), r * Math.sin(angle)));
    }
    for (let k = 0; k < AROUND; k++) {
      const angle = 2 * Math.PI * random();
      const r = MOWN.radius + (AROUND_RADIUS - MOWN.radius) * random() ** 1.3;
      spots.push(new THREE.Vector2(keepClear.x + r * Math.cos(angle), keepClear.z + r * Math.sin(angle)));
    }

    const material = new THREE.MeshStandardMaterial({ color: theme.scene.grass, roughness: 0.9, vertexColors: true });
    sway(material, wind, BEND);
    const placed: THREE.Matrix4[][] = SHAPES.map(() => []);
    const shades: number[][] = SHAPES.map(() => []);
    const lean = new THREE.Quaternion();
    const turn = new THREE.Quaternion();
    for (const spot of spots) {
      if (pastBank(spot.x, spot.y) < FROM) continue; // down the bank or in the water
      if (trunks.some((t) => Math.hypot(spot.x - t.x, spot.y - t.z) < TRUNK)) continue;
      const mown = 1 - THREE.MathUtils.smoothstep(Math.hypot(spot.x - keepClear.x, spot.y - keepClear.z), MOWN.radius, MOWN.radius + MOWN.over);
      const short = random() ** 1.3;
      const height = THREE.MathUtils.lerp(
        THREE.MathUtils.lerp(HEIGHT[0], HEIGHT[1], short),
        THREE.MathUtils.lerp(MOWN.height[0], MOWN.height[1], short),
        mown,
      );
      const shape = Math.floor(random() * SHAPES.length);
      lean.setFromUnitVectors(UP, groundNormal(spot.x, spot.y)).slerp(new THREE.Quaternion(), 1 - FOLLOW);
      turn.setFromAxisAngle(UP, 2 * Math.PI * random());
      placed[shape].push(
        new THREE.Matrix4().compose(
          new THREE.Vector3(spot.x, groundHeight(spot.x, spot.y), spot.y),
          lean.clone().multiply(turn),
          new THREE.Vector3(height, height, height),
        ),
      );
      shades[shape].push(between(random, ...TINT));
    }

    const color = new THREE.Color();
    SHAPES.forEach((shape, s) => {
      const tufts = new THREE.InstancedMesh(tuftGeometry(shape, random), material, placed[s].length);
      placed[s].forEach((matrix, i) => {
        tufts.setMatrixAt(i, matrix);
        tufts.setColorAt(i, color.setScalar(shades[s][i]));
      });
      tufts.receiveShadow = true;
      this.add(tufts);
    });
  }
}
