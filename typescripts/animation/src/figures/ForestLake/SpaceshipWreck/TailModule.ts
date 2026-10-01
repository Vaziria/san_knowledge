import * as THREE from 'three';
import { addBlock, addHull, addLathe, addTube, color, glowing, halo, matte, mesh, palette, seededRandom, shade, Shape, vary, type Season } from '../parts';
import { addPanel, addSlab, facePaint, tone, WRECK } from './parts';

export interface TailModuleOptions {
  season?: Season; // spring by default
  seed?: number;
}

const BASE = 3.6; // m, the fairing's length along the hull's top
const HIGH = 0.6; // m, the fairing's height
const WIDE = 0.62; // m, half its width
const FIN_HIGH = 4.7; // m, the fin's tip over the fairing's foot

// The spaceship wreck's tail (SpaceshipWreck.md; the sheet "Tail
// Module"): a fairing of plates along the hull's top, and on it the
// vertical fin, tall and swept back, plated on both faces, orange along its
// leading edge and up its front, pale behind; an antenna mast beside it,
// and at the fin's tip a navigation light, blinking orange. Its origin is
// the middle of the fairing's front foot; it runs along +x.
export class TailModule extends THREE.Group {
  readonly fairing: THREE.Mesh; // the fairing, the fin and the mast
  readonly lens: THREE.Mesh;
  readonly light: THREE.Sprite;
  private time = 0;

  constructor(options: TailModuleOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 491);
    const P = palette(options.season ?? 'spring');
    const shape = new Shape();
    const lit = new Shape();
    const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

    // The fairing: a dark core, its front sloping up to its top, plated.
    const halfAt = (x: number, y: number) => WIDE * (0.88 + 0.12 * (x / BASE)) * (1 - 0.08 * (y / HIGH));
    const core: THREE.Vector3[] = [];
    for (const s of [-1, 1]) {
      core.push(v(0, 0, s * halfAt(0, 0) * 0.9), v(0.1, 0.18, s * halfAt(0, 0.18) * 0.9), v(0.8, HIGH, s * halfAt(0.8, HIGH)), v(BASE, HIGH, s * halfAt(BASE, HIGH)), v(BASE, 0, s * halfAt(BASE, 0)));
    }
    addHull(shape, core, (n) => vary(shade(tone('metalDark'), n.y > 0.5 ? 1.1 : 1), random, 1));
    const plateColor = (k: number) => (k === 0 ? tone('orange') : random() < 0.1 ? tone('metal') : random() < 0.05 ? vary(color(P.moss), random, 1.4) : shade(tone('plate'), random() < 0.3 ? 0.9 : 1));
    const segments = 4;
    for (let k = 0; k < segments; k++) {
      const x0 = 0.8 + ((BASE - 0.8) * k) / segments;
      const x1 = 0.8 + ((BASE - 0.8) * (k + 1)) / segments;
      // Its top, either side of the fin's root, and its sides.
      for (const s of [-1, 1]) {
        const inner = 0.24 * s;
        const top = [v(x0, HIGH, inner), v(x0, HIGH, s * halfAt(x0, HIGH)), v(x1, HIGH, s * halfAt(x1, HIGH)), v(x1, HIGH, inner)];
        addPanel(shape, s > 0 ? top : [top[0], top[3], top[2], top[1]], plateColor(k), random, { thick: 0.04 });
        const side = [v(x0, 0.02, s * halfAt(x0, 0)), v(x1, 0.02, s * halfAt(x1, 0)), v(x1, HIGH - 0.02, s * halfAt(x1, HIGH)), v(x0, HIGH - 0.02, s * halfAt(x0, HIGH))];
        addPanel(shape, s > 0 ? side : [side[0], side[3], side[2], side[1]], plateColor(k), random, { thick: 0.04 });
      }
    }
    // Its sloping front, orange.
    for (const s of [-1, 1]) {
      const front = [v(0.02, 0.05, 0), v(0.02, 0.05, s * halfAt(0, 0) * 0.85), v(0.78, HIGH - 0.02, s * halfAt(0.8, HIGH) * 0.95), v(0.78, HIGH - 0.02, 0)];
      const n = new THREE.Vector3().crossVectors(front[1].clone().sub(front[0]), front[3].clone().sub(front[0]));
      addPanel(shape, n.x < 0 ? front : [front[0], front[3], front[2], front[1]], tone('orange'), random, { thick: 0.04 });
    }

    // The fin: orange along its leading edge and up its front, pale
    // behind, a dark band at its root.
    const rows = 5;
    const columns = 4;
    addSlab(shape, {
      root: [v(0.8, HIGH, 0), v(BASE, HIGH, 0)],
      tip: [v(3.9, FIN_HIGH, 0), v(5.4, FIN_HIGH, 0)],
      normal: v(0, 0, 1),
      thick: [0.42, 0.2],
      rows,
      columns,
      paint: (i, j) => {
        if (i === 0) return vary(tone('metal'), random, 1);
        if (j <= 1 && i >= 1 && (j === 0 || i >= 2)) return tone(random() < 0.85 ? 'orange' : 'orangeDeep');
        if (i === rows - 1) return tone('orange');
        return random() < 0.05 ? tone('metal') : shade(tone('plate'), random() < 0.3 ? 0.9 : 1);
      },
      state: () => (random() < 0.04 ? 'hole' : 'panel'),
      random,
    });

    // The antenna mast beside the fin: a pole leaning back, two cross
    // bars, a ball at its top.
    const foot = v(1.5, HIGH, 0.4);
    const top = foot.clone().add(v(0.35, 2.2, 0));
    addBlock(shape, foot.clone().add(v(0, 0.05, 0)), v(0.24, 0.1, 0.24), facePaint(tone('metalDark'), random), random, { chamfer: 0.02 });
    addTube(shape, [foot, top], [0.04, 0.022], { sides: 6, random, paint: () => vary(tone('metal'), random, 0.6) });
    for (const f of [0.55, 0.8]) {
      const p = foot.clone().lerp(top, f);
      addTube(shape, [p.clone().add(v(0, 0, -0.28 * (1.2 - f))), p.clone().add(v(0, 0, 0.28 * (1.2 - f)))], [0.016, 0.016], { sides: 4, random, paint: () => tone('metal') });
    }
    addLathe(
      shape,
      [
        [0, -0.05],
        [0.05, 0],
        [0, 0.05],
      ],
      6,
      new THREE.Matrix4().setPosition(top),
      () => tone('orange'),
    );

    // The navigation light at the fin's tip: a housing and its lens.
    const lamp = v(5.25, FIN_HIGH + 0.08, 0);
    addBlock(shape, lamp.clone().add(v(0, -0.02, 0)), v(0.3, 0.14, 0.22), facePaint(tone('metalDark'), random), random, { chamfer: 0.02 });
    addLathe(
      lit,
      [
        [0.08, 0.04],
        [0.08, 0.1],
        [0.05, 0.16],
        [0, 0.18],
      ],
      8,
      new THREE.Matrix4().setPosition(lamp),
      (r) => (r >= 2 ? color(0xffd9a0) : tone('navigation')),
    );
    this.fairing = mesh(shape.geometry(), matte());
    this.lens = mesh(lit.geometry(), glowing(), false);
    this.light = halo(WRECK.navigation, 1.6, 1);
    this.light.position.copy(lamp).add(v(0, 0.12, 0));
    this.add(this.fairing, this.lens, this.light);
  }

  // The navigation light blinks: a flash every second and a half.
  update(delta: number): void {
    this.time += delta;
    const phase = this.time % 1.5;
    this.light.material.opacity = phase < 0.18 ? 1 : phase < 0.3 ? 1 - (phase - 0.18) / 0.12 : 0.06;
  }
}
