import * as THREE from 'three';
import { blend, Sculpt, tone, type Palette, type Tone } from '../parts';
import { neckShell } from './Neck';
import { backSpike, bodyScaleTone, cover, hash, partMeshes, plate, Probe, ringLoft, sectionRing, shellAt, shellOf, shellProbe, type Face, type Materials, type Shell } from './parts';

// The forest spirit dragon's body (ForestSpiritDragon.md), as the overview
// draws it: heavy and rounded, 4.3 m from its chest to its rump, its back
// 2.45 m up at the shoulders and falling toward the tail; green above,
// covered in big leaf scales lying back like shingles; cream below, the
// plates of the throat carrying on down the front of the chest, and broad,
// uneven ones over the belly, each row over the next; a row of crystals down
// its spine, the biggest at the shoulders, each in a cup of leaves.
//
// Meters, in the dragon's frame (y up, facing +z, its left at -x). Its
// origin is the middle of its torso. The neck, legs, wings and tail are
// parts of their own, their roots sunk in it.

const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

export const BODY_AT = v(0, 1.7, 0);

// Its cross-sections from the chest back, measured off the overview's size
// comparison (its side) and its back view: at each z the height of its back
// and of its belly, the height of its middle, half its width, and how far
// round from its back the green reaches either side (radians); below it the
// cream, which comes higher up the chest.
interface Section {
  z: number;
  top: number;
  bottom: number;
  mid: number;
  w: number;
  green: number;
}

const SECTIONS: readonly Section[] = [
  { z: 2.4, top: 2.08, bottom: 1.68, mid: 1.88, w: 0.22, green: 0.8 },
  { z: 2.3, top: 2.2, bottom: 1.56, mid: 1.9, w: 0.42, green: 0.85 },
  { z: 2.1, top: 2.38, bottom: 1.3, mid: 1.84, w: 0.6, green: 0.95 },
  { z: 1.7, top: 2.42, bottom: 1.1, mid: 1.76, w: 0.68, green: 1.05 },
  { z: 1.15, top: 2.44, bottom: 1.02, mid: 1.7, w: 0.66, green: 1.2 },
  { z: 0.5, top: 2.44, bottom: 0.99, mid: 1.68, w: 0.67, green: 1.28 },
  { z: -0.15, top: 2.32, bottom: 1.0, mid: 1.66, w: 0.67, green: 1.28 },
  { z: -0.75, top: 2.15, bottom: 1.07, mid: 1.63, w: 0.66, green: 1.3 },
  { z: -1.3, top: 1.98, bottom: 1.2, mid: 1.6, w: 0.57, green: 1.35 },
  { z: -1.72, top: 1.82, bottom: 1.36, mid: 1.6, w: 0.4, green: 1.4 },
];
const CHEST = v(0, 1.88, 2.44);
const RUMP = v(0, 1.6, -1.9);
const CORNERS = 16;

export function bodyShell(): Shell {
  return shellOf(SECTIONS.map((s) => sectionRing(v(0, s.mid, s.z), v(1, 0, 0), v(0, 1, 0), s.w, s.top - s.mid, s.mid - s.bottom, CORNERS, 2.3)));
}

// Where along the body (in rings) a z is.
function uAt(z: number): number {
  for (let i = 0; i + 1 < SECTIONS.length; i++) {
    const a = SECTIONS[i];
    const b = SECTIONS[i + 1];
    if (z <= a.z && z >= b.z) return i + (a.z - z) / (a.z - b.z);
  }
  return z > SECTIONS[0].z ? 0 : SECTIONS.length - 1;
}

// How far round from the back the green reaches, `u` along.
function greenAt(u: number): number {
  const i = THREE.MathUtils.clamp(Math.floor(u), 0, SECTIONS.length - 2);
  return THREE.MathUtils.lerp(SECTIONS[i].green, SECTIONS[i + 1].green, THREE.MathUtils.clamp(u - i, 0, 1));
}

function angleOf(k: number): number {
  return (Math.PI * 2 * (k + 0.5)) / CORNERS;
}

// Under the plates the skin shows as the seams between them; under the
// leaves it is a deeper green; the chest's tip is cream, the rump's green.
function skinTone(r: number, k: number): Tone {
  if (r < 0) return tone('plate');
  const a = angleOf(k);
  const u = Math.min(r + 0.5, SECTIONS.length - 1);
  if (r < SECTIONS.length - 1 && Math.abs(a - Math.PI) < Math.PI - greenAt(u)) return tone('seam');
  return blend('scale', 'scaleDark', hash(r, k, 5) < 0.5 ? 0.4 : 0.6);
}

export class Body extends THREE.Group {
  constructor(palette: Palette, random: () => number, materials: Materials) {
    super();
    const skin = new Sculpt(random, 0.025);
    const gems = new Sculpt(random, 0.05);
    const sh = bodyShell();
    // The chest's plates are seated on the body and the neck's foot, both.
    const front = new Probe();
    const face: Face = (a, b, c, inside, t) => {
      skin.facing(a, b, c, inside, t);
      front.add(a, b, c);
    };
    ringLoft(face, sh.rings, skinTone, CHEST, RUMP);
    const neck = neckShell();
    for (let r = 0; r + 1 < neck.rings.length; r++) {
      const [a, b] = [neck.rings[r], neck.rings[r + 1]];
      for (let k = 0; k < a.length; k++) {
        const k1 = (k + 1) % a.length;
        front.add(a[k], a[k1], b[k1]);
        front.add(a[k], b[k1], b[k]);
      }
    }
    chestPlates(skin, front);
    bellyPlates(skin, sh, random);
    scales(skin, sh, random);
    spine(skin, gems, sh, random);
    this.add(...partMeshes(BODY_AT, palette, [
      [skin, materials.skin],
      [gems, materials.crystal],
    ]));
  }
}

// The plates down the front of the chest, under the neck's: each a band
// across it, drawn down to a shallow point over the one below, as the neck
// sheet carries them from the throat onto the chest; seated by casting at
// the chest from in front.
function chestPlates(skin: Sculpt, front: Probe): void {
  const TOPS = [2.4, 2.18, 1.96, 1.74, 1.52]; // m, each row's top
  const HEIGHT = 0.23;
  const VEE = 0.07;
  const seat = (x: number, y: number, lift: number) => {
    const hit = front.hit(v(x, y, 4), v(0, 0, -1));
    return hit ? { at: hit.point.addScaledVector(hit.normal, lift), normal: hit.normal } : null;
  };
  // How far out the chest still faces forward at a height.
  const halfWidth = (y: number) => {
    let x = 0;
    for (; x < 0.7; x += 0.02) {
      const hit = front.hit(v(x, y, 4), v(0, 0, -1));
      if (!hit || hit.normal.z < 0.5) break;
    }
    return x;
  };
  // Each plate a grid seated on the chest, so it follows its curve: its
  // top edge just off it, its middle domed, its pointed bottom edge proud.
  const COLUMNS = 6;
  TOPS.forEach((top, i) => {
    const bottom = top - HEIGHT;
    const half = halfWidth(top - HEIGHT / 2) * 0.92;
    const grid = [0, 0.5, 1].map((t, row) =>
      Array.from({ length: COLUMNS + 1 }, (_, c) => {
        const s = c / COLUMNS;
        const point = 1 - Math.abs(2 * s - 1);
        const y = THREE.MathUtils.lerp(top, bottom, t) - (row === 2 ? VEE * point : 0);
        const edge = row !== 1 || c === 0 || c === COLUMNS;
        const lift = 0.012 + (row === 2 ? 0.02 : 0) + (edge ? 0 : 0.035 * Math.sin(Math.PI * s));
        return seat(THREE.MathUtils.lerp(-half, half, s), y, lift);
      }),
    );
    if (grid.some((row) => row.some((p) => !p))) return;
    const t = i % 3 === 1 ? tone('plateLight') : i % 3 === 2 ? blend('plate', 'plateShade', 0.4) : tone('plate');
    for (let r = 0; r < 2; r++) {
      for (let c = 0; c < COLUMNS; c++) {
        const [p, q, w, x] = [grid[r][c]!, grid[r][c + 1]!, grid[r + 1][c + 1]!, grid[r + 1][c]!];
        const n = p.normal.clone().add(w.normal);
        skin.toward(p.at, q.at, w.at, n, t);
        skin.toward(p.at, w.at, x.at, n, t);
      }
    }
  });
}

// Broad cream plates over the belly, uneven, laid like paving, every other
// row set half a plate round, from behind the chest's.
function bellyPlates(skin: Sculpt, sh: Shell, random: () => number): void {
  const LENGTH = 0.48; // m each, along the body
  const ACROSS = 0.5; // m each, round it
  let u = 2.0;
  let row = 0;
  while (u < SECTIONS.length - 1.3) {
    const du = LENGTH / shellAt(sh, u, Math.PI).perU;
    const reach = Math.PI - greenAt(u + du / 2);
    const perRadian = shellAt(sh, u + du / 2, Math.PI).perRadian;
    const count = Math.max(2, Math.round((2 * reach * perRadian) / ACROSS));
    const da = (2 * reach) / count;
    const offset = row % 2 === 0 ? 0 : da / 2;
    for (let c = -1; c <= count; c++) {
      const a0 = Math.max(Math.PI - reach, Math.PI - reach + c * da - offset);
      const a1 = Math.min(Math.PI + reach, Math.PI - reach + (c + 1) * da - offset);
      if (a1 - a0 < da * 0.3) continue;
      const h = hash(row, c, 11);
      const t = h < 0.3 ? tone('plateLight') : h > 0.75 ? blend('plate', 'plateShade', 0.5) : tone('plate');
      plate(skin, sh, u, u + du, a0, a1, t, { gap: 0.02, bulge: 0.025, vee: 0.16, lip: 0.02, random, uneven: 0.12 });
    }
    u += du;
    row++;
  }
}

// Leaf scales over its back and sides, pointing back and a little down the
// sides, as the overview lays them; none under the neck, nor on the front
// of the chest, where the cream plates come down from the throat.
function scales(skin: Sculpt, sh: Shell, random: () => number): void {
  const neck = shellProbe(neckShell());
  const chest = (p: THREE.Vector3) => p.z > 1.9 && Math.abs(p.x) < 0.5;
  cover(skin, sh, random, {
    from: 0.4,
    to: SECTIONS.length - 1,
    half: (u) => greenAt(u) - 0.04,
    length: (u) => (u < 1.5 || u > 7.5 ? 0.56 : 0.7),
    tone: () => bodyScaleTone(random),
    way: 1,
    width: 0.74,
    along: 0.62,
    across: 0.8,
    droop: 0.35,
    lift: 0.04,
    curl: 0.04,
    ridge: 0.1,
    sink: 0.025,
    contrast: 0.45,
    skip: (p) => chest(p) || neck.inside(p),
  });
}

// The crystals down its spine, from behind the neck to the tail, getting
// smaller toward it, each leaning back.
function spine(skin: Sculpt, gems: Sculpt, sh: Shell, random: () => number): void {
  const spikes = [
    { z: 0.62, length: 0.66 },
    { z: 0.26, length: 0.61 },
    { z: -0.1, length: 0.56 },
    { z: -0.46, length: 0.5 },
    { z: -0.82, length: 0.44 },
    { z: -1.18, length: 0.38 },
    { z: -1.52, length: 0.33 },
  ];
  for (const s of spikes) {
    const at = shellAt(sh, uAt(s.z), 0);
    backSpike(skin, gems, at.point, at.normal.clone().add(v(0, 0, -0.7)), s.length, random);
  }
}
