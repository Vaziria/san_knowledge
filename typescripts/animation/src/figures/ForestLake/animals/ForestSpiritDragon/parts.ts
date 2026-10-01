import * as THREE from 'three';
import { VIVID } from '../../parts';
import { blend, bone, glowing, Sculpt, skinnedMesh, tone, weighAlong, type Palette, type Tone } from '../parts';

// What the forest spirit dragon's parts share (ForestSpiritDragon.md). It is
// built as the user's ten reference sheets draw it ("Forest Spirit Dragon:
// Majestic, Friendly, Low Poly, Game Ready", kept in
// docs/references/forest_spirit_dragon/): low poly, every face flat, a
// body of overlapping leaf-shaped scales in greens and yellows over cream,
// tan horns like branching wood, and blue crystals that glow. Its colours
// are the sheets' own, picked from their pictures, not the theme's.
//
// Its geometry is sculpted with the forest lake animals' Sculpt (flat
// triangles, each with a colour role of the palette), so that it can be
// painted again in another of the sheets' colour variations.

// Colour roles to sRGB, as picked from the sheets (the head, horn, snout and
// eye sheets): the default, "Forest".
export const FOREST: Palette = {
  scale: 0xa3c266, // the leaf scales' green
  scaleLight: 0xc9d870, // yellow-green scales, the nose
  scaleDark: 0x6b8c52, // the deeper green among them
  leaf: 0xe8e38a, // pale yellow-green highlights
  gold: 0xe6c67a, // golden cheek and frill scales
  cream: 0xf6dcb4, // the muzzle, jaw, throat and belly
  creamShade: 0xdcbf98,
  horn: 0xe8b869, // the horns, like pale wood
  hornLight: 0xf2cf8b,
  hornDark: 0xa6784b,
  ear: 0xddb173,
  earInner: 0xb4824f,
  crystal: 0x52c8fa, // the crystals' cyan
  crystalLight: 0xb3f2f9,
  crystalDeep: 0x2f9be6,
  frost: 0x5ab6d6, // the blue scales among the leaves
  iris: 0xffcf55, // gold, lighter in the middle
  irisMid: 0xf3a42a,
  irisRim: 0x9a5a1c,
  pupil: 0x140d08,
  lid: 0x3a2418, // the dark line round the eye, and the mouth line
  sclera: 0xead2c4, // pale, a little pink at the corners
  glint: 0xffffff,
  nostril: 0x3b2a22,
  mouth: 0x8e3a3a, // inside the mouth
  // The body's (the overview's and the neck sheet's): the cream plates of
  // the throat, chest, belly and the underside of the tail, a little warmer
  // than the muzzle's cream, and the lines between them.
  plate: 0xf8d8a8,
  plateLight: 0xfde7bd,
  plateShade: 0xe8c294,
  seam: 0xe4c296,
  // The wings, as the overview draws them: a cream membrane on olive green
  // bones.
  membrane: 0xf1d09c,
  membraneLight: 0xf8dfb2,
  membraneShade: 0xd9b585,
  wingBone: 0x7f9852,
  wingBoneDark: 0x5d7243,
  wingBoneLight: 0xa6b766,
  // Ivory claws on cream toes.
  claw: 0xf7ead0,
  clawShade: 0xd8c09a,
};

// ---------------------------------------------------------------- materials

// Its scales, skin, horns and lids: matte, coloured by the faces' tones and
// flat shaded, their colours kept vivid in the shade as the sheets paint
// them (the forest lake's VIVID), not dulled toward grey.
export function skinMaterial(): THREE.MeshStandardMaterial {
  const material = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.85, metalness: 0 });
  material.onBeforeCompile = (shader) => {
    shader.fragmentShader = shader.fragmentShader.replace('#include <opaque_fragment>', VIVID);
  };
  material.customProgramCacheKey = () => 'forest-spirit-dragon-skin';
  material.name = 'skin';
  return material;
}

// Glossy, and glowing a little in each face's own colour (`glow`): the
// eyes, which the sheets paint lit from within, amber even on the side away
// from the sun, and the crystals.
function glossyGlow(name: string, roughness: number, metalness: number, glow: number): THREE.MeshStandardMaterial {
  const material = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness, metalness, emissive: 0xffffff, emissiveIntensity: glow });
  material.onBeforeCompile = (shader) => {
    shader.fragmentShader = shader.fragmentShader.replace('#include <emissivemap_fragment>', '#include <emissivemap_fragment>\n totalEmissiveRadiance *= vColor.rgb;');
  };
  material.customProgramCacheKey = () => `forest-spirit-dragon-${name}`;
  material.name = name;
  return material;
}

export function eyeMaterial(): THREE.MeshStandardMaterial {
  return glossyGlow('eye', 0.22, 0, 0.3);
}

// The crystals: glossy, and glowing a little in each face's own colour (the
// sheets' crystals are lit from within, and glow at night).
export function crystalMaterial(): THREE.MeshStandardMaterial {
  return glossyGlow('crystal', 0.18, 0.05, 0.3);
}

// The wings' membrane: matte, seen from both sides and casting its shadow
// from both, and glowing a little in its own cream, as light comes through
// it: the sheets paint it cream even on its shaded side.
export function membraneMaterial(): THREE.MeshStandardMaterial {
  const material = glossyGlow('membrane', 0.85, 0, 0.28);
  material.side = THREE.DoubleSide;
  material.shadowSide = THREE.DoubleSide;
  return material;
}

// The materials every part of it shares: made once for the dragon.
export interface Materials {
  skin: THREE.MeshStandardMaterial;
  membrane: THREE.MeshStandardMaterial;
  crystal: THREE.MeshStandardMaterial;
  eye: THREE.MeshStandardMaterial;
  glint: THREE.MeshBasicMaterial;
}

export function dragonMaterials(): Materials {
  return { skin: skinMaterial(), membrane: membraneMaterial(), crystal: crystalMaterial(), eye: eyeMaterial(), glint: glowing() };
}

// ---------------------------------------------------------------- variety

// A number in 0..1 for a pair of whole numbers, the same every time: which
// facets of a surface are a little lighter or darker.
export function hash(a: number, b: number, c = 0): number {
  const h = Math.sin(a * 12.9898 + b * 78.233 + c * 37.719) * 43758.5453;
  return h - Math.floor(h);
}

// One of `tones` at random, by their weights.
export function pick<T>(random: () => number, choices: readonly (readonly [T, number])[]): T {
  const total = choices.reduce((sum, [, w]) => sum + w, 0);
  let r = random() * total;
  for (const [choice, w] of choices) {
    r -= w;
    if (r <= 0) return choice;
  }
  return choices[choices.length - 1][0];
}

export function shaded(t: Tone, factor: number): Tone {
  return { ...t, shade: (t.shade ?? 1) * factor };
}

// ---------------------------------------------------------------- surfaces

// How a face is put down: its corners, a point inside the solid it bounds
// (so it faces away from it), and its colour.
export type Face = (a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3, inside: THREE.Vector3, tone: Tone) => void;

// Rings of corners, all the same count, joined by flat faces facing away
// from the line through their middles, the quads split one way and the
// other in turn, as the sheets' facets run both ways. Closed to `start`
// before the first ring and to `end` after the last, when given. `paint(r,
// k)` colours the faces from ring r to r + 1 between corners k and k + 1
// (r is -1 for the start's cap and the last ring's index for the end's).
export function ringLoft(face: Face, rings: readonly THREE.Vector3[][], paint: (r: number, k: number) => Tone, start?: THREE.Vector3, end?: THREE.Vector3): void {
  const count = rings[0].length;
  const centres = rings.map((ring) => ring.reduce((sum, p) => sum.add(p), new THREE.Vector3()).divideScalar(ring.length));
  const middle = new THREE.Vector3();
  for (let r = 0; r + 1 < rings.length; r++) {
    middle.copy(centres[r]).add(centres[r + 1]).multiplyScalar(0.5);
    for (let k = 0; k < count; k++) {
      const k1 = (k + 1) % count;
      const a = rings[r][k];
      const b = rings[r][k1];
      const c = rings[r + 1][k1];
      const d = rings[r + 1][k];
      const t = paint(r, k);
      if ((r + k) % 2 === 0) {
        face(a, b, c, middle, t);
        face(a, c, d, middle, t);
      } else {
        face(a, b, d, middle, t);
        face(b, c, d, middle, t);
      }
    }
  }
  const cap = (ring: THREE.Vector3[], centre: THREE.Vector3, tip: THREE.Vector3, r: number) => {
    // Inside: the ring's middle drawn a little back from the tip.
    const inside = centre.clone().lerp(tip, -0.2);
    for (let k = 0; k < count; k++) face(ring[k], ring[(k + 1) % count], tip, inside, paint(r, k));
  };
  if (start) cap(rings[0], centres[0], start, -1);
  if (end) cap(rings[rings.length - 1], centres[rings.length - 1], end, rings.length - 1);
}

// A surface's triangles, kept to find where a ray meets it: to seat eyes,
// scales, horns and crystals on the head, whatever its facets.
export class Probe {
  private readonly corners: number[] = [];

  add(a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3): void {
    this.corners.push(a.x, a.y, a.z, b.x, b.y, b.z, c.x, c.y, c.z);
  }

  // Where a ray from `from` going `way` first meets the surface, and the
  // face's normal there turned back toward the ray's start; null if it
  // meets none.
  hit(from: THREE.Vector3, way: THREE.Vector3): { point: THREE.Vector3; normal: THREE.Vector3 } | null {
    const w = way.clone().normalize();
    const p = this.corners;
    const e1 = new THREE.Vector3();
    const e2 = new THREE.Vector3();
    const pv = new THREE.Vector3();
    const tv = new THREE.Vector3();
    const qv = new THREE.Vector3();
    let best = Infinity;
    const normal = new THREE.Vector3();
    for (let i = 0; i < p.length; i += 9) {
      e1.set(p[i + 3] - p[i], p[i + 4] - p[i + 1], p[i + 5] - p[i + 2]);
      e2.set(p[i + 6] - p[i], p[i + 7] - p[i + 1], p[i + 8] - p[i + 2]);
      pv.crossVectors(w, e2);
      const det = e1.dot(pv);
      if (Math.abs(det) < 1e-14) continue;
      tv.set(from.x - p[i], from.y - p[i + 1], from.z - p[i + 2]);
      const u = tv.dot(pv) / det;
      if (u < 0 || u > 1) continue;
      qv.crossVectors(tv, e1);
      const v = w.dot(qv) / det;
      if (v < 0 || u + v > 1) continue;
      const t = e2.dot(qv) / det;
      if (t > 0 && t < best) {
        best = t;
        normal.crossVectors(e1, e2).normalize();
      }
    }
    if (!Number.isFinite(best)) return null;
    if (normal.dot(w) > 0) normal.negate();
    return { point: from.clone().addScaledVector(w, best), normal };
  }

  // How many times a ray from `from` going `way` crosses the surface: odd
  // when `from` is inside a closed one.
  crossings(from: THREE.Vector3, way: THREE.Vector3): number {
    const w = way.clone().normalize();
    const p = this.corners;
    const e1 = new THREE.Vector3();
    const e2 = new THREE.Vector3();
    const pv = new THREE.Vector3();
    const tv = new THREE.Vector3();
    const qv = new THREE.Vector3();
    let count = 0;
    for (let i = 0; i < p.length; i += 9) {
      e1.set(p[i + 3] - p[i], p[i + 4] - p[i + 1], p[i + 5] - p[i + 2]);
      e2.set(p[i + 6] - p[i], p[i + 7] - p[i + 1], p[i + 8] - p[i + 2]);
      pv.crossVectors(w, e2);
      const det = e1.dot(pv);
      if (Math.abs(det) < 1e-14) continue;
      tv.set(from.x - p[i], from.y - p[i + 1], from.z - p[i + 2]);
      const u = tv.dot(pv) / det;
      if (u < 0 || u > 1) continue;
      qv.crossVectors(tv, e1);
      const v = w.dot(qv) / det;
      if (v < 0 || u + v > 1) continue;
      if (e2.dot(qv) / det > 0) count++;
    }
    return count;
  }

  // Whether a point is inside the closed surface.
  inside(point: THREE.Vector3): boolean {
    return this.crossings(point, INSIDE_RAY) % 2 === 1;
  }
}

// A way no edge of a lofted surface runs along, for inside().
const INSIDE_RAY = new THREE.Vector3(0.123, 0.987, 0.101).normalize();

// A closed shell's faces in a probe, its ends closed to their middles: to
// ask whether a point is inside it.
export function shellProbe(sh: Shell): Probe {
  const probe = new Probe();
  const { rings, centres } = sh;
  const n = rings[0].length;
  for (let r = 0; r + 1 < rings.length; r++) {
    for (let k = 0; k < n; k++) {
      const k1 = (k + 1) % n;
      probe.add(rings[r][k], rings[r][k1], rings[r + 1][k1]);
      probe.add(rings[r][k], rings[r + 1][k1], rings[r + 1][k]);
    }
  }
  for (const r of [0, rings.length - 1]) {
    for (let k = 0; k < n; k++) probe.add(rings[r][k], rings[r][(k + 1) % n], centres[r]);
  }
  return probe;
}

// A part's meshes, sculpted in the dragon's space and moved into the
// part's own, its origin at `at`; none for a sculpt left empty.
export function partMeshes(at: THREE.Vector3, palette: Palette, list: readonly (readonly [Sculpt, THREE.Material])[]): THREE.Mesh[] {
  const meshes: THREE.Mesh[] = [];
  for (const [sculpt, material] of list) {
    if (sculpt.triangles === 0) continue;
    const mesh = new THREE.Mesh(sculpt.geometry(palette).translate(-at.x, -at.y, -at.z), material);
    mesh.castShadow = true;
    mesh.receiveShadow = true;
    meshes.push(mesh);
  }
  return meshes;
}

// A part that bends (the neck, the tail, a leg): a chain of bones through
// its joints, the first at the part's origin, each the child of the one
// before, so turning one turns everything past it.
export interface Chain {
  joints: THREE.Vector3[]; // in the dragon's space, as it stands
  bones: THREE.Bone[];
  skeleton: THREE.Skeleton;
}

// Its bones, made in `part` (its origin at `joints[0]`, standing as built,
// not yet placed on the dragon), and their skeleton bound as they stand.
export function chainOf(part: THREE.Object3D, name: string, joints: readonly THREE.Vector3[]): Chain {
  const at = joints[0];
  const bones: THREE.Bone[] = [];
  joints.forEach((joint, i) => {
    bones.push(bone(`${name}${i}`, i === 0 ? part : bones[i - 1], joint.clone().sub(at), i === 0 ? new THREE.Vector3() : joints[i - 1].clone().sub(at)));
  });
  part.updateMatrixWorld(true);
  return { joints: joints.map((j) => j.clone()), bones, skeleton: new THREE.Skeleton(bones) };
}

// A bending part's meshes, sculpted in the dragon's space and moved into
// the part's own: each corner weighed to the chain's bones by where along
// it it lies (weighAlong: `blend` 'linear' bends all along, as a neck or a
// tail; a number only round the joints, as a leg), or all of it to one bone
// (`rigid`, the bone's index), for what rides on one (the tail's plume).
export function bendingMeshes(chain: Chain, palette: Palette, list: readonly (readonly [Sculpt, THREE.Material, number?])[], blend: number | 'linear'): THREE.SkinnedMesh[] {
  const at = chain.joints[0];
  const joints = chain.joints.map((j) => j.clone().sub(at));
  const indices = chain.bones.map((_, i) => i);
  const meshes: THREE.SkinnedMesh[] = [];
  for (const [sculpt, material, rigid] of list) {
    if (sculpt.triangles === 0) continue;
    const geometry = sculpt.geometry(palette).translate(-at.x, -at.y, -at.z);
    if (rigid === undefined) weighAlong(geometry, joints, indices, blend);
    else weighAlong(geometry, [new THREE.Vector3()], [rigid], blend);
    meshes.push(skinnedMesh(geometry, material, chain.skeleton));
  }
  return meshes;
}

// Turns a chain's bones so each runs the way `ways[i]` says (in the part's
// own space, from its joint toward the next), each by the least turn from
// the way it runs as built: a neck rearing, a tail streaming out behind.
// The last bone carries what is past the chain's end (the head, the plume)
// and is turned to `last`, in the part's space, when given; otherwise it
// keeps its place on the bone before.
export function aim(chain: Chain, ways: readonly THREE.Vector3[], last?: THREE.Quaternion): void {
  const { joints, bones } = chain;
  const parent = new THREE.Quaternion();
  const turn = new THREE.Quaternion();
  const rest = new THREE.Vector3();
  for (let i = 0; i < bones.length; i++) {
    if (i < joints.length - 1) {
      rest.subVectors(joints[i + 1], joints[i]).normalize();
      turn.setFromUnitVectors(rest, ways[i].clone().normalize());
    } else {
      turn.copy(last ?? parent);
    }
    bones[i].quaternion.copy(parent).invert().multiply(turn);
    parent.copy(turn);
  }
}

// ---------------------------------------------------------------- scales

export interface Leaf {
  base: THREE.Vector3; // where it grows from
  along: THREE.Vector3; // the way it points, base to tip
  normal: THREE.Vector3; // the way its face looks (made square to `along`)
  length: number;
  width: number;
  lift?: number; // radians its tip rises off the way it points, toward its face (0)
  ridge?: number; // how high its midrib stands over its edges, times its width (0.2); below 0 a hollow
  widest?: number; // where it is widest, a share of its length from its base (0.42)
  sink?: number; // how far its base goes in behind its face (0.012 m)
  curl?: number; // radians its tip half bends further toward its face (0)
  wrap?: number; // m its sides bend back, to lie round a rounded surface (0)
  contrast?: number; // how much lighter and darker its facets are than its colour (1)
  top: Tone;
  under?: Tone; // its back, the top's colour a shade darker by default
}

// A leaf-shaped scale, as the sheets draw every scale of the dragon: a
// pointed lance widest a little past its base, its midrib standing up, so
// each half is two facets that catch the light differently, closed behind.
export function leaf(s: Sculpt, o: Leaf): void {
  const L = o.along.clone().normalize();
  const N = o.normal.clone().addScaledVector(L, -o.normal.dot(L)).normalize();
  if (o.lift) {
    const axis = new THREE.Vector3().crossVectors(L, N).normalize();
    L.applyAxisAngle(axis, o.lift);
    N.applyAxisAngle(axis, o.lift);
  }
  const W = new THREE.Vector3().crossVectors(N, L).normalize();
  const f = o.widest ?? 0.42;
  const base = o.base.clone().addScaledVector(N, -(o.sink ?? 0.012));
  const P = base.clone().addScaledVector(L, f * o.length);
  const tipWay = L.clone();
  if (o.curl) tipWay.applyAxisAngle(W, -o.curl);
  const T = P.clone().addScaledVector(tipWay, (1 - f) * o.length);
  const Pl = P.clone().addScaledVector(W, -o.width / 2).addScaledVector(N, -(o.wrap ?? 0));
  const Pr = P.clone().addScaledVector(W, o.width / 2).addScaledVector(N, -(o.wrap ?? 0));
  const M = P.clone().addScaledVector(N, (o.ridge ?? 0.2) * o.width);
  // One half lit and the other dim, and the tip lighter than the base, so
  // that a row of them reads as leaves over the shade of the row before.
  const k = o.contrast ?? 1;
  const facet = (factor: number) => shaded(o.top, 1 + (factor - 1) * k);
  s.toward(base, Pr, M, N, facet(0.96));
  s.toward(M, Pr, T, N, facet(1.12));
  s.toward(base, M, Pl, N, facet(0.8));
  s.toward(M, T, Pl, N, facet(0.94));
  const back = N.clone().negate();
  const under = o.under ?? facet(0.82);
  s.toward(base, Pl, T, back, under);
  s.toward(base, T, Pr, back, under);
}

// A big leaf folded along its midrib into a hollow (the ears): its rims in
// `rim`, the hollow between them in `hollow`, its back in `back`.
export function foldedLeaf(s: Sculpt, o: Omit<Leaf, 'top' | 'under' | 'ridge'> & { depth: number; rim: Tone; hollow: Tone; back: Tone }): void {
  const L = o.along.clone().normalize();
  const N = o.normal.clone().addScaledVector(L, -o.normal.dot(L)).normalize();
  if (o.lift) {
    const axis = new THREE.Vector3().crossVectors(L, N).normalize();
    L.applyAxisAngle(axis, o.lift);
    N.applyAxisAngle(axis, o.lift);
  }
  const W = new THREE.Vector3().crossVectors(N, L).normalize();
  const f = o.widest ?? 0.4;
  const base = o.base.clone().addScaledVector(N, -(o.sink ?? 0.02));
  const P = base.clone().addScaledVector(L, f * o.length);
  const tipWay = L.clone();
  if (o.curl) tipWay.applyAxisAngle(W, -o.curl);
  const T = P.clone().addScaledVector(tipWay, (1 - f) * o.length);
  const Pl = P.clone().addScaledVector(W, -o.width / 2);
  const Pr = P.clone().addScaledVector(W, o.width / 2);
  // The hollow's floor along the midrib, sunk, and the rims' inner edges.
  const H = P.clone().addScaledVector(N, -o.depth * o.width).addScaledVector(L, 0.05 * o.length);
  const Il = P.clone().addScaledVector(W, -o.width * 0.2).addScaledVector(N, -o.depth * o.width * 0.4);
  const Ir = P.clone().addScaledVector(W, o.width * 0.2).addScaledVector(N, -o.depth * o.width * 0.4);
  // Rims, the outer strips either side.
  s.toward(base, Pr, Ir, N, shaded(o.rim, 1.04));
  s.toward(Ir, Pr, T, N, shaded(o.rim, 1.04));
  s.toward(base, Il, Pl, N, shaded(o.rim, 0.95));
  s.toward(Il, T, Pl, N, shaded(o.rim, 0.95));
  // The hollow.
  s.toward(base, Ir, H, N, o.hollow);
  s.toward(H, Ir, T, N, shaded(o.hollow, 1.06));
  s.toward(base, H, Il, N, shaded(o.hollow, 0.92));
  s.toward(H, T, Il, N, o.hollow);
  // Its back, bulging behind the hollow.
  const B = P.clone().addScaledVector(N, -(o.depth + 0.12) * o.width);
  const back = N.clone().negate();
  s.toward(base, Pl, B, back, o.back);
  s.toward(B, Pl, T, back, shaded(o.back, 0.92));
  s.toward(base, B, Pr, back, shaded(o.back, 1.04));
  s.toward(B, T, Pr, back, o.back);
}

// ---------------------------------------------------------------- crystals

export interface CrystalTones {
  face: Tone;
  light: Tone;
  deep: Tone;
}

// A crystal shard, as the sheets draw them: a prism of `sides` uneven faces
// standing from its `base` along `axis`, a quarter of its length sunk
// below it, closed in a point a little off its middle, its facets in light,
// mid and deep blues.
export function shard(s: Sculpt, base: THREE.Vector3, axis: THREE.Vector3, length: number, radius: number, random: () => number, tones: CrystalTones, sides = 5, shoulder = 0.5): void {
  const a = axis.clone().normalize();
  const x = Math.abs(a.x) > 0.9 ? new THREE.Vector3(0, 1, 0) : new THREE.Vector3(1, 0, 0);
  x.addScaledVector(a, -x.dot(a)).normalize();
  const y = new THREE.Vector3().crossVectors(a, x);
  const spin = random() * Math.PI * 2;
  const ring = (at: THREE.Vector3, r: number) =>
    Array.from({ length: sides }, (_, k) => {
      const angle = spin + (Math.PI * 2 * (k + (random() - 0.5) * 0.3)) / sides;
      const rr = r * (0.82 + 0.36 * random());
      return at.clone().addScaledVector(x, Math.cos(angle) * rr).addScaledVector(y, Math.sin(angle) * rr);
    });
  const bottom = ring(base.clone().addScaledVector(a, -0.25 * length), radius * 0.85);
  const shoulderAt = base.clone().addScaledVector(a, shoulder * length);
  const top = ring(shoulderAt, radius);
  const tip = base
    .clone()
    .addScaledVector(a, length)
    .addScaledVector(x, (random() - 0.5) * radius * 0.5)
    .addScaledVector(y, (random() - 0.5) * radius * 0.5);
  const inside = base.clone().addScaledVector(a, shoulder * length * 0.4);
  const toneOf = () => pick(random, [
    [tones.face, 0.45],
    [tones.light, 0.3],
    [tones.deep, 0.25],
  ] as const);
  for (let k = 0; k < sides; k++) {
    const k1 = (k + 1) % sides;
    const side = toneOf();
    s.facing(bottom[k], bottom[k1], top[k1], inside, side);
    s.facing(bottom[k], top[k1], top[k], inside, shaded(side, 0.96));
    s.facing(top[k], top[k1], tip, shoulderAt, random() < 0.5 ? tones.light : toneOf());
  }
}

// ---------------------------------------------------------------- horns

export interface HornPath {
  root: THREE.Vector3;
  start: THREE.Vector3; // the way it leaves its root
  end: THREE.Vector3; // the way it points at its tip
  length: number;
}

// Where along a horn's curve a share `t` of its length is: it turns
// steadily from its start's way to its end's.
export function hornAt(h: HornPath, t: number): THREE.Vector3 {
  const d0 = h.start.clone().normalize();
  const d1 = h.end.clone().normalize();
  return h.root
    .clone()
    .addScaledVector(d0, h.length * t)
    .addScaledVector(d1.sub(d0), (h.length * t * t) / 2);
}

// A horn as the sheets draw them, like a branch of pale wood cut in
// facets: `sides` uneven faces round a curve, thinning from `radius` at its
// root (sunk a little into the head) to a point.
export function horn(s: Sculpt, h: HornPath, radius: number, random: () => number, paint: (ring: number, k: number) => Tone, sides = 6, steps = 5): void {
  const ts = [-0.08, ...Array.from({ length: steps }, (_, i) => (i / steps) * 0.92)];
  const path = ts.map((t) => hornAt(h, t));
  const tip = hornAt(h, 1);
  const tangents = path.map((_, i) => (i + 1 < path.length ? path[i + 1] : tip).clone().sub(path[Math.max(0, i - 1)]).normalize());
  // A sideways axis carried along the curve without twisting.
  let x = new THREE.Vector3(0, 1, 0).cross(tangents[0]);
  if (x.lengthSq() < 1e-8) x = new THREE.Vector3(1, 0, 0);
  x.normalize();
  const spin = random() * Math.PI * 2;
  const rings = path.map((at, i) => {
    if (i > 0) {
      const axis = new THREE.Vector3().crossVectors(tangents[i - 1], tangents[i]);
      if (axis.lengthSq() > 1e-12) x.applyAxisAngle(axis.normalize(), Math.acos(THREE.MathUtils.clamp(tangents[i - 1].dot(tangents[i]), -1, 1)));
      x.addScaledVector(tangents[i], -x.dot(tangents[i])).normalize();
    }
    const y = new THREE.Vector3().crossVectors(tangents[i], x);
    // Thick most of its length, as the sheets' horns are, then drawn to its point.
    const r = radius * (1 - 0.8 * Math.max(0, ts[i]) ** 1.4);
    return Array.from({ length: sides }, (_, k) => {
      const angle = spin + (Math.PI * 2 * k) / sides + (random() - 0.5) * 0.25;
      const rr = r * (0.86 + 0.28 * random());
      return at.clone().addScaledVector(x, Math.cos(angle) * rr).addScaledVector(y, Math.sin(angle) * rr);
    });
  });
  const face: Face = (a, b, c, inside, t) => s.facing(a, b, c, inside, t);
  ringLoft(face, rings, paint, undefined, tip);
}

// ---------------------------------------------------------------- scale colours

const TAU = Math.PI * 2;

// A scale's colour: mostly the leaf green, some yellow-green, some deeper,
// a few pale.
export function scaleTone(random: () => number): Tone {
  const role = pick(random, [
    ['scale', 4],
    ['scaleLight', 2.5],
    ['scaleDark', 1.5],
    ['leaf', 0.6],
  ] as const);
  return random() < 0.4 ? blend(role, 'scaleLight', random() * 0.3) : tone(role);
}

// The body's scales, as the overview paints them: the head's green, each a
// little lighter or deeper than the next, now and then a yellow-green or a
// deeper one, and a few gold ones among them.
export function bodyScaleTone(random: () => number): Tone {
  const r = random();
  if (r < 0.03) return blend('gold', 'scaleLight', 0.3 + 0.3 * random());
  if (r < 0.2) return blend('scaleLight', 'leaf', 0.25 * random());
  if (r < 0.32) return blend('scale', 'scaleDark', 0.5 + 0.3 * random());
  return random() < 0.5 ? blend('scale', 'scaleLight', 0.3 * random()) : blend('scale', 'scaleDark', 0.25 * random());
}

export const CRYSTAL: CrystalTones = { face: tone('crystal'), light: tone('crystalLight'), deep: tone('crystalDeep') };

// ---------------------------------------------------------------- shells

// A lofted surface: its rings, each the same count of corners round its
// middle, corner 0 at angle 0 (the spine, on the body) and the rest going
// round toward +x first; and their middles.
export interface Shell {
  rings: THREE.Vector3[][];
  centres: THREE.Vector3[];
}

export function shellOf(rings: THREE.Vector3[][]): Shell {
  return { rings, centres: rings.map((ring) => ring.reduce((sum, p) => sum.add(p), new THREE.Vector3()).divideScalar(ring.length)) };
}

// A point just past a shell's first ring (`end` false) or its last, along
// it: to close a root sunk in another part with a shallow cap, so the part
// shown alone (?part=) is closed.
export function lid(sh: Shell, end: boolean, depth = 0.04): THREE.Vector3 {
  const c = sh.centres;
  const [a, b] = end ? [c[c.length - 1], c[c.length - 2]] : [c[0], c[1]];
  return a.clone().addScaledVector(a.clone().sub(b).normalize(), depth);
}

// A ring of `count` corners round `at`, in the plane of `x` (its side) and
// `y` (the way to corner 0): `w` to either side, `up` toward corner 0 and
// `down` away from it, rounded as a superellipse (`n` 2 an ellipse, over 2
// boxier).
export function sectionRing(at: THREE.Vector3, x: THREE.Vector3, y: THREE.Vector3, w: number, up: number, down: number, count: number, n = 2): THREE.Vector3[] {
  const e = 2 / n;
  return Array.from({ length: count }, (_, k) => {
    const angle = (TAU * k) / count;
    const s = Math.sin(angle);
    const c = Math.cos(angle);
    return at
      .clone()
      .addScaledVector(x, Math.sign(s) * Math.abs(s) ** e * w)
      .addScaledVector(y, Math.sign(c) * Math.abs(c) ** e * (c >= 0 ? up : down));
  });
}

// A point of a shell, its normal out of it, the ways along it (toward its
// later rings) and round it (toward growing angle) on its surface, and how
// many meters a ring along or a radian round cover there.
export interface OnShell {
  point: THREE.Vector3;
  normal: THREE.Vector3;
  along: THREE.Vector3;
  round: THREE.Vector3;
  perU: number;
  perRadian: number;
}

// The point `u` along a shell (0 at its first ring, 1 at its second, ...)
// and `angle` round it (radians from corner 0), between the corners either
// side: for seating scales, plates and spikes on it.
export function shellAt(sh: Shell, u: number, angle: number): OnShell {
  const { rings, centres } = sh;
  const n = rings[0].length;
  const i = THREE.MathUtils.clamp(Math.floor(u), 0, rings.length - 2);
  const g = THREE.MathUtils.clamp(u - i, 0, 1);
  const a = ((((angle / TAU) * n) % n) + n) % n;
  const j = Math.floor(a) % n;
  const f = a - Math.floor(a);
  const j1 = (j + 1) % n;
  const [p00, p01, p10, p11] = [rings[i][j], rings[i][j1], rings[i + 1][j], rings[i + 1][j1]];
  const lo = p00.clone().lerp(p01, f);
  const hi = p10.clone().lerp(p11, f);
  const point = lo.clone().lerp(hi, g);
  const along = hi.clone().sub(lo);
  const round = p01.clone().sub(p00).lerp(p11.clone().sub(p10), g).multiplyScalar(n / TAU);
  const centre = centres[i].clone().lerp(centres[i + 1], g);
  const normal = new THREE.Vector3().crossVectors(round, along).normalize();
  if (normal.dot(point.clone().sub(centre)) < 0) normal.negate();
  const perU = along.length();
  const perRadian = round.length();
  return { point, normal, along: along.normalize(), round: round.normalize(), perU, perRadian };
}

// How a shell is covered in leaf scales (cover()).
export interface Cover {
  from: number; // u of its first row
  to: number; // u past which it stops
  half: (u: number) => number; // radians either side of angle 0 a row at u reaches
  length: (u: number, angle: number) => number; // a leaf's length there
  tone: (u: number, angle: number) => Tone | null; // its colour there, or none there
  way: 1 | -1; // they point toward its later rings, or its earlier ones
  width?: number; // a leaf's width, times its length (0.55)
  along?: number; // the rows' spacing, times a leaf's length (0.5)
  across?: number; // the leaves' spacing in a row, times a leaf's width (0.85)
  droop?: number; // how far they also point down its sides, away from angle 0 (0.3)
  lift?: number;
  curl?: number;
  ridge?: number;
  sink?: number;
  widest?: number; // where a leaf is widest, a share of its length (0.48: a diamond of a plate)
  contrast?: number; // how strongly its facets differ (0.45)
  skip?: (point: THREE.Vector3) => boolean; // none here (inside another part)
}

// A leaf scale draped over a shell rather than laid flat on it, so it
// follows the surface wherever that curves away under it (a leg, the tip of
// the tail): its root `sink` m under the surface at (u, angle), its widest
// point `widest` of its length on, `rise` m off the surface, its sides there
// `side` m off, its tip `tip` m off, its midrib standing `ridge` of its width
// over its widest point. It points along the shell (`way`) and `droop` as
// far again round it, away from angle 0.
export interface Drape {
  u: number;
  angle: number;
  way: 1 | -1;
  droop: number;
  length: number;
  width: number;
  widest: number;
  sink: number;
  rise: number;
  side: number;
  tip: number;
  ridge: number;
  contrast: number;
  top: Tone;
}

export function drape(s: Sculpt, sh: Shell, o: Drape): void {
  const at = shellAt(sh, o.u, o.angle);
  // Its way in meters along and round the shell, and across it.
  const sign = Math.sign(o.angle) || 1;
  const k = 1 / Math.hypot(1, o.droop);
  const way = [o.way * k, o.droop * sign * k] as const;
  const point = (along: number, across: number, lift: number) => {
    const u = o.u + (way[0] * along - way[1] * across) / Math.max(1e-6, at.perU);
    const angle = o.angle + (way[1] * along + way[0] * across) / Math.max(1e-6, at.perRadian);
    const p = shellAt(sh, u, angle);
    return { at: p.point.addScaledVector(p.normal, lift), normal: p.normal };
  };
  const base = point(0, 0, -o.sink).at;
  const middle = point(o.widest * o.length, 0, o.rise);
  const left = point(o.widest * o.length, -o.width / 2, o.side).at;
  const right = point(o.widest * o.length, o.width / 2, o.side).at;
  const tip = point(o.length, 0, o.tip).at;
  const N = middle.normal;
  const M = middle.at.clone().addScaledVector(N, o.ridge * o.width);
  // Which side is lit and which dim, as leaf() shades them, so a row of
  // them reads as leaves.
  const facet = (factor: number) => shaded(o.top, 1 + (factor - 1) * o.contrast);
  const [Pr, Pl] = new THREE.Vector3().crossVectors(right.clone().sub(base), tip.clone().sub(base)).dot(N) < 0 ? [right, left] : [left, right];
  s.toward(base, Pr, M, N, facet(0.96));
  s.toward(M, Pr, tip, N, facet(1.12));
  s.toward(base, M, Pl, N, facet(0.8));
  s.toward(M, tip, Pl, N, facet(0.94));
  const back = N.clone().negate();
  s.toward(base, Pl, tip, back, facet(0.82));
  s.toward(base, tip, Pr, back, facet(0.82));
}

// Covers a shell in rows of leaf scales draped over it like shingles, each
// row over the roots of the next, from angle 0 out to either side, every
// other row set half a leaf round, as the sheets' scales lie.
export function cover(s: Sculpt, sh: Shell, random: () => number, c: Cover): void {
  const along = c.along ?? 0.5;
  const across = c.across ?? 0.85;
  const widthOf = c.width ?? 0.55;
  const droop = c.droop ?? 0.3;
  const place = (u: number, angle: number, du: number, da: number) => {
    const t = c.tone(u, angle);
    if (!t) return;
    const ju = u + (random() - 0.5) * 0.3 * du;
    const ja = angle + (random() - 0.5) * 0.3 * da;
    if (c.skip?.(shellAt(sh, ju, ja).point)) return;
    const length = c.length(u, angle) * (0.9 + 0.2 * random());
    const lift = c.lift ?? 0.1;
    drape(s, sh, {
      u: ju,
      angle: ja,
      way: c.way,
      droop: droop + (random() - 0.5) * 0.12,
      length,
      width: length * widthOf,
      widest: c.widest ?? 0.48,
      sink: c.sink ?? 0.02,
      rise: 0.012 + lift * length * 0.48,
      side: 0.006 + lift * length * 0.2,
      tip: 0.02 + (lift + (c.curl ?? 0.05)) * length * 0.75,
      ridge: c.ridge ?? 0.18,
      contrast: c.contrast ?? 0.45,
      top: t,
    });
  };
  let u = c.from;
  let row = 0;
  while (u <= c.to) {
    const half = c.half(u);
    const size = c.length(u, 0);
    const du = (size * along) / Math.max(1e-6, shellAt(sh, u, 0).perU);
    const step = (a: number) => (c.length(u, a) * widthOf * across) / Math.max(1e-6, shellAt(sh, u, a).perRadian);
    let angle = row % 2 === 0 ? 0 : step(0) / 2;
    while (angle <= half) {
      const da = step(angle);
      for (const side of angle < 1e-9 ? [1] : [1, -1]) place(u, side * angle, du, da);
      angle += da;
    }
    u += du;
    row++;
  }
}

// A plate lying on a shell, as the sheets' cream plates of the throat,
// chest, belly and tail: from u0 to u1 along it and a0 to a1 round it, set
// in from those edges by `gap` of its size so the seams between plates show,
// its middle domed `bulge` m off the surface. `vee` draws its edge at u1
// (over 0) or at u0 (under 0) out to a shallow point, that share of its
// length further, standing `lip` m prouder than its other edge, as the
// neck's plates overlap downward. With `random`, each corner is drawn in
// toward its middle by up to `uneven` of the way, so the plates are uneven
// flagstones rather than tiles.
export function plate(
  s: Sculpt,
  sh: Shell,
  u0: number,
  u1: number,
  a0: number,
  a1: number,
  t: Tone,
  o: { gap?: number; bulge?: number; vee?: number; rim?: number; lip?: number; random?: () => number; uneven?: number } = {},
): void {
  const gap = o.gap ?? 0.07;
  const du = (u1 - u0) * gap;
  const da = (a1 - a0) * gap;
  const [U0, U1, A0, A1] = [u0 + du, u1 - du, a0 + da, a1 - da];
  const am = (A0 + A1) / 2;
  const um = (U0 + U1) / 2;
  const vee = (U1 - U0) * (o.vee ?? 0);
  const rim = o.rim ?? 0.012;
  const lip = o.lip ?? 0;
  const bulge = o.bulge ?? 0.03;
  // A grid over it, a column every 0.18 m or so round the shell, so that
  // it follows the surface's curve rather than cutting under it; its edges
  // just off the surface, its middle domed, its pointed edge proud.
  const columns = Math.max(2, Math.ceil(((A1 - A0) * shellAt(sh, um, am).perRadian) / 0.18));
  const inward = Array.from({ length: 2 * columns + 4 }, () => (o.random ? (o.uneven ?? 0.15) * o.random() : 0));
  const grid = [0, 0.5, 1].map((t, row) =>
    Array.from({ length: columns + 1 }, (_, c) => {
      const s = c / columns;
      const edgeRow = row !== 1;
      const edgeColumn = c === 0 || c === columns;
      // Toward the pointed edge, the middle of it reaches further.
      const point = 1 - Math.abs(2 * s - 1);
      let u = THREE.MathUtils.lerp(U0, U1, t);
      if (vee > 0 && row === 2) u += vee * point;
      if (vee < 0 && row === 0) u += vee * point;
      let a = THREE.MathUtils.lerp(A0, A1, s);
      // Uneven: an edge point drawn in toward the middle.
      if (edgeRow || edgeColumn) {
        const k = edgeRow ? (row === 0 ? c : columns + 1 + c) : 2 * columns + 2 + (c === 0 ? 0 : 1);
        u += (um - u) * inward[k] * 0.5;
        a += (am - a) * inward[k];
      }
      const proud = (vee > 0 && row === 2) || (vee < 0 && row === 0);
      const dome = edgeRow || edgeColumn ? 0 : bulge * Math.sin(Math.PI * s);
      const p = shellAt(sh, u, a);
      return { at: p.point.addScaledVector(p.normal, rim + (proud ? lip : 0) + dome), normal: p.normal };
    }),
  );
  for (let r = 0; r < 2; r++) {
    for (let c = 0; c < columns; c++) {
      const [p, q, w, x] = [grid[r][c], grid[r][c + 1], grid[r + 1][c + 1], grid[r + 1][c]];
      const n = p.normal.clone().add(w.normal);
      s.toward(p.at, q.at, w.at, n, t);
      s.toward(p.at, w.at, x.at, n, t);
    }
  }
}

// ---------------------------------------------------------------- spikes and tubes

// A back spike, as the back spike sheet takes them apart: a crystal
// standing up along `axis` from a cup of three leaves, green and yellow,
// on the scales, `length` m tall.
export function backSpike(skin: Sculpt, gems: Sculpt, base: THREE.Vector3, axis: THREE.Vector3, length: number, random: () => number, slim = 1): void {
  const a = axis.clone().normalize();
  shard(gems, base, a, length, length * 0.26 * slim, random, CRYSTAL, 5, 0.5);
  const x = Math.abs(a.x) > 0.9 ? new THREE.Vector3(0, 1, 0) : new THREE.Vector3(1, 0, 0);
  x.addScaledVector(a, -x.dot(a)).normalize();
  const y = new THREE.Vector3().crossVectors(a, x);
  const spin = random() * TAU;
  const cup = [tone('gold'), blend('scaleLight', 'leaf', 0.3), tone('scale')];
  for (let i = 0; i < 3; i++) {
    const phi = spin + (TAU * i) / 3;
    const out = x.clone().multiplyScalar(Math.cos(phi)).addScaledVector(y, Math.sin(phi));
    const size = length * (0.42 + 0.12 * random());
    leaf(skin, {
      base: base.clone().addScaledVector(out, length * 0.07).addScaledVector(a, -length * 0.06),
      along: a.clone().addScaledVector(out, 0.5),
      normal: out,
      length: size,
      width: size * 0.42,
      curl: 0.15,
      sink: 0.01,
      top: cup[i],
    });
  }
}

// A tube through `points`, `radii[i]` round at each, its sides a little
// uneven, closed to `tip` past its last point and to `start` before its
// first when given: the wings' bones, the toes, the claws. Returns its
// rings.
export function tube(
  s: Sculpt,
  points: readonly THREE.Vector3[],
  radii: readonly number[],
  sides: number,
  random: () => number,
  paint: (ring: number, k: number) => Tone,
  tip?: THREE.Vector3,
  start?: THREE.Vector3,
  jitter = 0.12,
): THREE.Vector3[][] {
  const last = points.length - 1;
  const tangents = points.map((_, i) => points[Math.min(last, i + 1)].clone().sub(points[Math.max(0, i - 1)]).normalize());
  let x = new THREE.Vector3(0, 1, 0).cross(tangents[0]);
  if (x.lengthSq() < 1e-8) x = new THREE.Vector3(1, 0, 0).cross(tangents[0]);
  x.normalize();
  const spin = random() * TAU;
  const rings = points.map((at, i) => {
    if (i > 0) {
      const axis = new THREE.Vector3().crossVectors(tangents[i - 1], tangents[i]);
      if (axis.lengthSq() > 1e-12) x.applyAxisAngle(axis.normalize(), Math.acos(THREE.MathUtils.clamp(tangents[i - 1].dot(tangents[i]), -1, 1)));
      x.addScaledVector(tangents[i], -x.dot(tangents[i])).normalize();
    }
    const y = new THREE.Vector3().crossVectors(tangents[i], x);
    return Array.from({ length: sides }, (_, k) => {
      const angle = spin + (TAU * k) / sides + (random() - 0.5) * 2 * jitter;
      const r = radii[i] * (1 - jitter + 2 * jitter * random());
      return at.clone().addScaledVector(x, Math.cos(angle) * r).addScaledVector(y, Math.sin(angle) * r);
    });
  });
  const face: Face = (a, b, c, inside, t) => s.facing(a, b, c, inside, t);
  ringLoft(face, rings, paint, start, tip);
  return rings;
}

// A claw: a curved, faceted point from `root` along `way`, bending down by
// `hook` radians over its length, `radius` round at its root.
export function claw(s: Sculpt, root: THREE.Vector3, way: THREE.Vector3, length: number, radius: number, hook: number, random: () => number, t: Tone): void {
  const w = way.clone().normalize();
  const side = new THREE.Vector3(0, 1, 0).cross(w);
  if (side.lengthSq() < 1e-8) side.set(1, 0, 0);
  side.normalize();
  const steps = 3;
  const points = [root.clone()];
  const dir = w.clone();
  for (let i = 1; i <= steps; i++) {
    dir.applyAxisAngle(side, hook / steps);
    points.push(points[i - 1].clone().addScaledVector(dir, length / (steps + 1)));
  }
  const tip = points[steps].clone().addScaledVector(dir.applyAxisAngle(side, hook / steps), length / (steps + 1));
  const radii = points.map((_, i) => radius * (1 - i / (steps + 1)));
  tube(s, points, radii, 5, random, (ring) => (ring < 1 ? shaded(t, 0.95) : t), tip, root.clone().addScaledVector(w, -radius * 0.5), 0.05);
}
