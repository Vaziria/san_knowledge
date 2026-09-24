import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Animal, type AnimalShape, type Gait, type Motion } from './Animal';
import { Body, type TorsoShape } from './Body';
import { Head, type HeadShape } from './Head';
import { Leg, type LegShape } from './Leg';
import { createMaterials, palette, twoTone, type AnimalOptions, type Palette } from './parts';
import { Tail, type TailShape } from './Tail';

// A four-legged animal (the cat, fox, wolf, deer and bear), built from a
// Build: the numbers of its body, head, legs and tail, its colours and its
// gaits. Its parts are a body (torso and neck), a head, a tail and four legs,
// each a group with its origin at its joint, exposed for animation. Units are
// meters, it faces +z, and the origin is on the ground under the middle of
// its torso.
//
// It walks with the four-beat walk of dogs and cats (left hind, left fore,
// right hind, right fore, a quarter of a stride apart) and runs at a gallop
// (the hind feet close together, then the fore), rocking its body. Each foot
// is placed on the ground and its leg bent to reach it (Leg.reach), so a
// planted foot stays put while the body moves on. The head steadies against
// the body's rocking, nods as it walks and lifts to carry what it holds, and
// the tail swings. A jump crouches, draws the legs up in the air, tips the
// nose up leaving and down landing, and bends to take the landing.

export interface Coat {
  coat: THREE.Color; // back and sides
  belly: THREE.Color; // underneath, the throat and chest
  legs: THREE.Color; // the lower legs and the feet
  muzzle: THREE.Color; // under the snout, the inside of the ears
  tail: THREE.Color; // the tail's tip
  horn: THREE.Color; // antlers
}

export type Joint = LegShape & { x: number; y: number; z: number }; // the leg's joint, from the torso's middle

export interface Build {
  name: string;
  torso: TorsoShape;
  neck: { length: number; rise: number; radius: number };
  head: HeadShape;
  tail: TailShape;
  front: Joint;
  hind: Joint;
  colors: (p: Palette) => Coat;
  walk?: Gait; // left out, worked out from its legs' length (gaits())
  run?: Gait;
  jump: number;
  hold: number;
}

const REST_REACH = 0.95; // a standing leg's reach, as a share of its full length
const NECK_BASE = { y: 0.2, z: 0.36 }; // where the neck leaves the chest, as shares of the torso's height and length
const LOWER_RADIUS = 0.55; // as in Leg.ts

// Legs in the order they are posed: [side, front?].
const LEGS = [
  [-1, false],
  [-1, true],
  [1, false],
  [1, true],
] as const;
// Where in the stride each leg sets down, in the same order.
const WALK_OFFSETS = [0, 0.25, 0.5, 0.75]; // left hind, left fore, right hind, right fore
const RUN_OFFSETS = [0, 0.6, 0.1, 0.5]; // hind feet close together, then the fore
const LIFT = { walk: 0.14, run: 0.26 }; // a stepping foot rises this share of the leg's length
const PEEL = 0.7; // radians a paw tips as it leaves the ground
const BOB = { walk: 0.015, run: 0.05 }; // shares of the leg's length the body rises and falls
const LOWER = { walk: 0.03, run: 0.16 }; // shares of the leg's length the body sinks into each gait, so a foot can reach the ground at both ends of its stride
const ROCK = 0.1; // radians the body rocks nose up and down at a gallop
const SWAY = 0.03; // radians it rolls from side to side walking
const NOD = 0.08; // radians the head nods walking
const STEADY = 0.7; // share of the body's rocking the head undoes, to keep looking ahead
const RUN_HEAD = 0.15; // radians the head lowers and stretches out at a gallop
const HOLD_LIFT = 0.25; // radians the head lifts to carry what it holds
const CROUCH = 0.22; // share of the leg's length the body sinks crouching
const TUCK = 0.3; // share of the leg's length the feet draw up in the air
const JUMP_PITCH = 0.25; // radians the nose tips up leaving and down landing
const WAG = 0.25; // radians the tail swings with the stride
const TAIL_RUN = 0.35; // radians the tail rises at a gallop
const IDLE = { sway: 0.12, rate: 0.8 }; // a slow swish of the tail standing still

// Gaits worked out from the length of the legs, so a bigger animal takes
// longer, slower strides: walking about 0.8 sqrt(length) m/s, galloping about
// 5 sqrt(length). A galloping foot is down for less than a third of the
// stride and the animal flies through the rest, so it goes fast on a stride
// its legs can reach: a longer one put the planted foot out of reach at
// either end, where it lifted and slid.
export function gaits(leg: number): { walk: Gait; run: Gait } {
  return {
    walk: { stride: 0.65 * leg, cadence: 0.8 / Math.sqrt(leg), duty: 0.65 },
    run: { stride: 1 * leg, cadence: 1.5 / Math.sqrt(leg), duty: 0.3 },
  };
}

// The height of the ankle above the ground, so the foot sits on it.
function ankle(leg: LegShape): number {
  const r = leg.radius * LOWER_RADIUS;
  return leg.foot === 'hoof' ? 0.2 * r : 0.35 * r;
}

function length(leg: LegShape): number {
  return leg.upper + leg.lower;
}

// Where the animal's body and head are at rest, and so where its speech
// bubble goes.
function layout(build: Build) {
  const torsoY = ankle(build.front) + REST_REACH * length(build.front) - build.front.y;
  const neckBase = new THREE.Vector3(0, build.torso.height * NECK_BASE.y, build.torso.length * NECK_BASE.z);
  const way = new THREE.Vector3(0, Math.sin(build.neck.rise), Math.cos(build.neck.rise));
  const head = neckBase.clone().addScaledVector(way, build.neck.length).add(new THREE.Vector3(0, torsoY, 0));
  const top = Math.max(build.head.height * 0.6 + build.head.ears.height, build.head.antlers?.height ?? 0);
  return { torsoY, neckBase, bubble: new THREE.Vector3(0, head.y + top + 0.04, head.z + build.head.length * 0.35) };
}

function shapeOf(build: Build): AnimalShape {
  const own = gaits(length(build.front));
  return { walk: build.walk ?? own.walk, run: build.run ?? own.run, jump: build.jump, hold: build.hold, bubble: layout(build).bubble };
}

export class Quadruped extends Animal {
  readonly body: Body;
  readonly head: Head;
  readonly tail: Tail;
  readonly leftFrontLeg: Leg;
  readonly rightFrontLeg: Leg;
  readonly leftHindLeg: Leg;
  readonly rightHindLeg: Leg;

  // The torso, everything above the legs' joints, bobs and rocks while the
  // feet stay on the ground.
  private readonly torso = new THREE.Group();
  private readonly legs: Leg[];
  private readonly torsoY: number;
  private readonly build: Build;
  private time = 0;
  private readonly inverse = new THREE.Matrix4();
  private readonly target = new THREE.Vector3();

  protected constructor(options: AnimalOptions, build: Build) {
    const theme = options.theme ?? defaultTheme;
    super(theme, shapeOf(build));
    this.name = build.name;
    this.build = build;
    const m = createMaterials(theme);
    const colors = build.colors(palette(theme));
    const { torsoY, neckBase } = layout(build);
    this.torsoY = torsoY;

    this.body = new Body(m, build.torso, { ...build.neck, base: neckBase }, colors.coat, colors.belly);
    this.head = new Head(m, build.head, colors);
    this.head.position.copy(this.body.neckEnd);
    this.head.mouth.add(this.holder); // a held figure goes in its mouth

    this.tail = new Tail(m, build.tail, colors.coat, colors.tail);
    this.tail.position.set(0, build.torso.height * 0.28, -build.torso.length * 0.46);

    // Legs: the coat down to the knee, the leg colour below.
    const c = new THREE.Color();
    const shade = twoTone(colors.coat, colors.belly, -0.6);
    const legShade = (u: number, out: THREE.Vector3) => c.copy(shade(u, out)).lerp(colors.legs, THREE.MathUtils.smoothstep(u, 0.38, 0.55));
    const make = (joint: Joint, side: -1 | 1) => {
      const leg = new Leg(m, joint, legShade);
      leg.position.set(side * joint.x, joint.y, joint.z);
      return leg;
    };
    this.leftHindLeg = make(build.hind, -1);
    this.leftFrontLeg = make(build.front, -1);
    this.rightHindLeg = make(build.hind, 1);
    this.rightFrontLeg = make(build.front, 1);
    this.legs = [this.leftHindLeg, this.leftFrontLeg, this.rightHindLeg, this.rightFrontLeg];

    this.torso.position.y = torsoY;
    this.torso.add(this.body, this.head, this.tail, ...this.legs);
    this.rig.add(this.torso);
    this.update(0);
  }

  protected pose(mo: Motion): void {
    this.time += mo.delta;
    const build = this.build;
    const legLength = length(build.front);
    const walking = mo.moving * (1 - mo.running);
    const galloping = mo.moving * mo.running;
    const turn = 2 * Math.PI * mo.phase;

    // The torso: bobbing, rolling as it walks, rocking at a gallop, sinking
    // to crouch, tipping in a jump.
    const bob = legLength * (BOB.walk * walking * -Math.cos(2 * turn) + BOB.run * galloping * Math.cos(turn));
    const rock = ROCK * galloping * Math.sin(turn) + JUMP_PITCH * mo.air;
    this.rig.position.y = mo.height;
    const sink = legLength * (LOWER.walk * walking + LOWER.run * galloping + CROUCH * mo.crouch);
    this.torso.position.y = this.torsoY + bob - sink;
    this.torso.rotation.set(rock, 0, SWAY * walking * Math.sin(turn));
    this.torso.updateMatrix();
    this.inverse.copy(this.torso.matrix).invert();

    // The legs: each foot planted for its share of the stride, sliding back
    // under the body, then lifted and swung forward; drawn up in the air.
    const lift = legLength * THREE.MathUtils.lerp(LIFT.walk, LIFT.run, mo.running) * mo.moving;
    LEGS.forEach(([side, front], k) => {
      const joint = front ? build.front : build.hind;
      const offset = THREE.MathUtils.lerp(WALK_OFFSETS[k], RUN_OFFSETS[k], mo.running);
      const p = (mo.phase + offset) % 1;
      let z = mo.stride / 2 - mo.stride * (p / mo.duty);
      let y = 0;
      let tip = 0;
      if (p >= mo.duty) {
        const s = (p - mo.duty) / (1 - mo.duty);
        z = -mo.stride / 2 + mo.stride * THREE.MathUtils.smoothstep(s, 0, 1);
        y = lift * Math.sin(Math.PI * s);
        tip = PEEL * mo.moving * Math.sin(Math.PI * Math.min(1, s * 1.6));
      }
      y += TUCK * length(joint) * mo.tuck;
      this.target.set(side * joint.x, ankle(joint) + y, joint.z + z).applyMatrix4(this.inverse).sub(this.legs[k].position);
      this.legs[k].reach(this.target, tip);
    });

    // The head keeps looking ahead, nods walking, stretches out galloping and
    // lifts to carry what it holds.
    this.head.rotation.x = -STEADY * rock + NOD * walking * Math.sin(2 * turn) + RUN_HEAD * galloping - HOLD_LIFT * mo.holding;

    // The tail swings with the stride, rises at a gallop, and swishes slowly
    // standing still.
    this.tail.rotation.y = WAG * mo.moving * Math.sin(turn) + IDLE.sway * (1 - mo.moving) * Math.sin(IDLE.rate * this.time);
    this.tail.rotation.x = TAIL_RUN * galloping;
  }
}
