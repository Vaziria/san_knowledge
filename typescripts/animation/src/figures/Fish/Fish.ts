import * as THREE from 'three';
import { defaultTheme, type Theme } from '../../theme';
import { Body, sizeAt } from './Body';
import { Fin } from './Fin';
import { FISH_MODELS, SALMON, type FishKind, type FishModel } from './models';
import { createMaterials } from './parts';
import { Tail } from './Tail';

export { FISH_KINDS, type FishKind } from './models';

// A fish of one of three kinds, each built from the user's low poly model
// (models.ts; the shapes are in animals/Fish.md, the behaviours in Fish.md):
// a salmon (the default), 30 cm long, a piranha, 25 cm, or a clownfish,
// 10 cm. Put together from its parts: the body (with its back and belly
// fins, eyes, and the piranha's jaw and teeth), the forked tail fin and a
// pair of pectoral fins. Every face is flat and of one colour, mapped from
// the model's onto the theme. Units are meters, the snout points +z, and the
// origin is on the floor under the middle of the fish (the model's origin,
// which it turns about), below its lowest point. At rest, as built (and as
// held), every face is its model's.
//
// Behaviour, from the spec (Fish.md), the same for every kind:
// SwimOnSurface(direction) swims that way just under the surface, with the
// dorsal fin cutting through it, and SwimOnDepth(direction) swims that way
// deeper down. The water's surface is y = 0 of the fish's parent, as in the
// lake (environtments/Lake). The fish moves itself: it turns toward the
// direction the short way round, goes forward along its own +z at `speed`,
// and rises or dives to its depth, nose up or down. Both keep going until
// another behaviour replaces them. Stop(), added beyond the spec, glides to a
// halt and hovers. Call update(delta) once per frame. Its speeds and depths
// follow its size: the salmon swims 0.35 m/s and dives 0.6 m down, as the
// fish did before its kinds; a smaller fish dives less deep, and each kind
// swims as many more lengths a second as its model beats its tail faster.
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
// While it swims, the tail beats, faster and wider the faster it goes, up to
// the model's own sway at full speed: the back of the body bends in the
// model's wave, more and more toward the tail, and the tail fin flicks
// further as the model's does; the body sways against the tail, the pectoral
// fins paddle as the model's do, and it leans into turns with its tail bent
// toward the turn. It keeps finning gently while it hovers.

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

// The salmon's, as the fish's were before its kinds. Another kind's
// distances and speeds are these times its length over the salmon's (its
// `size`), and it swims faster again by as much as its model beats faster
// (its swim.speed over the salmon's): the piranha 1.6 lengths a second, the
// clownfish 1.4, the salmon 1.2.
const SWIM_SPEED = 0.35; // m/s, a little over a body length a second
const SURFACE_DIP = 0.01; // on the surface, the back is this far under the water
const DEPTH = 0.6; // at depth, the back is this far under the water
const DIVE_SPEED = 0.25; // fastest rise or dive, m/s
const SLOWEST = 0.1; // m/s: its nose follows the way it rises or dives as if it swam at least this fast

const ACCELERATION = 1.5; // how quickly it reaches its speed or stops, per second
const TURN_RATE = 2; // fastest turn, radians a second
const TURN_EASE = 3; // how quickly it turns toward its heading, per second
const TURN_RESPONSE = 6; // how quickly a turn starts and ends, per second
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
const RUN_UP = 0.35; // near the surface it first dives until its middle is this far under (the salmon's)
const SINK_SPEED = 0.6; // m/s on average, diving to the run-up depth (the salmon's)
const MIN_SINK = 0.2; // seconds, the shortest dive
const MAX_JUMP = 2; // m, the highest leap
const JUMP_PITCH_LIMIT = 1.4; // nose up or down in a leap and its plunge back in, radians
const DRAG = 6; // how quickly the water brakes a leap's speed back down to swimming, per second

// A fish moves about 0.75 of its length per tail beat, so the beat quickens
// with its speed. The body's wave and the tail fin's flick are the model's
// at full speed; while hovering both keep IDLE of their full-speed beat and
// size. In a leap's burst of speed the beat quickens to at most BURST times
// it, and the head's sway grows as much, but the wave and the flick stay the
// model's size: already wide (the tail end turned up to 36° and the fin 20°
// more), at BURST times it they turned the tail fin edge-on to the side.
const STRIDE = 0.75;
const SWAY = 0.08; // radians each way the head sways against the tail, at full speed
const IDLE = 0.25;
const BURST = 1.8;
const TURN_BEND = 0.25; // at the tail end the body bends toward a turn by this x the turn rate, radians
const BANK = 0.12; // it leans into a turn by this x the turn rate
const FOLDED = 0.5; // at full speed the pectoral fins paddle this share of their stroke

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
  kind?: FishKind; // the salmon when left out
}

// `splash`: in a leap, the middle of the fish breaks the water's surface,
// leaving it or coming back in, at (x, z) in its parent's coordinates, with
// `velocity` in meters a second (up when it leaves, down when it comes back).
export interface FishEventMap extends THREE.Object3DEventMap {
  splash: { x: number; z: number; velocity: THREE.Vector3 };
}

// From the front of the nose (the piranha's front teeth) to the tail fin's
// tips at rest, in the model's units.
function modelLength(model: FishModel<string>): number {
  const ahead = model.mouth ? [...Object.values(model.mouth.points), ...model.mouth.teeth.flat()].map((p) => p[2]) : [];
  const front = Math.max(model.noseTip[0], ...ahead);
  const back = model.profile[0][0] + Math.min(...Object.values(model.tail.points).map((p) => p[2]));
  return front - back;
}

const SALMON_LENGTH = modelLength(SALMON as FishModel<string>) * SALMON.unit;

const UP = new THREE.Vector3(0, 1, 0);
const ORIGIN = new THREE.Vector3();
const turned = new THREE.Matrix4();

export class Fish extends THREE.Group<FishEventMap> {
  static readonly MAX_JUMP = MAX_JUMP;

  readonly kind: FishKind;
  // From the snout (the piranha's front teeth) to the tips of the tail fin.
  readonly length: number;
  // Its length over the salmon's: the salmon's distances and speeds times
  // this are its own (a preview's square too).
  readonly size: number;
  readonly swimSpeed: number; // m/s, swimming
  readonly depth: number; // m its back is under the water at depth

  // Each part is its own group with its origin at its pivot, ready to animate.
  readonly body: Body;
  readonly tail: Tail;
  readonly leftFin: Fin;
  readonly rightFin: Fin;

  // The height of the fish's origin while it swims on the surface and at
  // depth, with the water's surface at y = 0.
  readonly surfaceY: number;
  readonly depthY: number;

  // All the parts, pivoted at the middle of the body (the model's origin),
  // so the fish pitches, leans and sways around its middle.
  private readonly rig = new THREE.Group();
  private readonly model: FishModel<string>;

  // The height of the fish's origin when the middle of its body is at the
  // surface: where a leap leaves the water and comes back in.
  private readonly launchY: number;
  private readonly diveSpeed: number;
  private readonly slowest: number;
  private readonly runUp: number;
  private readonly sinkSpeed: number;
  private readonly turnBend: number; // the model's units at the tail end per radian a second of turning

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
    const kind = (this.kind = options.kind ?? 'salmon');
    this.name = kind;
    const model = (this.model = FISH_MODELS[kind]);
    const m = createMaterials(options.theme ?? defaultTheme);
    const colors = model.colors(m.palette);
    const { unit } = model;

    // In the rig, the origin is the model's.
    this.body = new Body(m, model, colors);
    this.tail = new Tail(m, model, colors);
    const { x, y, z } = model.pectoral.at;
    const finX = (x ?? sizeAt(model.profile, z)[0] * 0.92) * unit; // the factory's: 0.92 of the body's half-width there
    this.leftFin = new Fin(m, model, colors, -1);
    this.leftFin.position.set(-finX, y * unit, z * unit);
    this.rightFin = new Fin(m, model, colors, 1);
    this.rightFin.position.set(finX, y * unit, z * unit);
    this.placeTail(0);

    this.rig.add(this.body, this.tail, this.leftFin, this.rightFin);
    this.add(this.rig);

    // Lift the rig so the lowest point of the fish touches the floor.
    const middle = -new THREE.Box3().setFromObject(this.rig, true).min.y;
    this.rig.position.y = middle;

    const length = (this.length = modelLength(model) * unit);
    const size = (this.size = length / SALMON_LENGTH);
    this.swimSpeed = SWIM_SPEED * size * (model.swim.speed / SALMON.swim.speed);
    this.depth = DEPTH * size;
    this.diveSpeed = DIVE_SPEED * size;
    this.slowest = SLOWEST * size;
    this.runUp = RUN_UP * size;
    this.sinkSpeed = SINK_SPEED * size;
    // A bend growing with the square of the way from `stillFrom` to the tail
    // end slopes there by twice its offset over that way.
    this.turnBend = (TURN_BEND * (model.swim.stillFrom - model.profile[0][0])) / 2;

    this.surfaceY = -(middle + this.body.top + SURFACE_DIP * size);
    this.depthY = -(middle + this.body.top + this.depth);
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
    if (this.jump || this.climb < -this.diveSpeed || !Number.isFinite(height)) return;
    const top = THREE.MathUtils.clamp(height, 0, MAX_JUMP);
    const lift = Math.sqrt(2 * GRAVITY * (top - this.launchY)); // up speed as it leaves the water
    const launch = lift / Math.tan(LAUNCH_ANGLE); // forward speed then, kept in the air
    const y = this.position.y;
    const climb = this.climb;
    const speed = this.currentSpeed;
    this.targetY ??= y;

    // Near the surface it first dives to the run-up depth and comes to rest
    // there; deeper down it rises from where it is.
    const runUpY = this.launchY - this.runUp;
    const sink = y > runUpY ? Math.max(MIN_SINK, (y - runUpY) / this.sinkSpeed) : 0;
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
    this.targetSpeed = this.swimSpeed;
  }

  update(delta: number): void {
    // In a leap the fish follows its plan. Once it is back in the water it
    // swims for what is left of the frame.
    const swim = this.jump ? this.leap(this.jump, delta) : delta;
    const ease = (rate: number) => 1 - Math.exp(-rate * swim);
    // Faster than it swims (after a leap), the water brakes it harder.
    this.currentSpeed += (this.targetSpeed - this.currentSpeed) * ease(this.currentSpeed > this.swimSpeed ? DRAG : ACCELERATION);
    const effort = THREE.MathUtils.clamp(this.currentSpeed / this.swimSpeed, IDLE, BURST); // IDLE hovering .. 1 at full speed

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
      wantedClimb = THREE.MathUtils.clamp((this.targetY - this.position.y) * DIVE_EASE, -this.diveSpeed, this.diveSpeed);
    }
    this.climb += (wantedClimb - this.climb) * ease(Math.abs(this.climb) > this.diveSpeed ? DRAG : DIVE_RESPONSE);
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
      const limit = jump || this.climb < -this.diveSpeed ? JUMP_PITCH_LIMIT : PITCH_LIMIT;
      const pitch = THREE.MathUtils.clamp(Math.atan2(-this.climb, Math.max(this.currentSpeed, this.slowest)), -limit, limit);
      rig.rotation.x += (pitch - rig.rotation.x) * (1 - Math.exp(-PITCH_RESPONSE * delta));
    }
    rig.rotation.z = -BANK * this.turnRate;

    // The tail beats: the body bends in the model's wave, `effort` of its
    // size (at most all of it), bent toward a turn, and the tail fin flicks
    // further; the head sways against the tail. A positive wave puts the
    // tail end at -x and the snout at +x.
    const beat = (effort * this.swimSpeed) / (STRIDE * this.length); // beats a second
    this.tailPhase += 2 * Math.PI * beat * delta;
    const wave = Math.sin(this.tailPhase);
    const size = Math.min(effort, 1);
    this.body.bend(size, this.tailPhase, this.turnBend * this.turnRate);
    this.placeTail(this.model.tail.flick * size * wave);
    rig.rotation.y = SWAY * effort * wave;

    // Both pectoral fins paddle together about their roots, as the model's
    // do, less far at speed; turning about y by +side swings a fin's tip in
    // toward the body.
    const { flap } = this.model.pectoral;
    this.finPhase += flap.speed * delta;
    const fan = flap.stroke * (1 - FOLDED * Math.min(1, this.currentSpeed / this.swimSpeed)) * Math.sin(this.finPhase);
    this.rightFin.rotation.y = fan;
    this.leftFin.rotation.y = -fan;
  }

  // Puts the tail fin at the body's tail end, facing the way the body runs
  // there (as the model's lookAt means to), then flicks it `flick` radians
  // further about its own y: > 0 swings its tips toward -x.
  private placeTail(flick: number): void {
    const { tail, body } = this;
    tail.position.copy(body.tailRoot);
    tail.quaternion.setFromRotationMatrix(turned.lookAt(body.tailAxis, ORIGIN, UP));
    tail.rotateY(flick);
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
