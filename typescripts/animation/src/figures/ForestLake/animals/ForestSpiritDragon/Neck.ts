import * as THREE from 'three';
import { blend, Sculpt, tone, type Palette, type Tone } from '../parts';
import { backSpike, bendingMeshes, bodyScaleTone, chainOf, cover, hash, leaf, lid, plate, ringLoft, sectionRing, shellAt, shellOf, type Chain, type Face, type Materials, type Shell } from './parts';

// The forest spirit dragon's neck (ForestSpiritDragon.md), as the neck
// sheet and the overview draw it: long and thick, rising from the chest
// and leaning forward into the head; cream plates down its front, each
// overlapping the one below like a snake's; leaf scales layered down its
// back and sides, bigger toward the body; a frill of bigger gold and green
// leaves low on either side; crystals along the back of it, and a cluster
// of big ones at its foot.
//
// It bends: four bones up its middle, from its root to where the head
// turns on it (the last, `top`, carries the head), its surface blending
// from each to the next all along, so it curves as it rears or reaches.
//
// Meters, in the dragon's frame (y up, facing +z, its left at -x). Its
// origin is its root in the chest, where it would bend. Its foot is sunk in
// the body and its top in the head, so neither shows a cut end.

const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

// Where the neck turns on the body.
export const NECK_AT = v(0, 2.35, 1.5);
// Where the head turns on the neck: the top of it, as the overview's size
// comparison stands it, the crown 4.4 m up and the snout 2.6 m ahead of the
// middle of the body.
export const HEAD_AT = v(0, 3.6, 1.6);
// The neck's bones: its root, two up its middle, and the head's.
const JOINTS = [NECK_AT, v(0, 2.82, 1.665), v(0, 3.2, 1.66), HEAD_AT];

// Its cross-sections from its foot (in the chest) to its top (in the head),
// measured off the overview's size comparison and the neck sheet's side
// view: each the point of its front (the throat's plates) and of its back,
// [z, y], and half its width. Each section stands square to its own
// front-to-back line, so they lean back as the neck rises.
interface Section {
  front: readonly [number, number];
  back: readonly [number, number];
  w: number;
}

const SECTIONS: readonly Section[] = [
  { front: [2.2, 2.0], back: [0.7, 2.18], w: 0.52 },
  { front: [2.36, 2.36], back: [0.82, 2.62], w: 0.52 },
  { front: [2.3, 2.72], back: [1.03, 2.92], w: 0.45 },
  { front: [2.14, 3.03], back: [1.19, 3.26], w: 0.38 },
  { front: [1.93, 3.3], back: [1.3, 3.6], w: 0.32 },
  { front: [1.76, 3.48], back: [1.36, 3.88], w: 0.26 },
];
const CORNERS = 14;
// How far either side of its front the cream plates reach (radians round
// from its back, corner 0).
const CREAM = THREE.MathUtils.degToRad(86);

// The neck's surface, in the dragon's space: corner 0 of each ring on its
// back, the rest going round its right side (+x) first.
export function neckShell(): Shell {
  return shellOf(
    SECTIONS.map((s) => {
      const front = v(0, s.front[1], s.front[0]);
      const back = v(0, s.back[1], s.back[0]);
      const depth = back.clone().sub(front);
      const half = depth.length() / 2;
      return sectionRing(front.clone().lerp(back, 0.5), v(1, 0, 0), depth.normalize(), s.w, half, half, CORNERS, 2.2);
    }),
  );
}

// The angle round the neck of the middle of its faces between corners k
// and k + 1, from its back.
function angleOf(k: number): number {
  return (Math.PI * 2 * (k + 0.5)) / CORNERS;
}

// Under the plates the skin shows as the seams between them; under the
// leaves it is a deeper green, so the gaps between them read as shade.
function skinTone(r: number, k: number): Tone {
  const a = angleOf(k);
  if (Math.abs(a - Math.PI) < CREAM) return tone('seam');
  return blend('scale', 'scaleDark', hash(r, k, 3) < 0.5 ? 0.4 : 0.6);
}

export class Neck extends THREE.Group {
  readonly chain: Chain;
  // The bone the head turns on.
  readonly top: THREE.Bone;

  constructor(palette: Palette, random: () => number, materials: Materials) {
    super();
    const skin = new Sculpt(random, 0.025);
    const gems = new Sculpt(random, 0.05);
    const sh = neckShell();
    const face: Face = (a, b, c, inside, t) => skin.facing(a, b, c, inside, t);
    // Its ends, sunk in the chest and the head, closed.
    ringLoft(face, sh.rings, skinTone, lid(sh, false), lid(sh, true));
    throatPlates(skin, sh);
    scales(skin, sh, random);
    frills(skin, sh, random);
    crystals(skin, gems, sh, random);
    this.chain = chainOf(this, 'neck', JOINTS);
    this.top = this.chain.bones[this.chain.bones.length - 1];
    this.add(
      ...bendingMeshes(this.chain, palette, [
        [skin, materials.skin],
        [gems, materials.crystal],
      ], 'linear'),
    );
  }

  // Bends it: `rear` rad back at its middle joints (< 0 reaching forward and
  // down), shared between them, `turn` rad to the dragon's right, and the
  // head's bone pitched `nod` rad more (< 0 nose up) in the neck's space.
  // All 0 is as it stands.
  bend(rear: number, turn: number, nod: number, tilt = 0): void {
    const [root, lower, upper, top] = this.chain.bones;
    root.rotation.set(-0.25 * rear, 0.3 * turn, 0);
    lower.rotation.set(-0.4 * rear, 0.35 * turn, 0);
    upper.rotation.set(-0.35 * rear, 0.35 * turn, 0);
    top.rotation.set(nod + 0.75 * rear, 0, tilt);
  }
}

// The cream plates down its front, from where it leaves the chest to under
// the jaw, each its own band across the front, drawn down to a shallow
// point that overlaps the plate below it.
function throatPlates(skin: Sculpt, sh: Shell): void {
  const HEIGHT = 0.23; // m each
  // From the foot of the throat (the chest's plates go on below it).
  let u = 1.0;
  let row = 0;
  while (u < 4.55) {
    const du = HEIGHT / shellAt(sh, u, Math.PI).perU;
    const t = row % 3 === 1 ? tone('plateLight') : row % 3 === 2 ? blend('plate', 'plateShade', 0.4) : tone('plate');
    plate(skin, sh, u, u + du, Math.PI - CREAM * 0.86, Math.PI + CREAM * 0.86, t, { gap: 0.05, bulge: 0.035, vee: -0.32, lip: 0.022 });
    u += du;
    row++;
  }
}

// Leaf scales down its back and sides, pointing down the neck, from the
// chest to under the head's ruff, smaller toward the head.
function scales(skin: Sculpt, sh: Shell, random: () => number): void {
  cover(skin, sh, random, {
    from: 0.7,
    to: 4.45,
    half: () => Math.PI - CREAM + 0.06,
    length: (u) => THREE.MathUtils.lerp(0.44, 0.28, THREE.MathUtils.clamp((u - 0.7) / 3.7, 0, 1)),
    tone: () => bodyScaleTone(random),
    way: -1,
    width: 0.72,
    along: 0.58,
    droop: 0.15,
    lift: 0.05,
    curl: 0.04,
    ridge: 0.14,
  });
}

// The side frills low on the neck: bigger leaves standing out from either
// side and sweeping back and down, gold and yellow-green.
function frills(skin: Sculpt, sh: Shell, random: () => number): void {
  const leaves = [
    { u: 1.15, angle: 1.5, length: 0.5, role: blend('gold', 'leaf', 0.25) },
    { u: 1.45, angle: 1.85, length: 0.44, role: tone('scaleLight') },
    { u: 1.75, angle: 1.55, length: 0.4, role: tone('gold') },
  ];
  for (const side of [1, -1]) {
    for (const f of leaves) {
      const at = shellAt(sh, f.u, side * f.angle);
      leaf(skin, {
        base: at.point,
        along: v(side * 0.5, -0.55, -0.7),
        normal: at.normal,
        length: f.length * (0.95 + 0.1 * random()),
        width: f.length * 0.42,
        lift: 0.35,
        curl: 0.12,
        sink: 0.03,
        top: f.role,
      });
    }
  }
}

// Crystals along the back of the neck, getting bigger toward the body,
// and a cluster of three big ones at its foot, leaning back and out.
function crystals(skin: Sculpt, gems: Sculpt, sh: Shell, random: () => number): void {
  const back = v(0, 0, -1);
  const along = [
    { u: 1.75, length: 0.44 },
    { u: 2.35, length: 0.37 },
    { u: 2.95, length: 0.31 },
    { u: 3.5, length: 0.25 },
    { u: 3.95, length: 0.2 },
  ];
  for (const c of along) {
    const at = shellAt(sh, c.u, 0);
    backSpike(skin, gems, at.point, at.normal.clone().addScaledVector(back, 0.75), c.length, random);
  }
  const foot = [
    { angle: 0, length: 0.75 },
    { angle: 0.55, length: 0.62 },
    { angle: -0.55, length: 0.62 },
  ];
  for (const c of foot) {
    const at = shellAt(sh, 1.1, c.angle);
    backSpike(skin, gems, at.point, at.normal.clone().addScaledVector(back, 0.6), c.length, random);
  }
}
