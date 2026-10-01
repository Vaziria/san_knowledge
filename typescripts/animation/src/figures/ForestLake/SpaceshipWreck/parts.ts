import * as THREE from 'three';
import { addBar, addBlock, addHull, addLathe, addTube, between, color, frame, shade, Shape, vary } from '../parts';

// What the spaceship wreck's parts share (SpaceshipWreck.md). The wreck is
// built as the user's reference sheets draw it ("Spaceship Wreck", its
// cockpit, hull, engine and tail modules, and its debris and small parts):
// low poly and chunky, every face flat shaded, its hull a skin of plates,
// warm off-white with dark grey ones among them and orange ones in patches,
// spotted and edged with rust, over a dark frame that shows between them and
// wherever a plate is gone; inside, walls and floors lit warm by its lamps.
// Its colours are picked from those sheets, as the forest lake's are from
// its own; what grows round it is the forest lake's (../parts.ts).
//
// Everything is BufferGeometry made in code, through the forest lake's
// Shape (a colour and a flat normal at each corner of each triangle).

// sRGB, as picked from the reference sheets.
export const WRECK = {
  plate: 0xe9e1dc, // the hull's plates: a warm off-white
  metal: 0x57555f, // frames, rings and the dark plates
  metalLight: 0x85838c,
  metalDark: 0x3a393f,
  gap: 0x29282d, // between the plates and under them
  orange: 0xed7532, // painted plates and stripes
  orangeDeep: 0xc9561f,
  rust: 0xb9652d, // rust spots and worn edges
  rustDark: 0x7f4022,
  inside: 0x93837a, // the walls, floor and things inside, before the lamps' light is baked in
  insideDark: 0x564b47,
  lamp: 0xffb65a, // a lamp's warm light
  lampCore: 0xfff1c9,
  screen: 0x4fc8f2, // the cockpit's screens
  screenLine: 0xb8f0ff,
  glass: 0xa9d6ea, // the windshield's broken glass
  window: 0x2f3d4b, // a porthole's dark glass
  glow: 0xff8a2b, // the engine's core, still warm
  glowCore: 0xffdc90,
  navigation: 0xff7a1f, // the tail's navigation light
  cable: 0x2d2c33,
  sleeve: 0xe8742c, // the cables' orange sleeves
  drum: 0xc8432f, // a fuel drum
  seat: 0x4f4b57,
  seatPad: 0x6d6775,
} as const;

// A colour of the wreck, as three.js keeps colours (linear).
export function tone(key: keyof typeof WRECK): THREE.Color {
  return color(WRECK[key]);
}

const UP = new THREE.Vector3(0, 1, 0);

// x ** p keeping its sign: for superellipses.
export function signedPow(x: number, p: number): number {
  return Math.sign(x) * Math.abs(x) ** p;
}

// A point of a squarish round section (a superellipse, rounder as `n`
// nears 2), `ry` high and `rz` wide either side of its middle, at angle `a`
// from +z toward +y: [y, z].
export function section(a: number, ry: number, rz: number, n: number): [number, number] {
  return [ry * signedPow(Math.sin(a), 2 / n), rz * signedPow(Math.cos(a), 2 / n)];
}

// ---------------------------------------------------------------------------
// Plates.

export interface PanelOptions {
  thick?: number; // m it stands off the frame; 0.05
  gap?: number; // m between it and the next; 0.05
  chamfer?: number; // m off its top edges
  rust?: number; // rust spots per m² of it; 2.5
  worn?: number; // the chance each corner is rusted over; 0.18
  feature?: 'vent' | 'window' | null; // a vent's slats or a porthole on it
  bend?: { hinge: number; angle: number } | null; // torn up about its edge `hinge` (corner hinge to hinge + 1) by `angle` radians
  under?: boolean; // its underside drawn too: a loose plate
}

// A plate of the hull over the cell `corners` (four, counter-clockwise seen
// from outside): its top face `base`'s colour, a little lighter or darker,
// its corners now and then rusted over, spotted with rust; its sides a
// shade darker, cut off at its top edges so they catch the light; `gap`
// narrower than the cell all round, so the frame shows dark between plates.
// A vent is a dark grille with slats across it, a porthole dark glass with
// a glint.
export function addPanel(shape: Shape, corners: readonly THREE.Vector3[], base: THREE.Color, random: () => number, options: PanelOptions = {}): void {
  const thick = options.thick ?? 0.05;
  const gap = options.gap ?? 0.05;
  const chamfer = options.chamfer ?? Math.min(0.03, thick * 0.6);
  const n = new THREE.Vector3().crossVectors(corners[1].clone().sub(corners[0]), corners[3].clone().sub(corners[0]));
  if (n.lengthSq() < 1e-8) return;
  n.normalize();
  const inset = (points: readonly THREE.Vector3[], by: number) =>
    points.map((p, k) => {
      const next = points[(k + 1) % 4].clone().sub(p).normalize();
      const prev = points[(k + 3) % 4].clone().sub(p).normalize();
      return p.clone().addScaledVector(next, by).addScaledVector(prev, by);
    });
  const b = inset(corners, gap / 2);
  const t = inset(b, chamfer).map((p) => p.addScaledVector(n, thick));
  if (options.bend) {
    // Turned about its hinge the way that lifts its free edge off the hull.
    const { hinge, angle } = options.bend;
    const a = b[hinge].clone();
    const axis = b[(hinge + 1) % 4].clone().sub(a).normalize();
    const free = b[(hinge + 2) % 4].clone().add(b[(hinge + 3) % 4]).multiplyScalar(0.5).sub(a);
    const sign = new THREE.Vector3().crossVectors(axis, free).dot(n) > 0 ? 1 : -1;
    const q = new THREE.Quaternion().setFromAxisAngle(axis, sign * angle);
    for (const p of [...b, ...t]) p.sub(a).applyQuaternion(q).add(a);
    n.applyQuaternion(q);
  }
  const rust = tone('rust');
  const top = t.map(() => {
    const c = vary(base, random, 1.2);
    return random() < (options.worn ?? 0.18) ? c.lerp(rust, between(random, 0.2, 0.45)) : c;
  });
  shape.quad(t[0], t[1], t[2], t[3], top[0], top[1], top[2], top[3]);
  const side = shade(base, 0.8);
  for (let k = 0; k < 4; k++) {
    const j = (k + 1) % 4;
    shape.quad(b[k], b[j], t[j], t[k], vary(side, random, 1));
  }
  if (options.bend || options.under) shape.quad(b[0], b[3], b[2], b[1], tone('metalDark'));

  // What is on its face: a point of it at (s, r), 0..1 along its first
  // edge and toward its last corner, lifted off it by `lift`.
  const on = (s: number, r: number, lift: number) =>
    t[0]
      .clone()
      .lerp(t[1], s)
      .lerp(t[3].clone().lerp(t[2], s), r)
      .addScaledVector(n, lift);
  const patch = (s0: number, s1: number, r0: number, r1: number, lift: number, c: THREE.Color) => shape.quad(on(s0, r0, lift), on(s1, r0, lift), on(s1, r1, lift), on(s0, r1, lift), c);
  if (options.feature === 'vent') {
    patch(0.22, 0.78, 0.25, 0.75, 0.004, tone('gap'));
    for (let k = 0; k < 4; k++) {
      const r = 0.3 + k * 0.13;
      patch(0.25, 0.75, r, r + 0.05, 0.012, vary(tone('metalLight'), random, 0.6));
    }
  } else if (options.feature === 'window') {
    patch(0.24, 0.76, 0.22, 0.78, 0.005, tone('metalDark'));
    patch(0.3, 0.7, 0.28, 0.72, 0.011, tone('window'));
    shape.triangle(on(0.34, 0.66, 0.014), on(0.42, 0.66, 0.014), on(0.34, 0.56, 0.014), shade(tone('glass'), 0.9));
  }

  // Spots of rust.
  const area = t[1].distanceTo(t[0]) * t[3].distanceTo(t[0]);
  const spots = Math.floor(area * (options.rust ?? 2.5) + random());
  const e1 = t[1].clone().sub(t[0]).normalize();
  const e2 = new THREE.Vector3().crossVectors(n, e1);
  for (let k = 0; k < spots; k++) {
    const middle = on(between(random, 0.12, 0.88), between(random, 0.12, 0.88), 0.004);
    const r = between(random, 0.03, 0.08);
    const c = vary(random() < 0.6 ? rust : tone('rustDark'), random, 1.5);
    const rim = Array.from({ length: 5 }, (_, i) => {
      const angle = (2 * Math.PI * i) / 5 + random() * 0.5;
      const d = r * between(random, 0.6, 1.2);
      return middle.clone().addScaledVector(e1, Math.cos(angle) * d).addScaledVector(e2, Math.sin(angle) * d);
    });
    for (let i = 0; i < 5; i++) shape.triangle(middle, rim[i], rim[(i + 1) % 5], c);
  }
}

// ---------------------------------------------------------------------------
// Skins: a hull's plates over its frame.

export type CellState = 'panel' | 'bent' | 'hole' | 'open';

export interface SkinOptions {
  rows: number; // plates along it
  columns: number; // plates round it
  at: (u: number, t: number) => THREE.Vector3; // its outer surface: u 0..1 along it, t round it
  middle: (u: number) => THREE.Vector3; // the section's middle at u: which way is out
  from?: number; // how much of the way round it covers, in t: all of it by default
  to?: number;
  state?: (i: number, j: number) => CellState;
  paint: (i: number, j: number, normal: THREE.Vector3) => THREE.Color;
  panel?: (i: number, j: number) => PanelOptions;
  feature?: (i: number, j: number) => 'vent' | 'window' | null;
  wall?: number; // m from its outside to its inside; 0.14
  torn?: readonly [number, number]; // how much of its first and its last row may be torn away, in rows
  random: () => number;
  lining?: THREE.Color; // its inside's colour
}

// A hull's skin, a cell of the grid (u, t) per plate: in `outer`, a dark
// frame under the plates and the plates on it, a plate `bent` torn up off
// it, a `hole` where the plate and the frame are gone but for a spar and a
// rib across it, `open` where the hull is torn wide open; in `inside` (if
// any), its inside, facing in. Wherever the wall ends (round a hole, an
// opening, its torn or its cut ends) its edge shows its thickness. A torn
// end is ragged: each column's last plate falls short by its own amount.
export function addSkin(outer: Shape, inside: Shape | null, o: SkinOptions): void {
  const { rows, columns, random } = o;
  const from = o.from ?? 0;
  const to = o.to ?? 1;
  const wraps = to - from >= 0.999;
  const wall = o.wall ?? 0.14;
  const torn = o.torn ?? [0, 0];
  const cutFront = Array.from({ length: columns }, () => random() * torn[0]);
  const cutBack = Array.from({ length: columns }, () => random() * torn[1]);
  const state = o.state ?? (() => 'panel');
  const stateOf = (i: number, j: number): CellState | null => {
    if (i < 0 || i >= rows) return null;
    if (!wraps && (j < 0 || j >= columns)) return null;
    return state(i, ((j % columns) + columns) % columns);
  };
  const solid = (s: CellState | null) => s === 'panel' || s === 'bent';
  const gapColor = tone('gap');
  const edgeColor = tone('metal');
  const lining = o.lining ?? tone('inside');
  // A point of the inside under a point of the outside at u.
  const within = (p: THREE.Vector3, u: number) => {
    const m = o.middle(u);
    const r = p.distanceTo(m);
    return m.clone().lerp(p, Math.max(0, (r - wall) / r));
  };

  for (let i = 0; i < rows; i++) {
    for (let j = 0; j < columns; j++) {
      const s = stateOf(i, j) as CellState;
      const u0 = (i + (i === 0 ? cutFront[j] : 0)) / rows;
      const u1 = (i + 1 - (i === rows - 1 ? cutBack[j] : 0)) / rows;
      const ta = from + ((to - from) * j) / columns;
      const tb = from + ((to - from) * (j + 1)) / columns;
      const p00 = o.at(u0, ta);
      const p10 = o.at(u1, ta);
      const p11 = o.at(u1, tb);
      const p01 = o.at(u0, tb);
      const mid = p00.clone().add(p10).add(p11).add(p01).multiplyScalar(0.25);
      const um = (u0 + u1) / 2;
      const n = new THREE.Vector3().crossVectors(p10.clone().sub(p00), p01.clone().sub(p00));
      if (n.lengthSq() < 1e-6) continue;
      const flip = n.dot(mid.clone().sub(o.middle(um))) < 0;
      // The corners counter-clockwise seen from outside, each with its u,
      // and for each edge (corner k to k + 1) the cell beyond it.
      const q = flip ? [p00, p01, p11, p10] : [p00, p10, p11, p01];
      const qu = flip ? [u0, u0, u1, u1] : [u0, u1, u1, u0];
      const beyond: [number, number][] = flip
        ? [
            [i - 1, j],
            [i, j + 1],
            [i + 1, j],
            [i, j - 1],
          ]
        : [
            [i, j - 1],
            [i + 1, j],
            [i, j + 1],
            [i - 1, j],
          ];
      const normal = n.normalize().multiplyScalar(flip ? -1 : 1);

      if (s === 'hole') {
        // The frame left across it: a spar along it and a rib round it,
        // halfway into the wall.
        const tm = (ta + tb) / 2;
        const spar = [o.at(u0, tm), o.at(u1, tm)].map((p, k) => within(p, k ? u1 : u0).lerp(p, 0.5));
        addStrut(outer, spar[0], spar[1], normal, 0.08, wall * 0.7, vary(edgeColor, random, 1), random);
        if (random() < 0.6) {
          const rib = [o.at(um, ta), o.at(um, tb)].map((p) => within(p, um).lerp(p, 0.5));
          addStrut(outer, rib[0], rib[1], normal, 0.07, wall * 0.7, vary(edgeColor, random, 1), random);
        }
        continue;
      }
      if (!solid(s)) continue;

      // The frame under the plate, and the inside.
      outer.quad(q[0], q[1], q[2], q[3], gapColor);
      const inner = q.map((p, k) => within(p, qu[k]));
      if (inside) inside.quad(inner[0], inner[3], inner[2], inner[1], vary(lining, random, 1));
      // The wall's edge wherever it ends.
      const ragged = (i === 0 && torn[0] > 0) || (i === rows - 1 && torn[1] > 0);
      for (let k = 0; k < 4; k++) {
        const [bi, bj] = beyond[k];
        const along = bi === i; // an edge between two columns
        if (solid(stateOf(bi, bj)) && !(ragged && along)) continue;
        const a = q[k];
        const b = q[(k + 1) % 4];
        const la = inner[k];
        const lb = inner[(k + 1) % 4];
        const away = a.clone().add(b).multiplyScalar(0.5).sub(mid);
        const face = new THREE.Vector3().crossVectors(b.clone().sub(a), la.clone().sub(a));
        if (face.dot(away) >= 0) outer.quad(a, b, lb, la, vary(edgeColor, random, 1));
        else outer.quad(a, la, lb, b, vary(edgeColor, random, 1));
      }

      // The plate, torn up about an edge it still holds to.
      let bend: PanelOptions['bend'] = null;
      if (s === 'bent') {
        let hinge = beyond.findIndex(([bi, bj]) => stateOf(bi, bj) === 'panel');
        if (hinge < 0) hinge = 0;
        bend = { hinge, angle: between(random, 0.35, 0.95) };
      }
      addPanel(outer, q, o.paint(i, j, normal), random, { ...o.panel?.(i, j), feature: s === 'bent' ? null : (o.feature?.(i, j) ?? null), bend });
    }
  }
}

// ---------------------------------------------------------------------------
// Slabs: a wing, a fin.

export interface SlabOptions {
  root: readonly [THREE.Vector3, THREE.Vector3]; // its leading and trailing edge's points at its root, on its middle
  tip: readonly [THREE.Vector3, THREE.Vector3]; // and at its tip
  normal: THREE.Vector3; // the way its top faces
  thick: readonly [number, number]; // m, at its root and its tip
  rows: number; // plates from root to tip
  columns: number; // plates from its leading edge back
  paint: (i: number, j: number, top: boolean) => THREE.Color;
  state?: (i: number, j: number, top: boolean) => 'panel' | 'bent' | 'hole';
  panel?: PanelOptions;
  torn?: number; // how much of its root row may be torn away, in rows: a wing broken off
  random: () => number;
}

// A flat, tapering slab plated on both faces (a wing, a fin): a dark core,
// its leading edge cut to a blunt wedge, and plates over its top and its
// underside behind that edge, `rows` by `columns`; where a plate is gone the
// core shows. A torn root is ragged.
export function addSlab(shape: Shape, o: SlabOptions): void {
  const { rows, columns, random } = o;
  const n = o.normal.clone().normalize();
  const mid = (s: number, c: number) => o.root[0].clone().lerp(o.root[1], c).lerp(o.tip[0].clone().lerp(o.tip[1], c), s);
  const thick = (s: number) => o.thick[0] + (o.thick[1] - o.thick[0]) * s;
  const LEAD = 0.1; // the share of the chord the leading edge's wedge takes
  const torn = Array.from({ length: columns }, () => random() * (o.torn ?? 0));
  const s0 = o.torn ? Math.max(...torn) / rows : 0;
  // The core, as the hull of its corners: where the wedge meets the
  // faces, the edges, and the tip and root ends.
  const core: THREE.Vector3[] = [];
  for (const s of [s0 * 0.4, 1]) {
    const h = thick(s) / 2;
    core.push(mid(s, 0).addScaledVector(n, h * 0.3), mid(s, 0).addScaledVector(n, -h * 0.3));
    for (const c of [LEAD, 1]) core.push(mid(s, c).addScaledVector(n, h), mid(s, c).addScaledVector(n, -h));
  }
  const dark = tone('metalDark');
  const orange = tone('orange');
  // Its leading edge's wedge orange, the rest dark.
  const forward = o.root[0].clone().sub(o.root[1]);
  addHull(shape, core, (normal) => (Math.abs(normal.dot(n)) < 0.75 && normal.dot(forward) > 0 ? vary(shade(orange, 0.9), random, 1) : vary(dark, random, 1)));
  // The plates on each face.
  for (const top of [true, false]) {
    const side = top ? 1 : -1;
    const surface = (s: number, c: number) => mid(s, c).addScaledVector(n, (side * thick(s)) / 2);
    for (let i = 0; i < rows; i++) {
      for (let j = 0; j < columns; j++) {
        const state = o.state?.(i, j, top) ?? 'panel';
        if (state === 'hole') continue;
        const a0 = (i + (i === 0 ? torn[j] : 0)) / rows;
        const a1 = (i + 1) / rows;
        const c0 = LEAD + ((1 - LEAD) * j) / columns;
        const c1 = LEAD + ((1 - LEAD) * (j + 1)) / columns;
        let q = [surface(a0, c0), surface(a0, c1), surface(a1, c1), surface(a1, c0)];
        const face = new THREE.Vector3().crossVectors(q[1].clone().sub(q[0]), q[3].clone().sub(q[0]));
        if (face.dot(n) * side < 0) q = [q[0], q[3], q[2], q[1]];
        addPanel(shape, q, o.paint(i, j, top), random, { ...o.panel, bend: state === 'bent' ? { hinge: 1, angle: between(random, 0.3, 0.8) } : null });
      }
    }
  }
}

// ---------------------------------------------------------------------------
// Frames, struts, walls.

// A square bar of metal from `from` to `to`: `tall` in the way nearest
// `up`, `thick` the other way, its edges cut off.
export function addStrut(shape: Shape, from: THREE.Vector3, to: THREE.Vector3, up: THREE.Vector3, tall: number, thick: number, c: THREE.Color, random: () => number): void {
  const along = to.clone().sub(from);
  if (along.lengthSq() < 1e-8) return;
  const out = new THREE.Vector3().crossVectors(along, up);
  if (out.lengthSq() < 1e-10) out.set(0, 0, 1).cross(along);
  if (out.lengthSq() < 1e-10) out.set(1, 0, 0);
  const small = Math.min(tall, thick);
  addBar(shape, frame(from, along, out), {
    length: along.length(),
    width: thick,
    height: tall,
    chamfer: small * 0.18,
    bevel: small * 0.1,
    segments: 1,
    colors: { side: c, groove: c, end: shade(c, 1.15), core: c },
    random,
  });
}

// A flat round disc facing `normal`, `radius` across, `sides` corners.
export function addDisc(shape: Shape, center: THREE.Vector3, normal: THREE.Vector3, radius: number, sides: number, c: THREE.Color | ((i: number) => THREE.Color)): void {
  const n = normal.clone().normalize();
  const u = new THREE.Vector3().crossVectors(n, Math.abs(n.y) < 0.9 ? UP : new THREE.Vector3(1, 0, 0)).normalize();
  const v = new THREE.Vector3().crossVectors(n, u);
  const rim = Array.from({ length: sides }, (_, i) => {
    const a = (2 * Math.PI * i) / sides;
    return center.clone().addScaledVector(u, Math.cos(a) * radius).addScaledVector(v, Math.sin(a) * radius);
  });
  const paint = typeof c === 'function' ? c : () => c;
  // u then v round n is counter-clockwise seen from +n.
  for (let i = 0; i < sides; i++) shape.triangle(center, rim[i], rim[(i + 1) % sides], paint(i));
}

// A flat wall with openings in it (a bulkhead with its doorway): `outline`
// and `holes` in x and y, placed by `matrix`, its front facing +z of that
// frame in `front`'s colour and, if given, its back in `back`'s.
export function addWall(shape: Shape, outline: readonly THREE.Vector2[], holes: readonly (readonly THREE.Vector2[])[], matrix: THREE.Matrix4, front: THREE.Color, back: THREE.Color | null): void {
  const contour = THREE.ShapeUtils.isClockWise(outline as THREE.Vector2[]) ? [...outline].reverse() : [...outline];
  const inner = holes.map((h) => (THREE.ShapeUtils.isClockWise(h as THREE.Vector2[]) ? [...h] : [...h].reverse()));
  const all = [...contour, ...inner.flat()];
  const at = all.map((p) => new THREE.Vector3(p.x, p.y, 0).applyMatrix4(matrix));
  for (const [a, b, c] of THREE.ShapeUtils.triangulateShape(contour, inner)) {
    const ccw = (all[b].x - all[a].x) * (all[c].y - all[a].y) - (all[b].y - all[a].y) * (all[c].x - all[a].x) > 0;
    const [i, j, k] = ccw ? [a, b, c] : [a, c, b];
    shape.triangle(at[i], at[j], at[k], front);
    if (back) shape.triangle(at[i], at[k], at[j], back);
  }
}

// A frame whose +z is `facing` and whose y is as near up as that allows,
// at `at`: for walls and screens.
export function facingFrame(at: THREE.Vector3, facing: THREE.Vector3): THREE.Matrix4 {
  const z = facing.clone().normalize();
  const x = new THREE.Vector3().crossVectors(Math.abs(z.y) < 0.95 ? UP : new THREE.Vector3(0, 0, -1), z).normalize();
  const y = new THREE.Vector3().crossVectors(z, x);
  return new THREE.Matrix4().makeBasis(x, y, z).setPosition(at);
}

// ---------------------------------------------------------------------------
// Light inside.

export interface Lamp {
  at: THREE.Vector3;
  light: THREE.Color;
  reach: number; // m it lights well
}

// Bakes lamps into a geometry's corner colours, for the inside of the hull,
// which the sun hardly reaches: each corner's colour times `ambient` plus
// each lamp's light as much as the corner faces it, fading past its reach.
// Drawn unlit (glowing()), it looks lit by those lamps alone, as the
// reference's warm insides are.
export function bakeLamps(geometry: THREE.BufferGeometry, lamps: readonly Lamp[], ambient: THREE.Color): void {
  const position = geometry.getAttribute('position');
  const normal = geometry.getAttribute('normal');
  const tint = geometry.getAttribute('color');
  if (!position || !tint) return;
  const p = new THREE.Vector3();
  const n = new THREE.Vector3();
  const toward = new THREE.Vector3();
  const c = new THREE.Color();
  for (let i = 0; i < position.count; i++) {
    p.fromBufferAttribute(position, i);
    n.fromBufferAttribute(normal, i);
    let r = ambient.r;
    let g = ambient.g;
    let b = ambient.b;
    for (const lamp of lamps) {
      toward.subVectors(lamp.at, p);
      const d = toward.length();
      const facing = Math.max(0, n.dot(toward.divideScalar(Math.max(d, 1e-6)))) * 0.75 + 0.25;
      const fall = 1 / (1 + (d / lamp.reach) ** 2);
      r += lamp.light.r * facing * fall;
      g += lamp.light.g * facing * fall;
      b += lamp.light.b * facing * fall;
    }
    c.fromBufferAttribute(tint as THREE.BufferAttribute, i);
    tint.setXYZ(i, c.r * r, c.g * g, c.b * b);
  }
  tint.needsUpdate = true;
}

// ---------------------------------------------------------------------------
// Cargo and small things.

// Where a point of something turned by `turn` about y and standing at `at`
// is.
export function placed(at: THREE.Vector3, turn: number, x: number, y: number, z: number): THREE.Vector3 {
  return new THREE.Vector3(x, y, z).applyAxisAngle(UP, turn).add(at);
}

// Paint by the way a face turns: `c` lighter on top, darker underneath.
export function facePaint(c: THREE.Color, random: () => number): (normal: THREE.Vector3) => THREE.Color {
  return (normal) => vary(shade(c, normal.y > 0.5 ? 1.12 : normal.y < -0.5 ? 0.7 : 0.92 + normal.x * 0.05), random, 0.8);
}

// A cargo box of the ship's standing at `at`: dark grey, `size` across,
// with orange straps round it and a lighter lid; turned by `turn`, tipped by
// `tip` (about x and z).
export function addCargo(shape: Shape, at: THREE.Vector3, size: THREE.Vector3, turn: number, random: () => number, tip: readonly [number, number] = [0, 0]): void {
  const middle = at.clone().add(new THREE.Vector3(0, size.y / 2, 0));
  const matrix = new THREE.Matrix4().makeRotationFromEuler(new THREE.Euler(tip[0], turn, tip[1], 'YXZ'));
  const off = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z).applyMatrix4(matrix).add(middle);
  const options = { turn, tip, chamfer: Math.min(0.035, size.y * 0.1), rough: 0.004 };
  addBlock(shape, middle, size, facePaint(tone('metal'), random), random, options);
  for (const side of [-1, 1]) {
    addBlock(shape, off(side * size.x * 0.3, 0, 0), new THREE.Vector3(size.x * 0.12, size.y + 0.02, size.z + 0.02), facePaint(tone('orange'), random), random, { ...options, chamfer: 0.01 });
  }
  addBlock(shape, off(0, size.y / 2 + 0.015, 0), new THREE.Vector3(size.x * 0.4, 0.03, size.z * 0.7), facePaint(tone('metalLight'), random), random, { ...options, chamfer: 0.008 });
}

// A wooden crate as the explorer's stack them (the forest lake's own wood):
// pale planks in a darker frame.
export function addWoodCrate(shape: Shape, at: THREE.Vector3, size: number, turn: number, random: () => number): void {
  const middle = at.clone().add(new THREE.Vector3(0, size / 2, 0));
  const plank = color(0xb86d33);
  const post = color(0x7d4522);
  addBlock(shape, middle, new THREE.Vector3(size - 0.04, size - 0.04, size - 0.04), facePaint(plank, random), random, { turn, chamfer: 0.01, rough: 0.004 });
  const h = size / 2 - 0.03;
  for (const [x, z] of [
    [-h, -h],
    [h, -h],
    [h, h],
    [-h, h],
  ]) {
    addBlock(shape, placed(middle, turn, x, 0, z), new THREE.Vector3(0.07, size, 0.07), facePaint(post, random), random, { turn, chamfer: 0.01 });
  }
  for (const y of [-h, h]) {
    for (const [x, z, w, d] of [
      [0, -h, size, 0.07],
      [0, h, size, 0.07],
      [-h, 0, 0.07, size],
      [h, 0, 0.07, size],
    ]) {
      addBlock(shape, placed(middle, turn, x, y, z), new THREE.Vector3(w, 0.07, d), facePaint(post, random), random, { turn, chamfer: 0.01 });
    }
  }
}

// A fuel drum, red with dark rims, `height` tall and `radius` round,
// standing with its foot at the origin of `matrix` (which may lay it down).
export function addDrum(shape: Shape, matrix: THREE.Matrix4, height: number, radius: number, random: () => number): void {
  const red = tone('drum');
  const rim = shade(red, 0.62);
  const h = height;
  const r = radius;
  addLathe(
    shape,
    [
      [0, 0],
      [r * 0.94, 0],
      [r, 0.03],
      [r, h * 0.3],
      [r * 1.04, h * 0.32],
      [r, h * 0.34],
      [r, h * 0.66],
      [r * 1.04, h * 0.68],
      [r, h * 0.7],
      [r, h - 0.03],
      [r * 0.94, h],
      [r * 0.9, h - 0.01],
      [0, h - 0.01],
    ],
    10,
    matrix,
    (ring, i) => (ring === 3 || ring === 4 || ring === 6 || ring === 7 ? rim : ring >= 10 ? shade(red, 1.1) : vary(red, random, i % 3 === 0 ? 1.4 : 0.6)),
  );
}

// A cable lying or hanging along `points` (a smooth curve through them),
// `radius` thick, dark, with an orange sleeve here and there.
export function addCable(shape: Shape, points: readonly THREE.Vector3[], radius: number, random: () => number, sleeves = 2): void {
  const path = smoothPath(points, 0.25);
  const dark = tone('cable');
  addTube(shape, path, path.map(() => radius), { sides: 5, random, paint: () => vary(dark, random, 1.2), caps: [dark, dark] });
  const orange = tone('sleeve');
  for (let k = 0; k < sleeves; k++) {
    const at = Math.floor(between(random, 0.2, 0.8) * (path.length - 2));
    addTube(shape, [path[at], path[at + 1]], [radius * 1.5, radius * 1.5], { sides: 5, random, paint: () => orange, caps: [orange, orange] });
  }
}

// A smooth curve through points (Catmull-Rom), a point about every `step` m.
export function smoothPath(points: readonly THREE.Vector3[], step: number): THREE.Vector3[] {
  const out: THREE.Vector3[] = [];
  const at = (i: number) => points[Math.max(0, Math.min(points.length - 1, i))];
  for (let i = 0; i + 1 < points.length; i++) {
    const [p0, p1, p2, p3] = [at(i - 1), at(i), at(i + 1), at(i + 2)];
    const steps = Math.max(1, Math.ceil(p1.distanceTo(p2) / step));
    for (let k = 0; k < steps; k++) {
      const t = k / steps;
      const t2 = t * t;
      const t3 = t2 * t;
      out.push(
        new THREE.Vector3()
          .addScaledVector(p1, 2)
          .addScaledVector(p2.clone().sub(p0), t)
          .addScaledVector(p0.clone().multiplyScalar(2).addScaledVector(p1, -5).addScaledVector(p2, 4).sub(p3), t2)
          .addScaledVector(p0.clone().negate().addScaledVector(p1, 3).addScaledVector(p2, -3).add(p3), t3)
          .multiplyScalar(0.5),
      );
    }
  }
  out.push(points[points.length - 1].clone());
  return out;
}

// A pilot's seat standing at `at`, facing -x turned by `turn`: a dark
// base, a padded seat and back leaning back, a headrest, arms, orange
// piping; `tipped` lays it over (a seat thrown out of the wreck).
export function addSeat(shape: Shape, at: THREE.Vector3, turn: number, random: () => number, tipped: readonly [number, number] = [0, 0]): void {
  const matrix = new THREE.Matrix4().makeRotationFromEuler(new THREE.Euler(tipped[0], turn, tipped[1], 'YXZ')).setPosition(at);
  const place = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z).applyMatrix4(matrix);
  const block = (x: number, y: number, z: number, size: THREE.Vector3, c: THREE.Color, lean = 0) =>
    addBlock(shape, place(x, y, z), size, facePaint(c, random), random, { turn, tip: [tipped[0], tipped[1] + lean], chamfer: 0.025, rough: 0.003 });
  const dark = tone('seat');
  const pad = tone('seatPad');
  const orange = tone('orange');
  block(0, 0.17, 0, new THREE.Vector3(0.44, 0.34, 0.46), shade(dark, 0.85));
  block(0, 0.4, 0, new THREE.Vector3(0.56, 0.12, 0.56), pad);
  block(0.3, 0.82, 0, new THREE.Vector3(0.13, 0.78, 0.54), pad, -0.18);
  block(0.37, 1.3, 0, new THREE.Vector3(0.12, 0.24, 0.36), dark, -0.18);
  for (const side of [-1, 1]) {
    block(0.02, 0.58, side * 0.31, new THREE.Vector3(0.46, 0.06, 0.08), dark);
    block(0.3, 0.82, side * 0.285, new THREE.Vector3(0.14, 0.7, 0.03), orange, -0.18);
  }
}

// A screen on a wall at `at` facing `facing`, `w` by `h`: its dark bezel in
// `body` and its glowing face, with lines of light on it, in `lit`.
export function addScreen(body: Shape, lit: Shape, at: THREE.Vector3, facing: THREE.Vector3, w: number, h: number, random: () => number): void {
  const m = facingFrame(at, facing);
  const p = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z).applyMatrix4(m);
  const n = facing.clone().normalize();
  addBlock(body, p(0, 0, -0.02), new THREE.Vector3(w + 0.06, h + 0.06, 0.05), facePaint(tone('metalDark'), random), random, {
    turn: Math.atan2(n.x, n.z),
    tip: [-Math.asin(THREE.MathUtils.clamp(n.y, -1, 1)), 0],
    chamfer: 0.01,
    rough: 0,
  });
  const face = tone('screen');
  const deep = color(WRECK.screen).lerp(color(0x1c5b82), 0.5);
  lit.quad(p(-w / 2, -h / 2, 0.01), p(w / 2, -h / 2, 0.01), p(w / 2, h / 2, 0.01), p(-w / 2, h / 2, 0.01), deep, deep, face, face);
  const line = tone('screenLine');
  for (let k = 0; k < 3; k++) {
    const y = -h * 0.3 + k * h * 0.25;
    const x1 = -w * 0.38 + between(random, 0.3, 0.7) * w * 0.76;
    lit.quad(p(-w * 0.38, y, 0.014), p(x1, y, 0.014), p(x1, y + h * 0.06, 0.014), p(-w * 0.38, y + h * 0.06, 0.014), line);
  }
}
