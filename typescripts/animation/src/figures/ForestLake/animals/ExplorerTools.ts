import * as THREE from 'three';
import { bands, ring, U, type End, type V3 } from './ExplorerKit';
import { loft, rigidMesh, Sculpt, tone, tubeRings, type Palette, type Tone } from './parts';

// The explorer's optional tools (Explorer.ts's Hold()), each built from the
// sheet's "Weapon / Tools (Optional)" picture of it: a wooden sword, a
// fishing rod with a red and white float, a pickaxe and an axe (the fifth,
// the lantern, is the one it carries at its side). In meters, in the frame
// of the hand that holds them: the grip at the origin, +y along the handle
// toward the blade or head.

export const TOOLS = ['nothing', 'sword', 'fishing-rod', 'pickaxe', 'axe', 'lantern'] as const;
export type ToolName = (typeof TOOLS)[number];

const X = new THREE.Vector3(1, 0, 0);
const Y = new THREE.Vector3(0, 1, 0);
const Z = new THREE.Vector3(0, 0, 1);

const M = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

// A ring in meters (ExplorerKit's rings are in the sheet's pixels).
function mring(centre: V3, ax: V3, ay: V3, w: number, up: number, down: number, sides: number, n = 2, turn = 0): V3[] {
  return ring(centre, ax, ay, w / U, up / U, down / U, sides, n, turn);
}

// Round sections stacked up y: [y, radius] in meters.
function lathe(s: Sculpt, list: readonly (readonly [number, number])[], paint: (r: number, k: number) => Tone, sides = 6, start: End = 'flat', end: End = 'flat'): void {
  bands(
    s,
    list.map(([y, r]) => mring(M(0, y, 0), X, Z, r, r, r, sides, 2, Math.PI / sides)),
    paint,
    start,
    end,
  );
}

// A box in meters: half sizes along x, y and z.
function block(s: Sculpt, centre: V3, hx: number, hy: number, hz: number, paint: (face: string) => Tone, x = X, y = Y, z = Z): void {
  const c = (i: number, j: number, k: number) =>
    centre
      .clone()
      .addScaledVector(x, i * hx)
      .addScaledVector(y, j * hy)
      .addScaledVector(z, k * hz);
  const quad = (a: V3, b: V3, cc: V3, d: V3, t: Tone) => {
    s.facing(a, b, cc, centre, t);
    s.facing(a, cc, d, centre, t);
  };
  quad(c(1, -1, -1), c(1, 1, -1), c(1, 1, 1), c(1, -1, 1), paint('x'));
  quad(c(-1, -1, -1), c(-1, -1, 1), c(-1, 1, 1), c(-1, 1, -1), paint('-x'));
  quad(c(-1, 1, -1), c(-1, 1, 1), c(1, 1, 1), c(1, 1, -1), paint('y'));
  quad(c(-1, -1, -1), c(1, -1, -1), c(1, -1, 1), c(-1, -1, 1), paint('-y'));
  quad(c(-1, -1, 1), c(1, -1, 1), c(1, 1, 1), c(-1, 1, 1), paint('z'));
  quad(c(-1, -1, -1), c(-1, 1, -1), c(1, 1, -1), c(1, -1, -1), paint('-z'));
}

export interface Tools {
  groups: Record<Exclude<ToolName, 'nothing' | 'lantern'>, THREE.Group>;
  line: THREE.Group; // the fishing line and float, hanging from the rod's tip
}

// Builds the four tools (each added to the hand only while held); `keep`
// registers each mesh to be painted.
export function buildTools(palette: Palette, random: () => number, materials: { coat: THREE.Material; metal: THREE.Material }, keep: (mesh: THREE.Mesh) => THREE.Mesh): Tools {
  const group = (wood: Sculpt, metal?: Sculpt) => {
    const g = new THREE.Group();
    g.add(keep(rigidMesh(wood.geometry(palette), materials.coat)));
    if (metal && metal.triangles > 0) g.add(keep(rigidMesh(metal.geometry(palette), materials.metal)));
    return g;
  };

  // The wooden sword: a pale wooden blade with a ridge down its middle and
  // a point, a dark crossguard, a grip wrapped in bands, a round pommel.
  const sword = new Sculpt(random);
  lathe(
    sword,
    [
      [-0.094, 0.009],
      [-0.088, 0.019],
      [-0.071, 0.02],
      [-0.064, 0.013],
    ],
    (r) => tone('woodShade', r === 1 ? 1.08 : 0.95),
    8,
  );
  lathe(
    sword,
    [
      [-0.066, 0.0135],
      [-0.043, 0.0142],
      [-0.041, 0.0128],
      [-0.017, 0.0138],
      [-0.015, 0.0126],
      [0.01, 0.0136],
      [0.012, 0.0126],
      [0.043, 0.0134],
      [0.047, 0.012],
    ],
    (r) => (r % 2 === 0 ? tone('handle') : tone('handleDark')),
    6,
  );
  block(sword, M(0, 0.056, 0), 0.064, 0.011, 0.016, (f) => (f === 'y' || f === 'z' ? tone('handleDark', 1.1) : tone('handleDark')));
  const blade = [
    [0.064, 0.027, 0.0085],
    [0.1, 0.029, 0.009],
    [0.37, 0.03, 0.0092],
    [0.395, 0.027, 0.0085],
  ].map(([y, w, t]) => mring(M(0, y, 0), X, Z, w, t, t, 4, 1.4, 0));
  bands(sword, blade, (r, k) => (k % 2 === 0 ? tone('wood', r === 1 ? 1.03 : 1) : tone('woodShade', 1.12)), 'flat', M(0, 0.445, 0));
  const swordGroup = group(sword);

  // The fishing rod: a long, bending wooden pole, cream bands wound round
  // it by the grip and a few dark ones along it, its line hanging from the
  // tip to a red and white float.
  const rod = new Sculpt(random);
  const path = [M(0, -0.17, 0), M(0, -0.05, 0), M(0, 0.15, 0.004), M(0, 0.42, 0.016), M(0, 0.68, 0.01), M(0, 0.88, -0.03), M(0, 1.02, -0.09)];
  const radii = [0.017, 0.016, 0.0135, 0.011, 0.0085, 0.0065, 0.005];
  loft(rod, tubeRings(path, radii), {
    sides: 6,
    start: 'flat',
    end: 'flat',
    paint: (r, angle) => (r === 0 ? tone('handleDark') : angle > Math.PI ? tone('handle', 0.9) : tone('handle')),
  });
  // Wound bands.
  const band = (y: number, length: number, r: number, t: Tone) =>
    lathe(
      rod,
      [
        [y, r],
        [y + length, r],
      ],
      () => t,
      6,
    );
  band(-0.075, 0.035, 0.0185, tone('wrap'));
  band(0.03, 0.03, 0.0175, tone('wrap', 0.95));
  band(0.3, 0.012, 0.0132, tone('handleDark'));
  band(0.55, 0.012, 0.0106, tone('handleDark'));
  // The line's ring at the tip.
  const tip = path[path.length - 1];
  const rodGroup = group(rod);
  const line = new THREE.Group();
  line.position.copy(tip);
  const cord = new Sculpt(random, 0.01);
  lathe(
    cord,
    [
      [0, 0.0028],
      [-0.4, 0.0028],
    ],
    () => tone('line'),
    4,
    'open',
    'open',
  );
  // The float: red above, a white band, white below, a stick through it.
  lathe(
    cord,
    [
      [-0.39, 0.003],
      [-0.398, 0.011],
      [-0.407, 0.018],
      [-0.414, 0.021],
      [-0.42, 0.021],
      [-0.428, 0.019],
      [-0.438, 0.012],
      [-0.444, 0.004],
    ],
    (r) => (r <= 1 || r === 3 ? tone('bobber') : tone('white')),
    8,
  );
  lathe(
    cord,
    [
      [-0.443, 0.0035],
      [-0.458, 0.0028],
    ],
    () => tone('bobber', 0.9),
    4,
  );
  line.add(keep(rigidMesh(cord.geometry(palette), materials.coat, false)));
  rodGroup.add(line);

  // The pickaxe: a long wooden handle banded at its foot and wrapped under
  // its head, and a grey steel head curving down to a point either side.
  const pick = new Sculpt(random);
  const pickMetal = new Sculpt(random);
  const handle = (s: Sculpt, top: number) =>
    lathe(
      s,
      [
        [-0.14, 0.016],
        [-0.135, 0.019],
        [-0.1, 0.019],
        [-0.096, 0.0165],
        [top - 0.1, 0.0158],
        [top - 0.096, 0.018],
        [top - 0.05, 0.018],
        [top - 0.046, 0.0156],
        [top + 0.035, 0.0148],
      ],
      (r) => (r <= 1 ? tone('wrap', 0.92) : r === 5 ? tone('handleDark') : tone('handle', r % 2 ? 0.97 : 1.03)),
      6,
    );
  handle(pick, 0.4);
  block(pickMetal, M(0, 0.4, 0), 0.026, 0.03, 0.022, (f) => (f === 'y' ? tone('metal') : tone('metalDark')));
  for (const side of [-1, 1]) {
    const spike = [M(side * 0.02, 0.405, 0), M(side * 0.075, 0.41, 0), M(side * 0.13, 0.395, 0), M(side * 0.175, 0.366, 0)];
    const r: [number, number, number][] = [
      [0.012, 0.021, 0.021],
      [0.011, 0.018, 0.017],
      [0.008, 0.013, 0.012],
      [0.005, 0.008, 0.007],
    ];
    loft(pickMetal, tubeRings(spike, r, Z), {
      sides: 4,
      start: 'flat',
      end: 0.042,
      paint: (_r, angle) => (angle < Math.PI / 2 || angle > Math.PI * 1.5 ? tone('metal', 1.08) : tone('metalDark', 1.05)),
    });
  }
  const pickGroup = group(pick, pickMetal);

  // The axe: the same handle, and a steel head with a broad, flaring blade
  // on one side and a smaller one on the other, their edges bright.
  const axe = new Sculpt(random);
  const axeMetal = new Sculpt(random);
  handle(axe, 0.35);
  block(axeMetal, M(0, 0.35, 0), 0.022, 0.036, 0.022, (f) => (f === 'y' ? tone('metal') : tone('metalDark')));
  for (const [side, reach, top, bottom] of [
    [1, 0.135, 0.43, 0.27],
    [-1, 0.088, 0.395, 0.305],
  ]) {
    // Columns from the head out to the edge: x, top, bottom, half thickness.
    const cols = [
      [0.02, 0.378, 0.322, 0.011],
      [reach * 0.55, 0.35 + (top - 0.35) * 0.45, 0.35 - (0.35 - bottom) * 0.45, 0.007],
      [reach, top, bottom, 0.002],
    ];
    const pts = cols.map(([x, t, b, h]) => ({
      topF: M(side * x, t, h),
      topB: M(side * x, t, -h),
      midF: M(side * (x + (x === reach ? 0.012 : 0)), (t + b) / 2, h),
      midB: M(side * (x + (x === reach ? 0.012 : 0)), (t + b) / 2, -h),
      botF: M(side * x, b, h),
      botB: M(side * x, b, -h),
    }));
    const centre = M(side * reach * 0.4, 0.35, 0);
    for (let i = 0; i + 1 < pts.length; i++) {
      const a = pts[i];
      const b = pts[i + 1];
      const t = i === 1 ? tone('metalEdge') : tone('metal', 1.02);
      for (const [p, q, rr, ss] of [
        [a.topF, b.topF, b.midF, a.midF],
        [a.midF, b.midF, b.botF, a.botF],
      ]) {
        axeMetal.facing(p, q, rr, centre, t);
        axeMetal.facing(p, rr, ss, centre, t);
      }
      for (const [p, q, rr, ss] of [
        [a.topB, a.midB, b.midB, b.topB],
        [a.midB, a.botB, b.botB, b.midB],
      ]) {
        axeMetal.facing(p, q, rr, centre, tone('metalDark', 1.1));
        axeMetal.facing(p, rr, ss, centre, tone('metalDark', 1.1));
      }
      // Top and bottom edges.
      axeMetal.facing(a.topF, a.topB, b.topB, centre, tone('metalDark'));
      axeMetal.facing(a.topF, b.topB, b.topF, centre, tone('metalDark'));
      axeMetal.facing(a.botF, b.botF, b.botB, centre, tone('metalDark'));
      axeMetal.facing(a.botF, b.botB, a.botB, centre, tone('metalDark'));
    }
    // The cutting edge.
    const e = pts[pts.length - 1];
    axeMetal.facing(e.topF, e.midF, e.midB, centre, tone('metalEdge', 1.1));
    axeMetal.facing(e.topF, e.midB, e.topB, centre, tone('metalEdge', 1.1));
    axeMetal.facing(e.midF, e.botF, e.botB, centre, tone('metalEdge', 1.1));
    axeMetal.facing(e.midF, e.botB, e.midB, centre, tone('metalEdge', 1.1));
  }
  const axeGroup = group(axe, axeMetal);

  return { groups: { sword: swordGroup, 'fishing-rod': rodGroup, pickaxe: pickGroup, axe: axeGroup }, line };
}
