import * as THREE from 'three';
import { SpeechBubble } from '../../speech/SpeechBubble';
import { defaultTheme, type Theme } from '../../theme';
import { Body } from './Body';
import { Foot } from './Foot';
import { Head } from './Head';
import { createMaterials } from './parts';
import { Tail } from './Tail';
import { Wing } from './Wing';

// A small, round penguin, about 55 cm tall (the size of an Adélie penguin),
// put together from its parts: body, head, two wings, two feet and a tail.
// Every colour comes from the theme. Units are meters, it faces +z, and the
// origin is on the floor under its center.
//
// Animation behaviour follows the spec, Penguin.md: Flap(), Jump(),
// Hold(figure), Walk(), Run() and Speech(text), plus Stop() to end a walk or
// run. Call update(delta) once per frame. Walk() and Run() move the penguin itself: it
// goes forward the way it faces (its +z) at `speed` meters per second, so turn
// it (rotation.y) to steer.

// Where each part's pivot sits on the penguin.
const NECK_Y = 0.36;
const SHOULDER = new THREE.Vector3(0.125, 0.31, 0); // x mirrored for the left wing
const HIP_SPACING = 0.06; // feet, either side of the center
const HEEL_Z = 0.07; // feet, under the front of the body
const TAIL_BASE = new THREE.Vector3(0, 0.07, -0.09);

// Flap: the wings beat out and back FLAP_COUNT times.
const FLAP_DURATION = 0.9; // seconds
const FLAP_COUNT = 3;
const FLAP_ANGLE = 1.1; // radians away from the body at the top of a beat

// Jump: crouch, hop, and land with a squash. The first and last CROUCH part
// of the jump is on the ground, squashing down and back up.
const JUMP_DURATION = 0.75; // seconds
const JUMP_HEIGHT = 0.12; // meters
const CROUCH = 0.18; // fraction of the jump spent crouching, at each end
const SQUASH = 0.12; // how much shorter the penguin gets when squashed
const JUMP_WING_LIFT = 0.5; // wings lift away from the body in the air, radians

// Hold: the figure sits in front of the belly and the wings wrap forward and
// in around it. A figure wider than HOLD_WIDTH is scaled down to fit.
const HOLD_POINT = new THREE.Vector3(0, 0.16, 0.24); // bottom center of the held figure
const HOLD_WIDTH = 0.3; // meters
const HOLD_WING_FORWARD = -0.9; // rotation.x, negative swings the wing tip forward
const HOLD_WING_IN = 0.35; // radians toward the body
const HOLD_SPEED = 8; // how fast the wings move into and out of the hold

// Walk and Run: a waddle. The body leans over the foot it stands on, bobs and
// twists a little, the feet step in turn, and the wings go out for balance.
// Running takes quicker, longer steps, leans forward and holds the wings out
// and back. Each foot is planted for half a cycle and slides back one stride
// under the body in that time, so the speed is 2 * stride * cadence and a
// planted foot stays put on the floor: about 0.19 m/s walking, 0.52 running.
interface Gait {
  stride: number; // meters a planted foot travels back under the body
  cadence: number; // cycles per second; a cycle is two steps, right then left
  lift: number; // meters a stepping foot rises
  roll: number; // radians the body leans over the planted foot
  bob: number; // meters the body rises over the planted foot
  twist: number; // radians the body turns with the stepping foot
  lean: number; // radians the body leans forward
  wingOut: number; // radians the wings go out from the body
  wingBack: number; // radians the wings swing back (rotation.x)
}
const WALK: Gait = {
  stride: 0.06,
  cadence: 1.6,
  lift: 0.025,
  roll: 0.12,
  bob: 0.008,
  twist: 0.08,
  lean: 0.05,
  wingOut: 0.35,
  wingBack: 0,
};
const RUN: Gait = {
  stride: 0.1,
  cadence: 2.6,
  lift: 0.04,
  roll: 0.1,
  bob: 0.015,
  twist: 0.06,
  lean: 0.28,
  wingOut: 0.8,
  wingBack: 0.4,
};
const GAIT_SPEED = 6; // how fast it starts, stops, and changes between walking and running
const WING_BALANCE = 0.15; // the wing away from the planted foot swings further out, radians
const TOE_LIFT = 0.3; // toes tip up while a foot swings forward, radians
const TAIL_WAG = 0.25; // radians, against the body's twist
const HEAD_STEADY = 0.6; // share of the body's lean and roll the head undoes, to keep looking ahead

// Speech: the bubble's tail points down at the top of the head (about 0.565 m
// up) from just above it, and rises with a jump.
const BUBBLE_Y = 0.6;

const forward = new THREE.Vector3(); // reused by update()

export interface PenguinOptions {
  theme?: Theme;
}

export class Penguin extends THREE.Group {
  // Each part is its own group with its origin at its pivot, ready to animate.
  readonly body: Body;
  readonly head: Head;
  readonly leftWing: Wing;
  readonly rightWing: Wing;
  readonly leftFoot: Foot;
  readonly rightFoot: Foot;
  readonly tail: Tail;

  // Everything moves inside the rig, so jumping never changes the position
  // the caller gave the penguin; only Walk() and Run() move it. The torso,
  // everything above the feet, leans, bobs and twists while the feet step.
  // The holder carries the held figure.
  private readonly rig = new THREE.Group();
  private readonly torso = new THREE.Group();
  private readonly holder = new THREE.Group();
  // Outside the rig, so a jump's squash never squashes the words.
  private readonly bubble: SpeechBubble;

  private flapElapsed = FLAP_DURATION; // not flapping
  private jumpElapsed = JUMP_DURATION; // not jumping
  private held: THREE.Object3D | null = null;
  private holdWeight = 0; // 0 = wings at rest, 1 = wings wrapped around the held figure
  private gait: 'stand' | 'walk' | 'run' = 'stand';
  private moving = 0; // 0 standing, 1 walking or running; eased
  private running = 0; // 0 walking, 1 running; eased
  private stepPhase = 0; // 0..1 through a cycle: right foot planted, then left
  private currentSpeed = 0;

  constructor(options: PenguinOptions = {}) {
    super();
    this.name = 'penguin';
    const m = createMaterials(options.theme ?? defaultTheme);

    this.body = new Body(m);

    this.head = new Head(m);
    this.head.position.y = NECK_Y;

    this.leftWing = new Wing(m, -1);
    this.leftWing.position.set(-SHOULDER.x, SHOULDER.y, SHOULDER.z);
    this.rightWing = new Wing(m, 1);
    this.rightWing.position.copy(SHOULDER);

    this.leftFoot = new Foot(m, -1);
    this.leftFoot.position.set(-HIP_SPACING, 0, HEEL_Z);
    this.rightFoot = new Foot(m, 1);
    this.rightFoot.position.set(HIP_SPACING, 0, HEEL_Z);

    this.tail = new Tail(m);
    this.tail.position.copy(TAIL_BASE);

    this.holder.position.copy(HOLD_POINT);

    this.torso.add(this.body, this.head, this.leftWing, this.rightWing, this.tail, this.holder);
    this.rig.add(this.torso, this.leftFoot, this.rightFoot);
    this.bubble = new SpeechBubble(options.theme ?? defaultTheme);
    this.bubble.position.y = BUBBLE_Y;
    this.add(this.rig, this.bubble);
  }

  // Forward speed in meters per second; 0 when standing. Turning the penguin
  // by speed / radius radians a second walks it round a circle of that radius.
  get speed(): number {
    return this.currentSpeed;
  }

  // Beats both wings out and back three times. Calling it again restarts the
  // flap. The wings don't flap while they hold a figure.
  Flap(): void {
    this.flapElapsed = 0;
  }

  // Crouches, hops about 12 cm and lands with a squash, carrying anything it
  // holds. Ignored while a jump is already under way.
  Jump(): void {
    if (this.jumpElapsed < JUMP_DURATION) return;
    this.jumpElapsed = 0;
  }

  // Holds a figure in front of its belly, wings wrapped around it; the figure
  // is moved into the penguin (its position and rotation reset) and scaled
  // down to fit if it is wider than 30 cm. Holding another figure drops the
  // first; Hold(null) drops what it holds. Named as in the spec, Penguin.md.
  Hold(figure: THREE.Object3D | null): void {
    if (this.held) this.holder.remove(this.held);
    this.held = figure;
    this.holder.scale.setScalar(1);
    if (!figure) return;

    figure.removeFromParent();
    figure.position.set(0, 0, 0);
    figure.rotation.set(0, 0, 0);
    figure.updateMatrixWorld(true);
    const width = new THREE.Box3().setFromObject(figure).getSize(new THREE.Vector3()).x;
    this.holder.scale.setScalar(width > HOLD_WIDTH ? HOLD_WIDTH / width : 1);
    this.holder.add(figure);
  }

  // Waddles forward the way it faces, about 19 cm a second, until Run() or
  // Stop(). Named as in the spec, Penguin.md.
  Walk(): void {
    this.gait = 'walk';
  }

  // Runs forward the way it faces, about 52 cm a second, until Walk() or
  // Stop(): quicker, longer steps, leaning forward with the wings out and
  // back. Named as in the spec, Penguin.md.
  Run(): void {
    this.gait = 'run';
  }

  // Slows from a walk or run to standing still. Not in the spec yet.
  Stop(): void {
    this.gait = 'stand';
  }

  // Says the text in a speech bubble above its head. A bubble holds at most
  // nine words, so longer text is split into several, shown one after
  // another, each long enough to read; a word over 50 characters is cut into
  // 50-character pieces first. Saying something new cuts off what it
  // was saying; Speech('') just stops. It can talk while it does anything
  // else. Named as in the spec, Penguin.md.
  Speech(text: string): void {
    this.bubble.say(text);
  }

  // Whether a speech bubble is still showing.
  get speaking(): boolean {
    return this.bubble.speaking;
  }

  update(delta: number): void {
    // Flap: |sin| gives one beat out and back per half turn.
    let flap = 0;
    if (this.flapElapsed < FLAP_DURATION) {
      this.flapElapsed += delta;
      const p = Math.min(1, this.flapElapsed / FLAP_DURATION);
      flap = FLAP_ANGLE * Math.abs(Math.sin(Math.PI * FLAP_COUNT * p));
    }

    // Jump: squash down and up at both ends, a parabola in between.
    let height = 0;
    let squash = 0;
    if (this.jumpElapsed < JUMP_DURATION) {
      this.jumpElapsed += delta;
      const p = Math.min(1, this.jumpElapsed / JUMP_DURATION);
      if (p < CROUCH) {
        squash = Math.sin((Math.PI * p) / CROUCH);
      } else if (p > 1 - CROUCH) {
        squash = Math.sin((Math.PI * (p - (1 - CROUCH))) / CROUCH);
      } else {
        const q = (p - CROUCH) / (1 - 2 * CROUCH);
        height = JUMP_HEIGHT * 4 * q * (1 - q);
      }
    }
    // The rig's origin is on the floor, so squashing keeps the feet down.
    this.rig.position.y = height;
    this.rig.scale.set(1 + (SQUASH / 2) * squash, 1 - SQUASH * squash, 1 + (SQUASH / 2) * squash);
    this.bubble.position.y = BUBBLE_Y + height;
    this.bubble.update(delta);

    // Walk and run: ease into the gait, then step and move forward.
    const ease = 1 - Math.exp(-GAIT_SPEED * delta);
    this.moving += ((this.gait === 'stand' ? 0 : 1) - this.moving) * ease;
    this.running += ((this.gait === 'run' ? 1 : 0) - this.running) * ease;
    const mix = (key: keyof Gait) => THREE.MathUtils.lerp(WALK[key], RUN[key], this.running);

    const cadence = mix('cadence');
    const stride = mix('stride') * this.moving;
    this.stepPhase = (this.stepPhase + cadence * delta) % 1;
    this.currentSpeed = 2 * stride * cadence;
    forward.set(0, 0, 1).applyQuaternion(this.quaternion).setY(0);
    if (forward.lengthSq() > 0) this.position.addScaledVector(forward.normalize(), this.currentSpeed * delta);

    // Positive while the right foot is planted, negative while the left is.
    const wave = Math.sin(2 * Math.PI * this.stepPhase);
    const roll = -mix('roll') * this.moving * wave; // negative leans toward +x, the right foot
    const lean = mix('lean') * this.moving;
    this.torso.position.y = mix('bob') * this.moving * Math.abs(wave);
    this.torso.rotation.set(lean, mix('twist') * this.moving * wave, roll);
    this.head.rotation.x = -HEAD_STEADY * lean;
    this.head.rotation.z = -HEAD_STEADY * roll;
    this.tail.rotation.y = -TAIL_WAG * this.moving * wave;

    // Feet: planted for half a cycle, sliding back under the moving body, then
    // lifted and swung forward with the toes up. The left foot is half a cycle
    // behind the right.
    const lift = mix('lift') * this.moving;
    for (const [foot, side, lag] of [[this.rightFoot, 1, 0], [this.leftFoot, -1, 0.5]] as const) {
      const p = (this.stepPhase + lag) % 1;
      let z = stride / 2 - stride * (p / 0.5);
      let y = 0;
      let toe = 0;
      if (p >= 0.5) {
        const s = (p - 0.5) / 0.5;
        z = -stride / 2 + stride * THREE.MathUtils.smoothstep(s, 0, 1);
        y = lift * Math.sin(Math.PI * s);
        toe = -TOE_LIFT * this.moving * Math.sin(Math.PI * s);
      }
      foot.position.set(side * HIP_SPACING, y, HEEL_Z + z);
      foot.rotation.x = toe;
    }

    // Hold: ease the wings into or out of the hold.
    const t = 1 - Math.exp(-HOLD_SPEED * delta);
    this.holdWeight += ((this.held ? 1 : 0) - this.holdWeight) * t;

    // Wings: away from the body is negative rotation.z on the left, positive
    // on the right. Holding wins over flapping, the in-air lift and the gait.
    const out = flap + JUMP_WING_LIFT * (height / JUMP_HEIGHT) + mix('wingOut') * this.moving;
    const back = mix('wingBack') * this.moving;
    for (const [wing, side] of [[this.leftWing, -1], [this.rightWing, 1]] as const) {
      // The wing away from the planted foot swings further out, for balance.
      const balance = WING_BALANCE * this.moving * -side * wave;
      wing.rotation.z = side * ((out + balance) * (1 - this.holdWeight) - HOLD_WING_IN * this.holdWeight);
      wing.rotation.x = HOLD_WING_FORWARD * this.holdWeight + back * (1 - this.holdWeight);
    }
  }
}
