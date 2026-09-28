import * as THREE from 'three';
import { addBlock, addFallenLeaves, addFlower, addHull, addMoss, addTube, addTuft, between, color, matte, mesh, palette, seededRandom, Shape, snowOn, stonePaint, type Season } from './parts';

export interface RuinsOptions {
  dressing?: boolean; // grass, moss and flowers among the stones; true by default
  season?: Season; // spring by default
  seed?: number;
}

const BASE = 0.3; // m, the paved floor's top

// The forest lake's ruins (Ruins.md; from the reference's "Ruins"): an arch
// of pale dressed stone on a paved floor with steps up to it at the front
// (+z), each pillar a stack of chamfered blocks under a wider capital, the
// arch a ring of wedge-shaped blocks round a keystone; beside it two broken
// pillars, one each side, a low broken wall in front, fallen blocks and
// paving stones in the grass, moss on the stones' tops. As the reference's
// seasons show them: leaves blown among them in autumn, snow on every
// stone's top in winter.
export class Ruins extends THREE.Group {
  readonly arch: THREE.Mesh; // the arch and its pillars
  readonly rubble: THREE.Mesh; // the floor, steps, broken pillars and wall, fallen blocks, paving
  readonly dressing: THREE.Mesh;

  constructor(options: RuinsOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 211);
    const season = options.season ?? 'spring';
    const P = palette(season);
    const paint = stonePaint(random, 0.8, 0.35, P);
    const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

    // The arch: two pillars of blocks, their capitals, and the ring.
    const arch = new Shape();
    const pillar = 0.7;
    const inner = 0.9; // the arch's inner radius, the pillars' inner faces
    const px = inner + pillar / 2;
    const course = 0.42;
    const courses = 6;
    for (const side of [-1, 1]) {
      for (let k = 0; k < courses; k++) {
        const y = BASE + course * (k + 0.5);
        addBlock(arch, v(side * px + between(random, -0.02, 0.02), y, between(random, -0.02, 0.02)), v(pillar, course - 0.012, pillar), paint, random, {
          turn: between(random, -0.03, 0.03),
          chamfer: 0.045,
          rough: 0.012,
        });
      }
      addBlock(arch, v(side * px, BASE + course * courses + 0.1, 0), v(pillar + 0.16, 0.2, pillar + 0.16), paint, random, { chamfer: 0.04 });
    }
    const spring = BASE + course * courses + 0.2; // where the ring starts
    const outer = inner + 0.62;
    const stones = 9;
    for (let k = 0; k < stones; k++) {
      const gap = 0.012;
      const a0 = (Math.PI * k) / stones + gap;
      const a1 = (Math.PI * (k + 1)) / stones - gap;
      const out = outer + (k === 4 ? 0.1 : 0) + between(random, -0.03, 0.03);
      const depth = k === 4 ? 0.42 : 0.35;
      const points: THREE.Vector3[] = [];
      for (const a of [a0, a1]) {
        for (const r of [inner, out]) {
          for (const z of [-depth, depth]) points.push(v(Math.cos(a) * r, spring + Math.sin(a) * r, z));
        }
      }
      addHull(arch, points, paint);
    }

    // The floor of paving slabs and the steps up to it.
    const rubble = new Shape();
    for (let i = 0; i < 4; i++) {
      for (let j = 0; j < 2; j++) {
        addBlock(rubble, v(-1.65 + i * 1.1 + between(random, -0.03, 0.03), BASE / 2, -0.55 + j * 1.1), v(1.06, BASE, 1.06), paint, random, { chamfer: 0.035, rough: 0.012, turn: between(random, -0.03, 0.03) });
      }
    }
    for (let i = 0; i < 3; i++) {
      addBlock(rubble, v(-1.0 + i * 1.0, 0.09, 1.38), v(0.96, 0.2, 0.5), paint, random, { chamfer: 0.03, rough: 0.012, turn: between(random, -0.05, 0.05) });
    }

    // The broken pillars, one each side, their top blocks broken off
    // raggedly; the right one leans.
    const broken = (x: number, z: number, blocks: number, lean: number) => {
      const size = 0.64;
      const h = 0.45;
      const tip: readonly [number, number] = [0, -lean]; // toward +x
      for (let k = 0; k < blocks; k++) {
        const y = h * (k + 0.5);
        const shift = new THREE.Vector3(Math.sin(lean) * y, 0, 0);
        addBlock(rubble, v(x, y, z).add(shift), v(size, h - 0.012, size), paint, random, { chamfer: 0.04, rough: 0.012, turn: between(random, -0.08, 0.08), tip });
      }
      const y0 = h * blocks;
      const shift = Math.sin(lean) * y0;
      const points: THREE.Vector3[] = [];
      for (const sx of [-1, 1]) for (const sz of [-1, 1]) points.push(v(x + shift + sx * size * 0.47, y0, z + sz * size * 0.47));
      for (let k = 0; k < 7; k++) points.push(v(x + shift + between(random, -0.3, 0.3), y0 + between(random, 0.05, 0.42), z + between(random, -0.3, 0.3)));
      addHull(rubble, points, paint);
    };
    broken(-3.25, 0.35, 5, 0);
    broken(3.2, -0.2, 3, 0.05);

    // A low broken wall in front, to the left, and blocks fallen in the
    // grass.
    for (let i = 0; i < 4; i++) {
      addBlock(rubble, v(-3.4 + i * 0.58, 0.18, 2.35), v(0.55, 0.36, 0.42), paint, random, { chamfer: 0.035, rough: 0.015, turn: between(random, -0.06, 0.06) });
    }
    for (let i = 0; i < 2; i++) {
      addBlock(rubble, v(-3.1 + i * 0.6, 0.53, 2.35), v(0.55, 0.34, 0.4), paint, random, { chamfer: 0.035, rough: 0.015, turn: between(random, -0.08, 0.08) });
    }
    for (const [x, z, turn, tipX, tipZ] of [
      [1.9, 2.3, 0.5, 0.25, 0.1],
      [-0.9, 2.9, -0.3, -0.12, 0.2],
      [2.9, 1.3, 1.1, 0.1, -0.3],
      [-2.4, -1.5, 0.2, 0.3, 0],
    ]) {
      addBlock(rubble, v(x, 0.16, z), v(0.62, 0.38, 0.45), paint, random, { turn, tip: [tipX, tipZ], chamfer: 0.04, rough: 0.02 });
    }
    // Paving stones in the grass in front.
    for (let k = 0; k < 9; k++) {
      const x = between(random, -2.2, 2.4);
      const z = between(random, 1.9, 3.6);
      const w = between(random, 0.35, 0.6);
      addBlock(rubble, v(x, 0.02, z), v(w, 0.08, w * between(random, 0.7, 1.1)), paint, random, { turn: random() * Math.PI, chamfer: 0.025, rough: 0.012 });
    }

    // Grass, moss and a few flowers among the stones.
    const green = new Shape();
    if (options.dressing ?? true) {
      for (let k = 0; k < 12; k++) {
        const x = between(random, -3.8, 3.8);
        const z = between(random, -1.6, 3.6);
        if (Math.abs(x) < 2.2 && z > -1.1 && z < 1.7) continue; // not on the floor or steps
        addTuft(green, v(x, 0, z), 9, between(random, 0.2, 0.34), random, 0, 0, P);
      }
      for (const [x, z] of [
        [-3.25, 0.35],
        [3.2, -0.2],
        [-2.1, 2.9],
        [2.4, 2.1],
      ]) {
        addMoss(green, v(x + between(random, -0.6, 0.6), 0, z + between(random, 0.3, 0.6)), between(random, 0.2, 0.34), random, P);
      }
      const petals = [P.petalPink, P.petalWhite, P.petalLight, P.petalLilac];
      for (let k = 0; k < 6; k++) {
        const foot = v(between(random, -3.6, 3.6), -0.01, between(random, 1.8, 3.8));
        const head = foot.clone().add(v(0, between(random, 0.14, 0.26), 0));
        addTube(green, [foot, head], [0.005, 0.004], { sides: 3, random, paint: () => color(P.grass) });
        addFlower(green, head, v(0, 1, 0.3), 6, 0.045, color(petals[k % petals.length]), color(P.flowerHeart), random);
      }
      if (season === 'autumn') {
        addFallenLeaves(green, v(-2.6, 0, 2.8), 1.4, 16, random);
        addFallenLeaves(green, v(2.4, 0, 2.4), 1.2, 12, random);
        addFallenLeaves(green, v(0, 0, 3.1), 1.5, 14, random);
      }
    }

    this.arch = mesh(arch.geometry(), matte());
    this.rubble = mesh(rubble.geometry(), matte());
    this.dressing = mesh(green.geometry(), matte());
    this.add(this.arch, this.rubble, this.dressing);
    if (season === 'winter') snowOn(this, random, 0.4, 0.75);
  }
}
