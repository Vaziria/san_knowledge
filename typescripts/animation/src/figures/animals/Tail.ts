import * as THREE from 'three';
import { capsule, mesh, solid, type AnimalMaterials } from './parts';

// A tail, from where it leaves the rump (its origin) backward (-z): thin and
// tapering for a cat, a thick brush for a fox or wolf, a stub for a bear or a
// deer. It sets off `droop` radians below level (a negative droop carries it
// up) and curves `curl` radians further up by its tip. Its tip has a colour
// of its own (a fox's white tip). The animal swings it about its origin.

export interface TailShape {
  length: number; // m
  radius: number; // m at its root
  bush: number; // 0 tapering evenly, 1 a brush fuller in its middle than at its root
  droop: number; // radians below level it sets off
  curl: number; // radians it curves up by its tip
}

const ROWS = 14;
const TIP = 0.78; // how far along the tip colour begins

export class Tail extends THREE.Group {
  constructor(m: AnimalMaterials, shape: TailShape, coat: THREE.Color, tip: THREE.Color) {
    super();
    this.name = 'tail';
    const points = [new THREE.Vector3(0, 0, shape.radius * 0.8)]; // starting inside the rump
    const p = new THREE.Vector3();
    const steps = 6;
    for (let i = 0; i <= steps; i++) {
      const s = i / steps;
      const angle = -shape.droop + shape.curl * s * s; // above level
      p.add(new THREE.Vector3(0, Math.sin(angle), -Math.cos(angle)).multiplyScalar(shape.length / steps));
      points.push(p.clone());
    }
    const round = capsule(3);
    const c = new THREE.Color();
    this.add(
      mesh(
        solid(
          points,
          (u) => {
            const even = THREE.MathUtils.lerp(1, 0.35, u);
            const brush = 0.8 + 0.9 * Math.sin(Math.PI * Math.min(1, u * 1.1));
            const r = shape.radius * THREE.MathUtils.lerp(even, brush, shape.bush) * round(u);
            return { width: r, top: r, bottom: r };
          },
          (u) => c.copy(coat).lerp(tip, THREE.MathUtils.smoothstep(u, TIP, TIP + 0.08)),
          ROWS,
          14,
        ),
        m.coat,
      ),
    );
  }
}
