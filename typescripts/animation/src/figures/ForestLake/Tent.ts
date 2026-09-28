import * as THREE from 'three';
import { addBar, addPole, addTube, between, color, frame, matte, mesh, PALETTE, palette, seededRandom, Shape, shade, snowOn, twoSided, vary, woodColors, type Season } from './parts';

export interface TentOptions {
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's tent (Tent.md), as its asset sheet and the camp area
// draw it: a cream canvas ridge tent, its front (+z) open with the flaps
// tied back, its canvas sagging a little between the ridge and the ground,
// darker toward the ground and at its seams. Two poles at each end cross
// over the ridge and stand out past it, a ridge pole runs along the top,
// and guy ropes run out to pegs in the ground. As the reference's seasons
// show it: its canvas orange in autumn, and snow on its ridge in winter.
export class Tent extends THREE.Group {
  static readonly WIDTH = 2.3;
  static readonly LENGTH = 2.5;
  static readonly HEIGHT = 1.8;

  readonly canvas: THREE.Mesh;
  readonly poles: THREE.Mesh; // with the pegs
  readonly ropes: THREE.Mesh;

  constructor(options: TentOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 131);
    const W = Tent.WIDTH;
    const L = Tent.LENGTH;
    const H = Tent.HEIGHT;
    const eave = 0.02;
    const P = palette(options.season);

    // The canvas: each side a sagging sheet from the ridge to the ground.
    const cloth = new Shape();
    const light = color(P.canvas);
    const dim = color(P.canvasShade);
    const U = 6;
    const V = 4;
    const point = (side: number, u: number, v: number) => {
      const sag = 0.06 * Math.sin(Math.PI * u) * Math.sin(Math.PI * v);
      return new THREE.Vector3(side * ((W / 2) * v - sag * 0.55), H - (H - eave) * v - sag * 0.8, -L / 2 + L * u);
    };
    // Its colour at (u, v): darker toward the ground and along the seams.
    const tones = Array.from({ length: U + 1 }, () => between(random, 0.97, 1.03));
    const paint = (u: number, v: number) => {
      const seam = Math.abs(u * U - 2) < 0.01 || Math.abs(u * U - 4) < 0.01 ? 0.86 : 1;
      return light.clone().lerp(dim, v * 0.55).multiplyScalar(seam * tones[Math.round(u * U)]);
    };
    for (const side of [1, -1]) {
      for (let i = 0; i < U; i++) {
        for (let j = 0; j < V; j++) {
          const [u0, u1, v0, v1] = [i / U, (i + 1) / U, j / V, (j + 1) / V];
          const a = point(side, u0, v0);
          const b = point(side, u1, v0);
          const c = point(side, u1, v1);
          const d = point(side, u0, v1);
          // Faces out: along the tent, then down the side.
          if (side > 0) cloth.quad(a, b, c, d, paint(u0, v0), paint(u1, v0), paint(u1, v1), paint(u0, v1));
          else cloth.quad(a, d, c, b, paint(u0, v0), paint(u0, v1), paint(u1, v1), paint(u1, v0));
        }
      }
    }
    // The back wall, facing -z, and the groundsheet inside.
    const back = [new THREE.Vector3(-W / 2, eave, -L / 2), new THREE.Vector3(0, H, -L / 2), new THREE.Vector3(W / 2, eave, -L / 2)];
    cloth.triangle(back[0], back[1], back[2], shade(dim, 0.95), light, shade(dim, 0.95));
    const sheet = shade(dim, 0.5);
    cloth.quad(
      new THREE.Vector3(-W / 2 + 0.06, 0.015, L / 2 - 0.02),
      new THREE.Vector3(W / 2 - 0.06, 0.015, L / 2 - 0.02),
      new THREE.Vector3(W / 2 - 0.06, 0.015, -L / 2 + 0.02),
      new THREE.Vector3(-W / 2 + 0.06, 0.015, -L / 2 + 0.02),
      sheet,
    );
    // The front flaps, folded back to either side and tied, their insides
    // showing, a shade darker.
    const inner = shade(light, 0.9);
    for (const side of [1, -1]) {
      const top = new THREE.Vector3(0.03 * side, H - 0.04, L / 2 + 0.01);
      const foot = new THREE.Vector3((W / 2) * side, eave, L / 2 + 0.01);
      const bend = new THREE.Vector3(side * (W / 2) * 0.42, H * 0.5, L / 2 + 0.2);
      const tie = new THREE.Vector3(side * (W / 2) * 0.78, H * 0.3, L / 2 + 0.26);
      cloth.triangle(top, foot, bend, inner, shade(dim, 0.92), inner);
      cloth.triangle(bend, foot, tie, inner, shade(dim, 0.92), shade(inner, 0.95));
    }

    // The poles: two at each end, crossing over the ridge and standing out
    // past it, and the ridge pole along the top; pegs for the ropes.
    const wood = new Shape();
    const crossings = [L / 2 + 0.04, -L / 2 - 0.04].map((z) => new THREE.Vector3(0, H + 0.02, z));
    for (const cross of crossings) {
      for (const side of [1, -1]) {
        const foot = new THREE.Vector3(side * (W / 2) * 0.93, -0.08, cross.z);
        const past = cross.clone().sub(foot).normalize().multiplyScalar(0.34).add(cross);
        addPole(wood, foot, past, 0.03, random, 0.85, PALETTE.post, 6);
      }
    }
    addPole(wood, new THREE.Vector3(0, H + 0.05, -L / 2 - 0.3), new THREE.Vector3(0, H + 0.05, L / 2 + 0.3), 0.028, random, 1, PALETTE.post, 6);
    const pegs: THREE.Vector3[] = [
      new THREE.Vector3(0, 0, L / 2 + 1.05),
      new THREE.Vector3(0, 0, -L / 2 - 1.05),
      ...[-1, 1].flatMap((sx) => [-1, 1].map((sz) => new THREE.Vector3(sx * (W / 2 + 0.55), 0, sz * L * 0.4))),
    ];
    for (const peg of pegs) {
      addBar(wood, frame(peg.clone().add(new THREE.Vector3(0, -0.12, 0)), new THREE.Vector3(0, 1, 0.25), new THREE.Vector3(1, 0, 0)), {
        length: 0.2,
        width: 0.035,
        height: 0.035,
        chamfer: 0.006,
        bevel: 0.004,
        segments: 1,
        caps: [false, true],
        colors: woodColors(PALETTE.wood),
        random,
      });
    }

    // The guy ropes: from each end's crossing out to its peg, and from low
    // on each side out to the side pegs, each sagging a little.
    const cords = new Shape();
    const rope = color(PALETTE.rope);
    const sagging = (from: THREE.Vector3, to: THREE.Vector3, sag: number) => {
      const path = Array.from({ length: 6 }, (_, i) => {
        const t = i / 5;
        return from.clone().lerp(to, t).add(new THREE.Vector3(0, -sag * 4 * t * (1 - t), 0));
      });
      addTube(cords, path, path.map(() => 0.009), { sides: 4, random, paint: () => vary(rope, random, 0.6) });
    };
    sagging(crossings[0], pegs[0].clone().setY(0.1), 0.05);
    sagging(crossings[1], pegs[1].clone().setY(0.1), 0.05);
    for (const peg of pegs.slice(2)) {
      const on = point(Math.sign(peg.x), (peg.z + L / 2) / L, 0.8);
      sagging(on, peg.clone().setY(0.1), 0.03);
    }

    this.canvas = mesh(cloth.geometry(), twoSided());
    this.poles = mesh(wood.geometry(), matte());
    this.ropes = mesh(cords.geometry(), matte());
    this.add(this.canvas, this.poles, this.ropes);
    if (options.season === 'winter') snowOn(this, random, 0.4, 0.75);
  }
}
