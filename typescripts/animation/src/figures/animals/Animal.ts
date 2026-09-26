import * as THREE from 'three';
import { SpeechBubble } from '../../speech/SpeechBubble';
import type { Theme } from '../../theme';

// What every animal shares: the behaviours of the animals' spec (Jump(),
// Hold(figure), Walk(), Run() and Speech(text), the same as the penguin's
// without Flap()), plus Stop() to end a walk or run, and the bookkeeping
// behind them. Each animal builds its own body into `rig` and poses it every
// frame in pose(), from the Motion this class works out: where it is in its
// stride, how far it is into a jump, and so on. Call update(delta) once per
// frame.
//
// Walk() and Run() move the animal itself: it goes forward the way it faces
// (its +z) at `speed` meters a second, so turn it (rotation.y) to steer.
// Jumps stay inside the rig and never change its position. A stride is a
// cycle of steps; a foot is down for `duty` of it and slides back one
// `stride` under the body in that time, so the speed is
// stride * cadence / duty and a planted foot stays put on the ground.

export interface Gait {
  stride: number; // m a planted foot travels back under the body
  cadence: number; // strides a second
  duty: number; // share of a stride a foot is on the ground
}

export interface AnimalShape {
  walk: Gait;
  run: Gait;
  jump: number; // m it jumps
  hold: number; // m: a held figure is scaled down to fit this, its largest side
  bubble: THREE.Vector3; // where its speech bubble's tail points, just above its head
  // How big its speech bubble is, times the penguin's: the camera that shows
  // a deer whole stands far back, so its words must be bigger to read, and a
  // bird's camera stands close. Left out, it follows the animal's height.
  bubbleScale?: number;
}

const BUBBLE_SCALE = { per: 0.55, least: 0.5, most: 4 }; // the height (m) that takes a bubble the penguin's size, and the limits

// Where the animal is in its movements this frame, for pose().
export interface Motion {
  delta: number; // s since the last frame
  phase: number; // 0..1 through the stride
  moving: number; // 0 standing, 1 walking or running; eased
  running: number; // 0 walking, 1 running; eased
  stride: number; // m, the stride now (0 standing)
  duty: number;
  height: number; // m the rig is off the ground in a jump
  crouch: number; // 0..1: bending down before a jump and on landing
  air: number; // -1 at take-off, 0 at the top, 1 at landing, 0 on the ground
  tuck: number; // 0..1: legs drawn up in the air
  holding: number; // 0..1, eased toward 1 while it holds something
}

const GRAVITY = 9.81;
const CROUCH_TIME = 0.2; // s spent bending down before a jump, and again landing
const GAIT_SPEED = 4; // how fast it starts, stops, and changes between walking and running
const HOLD_SPEED = 8;

const forward = new THREE.Vector3(); // reused by update()

export abstract class Animal extends THREE.Group {
  // Everything that moves goes in the rig. The holder is where a held figure
  // goes: each animal puts it in its mouth (or beak).
  protected readonly rig = new THREE.Group();
  protected readonly holder = new THREE.Group();
  // How far pose() moves the speech bubble from its resting place this frame,
  // for a head that moves about (a snake's).
  protected readonly bubbleShift = new THREE.Vector3();
  // Outside the rig, so a jump's squash never squashes the words.
  private readonly bubble: SpeechBubble;
  private readonly grip = new THREE.Group(); // turns and scales the held figure to fit

  private held: THREE.Object3D | null = null;
  private holding = 0;
  private jumpElapsed = Infinity; // not jumping
  private gait: 'stand' | 'walk' | 'run' = 'stand';
  private moving = 0;
  private running = 0;
  private phase = 0;
  private currentSpeed = 0;

  protected constructor(
    theme: Theme,
    protected readonly shape: AnimalShape,
  ) {
    super();
    this.holder.add(this.grip);
    this.bubble = new SpeechBubble(theme);
    this.bubble.position.copy(shape.bubble);
    const scale = shape.bubbleScale ?? THREE.MathUtils.clamp(shape.bubble.y / BUBBLE_SCALE.per, BUBBLE_SCALE.least, BUBBLE_SCALE.most);
    this.bubble.scale.setScalar(scale);
    this.add(this.rig, this.bubble);
  }

  // Forward speed in meters per second; 0 when standing. Turning the animal
  // by speed / radius radians a second walks it round a circle of that radius.
  get speed(): number {
    return this.currentSpeed;
  }

  private get airtime(): number {
    return 2 * Math.sqrt((2 * this.shape.jump) / GRAVITY);
  }

  // Crouches, springs up (its jump height, just the time in the air gravity
  // gives it) and lands, bending to take the landing, carrying anything it
  // holds. Ignored while a jump is under way. Named as in the spec.
  Jump(): void {
    if (this.jumpElapsed < 2 * CROUCH_TIME + this.airtime) return;
    this.jumpElapsed = 0;
  }

  // Holds a figure in its mouth (or beak): the figure is moved into the
  // animal (its position and rotation reset), turned so its longest level
  // side lies across the mouth, and scaled down to fit if it is bigger than
  // the animal can carry. Holding another figure drops the first;
  // Hold(null) drops what it holds. Named as in the spec.
  Hold(figure: THREE.Object3D | null): void {
    if (this.held) this.grip.remove(this.held);
    this.held = figure;
    if (!figure) return;

    figure.removeFromParent();
    figure.position.set(0, 0, 0);
    figure.rotation.set(0, 0, 0);
    figure.updateMatrixWorld(true);
    const box = new THREE.Box3().setFromObject(figure);
    const size = box.getSize(new THREE.Vector3());
    figure.position.copy(box.getCenter(new THREE.Vector3())).negate();
    this.grip.rotation.y = size.z > size.x ? Math.PI / 2 : 0;
    this.grip.scale.setScalar(Math.min(1, this.shape.hold / Math.max(size.x, size.y, size.z)));
    this.grip.add(figure);
  }

  // Walks forward the way it faces until Run() or Stop(). Named as in the spec.
  Walk(): void {
    this.gait = 'walk';
  }

  // Runs forward the way it faces until Walk() or Stop(). Named as in the spec.
  Run(): void {
    this.gait = 'run';
  }

  // Slows from a walk or run to standing still. Not in the spec yet.
  Stop(): void {
    this.gait = 'stand';
  }

  // Says the text in a speech bubble above its head. A bubble holds at most
  // nine words, so longer text is split into several, shown one after
  // another, each long enough to read. Saying something new cuts off what it
  // was saying; Speech('') just stops. Named as in the spec. Beyond the
  // spec, `speaker` names who speaks at the top of each bubble (a viewer, at
  // the lake meeting).
  Speech(text: string, speaker = ''): void {
    this.bubble.say(text, speaker);
  }

  // Whether a speech bubble is still showing.
  get speaking(): boolean {
    return this.bubble.speaking;
  }

  // The speech bubble, whose origin is its tail's tip, for framing what is
  // said (the lake meeting's camera).
  get speechBubble(): THREE.Object3D {
    return this.bubble;
  }

  update(delta: number): void {
    const ease = 1 - Math.exp(-GAIT_SPEED * delta);
    this.moving += ((this.gait === 'stand' ? 0 : 1) - this.moving) * ease;
    this.running += ((this.gait === 'run' ? 1 : 0) - this.running) * ease;
    const { walk, run } = this.shape;
    const cadence = THREE.MathUtils.lerp(walk.cadence, run.cadence, this.running);
    const duty = THREE.MathUtils.lerp(walk.duty, run.duty, this.running);
    const stride = THREE.MathUtils.lerp(walk.stride, run.stride, this.running) * this.moving;
    this.phase = (this.phase + cadence * delta) % 1;
    this.currentSpeed = (stride * cadence) / duty;
    forward.set(0, 0, 1).applyQuaternion(this.quaternion).setY(0);
    if (forward.lengthSq() > 0) this.position.addScaledVector(forward.normalize(), this.currentSpeed * delta);

    // Jump: crouch, fly a parabola, crouch again landing.
    let height = 0;
    let crouch = 0;
    let air = 0;
    let tuck = 0;
    const airtime = this.airtime;
    if (this.jumpElapsed < 2 * CROUCH_TIME + airtime) {
      this.jumpElapsed += delta;
      const t = this.jumpElapsed;
      if (t < CROUCH_TIME) {
        crouch = Math.sin((Math.PI / 2) * (t / CROUCH_TIME)); // down, springing at the bottom
      } else if (t < CROUCH_TIME + airtime) {
        const q = (t - CROUCH_TIME) / airtime;
        height = 4 * this.shape.jump * q * (1 - q);
        air = 2 * q - 1;
        tuck = Math.sin(Math.PI * q);
        crouch = Math.max(0, 1 - q / 0.1) * 0.6; // straightening as it leaves the ground
      } else {
        const q = Math.min(1, (t - CROUCH_TIME - airtime) / CROUCH_TIME);
        crouch = Math.sin(Math.PI * q) * 0.8; // giving on landing, then standing up
      }
    }

    this.holding += ((this.held ? 1 : 0) - this.holding) * (1 - Math.exp(-HOLD_SPEED * delta));

    this.pose({
      delta,
      phase: this.phase,
      moving: this.moving,
      running: this.running,
      stride,
      duty,
      height,
      crouch,
      air,
      tuck,
      holding: this.holding,
    });
    this.bubble.position.copy(this.shape.bubble).add(this.bubbleShift);
    this.bubble.position.y += height;
    this.bubble.update(delta);
  }

  // Poses the body for this frame.
  protected abstract pose(motion: Motion): void;
}
