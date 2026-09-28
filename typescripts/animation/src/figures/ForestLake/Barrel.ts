import * as THREE from 'three';
import { addBeam, addLathe, color, matte, mesh, PALETTE, seededRandom, Shape, shade, snowOn, vary, woodColors, type Season } from './parts';

export interface BarrelOptions {
  height?: number; // m; 0.9 by default
  season?: Season; // spring by default; snow on its lid in winter
  seed?: number;
}

// The forest lake's barrel (Barrel.md; half of the asset sheet's "Crate &
// Barrel"): wooden staves bellied out round its middle, each its own shade
// of orange-brown, held by four dark iron hoops, its top a lid of planks
// sunk a little inside the staves' rim.
export class Barrel extends THREE.Group {
  readonly staves: THREE.Mesh; // with the lid
  readonly hoops: THREE.Mesh;

  constructor(options: BarrelOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 161);
    const H = options.height ?? 0.9;
    const s = H / 0.9;
    const sides = 14;
    const radius = (y: number) => s * (0.27 + 0.065 * Math.sin((Math.PI * y) / H));

    // The staves: up the outside, over the rim and down to the lid.
    const wood = new Shape();
    const staveColors = Array.from({ length: sides }, (_, i) => vary(shade(color(PALETTE.wood), i % 2 ? 1.05 : 0.93), random, 1.2));
    const heights = [0, 0.12, 0.3, 0.5, 0.7, 0.88, 1].map((f) => f * H);
    const rim = 0.025 * s;
    const lid = H - 0.035 * s;
    const profile: [number, number][] = [[0, 0.01], ...heights.map((y) => [radius(y), y] as [number, number]), [radius(H) - rim, H], [radius(H) - rim, lid], [0, lid]];
    const edge = heights.length; // the rim's top face runs from profile point `edge` to `edge + 1`
    addLathe(wood, profile, sides, new THREE.Matrix4(), (r, i) => {
      if (r === 0) return shade(staveColors[i], 0.6);
      if (r === edge) return color(PALETTE.woodEnd);
      if (r >= edge + 1) return shade(color(PALETTE.woodGroove), 1.2);
      return staveColors[i];
    });
    // The lid's planks, lying across the top.
    const planks = woodColors(PALETTE.wood);
    const inner = radius(H) - rim - 0.004;
    const wide = (inner * 2) / 3 - 0.01;
    for (const k of [-1, 0, 1]) {
      const z = k * (wide + 0.01);
      const reach = Math.sqrt(Math.max(0, inner * inner - (Math.abs(z) + wide / 2) ** 2));
      addBeam(wood, new THREE.Vector3(-reach, lid + 0.008, z), new THREE.Vector3(reach, lid + 0.008, z), new THREE.Vector3(0, 0, 1), wide, 0.016, random, {
        colors: { ...planks, side: vary(planks.side, random, 1.4) },
        chamfer: 0.003,
        bevel: 0.002,
        segments: 1,
        cracks: [],
      });
    }

    // The hoops, a little proud of the staves.
    const iron = new Shape();
    const ironColor = color(PALETTE.iron);
    for (const f of [0.07, 0.27, 0.73, 0.93]) {
      const y0 = f * H - 0.024 * s;
      const y1 = f * H + 0.024 * s;
      addLathe(
        iron,
        [
          [radius(y0) + 0.004, y0],
          [radius(y0) + 0.01 * s, y0 + 0.006],
          [radius(y1) + 0.01 * s, y1 - 0.006],
          [radius(y1) + 0.004, y1],
        ],
        sides,
        new THREE.Matrix4(),
        (r) => (r === 1 ? shade(ironColor, 1.15) : ironColor),
      );
    }

    this.staves = mesh(wood.geometry(), matte());
    this.hoops = mesh(iron.geometry(), matte());
    this.add(this.staves, this.hoops);
    if (options.season === 'winter') snowOn(this, random);
  }
}
