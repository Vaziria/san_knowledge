import * as THREE from 'three';
import { addBar, addBeam, addTube, between, color, frame, grain, matte, mesh, PALETTE, seededRandom, Shape, snowOn, vary, woodColors, type Season } from './parts';

export interface BridgeOptions {
  length?: number; // m, end to end along x; 2.4 by default, one module as the asset sheet draws it
  width?: number; // m, across its deck; 1.6 by default
  arch?: number; // m its deck rises at its middle over its ends; 0 by default
  season?: Season; // spring by default; snow on its deck and posts in winter
  seed?: number;
}

// The forest lake's wooden bridge (Bridge.md; the asset sheet's "Bridge
// Module"): a deck of planks laid across two beams, each plank a little
// askew, with square posts along both sides and ropes slung between their
// tops, sagging a little, and a lower rope below them. It runs along x;
// longer, arched, it spans the stream below the lake.
export class Bridge extends THREE.Group {
  static readonly DECK = 0.35; // m, the deck's top over the ground at its ends

  readonly deck: THREE.Mesh; // the planks and the beams under them
  readonly posts: THREE.Mesh;
  readonly ropes: THREE.Mesh;
  private readonly length: number;
  private readonly width: number;
  private readonly arch: number;

  constructor(options: BridgeOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 171);
    const L = (this.length = options.length ?? 2.4);
    const W = (this.width = options.width ?? 1.6);
    this.arch = options.arch ?? 0;
    const up = new THREE.Vector3(0, 1, 0);
    const top = (x: number) => this.curve(x);

    // Two beams under the deck, along its curve.
    const deck = new Shape();
    const beams = 8;
    for (const z of [-(W / 2 - 0.14), W / 2 - 0.14]) {
      for (let k = 0; k < beams; k++) {
        const x0 = -L / 2 + (L * k) / beams;
        const x1 = -L / 2 + (L * (k + 1)) / beams;
        addBeam(deck, new THREE.Vector3(x0 - (k === 0 ? 0.05 : 0.01), top(x0) - 0.13, z), new THREE.Vector3(x1 + (k === beams - 1 ? 0.05 : 0.01), top(x1) - 0.13, z), up, 0.17, 0.12, random, {
          colors: woodColors(PALETTE.woodDark),
          cracks: [],
          caps: [k === 0, k === beams - 1],
        });
      }
    }
    // The planks across them, each a little askew.
    const pitch = 0.235;
    const count = Math.max(2, Math.floor(L / pitch));
    const planks = woodColors(PALETTE.wood);
    for (let k = 0; k < count; k++) {
      const x = -L / 2 + pitch / 2 + ((L - pitch) * k) / (count - 1);
      const slope = (top(x + 0.01) - top(x - 0.01)) / 0.02;
      const normal = new THREE.Vector3(-slope, 1, 0).normalize();
      const y = top(x) - 0.0225 + between(random, -0.006, 0.006);
      const skew = between(random, -0.025, 0.025);
      addBeam(
        deck,
        new THREE.Vector3(x - skew, y, -W / 2 - between(random, 0.02, 0.07)),
        new THREE.Vector3(x + skew, y, W / 2 + between(random, 0.02, 0.07)),
        normal,
        0.045,
        0.2,
        random,
        { colors: { ...planks, side: vary(planks.side, random, 1.6) }, cracks: grain(random, [1], [0, 2]), segments: 3 },
      );
    }

    // Posts along both sides, at the ends and no more than 2.4 m apart.
    const posts = new Shape();
    const spans = Math.max(1, Math.ceil((L - 0.2) / 2.4));
    const xs = Array.from({ length: spans + 1 }, (_, k) => -L / 2 + 0.1 + ((L - 0.2) * k) / spans);
    const postTop = (x: number) => top(x) + 0.95;
    for (const z of [-(W / 2 + 0.03), W / 2 + 0.03]) {
      for (const x of xs) {
        addBar(posts, frame(new THREE.Vector3(x, top(x) - 0.55, z), up, new THREE.Vector3(0, 0, 1)), {
          length: postTop(x) - top(x) + 0.55,
          width: 0.12,
          height: 0.12,
          chamfer: 0.02,
          bevel: 0.014,
          segments: 3,
          rough: 0.003,
          slant: [between(random, -0.08, 0.08), between(random, -0.08, 0.08)],
          cracks: grain(random, [0, 1, 2, 3], [1, 1], 0.4),
          caps: [false, true],
          colors: woodColors(PALETTE.post),
          random,
        });
      }
    }

    // Ropes slung between the posts' tops, and lower ones, each wrapped
    // round the posts.
    const cords = new Shape();
    const rope = color(PALETTE.rope);
    const paint = () => vary(rope, random, 0.6);
    for (const z of [-(W / 2 + 0.03), W / 2 + 0.03]) {
      for (const [below, sag] of [
        [0.08, 0.13],
        [0.48, 0.08],
      ]) {
        for (let k = 0; k + 1 < xs.length; k++) {
          const a = new THREE.Vector3(xs[k], postTop(xs[k]) - below, z);
          const b = new THREE.Vector3(xs[k + 1], postTop(xs[k + 1]) - below, z);
          const path = Array.from({ length: 9 }, (_, i) => {
            const t = i / 8;
            return a.clone().lerp(b, t).add(new THREE.Vector3(0, -sag * 4 * t * (1 - t), 0));
          });
          addTube(cords, path, path.map(() => 0.017), { sides: 5, random, paint });
        }
        for (const x of xs) {
          const center = new THREE.Vector3(x, postTop(x) - below, z);
          const wrap = Array.from({ length: 9 }, (_, i) => {
            const angle = (2 * Math.PI * i) / 8;
            return center.clone().add(new THREE.Vector3(Math.cos(angle) * 0.078, (i / 8) * 0.03, Math.sin(angle) * 0.078));
          });
          addTube(cords, wrap, wrap.map(() => 0.016), { sides: 4, random, paint });
        }
      }
    }

    this.deck = mesh(deck.geometry(), matte());
    this.posts = mesh(posts.geometry(), matte());
    this.ropes = mesh(cords.geometry(), matte());
    this.add(this.deck, this.posts, this.ropes);
    if (options.season === 'winter') snowOn(this, random);
  }

  // The deck's top at x along it and z across it, over its own ground (y 0
  // at its ends), or NaN off the deck.
  deckAt(x: number, z = 0): number {
    if (Math.abs(x) > this.length / 2 + 0.05 || Math.abs(z) > this.width / 2 + 0.05) return NaN;
    return this.curve(x);
  }

  // The deck's top along its length, arching up to its middle.
  private curve(x: number): number {
    const t = (2 * x) / this.length;
    return Bridge.DECK + this.arch * (1 - t * t);
  }
}
