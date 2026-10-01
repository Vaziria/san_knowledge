import * as THREE from 'three';
import { blend, Sculpt, tone, type Palette, type Tone } from '../parts';
import { bodyScaleTone, claw, CRYSTAL, hash, leaf, partMeshes, shard, tube, type Materials } from './parts';

// The forest spirit dragon's wings (ForestSpiritDragon.md), as the overview
// draws them, raised and half open as it stands in every picture of it: a
// bat's wing on olive green bones, the arm rising from the back to the
// wrist, higher than the head, where a pale claw points forward; two
// fingers from the wrist, the first along the leading edge to the wing's
// tip, the second down to the membrane's lowest point; a cream membrane
// between them and the arm, cut in a deep scallop between the fingers' tips
// and a shallow one from the second finger to the body; leaf scales along
// the arm, and a crystal at its root. The wing sheet's bones and membrane
// are followed where the overview shows them; its colours, tan bones and
// lilac tips, are not: the overview's green bones are.
//
// It beats about its hinge, the line along its root from the shoulder back
// to where the membrane meets the body (lower()), so the membrane stays on
// the body as it rises and falls.
//
// Meters, in the dragon's frame (y up, facing +z, its left at -x). Each
// wing's origin is its root on the back, where it would turn. The left wing
// is the right one mirrored.

type V = THREE.Vector3;
const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

// The right wing's points as it stands, measured off the overview's size
// comparison (from the side) and its back view: its root on the back, the
// elbow, the wrist, the thumb claw's way, the two fingers' tips, and where
// the membrane's inner edge meets the body behind the root.
export const WING_ROOT = v(0.4, 2.3, 0.2);
const ELBOW = v(0.9, 3.5, 0.0);
const WRIST = v(1.55, 4.6, -0.25);
const THUMB = v(0.12, 0.65, 0.75);
// The sheets' side view has the wings reaching straight back and their
// back view has them spread out to the sides: here they spread halfway.
const TIPS = [v(3.0, 3.0, -4.4), v(2.55, 1.65, -2.5)] as const;
const ON_BODY = v(0.45, 2.0, -0.95);
// How far the leading edge arcs up from straight (most of the way out it
// stands higher than the wrist, as the overview draws it).
const BOW = v(0.1, 0.72, 0);
const BILLOW = 0.08; // m the membrane bulges out between its bones
const ROWS = 4; // facets from the wrist out
const COLUMNS = 4; // facets across, between two bones

// A point a share `t` of the way along a polyline (by length).
function along(points: readonly V[], t: number): V {
  const lengths = points.slice(1).map((p, i) => p.distanceTo(points[i]));
  let rest = t * lengths.reduce((a, b) => a + b, 0);
  for (let i = 0; i < lengths.length; i++) {
    if (rest <= lengths[i] || i === lengths.length - 1) return points[i].clone().lerp(points[i + 1], Math.min(1, rest / lengths[i]));
    rest -= lengths[i];
  }
  return points[points.length - 1].clone();
}

// The edge of the membrane between two tips, drawn in toward `toward` in a
// curve, `depth` of the way there at its deepest, which is `peak` of the
// way from the first tip to the second.
function scallop(from: V, to: V, toward: V, depth: number, peak: number, count: number): V[] {
  const power = Math.log(0.5) / Math.log(peak);
  return Array.from({ length: count + 1 }, (_, i) => {
    const s = i / count;
    const p = from.clone().lerp(to, s);
    return p.lerp(toward, depth * Math.sin(Math.PI * s ** power));
  });
}

export class Wing extends THREE.Group {
  readonly side: 1 | -1;
  // Its hinge, in its own space: from its root back along the body.
  readonly hinge: V;
  // The ends of its fingers, in its own space: what reaches lowest as it
  // beats down.
  readonly tips: V[];

  constructor(side: 1 | -1, palette: Palette, random: () => number, materials: Materials) {
    super();
    this.side = side;
    const m = (p: V) => v(p.x * side, p.y, p.z);
    const root = m(WING_ROOT);
    const elbow = m(ELBOW);
    const wrist = m(WRIST);
    const tips = TIPS.map(m);
    const onBody = m(ON_BODY);
    this.hinge = onBody.clone().sub(root).normalize();
    const skin = new Sculpt(random, 0.025);
    const sheet = new Sculpt(random, 0.03);
    const gems = new Sculpt(random, 0.05);
    const outward = v(side, 0.15, -0.35).normalize();

    // The membrane: between the two fingers, cut deep up under the leading
    // edge, as the overview's is, and between the second finger and the
    // arm, down to the body.
    const arm = [wrist, elbow, root] as const;
    const leading = [wrist, wrist.clone().lerp(tips[0], 0.3).addScaledVector(m(BOW), 0.9), wrist.clone().lerp(tips[0], 0.62).addScaledVector(m(BOW), 0.6), tips[0]];
    const under = along(leading, 0.68);
    membrane(sheet, leading, [wrist, tips[1]], scallop(tips[0], tips[1], under, 0.82, 0.72, COLUMNS + 1), outward);
    membrane(sheet, [wrist, tips[1]], arm, [...scallop(tips[1], onBody, wrist, 0.2, 0.5, COLUMNS), root], outward);

    // The bones: the arm, thick, with knuckles at the elbow and the wrist,
    // and the fingers thinning to points just past the membrane.
    const bone = (r: number, k: number): Tone => {
      const h = hash(r + 17, k);
      return h < 0.3 ? tone('wingBoneDark') : h > 0.75 ? tone('wingBoneLight') : tone('wingBone');
    };
    tube(skin, [root, root.clone().lerp(elbow, 0.5), elbow, elbow.clone().lerp(wrist, 0.5), wrist], [0.2, 0.16, 0.17, 0.13, 0.15], 7, random, bone, wrist.clone().addScaledVector(wrist.clone().sub(elbow).normalize(), 0.1), root.clone().add(v(-side * 0.05, -0.08, 0)));
    const fingers = [leading, [wrist, wrist.clone().lerp(tips[1], 0.45), tips[1]]];
    this.tips = [];
    fingers.forEach((finger, i) => {
      const tip = finger[finger.length - 1];
      const end = tip.clone().addScaledVector(tip.clone().sub(finger[finger.length - 2]).normalize(), 0.16);
      this.tips.push(end.clone().sub(root));
      const radius = [0.1, 0.075][i];
      const radii = finger.map((_, k) => radius * (1 - (0.7 * k) / (finger.length - 1)));
      tube(skin, finger, radii, 5, random, bone, end);
    });
    claw(skin, wrist.clone().add(v(0, 0.06, 0.04)), v(THUMB.x * side, THUMB.y, THUMB.z), 0.34, 0.06, 0.45, random, blend('hornLight', 'claw', 0.4));

    armScales(skin, arm, onBody, outward, random);
    shard(gems, root.clone().add(v(side * 0.05, 0.08, -0.05)), v(side * 0.45, 0.8, -0.4), 0.42, 0.075, random, CRYSTAL);

    this.add(...partMeshes(root, palette, [
      [sheet, materials.membrane],
      [skin, materials.skin],
      [gems, materials.crystal],
    ]));
  }

  // Turns it `angle` rad about its hinge, out and down from as it stands
  // (< 0 raises it higher), and `sweep` rad more forward about the upright
  // through its root (< 0 back).
  lower(angle: number, sweep = 0): void {
    this.quaternion.setFromAxisAngle(this.hinge, this.side * angle);
    if (sweep) this.quaternion.premultiply(_sweep.setFromAxisAngle(_up, -this.side * sweep));
  }
}

const _up = new THREE.Vector3(0, 1, 0);
const _sweep = new THREE.Quaternion();

// One panel of the membrane, from the wrist out between two bones (each a
// polyline from the wrist) to its trailing edge (from the first bone's end
// to the second's), in flat facets of a few creams, bulging out a little.
function membrane(sheet: Sculpt, a: readonly V[], b: readonly V[], edge: readonly V[], outward: V): void {
  const columns = edge.length - 1;
  const start = edge[0];
  const end = edge[columns];
  const normal = new THREE.Vector3().crossVectors(start.clone().sub(a[0]), end.clone().sub(a[0])).normalize();
  if (normal.dot(outward) < 0) normal.negate();
  const grid: V[][] = [];
  for (let r = 0; r <= ROWS; r++) {
    const t = r / ROWS;
    const pa = along(a, t);
    const pb = along(b, t);
    grid.push(
      Array.from({ length: columns + 1 }, (_, c) => {
        const s = c / columns;
        const straight = start.clone().lerp(end, s);
        const p = pa.clone().lerp(pb, s).addScaledVector(edge[c].clone().sub(straight), t);
        return p.addScaledVector(normal, BILLOW * Math.sin(Math.PI * t) * Math.sin(Math.PI * s));
      }),
    );
  }
  const paint = (r: number, c: number, half: number): Tone => {
    const h = hash(r * 7 + half, c, 23);
    return h < 0.3 ? tone('membraneLight') : h > 0.72 ? tone('membraneShade') : blend('membrane', 'membraneLight', 0.25);
  };
  for (let r = 0; r < ROWS; r++) {
    for (let c = 0; c < columns; c++) {
      const [p, q, s, t] = [grid[r][c], grid[r][c + 1], grid[r + 1][c + 1], grid[r + 1][c]];
      if (r === 0) {
        // Every point of the first row is the wrist.
        sheet.toward(p, s, t, normal, paint(r, c, 0));
        continue;
      }
      sheet.toward(p, q, s, normal, paint(r, c, 0));
      sheet.toward(p, s, t, normal, paint(r, c, 1));
    }
  }
}

// Big leaf scales lying on the membrane along the lower half of the arm, in
// two overlapping rows, pointing down and back across it, as the overview
// draws them where the wing meets the body; bigger toward the root.
function armScales(skin: Sculpt, arm: readonly [V, V, V], onBody: V, outward: V, random: () => number): void {
  const [wrist, elbow, root] = arm;
  // The inner membrane's face, out from the body, and the way across it.
  const normal = new THREE.Vector3().crossVectors(root.clone().sub(wrist), onBody.clone().sub(wrist)).normalize();
  if (normal.dot(outward) < 0) normal.negate();
  const across = onBody.clone().sub(elbow).normalize();
  const rows = [
    { out: 0.06, ts: [0.62, 0.74, 0.86, 0.96], size: [0.5, 0.62] },
    { out: 0.32, ts: [0.68, 0.8, 0.92], size: [0.42, 0.52] },
  ];
  rows.forEach((row, r) => {
    for (const t of row.ts) {
      const at = along(arm, t);
      const size = THREE.MathUtils.lerp(row.size[0], row.size[1], (t - 0.6) / 0.4) * (0.92 + 0.16 * random());
      leaf(skin, {
        base: at.addScaledVector(across, row.out).addScaledVector(normal, 0.04 + 0.03 * r),
        along: across.clone().addScaledVector(v(0, 1, 0), 0.1),
        normal,
        length: size,
        width: size * 0.56,
        lift: 0.06,
        curl: 0.05,
        sink: 0.02,
        contrast: 0.6,
        top: r === 1 && random() < 0.4 ? blend('scaleLight', 'leaf', 0.3) : bodyScaleTone(random),
      });
    }
  });
}
