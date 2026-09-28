import * as THREE from 'three';
import { addTube, between, color, matte, mesh, seededRandom, Shape, shade, snowOn, vary, type Season } from './parts';

export interface DeadTreeOptions {
  height?: number; // m, to its highest twigs; 5 by default
  season?: Season; // winter by default: snow along its branches
  seed?: number;
}

// The forest lake's bare tree (DeadTree.md; the reference's winter "Tree
// (Dead)"): a grey-brown trunk flaring into its roots, forking into a few
// crooked branches that fork again and again into thin twigs, not a leaf
// on it. In winter snow lies along the tops of its branches.
export class DeadTree extends THREE.Group {
  readonly wood: THREE.Mesh;

  constructor(options: DeadTreeOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 271);
    const height = options.height ?? 5;
    const s = height / 5;
    const bark = color(0x6b5446);
    const shades = Array.from({ length: 6 }, (_, i) => vary(shade(bark, i % 2 ? 1.08 : 0.9), random, 1));
    const paint = (_: number, i: number) => shades[i % shades.length];
    const wood = new Shape();

    // A branch from `from` along `dir`, crooked in its middle, and the
    // branches forking from its end, thinner and shorter each time.
    const grow = (from: THREE.Vector3, dir: THREE.Vector3, length: number, radius: number, depth: number) => {
      const bend = new THREE.Vector3(between(random, -1, 1), between(random, -0.3, 0.3), between(random, -1, 1)).multiplyScalar(length * 0.12);
      const end = from.clone().addScaledVector(dir, length);
      const mid = from.clone().lerp(end, 0.5).add(bend);
      addTube(wood, [from, mid, end], [radius, radius * 0.82, radius * 0.62], { sides: depth > 1 ? 6 : 4, rough: 0.05, random, paint });
      if (depth === 0) return;
      const forks = depth >= 2 ? 3 : 2;
      const turn = random() * Math.PI * 2;
      for (let k = 0; k < forks; k++) {
        const a = turn + (k / forks) * Math.PI * 2 + between(random, -0.4, 0.4);
        const out = new THREE.Vector3(Math.cos(a), 0, Math.sin(a));
        const next = dir.clone().multiplyScalar(between(random, 0.9, 1.3)).addScaledVector(out, between(random, 0.55, 0.95)).normalize();
        grow(end, next, length * between(random, 0.58, 0.72), radius * 0.62, depth - 1);
      }
    };
    const lean = new THREE.Vector3(between(random, -0.06, 0.06), 1, between(random, -0.06, 0.06)).normalize();
    const r = 0.17 * s;
    const fork = lean.clone().multiplyScalar(2.1 * s);
    addTube(wood, [new THREE.Vector3(0, -0.2, 0), new THREE.Vector3(0, 0.05, 0), lean.clone().multiplyScalar(1 * s), fork], [r * 1.6, r * 1.3, r, r * 0.85], { sides: 7, rough: 0.07, random, paint });
    for (let k = 0; k < 4; k++) {
      const angle = ((k + random() * 0.6) / 4) * Math.PI * 2;
      const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
      addTube(wood, [out.clone().multiplyScalar(r * 0.5).setY(r * 1.1), out.clone().multiplyScalar(r * 1.6).setY(0.03), out.clone().multiplyScalar(r * 2.5).setY(-0.1)], [r * 0.5, r * 0.32, r * 0.12], { sides: 5, random, paint });
    }
    const branches = 3;
    for (let k = 0; k < branches; k++) {
      const a = (k / branches) * Math.PI * 2 + between(random, -0.3, 0.3);
      const dir = new THREE.Vector3(Math.cos(a) * 0.6, 1, Math.sin(a) * 0.6).normalize();
      grow(fork, dir, 1.35 * s, r * 0.72, 2);
    }
    // The trunk runs on up the middle a little way.
    grow(fork, lean, 1.1 * s, r * 0.7, 1);

    this.wood = mesh(wood.geometry(), matte());
    this.add(this.wood);
    if ((options.season ?? 'winter') === 'winter') snowOn(this, random, 0.3, 0.7);
  }
}
