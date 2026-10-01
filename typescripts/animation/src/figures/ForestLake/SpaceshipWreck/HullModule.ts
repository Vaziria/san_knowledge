import * as THREE from 'three';
import { addBlock, addTube, color, glowing, halo, matte, mesh, noise2, palette, seededRandom, shade, Shape, vary, type Season } from '../parts';
import { addCargo, addDisc, addScreen, addSkin, addStrut, addWall, addWoodCrate, bakeLamps, facePaint, facingFrame, section, tone, WRECK, type CellState, type Lamp } from './parts';

export type HullKind = 'front' | 'middle' | 'rear';

export interface HullModuleOptions {
  kind: HullKind;
  length?: number; // m along it; 4.6 by default
  season?: Season; // spring by default
  seed?: number;
}

// The hull's section: a squarish round, as the hull module's sheet draws
// it (Hull Module, "Front View": 4 m wide and 4 m high), sunk a little into
// the ground by the wreck.
export const HULL_HIGH = 1.85; // m, from its axis to its top or bottom
export const HULL_WIDE = 1.95; // m, from its axis to either side
// Where the ground is, from its axis: the wreck sinks each module 0.3 m.
export const HULL_GROUND = -(HULL_HIGH - 0.3);
const ROUNDNESS = 2.5;
const COLUMNS = 16;
const WALL = 0.16;
const FLOOR = -1.2; // m, the floor inside, from the axis
const OPEN_FROM = -0.52; // radians round from +z: the middle's side torn open from here...
const OPEN_TO = 1.95; // ...over the top to here

// One of the spaceship wreck's three hull modules (SpaceshipWreck.md; the
// sheet "Hull Module"): a tube of plates on a dark frame, a ring frame of
// dark plates at each end, square portholes and vents here and there, a
// few plates gone or torn up. Its origin is the middle of its front end
// (-x), on its axis; it runs along +x.
//
// - front: its front end, where the cockpit broke off, torn ragged, plates
//   bent out round it; looking in, a dark hold, its ribs and a bulkhead.
// - middle: its side and top torn wide open (+z), as the sheet's cutaway:
//   the ceiling's frames still across the gap, a floor of plates with a
//   grate down it, bulkheads with doorways at both ends, pipes and a cable
//   tray on the far wall, a console, lamps glowing warm, cargo boxes and
//   the explorer's wooden crates; its ramp let down onto the ground from
//   the gap's foot.
// - rear: whole but for a few holes; the wing's root and the tail sit on it.
export class HullModule extends THREE.Group {
  readonly shell: THREE.Mesh; // its plates, frame and all outside
  readonly inside: THREE.Mesh; // its walls, floor and what is in it, lit by its lamps
  readonly lamps: THREE.Mesh | null; // the lamps and screens, glowing
  readonly halos: THREE.Sprite[] = [];
  readonly length: number;
  readonly kind: HullKind;
  private readonly ramp: { from: THREE.Vector3; to: THREE.Vector3; width: number } | null = null;
  private time = 0;

  constructor(options: HullModuleOptions) {
    super();
    const kind = (this.kind = options.kind);
    const random = seededRandom((options.seed ?? 401) + (kind === 'front' ? 1013 : kind === 'middle' ? 2027 : 3041));
    const L = (this.length = options.length ?? 4.6);
    const P = palette(options.season ?? 'spring');
    const rows = Math.max(3, Math.round(L / 0.92));
    const outer = new Shape();
    const inside = new Shape();
    const lit = new Shape();
    const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

    const at = (u: number, t: number) => {
      const [y, z] = section(2 * Math.PI * t, HULL_HIGH, HULL_WIDE, ROUNDNESS);
      return v(u * L, y, z);
    };
    const middle = (u: number) => v(u * L, 0, 0);
    const angle = (j: number) => ((2 * Math.PI * (j + 0.5)) / COLUMNS + Math.PI) % (2 * Math.PI) - Math.PI; // the column's middle, -π..π from +z
    const opening = (j: number) => angle(j) > OPEN_FROM && angle(j) < OPEN_TO;
    const seed = options.seed ?? 401;

    // Which plates are there.
    const states: CellState[][] = [];
    for (let i = 0; i < rows; i++) {
      states.push([]);
      for (let j = 0; j < COLUMNS; j++) {
        const a = angle(j);
        let s: CellState = random() < 0.05 ? 'hole' : random() < 0.06 ? 'bent' : 'panel';
        if (a < -1.3 && a > -1.85) s = 'panel'; // the bottom stays whole, in the ground
        if (kind === 'middle' && i > 0 && i < rows - 1) {
          if (opening(j)) s = 'open';
          else if (opening(j - 1) || opening(j + 1)) s = random() < 0.45 ? 'bent' : random() < 0.3 ? 'hole' : 'panel';
        }
        if (kind === 'middle' && (i === 0 || i === rows - 1) && opening(j) && random() < 0.35) s = 'hole';
        if (kind === 'front' && i === 0 && random() < 0.35) s = random() < 0.6 ? 'bent' : 'hole';
        states[i].push(s);
      }
    }
    // Plates by colour: the end rings dark; orange in patches; dark ones
    // scattered; moss on some of those facing up.
    const paint = (i: number, j: number, normal: THREE.Vector3) => {
      const band = (i === 0 && kind !== 'front') || i === rows - 1;
      if (band) return vary(tone('metal'), random, 1.2);
      if (normal.y > 0.7 && random() < 0.06) return vary(color(P.moss), random, 1.4);
      const patch = noise2(i * 0.55 + seed * 0.37, j * 0.5, 71);
      if (patch > 0.7) return tone(random() < 0.8 ? 'orange' : 'orangeDeep');
      if (random() < 0.07) return tone('metal');
      return shade(tone('plate'), random() < 0.3 ? 0.9 : 1);
    };
    const featureAt = (i: number, j: number): 'vent' | 'window' | null => {
      if (i === 0 || i === rows - 1) return null;
      const a = angle(j);
      if (kind !== 'middle' && Math.abs(a) < 0.3 && i % 2 === 1) return 'window';
      if (Math.abs(Math.abs(a) - Math.PI) < 0.3 && i === 1) return 'window';
      return random() < 0.07 ? 'vent' : null;
    };
    addSkin(outer, inside, {
      rows,
      columns: COLUMNS,
      at,
      middle,
      state: (i, j) => states[i][j],
      paint,
      feature: featureAt,
      panel: (i) => (i === 0 || i === rows - 1 ? { thick: 0.09, gap: 0.025, rust: 1.2 } : {}),
      wall: WALL,
      torn: kind === 'front' ? [0.7, 0] : [0, 0],
      random,
      lining: tone('inside'),
    });

    // Ribs round the inside at the plates' seams, and where it is open the
    // ceiling's frames across the gap.
    const rib = (u: number, from: number, to: number, out: Shape) => {
      const steps = Math.max(2, Math.round(((to - from) / (2 * Math.PI)) * 24));
      let last: THREE.Vector3 | null = null;
      for (let k = 0; k <= steps; k++) {
        const a = from + ((to - from) * k) / steps;
        const [y, z] = section(a, HULL_HIGH - WALL - 0.05, HULL_WIDE - WALL - 0.05, ROUNDNESS);
        const p = v(u * L, y, z);
        if (last) addStrut(out, last, p, v(1, 0, 0), 0.1, 0.12, vary(tone(kind === 'middle' ? 'inside' : 'insideDark'), random, 1), random);
        last = p;
      }
    };
    for (let i = 1; i < rows; i++) {
      if (kind === 'middle') {
        rib(i / rows, OPEN_TO, 2 * Math.PI + OPEN_FROM, inside);
        if (i % 2 === 0 || random() < 0.4) rib(i / rows, OPEN_FROM + 0.05, OPEN_TO - 0.05, outer);
      } else rib(i / rows, 0, 2 * Math.PI, inside);
    }

    // The bulkheads at its ends: the middle's with doorways, the others'
    // closed (a door shut in the front's); the ends it meets another
    // module by are dark rings.
    const outline = Array.from({ length: 40 }, (_, k) => {
      const [y, z] = section((2 * Math.PI * k) / 40, HULL_HIGH - WALL, HULL_WIDE - WALL, ROUNDNESS);
      return new THREE.Vector2(z, y);
    });
    const door = [new THREE.Vector2(-0.5, FLOOR), new THREE.Vector2(0.5, FLOOR), new THREE.Vector2(0.5, FLOOR + 1.7), new THREE.Vector2(0.3, FLOOR + 1.9), new THREE.Vector2(-0.3, FLOOR + 1.9), new THREE.Vector2(-0.5, FLOOR + 1.7)];
    const bulkhead = (x: number, facing: 1 | -1, doorway: boolean) => {
      // Its frame's x is -z facing +x (and z facing -x), so the outline's
      // (z, y) is flipped to match.
      const frame = facingFrame(v(x, 0, 0), v(facing, 0, 0));
      const flip = (p: THREE.Vector2) => new THREE.Vector2(facing > 0 ? -p.x : p.x, p.y);
      const ring = outline.map(flip);
      addWall(inside, ring, doorway ? [door.map(flip)] : [], frame, tone(kind === 'middle' ? 'inside' : 'insideDark'), tone('metalDark'));
      // The doorway's frame, or the shut door.
      const edge = door.map((p) => new THREE.Vector3(p.x, p.y, 0.05).applyMatrix4(frame));
      for (let k = 0; k < edge.length; k++) {
        if (k === 0) continue; // not across the floor
        addStrut(inside, edge[k - 1], edge[k], v(facing, 0, 0), 0.12, 0.1, vary(tone('metal'), random, 1), random);
      }
      addStrut(inside, edge[edge.length - 1], edge[0], v(facing, 0, 0), 0.12, 0.1, vary(tone('metal'), random, 1), random);
      if (!doorway) {
        const shut = door.map((p) => new THREE.Vector2(p.x * 0.92, FLOOR + (p.y - FLOOR) * 0.97));
        addWall(inside, shut.map(flip), [], facingFrame(v(x + facing * 0.03, 0, 0), v(facing, 0, 0)), tone('metal'), null);
      }
    };
    if (kind === 'middle') {
      bulkhead(0.06, 1, true);
      bulkhead(L - 0.06, -1, true);
    } else if (kind === 'front') {
      bulkhead(L - 0.06, -1, false);
    } else {
      bulkhead(0.06, 1, false);
      bulkhead(L - 0.06, -1, false);
    }

    // Inside the hold: the floor, and in the middle all that is in it.
    const lamps: Lamp[] = [];
    if (kind === 'middle') {
      const halfFloor = floorHalfWidth() - 0.02;
      const floorColor = tone('inside');
      for (let k = 0; k < rows; k++) {
        const x0 = (k * L) / rows + 0.02;
        const x1 = ((k + 1) * L) / rows - 0.02;
        for (const [z0, z1, grate] of [
          [-halfFloor, -0.45, false],
          [-0.43, 0.43, true],
          [0.45, halfFloor, false],
        ] as const) {
          const c = grate ? shade(floorColor, 0.55) : vary(floorColor, random, 1);
          addBlock(inside, v((x0 + x1) / 2, FLOOR - 0.04, (z0 + z1) / 2), v(x1 - x0, 0.08, z1 - z0), facePaint(c, random), random, { chamfer: 0.012, rough: 0.002 });
          if (grate) {
            for (let g = x0 + 0.08; g < x1 - 0.04; g += 0.16) addBlock(inside, v(g, FLOOR + 0.01, 0), v(0.04, 0.03, 0.8), facePaint(shade(floorColor, 1.05), random), random, { chamfer: 0.005, rough: 0 });
          }
        }
      }
      // The far wall: lamps high up, pipes low down, a cable tray under
      // the ceiling, a console.
      const wallAt = (a: number, x: number, inward = 0.05) => {
        const [y, z] = section(a, HULL_HIGH - WALL - inward, HULL_WIDE - WALL - inward, ROUNDNESS);
        return v(x, y, z);
      };
      const lampColor = tone('lamp');
      for (const x of [0.8, L / 2, L - 0.8]) {
        const p = wallAt(2.35, x, 0.08);
        const n = v(0, -p.y, -p.z).normalize();
        addBlock(inside, p, v(0.62, 0.12, 0.14), facePaint(tone('metalDark'), random), random, { chamfer: 0.02, rough: 0 });
        const face = p.clone().addScaledVector(n, 0.075);
        const halfX = 0.27;
        const across = new THREE.Vector3().crossVectors(n, v(1, 0, 0)).normalize().multiplyScalar(0.045);
        lit.quad(face.clone().add(v(-halfX, 0, 0)).sub(across), face.clone().add(v(halfX, 0, 0)).sub(across), face.clone().add(v(halfX, 0, 0)).add(across), face.clone().add(v(-halfX, 0, 0)).add(across), tone('lampCore'), tone('lampCore'), lampColor, lampColor);
        lamps.push({ at: face.clone().addScaledVector(n, 0.25), light: color(WRECK.lamp).multiplyScalar(1.5), reach: 1.9 });
        const glow = halo(WRECK.lamp, 1.1, 0.7);
        glow.position.copy(face.clone().addScaledVector(n, 0.05));
        this.halos.push(glow);
      }
      for (const [a, r] of [
        [3.55, 0.07],
        [3.75, 0.09],
      ] as const) {
        const from = wallAt(a, 0.1, 0.14);
        const to = wallAt(a, L - 0.1, 0.14);
        addTube(inside, [from, to], [r, r], { sides: 6, random, paint: (_, i) => shade(tone('metalLight'), 0.85 + (i % 2) * 0.15) });
        for (let x = 0.5; x < L - 0.2; x += 1.1) {
          const p = wallAt(a, x, 0.14);
          addBlock(inside, p, v(0.08, r * 3, r * 3), facePaint(tone('orange'), random), random, { chamfer: 0.01, rough: 0 });
        }
      }
      const tray = [wallAt(2.05, 0.1, 0.18), wallAt(2.05, L - 0.1, 0.18)];
      addStrut(inside, tray[0], tray[1], v(0, 1, 0), 0.08, 0.3, tone('metalDark'), random);
      for (let k = 0; k < 3; k++) {
        const drop = 0.06 + k * 0.035;
        addTube(inside, [tray[0].clone().add(v(0, -drop, 0.05 * (k - 1))), tray[0].clone().add(v(L * 0.5, -drop - 0.12, 0.05 * (k - 1))), tray[1].clone().add(v(0, -drop, 0.05 * (k - 1)))], [0.025, 0.025, 0.025], {
          sides: 4,
          random,
          paint: () => (k === 1 ? tone('sleeve') : tone('cable')),
        });
      }
      // The console against the far wall, its screens toward the gap.
      const consoleAt = v(L * 0.3, FLOOR, -halfFloor + 0.35);
      addBlock(inside, consoleAt.clone().add(v(0, 0.45, 0)), v(1.2, 0.9, 0.5), facePaint(tone('metal'), random), random, { chamfer: 0.03, rough: 0.003 });
      addBlock(inside, consoleAt.clone().add(v(0, 0.93, 0.08)), v(1.2, 0.08, 0.42), facePaint(tone('metalDark'), random), random, { tip: [0.35, 0], chamfer: 0.015 });
      const screenFacing = v(0, 0.35, 1);
      addScreen(inside, lit, consoleAt.clone().add(v(-0.28, 1.25, -0.12)), screenFacing, 0.42, 0.3, random);
      addScreen(inside, lit, consoleAt.clone().add(v(0.28, 1.25, -0.12)), screenFacing, 0.42, 0.3, random);
      lamps.push({ at: consoleAt.clone().add(v(0, 1.3, 0.4)), light: color(WRECK.screen).multiplyScalar(0.55), reach: 1.2 });
      // Cargo: the ship's boxes and the explorer's crates.
      addCargo(inside, v(L * 0.66, FLOOR, -halfFloor + 0.5), v(0.9, 0.7, 0.75), 0.08, random);
      addCargo(inside, v(L * 0.66 + 0.05, FLOOR + 0.7, -halfFloor + 0.5), v(0.7, 0.5, 0.6), -0.12, random);
      addCargo(inside, v(L * 0.86, FLOOR, -halfFloor + 0.55), v(0.6, 0.55, 0.7), 0.3, random);
      addWoodCrate(inside, v(L * 0.14, FLOOR, -halfFloor + 0.45), 0.6, 0.2, random);
      addWoodCrate(inside, v(L * 0.14 + 0.62, FLOOR, -halfFloor + 0.4), 0.5, -0.15, random);
      addWoodCrate(inside, v(L * 0.14 + 0.1, FLOOR + 0.6, -halfFloor + 0.42), 0.45, 0.5, random);

      // The ramp, let down from the gap's foot to the ground: from the
      // lowest seam the gap is open to.
      let foot = Math.PI;
      for (let j = 0; j < COLUMNS; j++) if (opening(j)) foot = Math.min(foot, angle(j) - Math.PI / COLUMNS);
      const [footY, footZ] = section(foot, HULL_HIGH, HULL_WIDE, ROUNDNESS);
      const width = 2.3;
      const x = L * 0.5;
      const top = v(x, footY + 0.04, footZ + 0.05);
      const reach = 2.5;
      const bottom = v(x, HULL_GROUND + 0.06, footZ + Math.sqrt(reach * reach - (top.y - HULL_GROUND - 0.06) ** 2));
      this.ramp = { from: top, to: bottom, width };
      const along = bottom.clone().sub(top);
      const up = new THREE.Vector3().crossVectors(v(1, 0, 0), along).normalize();
      if (up.y < 0) up.negate();
      const grate = tone('metal');
      const rails = tone('orange');
      addStrut(outer, top.clone().addScaledVector(up, -0.05), bottom.clone().addScaledVector(up, -0.05), up, 0.1, width, shade(grate, 0.8), random);
      for (let k = 0; k < 8; k++) {
        const f0 = (k + 0.1) / 8;
        const f1 = (k + 0.85) / 8;
        const a = top.clone().lerp(bottom, f0);
        const b = top.clone().lerp(bottom, f1);
        addBlock(outer, a.clone().lerp(b, 0.5).addScaledVector(up, 0.03), v(width - 0.24, 0.05, a.distanceTo(b)), facePaint(vary(grate, random, 1), random), random, {
          tip: [Math.atan2(-along.y, along.z), 0],
          chamfer: 0.01,
          rough: 0.002,
        });
      }
      for (const s of [-1, 1]) {
        addStrut(outer, top.clone().add(v(s * (width / 2 - 0.06), 0, 0)).addScaledVector(up, 0.06), bottom.clone().add(v(s * (width / 2 - 0.06), 0, 0)).addScaledVector(up, 0.06), up, 0.12, 0.12, rails, random);
      }
      addStrut(outer, top.clone().add(v(-width / 2, 0, 0)), top.clone().add(v(width / 2, 0, 0)), up, 0.14, 0.14, tone('metalDark'), random);
    } else {
      // A dark floor, and a lamp gone dim.
      const halfFloor = floorHalfWidth() - 0.02;
      addBlock(inside, v(L / 2, FLOOR - 0.04, 0), v(L - 0.1, 0.08, 2 * halfFloor), facePaint(tone('insideDark'), random), random, { chamfer: 0.01, rough: 0.002 });
    }

    // The ends it meets its neighbours by: a dark ring over the seam.
    const endRing = (x: number) => {
      const steps = 32;
      let last: THREE.Vector3 | null = null;
      for (let k = 0; k <= steps; k++) {
        const [y, z] = section((2 * Math.PI * k) / steps, HULL_HIGH + 0.06, HULL_WIDE + 0.06, ROUNDNESS);
        const p = v(x, y, z);
        if (last) addStrut(outer, last, p, v(0, 0, 0).sub(p).setX(0), 0.1, 0.22, vary(tone('metalDark'), random, 0.8), random);
        last = p;
      }
    };
    if (kind !== 'front') endRing(0.08);
    endRing(L - 0.08);
    if (kind === 'rear') {
      // Closed off behind, where the engine was joined on.
      addDisc(outer, v(L + 0.02, 0, 0), v(1, 0, 0), HULL_HIGH - 0.1, 16, tone('metalDark'));
    }

    // The lamps baked into the inside: warm where they are, dark in the
    // holds without them.
    const insideGeometry = inside.geometry();
    bakeLamps(insideGeometry, lamps, kind === 'middle' ? color(0x8a6f63).multiplyScalar(0.62) : color(0x7d7682).multiplyScalar(0.55));
    this.shell = mesh(outer.geometry(), matte());
    this.inside = mesh(insideGeometry, glowing());
    this.add(this.shell, this.inside);
    if (lit.triangles > 0) {
      this.lamps = mesh(lit.geometry(), glowing(), false);
      this.add(this.lamps);
    } else this.lamps = null;
    for (const h of this.halos) this.add(h);
  }

  // The floor to stand on at a point (its own x and z): the hold's floor
  // and the ramp, NaN off them.
  floorAt(x: number, z: number): number {
    const half = floorHalfWidth() - 0.05;
    if (x > 0.1 && x < this.length - 0.1 && Math.abs(z) < half) return FLOOR + 0.03;
    const ramp = this.ramp;
    if (!ramp || Math.abs(x - ramp.from.x) > ramp.width / 2) return NaN;
    const f = (z - ramp.from.z) / (ramp.to.z - ramp.from.z);
    if (f < -0.05 || f > 1) return NaN;
    return ramp.from.y + (ramp.to.y - ramp.from.y) * Math.max(0, f) + 0.08;
  }

  // One of the lamps flickers, its wiring half torn out.
  update(delta: number): void {
    this.time += delta;
    const t = this.time;
    const flicker = Math.sin(t * 23) > 0.7 || Math.sin(t * 3.1) > 0.93 ? 0.15 : 1;
    this.halos.forEach((h, k) => {
      h.material.opacity = k === 1 ? 0.7 * flicker : 0.7 * (0.94 + 0.06 * Math.sin(t * 7 + k));
    });
  }
}

// How wide the floor is at its height: where it meets the walls.
function floorHalfWidth(): number {
  const inY = HULL_HIGH - WALL;
  const inZ = HULL_WIDE - WALL;
  return inZ * (1 - Math.abs(FLOOR / inY) ** ROUNDNESS) ** (1 / ROUNDNESS);
}
