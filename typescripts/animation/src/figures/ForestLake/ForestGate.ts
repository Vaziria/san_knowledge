import * as THREE from 'three';
import { addBar, addBeam, addLantern, addTuft, frame, glowing, grain, halo, matte, mesh, PALETTE, palette, seededRandom, Shape, snowOn, woodColors, type Season } from './parts';

export interface ForestGateOptions {
  width?: number; // m between the posts' middles; 3 by default
  season?: Season; // spring by default; snow on it in winter
  seed?: number;
}

const HEIGHT = 3.1; // m, the posts over the ground

// The forest lake's entrance gate (ForestGate.md; the reference's "Forest
// Entrance"): where the forest path comes into the valley, two tall square
// posts either side of it carry a heavy crossbeam, its ends running out
// past them and cut off aslant, a tie beam under it and braces in its
// corners; a lantern hangs from the crossbeam inside each post, over the
// path. The posts stand along x, the path running through along z. The
// lanterns flicker (update()). In winter snow lies along its beams.
export class ForestGate extends THREE.Group {
  readonly frame: THREE.Mesh; // the posts, beams and braces
  readonly lanterns: THREE.Mesh; // their iron
  readonly glass: THREE.Mesh;
  readonly tufts: THREE.Mesh;
  private readonly glows: THREE.Sprite[] = [];
  private time: number;

  constructor(options: ForestGateOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 281);
    const width = options.width ?? 3;
    const P = palette(options.season);
    const up = new THREE.Vector3(0, 1, 0);
    this.time = random() * 10;

    const wood = new Shape();
    const half = width / 2;
    for (const side of [-1, 1]) {
      addBar(wood, frame(new THREE.Vector3(side * half, -0.4, 0), up, new THREE.Vector3(0, 0, 1)), {
        length: HEIGHT + 0.4,
        width: 0.24,
        height: 0.24,
        chamfer: 0.035,
        bevel: 0.02,
        segments: 5,
        rough: 0.005,
        slant: [0.04 * side, 0],
        cracks: grain(random, [0, 1, 2, 3], [1, 2], 0.15),
        caps: [false, true],
        colors: woodColors(PALETTE.post),
        random,
      });
    }
    // The crossbeam, resting on the posts' tops and running out past them,
    // both its ends cut aslant, the top running further; the tie beam under
    // it; the braces.
    const beamY = HEIGHT + 0.12;
    const overhang = 0.5;
    addBar(wood, frame(new THREE.Vector3(-half - overhang, beamY, 0), new THREE.Vector3(1, 0, 0), new THREE.Vector3(0, 0, 1)), {
      length: width + overhang * 2,
      width: 0.26,
      height: 0.24,
      chamfer: 0.035,
      bevel: 0.02,
      segments: 6,
      rough: 0.006,
      slant: [0.45, 0],
      slantStart: [-0.45, 0],
      cracks: grain(random, [0, 1, 2], [1, 2]),
      colors: woodColors(PALETTE.wood),
      random,
    });
    addBeam(wood, new THREE.Vector3(-half - 0.1, HEIGHT - 0.42, 0.02), new THREE.Vector3(half + 0.1, HEIGHT - 0.42, 0.02), up, 0.14, 0.11, random, { colors: woodColors(PALETTE.post) });
    for (const side of [-1, 1]) {
      addBeam(wood, new THREE.Vector3(side * (half - 0.08), HEIGHT - 1.05, 0), new THREE.Vector3(side * (half - 0.72), beamY - 0.1, 0), new THREE.Vector3(side, 1, 0), 0.11, 0.1, random);
    }

    // The lanterns, one inside each post, hanging from the crossbeam.
    const iron = new Shape();
    const glass = new Shape();
    for (const side of [-1, 1]) {
      const glow = halo(PALETTE.glass, 1.6, 0.85);
      glow.position.copy(addLantern(iron, glass, new THREE.Vector3(side * (half - 0.42), beamY - 0.12, 0.2), random));
      this.glows.push(glow);
    }

    // Grass round the posts' feet.
    const green = new Shape();
    for (const side of [-1, 1]) addTuft(green, new THREE.Vector3(side * half, 0, 0), 13, 0.28, random, 0.14, 0, P);

    this.frame = mesh(wood.geometry(), matte());
    this.lanterns = mesh(iron.geometry(), matte());
    this.glass = mesh(glass.geometry(), glowing(), false);
    this.tufts = mesh(green.geometry(), matte());
    this.add(this.frame, this.lanterns, this.glass, ...this.glows, this.tufts);
    if (options.season === 'winter') snowOn(this, random);
  }

  update(delta: number): void {
    this.time += delta;
    const t = this.time;
    const flicker = 1 + 0.06 * Math.sin(t * 6.9) + 0.04 * Math.sin(t * 15.7 + 1) + 0.03 * Math.sin(t * 33.1);
    (this.glass.material as THREE.MeshBasicMaterial).color.setScalar(flicker);
    for (const glow of this.glows) glow.material.opacity = 0.85 * flicker;
  }
}
