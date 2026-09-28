import * as THREE from 'three';
import { addBerry, addBlade, addBlob, addFlower, between, color, matte, mesh, palette, seededRandom, Shape, snowOn, vary, type Season } from './parts';

export interface BushOptions {
  size?: number; // m, across; 1.1 by default
  // Small flowers on it, as the asset sheet has (white, pink and yellow in
  // spring, white and yellow in summer; none in autumn; red berries in
  // winter); true by default.
  flowers?: boolean;
  season?: Season; // spring by default
  // Fewer facets, for bushes seen from afar: plainer clumps, no leaves at
  // its foot, fewer flowers. False by default.
  simple?: boolean;
  seed?: number;
}

// The forest lake's bush (Bush.md), as its asset sheet draws it: a round
// mound of leafy clumps, dark green in its hollows and fresh green where the
// sun catches it, dotted with small white, pink and yellow flowers, with a
// few long leaves at its foot. As the reference's seasons show it: deeper
// green in summer, orange and red in autumn, and in winter dark under a
// cap of snow, dotted with red berries.
export class Bush extends THREE.Group {
  readonly leaves: THREE.Mesh;
  readonly flowers: THREE.Mesh; // or berries

  constructor(options: BushOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 51);
    const size = options.size ?? 1.1;
    const s = size / 1.1;
    const season = options.season ?? 'spring';
    const P = palette(season);

    const leafy = new Shape();
    const light = color(P.leafLight);
    const mid = color(P.leaf);
    const dark = color(P.leafDark);
    const paint = (n: THREE.Vector3) => {
      const base = n.y > 0 ? mid.clone().lerp(light, Math.min(1, n.y * 1.2)) : mid.clone().lerp(dark, Math.min(1, -n.y * 1.3 + 0.2));
      return vary(base, random, 2);
    };
    const clumps: { at: THREE.Vector3; radii: THREE.Vector3 }[] = [];
    const ring = 5;
    for (let k = 0; k < ring; k++) {
      const angle = ((k + random() * 0.5) / ring) * Math.PI * 2;
      const r = between(random, 0.3, 0.38) * s;
      clumps.push({
        at: new THREE.Vector3(Math.cos(angle) * 0.26 * s, r * 0.72, Math.sin(angle) * 0.24 * s),
        radii: new THREE.Vector3(r, r * 0.85, r),
      });
    }
    const top = between(random, 0.34, 0.4) * s;
    clumps.push({ at: new THREE.Vector3(0, 0.52 * s, 0), radii: new THREE.Vector3(top, top * 0.85, top) });
    for (const { at, radii } of clumps) addBlob(leafy, at, radii, options.simple ? 0 : 1, 0.26, random, paint);
    // Long leaves at its foot.
    for (let k = 0; k < (options.simple ? 0 : 7); k++) {
      const angle = random() * Math.PI * 2;
      const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
      const side = new THREE.Vector3(-out.z, 0, out.x);
      const base = out.clone().multiplyScalar(0.42 * s).setY(-0.01);
      addBlade(leafy, base, out, side, between(random, 0.14, 0.24) * s, 0.9, 0.05 * s, vary(dark, random), vary(mid, random), vary(light, random));
    }

    // Its flowers, or in winter its berries, out past the clumps' lumps,
    // which stand up to a quarter out of them.
    const blooms = new Shape();
    const out = (radius: number) => {
      const { at, radii } = clumps[Math.floor(random() * clumps.length)];
      const dir = new THREE.Vector3(between(random, -1, 1), between(random, -0.1, 1), between(random, -1, 1)).normalize();
      return { point: at.clone().add(dir.clone().multiply(radii).multiplyScalar(radius)), dir };
    };
    if ((options.flowers ?? true) && season === 'winter') {
      const berry = color(P.berry);
      for (let k = 0; k < (options.simple ? 8 : 16); k++) {
        const { point } = out(1.08);
        for (let j = 0; j < 3; j++) addBerry(blooms, point.clone().add(new THREE.Vector3(between(random, -0.03, 0.03), between(random, -0.02, 0.02), between(random, -0.03, 0.03))), 0.022 * s, vary(berry, random, 0.8));
      }
    } else if ((options.flowers ?? true) && season !== 'autumn') {
      const kinds = season === 'summer' ? [P.petalWhite, P.petalWhite, P.petalYellow] : [P.petalWhite, P.petalWhite, P.petalPink, P.petalYellow, P.petalLight];
      const count = options.simple ? 6 : 12 + Math.floor(random() * 5);
      for (let k = 0; k < count; k++) {
        const { point, dir } = out(1.2);
        const petal = color(kinds[Math.floor(random() * kinds.length)]);
        addFlower(blooms, point, dir, 5 + Math.floor(random() * 2), between(random, 0.075, 0.095) * s, petal, color(P.flowerHeart), random, 0.15);
      }
    }

    this.leaves = mesh(leafy.geometry(), matte());
    this.flowers = mesh(blooms.geometry(), matte());
    this.add(this.leaves, this.flowers);
    if (season === 'winter') snowOn(this.leaves, random, 0.2, 0.55);
  }
}
