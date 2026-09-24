import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Animal, type AnimalShape, type Motion } from './Animal';
import { blob, createMaterials, mesh, mix, palette, type AnimalOptions } from './parts';

// A snake, 1.4 m long and 5.5 cm thick at its thickest, lying on the ground
// in an S: a slim neck behind a flat, rounded head, a body swelling and then
// tapering to a fine tail. An olive coat (the fur colour toward the grass
// colour) with dark saddles across its back, light beneath.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its length. Its parts are the body (one tube along its spine,
// reshaped every frame) and the head, a group with its origin where the head
// meets the neck. Animation behaviour follows its spec, Snake.md: Jump(),
// Hold(figure), Walk(), Run() and Speech(text), plus Stop() (Animal.ts).
//
// It has no legs, so it walks and runs by slithering (lateral undulation):
// its body lies along a wave fixed on the ground, and as it moves forward
// every part of it follows the same wave, so nothing slides sideways, like
// feet that stay planted. Walking is a slow glide, running a quicker one
// with a tighter wave. Standing still it holds its last curves. Its head is
// held a little off the ground and turns along the curve; it lifts higher
// to carry what it holds. Its jump springs the whole body off the ground,
// bowed up in the middle. It carries a figure up to 12 cm across in its mouth.

const LENGTH = 1.4;
const RADIUS = 0.0275; // m at its thickest
const NECK = 0.06; // share of the length behind the head where it is slimmest
const THICKEST = 0.3; // share of the length where it is thickest
const RINGS = 64; // along the body
const SIDES = 12;
const HEAD = { length: 0.065, width: 0.046, height: 0.028 };
const EYE = 0.0055;

// The wave the body lies along, by gait: its wavelength and height either
// side, both in meters.
const WAVE = { rest: { length: 0.8, height: 0.12 }, walk: { length: 0.8, height: 0.14 }, run: { length: 0.6, height: 0.1 } };
const STILL_HEAD = 0.5; // share of the wave the head swings, so it stays steadier than the body
const STEADIED = 0.15; // share of the length behind the head that swings less; it slides a little sideways as it passes on, the rest none
const HEAD_LIFT = { rest: 0.025, holding: 0.07 }; // m the head is off the ground
const LIFTED = 0.18; // share of the length behind the head that rises with it
const BOW = 0.5; // share of the jump the middle of the body bows up
const SADDLES = 0.11; // m from one dark saddle to the next

const WALK = { stride: 0.3, cadence: 1, duty: 1 }; // m/s: it glides, so the "stride" is its speed
const RUN = { stride: 0.9, cadence: 1, duty: 1 };
const JUMP = 0.2;
const HOLD = 0.12;
const BUBBLE = new THREE.Vector3(0, 0.12, LENGTH / 2);
const BUBBLE_SCALE = 1.2; // times the penguin's: the camera stands back to take in its length, though its head is low

// The body's radius at a share of its length from the head: slim at the
// neck, swelling to its thickest, then tapering to the tail.
function radius(s: number): number {
  const neck = 0.55 + 0.45 * THREE.MathUtils.smoothstep(s, NECK, THICKEST);
  const tail = s < THICKEST ? 1 : Math.pow(Math.max(0, 1 - (s - THICKEST) / (1 - THICKEST)), 0.75);
  return RADIUS * Math.max(0.06, neck * tail);
}

export class Snake extends Animal {
  readonly body: THREE.Mesh;
  readonly head = new THREE.Group();

  private readonly geometry: THREE.BufferGeometry;
  private travel = 0; // m moved along its wave so far
  private wave = { ...WAVE.rest };
  private lift: number = HEAD_LIFT.rest;
  private readonly spine = Array.from({ length: RINGS }, () => new THREE.Vector3());

  constructor(options: AnimalOptions = {}) {
    const theme = options.theme ?? defaultTheme;
    const shape: AnimalShape = { walk: WALK, run: RUN, jump: JUMP, hold: HOLD, bubble: BUBBLE, bubbleScale: BUBBLE_SCALE };
    super(theme, shape);
    this.name = 'snake';
    const m = createMaterials(theme);
    const p = palette(theme);
    const coat = mix(p.fur, p.grass, 0.5);
    const saddle = mix(coat, p.dark, 0.6);
    const belly = mix(coat, p.light, 0.6);

    // The body: RINGS rings of SIDES vertices, reshaped every frame (reshape()),
    // closed at the tail. Its colours stay with the body as it moves: dark
    // saddles across the back, light beneath.
    const positions = new Float32Array((RINGS * SIDES + 1) * 3);
    const colors: number[] = [];
    const c = new THREE.Color();
    for (let i = 0; i < RINGS; i++) {
      const s = i / (RINGS - 1);
      const band = Math.sin((2 * Math.PI * s * LENGTH) / SADDLES) > 0.35 && s > NECK + 0.04;
      for (let j = 0; j < SIDES; j++) {
        const up = Math.sin((j / SIDES) * 2 * Math.PI - Math.PI / 2); // -1 underneath, 1 on top
        c.copy(band && up > -0.1 ? saddle : coat).lerp(belly, THREE.MathUtils.smoothstep(-up, 0.3, 0.7));
        colors.push(c.r, c.g, c.b);
      }
    }
    colors.push(coat.r, coat.g, coat.b); // the tail's tip
    const indices: number[] = [];
    const at = (i: number, j: number) => i * SIDES + (j % SIDES);
    for (let i = 0; i < RINGS - 1; i++) {
      for (let j = 0; j < SIDES; j++) indices.push(at(i, j), at(i + 1, j), at(i, j + 1), at(i, j + 1), at(i + 1, j), at(i + 1, j + 1));
    }
    const tip = RINGS * SIDES;
    for (let j = 0; j < SIDES; j++) indices.push(at(RINGS - 1, j), tip, at(RINGS - 1, j + 1));
    this.geometry = new THREE.BufferGeometry();
    this.geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    this.geometry.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    this.geometry.setIndex(indices);
    this.body = mesh(this.geometry, m.coat);
    this.body.frustumCulled = false; // it changes shape, so its bounds would go stale

    // The head: flat and rounded, a little wider than the neck, with eyes
    // on top of its sides and the mouth at its front.
    const face = (_u: number, out: THREE.Vector3) => (out.y < -0.2 ? belly : coat);
    this.head.add(mesh(blob(new THREE.Vector3(0, 0, HEAD.length * 0.45), new THREE.Vector3(HEAD.width / 2, HEAD.height / 2, HEAD.length / 2), face, { top: 0.15 }, new THREE.Euler(Math.PI / 2, 0, 0)), m.coat));
    for (const side of [-1, 1]) {
      const eye = mesh(new THREE.SphereGeometry(EYE, 10, 8), m.eye);
      eye.position.set(side * HEAD.width * 0.36, HEAD.height * 0.22, HEAD.length * 0.62);
      const shine = mesh(new THREE.SphereGeometry(EYE * 0.35, 6, 4), m.shine);
      shine.position.copy(eye.position).add(new THREE.Vector3(side * 0.001, 0.002, 0.002));
      this.head.add(eye, shine);
    }
    this.holder.position.set(0, -HEAD.height * 0.2, HEAD.length * 0.95);
    this.head.add(this.holder);

    this.rig.add(this.body, this.head);
    this.update(0);
  }

  protected pose(mo: Motion): void {
    const ease = 1 - Math.exp(-3 * mo.delta);
    const target = mo.moving < 0.01 ? WAVE.rest : mo.running > 0.5 ? WAVE.run : WAVE.walk;
    this.wave.length += (target.length - this.wave.length) * ease;
    this.wave.height += (target.height - this.wave.height) * ease;
    this.lift += (THREE.MathUtils.lerp(HEAD_LIFT.rest, HEAD_LIFT.holding, mo.holding) - this.lift) * ease;
    this.travel += (mo.stride * mo.delta) / mo.duty; // its own speed, as the base moves it forward

    // The spine: each point a share s of the length back from the head, on
    // the ground's wave where that part of the body is now. The head's end
    // swings less, and rises off the ground; in a jump the middle bows up.
    this.rig.position.y = mo.height;
    const k = (2 * Math.PI) / this.wave.length;
    for (let i = 0; i < RINGS; i++) {
      const s = i / (RINGS - 1);
      const z = LENGTH / 2 - s * LENGTH;
      const swing = THREE.MathUtils.lerp(STILL_HEAD, 1, THREE.MathUtils.smoothstep(s, 0, STEADIED));
      const x = this.wave.height * swing * Math.sin(k * (this.travel + z));
      const rise = this.lift * (1 - THREE.MathUtils.smoothstep(s, 0, LIFTED)) + BOW * mo.height * Math.sin(Math.PI * s) * (mo.height > 0 ? 1 : 0);
      this.spine[i].set(x, radius(s) + rise, z);
    }
    this.reshape();

    // The head sits on the front of the spine, turned along it.
    const front = this.spine[0];
    const next = this.spine[2];
    this.head.position.copy(front);
    this.head.rotation.set(Math.atan2(next.y - front.y, front.z - next.z), Math.atan2(front.x - next.x, front.z - next.z), 0, 'YXZ');
    this.bubbleShift.set(front.x, front.y - HEAD_LIFT.rest, 0);
  }

  // Puts the body's rings round the spine, and the tail's tip on its end.
  private reshape(): void {
    const position = this.geometry.getAttribute('position') as THREE.BufferAttribute;
    const tangent = new THREE.Vector3();
    const side = new THREE.Vector3();
    const up = new THREE.Vector3();
    const UP = new THREE.Vector3(0, 1, 0);
    for (let i = 0; i < RINGS; i++) {
      const s = i / (RINGS - 1);
      tangent.subVectors(this.spine[Math.max(0, i - 1)], this.spine[Math.min(RINGS - 1, i + 1)]).normalize(); // toward the head
      side.crossVectors(UP, tangent).normalize();
      up.crossVectors(tangent, side);
      const r = radius(s);
      for (let j = 0; j < SIDES; j++) {
        const angle = (j / SIDES) * 2 * Math.PI - Math.PI / 2;
        const p = this.spine[i];
        position.setXYZ(i * SIDES + j, p.x + r * (side.x * Math.cos(angle) + up.x * Math.sin(angle) * 0.85), p.y + r * (side.y * Math.cos(angle) + up.y * Math.sin(angle) * 0.85), p.z + r * (side.z * Math.cos(angle) + up.z * Math.sin(angle) * 0.85));
      }
    }
    const end = this.spine[RINGS - 1];
    const before = this.spine[RINGS - 2];
    position.setXYZ(RINGS * SIDES, end.x + (end.x - before.x) * 0.5, end.y, end.z + (end.z - before.z) * 0.5);
    position.needsUpdate = true;
    this.geometry.computeVertexNormals();
    this.geometry.computeBoundingSphere();
  }
}
