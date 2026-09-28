import * as THREE from 'three';
import { addBlob, addTube, between, color, matte, mesh, palette, seededRandom, Shape, shade, snowOn, vary, type Season } from './parts';

export interface CherryTreeOptions {
  height?: number; // m, to the top of the crown; 5.5 by default
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's cherry tree (CherryTree.md), as its asset sheet draws it
// in blossom: a short orange-brown trunk flaring into its roots, forking
// into a few thick branches, under a big rounded crown of pink blossom in
// chunky clusters, pale where the sun catches them and deep pink in the
// shade underneath. As the reference's seasons show the tree: in summer its
// crown is in green leaf, in autumn red and orange, and in winter its
// branches are bare but for clumps of snow at their ends, snow along their
// tops.
export class CherryTree extends THREE.Group {
  readonly trunk: THREE.Mesh; // with its branches
  readonly crown: THREE.Mesh;

  constructor(options: CherryTreeOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 21);
    const height = options.height ?? 5.5;
    const s = height / 5.5;
    const P = palette(options.season);
    const winter = options.season === 'winter';

    // The trunk, leaning a little, with its roots.
    const wood = new Shape();
    const bark = color(P.bark);
    const barkDark = color(P.barkDark);
    const sides = 7;
    const shades = Array.from({ length: sides }, (_, i) => vary(i % 2 ? bark : bark.clone().lerp(barkDark, 0.3), random, 1.2));
    const paint = (_: number, i: number) => shades[i % sides];
    const lean = new THREE.Vector3(between(random, -0.08, 0.08), 1, between(random, -0.08, 0.08)).normalize();
    const fork = lean.clone().multiplyScalar(1.9 * s);
    const r = 0.2 * s;
    addTube(
      wood,
      [new THREE.Vector3(0, -0.2, 0), new THREE.Vector3(0, 0.05, 0), lean.clone().multiplyScalar(0.7 * s).add(new THREE.Vector3(0.05 * s, 0, 0)), fork],
      [r * 1.7, r * 1.45, r * 1.02, r * 0.85],
      { sides, rough: 0.07, random, paint },
    );
    for (let k = 0; k < 4; k++) {
      const angle = ((k + random() * 0.6) / 4) * Math.PI * 2;
      const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
      addTube(
        wood,
        [out.clone().multiplyScalar(r * 0.5).setY(r * 1.2), out.clone().multiplyScalar(r * 1.7).setY(0.03), out.clone().multiplyScalar(r * 2.6).setY(-0.1)],
        [r * 0.55, r * 0.36, r * 0.14],
        { sides: 5, random, paint },
      );
    }

    // The branches, forking from the top of the trunk up and out, each
    // ending in a cluster of blossom; a few more clusters fill the crown.
    const clusters: { at: THREE.Vector3; size: number }[] = [];
    const branches = 4 + Math.floor(random() * 2);
    for (let k = 0; k < branches; k++) {
      const angle = ((k + between(random, -0.25, 0.25)) / branches) * Math.PI * 2;
      const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
      const rise = between(random, 0.9, 1.5);
      const reach = between(random, 0.75, 1.1) * s;
      const mid = fork.clone().addScaledVector(out, reach * 0.55).add(new THREE.Vector3(0, rise * 0.6 * s, 0));
      const end = fork.clone().addScaledVector(out, reach * 1.35).add(new THREE.Vector3(0, rise * s, 0));
      addTube(wood, [fork.clone().addScaledVector(out, r * 0.2), mid, end], [r * 0.62, r * 0.42, r * 0.22], { sides: 6, random, paint });
      clusters.push({ at: end, size: between(random, 0.95, 1.2) * s });
      clusters.push({ at: mid.clone().addScaledVector(out, 0.25 * s).add(new THREE.Vector3(0, 0.35 * s, 0)), size: between(random, 0.7, 0.9) * s });
    }
    clusters.push({ at: fork.clone().add(new THREE.Vector3(0, 1.75 * s, 0)), size: 1.15 * s });
    clusters.push({ at: fork.clone().add(new THREE.Vector3(between(random, -0.3, 0.3) * s, 2.35 * s, between(random, -0.3, 0.3) * s)), size: 0.85 * s });

    // The blossom (the season's crown: leaves, or snow): pale where it
    // faces the sky, warm here and there where the sun catches it, deep
    // underneath, each face its own shade, with small chunks standing out of
    // the clusters.
    const blossom = new Shape();
    const light = color(P.blossomLight);
    const mid = color(P.blossom);
    const deep = color(P.blossomDeep);
    const warm = color(P.blossomWarm);
    const petals = (n: THREE.Vector3) => {
      const roll = random();
      if (roll < 0.07) return vary(warm, random, 1);
      if (roll < 0.15) return vary(shade(light, 1.04), random, 0.6);
      const base = n.y > 0 ? mid.clone().lerp(light, Math.min(1, n.y * 1.1)) : mid.clone().lerp(deep, Math.min(1, -n.y * 1.2));
      return vary(base, random, 1.8);
    };
    // Each cluster a ball of blossom with many smaller clumps standing out
    // of it, so it looks heaped with flowers rather than smooth; in winter
    // only a clump of snow caught at each branch's end, flattened.
    for (const { at, size } of clusters) {
      const radii = new THREE.Vector3(size, size * between(random, 0.72, 0.85), size).multiplyScalar(winter ? 0.5 : 0.84);
      if (winter) radii.y *= 0.62;
      addBlob(blossom, at, radii, 1, 0.2, random, petals);
      const chunks = winter ? 2 : 7 + Math.floor(random() * 4);
      for (let k = 0; k < chunks; k++) {
        const dir = new THREE.Vector3(between(random, -1, 1), between(random, -0.25, 1), between(random, -1, 1)).normalize();
        const c = at.clone().add(dir.clone().multiply(radii).multiplyScalar(between(random, 0.86, 1.0)));
        const sizeChunk = size * between(random, 0.2, 0.34) * (winter ? 0.7 : 1);
        addBlob(blossom, c, new THREE.Vector3(sizeChunk, sizeChunk * 0.85, sizeChunk), 0, 0.22, random, petals);
      }
    }

    this.trunk = mesh(wood.geometry(), matte());
    this.crown = mesh(blossom.geometry(), matte());
    this.add(this.trunk, this.crown);
    if (winter) snowOn(this.trunk, random, 0.35, 0.7);
  }
}
