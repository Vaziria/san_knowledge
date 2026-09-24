import * as THREE from 'three';
import { defaultTheme, type Theme } from '../../theme';
import { Body, BODY_LENGTH, BODY_TOP, sideX } from './Body';
import { Fin } from './Fin';
import { createMaterials } from './parts';
import { Tail, TAIL_LENGTH } from './Tail';

// A freshwater fish about 30 cm long, like a tilapia or a carp, put together
// from its parts: the body (with its back and belly fins, eyes and mouth), the
// forked tail fin and a pair of pectoral fins. The back is the theme's body
// colour, the belly its light colour and the fins its trim. Units are meters,
// the snout points +z, and the origin is on the floor under the middle of the
// fish, below its lowest point (the tip of the fin under the tail end).
//
// Behaviour, from the spec (Fish.md): SwimOnSurface(direction) swims that way
// just under the surface, with the dorsal fin cutting through it, and
// SwimOnDepth(direction) swims that way DEPTH down. The water's surface is
// y = 0 of the fish's parent, as in the lake (environtments/Lake). The fish
// moves itself: it turns toward the direction the short way round, goes
// forward along its own +z at `speed`, and rises or dives to its depth, nose
// up or down. Both keep going until another behaviour replaces them. Stop(),
// added beyond the spec, glides to a halt and hovers. Call update(delta) once
// per frame.
//
// JumpOutFromWater(height) leaps out of the water the way the fish faces and
// back in, once. Near the surface it first dives to gather speed, then it
// shoots up and flies in an arc under gravity, high enough that its lowest
// point clears the surface by `height` at the top. Back in the water, the
// water brakes it and it swims on as before; swimming behaviours given during
// the leap take over once it is back in. Where its middle leaves the water
// and where it comes back in, it sends a `splash` event, so the water can
// splash there.
//
// While it swims, the tail beats (faster and wider the faster it goes), the
// body sways against it, the pectoral fins paddle, and it leans into turns
// with the tail bent toward the turn. It keeps finning gently while it hovers.

// Which way to swim, in the scene's axes (the axes figures are built in):
// forward is +z, back -z, left -x and right +x. The spec names the type but
// not its values; this is the reading chosen.
export type Direction = 'forward' | 'back' | 'left' | 'right';

// The heading (rotation.y) that faces each direction.
const HEADINGS: Record<Direction, number> = {
  forward: 0,
  right: Math.PI / 2,
  back: Math.PI,
  left: -Math.PI / 2,
};

// Every direction, for menus.
export const DIRECTIONS = Object.keys(HEADINGS) as Direction[];

const TAIL_ROOT = 0.04; // where the tail fin grows from the tail base, as u of the body
const PECTORAL_U = 0.72; // just behind the gills
const PECTORAL_Y = -0.012; // below the center line
const PECTORAL_INSET = 0.002; // the fin's base sits this far inside the body's side

const SWIM_SPEED = 0.35; // m/s, a little over a body length a second
const ACCELERATION = 1.5; // how quickly it reaches its speed or stops, per second
const TURN_RATE = 2; // fastest turn, radians a second
const TURN_EASE = 3; // how quickly it turns toward its heading, per second
const TURN_RESPONSE = 6; // how quickly a turn starts and ends, per second
const SURFACE_DIP = 0.01; // on the surface, the back is this far under the water
const DEPTH = 0.6; // at depth, the back is this far under the water
const DIVE_SPEED = 0.25; // fastest rise or dive, m/s
const DIVE_EASE = 1.5; // how quickly it settles at its depth, per second
const DIVE_RESPONSE = 4; // how quickly a rise or dive starts and ends, per second
const PITCH_LIMIT = 0.5; // nose up or down while rising or diving, radians
const PITCH_RESPONSE = 10; // how quickly its nose follows the way it swims, per second

// JumpOutFromWater. The whole leap is worked out when it starts, so that it
// clears the water by exactly the height asked: the middle of the fish leaves
// the water LAUNCH_ANGLE above level, just fast enough for gravity to stop
// its rise at that height.
const GRAVITY = 9.81; // m/s²
const LAUNCH_ANGLE = Math.PI / 3; // above level
const RUN_UP = 0.35; // near the surface it first dives until its middle is this far under
const SINK_SPEED = 0.6; // m/s on average, diving to the run-up depth
const MIN_SINK = 0.2; // seconds, the shortest dive
const MAX_JUMP = 2; // m, the highest leap
const JUMP_PITCH_LIMIT = 1.4; // nose up or down in a leap and its plunge back in, radians
const DRAG = 6; // how quickly the water brings a leap's speed back down to swimming, per second

// A fish moves about 0.75 of its length per tail beat, so the beat quickens
// with its speed. Swing and sway are radians each way at full speed; while
// hovering both keep IDLE of their full-speed beat and size, and in a leap's
// burst of speed they grow to at most BURST times it.
const STRIDE = 0.75;
const TAIL_SWING = 0.45;
const SWAY = 0.08; // the head's sway against the tail
const IDLE = 0.25;
const BURST = 1.8;
const TURN_BEND = 0.25; // the tail bends toward a turn by this x the turn rate
const BANK = 0.12; // it leans into a turn by this x the turn rate
const FIN_BEAT = 1.6; // pectoral strokes a second
const FIN_STROKE = 0.35; // how far a pectoral fin fans out, radians; half that at full speed

// a + b t + c t² + d t³, as [a, b, c, d].
type Cubic = [number, number, number, number];

const valueAt = ([a, b, c, d]: Cubic, t: number) => a + t * (b + t * (c + t * d));
const slopeAt = ([, b, c, d]: Cubic, t: number) => b + t * (2 * c + 3 * d * t);

// One stage of a leap: the height of the fish's origin and its distance along
// the leap, as cubics in the seconds since the stage began.
interface Stage {
  duration: number;
  y: Cubic;
  s: Cubic;
}

// A leap, planned when it starts. It goes straight the way the fish faced
// then: a dive to the run-up depth (only when it started shallower), the rise
// to the surface, and the flight, from its middle leaving the water to its
// middle coming back in.
interface Jump {
  time: number; // seconds since it started
  x: number; // where it started
  z: number;
  dx: number; // the way it goes, a unit vector
  dz: number;
  stages: Stage[]; // the last one is the flight
  takeoff: number; // seconds from the start to the flight
  launchPitch: number | null; // its pitch as it left the water, once it has
}

export interface FishOptions {
  theme?: Theme;
}

// `splash`: in a leap, the middle of the fish breaks the water's surface,
// leaving it or coming back in, at (x, z) in its parent's coordinates, with
// `velocity` in meters a second (up when it leaves, down when it comes back).
export interface FishEventMap extends THREE.Object3DEventMap {
  splash: { x: number; z: number; velocity: THREE.Vector3 };
}

export class Fish extends THREE.Group<FishEventMap> {
  // From the snout to the tips of the tail fin.
  static readonly LENGTH = BODY_LENGTH + TAIL_LENGTH - TAIL_ROOT * BODY_LENGTH;
  static readonly SWIM_SPEED = SWIM_SPEED;
  static readonly DEPTH = DEPTH;
  static readonly MAX_JUMP = MAX_JUMP;

  // Each part is its own group with its origin at its pivot, ready to animate.
  readonly body: Body;
  readonly tail: Tail;
  readonly leftFin: Fin;
  readonly rightFin: Fin;

  // The height of the fish's origin while it swims on the surface and at
  // depth, with the water's surface at y = 0.
  readonly surfaceY: number;
  readonly depthY: number;

  // All the parts, pivoted at the middle of the body's center line, so the
  // fish pitches, leans and sways around its middle.
  private readonly rig = new THREE.Group();

  // The height of the fish's origin when the middle of its body is at the
  // surface: where a leap leaves the water and comes back in.
  private readonly launchY: number;

  private currentSpeed = 0;
  private targetSpeed = 0;
  private heading: number | null = null; // the heading it turns toward; null keeps its own
  private targetY: number | null = null; // the height it rises or dives to; null stays put
  private turnRate = 0; // radians a second, > 0 turns toward +x
  private climb = 0; // m/s, > 0 rises
  private tailPhase = 0;
  private finPhase = 0;
  private jump: Jump | null = null;

  constructor(options: FishOptions = {}) {
    super();
    this.name = 'fish';
    const m = createMaterials(options.theme ?? defaultTheme);

    // Along z: the snout at +LENGTH / 2, the tail tips at -LENGTH / 2. In the
    // rig, y = 0 is the body's center line.
    const bodyStart = Fish.LENGTH / 2 - BODY_LENGTH;

    this.body = new Body(m);
    this.body.position.z = bodyStart;

    this.tail = new Tail(m);
    this.tail.position.z = bodyStart + TAIL_ROOT * BODY_LENGTH;

    const finX = sideX(PECTORAL_U, PECTORAL_Y) - PECTORAL_INSET;
    const finZ = bodyStart + PECTORAL_U * BODY_LENGTH;
    this.leftFin = new Fin(m, -1);
    this.leftFin.position.set(-finX, PECTORAL_Y, finZ);
    this.rightFin = new Fin(m, 1);
    this.rightFin.position.set(finX, PECTORAL_Y, finZ);

    this.rig.add(this.body, this.tail, this.leftFin, this.rightFin);
    this.add(this.rig);

    // Lift the rig so the lowest point of the fish touches the floor.
    const middle = -new THREE.Box3().setFromObject(this.rig, true).min.y;
    this.rig.position.y = middle;

    this.surfaceY = -(middle + BODY_TOP + SURFACE_DIP);
    this.depthY = -(middle + BODY_TOP + DEPTH);
    this.launchY = -middle;
  }

  // Forward speed in meters per second; 0 when hovering.
  get speed(): number {
    return this.currentSpeed;
  }

  // Whether a leap is under way, until the fish is back in the water.
  get jumping(): boolean {
    return this.jump !== null;
  }

  // Spec (Fish.md): SwimOnSurface(direction Direction).
  SwimOnSurface(direction: Direction): void {
    this.swim(direction, this.surfaceY);
  }

  // Spec (Fish.md): SwimOnDepth(direction Direction).
  SwimOnDepth(direction: Direction): void {
    this.swim(direction, this.depthY);
  }

  // Spec (Fish.md): JumpOutFromWater(height). Leaps the way it faces, so
  // high that at the top its lowest point is `height` meters above the
  // surface (clamped to 0..MAX_JUMP), and falls back in. Afterwards it swims
  // on as before; a fish that had no depth to keep goes back to the depth it
  // leapt from. Ignored during a leap and until the water has slowed its
  // plunge back in, and for a height that is not a number.
  JumpOutFromWater(height: number): void {
    if (this.jump || this.climb < -DIVE_SPEED || !Number.isFinite(height)) return;
    const top = THREE.MathUtils.clamp(height, 0, MAX_JUMP);
    const lift = Math.sqrt(2 * GRAVITY * (top - this.launchY)); // up speed as it leaves the water
    const launch = lift / Math.tan(LAUNCH_ANGLE); // forward speed then, kept in the air
    const y = this.position.y;
    const climb = this.climb;
    const speed = this.currentSpeed;
    this.targetY ??= y;

    // Near the surface it first dives to the run-up depth and comes to rest
    // there; deeper down it rises from where it is.
    const runUpY = this.launchY - RUN_UP;
    const sink = y > runUpY ? Math.max(MIN_SINK, (y - runUpY) / SINK_SPEED) : 0;
    const fromY = sink > 0 ? runUpY : y;
    const fromClimb = sink > 0 ? 0 : climb;
    // The rise starts without a jolt: its up speed grows from fromClimb with
    // the square of the time, reaching `lift` at the surface.
    const rise = (3 * (this.launchY - fromY)) / (2 * fromClimb + lift);
    // Forward, it speeds up evenly from its speed now to `launch`.
    const push = (launch - speed) / (sink + rise);
    const diveRun = speed * sink + (push / 2) * sink * sink; // how far forward it goes while diving
    const run = diveRun + (speed + push * sink) * rise + (push / 2) * rise * rise; // and by the time it leaves the water

    const stages: Stage[] = [];
    if (sink > 0) {
      // From its height and climb now to rest at the run-up depth.
      const drop = runUpY - y;
      stages.push({
        duration: sink,
        y: [y, climb, ((3 * drop) / sink - 2 * climb) / sink, (climb - (2 * drop) / sink) / sink ** 2],
        s: [0, speed, push / 2, 0],
      });
    }
    stages.push(
      {
        duration: rise,
        y: [fromY, fromClimb, 0, (lift - fromClimb) / (3 * rise * rise)],
        s: [diveRun, speed + push * sink, push / 2, 0],
      },
      {
        duration: (2 * lift) / GRAVITY,
        y: [this.launchY, lift, -GRAVITY / 2, 0],
        s: [run, launch, 0, 0],
      },
    );
    this.jump = {
      time: 0,
      x: this.position.x,
      z: this.position.z,
      dx: Math.sin(this.rotation.y),
      dz: Math.cos(this.rotation.y),
      stages,
      takeoff: sink + rise,
      launchPitch: null,
    };
  }

  // Beyond the spec: glides to a halt and hovers at its depth, finning gently.
  Stop(): void {
    this.targetSpeed = 0;
  }

  // An unknown direction is ignored.
  private swim(direction: Direction, y: number): void {
    if (!Object.hasOwn(HEADINGS, direction)) return;
    this.heading = HEADINGS[direction];
    this.targetY = y;
    this.targetSpeed = SWIM_SPEED;
  }

  update(delta: number): void {
    // In a leap the fish follows its plan. Once it is back in the water it
    // swims for what is left of the frame.
    const swim = this.jump ? this.leap(this.jump, delta) : delta;
    const ease = (rate: number) => 1 - Math.exp(-rate * swim);
    // Faster than it swims (after a leap), the water brakes it harder.
    this.currentSpeed += (this.targetSpeed - this.currentSpeed) * ease(this.currentSpeed > SWIM_SPEED ? DRAG : ACCELERATION);
    const effort = THREE.MathUtils.clamp(this.currentSpeed / SWIM_SPEED, IDLE, BURST); // IDLE hovering .. 1 at full speed

    // Turn toward the heading the short way round, more slowly when slow.
    let wantedTurn = 0;
    if (this.heading !== null) {
      const off = this.heading - this.rotation.y;
      const shortest = Math.atan2(Math.sin(off), Math.cos(off));
      wantedTurn = THREE.MathUtils.clamp(shortest * TURN_EASE, -TURN_RATE, TURN_RATE) * (0.4 + 0.6 * Math.min(1, effort));
    }
    this.turnRate += (wantedTurn - this.turnRate) * ease(TURN_RESPONSE);
    this.rotation.y += this.turnRate * swim;

    // Forward along its own +z.
    this.position.x += Math.sin(this.rotation.y) * this.currentSpeed * swim;
    this.position.z += Math.cos(this.rotation.y) * this.currentSpeed * swim;

    // Rise or dive toward its depth, braking harder while it plunges back in
    // from a leap.
    let wantedClimb = 0;
    if (this.targetY !== null) {
      wantedClimb = THREE.MathUtils.clamp((this.targetY - this.position.y) * DIVE_EASE, -DIVE_SPEED, DIVE_SPEED);
    }
    this.climb += (wantedClimb - this.climb) * ease(Math.abs(this.climb) > DIVE_SPEED ? DRAG : DIVE_RESPONSE);
    this.position.y += this.climb * swim;

    // Nose up while rising and down while diving (rotation.x > 0 dips the
    // nose), easing toward the way it swims, and leaning into turns. In a
    // leap's flight it has nothing to push on, so it turns at a steady rate
    // instead: from its pitch as it leaves the water to as far nose down as
    // it comes back in, level at the top.
    const rig = this.rig;
    const jump = this.jump;
    if (jump && jump.time > jump.takeoff) {
      jump.launchPitch ??= rig.rotation.x;
      const flight = jump.stages[jump.stages.length - 1].duration;
      rig.rotation.x = jump.launchPitch * (1 - (2 * (jump.time - jump.takeoff)) / flight);
    } else {
      const limit = jump || this.climb < -DIVE_SPEED ? JUMP_PITCH_LIMIT : PITCH_LIMIT;
      const pitch = THREE.MathUtils.clamp(Math.atan2(-this.climb, Math.max(this.currentSpeed, 0.1)), -limit, limit);
      rig.rotation.x += (pitch - rig.rotation.x) * (1 - Math.exp(-PITCH_RESPONSE * delta));
    }
    rig.rotation.z = -BANK * this.turnRate;

    // The tail beats, bent toward a turn, and the body sways against it: a
    // positive swing puts the tail tip at -x and the snout at +x.
    const beat = (effort * SWIM_SPEED) / (STRIDE * Fish.LENGTH); // beats a second
    this.tailPhase += 2 * Math.PI * beat * delta;
    const wave = Math.sin(this.tailPhase);
    this.tail.rotation.y = TAIL_SWING * effort * wave - TURN_BEND * this.turnRate;
    rig.rotation.y = SWAY * effort * wave;

    // Both pectoral fins fan out and back together; turning about y by +side
    // swings a fin's tip in toward the body, so -side fans it out.
    this.finPhase += 2 * Math.PI * FIN_BEAT * delta;
    const fan = FIN_STROKE * (1 - 0.5 * Math.min(1, this.currentSpeed / SWIM_SPEED)) * (0.5 + 0.5 * Math.sin(this.finPhase));
    this.leftFin.rotation.y = fan;
    this.rightFin.rotation.y = -fan;
  }

  // Moves the fish delta seconds on along its leap, holding its heading, and
  // splashes as it leaves the water. Returns the seconds left over once it is
  // back in the water, splashing again: 0 while the leap goes on.
  private leap(jump: Jump, delta: number): number {
    const flight = jump.stages[jump.stages.length - 1];
    const leaving = jump.time <= jump.takeoff && jump.time + delta > jump.takeoff;
    this.turnRate *= Math.exp(-TURN_RESPONSE * delta);
    jump.time += delta;
    let t = jump.time;
    for (const stage of jump.stages) {
      if (t <= stage.duration) {
        this.follow(jump, stage, t);
        if (leaving) this.splash(jump, flight, 0);
        return 0;
      }
      t -= stage.duration;
    }
    this.follow(jump, flight, flight.duration);
    this.jump = null;
    if (leaving) this.splash(jump, flight, 0); // the whole flight within one frame
    this.splash(jump, flight, flight.duration);
    return Math.min(t, delta);
  }

  // Sends the splash of the fish's middle breaking the surface, t seconds
  // into its flight.
  private splash(jump: Jump, flight: Stage, t: number): void {
    const s = valueAt(flight.s, t);
    const forward = slopeAt(flight.s, t);
    const velocity = new THREE.Vector3(jump.dx * forward, slopeAt(flight.y, t), jump.dz * forward);
    this.dispatchEvent({ type: 'splash', x: jump.x + jump.dx * s, z: jump.z + jump.dz * s, velocity });
  }

  // Puts the fish t seconds into a stage of its leap, with the speed and
  // climb it has there, so it swims on smoothly when the leap ends.
  private follow(jump: Jump, stage: Stage, t: number): void {
    const s = valueAt(stage.s, t);
    this.position.set(jump.x + jump.dx * s, valueAt(stage.y, t), jump.z + jump.dz * s);
    this.currentSpeed = slopeAt(stage.s, t);
    this.climb = slopeAt(stage.y, t);
  }
}
