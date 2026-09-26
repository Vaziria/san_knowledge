import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Animal, type AnimalShape, type Gait, type Motion } from './Animal';
import { Body, type NeckShape, type TorsoShape } from './Body';
import { Head, headSize, type Extra, type HeadModel, type HeadShape } from './Head';
import { Leg, legBones, LOWER_RADIUS, REACH_SHARE, type LegModel, type LegShape } from './Leg';
import { createMaterials, lofted, paintLoft, palette, scaleLoft, twoTone, type AnimalOptions, type Loft, type Look, type Paint, type Palette } from './parts';
import { Tail, type TailModel, type TailShape } from './Tail';

// A four-legged animal (the cat, fox, wolf, deer, bear and lion), built from
// a Build: the numbers of its body, head, legs and tail, its colours and its
// gaits. Its parts are a body (torso and neck), a head, a tail and four legs,
// each a group with its origin at its joint, exposed for animation. Units are
// meters, it faces +z, and the origin is on the ground under the middle of
// its torso.
//
// Or it is modelled by hand, part by part, from a ModelBuild (every one of
// them now, each from the user's low poly model): a lofted body and neck in
// one, lofted legs and tail and a head of flat faces, each where the model
// puts it and painted as the model paints it. Its legs bend at two of their
// rings (Leg.ts), and standing still it stands as the model does, its feet
// where the model has them and its legs as straight. The models' front legs
// are all but straight, so to walk or run it first lowers its body to where
// the other animals stand (REST_REACH), and then as much further as it must
// for every leg to reach its foot, so its planted feet reach the ground at
// both ends of every stride.
//
// It walks with the four-beat walk of dogs and cats (left hind, left fore,
// right hind, right fore, a quarter of a stride apart) and runs at a gallop
// (the hind feet close together, then the fore), rocking its body. Each foot
// is placed on the ground and its leg bent to reach it (Leg.reach), so a
// planted foot stays put while the body moves on; a swinging foot rises no
// higher than lets its leg fold without folding shut. The head steadies against
// the body's rocking, nods as it walks and lifts to carry what it holds, and
// the tail swings. A jump crouches, draws the legs up in the air, tips the
// nose up leaving and down landing, and bends to take the landing.
//
// An animal with an `upright` build (the bear) stands up on its hind legs
// when it stands still, and drops onto all fours to walk or run: everything
// but the hind legs tips up about the hips, the hind feet stay where they
// are, the front paws leave the ground and hang from the shoulders, and the
// head tips back down to look ahead. Each of its legs is turned toward the
// way its middle joint points (Leg.reach with a pole), which holds with the
// body upright.

export type Coat = {
  coat: THREE.Color; // back and sides
  belly: THREE.Color; // underneath, the throat and chest
  legs: THREE.Color; // the lower legs and the feet
  muzzle: THREE.Color; // under the snout, the inside of the ears
  tail: THREE.Color; // the tail's tip
  horn: THREE.Color; // antlers
};

// A modelled animal's colours: the coat's, and each colour its model names
// (a fox's 'orange', 'white' and 'black'), which its parts are painted in.
// An 'eye' colour colours its glossy eyes.
export type Colors = Coat & { [name: string]: THREE.Color };

export type Joint = LegShape & { x: number; y: number; z: number }; // the leg's joint, from the torso's middle

// Standing up on the hind legs (the bear).
export interface Upright {
  pitch: number; // radians the body tips up about the hips
  // Where a front paw hangs, from its shoulder: down, forward and out, as
  // shares of the front leg's length; and how far it tips its toes down, in
  // radians.
  hang: { down: number; forward: number; out: number; tip: number };
  look: number; // radians the head tips down from level
}

export interface Build {
  name: string;
  torso: TorsoShape;
  neck: Omit<NeckShape, 'base'> & { at?: { y: number; z: number } }; // at: where it leaves the chest, as shares of the torso's height and length (NECK_BASE when left out)
  head: HeadShape;
  tail: TailShape;
  front: Joint;
  hind: Joint;
  colors: (p: Palette) => Coat;
  walk?: Gait; // left out, worked out from its legs' length (gaits())
  run?: Gait;
  jump: number;
  hold: number;
  upright?: Upright; // stands up on its hind legs when it stands still
  look?: Look; // smooth when left out
}

type Point = readonly [x: number, y: number, z: number];

// A four-legged animal modelled by hand (the wolf's, wolfModel.ts, and the
// others'), in units of its own, `unit` m each, and in its own axes: x to its
// right, y up, z forward, the origin in the middle of its torso. `at` puts a
// part's origin, its pivot, on it: the tail's root, a leg's joint (the right
// leg's; the left one is the same, at -x). The head's points are in units
// `scale` times the animal's, from `at`; it turns about the body's end, where
// the neck ends. A lofted part's `paint` names each face's colour as the
// user's model does (its colorFor), in the part's own units; without it the
// part is shaded from the coat (the wolf).
export interface Model<Name extends string = string> {
  unit: number;
  body: Painted<Loft>; // the torso and neck in one, from the rump to where the head joins it (the middle of its last section)
  head: Omit<HeadModel<Name>, 'unit' | 'origin'> & { at: Point; scale: number; extras?: readonly Extra[] };
  tail: Painted<TailModel> & { at: Point };
  front: Painted<LegModel> & { at: Point };
  hind: Painted<LegModel> & { at: Point };
}

type Painted<T extends Loft> = T & { paint?: Paint<string> };

// A four-legged animal from a model: the model, and its colours and gaits as
// in a Build.
export interface ModelBuild {
  name: string;
  model: Model;
  colors: (p: Palette) => Colors;
  walk?: Gait;
  run?: Gait;
  jump: number;
  hold: number;
}

const REST_REACH = 0.95; // a standing leg's reach, as a share of its full length
const NECK_BASE = { y: 0.2, z: 0.36 }; // where the neck leaves the chest, as shares of the torso's height and length

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
const STILL = 0.02; // below this much of its gait it counts as standing still, and an upright animal stands up
const REAR_SPEED = 2.5; // how fast it stands up and drops onto all fours
const HANG = { from: 0.05, to: 0.55 }; // how far up it is when its front paws start to leave the ground, and when they hang
const FOLD = 1.15; // a swinging foot comes no nearer its leg's joint than this many times the nearest the leg can reach (fold())

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
  const r = leg.radius * (leg.lowerRadius ?? LOWER_RADIUS);
  return leg.foot === 'hoof' ? 0.2 * r : 0.35 * r;
}

function length(leg: LegShape): number {
  return leg.upper + leg.lower;
}

// Where a leg is on the animal and how it stands, from either kind of build.
interface Stance {
  x: number; // m, its joint, from the torso's middle (the right leg's; the left's is at -x)
  y: number;
  z: number;
  foot: number; // m its foot stands ahead of the joint at rest
  ankle: number; // m, the ankle's height above the ground at rest
  length: number; // m, from the joint to the ankle, the leg straight
  stand: number; // how far the joint is above the ankle at rest, as a share of `length` (the front leg's sets how high the body stands)
  bend: 1 | -1;
}

function stance(build: Build | ModelBuild, front: boolean): Stance {
  if (!('model' in build)) {
    const joint = front ? build.front : build.hind;
    return { x: joint.x, y: joint.y, z: joint.z, foot: 0, ankle: ankle(joint), length: length(joint), stand: REST_REACH, bend: joint.bend };
  }
  const { unit } = build.model;
  const leg = front ? build.model.front : build.model.hind;
  const bones = legBones(scaleLoft(leg, unit));
  const reach = bones.upper + bones.lower;
  const [x, y, z] = leg.at;
  return { x: x * unit, y: y * unit, z: z * unit, foot: bones.ankle.z, ankle: bones.height, length: reach, stand: -bones.ankle.y / reach, bend: leg.bend };
}

// A model's head as a HeadModel, in meters, turning about the body's end.
function headOf(model: Model): HeadModel {
  const { at, scale, ...head } = model.head;
  const [y, z] = model.body.sections[model.body.sections.length - 1];
  return { ...head, unit: model.unit * scale, origin: [-at[0] / scale, (y - at[1]) / scale, (z - at[2]) / scale] };
}

// Where the animal's legs, body and head are at rest, and so where its
// speech bubble goes.
function layout(build: Build | ModelBuild) {
  const front = stance(build, true);
  const hind = stance(build, false);
  const torsoY = front.ankle + front.stand * front.length - front.y;
  let neckBase = new THREE.Vector3(); // where the neck leaves the chest (a rounded build's)
  let head: THREE.Vector3; // where the head joins the neck, from the torso's middle
  let size: { top: number; length: number };
  if ('model' in build) {
    const [y, z] = build.model.body.sections[build.model.body.sections.length - 1];
    head = new THREE.Vector3(0, y, z).multiplyScalar(build.model.unit);
    size = headSize(headOf(build.model));
  } else {
    const at = build.neck.at ?? NECK_BASE;
    neckBase = new THREE.Vector3(0, build.torso.height * at.y, build.torso.length * at.z);
    const way = new THREE.Vector3(0, Math.sin(build.neck.rise), Math.cos(build.neck.rise));
    head = neckBase.clone().addScaledVector(way, build.neck.length);
    size = headSize(build.head);
  }
  head.add(new THREE.Vector3(0, torsoY, 0));
  return { front, hind, torsoY, neckBase, bubble: new THREE.Vector3(0, head.y + size.top + 0.04, head.z + size.length * 0.35) };
}

type Layout = ReturnType<typeof layout>;

function shapeOf(build: Build | ModelBuild, place: Layout): AnimalShape {
  const own = gaits(place.front.length);
  return { walk: build.walk ?? own.walk, run: build.run ?? own.run, jump: build.jump, hold: build.hold, bubble: place.bubble };
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
  // feet stay on the ground. Within it, everything but the hind legs tips up
  // about the hips (`hips`, at the hind legs' joints) when an upright
  // animal stands up; `front` puts it back where it was.
  private readonly torso = new THREE.Group();
  private readonly hips = new THREE.Group();
  private readonly front = new THREE.Group();
  private readonly legs: Leg[];
  private readonly torsoY: number;
  private readonly stances: { front: Stance; hind: Stance };
  private readonly slack: number; // m it lowers its body to walk or run, beyond the other animals: a model's legs stand straighter
  private readonly level: boolean; // keeps its feet level with the ground as its body rocks (a model's)
  private readonly upright?: Upright;
  private readonly headRest: THREE.Vector3; // where the head is on all fours, in the rig
  private time = 0;
  private rear: number; // 0 on all fours, 1 up on its hind legs; eased
  private readonly inverse = new THREE.Matrix4();
  private readonly frontMatrix = new THREE.Matrix4();
  private readonly frontInverse = new THREE.Matrix4();
  private readonly target = new THREE.Vector3();
  private readonly hang = new THREE.Vector3();
  private readonly pole = new THREE.Vector3();
  private readonly joint = new THREE.Vector3();
  private readonly down = new THREE.Vector3();
  private readonly targets = LEGS.map(() => new THREE.Vector3()); // each foot's, in the rig, this frame
  private readonly tips = LEGS.map(() => 0);
  private readonly swinging = LEGS.map(() => false);

  protected constructor(options: AnimalOptions, build: Build | ModelBuild) {
    const theme = options.theme ?? defaultTheme;
    const place = layout(build);
    super(theme, shapeOf(build, place));
    this.name = build.name;
    this.upright = 'model' in build ? undefined : build.upright;
    this.rear = this.upright ? 1 : 0;
    const colors: Colors = build.colors(palette(theme));
    const m = createMaterials(theme, 'model' in build ? undefined : build.look, colors.eye);
    const { front, hind, torsoY, neckBase } = place;
    this.torsoY = torsoY;
    this.stances = { front, hind };
    this.slack = (front.stand - REST_REACH) * front.length;
    this.level = 'model' in build;

    // A modelled part's faces in the colours its model paints them.
    const painted = (part: Painted<Loft>) =>
      part.paint &&
      paintLoft(lofted(part), part.paint).map((name) => {
        const color = colors[name];
        if (!color) throw new Error(`the ${build.name}'s model paints a face "${name}", a colour it hasn't`);
        return color;
      });

    if ('model' in build) {
      const { model } = build;
      this.body = new Body(m, scaleLoft(model.body, model.unit), colors.coat, colors.belly, painted(model.body));
      this.head = new Head(m, headOf(model), colors);
      this.tail = new Tail(m, scaleLoft(model.tail, model.unit), colors.coat, colors.tail, painted(model.tail));
      this.tail.position.fromArray(model.tail.at).multiplyScalar(model.unit);
    } else {
      this.body = new Body(m, { torso: build.torso, neck: { ...build.neck, base: neckBase } }, colors.coat, colors.belly);
      this.head = new Head(m, build.head, colors);
      this.tail = new Tail(m, build.tail, colors.coat, colors.tail);
      this.tail.position.set(0, build.torso.height * 0.28, -build.torso.length * 0.46);
    }
    this.head.position.copy(this.body.neckEnd);
    this.head.mouth.add(this.holder); // a held figure goes in its mouth

    // Legs: the coat down to the knee, the leg colour below.
    const c = new THREE.Color();
    const shade = twoTone(colors.coat, colors.belly, -0.6);
    const legShade = (u: number, out: THREE.Vector3) => c.copy(shade(u, out)).lerp(colors.legs, THREE.MathUtils.smoothstep(u, 0.38, 0.55));
    const make = (isFront: boolean, side: -1 | 1) => {
      const s = isFront ? front : hind;
      let leg: Leg;
      if ('model' in build) {
        const model = isFront ? build.model.front : build.model.hind;
        leg = new Leg(m, scaleLoft(model, build.model.unit), legShade, painted(model));
      } else {
        leg = new Leg(m, isFront ? build.front : build.hind, legShade);
      }
      leg.position.set(side * s.x, s.y, s.z);
      return leg;
    };
    this.leftHindLeg = make(false, -1);
    this.leftFrontLeg = make(true, -1);
    this.rightHindLeg = make(false, 1);
    this.rightFrontLeg = make(true, 1);
    this.legs = [this.leftHindLeg, this.leftFrontLeg, this.rightHindLeg, this.rightFrontLeg];

    this.torso.position.y = torsoY;
    this.hips.position.set(0, hind.y, hind.z);
    this.front.position.copy(this.hips.position).negate();
    this.front.updateMatrix();
    this.front.add(this.body, this.head, this.tail, this.leftFrontLeg, this.rightFrontLeg);
    this.hips.add(this.front);
    this.torso.add(this.hips, this.leftHindLeg, this.rightHindLeg);
    this.rig.add(this.torso);
    this.headRest = this.head.position.clone().add(this.torso.position);
    this.update(0);
  }

  protected pose(mo: Motion): void {
    this.time += mo.delta;
    const legLength = this.stances.front.length;
    const walking = mo.moving * (1 - mo.running);
    const galloping = mo.moving * mo.running;
    const turn = 2 * Math.PI * mo.phase;

    // Standing up on its hind legs while it stands still, dropping onto all
    // fours as soon as it moves.
    const up = this.upright;
    if (up) this.rear += ((mo.moving < STILL ? 1 : 0) - this.rear) * (1 - Math.exp(-REAR_SPEED * mo.delta));
    const rear = THREE.MathUtils.smootherstep(this.rear, 0, 1);
    const pitch = (up?.pitch ?? 0) * rear;
    this.hips.rotation.x = -pitch;
    this.hips.updateMatrix();

    // The torso: bobbing, rolling as it walks, rocking at a gallop, sinking
    // to crouch, tipping in a jump; a model sinks first to where the others
    // stand.
    const bob = legLength * (BOB.walk * walking * -Math.cos(2 * turn) + BOB.run * galloping * Math.cos(turn));
    const rock = ROCK * galloping * Math.sin(turn) + JUMP_PITCH * mo.air;
    this.rig.position.y = mo.height;
    const sink = legLength * (LOWER.walk * walking + LOWER.run * galloping + CROUCH * mo.crouch) + this.slack * mo.moving;
    this.torso.position.y = this.torsoY + bob - sink;
    this.torso.rotation.set(rock, 0, SWAY * walking * Math.sin(turn));
    this.torso.updateMatrix();

    // Where each foot goes: planted for its share of the stride, sliding back
    // under the body, then lifted and swung forward; drawn up in the air.
    const lift = legLength * THREE.MathUtils.lerp(LIFT.walk, LIFT.run, mo.running) * mo.moving;
    LEGS.forEach(([side, front], k) => {
      const joint = front ? this.stances.front : this.stances.hind;
      const offset = THREE.MathUtils.lerp(WALK_OFFSETS[k], RUN_OFFSETS[k], mo.running);
      const p = (mo.phase + offset) % 1;
      let z = mo.stride / 2 - mo.stride * (p / mo.duty);
      let y = 0;
      let tip = 0;
      this.swinging[k] = p >= mo.duty;
      if (this.swinging[k]) {
        const s = (p - mo.duty) / (1 - mo.duty);
        z = -mo.stride / 2 + mo.stride * THREE.MathUtils.smoothstep(s, 0, 1);
        y = lift * Math.sin(Math.PI * s);
        tip = PEEL * mo.moving * Math.sin(Math.PI * Math.min(1, s * 1.6));
        // It rolls over its toes rather than pushing them into the ground.
        y = Math.max(y, this.legs[k].drop(tip) - this.legs[k].drop(0));
      }
      y += TUCK * joint.length * mo.tuck;
      this.targets[k].set(side * joint.x, joint.ankle + y, joint.z + joint.foot + z);
      this.tips[k] = tip;
    });

    // The body sinks as far as it must for every leg to reach its foot. A
    // model's legs stand all but straight, so at either end of a stride a
    // planted foot was out of reach, and landed late or lifted early (up to
    // 28 mm galloping, for the lion). The foot's targets move smoothly,
    // swinging and planted, so the sink does too; a leg that reaches its foot
    // anyway (the wolf's at a gallop) leaves the body where it was.
    if (!up) {
      let reach = 0;
      LEGS.forEach(([, front], k) => {
        const stance = front ? this.stances.front : this.stances.hind;
        const joint = this.joint.copy(this.legs[k].position).applyMatrix4(this.torso.matrix);
        const target = this.targets[k];
        const far = REACH_SHARE * stance.length;
        const across = (joint.x - target.x) ** 2 + (joint.z - target.z) ** 2;
        reach = Math.max(reach, joint.y - target.y - Math.sqrt(Math.max(0, far * far - across)));
      });
      if (reach > 0) {
        this.torso.position.y -= reach;
        this.torso.updateMatrix();
      }
    }
    this.inverse.copy(this.torso.matrix).invert();
    this.frontMatrix.multiplyMatrices(this.torso.matrix, this.hips.matrix).multiply(this.front.matrix);
    this.frontInverse.copy(this.frontMatrix).invert();

    // The legs reach for their feet. Up on its hind legs, the front paws hang
    // from the shoulders instead. A foot is level with the body, but a
    // model's is kept level with the ground as the body rocks: the wolf's long
    // paws went 7 mm into the ground at a gallop, the other animals' 1 to 2.
    const lean = this.level ? rock : 0;
    LEGS.forEach(([side, front], k) => {
      const joint = front ? this.stances.front : this.stances.hind;
      const leg = this.legs[k];
      this.target.copy(this.targets[k]);
      let tip = this.tips[k];
      if (!up) {
        this.target.applyMatrix4(this.inverse).sub(leg.position);
        if (this.swinging[k]) this.fold(leg, this.target);
        leg.reach(this.target, tip - lean);
        return;
      }
      if (front) {
        const hanging = THREE.MathUtils.smoothstep(rear, HANG.from, HANG.to);
        const reach = joint.length;
        this.hang.copy(leg.position).applyMatrix4(this.frontMatrix);
        this.hang.add(this.pole.set(side * up.hang.out * reach, -up.hang.down * reach, up.hang.forward * reach));
        this.target.lerp(this.hang, hanging);
        tip = THREE.MathUtils.lerp(tip, up.hang.tip, hanging) + pitch; // level in the rig, not in the tipped body
      }
      const inverse = front ? this.frontInverse : this.inverse;
      this.pole.set(0, 0, joint.bend).transformDirection(inverse);
      leg.reach(this.target.applyMatrix4(inverse).sub(leg.position), tip, this.pole);
    });

    // The head keeps looking ahead, nods walking, stretches out galloping and
    // lifts to carry what it holds.
    this.head.rotation.x = -STEADY * rock + NOD * walking * Math.sin(2 * turn) + RUN_HEAD * galloping - HOLD_LIFT * mo.holding;
    // Up on its hind legs, it tips its head back down to look ahead, and its
    // speech bubble follows the head up.
    if (up) {
      this.head.rotation.x += pitch + up.look * rear;
      this.bubbleShift.copy(this.head.position).applyMatrix4(this.frontMatrix).sub(this.headRest);
    }

    // The tail swings with the stride, rises at a gallop, and swishes slowly
    // standing still.
    this.tail.rotation.y = WAG * mo.moving * Math.sin(turn) + IDLE.sway * (1 - mo.moving) * Math.sin(IDLE.rate * this.time);
    this.tail.rotation.x = TAIL_RUN * galloping;
  }

  // Keeps a swinging foot (its target, from the leg's joint in the torso's
  // axes) from coming so near the joint that the leg folds shut, its lower
  // segment turned back up along the upper: it is lowered, straight down in
  // the rig, to FOLD times the nearest the leg can reach. The cat's and the
  // lion's models have hind legs long above the hock and short below it, and
  // a galloping foot lifted as high as the others' folded them flat, the hock
  // 2 to 3 mm into the ground. The other animals' legs never come that near.
  private fold(leg: Leg, target: THREE.Vector3): void {
    const nearest = FOLD * leg.shortest;
    const near = target.lengthSq();
    if (near >= nearest * nearest) return;
    const along = target.dot(this.down.set(0, -1, 0).transformDirection(this.inverse));
    target.addScaledVector(this.down, -along + Math.sqrt(along * along + nearest * nearest - near));
  }
}
