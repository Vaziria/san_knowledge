import * as THREE from 'three';
import { keepDry } from '../../environtments/water';
import { addBeam, addExtrusion, addTube, color, matte, mesh, PALETTE, seededRandom, Shape, shade, snowOn, vary, type Season } from './parts';

export interface RowboatOptions {
  season?: Season; // spring by default; snow on its seats and gunwales in winter
  seed?: number;
}

// The forest lake's rowboat (Rowboat.md; the asset sheet's "Boat"), as its
// asset sheet and the pier draw it: a wooden rowing boat 3 m long, its bow
// (+x) pointed and its stern cut square, built of orange-brown planks
// (strakes) with a pale rim along the top and a keel under it, darker
// inside, with three seats across it, ribs inside and two oars lying in
// it. Its keel is at y 0.
export class Rowboat extends THREE.Group {
  static readonly LENGTH = 3;
  static readonly DRAFT = 0.15; // m of it under the water when afloat

  readonly hull: THREE.Mesh; // its outside, with the keel and rim
  readonly inside: THREE.Mesh; // its inside, seats and ribs
  readonly oars: THREE.Mesh;

  constructor(options: RowboatOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 141);
    const stern = -1.4;
    const thick = 0.04;
    const S = 16; // stations along it
    const T = 10; // strakes round it, five a side

    // Its shape: how wide, how high its sides and how far its keel rises
    // toward its ends, at s (0 the stern, 1 the bow).
    const halfBeam = (s: number) => (s <= 0.45 ? 0.56 - 0.12 * ((0.45 - s) / 0.45) ** 2 : 0.56 * Math.sqrt(Math.max(0, 1 - ((s - 0.45) / 0.55) ** 2)));
    const sheer = (s: number) => (s > 0.4 ? 0.47 + 0.14 * ((s - 0.4) / 0.6) ** 2 : 0.47 + 0.04 * ((0.4 - s) / 0.4) ** 2);
    const rocker = (s: number) => (s > 0.45 ? 0.09 * ((s - 0.45) / 0.55) ** 2 : 0.03 * ((0.45 - s) / 0.45) ** 2);
    // A point on its outside at (s, t): t from -1 (the rim on its -z side)
    // through 0 (the keel) to 1 (the rim on its +z side).
    const outside = (s: number, t: number) => {
      const b = halfBeam(s);
      const lateral = Math.sign(t) * b * Math.sin((Math.abs(t) * Math.PI) / 2) ** 0.6;
      const y = rocker(s) + (sheer(s) - rocker(s)) * Math.abs(t) ** 1.5;
      return new THREE.Vector3(stern + Rowboat.LENGTH * s, y, lateral);
    };
    const inside = (s: number, t: number) => {
      const p = outside(s, t);
      const lateral = Math.sign(p.z) * Math.max(0, Math.abs(p.z) - thick);
      return new THREE.Vector3(p.x + (s === 0 ? thick : 0), Math.abs(t) === 1 ? p.y : p.y + thick, lateral);
    };
    const ss = Array.from({ length: S + 1 }, (_, i) => i / S);
    const ts = Array.from({ length: T + 1 }, (_, j) => -1 + (2 * j) / T);

    // The outside: strakes of alternating shades, paler toward the rim,
    // darker at the bottom.
    const outer = new Shape();
    const wood = color(PALETTE.wood);
    const strake = (j: number) => {
      const fromRim = Math.min(j, T - 1 - j); // 0 at the rim, 4 at the keel
      return shade(wood, (j % 2 ? 1.04 : 0.93) * (1.08 - fromRim * 0.05));
    };
    for (let i = 0; i < S; i++) {
      for (let j = 0; j < T; j++) {
        const c = vary(strake(j), random, 0.5);
        outer.quad(outside(ss[i], ts[j]), outside(ss[i + 1], ts[j]), outside(ss[i + 1], ts[j + 1]), outside(ss[i], ts[j + 1]), c);
      }
    }
    // The rim along the top of each side, pale.
    const rim = shade(color(PALETTE.woodEnd), 0.92);
    for (let i = 0; i < S; i++) {
      for (const t of [-1, 1]) {
        const [o0, o1, n0, n1] = [outside(ss[i], t), outside(ss[i + 1], t), inside(ss[i], t), inside(ss[i + 1], t)];
        if (t > 0) outer.quad(o0, o1, n1, n0, rim);
        else outer.quad(o0, n0, n1, o1, rim);
      }
    }
    // The transom closing the stern, facing aft (-x).
    const transom = ts.map((t) => outside(0, t));
    const middle = transom.reduce((sum, p) => sum.add(p), new THREE.Vector3()).divideScalar(transom.length);
    for (let j = 0; j < T; j++) outer.triangle(middle, transom[j], transom[j + 1], shade(wood, 0.9));
    outer.triangle(middle, transom[T], transom[0], shade(wood, 0.9));
    // The keel under it and the stem up its bow.
    const keel = ss.slice(1, S).map((s) => outside(s, 0).add(new THREE.Vector3(0, -0.015, 0)));
    addTube(outer, keel, keel.map(() => 0.03), { sides: 5, random, paint: () => shade(wood, 0.8) });
    const stem = [0, 0.3, 0.6, 1].map((t) => outside(1, t).add(new THREE.Vector3(0.012, t === 1 ? 0.05 : 0, 0)));
    addTube(outer, stem, stem.map(() => 0.032), { sides: 5, random, paint: () => shade(wood, 0.85), caps: ['open', 'grain'] });

    // The inside, darker, facing in: its planking, the transom's inner face,
    // three seats across it and four ribs.
    const inner = new Shape();
    const dark = color(PALETTE.woodDark);
    for (let i = 0; i < S; i++) {
      for (let j = 0; j < T; j++) {
        const c = vary(shade(dark, j % 2 ? 1.06 : 0.96), random, 0.6);
        inner.quad(inside(ss[i], ts[j]), inside(ss[i], ts[j + 1]), inside(ss[i + 1], ts[j + 1]), inside(ss[i + 1], ts[j]), c);
      }
    }
    const aft = ts.map((t) => inside(0, t));
    const aftMiddle = aft.reduce((sum, p) => sum.add(p), new THREE.Vector3()).divideScalar(aft.length);
    for (let j = 0; j < T; j++) inner.triangle(aftMiddle, aft[j + 1], aft[j], shade(dark, 1.05));
    inner.triangle(aftMiddle, aft[0], aft[T], shade(dark, 1.05));
    const up = new THREE.Vector3(0, 1, 0);
    const seats: number[] = [];
    for (const [s, width] of [
      [0.08, 0.3],
      [0.34, 0.22],
      [0.64, 0.22],
    ] as const) {
      const y = sheer(s) - 0.14;
      // Across the boat, from side to side at that height.
      const half = halfBeam(s) * 0.93 - thick;
      const x = stern + Rowboat.LENGTH * s;
      addBeam(inner, new THREE.Vector3(x, y, -half), new THREE.Vector3(x, y, half), up, 0.035, width, random, { colors: { side: color(PALETTE.wood), groove: color(PALETTE.woodGroove), end: color(PALETTE.woodEnd), core: color(PALETTE.woodCore) } });
      seats.push(y + 0.0175);
    }
    for (const s of [0.2, 0.42, 0.56, 0.78]) {
      const path = ts.slice(1, T).map((t) => {
        const p = inside(s, t);
        return p.add(new THREE.Vector3(0, 0.012, -Math.sign(p.z) * 0.012));
      });
      addTube(inner, path, path.map(() => 0.016), { sides: 4, random, paint: () => shade(dark, 0.85) });
    }

    // The oars, lying along it on the seats, blades aft.
    const oars = new Shape();
    const oarY = Math.max(seats[1], seats[2]) + 0.025;
    for (const z of [-0.17, 0.17]) {
      const from = new THREE.Vector3(-0.75, oarY, z);
      const to = new THREE.Vector3(1.25, oarY + 0.04, z * 0.6);
      addTube(oars, [from, to], [0.022, 0.02], { sides: 6, random, paint: () => vary(shade(wood, 1.1), random, 0.5), caps: ['grain', 'grain'] });
      const along = to.clone().sub(from).normalize();
      const blade = [new THREE.Vector2(0, -0.035), new THREE.Vector2(0.08, -0.07), new THREE.Vector2(0.5, -0.075), new THREE.Vector2(0.52, 0), new THREE.Vector2(0.5, 0.075), new THREE.Vector2(0.08, 0.07), new THREE.Vector2(0, 0.035)];
      // The blade lies flat, its face turned up, reaching aft from the shaft.
      const flat = new THREE.Matrix4().makeBasis(along.clone().negate(), new THREE.Vector3().crossVectors(up, along.clone().negate()).normalize(), up);
      flat.setPosition(from);
      addExtrusion(oars, blade, 0.018, flat, { face: shade(wood, 1.12), side: shade(wood, 0.95) });
    }

    this.hull = mesh(outer.geometry(), matte());
    const dry = matte();
    dry.name = 'rowboat-inside';
    this.inside = mesh(inner.geometry(), dry);
    this.oars = mesh(oars.geometry(), matte());
    this.add(this.hull, this.inside, this.oars);
    if (options.season === 'winter') snowOn(this, random);
  }

  // Afloat, the water must not show inside it: its inside is marked dry
  // (environtments/water.ts), and a water surface made with waterSurface()
  // is not drawn over it.
  keepDry(): void {
    keepDry(this, [this.inside.material as THREE.Material, this.oars.material as THREE.Material]);
  }
}
