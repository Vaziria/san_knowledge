import * as THREE from 'three';
import { blend, Sculpt, tone, type Palette, type Tone } from '../parts';
import { backSpike, bendingMeshes, bodyScaleTone, chainOf, cover, CRYSTAL, hash, leaf, lid, plate, ringLoft, sectionRing, shard, shellAt, shellOf, type Chain, type Face, type Materials, type Shell } from './parts';

// The forest spirit dragon's tail (ForestSpiritDragon.md), as the overview
// and the tail sheet draw it: long and tapering, falling from the rump to
// just above the ground and rising a little at its end; leaf scales over
// its top and sides pointing to its tip, cream plates beneath, each
// overlapping the next toward the tip; a spine of flat crystals along its
// top, smaller toward the tip; and at its end a plume of crystal blades and
// leaf blades, green, gold and cream, fanning up and back.
//
// It bends: a bone at each point of its line, its surface blending from
// each to the next all along, so it sways and streams out; the plume rides
// on the last bone, whole.
//
// Meters, in the dragon's frame (y up, facing +z, its left at -x). Its
// origin is its root in the body, where it would swing.

const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

export const TAIL_AT = v(0, 1.62, -1.45);

// The line through its middle from inside the body to its end, measured
// off the overview's size comparison (its height), and curving round to
// the dragon's left in its last half, as the overview's big picture lays
// it; and at each point half its height and half its width.
const PATH = [v(0, 1.6, -1.2), v(0, 1.48, -1.92), v(-0.02, 1.14, -2.55), v(-0.12, 0.84, -3.18), v(-0.35, 0.68, -3.74), v(-0.66, 0.62, -4.18), v(-0.98, 0.68, -4.46)];
const SIZES: readonly (readonly [h: number, w: number])[] = [
  [0.38, 0.4],
  [0.44, 0.41],
  [0.4, 0.36],
  [0.32, 0.29],
  [0.24, 0.21],
  [0.16, 0.14],
  [0.1, 0.09],
];
const END = v(-1.14, 0.76, -4.6);
const CORNERS = 12;
// How far either side of its underside the cream plates reach.
const CREAM = 1.5;

export function tailShell(): Shell {
  const last = PATH.length - 1;
  return shellOf(
    PATH.map((at, i) => {
      const t = PATH[Math.min(last, i + 1)].clone().sub(PATH[Math.max(0, i - 1)]).normalize();
      // Its right side, square to it and level, and its top.
      const side = new THREE.Vector3().crossVectors(t, v(0, 1, 0)).normalize();
      const up = new THREE.Vector3().crossVectors(side, t);
      const [h, w] = SIZES[i];
      return sectionRing(at, side, up, w, h, h, CORNERS, 2.1);
    }),
  );
}

function angleOf(k: number): number {
  return (Math.PI * 2 * (k + 0.5)) / CORNERS;
}

function skinTone(r: number, k: number): Tone {
  if (Math.abs(angleOf(k) - Math.PI) < CREAM && r < PATH.length - 1) return tone('seam');
  return blend('scale', 'scaleDark', hash(r, k, 7) < 0.5 ? 0.4 : 0.6);
}

// The tail's bones: its root, then each point of its line.
const JOINTS = [TAIL_AT, ...PATH.slice(1)];
const LOW_CELL = 0.3; // m, the squares its underside is sampled in

export class Tail extends THREE.Group {
  readonly chain: Chain;

  constructor(palette: Palette, random: () => number, materials: Materials) {
    super();
    const skin = new Sculpt(random, 0.025);
    const gems = new Sculpt(random, 0.05);
    const plumeLeaves = new Sculpt(random, 0.025);
    const plumeGems = new Sculpt(random, 0.05);
    const sh = tailShell();
    const face: Face = (a, b, c, inside, t) => skin.facing(a, b, c, inside, t);
    ringLoft(face, sh.rings, skinTone, lid(sh, false), END);
    underPlates(skin, sh);
    cover(skin, sh, random, {
      from: 0.7,
      to: PATH.length - 1.2,
      half: () => Math.PI - CREAM - 0.04,
      length: (u) => THREE.MathUtils.lerp(0.54, 0.24, THREE.MathUtils.clamp((u - 0.7) / 5, 0, 1)),
      tone: () => bodyScaleTone(random),
      way: 1,
      width: 0.72,
      along: 0.58,
      droop: 0.4,
      lift: 0.05,
      curl: 0.04,
      ridge: 0.14,
    });
    spine(skin, gems, sh, random);
    plume(plumeLeaves, plumeGems, random);
    this.chain = chainOf(this, 'tail', JOINTS);
    const last = JOINTS.length - 1;
    this.add(
      ...bendingMeshes(this.chain, palette, [
        [skin, materials.skin],
        [gems, materials.crystal],
        [plumeLeaves, materials.skin, last],
        [plumeGems, materials.crystal, last],
      ], 'linear'),
    );
  }

  // Its underside as it stands: the lowest corner over each LOW_CELL m
  // square, of the tail and of its plume, which is what nears the ground as
  // it bends.
  lowest(): { mesh: THREE.SkinnedMesh; index: number }[] {
    const best = new Map<string, { mesh: THREE.SkinnedMesh; index: number; y: number }>();
    const p = new THREE.Vector3();
    this.children.forEach((child, m) => {
      if (!(child instanceof THREE.SkinnedMesh)) return;
      const position = child.geometry.getAttribute('position');
      for (let i = 0; i < position.count; i++) {
        p.fromBufferAttribute(position, i);
        const key = `${m}:${Math.round(p.z / LOW_CELL)}:${Math.round(p.x / LOW_CELL)}`;
        const known = best.get(key);
        if (!known || p.y < known.y) best.set(key, { mesh: child, index: i, y: p.y });
      }
    });
    return [...best.values()].map(({ mesh, index }) => ({ mesh, index }));
  }

  // The way each of its bones runs as it stands, in its own space.
  get ways(): THREE.Vector3[] {
    const j = this.chain.joints;
    return j.slice(1).map((p, i) => p.clone().sub(j[i]).normalize());
  }
}

// Cream plates down its underside, each drawn to a shallow point toward
// the tip over the next.
function underPlates(skin: Sculpt, sh: Shell): void {
  const LENGTH = 0.27;
  let u = 0.9;
  let row = 0;
  while (u < PATH.length - 1.35) {
    const du = LENGTH / shellAt(sh, u, Math.PI).perU;
    const t = row % 2 === 0 ? tone('plate') : blend('plate', 'plateLight', 0.6);
    plate(skin, sh, u, u + du, Math.PI - CREAM * 0.9, Math.PI + CREAM * 0.9, t, { gap: 0.05, bulge: 0.03, vee: 0.3, lip: 0.018 });
    u += du;
    row++;
  }
}

// Flat crystals along its top, leaning toward the tip, smaller toward it.
function spine(skin: Sculpt, gems: Sculpt, sh: Shell, random: () => number): void {
  const count = 8;
  for (let i = 0; i < count; i++) {
    const u = 1.0 + i * 0.6;
    const at = shellAt(sh, u, 0);
    backSpike(skin, gems, at.point, at.normal.clone().addScaledVector(at.along, 1.1), THREE.MathUtils.lerp(0.42, 0.15, i / (count - 1)), random, 0.75);
  }
}

// The plume at its end: crystal blades and leaf blades fanning up and back
// from the end of the tail, the crystals tallest in its middle, the leaves
// between and round them, a few turned out to either side. They grow
// from the tail's last meter, the upright ones further forward, as the
// overview's rise from it.
function plume(skin: Sculpt, gems: Sculpt, random: () => number): void {
  const end = PATH[PATH.length - 1].clone().lerp(END, 0.4);
  const start = PATH[PATH.length - 3].clone().add(v(0, 0.12, 0));
  const baseOf = (degrees: number) => end.clone().lerp(start, 0.8 * THREE.MathUtils.clamp((degrees - 10) / 100, 0, 1));
  // Fanned in the plane of the tail's end, turned with it.
  const heading = END.clone().sub(PATH[PATH.length - 1]);
  const turn = Math.atan2(-heading.x, -heading.z);
  const way = (degrees: number, out: number) => {
    const a = THREE.MathUtils.degToRad(degrees);
    return v(out, Math.sin(a), -Math.cos(a)).normalize().applyAxisAngle(v(0, 1, 0), turn);
  };
  const crystals = [
    { at: 22, out: 0, length: 1.35, radius: 0.15 },
    { at: 44, out: 0.12, length: 1.75, radius: 0.18 },
    { at: 64, out: -0.1, length: 1.95, radius: 0.19 },
    { at: 84, out: 0.06, length: 1.75, radius: 0.18 },
    { at: 106, out: -0.04, length: 1.3, radius: 0.15 },
    { at: 54, out: 0.45, length: 1.25, radius: 0.13 },
    { at: 54, out: -0.45, length: 1.25, radius: 0.13 },
  ];
  for (const c of crystals) shard(gems, baseOf(c.at).addScaledVector(way(c.at, c.out), 0.05), way(c.at, c.out), c.length, c.radius, random, CRYSTAL, 5, 0.4);
  const leaves: { at: number; out: number; length: number; top: Tone }[] = [
    { at: 12, out: 0.05, length: 1.1, top: tone('gold') },
    { at: 33, out: -0.18, length: 1.45, top: tone('scaleLight') },
    { at: 54, out: 0.22, length: 1.6, top: blend('gold', 'leaf', 0.4) },
    { at: 74, out: -0.22, length: 1.55, top: tone('scale') },
    { at: 96, out: 0.18, length: 1.3, top: tone('plate') },
    { at: 40, out: 0.5, length: 1.1, top: tone('scaleLight') },
    { at: 40, out: -0.5, length: 1.1, top: blend('gold', 'scaleLight', 0.5) },
  ];
  for (const l of leaves) {
    const dir = way(l.at, l.out);
    const across = v(1, 0, 0).applyAxisAngle(v(0, 1, 0), turn);
    const flat = across.addScaledVector(dir, -across.dot(dir)).normalize();
    leaf(skin, {
      base: baseOf(l.at),
      along: dir,
      normal: random() < 0.5 ? flat : flat.negate(),
      length: l.length * (0.95 + 0.1 * random()),
      width: l.length * 0.3,
      ridge: 0.25,
      curl: 0.1,
      sink: 0.05,
      top: l.top,
    });
  }
}
