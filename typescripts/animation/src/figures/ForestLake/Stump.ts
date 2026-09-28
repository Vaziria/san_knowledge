import * as THREE from 'three';
import { addFallenLeaves, addMoss, addMushroom, addTube, addTuft, between, color, matte, mesh, palette, seededRandom, Shape, shade, snowOn, vary, type Season } from './parts';

export interface StumpOptions {
  radius?: number; // m, at its sawn top; 0.34 by default
  dressing?: boolean; // grass and moss round it, as the asset sheet has; true by default
  // Mushrooms at its foot, red and tan, as the reference's stump has; true
  // by default.
  mushrooms?: boolean;
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's stump (Stump.md), as its asset sheet draws it: what is
// left of a felled trunk, sawn across, its top showing pale growth rings
// round a darker heart, its bark cracked and patched with moss, its roots
// spreading out into the ground, and mushrooms at its foot. As the
// reference's seasons show it: leaves fallen round it in autumn, snow on
// its top and roots in winter.
export class Stump extends THREE.Group {
  readonly wood: THREE.Mesh; // with its mushrooms
  readonly dressing: THREE.Mesh;

  constructor(options: StumpOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 101);
    const radius = options.radius ?? 0.34;
    const s = radius / 0.34;
    const season = options.season ?? 'spring';
    const P = palette(season);

    const wood = new Shape();
    const bark = color(P.bark);
    const crack = color(P.barkDark);
    const moss = color(P.moss);
    const sides = 10;
    const paint = (r: number, i: number) => {
      if (r === 0 && random() < 0.35) return vary(moss, random, 2);
      if (random() < 0.2) return vary(crack, random, 1);
      return vary(i % 2 ? bark : shade(bark, 0.87), random, 1.4);
    };
    const height = 0.55 * s;
    addTube(
      wood,
      [new THREE.Vector3(0, -0.12, 0), new THREE.Vector3(0.01, height * 0.3, 0), new THREE.Vector3(0.02 * s, height * 0.7, -0.01), new THREE.Vector3(0.02 * s, height, 0)],
      [radius * 1.3, radius * 1.1, radius * 1.02, radius],
      { sides, rough: 0.06, random, paint, caps: ['open', 'rings'] },
    );
    // Roots, spreading out and down into the ground.
    const roots = 5;
    const rootAngles: number[] = [];
    for (let k = 0; k < roots; k++) {
      const angle = ((k + between(random, -0.2, 0.2)) / roots) * Math.PI * 2;
      rootAngles.push(angle);
      const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
      const reach = between(random, 0.55, 0.75) * s;
      addTube(
        wood,
        [
          out.clone().multiplyScalar(radius * 0.6).setY(height * 0.45),
          out.clone().multiplyScalar(radius * 1.25).setY(height * 0.14),
          out.clone().multiplyScalar(reach).setY(0.02),
          out.clone().multiplyScalar(reach + 0.18 * s).setY(-0.1),
        ],
        [0.15 * s, 0.11 * s, 0.06 * s, 0.03 * s],
        { sides: 6, rough: 0.08, random, paint: (_, i) => vary(i % 2 ? bark : shade(bark, 0.85), random, 1.2) },
      );
    }
    // Mushrooms between two roots: a red one and two tan.
    if ((options.mushrooms ?? true) && season !== 'winter') {
      const between0 = (rootAngles[0] + rootAngles[1]) / 2;
      for (let k = 0; k < 3; k++) {
        const a = between0 + between(random, -0.35, 0.35);
        const d = radius * between(random, 1.3, 1.7);
        addMushroom(wood, new THREE.Vector3(Math.cos(a) * d, 0, Math.sin(a) * d), between(random, 0.07, 0.12) * s, between(random, 0.09, 0.14) * s, k === 0 ? 'red' : 'tan', random, random() * Math.PI, between(random, -0.2, 0.2));
      }
    }

    const green = new Shape();
    if (options.dressing ?? true) {
      for (let k = 0; k < 3; k++) {
        const angle = random() * Math.PI * 2;
        const d = between(random, 0.55, 0.8) * s;
        addTuft(green, new THREE.Vector3(Math.cos(angle) * d, 0, Math.sin(angle) * d), 8, between(random, 0.22, 0.32), random, 0, 0, P);
      }
      for (let k = 0; k < 3; k++) {
        const angle = random() * Math.PI * 2;
        const d = between(random, 0.5, 0.75) * s;
        addMoss(green, new THREE.Vector3(Math.cos(angle) * d, 0, Math.sin(angle) * d), between(random, 0.12, 0.2), random, P);
      }
      if (season === 'autumn') addFallenLeaves(green, new THREE.Vector3(), 0.95 * s, 12, random);
    }

    this.wood = mesh(wood.geometry(), matte());
    this.dressing = mesh(green.geometry(), matte());
    this.add(this.wood, this.dressing);
    if (season === 'winter') snowOn(this, random, 0.3, 0.65);
  }
}
