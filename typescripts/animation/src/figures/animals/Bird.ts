import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Animal, type AnimalShape, type Gait, type Motion } from './Animal';
import { BIRD_MODEL, bodyColor, type Point, type Role } from './birdModel';
import { Leg, legBones, type LegModel } from './Leg';
import { createMaterials, lofted, mesh, mix, paintLoft, palette, polygons, scaleLoft, type AnimalOptions, type Palette } from './parts';
import { sheets, Wing } from './Wing';

// A songbird, low poly like folded paper, in the shape the user modelled
// (birdModel.ts, from Bird.md's "Shape Reference"): 23 cm from the tip of its
// beak to the tips of its tail feathers and 14 cm to the top of its head. A
// round body and head in one, carried head-up, a short pointed beak, small
// eyes, flat wings folded back along its sides, a flat fan of tail feathers,
// and short legs with three toes forward and one back. Every face is flat and
// of one colour: a blue head, back and wings (the theme's sky blue), dark
// blue flight feathers and tail, an orange chest and throat, a light belly, a
// dark beak, dark glossy eyes and brown-grey legs.
//
// Units are meters, it faces +z, and the origin is on the ground under its
// hips, the middle between its two legs' joints. Its parts are the body (the
// body's half of the model's loft), the head (the other half, with the beak
// and eyes), the two wings and the tail, and two legs, each a group with its
// origin at its joint: the hips' middle, the neck, a shoulder, the root of
// the tail and a hip. The model's numbers are kept in its own units, and one
// unit (birdModel.ts) makes them meters; standing still it is the model.
//
// Beyond the model: its head and body are one loft there, but the head moves
// here, so the loft is two meshes that share the neck ring, which bends
// halfway between them (Neck below). A wing or the tail is a flat sheet drawn
// from both sides, as two meshes (Wing.ts). Each leg bends at its shank's
// middle ring and its foot (Leg.ts), with the toes on the foot.
//
// Animation behaviour follows its spec, Bird.md: Jump(), Hold(figure),
// Walk(), Run() and Speech(text), plus Stop() (Animal.ts). It walks in quick
// steps, its head jerking forward with each one and holding still between,
// like a pigeon's; it runs leaning forward about its hips with its wings a
// little out; its jump is a hop with the wings spread and beating, in the
// model's poses and flap; and it carries a figure up to 6 cm across in its
// beak. Its legs are short and all but straight at rest, as modelled, so it
// sinks a little to walk and more to run, so that a planted foot reaches the
// ground at both ends of its stride.

const MODEL = BIRD_MODEL;
const UNIT = MODEL.unit;

const WALK: Gait = { stride: 0.025, cadence: 3.3, duty: 0.6 }; // 0.14 m/s
const RUN: Gait = { stride: 0.04, cadence: 5.3, duty: 0.4 }; // 0.53 m/s
const JUMP = 0.12; // m, a hop
const HOLD = 0.06; // m, the largest figure it carries without scaling it down
const BUBBLE_ABOVE = 0.025; // m its speech bubble's tail is above the top of its head

const LIFT = { walk: 0.01, run: 0.014 }; // m a stepping foot rises
const BOB = 0.002; // m the body rises with each step
const REACH = 0.995; // share of its length a planted leg stretches to at most, a little short of straight
const THRUST = 0.25; // share of each step the head spends jerking forward
// m the head goes forward and back from its place with each step: as far as
// the body walks while the head holds still, so it stays put in the world.
const HEAD_JERK = (WALK.stride * (1 - THRUST)) / (4 * WALK.duty);
const RUN_LEAN = 0.35; // radians it leans forward about its hips running
const RUN_SPREAD = 0.15; // how far the wings open running, from folded (0) to spread (1)
const BEATS = 3; // wing beats in a hop
const CROUCH = 0.011; // m the body sinks crouching
const TUCK = 0.35; // share of the leg's length the feet draw up in the air
const TAIL_FLICK = 0.25; // radians the tail flicks up with each step, and up in a hop
const HOLD_LIFT = 0.2; // radians the head lifts to carry what it holds
const HOLD_AT = 0.7; // how far along the beak a held figure is, from its base to its tip
const SINK_STEPS = 16; // points through a stance where the sink is worked out

// Where the model stands and turns, in its units: the ground under its toes
// (the lowest point of a front toe), the middle between its hips (where the
// body leans), the origin on the ground under that, and the middle of the
// neck ring (where the head turns).
const TOE = lofted(MODEL.leg.toe);
const GROUND = MODEL.leg.at[1] + Math.min(...TOE.points.map((p) => p.y));
const HIPS = new THREE.Vector3(0, MODEL.leg.at[1], MODEL.leg.at[2]);
const ORIGIN = new THREE.Vector3(0, GROUND, MODEL.leg.at[2]);
const [NECK_Y, NECK_Z] = MODEL.body.sections[MODEL.neck];
const NECK = new THREE.Vector3(0, NECK_Y, NECK_Z);

// A point of the model in meters, from another (the origin, the hips or the
// neck).
function from(base: THREE.Vector3, p: THREE.Vector3 | Point): THREE.Vector3 {
  return (p instanceof THREE.Vector3 ? p.clone() : new THREE.Vector3(...p)).sub(base).multiplyScalar(UNIT);
}

// The body and head's loft, in the model's units; its speech bubble goes
// above the top of its head.
const BODY = lofted(MODEL.body);
const TOP = BODY.points.reduce((top, p) => (p.y > top.y ? p : top));
const BUBBLE = from(ORIGIN, TOP).setX(0).add(new THREE.Vector3(0, BUBBLE_ABOVE, 0));

// The model's colours through the theme: its blue is the sky's zenith (a
// muted sky-blue) a little darker, its dark blue far darker, its orange the
// trim, its white the light, its beak the dark warmed toward stone, and its
// brown-grey legs stone and fur mixed, darkened. Its black eyes are the eye
// material, dark and glossy.
function colorsOf(p: Palette): Record<Role, THREE.Color> {
  return {
    blue: mix(p.zenith, p.dark, 0.3), // head, back, wings
    darkBlue: mix(p.zenith, p.dark, 0.7), // flight feathers, tail
    orange: p.trim, // chest and throat
    white: p.light, // belly
    beak: mix(p.dark, p.stone, 0.2),
    black: p.dark, // eyes
    leg: mix(mix(p.stone, p.fur, 0.5), p.dark, 0.5),
  };
}

export class Bird extends Animal {
  readonly body = new THREE.Group();
  readonly head = new THREE.Group();
  readonly tail = new THREE.Group();
  readonly leftWing: Wing;
  readonly rightWing: Wing;
  readonly leftLeg: Leg;
  readonly rightLeg: Leg;

  private readonly neck: Neck;
  private readonly hipY: number; // m, the hips' height at rest
  private readonly ankle: THREE.Vector3; // a foot's ankle at rest, from its hip
  private readonly ankleY: number; // m, the ankle's height at rest
  private readonly legLength: number; // m, hip to ankle with the leg straight
  private readonly headRest: THREE.Vector3;
  private readonly inverse = new THREE.Matrix4();
  private readonly target = new THREE.Vector3();

  constructor(options: AnimalOptions = {}) {
    const theme = options.theme ?? defaultTheme;
    const shape: AnimalShape = { walk: WALK, run: RUN, jump: JUMP, hold: HOLD, bubble: BUBBLE };
    super(theme, shape);
    this.name = 'bird';
    const m = createMaterials(theme);
    const colors = colorsOf(palette(theme));
    const sheet = sheets(m);

    // The body and head: the model's loft, the body's bands in the body and
    // the head's with the beak in the head, sharing the neck ring. The body
    // turns about the hips' middle and the head about the neck's.
    this.body.position.copy(from(ORIGIN, HIPS));
    this.hipY = this.body.position.y;
    this.head.position.copy(from(HIPS, NECK));
    this.headRest = this.head.position.clone();
    const beak = lofted(MODEL.beak);
    const beakAt = new THREE.Vector3(...MODEL.beak.at);
    this.neck = new Neck(
      paintLoft(BODY, bodyColor).map((role) => colors[role]),
      beak.faces.map((face) => face.map((i) => from(NECK, beak.points[i].clone().add(beakAt)))),
      colors.beak,
      colors.blue,
      m.coat,
    );
    this.body.add(this.neck.body);
    this.head.add(this.neck.head);

    // The eyes, glossy: the same double-pointed shape each side, turned
    // across the head.
    const eye = lofted(MODEL.eye);
    const eyes = [-1, 1].flatMap((side) => {
      const place = new THREE.Matrix4().makeRotationY(MODEL.eye.turn).setPosition(side * MODEL.eye.at[0], MODEL.eye.at[1], MODEL.eye.at[2]);
      return eye.faces.map((face) => face.map((i) => from(NECK, eye.points[i].clone().applyMatrix4(place))));
    });
    const eyeMesh = mesh(polygons(eyes), m.eye);
    eyeMesh.name = 'eyes';
    this.head.add(eyeMesh);
    // A held figure goes in the beak, most of the way to its tip.
    const [tipY, tipZ] = MODEL.beak.end;
    this.holder.position.copy(from(NECK, new THREE.Vector3(0, tipY, tipZ).multiplyScalar(HOLD_AT).add(beakAt)));
    this.head.add(this.holder);

    // The wings, from the shoulders.
    this.rightWing = new Wing(sheet, MODEL, colors, 1);
    this.rightWing.position.copy(from(HIPS, MODEL.wing.at));
    this.leftWing = new Wing(sheet, MODEL, colors, -1);
    this.leftWing.position.copy(from(HIPS, [-MODEL.wing.at[0], MODEL.wing.at[1], MODEL.wing.at[2]]));

    // The tail feathers, from their root, tilted as the model tilts them
    // (their rest, so the tail's own rotation is 0 then).
    this.tail.position.copy(from(HIPS, MODEL.tail.at));
    const fan = new THREE.Group();
    fan.rotation.x = MODEL.tail.tilt;
    const feathers = MODEL.tail.faces.map((face) => face.map((name) => new THREE.Vector3(...MODEL.tail.points[name]).multiplyScalar(UNIT)));
    fan.add(...sheet(polygons(feathers, () => colors.darkBlue)));
    this.tail.add(fan);

    // The legs: the shank bends at its middle ring (forward, as modelled)
    // and at its foot, where the toes are, fanned about the leg's upright
    // axis as the model fans them, the back one smaller.
    const shank: LegModel = { ...scaleLoft(MODEL.leg.shank, UNIT), middle: 1, ankle: 2, bend: 1 };
    const bones = legBones(shank);
    this.ankle = bones.ankle.clone();
    this.ankleY = this.hipY + this.ankle.y;
    this.legLength = bones.upper + bones.lower;
    const toes = MODEL.leg.toes.flatMap((angle) => {
      const size = angle === Math.PI ? MODEL.leg.back : 1;
      const turn = new THREE.Matrix4().makeRotationY(angle).multiply(new THREE.Matrix4().makeScale(size, size, size));
      return TOE.faces.map((face) => face.map((i) => TOE.points[i].clone().applyMatrix4(turn).multiplyScalar(UNIT).sub(this.ankle)));
    });
    const toeGeometry = polygons(toes, () => colors.leg);
    const leg = (side: -1 | 1) => {
      const made = new Leg(m, shank, () => colors.leg);
      made.position.copy(from(HIPS, [side * MODEL.leg.at[0], MODEL.leg.at[1], MODEL.leg.at[2]]));
      const feet = mesh(toeGeometry, m.coat);
      feet.name = 'toes';
      made.foot.add(feet);
      return made;
    };
    this.leftLeg = leg(-1);
    this.rightLeg = leg(1);

    this.body.add(this.head, this.leftWing, this.rightWing, this.tail, this.leftLeg, this.rightLeg);
    this.rig.add(this.body);
    this.update(0);
  }

  protected pose(mo: Motion): void {
    const walking = mo.moving * (1 - mo.running);
    const running = mo.moving * mo.running;
    const turn = 2 * Math.PI * mo.phase;

    // The body: rising with each step, sunk as far as the gait needs, leaning
    // forward about the hips running, crouching to jump.
    this.rig.position.y = mo.height;
    const lean = RUN_LEAN * running;
    this.body.position.y = this.hipY + BOB * mo.moving * Math.abs(Math.sin(turn)) - this.sink(mo) - CROUCH * mo.crouch;
    this.body.rotation.x = lean;
    this.body.updateMatrix();
    this.inverse.copy(this.body.matrix).invert();

    // Legs: the right foot down for its share of the stride, then the left,
    // each planted under its hip and sliding back as the bird goes on, then
    // lifted and swung forward; drawn up in the air. A foot stays level with
    // the ground as the body leans.
    const lift = THREE.MathUtils.lerp(LIFT.walk, LIFT.run, mo.running) * mo.moving;
    for (const [leg, lag] of [
      [this.rightLeg, 0],
      [this.leftLeg, 0.5],
    ] as const) {
      const p = (mo.phase + lag) % 1;
      let z = mo.stride / 2 - mo.stride * (p / mo.duty);
      let y = 0;
      if (p >= mo.duty) {
        const s = (p - mo.duty) / (1 - mo.duty);
        z = -mo.stride / 2 + mo.stride * THREE.MathUtils.smoothstep(s, 0, 1);
        y = lift * Math.sin(Math.PI * s);
      }
      y += TUCK * this.legLength * mo.tuck;
      this.target.set(leg.position.x, this.ankleY + y, this.ankle.z + z).applyMatrix4(this.inverse).sub(leg.position);
      leg.reach(this.target, -lean);
    }

    // The head jerks forward at the start of each step and holds still (in
    // the world) for the rest, sliding back over the body as it walks on. It
    // stays level as the body leans, and lifts to carry what it holds. The
    // neck ring bends halfway between the body and the head.
    const step = (2 * mo.phase) % 1;
    const jerk = step < THRUST ? -1 + (2 * step) / THRUST : 1 - (2 * (step - THRUST)) / (1 - THRUST);
    this.head.position.z = this.headRest.z + HEAD_JERK * walking * jerk;
    this.head.rotation.x = -lean - HOLD_LIFT * mo.holding;
    this.neck.bend(this.head);

    // Wings: folded, a little open running, and in a hop spread and beating
    // as the model flaps them (its lift, times how far they are spread).
    const spread = Math.max(RUN_SPREAD * running, mo.tuck);
    const flap = mo.tuck > 0 ? MODEL.wing.flap * mo.tuck * Math.sin(2 * Math.PI * BEATS * ((mo.air + 1) / 2)) : 0;
    this.leftWing.pose(spread, flap);
    this.rightWing.pose(spread, flap);

    // The tail flicks up with each step, and up in a hop.
    this.tail.rotation.x = TAIL_FLICK * (walking * Math.max(0, Math.sin(2 * turn)) + mo.tuck);
  }

  // How far the body sinks for the gait it is in, so a planted foot reaches
  // the ground all through its stance: from half a stride ahead of its place
  // at rest to half a stride behind, with the body higher by its bob, the leg
  // stretched to REACH of its length at most.
  private sink(mo: Motion): number {
    const up = this.hipY - this.ankleY; // the hip's height over the ankle at rest
    const reach = REACH * this.legLength;
    let sink = 0;
    for (let k = 0; k <= SINK_STEPS; k++) {
      const p = (k / SINK_STEPS) * mo.duty; // through the stance (the left foot's is half a stride on, with the same bob)
      const z = this.ankle.z + mo.stride / 2 - mo.stride * (p / mo.duty);
      const bob = BOB * mo.moving * Math.abs(Math.sin(2 * Math.PI * p));
      sink = Math.max(sink, up + bob - Math.sqrt(Math.max(0, reach * reach - z * z)));
    }
    return sink;
  }
}

// The body and head are one loft in the model. Here the body's bands are one
// mesh in the body and the head's another in the head, with the beak, and
// they share the neck ring. Each frame the ring goes halfway with the head
// (bend()): half as far as the head has moved from its place at rest, and
// turned half as far about the neck, so the loft bends there without a gap,
// as a modelled leg folds at its joint rings (Leg.ts, Skin). The faces next
// to the ring stretch with it and take new normals. At rest the two meshes
// are the model's loft exactly.
class Neck {
  readonly body: THREE.Mesh; // in the body's axes (from the hips' middle)
  readonly head: THREE.Mesh; // in the head's (from the neck's middle)
  private readonly home = from(HIPS, NECK); // the head's place at rest, in the body's axes
  private readonly rest: THREE.Vector3[] = []; // the ring's corners at rest, from the neck's middle
  private readonly bodySide: Bent;
  private readonly headSide: Bent;

  // `paint`: the colour of each face of the loft; `beak`: the beak's faces,
  // in the head's axes; `lid`: the colour inside, where each half is closed.
  constructor(paint: readonly THREE.Color[], beak: THREE.Vector3[][], beakColor: THREE.Color, lid: THREE.Color, material: THREE.Material) {
    const ring = MODEL.neck;
    const sides = BODY.points.filter((_p, i) => BODY.ring[i] === ring).length;
    const first = BODY.ring.indexOf(ring); // the ring's corners are first, first + 1, ...
    for (let k = 0; k < sides; k++) this.rest.push(from(NECK, BODY.points[first + k]));

    // A mesh of the loft's bands on one side of the ring, and the extra
    // faces, every face a triangle. Each half is closed across the ring by a
    // lid, facing away from it, so it is a closed solid on its own (a part
    // shown alone); in the whole bird the two lids meet inside the neck,
    // where nothing sees them.
    const side = (bands: (band: number) => boolean, base: THREE.Vector3, toHead: boolean, extra: THREE.Vector3[][]): Bent => {
      const faces: THREE.Vector3[][] = [];
      const colors: THREE.Color[] = [];
      const slots: [vertex: number, corner: number][] = [];
      const moving: number[] = [];
      const add = (corners: readonly number[], color: THREE.Color) => {
        const at = faces.length;
        corners.forEach((i, k) => {
          if (i < first || i >= first + sides) return;
          slots.push([3 * at + k, i - first]);
          if (moving[moving.length - 1] !== at) moving.push(at);
        });
        faces.push(corners.map((i) => from(base, BODY.points[i])));
        colors.push(color);
      };
      BODY.faces.forEach((face, f) => {
        if (bands(BODY.band[f])) add(face, paint[f]);
      });
      // The ring fanned from its first corner faces along the loft, toward
      // the head.
      for (let k = 1; k < sides - 1; k++) add(toHead ? [first, first + k, first + k + 1] : [first, first + k + 1, first + k], lid);
      for (const face of extra) {
        faces.push(face);
        colors.push(beakColor);
      }
      return { mesh: mesh(polygons(faces, (_n, f) => colors[f]), material), slots, moving };
    };
    this.bodySide = side((band) => band < ring, HIPS, true, []);
    this.headSide = side((band) => band >= ring, NECK, false, beak);
    this.body = this.bodySide.mesh;
    this.body.name = 'body';
    this.head = this.headSide.mesh;
    this.head.name = 'head';
  }

  // Puts the ring halfway between the body and the head, whose place and
  // turn from its rest are the head group's.
  bend(head: THREE.Object3D): void {
    half.slerpQuaternions(IDENTITY, head.quaternion, 0.5);
    shift.subVectors(head.position, this.home).multiplyScalar(0.5).add(this.home);
    undo.copy(head.quaternion).invert();
    for (let k = 0; k < this.rest.length; k++) {
      placed[k] ??= new THREE.Vector3();
      local[k] ??= new THREE.Vector3();
      placed[k].copy(this.rest[k]).applyQuaternion(half).add(shift); // in the body's axes
      local[k].copy(placed[k]).sub(head.position).applyQuaternion(undo); // in the head's
    }
    move(this.bodySide, placed);
    move(this.headSide, local);
  }
}

// A mesh with corners on the neck ring: which vertices they are, and which
// faces have one.
interface Bent {
  mesh: THREE.Mesh;
  slots: [vertex: number, corner: number][];
  moving: number[];
}

// Reused by Neck.bend().
const IDENTITY = new THREE.Quaternion();
const half = new THREE.Quaternion();
const undo = new THREE.Quaternion();
const shift = new THREE.Vector3();
const placed: THREE.Vector3[] = [];
const local: THREE.Vector3[] = [];
const corner0 = new THREE.Vector3();
const edge1 = new THREE.Vector3();
const edge2 = new THREE.Vector3();

// Moves a mesh's corners on the ring to `corners`, and turns the faces they
// are on to their new slope.
function move(bent: Bent, corners: THREE.Vector3[]): void {
  const geometry = bent.mesh.geometry;
  const position = geometry.getAttribute('position') as THREE.BufferAttribute;
  const normal = geometry.getAttribute('normal') as THREE.BufferAttribute;
  for (const [vertex, corner] of bent.slots) position.setXYZ(vertex, corners[corner].x, corners[corner].y, corners[corner].z);
  for (const f of bent.moving) {
    corner0.fromBufferAttribute(position, 3 * f);
    edge1.fromBufferAttribute(position, 3 * f + 1).sub(corner0);
    edge2.fromBufferAttribute(position, 3 * f + 2).sub(corner0);
    edge1.cross(edge2).normalize();
    for (let k = 0; k < 3; k++) normal.setXYZ(3 * f + k, edge1.x, edge1.y, edge1.z);
  }
  position.needsUpdate = true;
  normal.needsUpdate = true;
  geometry.boundingSphere = null; // measured again when next asked for
  geometry.boundingBox = null;
}
