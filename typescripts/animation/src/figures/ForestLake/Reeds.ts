import * as THREE from 'three';
import { addBlade, addLathe, addTube, between, color, matte, mesh, palette, seededRandom, Shape, shade, snowOn, vary, type Season } from './parts';

export interface ReedsOptions {
  height?: number; // m, the tallest stems; 1.15 by default
  cattails?: number; // how many stems carry a brown head; 4 by default
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's reeds (Reeds.md; the reference's "Reed / Grass" and the
// cattails in its "Grass & Flower"): a clump of long, narrow blades standing
// up out of the shallows, arching over at their tips, and among them tall
// stems each topped with a cattail's velvety brown head and a thin spike
// above it. As the reference's seasons show them: deeper green in summer,
// golden brown in autumn, and dry straw with snow on their heads in winter.
// They stand at y 0, the water's surface or the ground.
export class Reeds extends THREE.Group {
  readonly blades: THREE.Mesh; // with the stems
  readonly heads: THREE.Mesh;

  constructor(options: ReedsOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 241);
    const height = options.height ?? 1.15;
    const P = palette(options.season);

    const green = new Shape();
    const reed = color(P.reed);
    const foot = shade(reed, 0.62);
    const tip = shade(reed, 1.25);
    const blades = 16;
    for (let k = 0; k < blades; k++) {
      const angle = (2 * Math.PI * (k + random() * 0.8)) / blades;
      const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
      const side = new THREE.Vector3(-out.z, 0, out.x);
      const base = out.clone().multiplyScalar(between(random, 0.02, 0.14)).setY(-0.15);
      addBlade(green, base, out, side, height * between(random, 0.55, 0.95), between(random, 0.12, 0.35), between(random, 0.028, 0.04), vary(foot, random), vary(reed, random, 1.5), vary(tip, random, 1.5));
    }

    // The cattails: straight stems, each with its brown head near the top
    // and a thin spike past it.
    const heads = new Shape();
    const brown = color(P.cattail);
    const count = options.cattails ?? 4;
    for (let k = 0; k < count; k++) {
      const angle = random() * Math.PI * 2;
      const d = between(random, 0.02, 0.12);
      const from = new THREE.Vector3(Math.cos(angle) * d, -0.15, Math.sin(angle) * d);
      const tall = height * between(random, 0.85, 1.12);
      const lean = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle)).multiplyScalar(tall * between(random, 0.03, 0.1));
      const top = from.clone().add(new THREE.Vector3(0, tall, 0)).add(lean);
      addTube(green, [from, from.clone().lerp(top, 0.5), top], [0.008, 0.007, 0.005], { sides: 3, random, paint: () => vary(reed, random, 1) });
      // The head turned on a lathe along the stem: its y up the stem, in a
      // right-handed frame.
      const dir = top.clone().sub(from).normalize();
      const headFoot = top.clone().addScaledVector(dir, -0.24);
      const z = new THREE.Vector3().crossVectors(new THREE.Vector3(1, 0, 0), dir).normalize();
      const matrix = new THREE.Matrix4().makeBasis(new THREE.Vector3().crossVectors(dir, z).normalize(), dir, z).setPosition(headFoot);
      addLathe(
        heads,
        [
          [0.012, 0],
          [0.03, 0.03],
          [0.033, 0.1],
          [0.03, 0.16],
          [0.01, 0.185],
          [0, 0.19],
        ],
        6,
        matrix,
        (r, i) => vary(shade(brown, r === 0 || r >= 3 ? 0.85 : i % 2 ? 1.08 : 0.96), random, 0.8),
      );
      addTube(green, [top.clone().addScaledVector(dir, -0.05), top.clone().addScaledVector(dir, 0.12)], [0.005, 0.002], { sides: 3, random, paint: () => vary(shade(reed, 0.8), random, 1) });
    }

    this.blades = mesh(green.geometry(), matte());
    this.heads = mesh(heads.geometry(), matte());
    this.add(this.blades, this.heads);
    if (options.season === 'winter') snowOn(this.heads, random, 0.2, 0.6);
  }
}
