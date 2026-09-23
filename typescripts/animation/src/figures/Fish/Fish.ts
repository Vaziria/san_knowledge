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

// A fish moves about 0.75 of its length per tail beat, so the beat quickens
// with its speed. Swing and sway are radians each way at full speed; while
// hovering both keep IDLE of their full-speed beat and size.
const STRIDE = 0.75;
const TAIL_SWING = 0.45;
const SWAY = 0.08; // the head's sway against the tail
const IDLE = 0.25;
const TURN_BEND = 0.25; // the tail bends toward a turn by this x the turn rate
const BANK = 0.12; // it leans into a turn by this x the turn rate
const FIN_BEAT = 1.6; // pectoral strokes a second
const FIN_STROKE = 0.35; // how far a pectoral fin fans out, radians; half that at full speed

export interface FishOptions {
  theme?: Theme;
}

export class Fish extends THREE.Group {
  // From the snout to the tips of the tail fin.
  static readonly LENGTH = BODY_LENGTH + TAIL_LENGTH - TAIL_ROOT * BODY_LENGTH;
  static readonly SWIM_SPEED = SWIM_SPEED;
  static readonly DEPTH = DEPTH;

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

  private currentSpeed = 0;
  private targetSpeed = 0;
  private heading: number | null = null; // the heading it turns toward; null keeps its own
  private targetY: number | null = null; // the height it rises or dives to; null stays put
  private turnRate = 0; // radians a second, > 0 turns toward +x
  private climb = 0; // m/s, > 0 rises
  private tailPhase = 0;
  private finPhase = 0;

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
  }

  // Forward speed in meters per second; 0 when hovering.
  get speed(): number {
    return this.currentSpeed;
  }

  // Spec (Fish.md): SwimOnSurface(direction Direction).
  SwimOnSurface(direction: Direction): void {
    this.swim(direction, this.surfaceY);
  }

  // Spec (Fish.md): SwimOnDepth(direction Direction).
  SwimOnDepth(direction: Direction): void {
    this.swim(direction, this.depthY);
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
    const ease = (rate: number) => 1 - Math.exp(-rate * delta);
    this.currentSpeed += (this.targetSpeed - this.currentSpeed) * ease(ACCELERATION);
    const effort = Math.max(IDLE, this.currentSpeed / SWIM_SPEED); // IDLE hovering .. 1 at full speed

    // Turn toward the heading the short way round, more slowly when slow.
    let wantedTurn = 0;
    if (this.heading !== null) {
      const off = this.heading - this.rotation.y;
      const shortest = Math.atan2(Math.sin(off), Math.cos(off));
      wantedTurn = THREE.MathUtils.clamp(shortest * TURN_EASE, -TURN_RATE, TURN_RATE) * (0.4 + 0.6 * effort);
    }
    this.turnRate += (wantedTurn - this.turnRate) * ease(TURN_RESPONSE);
    this.rotation.y += this.turnRate * delta;

    // Forward along its own +z.
    this.position.x += Math.sin(this.rotation.y) * this.currentSpeed * delta;
    this.position.z += Math.cos(this.rotation.y) * this.currentSpeed * delta;

    // Rise or dive toward its depth.
    let wantedClimb = 0;
    if (this.targetY !== null) {
      wantedClimb = THREE.MathUtils.clamp((this.targetY - this.position.y) * DIVE_EASE, -DIVE_SPEED, DIVE_SPEED);
    }
    this.climb += (wantedClimb - this.climb) * ease(DIVE_RESPONSE);
    this.position.y += this.climb * delta;

    // Nose up while rising and down while diving (rotation.x > 0 dips the
    // nose), leaning into turns.
    const rig = this.rig;
    const pitch = Math.atan2(-this.climb, Math.max(this.currentSpeed, 0.1));
    rig.rotation.x = THREE.MathUtils.clamp(pitch, -PITCH_LIMIT, PITCH_LIMIT);
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
    const fan = FIN_STROKE * (1 - 0.5 * (this.currentSpeed / SWIM_SPEED)) * (0.5 + 0.5 * Math.sin(this.finPhase));
    this.leftFin.rotation.y = fan;
    this.rightFin.rotation.y = -fan;
  }
}
