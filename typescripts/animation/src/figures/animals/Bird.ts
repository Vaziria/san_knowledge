import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Animal, type AnimalShape, type Motion } from './Animal';
import { Leg, type LegShape } from './Leg';
import { blob, createMaterials, mesh, mix, palette, type AnimalOptions, type Shade } from './parts';
import { Wing } from './Wing';

// A songbird the size of a thrush, about 23 cm from beak to tail and 15 cm
// tall standing: a plump, egg-shaped body carried head-up, a round head with
// a short pointed beak, wings folded along its sides, a tail sloping down
// behind, and thin legs with three toes forward and one back. A brown back,
// wings and head (the fur colour), an orange breast (toward trim) like a
// robin's, a light belly, and a dark beak and legs.
//
// Units are meters, it faces +z, and the origin is on the ground under its
// body. Its parts are the body (with the head, wings and tail) and two legs,
// each a group with its origin at its joint. Animation behaviour follows its
// spec, Bird.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop() (Animal.ts). It walks in quick steps, its head jerking forward with
// each one and holding still between, like a pigeon's; it runs leaning
// forward with its wings a little out; its jump is a hop with the wings
// spread and beating; and it carries a figure up to 6 cm across in its beak.

const BODY = { center: new THREE.Vector3(0, 0.075, 0), radii: new THREE.Vector3(0.036, 0.04, 0.064), tilt: -0.45 }; // tilt: head up
const HEAD = { at: new THREE.Vector3(0, 0.118, 0.05), radius: 0.028 };
const BEAK = { length: 0.022, radius: 0.007 };
const EYE = { radius: 0.0045, x: 0.019, y: 0.006, z: 0.015 }; // from the head's middle
const WING = { at: new THREE.Vector3(0.03, 0.095, 0.025), length: 0.085, width: 0.04 }; // x mirrored for the left
const TAIL = { at: new THREE.Vector3(0, 0.07, -0.05), length: 0.08, width: 0.045, droop: 0.45 };
const LEG: LegShape = { upper: 0.03, lower: 0.032, radius: 0.0055, foot: 'claws', bend: -1 };
const HIP = { x: 0.016, y: 0.058, z: 0.004 }; // under the body; y is the joint's height at rest
const ANKLE = 0.001; // the toes' height above the ground

const WALK = { stride: 0.04, cadence: 2.6, duty: 0.6 };
const RUN = { stride: 0.065, cadence: 4.2, duty: 0.4 };
const JUMP = 0.12; // m, a hop
const HOLD = 0.06; // m, the largest figure it carries without scaling it down
const BUBBLE = new THREE.Vector3(0, 0.17, 0.06);

const LIFT = { walk: 0.02, run: 0.028 }; // m a stepping foot rises
const BOB = 0.003; // m the body bobs with each step
const HEAD_JERK = 0.012; // m the head goes back and forward with each step
const THRUST = 0.25; // share of each step the head spends jerking forward
const RUN_LEAN = 0.35; // radians it leans forward running
const RUN_SINK = 0.006; // m the body sinks running, so a foot reaches the ground at both ends of its stride
const RUN_WINGS = 0.25; // radians the wings go out running
const SPREAD = 1.35; // radians the wings open in a hop
const BEATS = 3; // wing beats in a hop
const BEAT = 0.5; // radians a beat swings
const CROUCH = 0.018; // m the body sinks crouching
const TAIL_FLICK = 0.25; // radians the tail flicks up with each step, and up in a hop
const HOLD_LIFT = 0.2; // radians the head lifts to carry what it holds

export class Bird extends Animal {
  readonly body = new THREE.Group();
  readonly head = new THREE.Group();
  readonly tail = new THREE.Group();
  readonly leftWing: Wing;
  readonly rightWing: Wing;
  readonly leftLeg: Leg;
  readonly rightLeg: Leg;

  private readonly inverse = new THREE.Matrix4();
  private readonly target = new THREE.Vector3();

  constructor(options: AnimalOptions = {}) {
    const theme = options.theme ?? defaultTheme;
    const shape: AnimalShape = { walk: WALK, run: RUN, jump: JUMP, hold: HOLD, bubble: BUBBLE };
    super(theme, shape);
    this.name = 'bird';
    const m = createMaterials(theme);
    const p = palette(theme);
    const back = p.fur;
    const breast = mix(p.fur, p.trim, 0.75);
    const c = new THREE.Color();
    // Brown above, the orange breast in front and below, the light belly
    // further under and behind.
    const plumage: Shade = (_u, out) => {
      c.copy(back).lerp(breast, THREE.MathUtils.smoothstep(-out.y + 0.6 * out.z, 0.1, 0.5));
      return c.lerp(p.light, THREE.MathUtils.smoothstep(-out.y - 0.5 * out.z, 0.45, 0.8));
    };

    // The body, tipped head-up, with the head, beak and eyes.
    const body = mesh(blob(new THREE.Vector3(), BODY.radii, plumage, { bottom: 0.1 }, new THREE.Euler(BODY.tilt, 0, 0)), m.coat);
    body.position.copy(BODY.center);
    this.body.add(body);
    this.head.position.copy(HEAD.at);
    this.head.add(mesh(blob(new THREE.Vector3(), new THREE.Vector3(HEAD.radius, HEAD.radius * 0.95, HEAD.radius * 1.05), plumage), m.coat));
    const beak = mesh(new THREE.ConeGeometry(BEAK.radius, BEAK.length, 10), m.dark);
    beak.rotation.x = Math.PI / 2;
    beak.position.set(0, -0.003, HEAD.radius * 0.9 + BEAK.length / 2);
    this.head.add(beak);
    for (const side of [-1, 1]) {
      const eye = mesh(new THREE.SphereGeometry(EYE.radius, 12, 8), m.eye);
      eye.position.set(side * EYE.x, EYE.y, EYE.z);
      const shine = mesh(new THREE.SphereGeometry(EYE.radius * 0.35, 6, 4), m.shine);
      shine.position.set(side * (EYE.x + 0.0025), EYE.y + 0.0018, EYE.z + 0.0025);
      this.head.add(eye, shine);
    }
    this.holder.position.set(0, -0.004, HEAD.radius + BEAK.length * 0.7); // in the beak
    this.head.add(this.holder);

    // Wings and tail, a shade darker than the back.
    const feathers: Shade = () => back.clone().multiplyScalar(0.82);
    this.leftWing = new Wing(m, WING.length, WING.width, feathers, -1);
    this.leftWing.position.set(-WING.at.x, WING.at.y, WING.at.z);
    this.rightWing = new Wing(m, WING.length, WING.width, feathers, 1);
    this.rightWing.position.copy(WING.at);
    this.tail.position.copy(TAIL.at);
    const fan = mesh(blob(new THREE.Vector3(0, 0, -TAIL.length / 2), new THREE.Vector3(TAIL.width / 2, 0.005, TAIL.length / 2), feathers, { top: 0 }), m.coat);
    fan.rotation.x = TAIL.droop;
    this.tail.add(fan);
    this.body.add(this.head, this.leftWing, this.rightWing, this.tail);

    // Legs, dark, from under the body.
    const legs: Shade = () => p.dark;
    this.leftLeg = new Leg(m, LEG, legs);
    this.leftLeg.position.set(-HIP.x, HIP.y, HIP.z);
    this.rightLeg = new Leg(m, LEG, legs);
    this.rightLeg.position.set(HIP.x, HIP.y, HIP.z);
    this.body.add(this.leftLeg, this.rightLeg);

    this.rig.add(this.body);
    this.update(0);
  }

  protected pose(mo: Motion): void {
    const walking = mo.moving * (1 - mo.running);
    const turn = 2 * Math.PI * mo.phase;
    const inAir = mo.tuck > 0 ? 1 : 0;

    this.rig.position.y = mo.height;
    this.body.position.y = BOB * mo.moving * Math.abs(Math.sin(turn)) - CROUCH * mo.crouch - RUN_SINK * mo.running * mo.moving;
    this.body.rotation.x = RUN_LEAN * mo.running * mo.moving;
    this.body.updateMatrix();
    this.inverse.copy(this.body.matrix).invert();

    // Legs: the right foot down for its share of the stride, then the left,
    // each stride centred under its hip where the lean has put it.
    const lift = THREE.MathUtils.lerp(LIFT.walk, LIFT.run, mo.running) * mo.moving;
    for (const [leg, side, lag] of [
      [this.rightLeg, 1, 0],
      [this.leftLeg, -1, 0.5],
    ] as const) {
      const p = (mo.phase + lag) % 1;
      let z = mo.stride / 2 - mo.stride * (p / mo.duty);
      let y = 0;
      if (p >= mo.duty) {
        const s = (p - mo.duty) / (1 - mo.duty);
        z = -mo.stride / 2 + mo.stride * THREE.MathUtils.smoothstep(s, 0, 1);
        y = lift * Math.sin(Math.PI * s);
      }
      y += 0.35 * (LEG.upper + LEG.lower) * mo.tuck;
      const hip = leg.position.clone().applyMatrix4(this.body.matrix);
      this.target.set(side * HIP.x, ANKLE + y, hip.z + z).applyMatrix4(this.inverse).sub(leg.position);
      leg.reach(this.target);
    }

    // The head jerks forward at the start of each step and holds still (in
    // the world) for the rest, sliding back over the body as it walks on.
    const step = (2 * mo.phase) % 1;
    const jerk = step < THRUST ? -1 + (2 * step) / THRUST : 1 - (2 * (step - THRUST)) / (1 - THRUST);
    this.head.position.z = HEAD.at.z + HEAD_JERK * walking * jerk;
    this.head.rotation.x = -RUN_LEAN * mo.running * mo.moving - HOLD_LIFT * mo.holding;

    // Wings: out a little running, spread and beating in a hop. Spreading
    // swings a wing's tip out sideways (the right one toward +x); a beat then
    // swings the spread wing up and down.
    const open = RUN_WINGS * mo.running * mo.moving + SPREAD * mo.tuck;
    const beat = inAir * BEAT * Math.sin(2 * Math.PI * BEATS * ((mo.air + 1) / 2));
    this.leftWing.rotation.set(0, open, -beat);
    this.rightWing.rotation.set(0, -open, beat);
    this.tail.rotation.x = -TAIL_FLICK * (walking * Math.max(0, Math.sin(2 * turn)) + mo.tuck);
  }
}
