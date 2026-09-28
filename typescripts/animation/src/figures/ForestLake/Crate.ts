import * as THREE from 'three';
import { addBar, addBeam, addBlock, color, frame, matte, mesh, PALETTE, seededRandom, Shape, snowOn, vary, woodColors, type BarColors, type Season } from './parts';

export interface CrateOptions {
  size?: number; // m, each side; 0.6 by default
  season?: Season; // spring by default; snow on its lid in winter
  seed?: number;
}

// The forest lake's crate (Crate.md; half of the asset sheet's "Crate &
// Barrel"): a wooden box of pale planks laid across each side within a
// darker frame along its edges, a brace running corner to corner across
// its front and back, and nails where they meet.
export class Crate extends THREE.Group {
  readonly box: THREE.Mesh;

  constructor(options: CrateOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 151);
    const size = options.size ?? 0.6;
    const h = size / 2;
    const fw = size * 0.11; // the frame's width
    const plank = 0.02; // the planks' thickness
    const up = new THREE.Vector3(0, 1, 0);
    const shape = new Shape();

    // The dark inside, showing between the planks.
    const gap = color(PALETTE.woodGroove);
    addBlock(shape, new THREE.Vector3(0, h, 0), new THREE.Vector3(size - 0.05, size - 0.05, size - 0.05), () => gap, random, { chamfer: 0.005, rough: 0 });

    // The planks: three across each side and the top, inside the frame.
    const planks = woodColors(PALETTE.wood);
    const span = size - 2 * fw;
    const gapWidth = 0.012;
    const wide = (span + 2 * fw * 0.5 - 2 * gapWidth) / 3;
    const face = h - 0.012 - plank / 2; // how far the planks' middles stand from the crate's
    const sides: { out: THREE.Vector3; across: THREE.Vector3; up: THREE.Vector3 }[] = [
      { out: new THREE.Vector3(0, 0, 1), across: new THREE.Vector3(1, 0, 0), up },
      { out: new THREE.Vector3(0, 0, -1), across: new THREE.Vector3(-1, 0, 0), up },
      { out: new THREE.Vector3(1, 0, 0), across: new THREE.Vector3(0, 0, -1), up },
      { out: new THREE.Vector3(-1, 0, 0), across: new THREE.Vector3(0, 0, 1), up },
      { out: new THREE.Vector3(0, 1, 0), across: new THREE.Vector3(1, 0, 0), up: new THREE.Vector3(0, 0, -1) },
    ];
    for (const { out, across, up: rows } of sides) {
      const middle = new THREE.Vector3(0, h, 0).addScaledVector(out, face);
      for (let k = 0; k < 3; k++) {
        const offset = (k - 1) * (wide + gapWidth);
        const at = middle.clone().addScaledVector(rows, offset);
        const reach = h - fw * 0.5;
        addBeam(shape, at.clone().addScaledVector(across, -reach), at.clone().addScaledVector(across, reach), rows, wide, plank, random, {
          colors: { ...planks, side: vary(planks.side, random, 1.5) },
          chamfer: 0.004,
          bevel: 0.003,
          segments: 2,
        });
      }
    }

    // The frame along its twelve edges.
    const frameColors = woodColors(PALETTE.post);
    const e = h - fw / 2;
    const bar = (from: THREE.Vector3, to: THREE.Vector3, facing: THREE.Vector3) =>
      addBeam(shape, from, to, facing, fw, fw, random, { colors: frameColors, chamfer: fw * 0.18, bevel: fw * 0.12, segments: 2, rough: 0.002 });
    for (const [x, z] of [
      [-e, -e],
      [e, -e],
      [e, e],
      [-e, e],
    ]) {
      bar(new THREE.Vector3(x, 0, z), new THREE.Vector3(x, size, z), new THREE.Vector3(0, 0, 1));
    }
    for (const y of [fw / 2, size - fw / 2]) {
      for (const z of [-e, e]) bar(new THREE.Vector3(-h + fw, y, z), new THREE.Vector3(h - fw, y, z), up);
      for (const x of [-e, e]) bar(new THREE.Vector3(x, y, -h + fw), new THREE.Vector3(x, y, h - fw), up);
    }
    // A brace corner to corner across the front and the back.
    for (const z of [1, -1]) {
      const from = new THREE.Vector3(-h + fw, fw, z * (h - 0.004));
      const to = new THREE.Vector3(h - fw, size - fw, z * (h - 0.004));
      addBeam(shape, from, to, new THREE.Vector3(-1, 1, 0), fw * 0.85, 0.022, random, { colors: frameColors, chamfer: 0.005, bevel: 0.004, segments: 2 });
    }
    // Nails at the frame's corners on the front.
    const nails: BarColors = { side: color(PALETTE.nail), groove: color(PALETTE.nail), end: color(PALETTE.nail), core: color(PALETTE.nail) };
    for (const [x, y] of [
      [-e, fw / 2],
      [e, fw / 2],
      [-e, size - fw / 2],
      [e, size - fw / 2],
    ]) {
      addBar(shape, frame(new THREE.Vector3(x, y, h - 0.003), new THREE.Vector3(0, 0, 1), up), {
        length: 0.008,
        width: 0.016,
        height: 0.016,
        chamfer: 0.004,
        bevel: 0.002,
        segments: 1,
        caps: [false, true],
        colors: nails,
        random,
      });
    }

    this.box = mesh(shape.geometry(), matte());
    this.add(this.box);
    if (options.season === 'winter') snowOn(this, random);
  }
}
