import * as THREE from 'three';
import { addBlock, addTube, both, color, glowing, halo, matte, mesh, noise2, palette, seededRandom, shade, Shape, vary, type Season } from '../parts';
import { addScreen, addSeat, addSkin, addStrut, bakeLamps, facePaint, section, tone, WRECK, type CellState, type Lamp } from './parts';

export interface CockpitOptions {
  season?: Season; // spring by default
  seed?: number;
}

const LENGTH = 5.2; // m, from where it broke off to the tip of its nose
const HIGH = 1.5; // m, its section's half height where it broke off
const WIDE = 1.7; // m, its half width there
const ROUNDNESS = 2.4;
const COLUMNS = 14;
const ROWS = 7;
const SKIN = 0.93; // how much of its length is plated; the tip is a cap
const WALL = 0.13;
const FLOOR = -0.9; // m, the floor inside, from its axis where it broke off
const CANOPY: readonly [number, number] = [0.13, 0.55]; // where along it the canopy is torn open...
const CANOPY_ANGLE: readonly [number, number] = [0.72, 2.42]; // ...and how far round (radians from +z)

// The spaceship wreck's cockpit (SpaceshipWreck.md; the sheet "Cockpit
// Module"): a pointed nose, faceted, of plates like the hull's, orange low
// down toward its nose; its canopy torn open over the seats, the frame
// left round the gap and a few shards of glass in it; two antennas on its
// back. Inside, lit by its screens and a lamp: a floor, two pilots' seats
// side by side facing the nose, a console across in front with screens,
// a stick before each seat, a side console and a storage box. Broken off
// the hull at its back, ragged there. Its origin is the middle of that
// broken end; it runs along -x to its nose.
export class Cockpit extends THREE.Group {
  readonly shell: THREE.Mesh;
  readonly inside: THREE.Mesh;
  readonly screens: THREE.Mesh;
  readonly halos: THREE.Sprite[] = [];

  constructor(options: CockpitOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 431);
    const P = palette(options.season ?? 'spring');
    const outer = new Shape();
    const inside = new Shape();
    const lit = new Shape();
    const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

    // Its shape along it: s 0 where it broke off, 1 at its nose's tip.
    const size = (s: number) => ({
      cy: -0.7 * s ** 1.6,
      ry: HIGH * Math.max(0, 1 - s ** 2.2) ** 0.6,
      rz: WIDE * Math.max(0, 1 - s ** 2.5) ** 0.55,
    });
    const at = (u: number, t: number) => {
      const s = u * SKIN;
      const { cy, ry, rz } = size(s);
      const [y, z] = section(2 * Math.PI * t, ry, rz, ROUNDNESS);
      return v(-s * LENGTH, cy + y, z);
    };
    const middle = (u: number) => v(-u * SKIN * LENGTH, size(u * SKIN).cy, 0);
    const angle = (j: number) => ((2 * Math.PI * (j + 0.5)) / COLUMNS + Math.PI) % (2 * Math.PI) - Math.PI;
    const canopy = (i: number, j: number) => {
      const s = ((i + 0.5) / ROWS) * SKIN;
      const a = angle(j);
      return s > CANOPY[0] && s < CANOPY[1] && a > CANOPY_ANGLE[0] && a < CANOPY_ANGLE[1];
    };
    const seed = options.seed ?? 431;
    addSkin(outer, inside, {
      rows: ROWS,
      columns: COLUMNS,
      at,
      middle,
      state: (i, j): CellState => (canopy(i, j) ? 'open' : i === 0 && random() < 0.3 ? (random() < 0.6 ? 'bent' : 'hole') : random() < 0.04 ? 'bent' : 'panel'),
      paint: (i, j, normal) => {
        const a = angle(j);
        const low = a < -0.25 && a > -2.9; // under its middle
        if (low && i >= 3) return tone(random() < 0.85 ? 'orange' : 'orangeDeep');
        if (normal.y > 0.75 && random() < 0.05) return vary(color(P.moss), random, 1.4);
        if (noise2(i * 0.7 + seed * 0.29, j * 0.55, 83) > 0.72) return tone('orange');
        if (random() < 0.08) return tone('metal');
        return shade(tone('plate'), random() < 0.3 ? 0.9 : 1);
      },
      feature: (i, j) => (i === 2 && Math.abs(Math.abs(angle(j)) - 0.1) < 0.25 ? 'vent' : null),
      wall: WALL,
      torn: [0.65, 0],
      random,
      lining: tone('inside'),
    });

    // The nose's tip: a cone of orange and dark facets closing it.
    const ring = Array.from({ length: COLUMNS }, (_, j) => at(1, j / COLUMNS));
    const tip = v(-LENGTH - 0.05, size(1).cy - 0.02, 0);
    for (let j = 0; j < COLUMNS; j++) {
      const a = ring[j];
      const b = ring[(j + 1) % COLUMNS];
      // Seen from outside the ring runs clockwise here (it faces -x), so
      // the triangle goes tip, b, a.
      const n = new THREE.Vector3().crossVectors(b.clone().sub(tip), a.clone().sub(tip));
      const c = j % 3 === 0 ? tone('metalDark') : tone('orange');
      if (n.x < 0) outer.triangle(tip, b, a, vary(c, random, 1));
      else outer.triangle(tip, a, b, vary(c, random, 1));
    }

    // The canopy's frame round the gap, a bar down its middle broken off
    // short, and shards of glass left in its corners.
    const onSkin = (s: number, a: number, out: number) => {
      const { cy, ry, rz } = size(s);
      const [y, z] = section(a, ry + out, rz + out, ROUNDNESS);
      return v(-s * LENGTH, cy + y, z);
    };
    const frameColor = tone('metalDark');
    const rail = (a: number) => {
      for (let k = 0; k < 6; k++) {
        const s0 = CANOPY[0] + ((CANOPY[1] - CANOPY[0]) * k) / 6;
        const s1 = CANOPY[0] + ((CANOPY[1] - CANOPY[0]) * (k + 1)) / 6;
        addStrut(outer, onSkin(s0, a, 0.02), onSkin(s1, a, 0.02), v(0, 1, 0), 0.11, 0.12, vary(frameColor, random, 0.8), random);
      }
    };
    const arch = (s: number) => {
      for (let k = 0; k < 6; k++) {
        const a0 = CANOPY_ANGLE[0] + ((CANOPY_ANGLE[1] - CANOPY_ANGLE[0]) * k) / 6;
        const a1 = CANOPY_ANGLE[0] + ((CANOPY_ANGLE[1] - CANOPY_ANGLE[0]) * (k + 1)) / 6;
        addStrut(outer, onSkin(s, a0, 0.02), onSkin(s, a1, 0.02), v(1, 0, 0), 0.12, 0.11, vary(frameColor, random, 0.8), random);
      }
    };
    rail(CANOPY_ANGLE[0]);
    rail(CANOPY_ANGLE[1]);
    arch(CANOPY[0]);
    arch(CANOPY[1]);
    arch((CANOPY[0] + CANOPY[1]) / 2 + 0.04);
    addStrut(outer, onSkin(CANOPY[1], Math.PI / 2, 0.02), onSkin(CANOPY[1] - 0.12, Math.PI / 2, 0.02), v(0, 1, 0), 0.1, 0.1, frameColor, random);
    const glass = tone('glass');
    for (const [s, a, w, h] of [
      [CANOPY[1] - 0.02, CANOPY_ANGLE[0] + 0.02, -0.1, 0.35],
      [CANOPY[1] - 0.02, CANOPY_ANGLE[1] - 0.02, -0.08, -0.4],
      [CANOPY[0] + 0.02, CANOPY_ANGLE[0] + 0.02, 0.07, 0.3],
    ] as const) {
      const p = onSkin(s, a, 0.01);
      const q = onSkin(s + w, a, 0.01);
      const r = onSkin(s + w * 0.3, a + h, 0.01);
      both(outer, p, q, r, vary(glass, random, 0.6), shade(glass, 1.1), shade(glass, 0.85), 0.2);
    }

    // Two antennas on its back.
    const mast = (s: number, a: number, height: number, lean: number) => {
      const foot = onSkin(s, a, 0);
      const top = foot.clone().add(v(Math.sin(lean) * height, Math.cos(lean) * height, 0));
      addBlock(outer, foot, v(0.2, 0.12, 0.2), facePaint(tone('metalDark'), random), random, { chamfer: 0.02 });
      addTube(outer, [foot, top], [0.03, 0.018], { sides: 5, random, paint: () => tone('metal') });
      addTube(outer, [top.clone().lerp(foot, 0.3).add(v(0, 0, -0.2)), top.clone().lerp(foot, 0.3).add(v(0, 0, 0.2))], [0.015, 0.015], { sides: 4, random, paint: () => tone('metal') });
    };
    mast(0.06, Math.PI / 2 - 0.25, 1.3, 0.35);
    mast(0.1, Math.PI / 2 + 0.3, 0.9, 0.5);

    // Inside: the floor, the seats, the console with its screens, a stick
    // before each seat, a side console and a box.
    const floorAt = (s: number) => {
      const { cy, ry, rz } = size(s);
      const y = FLOOR - cy;
      return (rz - WALL) * Math.max(0, 1 - Math.abs(y / (ry - WALL)) ** ROUNDNESS) ** (1 / ROUNDNESS);
    };
    const floorEnd = 0.6;
    for (let k = 0; k < 5; k++) {
      const s0 = (floorEnd * k) / 5 + 0.01;
      const s1 = (floorEnd * (k + 1)) / 5 - 0.01;
      const half = Math.min(floorAt(s0), floorAt(s1)) - 0.03;
      addBlock(inside, v((-(s0 + s1) / 2) * LENGTH, FLOOR - 0.04, 0), v((s1 - s0) * LENGTH, 0.08, 2 * half), facePaint(vary(tone('inside'), random, 1), random), random, { chamfer: 0.012, rough: 0.002 });
    }
    for (const z of [-0.55, 0.55]) addSeat(inside, v(-1.25, FLOOR, z), 0, random);
    const consoleX = -2.45;
    const half = floorAt(consoleX / -LENGTH) - 0.05;
    addBlock(inside, v(consoleX, FLOOR + 0.42, 0), v(0.55, 0.84, 2 * half), facePaint(tone('metal'), random), random, { chamfer: 0.03, rough: 0.003 });
    addBlock(inside, v(consoleX + 0.18, FLOOR + 0.88, 0), v(0.5, 0.08, 2 * half - 0.1), facePaint(tone('metalDark'), random), random, { tip: [0, -0.45], chamfer: 0.015 });
    const facing = v(1, 0.55, 0);
    for (const z of [-0.62, 0, 0.62]) addScreen(inside, lit, v(consoleX + 0.05, FLOOR + 1.22, z), facing, 0.48, 0.34, random);
    for (const z of [-0.55, 0.55]) {
      addBlock(inside, v(-1.85, FLOOR + 0.15, z), v(0.2, 0.3, 0.2), facePaint(tone('metalDark'), random), random, { chamfer: 0.02 });
      addTube(inside, [v(-1.85, FLOOR + 0.3, z), v(-1.8, FLOOR + 0.72, z)], [0.03, 0.025], { sides: 5, random, paint: () => tone('metal') });
      addBlock(inside, v(-1.8, FLOOR + 0.76, z), v(0.09, 0.12, 0.09), facePaint(tone('orange'), random), random, { chamfer: 0.015 });
    }
    addBlock(inside, v(-0.9, FLOOR + 0.35, floorAt(0.17) - 0.2), v(1.1, 0.7, 0.3), facePaint(tone('metal'), random), random, { chamfer: 0.03, rough: 0.003 });
    addScreen(inside, lit, v(-0.9, FLOOR + 0.66, floorAt(0.17) - 0.34), v(0, 0.8, -0.6), 0.3, 0.2, random);
    addBlock(inside, v(-0.3, FLOOR + 0.24, -floorAt(0.06) + 0.35), v(0.5, 0.48, 0.5), facePaint(tone('orange'), random), random, { turn: 0.2, chamfer: 0.03, rough: 0.003 });
    addBlock(inside, v(-0.3, FLOOR + 0.5, -floorAt(0.06) + 0.35), v(0.52, 0.05, 0.52), facePaint(tone('metalDark'), random), random, { turn: 0.2, chamfer: 0.01 });
    // A lamp strip under its roof, behind the seats.
    const lampAt = v(-0.35, size(0.07).cy + size(0.07).ry - WALL - 0.08, 0);
    lit.quad(lampAt.clone().add(v(0.07, 0, -0.35)), lampAt.clone().add(v(0.07, 0, 0.35)), lampAt.clone().add(v(-0.07, 0, 0.35)), lampAt.clone().add(v(-0.07, 0, -0.35)), tone('lampCore'), tone('lampCore'), tone('lamp'), tone('lamp'));
    const glow = halo(WRECK.lamp, 0.9, 0.6);
    glow.position.copy(lampAt).add(v(0, -0.05, 0));
    this.halos.push(glow);
    const screenGlow = halo(WRECK.screen, 1.3, 0.4);
    screenGlow.position.set(consoleX + 0.2, FLOOR + 1.25, 0);
    this.halos.push(screenGlow);
    const lamps: Lamp[] = [
      { at: lampAt.clone().add(v(0, -0.3, 0)), light: color(WRECK.lamp).multiplyScalar(1.3), reach: 1.5 },
      { at: v(consoleX + 0.6, FLOOR + 1.2, 0), light: color(WRECK.screen).multiplyScalar(0.7), reach: 1.1 },
    ];

    const insideGeometry = inside.geometry();
    bakeLamps(insideGeometry, lamps, color(0x8a6f63).multiplyScalar(0.6));
    this.shell = mesh(outer.geometry(), matte());
    this.inside = mesh(insideGeometry, glowing());
    this.screens = mesh(lit.geometry(), glowing(), false);
    this.add(this.shell, this.inside, this.screens, ...this.halos);
  }
}
