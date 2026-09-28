import * as THREE from 'three';
import { addFallenLeaves, addMoss, addMushroom, addTuft, between, matte, mesh, palette, seededRandom, Shape, snowOn, type Season } from './parts';

export interface MushroomsOptions {
  count?: number; // 5 by default
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's mushrooms (Mushrooms.md; the reference's "Mushroom
// Cluster"): a few mushrooms of different sizes growing together, the
// biggest in the middle, most with red caps spotted white and some with
// plain tan caps, on pale stems, with moss and a tuft of grass at their
// feet. In autumn leaves lie round them; in winter snow sits on their caps.
export class Mushrooms extends THREE.Group {
  readonly mushrooms: THREE.Mesh;
  readonly dressing: THREE.Mesh;

  constructor(options: MushroomsOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 251);
    const count = options.count ?? 5;
    const season = options.season ?? 'spring';
    const P = palette(season);

    const caps = new Shape();
    for (let k = 0; k < count; k++) {
      const big = k === 0;
      const angle = (k / count) * Math.PI * 2 + random() * 0.8;
      const d = big ? 0 : between(random, 0.1, 0.22);
      const at = new THREE.Vector3(Math.cos(angle) * d, 0, Math.sin(angle) * d);
      const height = big ? 0.2 : between(random, 0.07, 0.15);
      const cap = big ? 0.2 : between(random, 0.08, 0.15);
      // Leaning out from the middle a little.
      addMushroom(caps, at, height, cap, random() < 0.7 ? 'red' : 'tan', random, Math.PI - angle, big ? 0 : between(random, 0.05, 0.25));
    }

    const green = new Shape();
    addMoss(green, new THREE.Vector3(), 0.2, random, P);
    addTuft(green, new THREE.Vector3(between(random, -0.2, 0.2), 0, -0.22), 7, 0.2, random, 0, 0, P);
    if (season === 'autumn') addFallenLeaves(green, new THREE.Vector3(), 0.45, 10, random);

    this.mushrooms = mesh(caps.geometry(), matte());
    this.dressing = mesh(green.geometry(), matte());
    this.add(this.mushrooms, this.dressing);
    if (season === 'winter') snowOn(this, random, 0.45, 0.8);
  }
}
