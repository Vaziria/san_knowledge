import * as THREE from 'three';
import type { Sculpt, Tone } from './parts';

// The explorer's own sculpting helpers (Explorer.ts), beyond the forest
// animals' toolkit (parts.ts): rings given by their corners, so a slice can
// be deeper in front than behind or dip at a hem; bands joining them; and
// shapes laid onto a sculpted surface (the eyes, brows and mouth on the
// face) by casting each corner onto it.
//
// Everything is traced from the explorer's sheet in its own pixels (U m
// each): the front view's 430 px from its soles to the top of its hair's
// tuft are 1.2 m. `x` is to the figure's right as the codebase names it
// (the viewer's right in the sheet's front view, +x), `y` up from the
// ground and `z` forward.

export type V3 = THREE.Vector3;

export const HEIGHT = 1.2; // m, soles to the tip of the hair's tuft
export const U = HEIGHT / 430; // m per pixel of the sheet's front view

export function P(x: number, y: number, z: number): V3 {
  return new THREE.Vector3(x * U, y * U, z * U);
}

const TAU = Math.PI * 2;

// A horizontal slice (a head's, a torso's): at height `y`, centred on `x`
// and `z`, `w` to either side (`wb` at the back when it differs, the width
// blending round from front to back), `f` in front and `b` behind, rounded
// as a superellipse (`n` 2 an ellipse, over 2 toward a box). `drop(angle)`
// lowers a corner (px): a hem cut to a point. Angles run from the front
// (+z) toward +x.
export interface Slice {
  y: number;
  z: number;
  x?: number;
  w: number;
  wb?: number;
  f: number;
  b: number;
  n?: number;
  drop?: (angle: number) => number;
}

export function slice(sl: Slice, sides: number, turn = 0): V3[] {
  return Array.from({ length: sides }, (_, k) => slicePoint(sl, turn + (TAU * k) / sides));
}

// The point of a slice at an angle round from the front toward +x (m).
export function slicePoint(sl: Slice, a: number): V3 {
  const e = 2 / (sl.n ?? 2);
  const s = Math.sin(a);
  const c = Math.cos(a);
  const w = sl.wb === undefined ? sl.w : sl.w + ((sl.wb - sl.w) * (1 - c)) / 2;
  const x = (sl.x ?? 0) + w * Math.sign(s) * Math.abs(s) ** e;
  const z = sl.z + (c >= 0 ? sl.f : sl.b) * Math.sign(c) * Math.abs(c) ** e;
  return P(x, sl.y - (sl.drop?.(a) ?? 0), z);
}

// The point at height `y` (px) and angle `a` of a stack of slices (listed
// from the top down), between the two slices round it.
export function stackPoint(slices: readonly Slice[], a: number, y: number): V3 {
  let i = 0;
  while (i < slices.length - 2 && y < slices[i + 1].y) i++;
  const upper = slices[i];
  const lower = slices[i + 1];
  const t = THREE.MathUtils.clamp((upper.y - y) / (upper.y - lower.y), 0, 1);
  return slicePoint(upper, a).lerp(slicePoint(lower, a), t);
}

// A ring square to an axis: round `centre` (m) in the plane of the unit
// vectors `ax` and `ay`, corner k at turn + 2πk/sides from +ay toward +ax,
// `w` along ax, `up` toward +ay and `down` toward -ay (px).
export function ring(centre: V3, ax: V3, ay: V3, w: number, up: number, down: number, sides: number, n = 2, turn = 0): V3[] {
  const e = 2 / n;
  return Array.from({ length: sides }, (_, k) => {
    const a = turn + (TAU * k) / sides;
    const s = Math.sin(a);
    const c = Math.cos(a);
    return centre
      .clone()
      .addScaledVector(ax, w * U * Math.sign(s) * Math.abs(s) ** e)
      .addScaledVector(ay, (c >= 0 ? up : down) * U * Math.sign(c) * Math.abs(c) ** e);
  });
}

// A ring round the y axis at height y (px), for parts built standing along
// it (an arm, a leg): `w` across x, `f` toward +z and `b` toward -z.
export function yRing(y: number, w: number, f: number, b: number, sides: number, n = 2, turn = 0, x = 0, z = 0): V3[] {
  return slice({ y, x, z, w, f, b, n }, sides, turn);
}

// The angle round from the front (or a ring's +ay) of the middle of face k
// of a ring of `sides` corners made with the usual half-side turn.
export function faceAngle(k: number, sides: number): number {
  return ((k + 1) * TAU) / sides;
}

export function centroid(points: readonly V3[]): V3 {
  const c = new THREE.Vector3();
  for (const p of points) c.add(p);
  return c.divideScalar(points.length);
}

// How a run of bands ends: open, closed flat, or closed to a point.
export type End = 'open' | 'flat' | V3;

// Flat faces between rings of equal corner counts, each turned to face away
// from the middle of its band, split one way and the other in turn as the
// sheet's facets run. `paint(r, k)` colours the face from ring r to r + 1
// between corners k and k + 1 (r is -1 for the first ring's cap and the last
// ring's index for the last's).
export function bands(s: Sculpt, rings: readonly V3[][], paint: (r: number, k: number) => Tone, start: End = 'open', end: End = 'open'): void {
  const count = rings.length;
  const sides = rings[0].length;
  const centres = rings.map(centroid);
  // Each band faces away from a point halfway between its own middle and
  // the middle of the whole: a band that turns in flat (a cuff's rim, a
  // hem) has its own middle in its plane, and the whole's is inside.
  const whole = centroid(centres);
  const mid = new THREE.Vector3();
  for (let r = 0; r + 1 < count; r++) {
    mid.copy(centres[r]).add(centres[r + 1]).multiplyScalar(0.5).lerp(whole, 0.5);
    for (let k = 0; k < sides; k++) {
      const k1 = (k + 1) % sides;
      const a = rings[r][k];
      const b = rings[r][k1];
      const c = rings[r + 1][k1];
      const d = rings[r + 1][k];
      const t = paint(r, k);
      if ((r + k) % 2 === 0) {
        s.facing(a, b, c, mid, t);
        s.facing(a, c, d, mid, t);
      } else {
        s.facing(a, b, d, mid, t);
        s.facing(b, c, d, mid, t);
      }
    }
  }
  const cap = (r: number, style: End, inner: V3, index: number) => {
    if (style === 'open') return;
    const centre = centres[r];
    const tip = style === 'flat' ? centre : style;
    const inside = style === 'flat' ? inner : centre.clone().lerp(inner, 0.35);
    for (let k = 0; k < sides; k++) s.facing(rings[r][k], rings[r][(k + 1) % sides], tip, inside, paint(index, k));
  };
  if (count > 1) {
    cap(0, start, centres[1], -1);
    cap(count - 1, end, centres[count - 2], count - 1);
  }
}

// Where things are laid on a surface: `right` and `up` along it, `out` away
// from it (unit vectors), round `origin` (m).
export interface Frame {
  origin: V3;
  right: V3;
  up: V3;
  out: V3;
}

// A point of a surface's frame (u, v in px) laid onto the surface `onto`:
// cast along -out from well in front of it, lifted `lift` m off it. Where
// the cast misses, it stays on the frame's plane.
export function lay(onto: Sculpt, f: Frame, u: number, v: number, lift: number): V3 {
  const on = f.origin.clone().addScaledVector(f.right, u * U).addScaledVector(f.up, v * U);
  const from = on.clone().addScaledVector(f.out, 0.1);
  const way = f.out.clone().negate();
  const t = onto.cast(from, way);
  const hit = Number.isFinite(t) ? from.addScaledVector(way, t) : on;
  return hit.addScaledVector(f.out, lift);
}

// A flat shape laid on a surface: its outline (px in the frame, in order
// round), filled as a fan from its middle through a ring halfway out, so
// that it hugs a curved, faceted surface.
export function decal(s: Sculpt, onto: Sculpt, f: Frame, outline: readonly (readonly [number, number])[], t: Tone, lift: number): void {
  let cu = 0;
  let cv = 0;
  for (const [u, v] of outline) {
    cu += u / outline.length;
    cv += v / outline.length;
  }
  const middle = lay(onto, f, cu, cv, lift);
  const inner = outline.map(([u, v]) => lay(onto, f, cu + (u - cu) * 0.5, cv + (v - cv) * 0.5, lift));
  const outer = outline.map(([u, v]) => lay(onto, f, u, v, lift));
  const n = outline.length;
  for (let k = 0; k < n; k++) {
    const k1 = (k + 1) % n;
    s.toward(middle, inner[k], inner[k1], f.out, t);
    s.toward(inner[k], outer[k], outer[k1], f.out, t);
    s.toward(inner[k], outer[k1], inner[k1], f.out, t);
  }
}

// An ellipse's outline, `rx` by `ry` px round (cu, cv), `n` corners.
export function oval(cu: number, cv: number, rx: number, ry: number, n = 12, from = 0, to = TAU): [number, number][] {
  const closed = Math.abs(to - from - TAU) < 1e-9;
  const count = closed ? n : n + 1;
  return Array.from({ length: count }, (_, k) => {
    const a = from + ((to - from) * k) / n;
    return [cu + rx * Math.cos(a), cv + ry * Math.sin(a)] as [number, number];
  });
}

// A line laid on a surface: `width(share)` px wide along its points (px in
// the frame), from its first point (share 0) to its last (1).
export function strip(s: Sculpt, onto: Sculpt, f: Frame, points: readonly (readonly [number, number])[], width: (share: number) => number, t: Tone, lift: number): { left: V3[]; right: V3[] } {
  const n = points.length;
  const left: V3[] = [];
  const right: V3[] = [];
  for (let i = 0; i < n; i++) {
    const [pu, pv] = points[Math.max(0, i - 1)];
    const [nu, nv] = points[Math.min(n - 1, i + 1)];
    let du = nu - pu;
    let dv = nv - pv;
    const l = Math.hypot(du, dv) || 1;
    du /= l;
    dv /= l;
    const w = width(n > 1 ? i / (n - 1) : 0) / 2;
    const [u, v] = points[i];
    left.push(lay(onto, f, u - dv * w, v + du * w, lift));
    right.push(lay(onto, f, u + dv * w, v - du * w, lift));
  }
  for (let i = 0; i + 1 < n; i++) {
    s.toward(left[i], right[i], right[i + 1], f.out, t);
    s.toward(left[i], right[i + 1], left[i + 1], f.out, t);
  }
  return { left, right };
}

// A strip standing `thick` m proud of the surface, with walls round it (a
// brow): it can be turned and moved a little on the surface and still show.
export function slab(s: Sculpt, onto: Sculpt, f: Frame, points: readonly (readonly [number, number])[], width: (share: number) => number, t: Tone, side: Tone, lift: number, thick: number): void {
  const top = strip(s, onto, f, points, width, t, lift + thick);
  const n = points.length;
  const low = (p: V3) => p.clone().addScaledVector(f.out, -thick);
  const wall = (a: V3, b: V3, way: V3) => {
    const a0 = low(a);
    const b0 = low(b);
    s.toward(a, b, b0, way, side);
    s.toward(a, b0, a0, way, side);
  };
  const along = (i: number) => top.right[Math.min(n - 1, i + 1)].clone().sub(top.right[Math.max(0, i - 1)]).normalize();
  for (let i = 0; i + 1 < n; i++) {
    const out = new THREE.Vector3().crossVectors(along(i), f.out).normalize();
    wall(top.right[i], top.right[i + 1], out);
    wall(top.left[i + 1], top.left[i], out.clone().negate());
  }
  const first = top.left[0].clone().sub(top.left[1]).normalize();
  wall(top.left[0], top.right[0], first);
  const last = top.left[n - 1].clone().sub(top.left[n - 2]).normalize();
  wall(top.right[n - 1], top.left[n - 1], last);
}

// A box `hx` by `hy` by `hz` px (half sizes) round `centre` (m), along the
// unit vectors x, y, z; each face painted by the axis it faces.
export function box(s: Sculpt, centre: V3, x: V3, y: V3, z: V3, hx: number, hy: number, hz: number, paint: (face: 'x' | '-x' | 'y' | '-y' | 'z' | '-z') => Tone): void {
  const corner = (i: number, j: number, k: number) =>
    centre
      .clone()
      .addScaledVector(x, i * hx * U)
      .addScaledVector(y, j * hy * U)
      .addScaledVector(z, k * hz * U);
  const quad = (a: V3, b: V3, c: V3, d: V3, t: Tone) => {
    s.facing(a, b, c, centre, t);
    s.facing(a, c, d, centre, t);
  };
  const c = (i: number, j: number, k: number) => corner(i, j, k);
  quad(c(1, -1, -1), c(1, 1, -1), c(1, 1, 1), c(1, -1, 1), paint('x'));
  quad(c(-1, -1, -1), c(-1, -1, 1), c(-1, 1, 1), c(-1, 1, -1), paint('-x'));
  quad(c(-1, 1, -1), c(-1, 1, 1), c(1, 1, 1), c(1, 1, -1), paint('y'));
  quad(c(-1, -1, -1), c(1, -1, -1), c(1, -1, 1), c(-1, -1, 1), paint('-y'));
  quad(c(-1, -1, 1), c(1, -1, 1), c(1, 1, 1), c(-1, 1, 1), paint('z'));
  quad(c(-1, -1, -1), c(-1, 1, -1), c(1, 1, -1), c(1, -1, -1), paint('-z'));
}

// Moves a sculpted geometry from the figure's space into a joint's, whose
// place in the figure (standing as built, unturned) is `at` (m).
export function into(geometry: THREE.BufferGeometry, at: V3): THREE.BufferGeometry {
  return geometry.applyMatrix4(new THREE.Matrix4().makeTranslation(-at.x, -at.y, -at.z));
}
