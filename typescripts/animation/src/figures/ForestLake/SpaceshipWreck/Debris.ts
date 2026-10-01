import * as THREE from 'three';
import { Bush } from '../Bush';
import { DeadTree } from '../DeadTree';
import { addBlock, addFallenLeaves, addFlower, addMoss, addRock, addTube, addTuft, between, color, matte, mesh, palette, seededRandom, shade, Shape, snowOn, vary, type Season } from '../parts';
import { HULL_HIGH, HULL_WIDE } from './HullModule';
import { addCable, addCargo, addDisc, addDrum, addPanel, addSeat, addSkin, addStrut, addWoodCrate, facePaint, section, tone } from './parts';

export interface DebrisOptions {
  season?: Season; // spring by default
  seed?: number;
}

// Where the ramp comes down, kept clear (x from, x to, z from, z to).
const RAMP: readonly [number, number, number, number] = [-0.6, 2.4, 1.6, 4.6];

// What lies round the spaceship wreck (SpaceshipWreck.md; the sheet
// "Debris & Small Parts" and the ground round the wreck on the others), in
// the wreck's own frame, on the ground at y 0: a torn piece of the hull
// lying like a trough by the cockpit, plates torn off it lying about or
// leaning on it, the ship's cargo boxes and the explorer's wooden crates by
// the ramp, red fuel drums, cables and pipes, small parts, a pilot's seat
// thrown out, an antenna mast stood up by the nose; rocks the crash pushed
// up; and what has grown back round it all: tall grass along the hull's
// foot, moss, flowers, a few bushes and two dead trees, fallen leaves in
// autumn, snow on it all in winter.
export class Debris extends THREE.Group {
  readonly scrap: THREE.Mesh; // the pieces, the rocks and what grows among them
  readonly trees: THREE.Object3D[] = []; // the dead trees and the bushes

  constructor(options: DebrisOptions = {}) {
    super();
    const season = options.season ?? 'spring';
    const random = seededRandom(options.seed ?? 551);
    const P = palette(season);
    const shape = new Shape();
    const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

    // The torn piece of the hull: a curved shell of plates lying on the
    // ground, its inside down.
    {
      const L = 2.1;
      const m = new THREE.Matrix4().makeRotationFromEuler(new THREE.Euler(0.1, 0.7, 0.05, 'YXZ')).setPosition(-9.2, -HULL_HIGH * 0.62, 4.9);
      addSkin(shape, shape, {
        rows: 2,
        columns: 6,
        from: 0.09,
        to: 0.41,
        at: (u, t) => {
          const [y, z] = section(2 * Math.PI * t, HULL_HIGH, HULL_WIDE, 2.5);
          return v(u * L, y, z).applyMatrix4(m);
        },
        middle: (u) => v(u * L, 0, 0).applyMatrix4(m),
        state: () => (random() < 0.15 ? 'bent' : 'panel'),
        paint: (_, j) => (j === 0 ? tone('metal') : random() < 0.25 ? tone('orange') : shade(tone('plate'), random() < 0.3 ? 0.9 : 1)),
        torn: [0.6, 0.6],
        random,
        lining: tone('insideDark'),
      });
    }

    // Plates torn off, lying about (some leaning on the hull).
    const plate = (x: number, z: number, w: number, d: number, turn: number, tipX: number, tipZ: number, c: THREE.Color, lift = 0.03) => {
      const m = new THREE.Matrix4().makeRotationFromEuler(new THREE.Euler(tipX, turn, tipZ, 'YXZ')).setPosition(x, lift, z);
      const q = [v(-w / 2, 0, d / 2), v(w / 2, 0, d / 2), v(w / 2, 0, -d / 2), v(-w / 2, 0, -d / 2)].map((p) => p.applyMatrix4(m));
      addPanel(shape, q, c, random, { thick: 0.05, gap: 0, under: true, rust: 3 });
    };
    const plates: [number, number, number, number, number, number, number, 'plate' | 'orange' | 'metal'][] = [
      [-9.8, -3.4, 1.3, 0.9, 0.4, 0.1, 0.05, 'plate'],
      [-4.4, -3.2, 1.1, 0.8, -0.3, -0.08, 0.12, 'orange'],
      [-0.4, -3.6, 1.5, 1.0, 1.2, 0.05, -0.1, 'plate'],
      [3.2, -2.7, 0.9, 0.7, 0.2, 0.15, 0.1, 'metal'],
      [-3.4, 3.4, 1.2, 0.9, -0.7, -0.1, 0.08, 'plate'],
      [11.8, 3.6, 1.4, 1.0, 0.9, 0.12, -0.06, 'plate'],
      [13.4, -2.2, 1.0, 0.8, -0.4, 0.08, 0.1, 'orange'],
      [-12.8, 3.4, 0.9, 0.7, 0.3, -0.12, 0.04, 'metal'],
      [6.2, -10.2, 1.2, 0.9, 1.4, 0.06, 0.1, 'plate'],
      [-6.6, 6.4, 1.1, 0.8, 0.5, 0.09, -0.08, 'plate'],
      [1.2, 8.8, 1.3, 0.9, -0.9, -0.05, 0.12, 'orange'],
    ];
    for (const [x, z, w, d, turn, tx, tz, c] of plates) plate(x, z, w, d, turn, tx, tz, tone(c));
    // Leaning on the hull.
    plate(-5.1, -2.35, 1.2, 0.9, Math.PI / 2, 0, -1.05, tone('plate'), 0.45);
    plate(7.1, 2.35, 1.0, 0.8, Math.PI / 2, 0, 1.0, tone('orange'), 0.4);

    // The ship's cargo boxes and the explorer's wooden crates.
    for (const [x, z, sx, sy, sz, turn] of [
      [2.9, 5.2, 0.9, 0.7, 0.75, 0.3],
      [3.2, 5.25, 0.65, 0.5, 0.55, -0.2],
      [-3.1, -3.4, 0.8, 0.6, 0.7, 0.7],
      [-9.4, -3.9, 0.7, 0.55, 0.6, -0.4],
      [12.4, -3.6, 0.9, 0.65, 0.7, 0.2],
    ]) {
      const stacked = x === 3.2;
      addCargo(shape, v(x, stacked ? 0.7 : 0, z), v(sx, sy, sz), turn, random, stacked ? [0.04, -0.06] : [0, 0]);
    }
    addWoodCrate(shape, v(-1.7, 0, 4.9), 0.62, 0.25, random);
    addWoodCrate(shape, v(-2.4, 0, 5.5), 0.55, -0.3, random);
    addWoodCrate(shape, v(-1.75, 0.62, 4.95), 0.48, 0.6, random);

    // Red fuel drums, two standing, one lying.
    addDrum(shape, new THREE.Matrix4().setPosition(-5.6, 0, 3.1), 0.9, 0.29, random);
    addDrum(shape, new THREE.Matrix4().setPosition(-5.0, 0, 3.75), 0.9, 0.29, random);
    addDrum(shape, new THREE.Matrix4().makeRotationFromEuler(new THREE.Euler(Math.PI / 2, 0.8, 0, 'YXZ')).setPosition(1.8, 0.27, 6.5), 0.9, 0.29, random);

    // Cables from the torn hull down onto the ground, and one in a coil.
    addCable(shape, [v(0.2, 0.75, 1.9), v(-0.1, 0.25, 2.8), v(-0.6, 0.03, 4.2), v(-0.2, 0.03, 5.6)], 0.045, random);
    addCable(shape, [v(-6.3, 1.1, 0.8), v(-6.8, 0.4, 1.8), v(-7.6, 0.03, 2.6), v(-8.4, 0.03, 2.4)], 0.05, random);
    addCable(shape, [v(11.6, 0.6, 1.4), v(12.3, 0.1, 2.2), v(13.2, 0.03, 2.0), v(13.8, 0.03, 1.2)], 0.05, random);
    addCable(shape, [v(-2.5, 1.0, -2.0), v(-2.7, 0.3, -2.6), v(-3.4, 0.03, -2.4)], 0.04, random, 1);
    {
      const coil: THREE.Vector3[] = [];
      for (let k = 0; k <= 26; k++) {
        const a = k * 0.62;
        const r = 0.34 + 0.04 * Math.sin(k * 1.7);
        coil.push(v(-5.4 + Math.cos(a) * r, 0.04 + k * 0.004, 5.6 + Math.sin(a) * r));
      }
      addTube(shape, coil, coil.map(() => 0.04), { sides: 5, random, paint: (_, i) => (i % 9 === 0 ? tone('sleeve') : tone('cable')) });
    }

    // Pipes lying on the ground, with orange joints.
    for (const [x, z, turn, length] of [
      [-1.4, -3.9, 0.3, 2.0],
      [8.4, 3.9, -0.6, 1.6],
    ]) {
      const dir = v(Math.cos(turn), 0, Math.sin(turn));
      const a = v(x, 0.1, z).addScaledVector(dir, -length / 2);
      const b = v(x, 0.1, z).addScaledVector(dir, length / 2);
      addTube(shape, [a, b], [0.1, 0.1], { sides: 7, random, paint: (_, i) => shade(tone('metalLight'), 0.85 + (i % 2) * 0.15), caps: [tone('metalDark'), tone('metalDark')] });
      addTube(shape, [a.clone().addScaledVector(dir, 0.05), a.clone().addScaledVector(dir, 0.22)], [0.13, 0.13], { sides: 7, random, paint: () => tone('orange'), caps: [tone('orange'), tone('orange')] });
    }

    // Small parts: boxes, a vent, brackets.
    for (const [x, z, s, kind] of [
      [-7.9, -3.0, 0.4, 0],
      [-2.2, 3.1, 0.3, 1],
      [5.4, -3.3, 0.45, 2],
      [9.8, -2.9, 0.35, 0],
      [-11.2, 2.4, 0.3, 1],
      [0.4, -2.8, 0.28, 2],
      [13.1, 0.4, 0.4, 1],
      [-13.9, -1.6, 0.32, 0],
    ]) {
      const c = kind === 1 ? tone('orange') : kind === 2 ? tone('metalDark') : tone('metal');
      const turn = random() * Math.PI;
      addBlock(shape, v(x, s * 0.35, z), v(s, s * 0.7, s * 0.8), facePaint(c, random), random, { turn, tip: [between(random, -0.2, 0.2), between(random, -0.2, 0.2)], chamfer: 0.03, rough: 0.006 });
      if (kind === 2) addDisc(shape, v(x, s * 0.72, z), v(0, 1, 0), s * 0.25, 8, tone('gap'));
    }

    // A pilot's seat thrown clear, on its side.
    addSeat(shape, v(-10.6, 0.28, 3.9), 0.9, random, [0, 1.35]);

    // An antenna mast stood up by the nose, on a foot of blocks.
    {
      const foot = v(-14.4, 0, 2.5);
      addBlock(shape, foot.clone().add(v(0, 0.12, 0)), v(0.6, 0.24, 0.6), facePaint(tone('metalDark'), random), random, { turn: 0.4, chamfer: 0.03 });
      const top = foot.clone().add(v(0.1, 2.8, 0.05));
      addTube(shape, [foot.clone().add(v(0, 0.2, 0)), top], [0.05, 0.03], { sides: 6, random, paint: () => vary(tone('metal'), random, 0.6) });
      for (const f of [0.55, 0.78]) {
        const p = foot.clone().lerp(top, f);
        addTube(shape, [p.clone().add(v(-0.35, 0, 0)), p.clone().add(v(0.35, 0, 0))], [0.02, 0.02], { sides: 4, random, paint: () => tone('metal') });
      }
      const dish = top.clone().add(v(0, -0.35, 0.12));
      addDisc(shape, dish, v(0.3, 0.2, 1), 0.28, 10, (i) => shade(tone('plate'), i % 2 ? 1 : 0.9));
      addDisc(shape, dish.clone().add(v(0, 0, -0.01)), v(-0.3, -0.2, -1), 0.28, 10, tone('metal'));
      addStrut(shape, top.clone().add(v(0, -0.4, 0)), dish, v(0, 1, 0), 0.04, 0.04, tone('metal'), random);
      addBlock(shape, top, v(0.1, 0.1, 0.1), facePaint(tone('orange'), random), random, { chamfer: 0.02 });
    }

    // Rocks the crash pushed up, most at its nose.
    for (const [x, z, s] of [
      [-13.0, 0.9, 1.1],
      [-13.4, -1.3, 0.9],
      [-12.6, 2.2, 0.6],
      [-12.2, -2.5, 0.7],
      [-6.8, -5.4, 0.8],
      [0.8, -6.1, 1.0],
      [9.8, 6.4, 0.7],
      [14.2, -4.6, 1.2],
      [-11.4, 6.4, 0.9],
      [4.6, 7.6, 0.6],
      [-2.4, 7.2, 0.7],
      [11.4, -8.4, 0.9],
    ]) {
      addRock(shape, v(x, -0.06, z), v(s * between(random, 0.9, 1.3), s * between(random, 0.6, 0.9), s * between(random, 0.8, 1.1)), random() * Math.PI, random, 0.75, 14, P);
    }

    // What grows back: tall grass along the hull's foot and round what
    // lies about, moss, and in spring and summer flowers among it.
    const tuft = (x: number, z: number, h: number) => addTuft(shape, v(x, 0, z), 8, h, random, 0.02, 0.35, P);
    for (let x = -11.5; x < 11.8; x += between(random, 0.5, 0.9)) {
      for (const s of [-1, 1]) {
        const z = s * (HULL_WIDE - 0.05 + between(random, 0, 0.35));
        if (s > 0 && x > RAMP[0] && x < RAMP[1]) continue;
        tuft(x, z, between(random, 0.25, 0.55));
      }
    }
    for (let k = 0; k < 70; k++) {
      const a = random() * Math.PI * 2;
      const r = between(random, 2.4, 13);
      const x = Math.cos(a) * r * 1.1;
      const z = Math.sin(a) * r * 0.72;
      if (Math.abs(z) < HULL_WIDE + 0.2 && Math.abs(x) < 12) continue;
      if (x > RAMP[0] && x < RAMP[1] && z > RAMP[2] && z < RAMP[3]) continue;
      tuft(x, z, between(random, 0.2, 0.45));
    }
    // Moss on the ground past the churned earth.
    for (let k = 0; k < 10; k++) {
      const a = random() * Math.PI * 2;
      addMoss(shape, v(Math.cos(a) * between(random, 11.5, 14), 0.01, Math.sin(a) * between(random, 6.5, 9)), between(random, 0.3, 0.55), random, P);
    }
    if (season === 'spring' || season === 'summer') {
      const petals = [P.petalPink, P.petalWhite, P.petalYellow, P.petalLilac];
      for (let k = 0; k < 18; k++) {
        const x = between(random, -13, 13);
        const z = (random() < 0.5 ? -1 : 1) * between(random, 2.6, 8);
        const foot = v(x, -0.01, z);
        const head = foot.clone().add(v(0, between(random, 0.16, 0.3), 0));
        addTube(shape, [foot, head], [0.006, 0.005], { sides: 3, random, paint: () => color(P.grass) });
        addFlower(shape, head, v(0, 1, 0.2), 6, 0.05, color(petals[k % petals.length]), color(P.flowerHeart), random);
      }
    }
    if (season === 'autumn') {
      for (const [x, z, r] of [
        [-8, 4.4, 1.6],
        [-2, -4.2, 1.8],
        [5, 5.8, 1.5],
        [10.5, -4.4, 1.6],
        [-12, -3, 1.2],
      ]) addFallenLeaves(shape, v(x, 0, z), r, 22, random);
    }

    this.scrap = mesh(shape.geometry(), matte());
    this.add(this.scrap);
    if (season === 'winter') snowOn(this.scrap, random, 0.35, 0.72);

    // Two dead trees and a few bushes, the forest lake's own.
    for (const [x, z, height, seed, turn] of [
      [-13.6, -4.4, 4.2, 3, 0.5],
      [14.6, 5.8, 3.6, 4, 2.1],
    ]) {
      const tree = new DeadTree({ seed, height, season });
      tree.position.set(x, -0.05, z);
      tree.rotation.y = turn;
      this.trees.push(tree);
    }
    for (const [x, z, size, seed] of [
      [-6.4, -4.8, 1.3, 5],
      [3.2, -9.6, 1.1, 6],
      [13.6, -3.9, 1.2, 7],
      [-10.2, 7.2, 1.0, 8],
    ]) {
      const bush = new Bush({ seed, size, season });
      bush.position.set(x, 0, z);
      bush.rotation.y = random() * Math.PI * 2;
      this.trees.push(bush);
    }
    this.add(...this.trees);
  }
}
