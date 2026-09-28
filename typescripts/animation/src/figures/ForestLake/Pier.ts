import * as THREE from 'three';
import { LampPost } from './LampPost';
import { addBeam, addTube, between, color, grain, matte, mesh, PALETTE, seededRandom, Shape, shade, snowOn, vary, woodColors, type Season } from './parts';

export interface PierOptions {
  length?: number; // m, from the shore out along x; 7 by default
  width?: number; // m, across its deck; 1.9 by default
  height?: number; // m, its deck's top over the water (y 0); 0.5 by default
  depth?: number; // m its piles go down under the water; 1.8 by default
  // A lamp post on the shore at its landward corner, its lantern hanging
  // over the deck, as the reference's pier has; `shore` is the ground's
  // height there. True, at 0.3 m, by default.
  lamp?: boolean;
  shore?: number;
  season?: Season; // spring by default; snow on its deck in winter
  seed?: number;
}

// The forest lake's pier (Pier.md; the reference's "Dock / Pier"): a deck of
// planks on beams, running out from the shore (x 0) over the water on round
// piles, darker and wet where they go into the water; the piles along one
// side stand up past the deck with a rope slung between them, and a lamp
// post stands on the shore at its landward corner. The water is at y 0.
export class Pier extends THREE.Group {
  readonly deck: THREE.Mesh; // planks and beams
  readonly piles: THREE.Mesh;
  readonly ropes: THREE.Mesh;
  readonly lamp: LampPost | null;
  readonly length: number;
  readonly width: number;
  readonly height: number;

  constructor(options: PierOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 181);
    const L = (this.length = options.length ?? 7);
    const W = (this.width = options.width ?? 1.9);
    const H = (this.height = options.height ?? 0.5);
    const depth = options.depth ?? 1.8;
    const up = new THREE.Vector3(0, 1, 0);

    // Where the piles stand along it: its ends and about every 2.2 m.
    const bays = Math.max(1, Math.round((L - 0.4) / 2.2));
    const xs = Array.from({ length: bays + 1 }, (_, k) => 0.2 + ((L - 0.45) * k) / bays);
    const sides = [-(W / 2 - 0.05), W / 2 - 0.05];

    // The piles, wet and dark near and under the water; those on the +z
    // side stand up past the deck for the rope.
    const piles = new Shape();
    const dry = color(PALETTE.post);
    const wet = color(PALETTE.woodDark).multiplyScalar(0.75);
    for (const z of sides) {
      for (const x of xs) {
        const tall = z > 0 ? H + 0.5 : H - 0.05;
        const steps = 6;
        const path = Array.from({ length: steps + 1 }, (_, i) => new THREE.Vector3(x + between(random, -0.01, 0.01), -depth + ((tall + depth) * i) / steps, z));
        const shades = Array.from({ length: 8 }, () => between(random, 0.9, 1.08));
        addTube(piles, path, path.map((_, i) => 0.105 - 0.01 * (i / steps)), {
          sides: 8,
          rough: 0.04,
          random,
          paint: (r, i) => {
            const y = path[r].y;
            const base = y < 0.12 ? wet : y < 0.3 ? wet.clone().lerp(dry, (y - 0.12) / 0.18) : dry;
            return shade(base, shades[i]);
          },
          caps: ['open', 'rings'],
        });
      }
    }

    // The beams: two along it under the deck's edges, one across at each
    // pair of piles.
    const deck = new Shape();
    const beam = woodColors(PALETTE.woodDark);
    for (const z of [-(W / 2 - 0.16), W / 2 - 0.16]) {
      addBeam(deck, new THREE.Vector3(-0.05, H - 0.045 - 0.08, z), new THREE.Vector3(L + 0.02, H - 0.045 - 0.08, z), up, 0.16, 0.12, random, { colors: beam, cracks: [] });
    }
    for (const x of xs) {
      addBeam(deck, new THREE.Vector3(x + 0.13, H - 0.3, -W / 2 - 0.02), new THREE.Vector3(x + 0.13, H - 0.3, W / 2 + 0.02), up, 0.16, 0.1, random, { colors: beam, cracks: [] });
    }
    // The planks across the beams, each a little askew.
    const planks = woodColors(PALETTE.wood);
    const pitch = 0.245;
    const count = Math.floor(L / pitch);
    for (let k = 0; k < count; k++) {
      const x = 0.12 + ((L - 0.24) * k) / (count - 1);
      const skew = between(random, -0.02, 0.02);
      const y = H - 0.0225 + between(random, -0.005, 0.005);
      addBeam(
        deck,
        new THREE.Vector3(x - skew, y, -W / 2 - between(random, 0.02, 0.06)),
        new THREE.Vector3(x + skew, y, W / 2 + between(random, 0.02, 0.06)),
        up,
        0.045,
        0.215,
        random,
        { colors: { ...planks, side: vary(planks.side, random, 1.6) }, cracks: grain(random, [1], [0, 2]), segments: 3 },
      );
    }

    // A rope slung between the tall piles, wrapped round each.
    const cords = new Shape();
    const rope = color(PALETTE.rope);
    const paint = () => vary(rope, random, 0.6);
    const z = sides[1];
    const ropeY = H + 0.38;
    for (let k = 0; k + 1 < xs.length; k++) {
      const a = new THREE.Vector3(xs[k], ropeY, z);
      const b = new THREE.Vector3(xs[k + 1], ropeY, z);
      const path = Array.from({ length: 9 }, (_, i) => {
        const t = i / 8;
        return a.clone().lerp(b, t).add(new THREE.Vector3(0, -0.14 * 4 * t * (1 - t), 0));
      });
      addTube(cords, path, path.map(() => 0.018), { sides: 5, random, paint });
    }
    for (const x of xs) {
      const wrap = Array.from({ length: 9 }, (_, i) => {
        const angle = (2 * Math.PI * i) / 8;
        return new THREE.Vector3(x + Math.cos(angle) * 0.118, ropeY + (i / 8) * 0.035, z + Math.sin(angle) * 0.118);
      });
      addTube(cords, wrap, wrap.map(() => 0.017), { sides: 4, random, paint });
    }

    this.deck = mesh(deck.geometry(), matte());
    this.piles = mesh(piles.geometry(), matte());
    this.ropes = mesh(cords.geometry(), matte());
    this.add(this.deck, this.piles, this.ropes);

    // The lamp post on the shore at its landward corner, its arm reaching
    // out over the deck.
    this.lamp = null;
    if (options.lamp ?? true) {
      const lamp = new LampPost({ seed: Math.floor(random() * 1000), tufts: false, season: options.season });
      lamp.position.set(0.35, options.shore ?? 0.3, -(W / 2 + 0.22));
      lamp.rotation.y = -Math.PI / 2 + 0.35;
      this.lamp = lamp;
      this.add(lamp);
    }
    if (options.season === 'winter') snowOn(this, random);
  }

  // The deck's top under a point (in the pier's own coordinates), or NaN
  // off it.
  deckAt(x: number, z: number): number {
    return x >= -0.05 && x <= this.length + 0.05 && Math.abs(z) <= this.width / 2 + 0.05 ? this.height : NaN;
  }

  update(delta: number): void {
    this.lamp?.update(delta);
  }
}
