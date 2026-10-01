import * as THREE from 'three';
import { between, color, matte, mesh, palette, seededRandom, shade, Shape, vary, type Season } from '../parts';
import { addCable, addSlab, addStrut, tone } from './parts';

export interface WingModuleOptions {
  side: -1 | 1; // which way it reaches: -z or +z
  broken?: boolean; // torn off at its root; false by default
  season?: Season; // spring by default
  seed?: number;
}

const SPAN = 6.8; // m, root to tip
const ROOT_LEAD = -2.0; // m along x: its leading edge at the root...
const ROOT_TRAIL = 2.0; // ...and its trailing edge
const TIP_LEAD = 0.8; // at its tip, swept back
const TIP_TRAIL = 2.4;
const THICK: readonly [number, number] = [0.42, 0.14];

// A wing of the spaceship wreck (SpaceshipWreck.md; "Wing Module (Left)"
// and "(Right)" on the sheets): a flat slab, swept back and tapering to its
// tip, plated on both faces, pale with its tip orange and orange along its
// leading edge toward the tip, a dark band at its root; a flap along its
// trailing edge hanging down, torn half off its hinge. Broken, its root is
// torn ragged, the spars sticking out of it and a cable hanging. Its origin
// is the middle of its root; it reaches along `side` z, its leading edge
// toward -x, its top up.
export class WingModule extends THREE.Group {
  readonly slab: THREE.Mesh; // the wing, its flap, its torn spars

  constructor(options: WingModuleOptions) {
    super();
    const random = seededRandom(options.seed ?? (options.side > 0 ? 521 : 523));
    const P = palette(options.season ?? 'spring');
    const side = options.side;
    const broken = options.broken ?? false;
    const shape = new Shape();
    const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);
    const rows = 6;
    const columns = 4;
    addSlab(shape, {
      root: [v(ROOT_LEAD, 0, 0), v(ROOT_TRAIL, 0, 0)],
      tip: [v(TIP_LEAD, 0, side * SPAN), v(TIP_TRAIL, 0, side * SPAN)],
      normal: v(0, 1, 0),
      thick: THICK,
      rows,
      columns,
      paint: (i, j, top) => {
        if (i === rows - 1) return tone(random() < 0.85 ? 'orange' : 'orangeDeep');
        if (i === 0 && !broken) return vary(tone('metal'), random, 1);
        if (j === 0 && i >= 3) return tone('orange');
        if (top && random() < 0.04) return vary(color(P.moss), random, 1.4);
        return random() < 0.08 ? tone('metal') : shade(tone('plate'), random() < 0.3 ? 0.9 : 1);
      },
      state: (i) => {
        if (broken && i === 0) return random() < 0.25 ? 'hole' : random() < 0.35 ? 'bent' : 'panel';
        return random() < 0.05 ? 'hole' : random() < 0.04 ? 'bent' : 'panel';
      },
      torn: broken ? 0.8 : 0,
      random,
    });

    // The flap along its trailing edge, from 2.2 m out to 5.2 m, hanging
    // down from its hinge, farther at its torn outer end.
    const trail = (s: number) => v(ROOT_TRAIL + (TIP_TRAIL - ROOT_TRAIL) * s, 0, side * SPAN * s);
    const s0 = 2.2 / SPAN;
    const s1 = 5.2 / SPAN;
    const droop = (s: number) => 0.3 + 0.35 * ((s - s0) / (s1 - s0));
    const flapEnd = (s: number) => trail(s).add(v(0.75 * Math.cos(droop(s)), -0.75 * Math.sin(droop(s)), 0));
    addSlab(shape, {
      root: [trail(s0).add(v(0.03, -0.04, 0)), flapEnd(s0)],
      tip: [trail(s1).add(v(0.03, -0.04, 0)), flapEnd(s1)],
      normal: v(Math.sin(0.45), Math.cos(0.45), 0),
      thick: [0.1, 0.08],
      rows: 3,
      columns: 1,
      paint: (i) => (i === 2 ? tone('orange') : shade(tone('plate'), random() < 0.4 ? 0.9 : 1)),
      random,
    });

    // Torn off: its spars sticking out of the root, and a cable.
    if (broken) {
      for (const x of [-1.1, 0.1, 1.2]) {
        const out = between(random, 0.35, 0.8);
        addStrut(shape, v(x, 0, side * 0.3), v(x + between(random, -0.1, 0.1), between(random, -0.08, 0.08), -side * out), v(0, 1, 0), 0.22, 0.1, vary(tone('metal'), random, 1), random);
      }
      addCable(shape, [v(0.6, 0, side * 0.2), v(0.8, -0.05, -side * 0.5), v(1.1, -0.2, -side * 1.0)], 0.035, random, 1);
    }
    this.slab = mesh(shape.geometry(), matte());
    this.add(this.slab);
  }
}
