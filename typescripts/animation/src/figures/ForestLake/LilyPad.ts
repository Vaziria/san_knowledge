import * as THREE from 'three';
import { between, both, color, matte, mesh, palette, seededRandom, Shape, shade, vary, type Season } from './parts';

export interface LilyPadOptions {
  pads?: number; // 4 by default
  flower?: boolean; // a pink water lily among them, as the asset sheet has; true by default
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's lily pads (LilyPad.md), as its asset sheet draws them:
// round green pads lying flat on the water, each with a notch cut to its
// middle and darker veins running out from it, and a pink water lily: two
// rings of pointed petals, pale at their feet and deep pink at their tips,
// round a yellow middle. They lie at y 0, the water's surface. As the
// reference's seasons show them: the lily white in summer, the pads
// yellowing in autumn, and frosted in winter, when the lake is ice.
export class LilyPad extends THREE.Group {
  readonly pads: THREE.Mesh;
  readonly lily: THREE.Mesh;

  constructor(options: LilyPadOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 81);
    const count = options.pads ?? 4;
    const P = palette(options.season);

    const green = new Shape();
    const light = color(P.lily);
    const dark = color(P.lilyDark);
    const up = new THREE.Vector3(0, 1, 0);
    const placed: { at: THREE.Vector3; radius: number }[] = [];
    for (let k = 0; k < count; k++) {
      const radius = k === 0 ? 0.34 : between(random, 0.17, 0.3);
      // Spread out round the first, touching at most.
      let at = new THREE.Vector3();
      for (let tries = 0; tries < 30 && k > 0; tries++) {
        const angle = random() * Math.PI * 2;
        const d = between(random, 0.35, 0.75);
        at = new THREE.Vector3(Math.cos(angle) * d, 0, Math.sin(angle) * d);
        if (placed.every((p) => p.at.distanceTo(at) > p.radius + radius - 0.02)) break;
      }
      placed.push({ at, radius });
      const notch = random() * Math.PI * 2;
      const gap = between(random, 0.35, 0.5); // radians of the notch
      const middle = at.clone().add(new THREE.Vector3(0, 0.016, 0));
      const segments = 14;
      const rim: THREE.Vector3[] = [];
      for (let i = 0; i <= segments; i++) {
        // Counter-clockwise seen from above (decreasing angle), round from
        // one side of the notch to the other.
        const angle = notch - gap / 2 - ((2 * Math.PI - gap) * i) / segments;
        const r = radius * between(random, 0.96, 1.02);
        rim.push(new THREE.Vector3(at.x + Math.cos(angle) * r, 0.01, at.z + Math.sin(angle) * r));
      }
      const pad = vary(light, random, 1);
      const vein = shade(pad, 0.78);
      const edge = pad.clone().lerp(dark, 0.35);
      for (let i = 0; i < segments; i++) {
        // Every other wedge darker toward the middle (the veins), all a
        // little darker at the rim. The notch is left open.
        wedge(green, middle, rim[i], rim[i + 1], i % 2 ? vein : pad, edge, edge);
      }
    }

    // The water lily, on the first pad.
    const lily = new Shape();
    if (options.flower ?? true) {
      const base = placed[0].at.clone().add(new THREE.Vector3(placed[0].radius * 0.15, 0.03, 0));
      const pale = color(P.lotus);
      const deep = color(P.lotusTip);
      const heart = color(P.flowerHeart);
      const ring = (count: number, length: number, rise: number, turn: number) => {
        for (let i = 0; i < count; i++) {
          const angle = turn + (2 * Math.PI * i) / count;
          const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
          const across = new THREE.Vector3(-out.z, 0, out.x);
          const foot = base.clone().addScaledVector(out, 0.012);
          const tip = base.clone().addScaledVector(out, length).addScaledVector(up, length * rise);
          const mid = base.clone().addScaledVector(out, length * 0.55).addScaledVector(up, length * rise * 0.45);
          const w = length * 0.34;
          const left = mid.clone().addScaledVector(across, w);
          const right = mid.clone().addScaledVector(across, -w);
          const tipColor = vary(deep, random, 0.6);
          both(lily, foot, left, tip, pale, shade(pale, 0.97), tipColor, 0.5);
          both(lily, foot, tip, right, pale, tipColor, shade(pale, 0.97), 0.5);
        }
      };
      ring(8, 0.2, 0.45, 0);
      ring(8, 0.155, 1.05, Math.PI / 8);
      ring(6, 0.1, 1.6, 0.2);
      // The yellow middle.
      const top = base.clone().add(new THREE.Vector3(0, 0.045, 0));
      const around = Array.from({ length: 6 }, (_, i) => {
        const angle = -(2 * Math.PI * i) / 6;
        return base.clone().add(new THREE.Vector3(Math.cos(angle) * 0.04, 0.018, Math.sin(angle) * 0.04));
      });
      for (let i = 0; i < 6; i++) lily.triangle(top, around[i], around[(i + 1) % 6], shade(heart, 1.1), heart, heart);
    }

    this.pads = mesh(green.geometry(), matte());
    this.lily = mesh(lily.geometry(), matte());
    this.add(this.pads, this.lily);
  }
}

// A pad's wedge, facing up: its corners counter-clockwise seen from above.
function wedge(target: Shape, middle: THREE.Vector3, a: THREE.Vector3, b: THREE.Vector3, cm: THREE.Color, ca: THREE.Color, cb: THREE.Color): void {
  target.triangle(middle, a, b, cm, ca, cb);
}
