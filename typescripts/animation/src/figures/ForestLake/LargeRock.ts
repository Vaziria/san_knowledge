import * as THREE from 'three';
import { addFallenLeaves, addMoss, addRock, addTuft, between, matte, mesh, palette, seededRandom, Shape, snowOn, type Season } from './parts';

export interface LargeRockOptions {
  size?: number; // m, across the boulder; 2.4 by default
  moss?: boolean; // moss on its top, as one of the asset sheet's has; false by default
  // Grass and moss round its foot, as the asset sheet has them; true by
  // default. The forest lake scatters bare rocks and its grass apart.
  dressing?: boolean;
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's large rock (LargeRock.md), as its asset sheet draws it:
// a big faceted lavender-grey boulder with a smaller one leaning on it and
// a couple of stones, light where they face the sky and warm in the sun,
// with grass round their feet. As the reference's seasons show them: leaves
// fallen round them in autumn, snow on their tops in winter.
export class LargeRock extends THREE.Group {
  readonly rocks: THREE.Mesh;
  readonly dressing: THREE.Mesh;

  constructor(options: LargeRockOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 41);
    const size = options.size ?? 2.4;
    const moss = options.moss ? 0.72 : 2;
    const season = options.season ?? 'spring';
    const P = palette(season);

    const stone = new Shape();
    addRock(stone, new THREE.Vector3(0, 0, 0), new THREE.Vector3(size, size * 0.86, size * 0.82), random() * Math.PI, random, moss, 30, P);
    const angle = between(random, 2.0, 2.6); // in front, to the left
    const second = size * between(random, 0.42, 0.5);
    addRock(stone, new THREE.Vector3(Math.cos(angle) * size * 0.5, 0, Math.sin(angle) * size * 0.45), new THREE.Vector3(second, second * 0.85, second * 0.9), random() * Math.PI, random, moss, 16, P);
    for (let k = 0; k < 2; k++) {
      const a = between(random, 0, Math.PI * 2);
      const small = size * between(random, 0.12, 0.2);
      addRock(stone, new THREE.Vector3(Math.cos(a) * size * 0.62, 0, Math.sin(a) * size * 0.55), new THREE.Vector3(small, small * 0.75, small), random() * Math.PI, random, 2, 10, P);
    }

    const green = new Shape();
    if (options.dressing ?? true) {
      for (let k = 0; k < 6; k++) {
        const a = ((k + random() * 0.7) / 6) * Math.PI * 2;
        const d = size * between(random, 0.45, 0.6);
        addTuft(green, new THREE.Vector3(Math.cos(a) * d, 0, Math.sin(a) * d * 0.9), 8, between(random, 0.22, 0.38), random, 0, 0, P);
      }
      for (let k = 0; k < 3; k++) {
        const a = random() * Math.PI * 2;
        const d = size * between(random, 0.5, 0.7);
        addMoss(green, new THREE.Vector3(Math.cos(a) * d, 0, Math.sin(a) * d), between(random, 0.2, 0.35), random, P);
      }
      if (season === 'autumn') addFallenLeaves(green, new THREE.Vector3(), size * 0.8, 18, random);
    }

    this.rocks = mesh(stone.geometry(), matte());
    this.dressing = mesh(green.geometry(), matte());
    this.add(this.rocks, this.dressing);
    if (season === 'winter') snowOn(this, random, 0.35, 0.7);
  }
}
