import * as THREE from 'three';
import { blend, rigidMesh, Sculpt, tone, type Palette, type Tone } from '../parts';
import { CRYSTAL, foldedLeaf, hash, horn, hornAt, leaf, pick, Probe, ringLoft, scaleTone, shaded, shard, type Face, type HornPath, type Materials } from './parts';

// The forest spirit dragon's head (ForestSpiritDragon.md), as the head,
// snout, eye and horn sheets draw it: a big rounded head with a short,
// friendly snout, green above and cream below, its forehead and nose
// covered in leaf scales pointing back; big amber eyes with slit pupils,
// set forward under cream brow scales; small nostrils at the green tip of
// the snout and a smiling mouth line; a cream lower jaw, hinged; a ruff of
// leaf scales round the back of the head, green, gold, cream and crystal
// blue, with a tan leaf of an ear on either side and cream fur under the
// chin; and on top a crown of glowing crystals between two branching horns
// of pale wood, with two more horns sweeping back below them.
//
// Meters. Its origin is where the neck holds it, under the back of the
// skull; +z is the way it looks, +y up, its left at -x. It is about 1.3 m
// from the back of its skull to the tip of its snout and 1 m from its chin
// to its crown, its horns reaching 0.7 m above that.

const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);
const TAU = Math.PI * 2;

type P2 = readonly [number, number];

// The skull's cross-sections from the snout back, measured off the snout
// sheet's side view and the horn sheet's front view: at each z, the height
// of its top, and for its right half (+x) each corner [x, y] from its ridge
// down to its lip, the mouth line's band between `lipLine` and `lip` (the
// gum, inside the lip, follows from it); then the palate, the roof of the
// mouth, under the middle. Green above `mid` (on the snout above `eye`),
// cream below: skullTone(). The face narrows in front of the eyes, so they
// look forward, and is widest at them.
interface Section {
  z: number;
  top: number;
  ridge: P2;
  brow: P2;
  eye: P2;
  mid: P2;
  cheek: P2;
  lipLine: P2;
  lip: P2;
  palate: number;
}

const SECTIONS: readonly Section[] = [
  { z: 0.93, top: 0.17, ridge: [0.055, 0.165], brow: [0.095, 0.145], eye: [0.118, 0.11], mid: [0.125, 0.08], cheek: [0.128, 0.025], lipLine: [0.122, -0.035], lip: [0.116, -0.052], palate: -0.045 },
  { z: 0.86, top: 0.235, ridge: [0.075, 0.228], brow: [0.14, 0.19], eye: [0.168, 0.13], mid: [0.18, 0.09], cheek: [0.188, 0.025], lipLine: [0.183, -0.045], lip: [0.177, -0.063], palate: -0.05 },
  { z: 0.72, top: 0.38, ridge: [0.09, 0.365], brow: [0.165, 0.295], eye: [0.19, 0.185], mid: [0.205, 0.12], cheek: [0.222, 0.04], lipLine: [0.216, -0.04], lip: [0.208, -0.06], palate: -0.045 },
  { z: 0.58, top: 0.55, ridge: [0.1, 0.53], brow: [0.18, 0.44], eye: [0.205, 0.27], mid: [0.235, 0.14], cheek: [0.262, 0.05], lipLine: [0.256, -0.03], lip: [0.248, -0.05], palate: -0.04 },
  { z: 0.46, top: 0.675, ridge: [0.115, 0.655], brow: [0.2, 0.535], eye: [0.205, 0.3], mid: [0.26, 0.13], cheek: [0.29, 0.065], lipLine: [0.282, -0.022], lip: [0.272, -0.042], palate: -0.033 },
  { z: 0.32, top: 0.77, ridge: [0.155, 0.75], brow: [0.31, 0.6], eye: [0.365, 0.31], mid: [0.35, 0.15], cheek: [0.33, 0.085], lipLine: [0.3, -0.012], lip: [0.29, -0.032], palate: -0.024 },
  { z: 0.16, top: 0.81, ridge: [0.185, 0.79], brow: [0.36, 0.63], eye: [0.425, 0.34], mid: [0.4, 0.19], cheek: [0.37, 0.11], lipLine: [0.332, 0.01], lip: [0.32, -0.01], palate: -0.005 },
  { z: 0.0, top: 0.79, ridge: [0.185, 0.77], brow: [0.35, 0.62], eye: [0.405, 0.36], mid: [0.385, 0.24], cheek: [0.355, 0.14], lipLine: [0.31, 0.04], lip: [0.295, 0.02], palate: 0.03 },
  { z: -0.14, top: 0.72, ridge: [0.17, 0.7], brow: [0.3, 0.58], eye: [0.33, 0.38], mid: [0.32, 0.29], cheek: [0.3, 0.2], lipLine: [0.26, 0.1], lip: [0.24, 0.08], palate: 0.08 },
  { z: -0.26, top: 0.61, ridge: [0.13, 0.595], brow: [0.215, 0.515], eye: [0.24, 0.39], mid: [0.235, 0.32], cheek: [0.215, 0.25], lipLine: [0.175, 0.18], lip: [0.155, 0.16], palate: 0.15 },
];
// The tip of the snout, at the nostrils, over the upper lip; the back of
// the skull.
const SNOUT_TIP = v(0, 0.07, 0.975);
const OCCIPUT = v(0, 0.39, -0.34);
// The rings up to which the band of the mouth line runs (it ends below the
// back of the eye; the line itself is drawn over it, mouthLine()), and
// between which the roof of the mouth is, over the lower jaw: in front of
// the chin the underside of the upper lip is cream.
const MOUTH_LINE_TO = 5;
const PALATE_FROM = 2;
const PALATE_TO = 6;

// The lower jaw's cross-sections, under the upper lip: at each z, its
// right half's corners [x, y] from the lip down its side, and the chin's
// height under the middle.
interface JawSection {
  z: number;
  lip: P2;
  side: P2;
  low: P2;
  chin: number;
}

const JAW: readonly JawSection[] = [
  { z: 0.78, lip: [0.115, -0.074], side: [0.115, -0.125], low: [0.078, -0.175], chin: -0.19 },
  { z: 0.7, lip: [0.165, -0.071], side: [0.168, -0.135], low: [0.112, -0.205], chin: -0.222 },
  { z: 0.58, lip: [0.21, -0.064], side: [0.215, -0.135], low: [0.152, -0.205], chin: -0.22 },
  { z: 0.46, lip: [0.245, -0.055], side: [0.25, -0.125], low: [0.188, -0.195], chin: -0.208 },
  { z: 0.3, lip: [0.27, -0.044], side: [0.28, -0.112], low: [0.215, -0.178], chin: -0.19 },
  { z: 0.14, lip: [0.29, -0.03], side: [0.3, -0.097], low: [0.23, -0.158], chin: -0.17 },
  { z: -0.02, lip: [0.28, -0.012], side: [0.29, -0.075], low: [0.22, -0.13], chin: -0.145 },
];
// The chin, well behind the tip of the snout, as the side view has it.
const CHIN_TIP = v(0, -0.13, 0.825);
const JAW_BACK = v(0, -0.07, -0.07);
// Where the lower jaw turns to open the mouth.
const HINGE = v(0, -0.02, 0.02);

// The eye's opening: half its width and half its height (the sheets' eyes
// are big almonds, a quarter of the head's length), and the iris inside
// it, nearly as tall.
const EYE_A = 0.16;
const EYE_B = 0.13;
const IRIS = 0.126;
// Where each eye sits (its right one; the left at -x), on the face where it
// narrows toward the snout, and the way it looks: out and well forward,
// as the front view's eyes, a little foreshortened, show.
const EYE_AT = v(0.27, 0.215, 0.39);
const EYE_LOOK = v(0.77, 0.08, 0.63);
const EYE_SLANT = 0.22;

export class Head extends THREE.Group {
  // The skull and upper jaw, with its scales, ruff, ears and horns.
  readonly skull: THREE.Mesh;
  // Its crystals and the crystal-blue scales, which glow.
  readonly crystals: THREE.Mesh;
  // The lower jaw, turning at its hinge: rotation.x > 0 opens the mouth.
  readonly jaw: THREE.Group;
  readonly leftEye: THREE.Group;
  readonly rightEye: THREE.Group;

  constructor(palette: Palette, random: () => number, materials: Materials) {
    super();
    const skin = new Sculpt(random, 0.025);
    const gems = new Sculpt(random, 0.05);
    // The skull goes into the probe too, to seat everything else on it.
    const probe = new Probe();
    const face: Face = (a, b, c, inside, t) => {
      skin.facing(a, b, c, inside, t);
      probe.add(a, b, c);
    };
    ringLoft(face, skullRings(), skullTone, SNOUT_TIP, OCCIPUT);
    nostrils(skin, probe);
    mouthLine(skin, probe);
    const eyes = ([1, -1] as const).map((side) => eyeFrame(probe, side));
    for (const eye of eyes) brows(skin, eye);
    foreheadScales(skin, probe, random, eyes);
    ruff(skin, gems, probe, random);
    ears(skin, probe);
    cheeks(skin, probe, random);
    chinFur(skin, probe);
    horns(skin, gems, probe, random);
    crown(gems, probe, random);

    this.skull = rigidMesh(skin.geometry(palette), materials.skin);
    this.skull.name = 'skull';
    this.crystals = rigidMesh(gems.geometry(palette), materials.crystal);
    this.crystals.name = 'crystals';
    this.jaw = lowerJaw(palette, random, materials);
    [this.rightEye, this.leftEye] = eyes.map((eye) => eyeball(eye, palette, random, materials));
    this.add(this.skull, this.crystals, this.jaw, this.leftEye, this.rightEye);
  }
}

// ---------------------------------------------------------------- skull

function skullRings(): THREE.Vector3[][] {
  return SECTIONS.map((s) => {
    // The gum, inside the lip, where the roof of the mouth begins.
    const gum: P2 = [s.lip[0] * 0.78, s.lip[1] + 0.012];
    const right = [s.ridge, s.brow, s.eye, s.mid, s.cheek, s.lipLine, s.lip, gum];
    return [
      v(0, s.top, s.z),
      ...right.map(([x, y]) => v(x, y, s.z)),
      v(0, s.palate, s.z),
      ...right.reverse().map(([x, y]) => v(-x, y, s.z)),
    ];
  });
}

// The faces between corners k and k + 1 are, from the top: green to `mid`,
// cream to the lip line, the mouth line, the lip's underside, then the roof
// of the mouth inside the gums; the same both sides. Toward the snout the
// green narrows to the bridge of the nose between cream sides (to the
// `eye` corner), so that from the front it comes down in a V to the nose;
// the tip (r -1) is yellow-green over a cream upper lip.
const GREEN_TO = [2, 2, 2];
function skullTone(r: number, k: number): Tone {
  const band = k <= 8 ? k : 17 - k;
  const h = hash(r + 3, k);
  if (band <= (r < 0 ? 2 : (GREEN_TO[r] ?? 3))) {
    if (r < 1) return h < 0.5 ? blend('scaleLight', 'leaf', 0.25) : tone('scaleLight');
    if (r >= SECTIONS.length - 1) return blend('scale', 'scaleDark', 0.4);
    if (h < 0.22) return blend('scale', 'scaleDark', 0.45);
    if (h > 0.78) return blend('scale', 'scaleLight', 0.55);
    return tone('scale');
  }
  if (band === 6 && r >= 0 && r <= MOUTH_LINE_TO) return tone('lid');
  if (band === 8 && r >= PALATE_FROM && r <= PALATE_TO) return tone('mouth');
  if (band === 7) return blend('cream', 'creamShade', 0.5);
  return h < 0.3 ? blend('cream', 'creamShade', 0.3) : tone('cream');
}

// The section at `z`, between the two measured either side.
function sectionAt(z: number): Section {
  const after = SECTIONS.findIndex((s) => s.z <= z);
  const i = after < 0 ? SECTIONS.length - 2 : Math.max(0, after - 1);
  const a = SECTIONS[i];
  const b = SECTIONS[Math.min(SECTIONS.length - 1, i + 1)];
  const t = a.z === b.z ? 0 : THREE.MathUtils.clamp((a.z - z) / (a.z - b.z), 0, 1);
  const mix = (p: P2, q: P2): P2 => [p[0] + (q[0] - p[0]) * t, p[1] + (q[1] - p[1]) * t];
  return {
    z,
    top: a.top + (b.top - a.top) * t,
    ridge: mix(a.ridge, b.ridge),
    brow: mix(a.brow, b.brow),
    eye: mix(a.eye, b.eye),
    mid: mix(a.mid, b.mid),
    cheek: mix(a.cheek, b.cheek),
    lipLine: mix(a.lipLine, b.lipLine),
    lip: mix(a.lip, b.lip),
    palate: a.palate + (b.palate - a.palate) * t,
  };
}

// Where a ray toward the head's middle line at height `y` meets the skull,
// from the direction `angle` round from straight up (toward `side`).
function around(probe: Probe, side: number, angle: number, z: number, y = 0.42) {
  const out = v(side * Math.sin(angle), Math.cos(angle), 0);
  return probe.hit(v(0, y, z).addScaledVector(out, 1.5), out.clone().negate());
}

function nostrils(skin: Sculpt, probe: Probe): void {
  for (const side of [1, -1]) {
    const hit = probe.hit(v(side * 0.23, 0.085, 1.25), v(-side * 0.145, 0, -0.3));
    if (!hit) continue;
    const n = hit.normal;
    const up = v(0, 1, 0).addScaledVector(n, -n.y).normalize();
    const across = new THREE.Vector3().crossVectors(up, n).normalize();
    const centre = hit.point.clone().addScaledVector(n, 0.004);
    const sides = 7;
    const ring = Array.from({ length: sides }, (_, i) => {
      const a = (i / sides) * TAU;
      return centre.clone().addScaledVector(across, Math.cos(a) * 0.03).addScaledVector(up, Math.sin(a) * 0.018);
    });
    for (let i = 0; i < sides; i++) skin.toward(centre, ring[i], ring[(i + 1) % sides], n, tone('nostril'));
  }
}

// The mouth line, as the sheets draw it: a dark line along the bottom of
// the upper lip, from below the back of one eye round the front of the
// snout to the other, turning up at its corners in a smile. Drawn as a thin
// band just off the skull.
function mouthLine(skin: Sculpt, probe: Probe): void {
  const smile = (z: number) => 0.03 * (1 - THREE.MathUtils.smoothstep(z, 0.2, 0.34));
  const side = (s: number) =>
    [0.22, 0.3, 0.4, 0.5, 0.6, 0.68, 0.76, 0.83, 0.89].map((z) => {
      const y = sectionAt(z).lip[1] + 0.012 + smile(z);
      return probe.hit(v(s, y, z), v(-s, 0, 0));
    });
  const front = [0.11, 0.055, 0, -0.055, -0.11].map((x) => probe.hit(v(x, -0.036, 2), v(0, 0, -1)));
  const points = [...side(1), ...front, ...side(-1).reverse()].filter((p) => p !== null);
  const HALF = 0.009;
  const edge = points.map(({ point, normal }) => {
    const up = v(0, 1, 0).addScaledVector(normal, -normal.y).normalize();
    const at = point.clone().addScaledVector(normal, 0.003);
    return { top: at.clone().addScaledVector(up, HALF), bottom: at.clone().addScaledVector(up, -HALF), normal };
  });
  for (let i = 0; i + 1 < edge.length; i++) {
    const a = edge[i];
    const b = edge[i + 1];
    const n = a.normal.clone().add(b.normal);
    skin.toward(a.bottom, b.bottom, b.top, n, tone('lid'));
    skin.toward(a.bottom, b.top, a.top, n, tone('lid'));
  }
}

// ---------------------------------------------------------------- scales

// The forehead and the bridge of the nose, covered in rows of broad leaf
// scales lying flat and pointing back, overlapping like shingles, from the
// bridge of the nose back over the crown and down each side to the green's
// edge, but round the eyes; bigger toward the crown. Seen from the front
// they point up and out from the middle, as the sheets' do.
function foreheadScales(skin: Sculpt, probe: Probe, random: () => number, eyes: readonly EyeFrame[]): void {
  const rows = 9;
  for (let row = 0; row < rows; row++) {
    const z = 0.76 - (row / (rows - 1)) * 0.98;
    const s = sectionAt(z);
    // Down to where the green ends (skullTone()).
    const line = ([[0, s.top], s.ridge, s.brow, s.eye, s.mid] as P2[]).slice(0, z > SECTIONS[2].z ? 4 : 5);
    const lengths = line.slice(1).map((p, i) => Math.hypot(p[0] - line[i][0], p[1] - line[i][1]));
    const total = lengths.reduce((a, b) => a + b, 0);
    const grow = THREE.MathUtils.smoothstep(0.76 - z, 0, 0.85);
    const length = 0.19 + 0.11 * grow;
    const width = length * 0.66;
    const pitch = width * 0.8;
    for (let along = (row % 2) * pitch * 0.5; along < total - pitch * 0.25; along += pitch) {
      // The point that far down the section's right half, and the way out of it.
      let rest = along;
      let i = 0;
      while (i < lengths.length - 1 && rest > lengths[i]) rest -= lengths[i++];
      const t = rest / lengths[i];
      const x = line[i][0] + (line[i + 1][0] - line[i][0]) * t;
      const y = line[i][1] + (line[i + 1][1] - line[i][1]) * t;
      const dx = line[i + 1][0] - line[i][0];
      const dy = line[i + 1][1] - line[i][1];
      const out = new THREE.Vector2(-dy, dx).normalize();
      const u = along / total;
      for (const side of along < 1e-6 ? [1] : [1, -1]) {
        const from = v(side * (x + out.x * 0.4), y + out.y * 0.4, z);
        const hit = probe.hit(from, v(-side * out.x, -out.y, 0));
        if (!hit) continue;
        if (eyes.some((eye) => eye.side === side && hit.point.distanceTo(eye.centre) < 0.27)) continue;
        const way = v(side * 0.35 * u, 0, -1);
        const n = hit.normal;
        leaf(skin, {
          base: hit.point,
          along: way.addScaledVector(n, -way.dot(n)),
          normal: n,
          length: length * (0.9 + 0.2 * random()),
          width,
          lift: 0.11,
          curl: 0.05,
          ridge: 0.18,
          sink: 0.02,
          top: scaleTone(random),
        });
      }
    }
  }
}

// The ruff of big leaves round the back of the head, from the crown down
// each side to the throat, standing out sideways and sweeping back, in
// three layers, each further back: green and yellow-green with crystal
// blue among them, a few gold, and a few cream low down by the chin fur.
// Broad leaves, as the sheets' are, overlapping like petals.
function ruff(skin: Sculpt, gems: Sculpt, probe: Probe, random: () => number): void {
  const layers = [
    { z: 0.0, angles: [30, 50, 70, 90, 110, 130], size: 1 },
    { z: -0.15, angles: [20, 40, 60, 80, 100, 120, 145], size: 1.1 },
    { z: -0.26, angles: [35, 65, 95], size: 0.85 },
  ];
  for (const layer of layers) {
    for (const degrees of layer.angles) {
      for (const side of [1, -1]) {
        const angle = THREE.MathUtils.degToRad(degrees + (random() - 0.5) * 6);
        const hit = around(probe, side, angle, layer.z);
        if (!hit) continue;
        const out = v(side * Math.sin(angle), Math.cos(angle), 0);
        const way = out.clone().multiplyScalar(0.42).add(v(0, -0.12 * (degrees / 180), -0.9)).normalize();
        const length = (0.32 + 0.16 * Math.sin(angle)) * layer.size * (0.92 + 0.16 * random());
        const role = degrees < 50 ? pick(random, [['frost', 3.5], ['scale', 3], ['scaleLight', 2], ['scaleDark', 1.5]] as const)
          : degrees < 115 ? pick(random, [['scale', 3.5], ['scaleLight', 2.5], ['frost', 2], ['scaleDark', 2], ['gold', 0.8]] as const)
          : pick(random, [['scale', 3], ['scaleDark', 2], ['scaleLight', 2], ['cream', 1]] as const);
        const crystal = role === 'frost';
        leaf(crystal ? gems : skin, {
          base: hit.point,
          along: way,
          normal: out,
          length,
          width: length * 0.56,
          sink: 0.03,
          curl: 0.06,
          ridge: 0.12,
          top: crystal ? blend('frost', 'crystalLight', 0.3) : random() < 0.35 ? blend(role, 'scaleLight', 0.25) : tone(role),
          under: crystal ? tone('crystalDeep') : undefined,
        });
      }
    }
  }
}

// An ear either side: a big tan leaf behind the eye at the eye's top,
// standing out and back, folded along its middle into a darker hollow.
function ears(skin: Sculpt, probe: Probe): void {
  for (const side of [1, -1]) {
    const angle = THREE.MathUtils.degToRad(56);
    const hit = around(probe, side, angle, 0.04);
    if (!hit) continue;
    foldedLeaf(skin, {
      base: hit.point,
      along: v(side * 0.78, 0.18, -0.6),
      normal: v(side * Math.sin(angle), Math.cos(angle), 0.2),
      length: 0.56,
      width: 0.26,
      depth: 0.22,
      curl: 0.1,
      rim: tone('ear'),
      hollow: tone('earInner'),
      back: shaded(tone('ear'), 0.9),
    });
  }
}

// Gold and cream scales on the cheeks behind and below the eye, standing
// out and back.
function cheeks(skin: Sculpt, probe: Probe, random: () => number): void {
  const scales = [
    { z: 0.17, y: 0.12, way: [0.55, -0.12, -0.83], length: 0.34, role: 'gold' },
    { z: 0.1, y: 0.21, way: [0.5, 0.05, -0.86], length: 0.3, role: 'cream' },
    { z: 0.22, y: 0.03, way: [0.5, -0.3, -0.8], length: 0.26, role: 'cream' },
  ] as const;
  for (const side of [1, -1]) {
    for (const c of scales) {
      const hit = probe.hit(v(side, c.y, c.z), v(-side, 0, 0));
      if (!hit) continue;
      leaf(skin, {
        base: hit.point,
        along: v(side * c.way[0], c.way[1], c.way[2]),
        normal: hit.normal,
        length: c.length * (0.95 + 0.1 * random()),
        width: c.length * 0.36,
        lift: 0.12,
        curl: 0.12,
        top: c.role === 'cream' ? blend('cream', 'gold', 0.2) : tone(c.role),
      });
    }
  }
}

// Cream fur under the chin, behind the jaw: a few points hanging down and
// back either side.
function chinFur(skin: Sculpt, probe: Probe): void {
  const tufts = [
    { x: 0.16, z: -0.02, way: [0.3, -0.8, -0.5], length: 0.3, role: tone('cream') },
    { x: 0.27, z: -0.1, way: [0.5, -0.7, -0.5], length: 0.26, role: blend('cream', 'gold', 0.35) },
  ] as const;
  for (const side of [1, -1]) {
    for (const t of tufts) {
      const hit = probe.hit(v(side * t.x, -1, t.z), v(0, 1, 0));
      if (!hit) continue;
      leaf(skin, {
        base: hit.point,
        along: v(side * t.way[0], t.way[1], t.way[2]),
        normal: v(side, -0.2, -0.4),
        length: t.length,
        width: t.length * 0.36,
        sink: 0.03,
        curl: 0.2,
        top: t.role,
      });
    }
  }
}

// ---------------------------------------------------------------- horns

// The horns either side, as the horn sheet takes them apart: the main horn
// rising up and back from the top of the head behind the crystals, a tine
// forking up and out of it low down; an upper branch behind it, sweeping
// back; and the middle horn, the longest, sweeping straight back from the
// side of the head above the ear. A green leaf below them, a crystal at
// their roots, and green leaves round their roots.
function horns(skin: Sculpt, gems: Sculpt, probe: Probe, random: () => number): void {
  const paint = (ring: number, k: number): Tone => {
    const h = hash(ring + 11, k, Math.floor(random() * 4));
    if (h < 0.25) return blend('horn', 'hornDark', 0.45);
    if (h > 0.68) return blend('horn', 'hornLight', 0.7);
    return tone('horn');
  };
  for (const side of [1, -1]) {
    const rootOf = (degrees: number, z: number) => {
      const hit = around(probe, side, THREE.MathUtils.degToRad(degrees), z);
      return hit ? { at: hit.point.clone().addScaledVector(hit.normal, -0.04), hit } : null;
    };
    const main = rootOf(26, 0.0);
    const upper = rootOf(36, -0.09);
    const middle = rootOf(50, -0.05);
    if (!main || !upper || !middle) continue;
    const top: HornPath = { root: main.at, start: v(side * 0.3, 0.9, -0.2), end: v(side * 0.22, 0.5, -0.84), length: 1.05 };
    horn(skin, top, 0.12, random, paint);
    horn(skin, { root: hornAt(top, 0.3), start: v(side * 0.6, 0.75, 0.22), end: v(side * 0.5, 0.86, -0.1), length: 0.42 }, 0.058, random, paint, 5, 3);
    horn(skin, { root: upper.at, start: v(side * 0.4, 0.6, -0.7), end: v(side * 0.32, 0.3, -0.9), length: 0.74 }, 0.075, random, paint, 6, 4);
    horn(skin, { root: middle.at, start: v(side * 0.48, 0.12, -0.87), end: v(side * 0.34, 0.3, -0.89), length: 1.0 }, 0.085, random, paint, 6, 5);
    // The lower branch: a green leaf below the middle horn.
    const lower = around(probe, side, THREE.MathUtils.degToRad(66), -0.05);
    if (lower) {
      leaf(skin, { base: lower.point, along: v(side * 0.45, -0.02, -0.9), normal: v(side, 0.3, 0), length: 0.46, width: 0.14, sink: 0.03, curl: 0.12, top: tone('scale') });
    }
    // Their roots: a crystal in front, green leaves round them.
    shard(gems, main.hit.point.clone().add(v(0, -0.02, 0.1)), v(side * 0.35, 0.85, -0.2), 0.3, 0.055, random, CRYSTAL);
    for (const [degrees, z, length] of [
      [22, 0.05, 0.24],
      [30, -0.06, 0.26],
    ] as const) {
      const hit = around(probe, side, THREE.MathUtils.degToRad(degrees), z);
      if (!hit) continue;
      leaf(skin, { base: hit.point, along: v(side * 0.35, 0.3, -0.9), normal: hit.normal, length, width: length * 0.42, lift: 0.25, curl: 0.15, top: scaleTone(random) });
    }
  }
}

// The crown of crystals on top of the head between the horns: on either
// side of the middle a tall one, a shorter one in front leaning forward and
// a small one behind leaning out, and one in the middle at the back,
// leaning back.
function crown(gems: Sculpt, probe: Probe, random: () => number): void {
  const crystals = [
    { x: 0.08, z: 0.22, axis: [0.18, 1, -0.05], length: 0.5, radius: 0.115 },
    { x: 0.1, z: 0.42, axis: [0.22, 0.95, 0.35], length: 0.32, radius: 0.085 },
    { x: 0.17, z: 0.1, axis: [0.5, 0.85, -0.25], length: 0.22, radius: 0.06 },
  ] as const;
  const seat = (x: number, z: number) => {
    const hit = probe.hit(v(x, 2, z), v(0, -1, 0));
    return hit ? hit.point.clone().addScaledVector(hit.normal, -0.01) : null;
  };
  for (const side of [1, -1]) {
    for (const c of crystals) {
      const at = seat(side * c.x, c.z);
      if (at) shard(gems, at, v(side * c.axis[0], c.axis[1], c.axis[2]), c.length, c.radius, random, CRYSTAL);
    }
  }
  const back = seat(0, -0.02);
  if (back) shard(gems, back, v(0, 0.75, -0.65), 0.36, 0.09, random, CRYSTAL);
}

// ---------------------------------------------------------------- eyes

interface EyeFrame {
  side: 1 | -1;
  centre: THREE.Vector3;
  // Across, up and out of the eye (right-handed), and which way along x
  // the snout is.
  x: THREE.Vector3;
  y: THREE.Vector3;
  z: THREE.Vector3;
  front: number;
}

// Where an eye sits on the skull and the way it looks: mostly well
// forward, partly straight out of the skull there, pushed out a little so
// that its socket (eyeball()) tucks it in.
function eyeFrame(probe: Probe, side: 1 | -1): EyeFrame {
  const at = v(side * EYE_AT.x, EYE_AT.y, EYE_AT.z);
  const hit = probe.hit(at.clone().add(v(side, 0, 0)), v(-side, 0, 0));
  if (!hit) throw new Error('the eye found no skull to sit in');
  const z = hit.normal.clone().multiplyScalar(0.3).addScaledVector(v(side * EYE_LOOK.x, EYE_LOOK.y, EYE_LOOK.z).normalize(), 0.7).normalize();
  const y = v(0, 1, 0).addScaledVector(z, -z.y).normalize();
  const x = new THREE.Vector3().crossVectors(y, z);
  const front = Math.sign(x.z) || 1;
  // Slanting, its back corner higher than its front, as the sheets draw it.
  x.applyAxisAngle(z, -front * EYE_SLANT);
  y.applyAxisAngle(z, -front * EYE_SLANT);
  return { side, centre: hit.point.clone().addScaledVector(z, 0.03), x, y, z, front };
}

// A point of the eye's opening round its middle, at angle t (0 toward x,
// up at π/2), `kx` and `ky` times its size: almond-shaped, its corners
// pointed, its top a little flatter than its bottom.
function opening(t: number, kx = 1, ky = kx, z = 0): THREE.Vector3 {
  const s = Math.sin(t);
  return v(Math.cos(t) * EYE_A * kx, s * (0.68 + 0.32 * Math.abs(s)) * EYE_B * ky * (s > 0 ? 0.96 : 1), z);
}

// How much wider than the opening the dark line round it is: thickest
// along the top, as the sheets line their eyes.
function lineWidth(t: number): number {
  const s = Math.sin(t);
  return s > 0 ? 0.07 + 0.12 * s : 0.035;
}

// The eye's own pieces, in its frame: the white, the iris domed in rings of
// dark rim, orange and gold, the slit pupil, and the glints; round them
// the dark line, the green upper lid standing over it, the lower lid, and
// the socket, which tucks into the skull. The brow scales over it are the
// skull's (brows()).
function eyeball(eye: EyeFrame, palette: Palette, random: () => number, materials: Materials): THREE.Group {
  const ball = new Sculpt(random, 0.02);
  const lids = new Sculpt(random, 0.03);
  const shine = new Sculpt(random, 0);
  const out = v(0, 0, 1);
  const segments = 16;
  const ts = Array.from({ length: segments + 1 }, (_, i) => (i / segments) * TAU);

  // The white, a low dome filling the opening.
  const white = v(0, 0, 0.006);
  for (let i = 0; i < segments; i++) ball.toward(white, opening(ts[i]), opening(ts[i + 1]), out, tone('sclera'));

  // The iris, toward the front of the opening, domed.
  const shift = eye.front * 0.006;
  const dome = (r: number) => 0.02 - 0.014 * (r / IRIS) ** 2;
  const radii = [IRIS, 0.86 * IRIS, 0.6 * IRIS, 0.32 * IRIS];
  const bands = [tone('irisRim'), tone('irisMid'), tone('iris'), blend('iris', 'leaf', 0.35)];
  const irisSides = 14;
  const irisRing = (r: number) =>
    Array.from({ length: irisSides }, (_, i) => {
      const a = (i / irisSides) * TAU + Math.PI / irisSides;
      return v(shift + Math.cos(a) * r, Math.sin(a) * r, dome(r));
    });
  const rings = radii.map(irisRing);
  for (let j = 0; j + 1 < rings.length; j++) {
    for (let i = 0; i < irisSides; i++) {
      const i1 = (i + 1) % irisSides;
      ball.toward(rings[j][i], rings[j][i1], rings[j + 1][i1], out, bands[j]);
      ball.toward(rings[j][i], rings[j + 1][i1], rings[j + 1][i], out, shaded(bands[j], 0.95));
    }
  }
  const middle = v(shift, 0, dome(0));
  const inner = rings[rings.length - 1];
  for (let i = 0; i < irisSides; i++) ball.toward(middle, inner[i], inner[(i + 1) % irisSides], out, bands[3]);

  // The pupil, a tall slit lying on the dome.
  const slit: P2[] = [
    [0, 0.098],
    [0.014, 0.066],
    [0.024, 0],
    [0.014, -0.066],
    [0, -0.098],
    [-0.014, -0.066],
    [-0.024, 0],
    [-0.014, 0.066],
  ];
  const onDome = (x: number, y: number, lift: number) => v(shift + x, y, dome(Math.min(IRIS, Math.hypot(x, y))) + lift);
  const pupil = onDome(0, 0, 0.003);
  for (let i = 0; i < slit.length; i++) {
    const [ax, ay] = slit[i];
    const [bx, by] = slit[(i + 1) % slit.length];
    ball.toward(pupil, onDome(ax, ay, 0.003), onDome(bx, by, 0.003), out, tone('pupil'));
  }

  // The glints: a big one up and toward the snout, a small one below.
  for (const [x, y, r] of [
    [eye.front * 0.044, 0.056, 0.022],
    [-eye.front * 0.04, -0.054, 0.009],
  ] as const) {
    const c = onDome(x, y, 0.006);
    const g = Array.from({ length: 6 }, (_, i) => c.clone().add(v(Math.cos((i / 6) * TAU) * r, Math.sin((i / 6) * TAU) * r, 0)));
    for (let i = 0; i < 6; i++) shine.toward(c, g[i], g[(i + 1) % 6], out, tone('glint'));
  }

  // The dark line round the opening, and the upper lid's thick edge.
  for (let i = 0; i < segments; i++) {
    const [t0, t1] = [ts[i], ts[i + 1]];
    const a = opening(t0, 1, 1, 0.003);
    const b = opening(t1, 1, 1, 0.003);
    const c = opening(t1, 1 + lineWidth(t1), 1 + lineWidth(t1), -0.002);
    const d = opening(t0, 1 + lineWidth(t0), 1 + lineWidth(t0), -0.002);
    lids.toward(a, b, c, out, tone('lid'));
    lids.toward(a, c, d, out, tone('lid'));
  }
  // The upper lid: green, standing out over the top of the eye, from the
  // dark line to its outer edge, its own edge dark.
  const half = segments / 2;
  const LID_OUT = 1.3;
  for (let i = 0; i < half; i++) {
    const [t0, t1] = [ts[i], ts[i + 1]];
    const rise = (t: number) => 0.026 * Math.sin(t);
    const e0 = opening(t0, 1 + lineWidth(t0), 1 + lineWidth(t0), -0.002);
    const e1 = opening(t1, 1 + lineWidth(t1), 1 + lineWidth(t1), -0.002);
    const i0 = e0.clone().setZ(rise(t0) - 0.002);
    const i1 = e1.clone().setZ(rise(t1) - 0.002);
    const o0 = opening(t0, LID_OUT, 1.6, -0.025);
    const o1 = opening(t1, LID_OUT, 1.6, -0.025);
    lids.toward(e0, e1, i1, v(0, -1, 0.3), tone('lid'));
    lids.toward(e0, i1, i0, v(0, -1, 0.3), tone('lid'));
    const green = hash(i, 3) < 0.5 ? tone('scale') : blend('scale', 'scaleDark', 0.3);
    lids.toward(i0, i1, o1, out, green);
    lids.toward(i0, o1, o0, out, shaded(green, 0.96));
  }
  // The lower lid, yellow-green toward cream.
  for (let i = half; i < segments; i++) {
    const [t0, t1] = [ts[i], ts[i + 1]];
    const lift = (t: number) => 0.006 * Math.abs(Math.sin(t));
    const i0 = opening(t0, 1 + lineWidth(t0), 1 + lineWidth(t0), lift(t0) - 0.002);
    const i1 = opening(t1, 1 + lineWidth(t1), 1 + lineWidth(t1), lift(t1) - 0.002);
    const o0 = opening(t0, LID_OUT, 1.38, -0.025);
    const o1 = opening(t1, LID_OUT, 1.38, -0.025);
    const lid = blend('scale', 'scaleLight', 0.3 + 0.3 * hash(i, 5));
    lids.toward(i0, i1, o1, out, lid);
    lids.toward(i0, o1, o0, out, shaded(lid, 0.96));
  }
  // The socket, from the lids' outer edge back into the skull.
  for (let i = 0; i < segments; i++) {
    const [t0, t1] = [ts[i], ts[i + 1]];
    const ky = (t: number) => (Math.sin(t) >= 0 ? 1.6 : 1.38);
    const a = opening(t0, LID_OUT, ky(t0), -0.025);
    const b = opening(t1, LID_OUT, ky(t1), -0.025);
    const c = opening(t1, LID_OUT * 1.12, ky(t1) * 1.12, -0.12);
    const d = opening(t0, LID_OUT * 1.12, ky(t0) * 1.12, -0.12);
    const wall = Math.sin((t0 + t1) / 2) >= 0 ? shaded(tone('scale'), 0.92) : shaded(tone('cream'), 0.94);
    const outward = a.clone().add(b).setZ(0).normalize().add(v(0, 0, 0.6));
    lids.toward(a, b, c, outward, wall);
    lids.toward(a, c, d, outward, wall);
  }

  const group = new THREE.Group();
  group.name = eye.side > 0 ? 'right-eye' : 'left-eye';
  group.position.copy(eye.centre);
  group.quaternion.setFromRotationMatrix(new THREE.Matrix4().makeBasis(eye.x, eye.y, eye.z));
  group.add(rigidMesh(ball.geometry(palette), materials.eye), rigidMesh(lids.geometry(palette), materials.skin), rigidMesh(shine.geometry(palette), materials.glint, false));
  return group;
}

// Two cream brow scales over each eye, pointing back and up from its top.
function brows(skin: Sculpt, eye: EyeFrame): void {
  const at = (x: number, y: number, z: number) => eye.centre.clone().addScaledVector(eye.x, x).addScaledVector(eye.y, y).addScaledVector(eye.z, z);
  const way = (x: number, y: number, z: number) => v(0, 0, 0).addScaledVector(eye.x, x).addScaledVector(eye.y, y).addScaledVector(eye.z, z);
  const f = eye.front;
  leaf(skin, {
    base: at(f * 0.04, 0.215, -0.02),
    along: way(-f * 0.75, 0.55, 0.15),
    normal: way(0, 0.3, 1),
    length: 0.3,
    width: 0.105,
    lift: 0.12,
    ridge: 0.25,
    curl: 0.1,
    top: blend('cream', 'gold', 0.25),
  });
  leaf(skin, {
    base: at(-f * 0.09, 0.24, -0.03),
    along: way(-f * 0.85, 0.42, 0.1),
    normal: way(0, 0.3, 1),
    length: 0.26,
    width: 0.095,
    lift: 0.15,
    curl: 0.12,
    top: tone('gold'),
  });
}

// ---------------------------------------------------------------- jaw

function lowerJaw(palette: Palette, random: () => number, materials: Materials): THREE.Group {
  const sculpt = new Sculpt(random, 0.04);
  const rings = JAW.map((s) => {
    const right = [s.lip, s.side, s.low];
    return [
      v(0, s.lip[1] - 0.012, s.z),
      ...right.map(([x, y]) => v(x, y, s.z)),
      v(0, s.chin, s.z),
      ...right.reverse().map(([x, y]) => v(-x, y, s.z)),
    ].map((p) => p.sub(HINGE));
  });
  // Its top inside the lips is the mouth's floor; the rest is cream, its
  // underside a shade darker.
  const paint = (r: number, k: number): Tone => {
    const band = k <= 3 ? k : 7 - k;
    if (band === 0 && r >= 0 && r < JAW.length - 2) return tone('mouth');
    if (band === 3 || r === JAW.length - 1) return blend('cream', 'creamShade', 0.35);
    return hash(r, k, 7) < 0.3 ? blend('cream', 'creamShade', 0.25) : tone('cream');
  };
  const face: Face = (a, b, c, inside, t) => sculpt.facing(a, b, c, inside, t);
  ringLoft(face, rings, paint, CHIN_TIP.clone().sub(HINGE), JAW_BACK.clone().sub(HINGE));
  const jaw = new THREE.Group();
  jaw.name = 'jaw';
  jaw.position.copy(HINGE);
  jaw.add(rigidMesh(sculpt.geometry(palette), materials.skin));
  return jaw;
}
