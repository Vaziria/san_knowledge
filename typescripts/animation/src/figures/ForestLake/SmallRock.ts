import * as THREE from 'three';
import { addFallenLeaves, addFlower, addMoss, addRock, addTube, addTuft, between, color, matte, mesh, palette, seededRandom, Shape, snowOn, type Season } from './parts';

export interface SmallRockOptions {
  size?: number; // m, across the biggest rock; 0.75 by default
  // Grass, moss and a flower round the rocks, as the asset sheet has them;
  // true by default. The forest lake scatters bare rocks and its grass apart.
  dressing?: boolean;
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's small rocks (SmallRock.md), as its asset sheet draws
// them: a faceted lavender-grey rock with a few pebbles round it, light
// where they face the sky, with grass tufts, moss and a small pink flower
// at their feet. As the reference's seasons show them: leaves fallen round
// them in autumn, snow on their tops in winter.
export class SmallRock extends THREE.Group {
  readonly rocks: THREE.Mesh;
  readonly dressing: THREE.Mesh;

  constructor(options: SmallRockOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 31);
    const size = options.size ?? 0.75;
    const season = options.season ?? 'spring';
    const P = palette(season);

    const stone = new Shape();
    addRock(stone, new THREE.Vector3(0, 0, 0), new THREE.Vector3(size, size * 0.85, size * 0.82), random() * Math.PI, random, 2, 16, P);
    const pebbles = 3 + Math.floor(random() * 2);
    const around: THREE.Vector3[] = [];
    for (let k = 0; k < pebbles; k++) {
      const angle = ((k + random() * 0.6) / pebbles) * Math.PI * 2;
      const d = size * between(random, 0.55, 0.75);
      const at = new THREE.Vector3(Math.cos(angle) * d, 0, Math.sin(angle) * d);
      const small = size * between(random, 0.25, 0.45);
      addRock(stone, at, new THREE.Vector3(small, small * 0.8, small * 0.85), random() * Math.PI, random, 2, 12, P);
      around.push(at);
    }

    const green = new Shape();
    if (options.dressing ?? true) {
      for (let k = 0; k < 3; k++) {
        const angle = random() * Math.PI * 2;
        const d = size * between(random, 0.45, 0.95);
        addTuft(green, new THREE.Vector3(Math.cos(angle) * d, 0, Math.sin(angle) * d), 7, size * between(random, 0.25, 0.4), random, 0, 0, P);
      }
      for (let k = 0; k < 3; k++) {
        const angle = random() * Math.PI * 2;
        const d = size * between(random, 0.6, 1.05);
        addMoss(green, new THREE.Vector3(Math.cos(angle) * d, 0, Math.sin(angle) * d), size * between(random, 0.2, 0.32), random, P);
      }
      // A small pink flower on a short stem.
      const foot = around[0].clone().multiplyScalar(1.35);
      const head = foot.clone().add(new THREE.Vector3(0, size * 0.22, 0));
      addTube(green, [foot, head], [0.006, 0.005], { sides: 3, random, paint: () => color(P.grass) });
      addFlower(green, head, new THREE.Vector3(0.2, 1, 0.3), 6, 0.04, color(P.petalPink), color(P.flowerHeart), random);
      if (season === 'autumn') addFallenLeaves(green, new THREE.Vector3(), size * 1.1, 10, random);
    }

    this.rocks = mesh(stone.geometry(), matte());
    this.dressing = mesh(green.geometry(), matte());
    this.add(this.rocks, this.dressing);
    if (season === 'winter') snowOn(this, random, 0.35, 0.7);
  }
}
