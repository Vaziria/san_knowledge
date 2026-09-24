import * as THREE from 'three';
import { capsule, mesh, solid, twoTone, type AnimalMaterials } from './parts';

// A four-legged animal's body: its torso, from the rump (-z) to the chest
// (+z), and its neck, rising from the top of the chest to where the head
// joins it. The origin is the middle of the torso. The legs, head and tail
// are separate parts that the animal places on it (Quadruped.ts).
//
// The torso is a solid of rounded sections: a little narrower at the hips or
// the chest, deeper at the chest with the belly tucked up toward the hind
// legs, and humped over the shoulders for a bear. The coat colour runs over
// the back and sides and the belly colour underneath.

export interface TorsoShape {
  length: number; // m, rump to chest
  height: number; // m, back to belly at the deepest
  width: number; // m, side to side at the widest
  hips: number; // the hips' width, as a share of the widest
  chest: number; // the chest's width, as a share of the widest
  tuck: number; // the belly's depth at the hips, as a share of its depth at the chest
  hump: number; // how much higher the back rises over the shoulders, as a share of the height
}

export interface NeckShape {
  length: number; // m, from the top of the chest to the head's joint
  rise: number; // radians above level it leaves the chest
  radius: number; // m where it meets the head; it thickens toward the chest
  base: THREE.Vector3; // where it leaves the chest, from the torso's middle
}

const ROWS = 24;
const NECK_ROWS = 10;
const HUMP_AT = 0.72; // how far along the torso the hump is, from the rump
const HUMP_WIDTH = 0.16;

export class Body extends THREE.Group {
  // Where the neck ends and the head joins it, from the torso's middle.
  readonly neckEnd: THREE.Vector3;

  constructor(m: AnimalMaterials, torso: TorsoShape, neck: NeckShape, coat: THREE.Color, belly: THREE.Color) {
    super();
    this.name = 'body';
    const shade = twoTone(coat, belly);
    const round = capsule(2.6);
    const half = torso.length / 2;
    this.add(
      mesh(
        solid(
          [new THREE.Vector3(0, 0, -half), new THREE.Vector3(0, 0, 0), new THREE.Vector3(0, 0, half)],
          (u) => {
            const e = round(u);
            const hump = 1 + torso.hump * Math.exp(-(((u - HUMP_AT) / HUMP_WIDTH) ** 2));
            return {
              width: (torso.width / 2) * e * THREE.MathUtils.lerp(torso.hips, torso.chest, u),
              top: (torso.height / 2) * e * hump,
              bottom: (torso.height / 2) * e * THREE.MathUtils.lerp(torso.tuck, 1, THREE.MathUtils.smoothstep(u, 0.15, 0.7)),
            };
          },
          shade,
          ROWS,
          20,
        ),
        m.coat,
      ),
    );

    // The neck, thicker where it meets the chest, sunk into it.
    const way = new THREE.Vector3(0, Math.sin(neck.rise), Math.cos(neck.rise));
    this.neckEnd = neck.base.clone().addScaledVector(way, neck.length);
    const start = neck.base.clone().addScaledVector(way, -0.35 * neck.length);
    this.add(
      mesh(
        solid(
          [start, neck.base.clone().addScaledVector(way, 0.4 * neck.length), this.neckEnd.clone().addScaledVector(way, 0.15 * neck.length)],
          (u) => {
            const r = neck.radius * THREE.MathUtils.lerp(1.6, 1, u) * capsule(3)(u);
            return { width: r * 0.9, top: r, bottom: r * 1.05 };
          },
          shade,
          NECK_ROWS,
          16,
        ),
        m.coat,
      ),
    );
  }
}
