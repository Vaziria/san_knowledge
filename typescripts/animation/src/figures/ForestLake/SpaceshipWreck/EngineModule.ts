import * as THREE from 'three';
import { addBlock, addLathe, addTube, between, both, color, glowing, halo, matte, mesh, noise2, palette, seededRandom, shade, Shape, vary, type Season } from '../parts';
import { addDisc, addSkin, addStrut, bakeLamps, facePaint, tone, WRECK, type CellState } from './parts';

export interface EngineModuleOptions {
  season?: Season; // spring by default
  seed?: number;
}

export const ENGINE_RADIUS = 2.2; // m, its casing's
const LENGTH = 3.8; // m, its casing, front to the intake ring
const WALL = 0.48; // the casing's depth: its inside is the duct
const COLUMNS = 18;
const ROWS = 4;
const BLADES = 18;
const FAN = LENGTH - 0.6; // m along it, the fan
const CORE = LENGTH - 1.25; // m along it, the stator and the core behind it

// The spaceship wreck's engine (SpaceshipWreck.md; the sheet "Engine
// Module"): a round casing of plates, dark rings at its ends, a cooling
// pipe along each upper side; at its back a thick intake ring round the
// duct, a fan of eighteen twisted blades on a hub inside it, and behind the
// fan the stator's vanes and the core, still glowing a dull orange. Its
// origin is the middle of its front end, on its axis; it runs along +x to
// its fan.
export class EngineModule extends THREE.Group {
  readonly casing: THREE.Mesh;
  readonly duct: THREE.Mesh; // the duct, the fan and the stator, lit by the core
  readonly core: THREE.Mesh;
  readonly glow: THREE.Sprite;
  private time = 0;

  constructor(options: EngineModuleOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 461);
    const P = palette(options.season ?? 'spring');
    const outer = new Shape();
    const inside = new Shape();
    const lit = new Shape();
    const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);
    const radius = (u: number) => ENGINE_RADIUS * (1 + 0.025 * Math.sin(Math.PI * u));
    const at = (u: number, t: number) => {
      const a = 2 * Math.PI * t;
      return v(u * LENGTH, Math.sin(a) * radius(u), Math.cos(a) * radius(u));
    };
    const seed = options.seed ?? 461;
    addSkin(outer, inside, {
      rows: ROWS,
      columns: COLUMNS,
      at,
      middle: (u) => v(u * LENGTH, 0, 0),
      state: (i, j): CellState => {
        const a = (2 * Math.PI * (j + 0.5)) / COLUMNS;
        if (Math.sin(a) < -0.8) return 'panel'; // in the ground
        return random() < 0.05 ? 'hole' : random() < 0.05 ? 'bent' : i === 1 && j === 3 ? 'hole' : 'panel';
      },
      paint: (i, j, normal) => {
        if (i === 0 || i === ROWS - 1) return vary(tone('metal'), random, 1.2);
        if (normal.y > 0.75 && random() < 0.05) return vary(color(P.moss), random, 1.4);
        if (noise2(i * 0.6 + seed * 0.31, j * 0.45, 91) > 0.68) return tone('orange');
        if (random() < 0.06) return tone('metal');
        return shade(tone('plate'), random() < 0.3 ? 0.9 : 1);
      },
      panel: (i) => (i === 0 || i === ROWS - 1 ? { thick: 0.09, gap: 0.025, rust: 1.2 } : {}),
      feature: (i) => (i === 1 && random() < 0.12 ? 'vent' : null),
      wall: WALL,
      random,
      lining: tone('insideDark'),
    });

    // Turned round x (the lathe turns round y).
    const along = (x: number) => new THREE.Matrix4().makeRotationZ(-Math.PI / 2).setPosition(x, 0, 0);
    const R = ENGINE_RADIUS;
    // The intake ring over the casing's back end and into the duct, in
    // segments of dark metal and pale plate.
    addLathe(
      outer,
      [
        [R + 0.02, -0.12],
        [R + 0.14, 0],
        [R + 0.16, 0.22],
        [R + 0.06, 0.38],
        [R - 0.2, 0.45],
        [R - 0.42, 0.34],
        [R - WALL, 0.12],
        [R - WALL, -0.2],
      ],
      COLUMNS,
      along(LENGTH),
      (r, i) => (r === 2 || r === 3 ? (i % 3 === 0 ? tone('orange') : vary(tone('plate'), random, 0.8)) : vary(tone('metal'), random, 0.8)),
    );
    // The front ring, where it was joined to the hull, and a bulkhead
    // closing it.
    addLathe(
      outer,
      [
        [R - WALL, -0.02],
        [R - 0.1, -0.2],
        [R + 0.05, -0.18],
        [R + 0.07, 0.12],
      ],
      COLUMNS,
      along(0),
      (r) => vary(tone(r === 1 ? 'metalDark' : 'metal'), random, 0.8),
    );
    addDisc(outer, v(0.05, 0, 0), v(-1, 0, 0), R - WALL + 0.02, COLUMNS, tone('metalDark'));

    // The fan: its hub, and its blades, each twisted from its root to its
    // tip.
    addLathe(
      inside,
      [
        [0.58, -0.45],
        [0.6, -0.1],
        [0.5, 0.08],
        [0.28, 0.2],
        [0, 0.26],
      ],
      12,
      along(FAN),
      (r, i) => (r === 1 ? tone('orange') : vary(tone(r >= 3 ? 'metalLight' : 'metal'), random, i % 2 ? 0.6 : 1.2)),
    );
    const hub = 0.55;
    const tipR = R - WALL - 0.04;
    for (let k = 0; k < BLADES; k++) {
      const phi = (2 * Math.PI * k) / BLADES;
      const point = (r: number, side: -1 | 1) => {
        const pitch = 0.55 + 0.35 * ((r - hub) / (tipR - hub));
        const chord = 0.36 - 0.1 * ((r - hub) / (tipR - hub));
        const tangent = v(0, -Math.sin(phi), Math.cos(phi));
        const dir = v(Math.cos(pitch), 0, 0).addScaledVector(tangent, Math.sin(pitch));
        return v(FAN, Math.sin(phi) * r, Math.cos(phi) * r).addScaledVector(dir, (side * chord) / 2);
      };
      const bent = random() < 0.12 ? between(random, 0.12, 0.25) : 0; // a blade or two bent back
      const tipA = point(tipR, -1).add(v(bent, 0, 0));
      const tipB = point(tipR, 1).add(v(bent, 0, 0));
      const dark = vary(tone('metal'), random, 1);
      const light = vary(tone('metalLight'), random, 1);
      both(inside, point(hub, -1), point(hub, 1), tipB, dark, dark, light, 0.25);
      both(inside, point(hub, -1), tipB, tipA, dark, light, light, 0.25);
    }
    // The stator behind it: a dark wall of vanes round the core.
    addLathe(
      inside,
      [
        [R - WALL + 0.02, 0],
        [0.95, 0.05],
        [0.7, 0.02],
      ],
      COLUMNS,
      along(CORE),
      () => vary(tone('metalDark'), random, 0.8),
    );
    for (let k = 0; k < 9; k++) {
      const phi = (2 * Math.PI * (k + 0.5)) / 9;
      const dir = v(0, Math.sin(phi), Math.cos(phi));
      addStrut(inside, v(CORE + 0.12, 0, 0).addScaledVector(dir, 0.72), v(CORE + 0.12, 0, 0).addScaledVector(dir, R - WALL - 0.02), v(1, 0, 0), 0.22, 0.06, vary(tone('metal'), random, 1), random);
    }
    // The core: a glowing disc in a dark ring.
    addDisc(lit, v(CORE + 0.04, 0, 0), v(1, 0, 0), 0.7, 14, (i) => (i % 2 ? tone('glow') : color(WRECK.glow).lerp(color(WRECK.glowCore), 0.5)));
    addDisc(lit, v(CORE + 0.06, 0, 0), v(1, 0, 0), 0.34, 10, tone('glowCore'));

    // Cooling pipes along its upper sides, on brackets, with orange joints.
    for (const a of [0.62, Math.PI - 0.62]) {
      const out = R + 0.2;
      const p = (x: number) => v(x, Math.sin(a) * out, Math.cos(a) * out);
      addTube(outer, [p(0.2), p(LENGTH - 0.3)], [0.085, 0.085], { sides: 7, random, paint: (_, i) => shade(tone('metalLight'), 0.85 + (i % 2) * 0.15), caps: [tone('metal'), tone('metal')] });
      for (const x of [0.6, LENGTH / 2, LENGTH - 0.7]) {
        addTube(outer, [p(x - 0.08), p(x + 0.08)], [0.12, 0.12], { sides: 7, random, paint: () => tone('orange'), caps: [tone('orange'), tone('orange')] });
        const foot = v(x, Math.sin(a) * (R + 0.02), Math.cos(a) * (R + 0.02));
        addStrut(outer, foot, p(x), v(1, 0, 0), 0.1, 0.1, tone('metalDark'), random);
      }
    }
    // A mount bracket at its foot, half in the ground.
    for (const x of [0.8, LENGTH - 0.9]) addBlock(outer, v(x, -R + 0.05, 0), v(0.5, 0.5, 1.4), facePaint(tone('metalDark'), random), random, { chamfer: 0.04 });

    // Lit by the core.
    const insideGeometry = inside.geometry();
    bakeLamps(insideGeometry, [{ at: v(CORE + 0.5, 0, 0), light: color(WRECK.glow).multiplyScalar(1.4), reach: 1.3 }], color(0x4a4250).multiplyScalar(0.55));
    this.casing = mesh(outer.geometry(), matte());
    this.duct = mesh(insideGeometry, glowing());
    this.core = mesh(lit.geometry(), glowing(), false);
    this.glow = halo(WRECK.glow, 2.6, 0.8);
    this.glow.position.set(CORE + 0.35, 0, 0);
    this.add(this.casing, this.duct, this.core, this.glow);
  }

  // The core glows brighter and dimmer, slowly.
  update(delta: number): void {
    this.time += delta;
    this.glow.material.opacity = 0.55 + 0.25 * Math.sin(this.time * 1.2) + 0.08 * Math.sin(this.time * 4.7);
  }
}
