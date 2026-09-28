import * as THREE from 'three';
import { ForestAnimal, type ForestAnimalOptions } from './ForestAnimal';
import { bone, coat, downAngle, ease, gloss, reach, rigidMesh, Sculpt, seededRandom, skinnedMesh, smooth, weighAlong, type Palette } from './parts';

type V3 = THREE.Vector3;

// The rig the forest lake's four-legged animals share (the wolf, the deer,
// the fox, the boar, the rabbit, the squirrel and the otter). Its bones are
// the spine (pelvis, chest, neck, head), the tail's, and four legs of four
// bones each (the shoulder or hip, the elbow or stifle, the wrist or hock,
// and the foot). The body, the tail and each leg are one mesh each that
// bends with them (skinned); the head is rigid on its bone.
//
// Legs are placed, not swung: each foot is planted where it lands, in the
// space the animal walks in, and stays there while the body goes on over it
// (so it never slides, however the animal is steered), then swings forward
// to land again. Each leg reaches its foot by two-bone inverse kinematics
// in its own plane, the cannon (below the wrist or hock) at the angle the
// stride gives it and the foot kept level on the ground.

// A way of going.
export interface Gait {
  stride: number; // m a planted foot travels back under the body
  cadence: number; // strides a second
  duty: number; // share of a stride each foot is down
  // Where each foot is in the stride: left fore, right fore, left hind,
  // right hind.
  offsets: readonly [number, number, number, number];
  lift: readonly [front: number, hind: number]; // m a foot rises as it swings
  fold: readonly [front: number, hind: number]; // rad its cannon folds back as it swings
  sink: number; // m the body is held lower going this way, so its legs have room to reach
  bob: number; // m the body rises and falls
  bounces: 1 | 2; // times a stride it does: twice walking, once galloping
  rock: number; // rad the body pitches, once a stride
  flex: number; // rad its back bends and stretches, once a stride
  nod: number; // rad the head nods with each bounce
  lag: number; // share of a stride its rise comes after the stride's start
}

export interface LegBuild {
  // The right leg's joints (x > 0), in the rig's space as it stands: its
  // root in the body (the shoulder or hip), its middle joint (the elbow or
  // stifle), the wrist or hock, and where the foot begins. All four at the same
  // x: the leg bends in its own plane.
  hip: V3;
  knee: V3;
  ankle: V3;
  paw: V3;
  bend: 1 | -1; // the middle joint ahead of the line to the foot (1, a stifle) or behind it (-1, an elbow)
  // Sculpts the leg and its foot as it stands, in the rig's space: the
  // right one (side 1), or the left (-1), whose joints are the right's
  // mirrored.
  build(sculpt: Sculpt, side: 1 | -1, joints: readonly [V3, V3, V3, V3]): void;
}

export interface FourLeggedBuild {
  variations: Readonly<Record<string, Palette>>;
  // The spine's joints as it stands, in the rig's space: the pelvis (between
  // the hips), the chest (between the shoulders), the neck's root, and where
  // the head turns on the neck.
  pelvis: V3;
  chest: V3;
  neck: V3;
  head: V3;
  // The torso and neck, in the rig's space as it stands.
  body(sculpt: Sculpt): void;
  // The head, as it stands; `shiny` takes the eyes and the nose.
  buildHead(sculpt: Sculpt, shiny: Sculpt): void;
  // How much bigger the head (and jaw) are than as sculpted, about the
  // head's pivot: the sheets draw some heads big. 1 by default.
  headScale?: number;
  // A lower jaw that opens (a howl), turning about its pivot.
  jaw?: { pivot: V3; build(sculpt: Sculpt, shiny: Sculpt): void };
  // The tail's joints, root first (inside the body), the tip of the tail
  // (the last joint when left out), how thick it is round each joint (half
  // its width; none by default), and the tail.
  tail: { joints: readonly V3[]; tip?: V3; radii?: readonly number[]; build(sculpt: Sculpt): void };
  front: LegBuild;
  hind: LegBuild;
  walk: Gait;
  run: Gait;
  seed?: number;
}

// Where the body is held, beyond the gait's own motion: what each pose
// eases toward.
export interface Posture {
  sink: number; // m both ends are lowered
  rear: number; // m the rump is lowered further (sitting, howling)
  front: number; // m the chest is lowered further (grazing)
  pitch: number; // rad the body tips further (< 0 its front up)
  flex: number; // rad the chest bends from the pelvis (< 0 up)
  neck: number; // rad the neck pitches from the chest (< 0 up)
  neckTurn: number; // rad it turns (> 0 toward the animal's right)
  head: number; // rad the head's own pitch in the world (< 0 nose up)
  headTurn: number; // rad it turns on the neck (> 0 to the right)
  headTilt: number; // rad it tilts
  jaw: number; // 0 shut, 1 open
  tail: number; // rad the tail lifts at its root (> 0 up)
  tailTurn: number; // rad it swings to one side (> 0 to the right)
  tailWag: number; // rad it wags either way
  tailCurl: number; // rad each tail bone curls up (> 0) or down
  sit: number; // 0..1: the hind cannons lie along the ground, as sitting
  hindReach: number; // m the hind feet reach forward of where they stand
  frontReach: number; // m the fore feet do
  frontFree: number; // 0..1: the fore paws off the ground, drawn up before the chest (standing up)
}

// A foot, as an animal's freeFoot() sees it: fore or hind, which side, where
// it stands (in the rig's space) and the leg's length.
export interface Foot {
  front: boolean;
  side: 1 | -1;
  rest: V3;
  length: number;
}

// A foot held off the ground: where it is (in the rig's space), its
// cannon's tilt from as built (rad, < 0 folded back) and its foot's curl.
export interface FreeFoot {
  at: V3;
  tilt: number;
  curl: number;
}

export const REST: Readonly<Posture> = {
  sink: 0,
  rear: 0,
  front: 0,
  pitch: 0,
  flex: 0,
  neck: 0,
  neckTurn: 0,
  head: 0,
  headTurn: 0,
  headTilt: 0,
  jaw: 0,
  tail: 0,
  tailTurn: 0,
  tailWag: 0,
  tailCurl: 0,
  sit: 0,
  hindReach: 0,
  frontReach: 0,
  frontFree: 0,
};

const GAIT_RATE = 3.5; // how fast it starts, stops and changes gait, a second
const POSE_RATE = 4; // how fast it changes pose
const SETTLE = 0.28; // s a foot still in the air takes to land once it stops
const REACH_SHARE = 0.998; // of a leg's length it stretches at most
const TAIL_CLEARANCE = 0.03; // m the tail's tip keeps above the ground

interface LegRig {
  front: boolean;
  side: 1 | -1;
  index: number; // in the gait's offsets
  parent: THREE.Bone;
  joints: readonly [V3, V3, V3, V3]; // as it stands, in the rig's space
  bones: readonly [THREE.Bone, THREE.Bone, THREE.Bone, THREE.Bone];
  rest: V3; // where its foot's joint stands, in the rig's space
  upper: number;
  lower: number;
  cannon: number;
  angles: readonly [number, number, number]; // the rest angles of its three bones, from straight down
  bend: 1 | -1;
  down: boolean;
  plant: V3; // where its planted foot is, in the space the animal moves in
  from: V3; // where it lifted off, in the rig's space
  q: number; // how far through its swing
  target: V3; // where its foot is this frame, in the rig's space
  tilt: number; // its cannon's tilt from as built, in the world
  curl: number; // its foot's
  held: 'leap' | 'hook' | 'held' | null; // off the ground this frame, and why
}

const v1 = new THREE.Vector3();
const v2 = new THREE.Vector3();
const m1 = new THREE.Matrix4();
const m2 = new THREE.Matrix4();
const q1 = new THREE.Quaternion();
const q2 = new THREE.Quaternion();
const X_AXIS = new THREE.Vector3(1, 0, 0);
const Z_AXIS = new THREE.Vector3(0, 0, 1);

export abstract class FourLegged extends ForestAnimal {
  readonly head = new THREE.Group();
  readonly body: THREE.SkinnedMesh;
  readonly tail: THREE.SkinnedMesh;
  readonly leftFrontLeg: THREE.SkinnedMesh;
  readonly rightFrontLeg: THREE.SkinnedMesh;
  readonly leftHindLeg: THREE.SkinnedMesh;
  readonly rightHindLeg: THREE.SkinnedMesh;

  protected readonly spine: { pelvis: THREE.Bone; chest: THREE.Bone; neck: THREE.Bone; head: THREE.Bone; tail: THREE.Bone[] };
  protected readonly jawPivot: THREE.Group | null = null;
  protected readonly posture: Posture = { ...REST };
  // What the posture eases toward this frame; pose() writes it.
  protected readonly goal: Posture = { ...REST };
  protected gait: 'stand' | 'walk' | 'run' = 'stand';
  protected mode = 'idle';
  protected modeTime = 0;
  protected time = 0;
  protected moving = 0; // 0 standing, 1 going; eased
  protected running = 0; // 0 walking, 1 running; eased
  protected phase = 0;
  // Rates, a second, the posture eases at; a pose may quicken them.
  protected poseRate = POSE_RATE;
  // Where the rig stands in the animal, beyond a leap's height: for an
  // animal that moves its whole body itself (a squirrel on a trunk). The
  // rig's turn is the animal's own to set too.
  protected readonly rigOffset = new THREE.Vector3();

  private readonly build: FourLeggedBuild;
  private readonly legs: LegRig[];
  private readonly tailTip = new THREE.Object3D();
  private readonly rest: { pelvis: V3; chest: V3 };
  private readonly span: { length: number; angle: number }; // from the pelvis to the chest
  private started = false;
  private leapState: { t: number; height: number; length: number; crouch: number; air: number } | null = null;
  private leapSink = 0; // m the body bends down for a leap this frame
  private leapPitch = 0; // rad it tips in the air

  protected constructor(build: FourLeggedBuild, options: ForestAnimalOptions) {
    super(build.variations, options);
    this.build = build;
    const random = seededRandom(options.seed ?? build.seed ?? 7);
    const palette = this.palette;
    const material = coat();
    const shine = gloss();

    // The bones, as it stands.
    const pelvis = bone('pelvis', this.rig, build.pelvis);
    const chest = bone('chest', pelvis, build.chest, build.pelvis);
    const neck = bone('neck', chest, build.neck, build.chest);
    const head = bone('head', neck, build.head, build.neck);
    neck.rotation.order = 'YXZ';
    head.rotation.order = 'YXZ';
    const tailJoints = build.tail.joints;
    const tail: THREE.Bone[] = [];
    tailJoints.forEach((at, i) => {
      const b = bone(`tail${i}`, i === 0 ? pelvis : tail[i - 1], at, i === 0 ? build.pelvis : tailJoints[i - 1]);
      b.rotation.order = 'YXZ';
      tail.push(b);
    });
    this.spine = { pelvis, chest, neck, head, tail };
    // The tail's tip, followed so it never goes into the ground.
    const last = tailJoints[tailJoints.length - 1];
    this.tailTip.position.copy(build.tail.tip ?? last).sub(last);
    tail[tail.length - 1].add(this.tailTip);
    this.rest = { pelvis: build.pelvis.clone(), chest: build.chest.clone() };
    const dy = build.chest.y - build.pelvis.y;
    const dz = build.chest.z - build.pelvis.z;
    this.span = { length: Math.hypot(dy, dz), angle: Math.atan2(dy, dz) };

    const all: THREE.Bone[] = [pelvis, chest, neck, head, ...tail];
    const legSpecs: { spec: LegBuild; front: boolean; side: 1 | -1; index: number }[] = [
      { spec: build.front, front: true, side: -1, index: 0 },
      { spec: build.front, front: true, side: 1, index: 1 },
      { spec: build.hind, front: false, side: -1, index: 2 },
      { spec: build.hind, front: false, side: 1, index: 3 },
    ];
    this.legs = legSpecs.map(({ spec, front, side, index }) => {
      const mirror = (v: V3) => new THREE.Vector3(v.x * side, v.y, v.z);
      const [h, k, a, p] = [mirror(spec.hip), mirror(spec.knee), mirror(spec.ankle), mirror(spec.paw)];
      const parent = front ? chest : pelvis;
      const parentAt = front ? build.chest : build.pelvis;
      const name = `${side < 0 ? 'left' : 'right'}${front ? 'Front' : 'Hind'}`;
      const b0 = bone(`${name}Hip`, parent, h, parentAt);
      const b1 = bone(`${name}Knee`, b0, k, h);
      const b2 = bone(`${name}Ankle`, b1, a, k);
      const b3 = bone(`${name}Foot`, b2, p, a);
      b0.rotation.order = 'ZYX';
      all.push(b0, b1, b2, b3);
      return {
        front,
        side,
        index,
        parent,
        joints: [h, k, a, p],
        bones: [b0, b1, b2, b3],
        rest: p.clone(),
        upper: h.distanceTo(k),
        lower: k.distanceTo(a),
        cannon: a.distanceTo(p),
        angles: [downAngle(k.y - h.y, k.z - h.z), downAngle(a.y - k.y, a.z - k.z), downAngle(p.y - a.y, p.z - a.z)],
        bend: spec.bend,
        down: true,
        plant: new THREE.Vector3(),
        from: p.clone(),
        q: 1,
        target: p.clone(),
        tilt: 0,
        curl: 0,
        held: null,
      } satisfies LegRig;
    });

    this.updateMatrixWorld(true);
    const skeleton = new THREE.Skeleton(all);
    const indexOf = (b: THREE.Bone) => all.indexOf(b);

    // The body, bending along the spine.
    const bodySculpt = new Sculpt(random);
    build.body(bodySculpt);
    const bodyGeometry = bodySculpt.geometry(palette);
    weighAlong(bodyGeometry, [build.pelvis, build.chest, build.neck, build.head], [pelvis, chest, neck, head].map(indexOf), 'linear');
    this.body = this.painting(skinnedMesh(bodyGeometry, material, skeleton));

    const tailSculpt = new Sculpt(random);
    build.tail.build(tailSculpt);
    const tailGeometry = tailSculpt.geometry(palette);
    weighAlong(tailGeometry, tailJoints, tail.map(indexOf), 'linear');
    this.tail = this.painting(skinnedMesh(tailGeometry, material, skeleton));

    const legMeshes = this.legs.map((leg) => {
      const spec = leg.front ? build.front : build.hind;
      const s = new Sculpt(random);
      spec.build(s, leg.side, leg.joints);
      const geometry = s.geometry(palette);
      weighAlong(geometry, leg.joints, leg.bones.map(indexOf), 0.22);
      return this.painting(skinnedMesh(geometry, material, skeleton));
    });
    [this.leftFrontLeg, this.rightFrontLeg, this.leftHindLeg, this.rightHindLeg] = legMeshes;

    // The head, rigid on its bone: sculpted as it stands and moved into the
    // bone's own space.
    const headSculpt = new Sculpt(random);
    const shinySculpt = new Sculpt(random, 0.01);
    build.buildHead(headSculpt, shinySculpt);
    const toHead = new THREE.Matrix4().makeTranslation(-build.head.x, -build.head.y, -build.head.z);
    this.head.scale.setScalar(build.headScale ?? 1);
    const headMesh = this.painting(rigidMesh(headSculpt.geometry(palette).applyMatrix4(toHead), material));
    this.head.add(headMesh);
    if (shinySculpt.triangles > 0) this.head.add(this.painting(rigidMesh(shinySculpt.geometry(palette).applyMatrix4(toHead), shine)));
    if (build.jaw) {
      const pivot = new THREE.Group();
      pivot.position.copy(build.jaw.pivot).sub(build.head);
      const jawSculpt = new Sculpt(random);
      const jawShiny = new Sculpt(random, 0.01);
      build.jaw.build(jawSculpt, jawShiny);
      const toJaw = new THREE.Matrix4().makeTranslation(-build.jaw.pivot.x, -build.jaw.pivot.y, -build.jaw.pivot.z);
      pivot.add(this.painting(rigidMesh(jawSculpt.geometry(palette).applyMatrix4(toJaw), material)));
      if (jawShiny.triangles > 0) pivot.add(this.painting(rigidMesh(jawShiny.geometry(palette).applyMatrix4(toJaw), shine)));
      this.head.add(pivot);
      this.jawPivot = pivot;
    }
    head.add(this.head);
    this.rig.add(this.body, this.tail, ...legMeshes);
  }

  // Stands still, breathing, looking about now and then. Named as in its
  // sheet's poses.
  Idle(): void {
    this.setMode('idle', 'stand');
  }

  // Walks forward the way it faces until told otherwise. Named as in its
  // sheet's poses.
  Walk(): void {
    this.setMode('walk', 'walk');
  }

  // Runs forward the way it faces until told otherwise. Named as in its
  // sheet's poses.
  Run(): void {
    this.setMode('run', 'run');
  }

  // What it is doing: its behaviour, from the sheet's poses.
  get doing(): string {
    return this.mode;
  }

  // Takes up the pose of what it is doing at once, rather than easing into
  // it: for an animal whose sheet draws it at rest in a pose of its own (a
  // squirrel sitting up), called once it is built.
  protected settle(): void {
    Object.assign(this.goal, REST);
    this.pose(this.goal);
    Object.assign(this.posture, this.goal);
  }

  protected setMode(mode: string, gait: 'stand' | 'walk' | 'run'): void {
    if (mode !== this.mode) this.modeTime = 0;
    this.mode = mode;
    this.gait = gait;
  }

  // The posture it holds for what it is doing, written into `goal` (every
  // field starts at rest). The idle look about is here; each animal adds its
  // own poses.
  protected pose(goal: Posture): void {
    if (this.mode === 'idle') {
      // Now and then a look to one side and back, and the tail's slow swing.
      const t = this.time;
      goal.headTurn = 0.28 * Math.sin(t * 0.37) * smooth(Math.sin(t * 0.23) * 1.5);
      goal.head = 0.05 * Math.sin(t * 0.31);
      goal.tailTurn = 0.08 * Math.sin(t * 0.9);
    }
  }

  // How the gait moves the posture this frame: lower going, so the legs can
  // reach, and the head held low and the tail out running.
  protected gaitPosture(goal: Posture, running: number, moving: number): void {
    const { walk, run } = this.build;
    goal.sink += (walk.sink + (run.sink - walk.sink) * running) * moving;
    goal.tail += (0.12 + 0.8 * running) * moving;
    goal.neck += 0.12 * running * moving;
  }

  // Springs up and forward: crouches, flies `length` m forward in an arc
  // `height` m high (just the time in the air gravity gives it), its fore
  // feet reaching forward and its hind feet back, and lands, bending to take
  // it. Ignored while one is under way.
  protected leap(height: number, length: number, crouch = 0.16): void {
    if (this.leapState) return;
    this.leapState = { t: 0, height, length, crouch, air: 2 * Math.sqrt((2 * height) / 9.81) };
  }

  // Whether it is in a leap, crouching for it or landing from it.
  get jumping(): boolean {
    return this.leapState !== null;
  }

  // A foot the animal holds off the ground this frame (paddling, reaching
  // up a trunk), in the rig's space, or null for one that walks. Each
  // animal may take its feet off the ground so; by default none.
  protected freeFoot(_foot: Foot): FreeFoot | null {
    return null;
  }

  // A forward speed of its own this frame (swimming), in place of its
  // gait's, which then stands; null to walk or run.
  protected cruise(): number | null {
    return null;
  }

  update(delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    this.time += dt;
    this.modeTime += dt;
    const { walk, run } = this.build;
    this.moving = ease(this.moving, this.gait === 'stand' ? 0 : 1, GAIT_RATE, dt);
    this.running = ease(this.running, this.gait === 'run' ? 1 : 0, GAIT_RATE, dt);
    const r = this.running;
    const mix = (a: number, b: number) => a + (b - a) * r;
    const cadence = mix(walk.cadence, run.cadence);
    const duty = mix(walk.duty, run.duty);
    const stride = mix(walk.stride, run.stride) * this.moving;

    // A leap: crouch, fly, land.
    let air = -1; // how far through the flight, or -1 on the ground
    this.leapSink = 0;
    this.leapPitch = 0;
    this.rig.position.copy(this.rigOffset);
    const L = this.leapState;
    if (L) {
      L.t += dt;
      const depth = 0.1 * this.rest.pelvis.y;
      if (L.t < L.crouch) this.leapSink = depth * Math.sin((Math.PI / 2) * (L.t / L.crouch));
      else if (L.t < L.crouch + L.air) {
        air = (L.t - L.crouch) / L.air;
        this.rig.position.y += 4 * L.height * air * (1 - air);
        this.leapPitch = -0.28 * Math.cos(Math.PI * air); // nose up leaving, level at the top, down landing
      } else if (L.t < 2 * L.crouch + L.air) this.leapSink = depth * Math.sin((Math.PI / 2) * (1 - (L.t - L.crouch - L.air) / L.crouch));
      else this.leapState = null;
    }
    const cruise = this.cruise();
    const active = this.moving > 0.02 && !this.leapState && cruise === null;
    if (active) this.phase = (this.phase + cadence * dt) % 1;
    this.speedNow = air >= 0 && L ? L.length / L.air : cruise ?? (active ? (stride * cadence) / duty : 0);
    this.advance(dt);

    // The posture: what it is doing, and the gait's own.
    Object.assign(this.goal, REST);
    this.pose(this.goal);
    this.gaitPosture(this.goal, r, this.moving);
    for (const key of Object.keys(REST) as (keyof Posture)[]) this.posture[key] = ease(this.posture[key], this.goal[key], this.poseRate, dt);

    // The feet: where each is this frame, planted, swinging, or held off
    // the ground.
    this.updateMatrix();
    this.rig.updateMatrix();
    const toWorld = m1.multiplyMatrices(this.matrix, this.rig.matrix);
    const toRig = m2.copy(toWorld).invert();
    if (!this.started) {
      this.started = true;
      for (const leg of this.legs) leg.plant.copy(leg.rest).applyMatrix4(toWorld);
    }
    const p = this.posture;
    for (const leg of this.legs) {
      const hooked = air >= 0 ? null : this.freeFoot({ front: leg.front, side: leg.side, rest: leg.rest, length: leg.upper + leg.lower + leg.cannon });
      const held = air >= 0 ? 'leap' : hooked ? 'hook' : leg.front && p.frontFree > 0.02 ? 'held' : null;
      if (held) {
        leg.down = false;
        leg.held = held;
        if (hooked) {
          leg.target.copy(hooked.at);
          leg.tilt = hooked.tilt;
          leg.curl = hooked.curl;
        }
        continue; // a tucked or held foot is placed once the body is
      }
      if (leg.held) {
        // Set down again where it is.
        leg.held = null;
        leg.target.y = leg.rest.y;
        this.touchDown(leg, toWorld);
      }
      const offset = mix(walk.offsets[leg.index], run.offsets[leg.index]);
      const lift = mix(walk.lift[leg.front ? 0 : 1], run.lift[leg.front ? 0 : 1]) * Math.min(1, this.moving * 1.5);
      const fold = mix(walk.fold[leg.front ? 0 : 1], run.fold[leg.front ? 0 : 1]) * Math.min(1, this.moving * 1.5);
      const land = v1.copy(leg.rest);
      land.z += stride / 2 + (leg.front ? p.frontReach : p.hindReach);
      if (active) {
        const u = (((this.phase + offset) % 1) + 1) % 1;
        const wantDown = u < duty;
        if (wantDown && !leg.down) this.touchDown(leg, toWorld);
        else if (!wantDown && leg.down) this.liftOff(leg);
        if (!leg.down) leg.q = (u - duty) / (1 - duty);
      } else if (!leg.down) {
        leg.q = Math.min(1, leg.q + dt / SETTLE);
        if (leg.q >= 1) this.touchDown(leg, toWorld);
      }
      if (leg.down) {
        // A foot left far behind (the animal put somewhere else) is set
        // down under it again.
        leg.target.copy(leg.plant).applyMatrix4(toRig);
        if (leg.target.distanceTo(leg.rest) > 2 * (leg.upper + leg.lower + leg.cannon)) {
          leg.plant.copy(leg.rest).applyMatrix4(toWorld);
          leg.target.copy(leg.rest);
        }
        leg.tilt = 0;
        leg.curl = 0;
      } else {
        const s = smooth(leg.q);
        const arc = Math.sin(Math.PI * leg.q);
        leg.target.lerpVectors(leg.from, land, s);
        leg.target.y += lift * arc;
        leg.tilt = -fold * arc;
        leg.curl = fold * (leg.front ? 0.8 : 0.4) * arc;
      }
      // Sitting: the hind cannons come down to lie along the ground.
      if (!leg.front && p.sit > 0) leg.tilt += p.sit * (Math.PI / 2 - 0.2 - leg.angles[2]);
    }

    // The body over its feet, lowered as far as the feet on the ground need
    // to reach them (never more than a third of the way down).
    let slack = 0;
    const most = this.rest.pelvis.y / 3;
    for (let pass = 0; pass < 6; pass++) {
      this.poseBody(slack);
      let need = 0;
      for (const leg of this.legs) if (!leg.held) need = Math.max(need, this.shortfall(leg));
      if (need <= 1e-6 || slack >= most) break;
      slack = Math.min(most, slack + need * 1.02);
    }

    // Feet off the ground: tucked in a leap, or the fore paws held up
    // before the chest (standing up, holding something).
    for (const leg of this.legs) {
      const length = leg.upper + leg.lower + leg.cannon;
      if (leg.held === 'leap') {
        const s = Math.sin(Math.PI * air);
        leg.target.copy(leg.rest);
        leg.target.y += (leg.front ? 0.22 : 0.12) * length * s;
        leg.target.z += (leg.front ? 0.38 : -0.42) * length * s;
        leg.tilt = (leg.front ? -0.9 : 0.35) * s;
        leg.curl = (leg.front ? 0.6 : -0.3) * s;
      } else if (leg.held === 'held') {
        // Before the chest as it is now, drawn up toward it.
        const f = p.frontFree;
        const chest = this.spine.chest;
        const hold = v2.copy(leg.rest).sub(this.build.chest);
        hold.y += 0.42 * length * f;
        hold.z += 0.2 * length * f;
        hold.applyMatrix4(chest.matrixWorld).applyMatrix4(m2.copy(this.rig.matrixWorld).invert());
        leg.target.lerpVectors(leg.rest, hold, Math.min(1, f * 1.2));
        leg.tilt = -1.1 * f;
        leg.curl = 0.9 * f;
      }
    }
    for (const leg of this.legs) this.place(leg);
  }

  private touchDown(leg: LegRig, toWorld: THREE.Matrix4): void {
    leg.down = true;
    leg.q = 1;
    leg.plant.copy(leg.target);
    leg.plant.y = leg.rest.y;
    leg.plant.applyMatrix4(toWorld);
  }

  private liftOff(leg: LegRig): void {
    leg.down = false;
    leg.q = 0;
    leg.from.copy(leg.target);
  }

  // The spine, the neck, the head and the tail, from the posture and the
  // gait, `slack` m lower than the posture holds it.
  private poseBody(slack: number): void {
    const p = this.posture;
    const { walk, run } = this.build;
    const r = this.running;
    const mix = (a: number, b: number) => a + (b - a) * r;
    const m = this.moving;
    const lag = mix(walk.lag, run.lag);
    const cycle = Math.PI * 2 * (this.phase - lag);
    // A walk rises twice a stride and a gallop once: blended by the gait.
    const bobWalk = walk.bob * Math.cos(2 * cycle);
    const bobRun = run.bob * (run.bounces === 1 ? Math.sin(cycle) : Math.cos(2 * cycle));
    const bob = (bobWalk + (bobRun - bobWalk) * r) * m;
    const rock = mix(walk.rock, run.rock) * m * Math.sin(cycle);
    const flex = mix(walk.flex, run.flex) * m * Math.cos(cycle);
    const nod = mix(walk.nod, run.nod) * m * Math.cos(2 * cycle);
    const breath = 0.004 * Math.sin(this.time * 2.1) * (1 - m);

    const { pelvis, chest, neck, head, tail } = this.spine;
    const low = p.sink + this.leapSink + slack - bob - breath;
    const yPelvis = this.rest.pelvis.y - low - p.rear;
    const yChest = this.rest.chest.y - low - p.front;
    pelvis.position.set(this.rest.pelvis.x, yPelvis, this.rest.pelvis.z);
    const lift = THREE.MathUtils.clamp((yChest - yPelvis) / this.span.length, -0.99, 0.99);
    const pitch = this.span.angle - Math.asin(lift) + p.pitch + rock + this.leapPitch;
    pelvis.rotation.x = pitch;
    chest.rotation.x = p.flex + flex;
    neck.rotation.set(p.neck + nod * 0.5, p.neckTurn, 0);
    head.rotation.set(p.head + nod * 0.5 - (pitch + chest.rotation.x + neck.rotation.x), p.headTurn, p.headTilt);
    const wag = p.tailWag * Math.sin(this.time * 9);
    tail.forEach((b, i) => {
      const wave = Math.sin(this.time * 2 - i * 0.7);
      if (i === 0) b.rotation.set(p.tail - pitch * 0.5 + 0.06 * m * Math.sin(cycle), p.tailTurn + wag + 0.05 * wave, 0);
      else b.rotation.set(p.tailCurl, (p.tailTurn * 0.3 + wag) * 0.6 + 0.04 * wave, 0);
    });
    if (this.jawPivot) this.jawPivot.rotation.x = 0.45 * p.jaw;
    this.rig.updateMatrixWorld(true);
    // A tail that would reach into the ground is lifted clear of it. (Read
    // from the matrices just worked out: getWorldPosition() would work the
    // animal's own again, moved on since, and leave the legs' behind.)
    const toRig = m2.copy(this.rig.matrixWorld).invert();
    const radii = this.build.tail.radii;
    // Lifted in small steps, so the lift grows smoothly as the rump comes
    // down (in steps of 0.1 rad it flicked the tail up).
    for (let i = 0; i < 50; i++) {
      // The tip, and each joint as far above the ground as the tail is
      // thick there.
      let low = v1.setFromMatrixPosition(this.tailTip.matrixWorld).applyMatrix4(toRig).y - TAIL_CLEARANCE;
      if (radii) tail.forEach((b, k) => (low = Math.min(low, v1.setFromMatrixPosition(b.matrixWorld).applyMatrix4(toRig).y - (radii[k] ?? 0) - 0.005)));
      // (Not while the rig itself is turned: a squirrel up a trunk.)
      if (low + this.rig.position.y > 0 || this.rig.quaternion.w < 0.999) break;
      tail[0].rotation.x += 0.02;
      tail[0].updateMatrixWorld(true);
    }
  }

  // Into a leg's parent bone's space from the rig's.
  private legSpace(leg: LegRig): THREE.Matrix4 {
    return new THREE.Matrix4().copy(this.rig.matrixWorld).invert().multiply(leg.parent.matrixWorld).invert();
  }

  // The leg's plane, its hip, and where its ankle must be: in the parent
  // bone's space.
  private solve(leg: LegRig) {
    const into = this.legSpace(leg);
    const foot = v2.copy(leg.target).applyMatrix4(into);
    const hip = leg.bones[0].position;
    // The parent bone's pitch in the rig.
    const forward = new THREE.Vector3(0, 0, 1).transformDirection(into);
    const pitch = Math.atan2(forward.y, forward.z);
    const dx = foot.x - hip.x;
    const dy = foot.y - hip.y;
    // The leg swings out to its foot: its plane turned about the body's
    // length so the foot lies in it, below its root, or above it (a leg
    // reaching far forward from a chest tipped up).
    const out = dy <= 0 ? Math.atan2(dx, -dy) : Math.atan2(-dx, dy);
    const down = dy <= 0 ? -Math.hypot(dx, dy) : Math.hypot(dx, dy);
    const dz = foot.z - hip.z;
    const cannon = leg.angles[2] + leg.tilt + pitch;
    const ankleY = down + leg.cannon * Math.cos(cannon);
    const ankleZ = dz - leg.cannon * Math.sin(cannon);
    return { pitch, out, cannon, ankleY, ankleZ };
  }

  // How much lower the body must be for the leg to reach its foot.
  private shortfall(leg: LegRig): number {
    const { ankleY, ankleZ } = this.solve(leg);
    const d = Math.hypot(ankleY, ankleZ);
    const most = (leg.upper + leg.lower) * REACH_SHARE;
    if (d <= most) return 0;
    // Lowered by s, the ankle comes nearer by about s * |y| / d.
    return (d - most) * Math.min(3, d / Math.max(Math.abs(ankleY), 1e-6));
  }

  private place(leg: LegRig): void {
    const { pitch, out, cannon, ankleY, ankleZ } = this.solve(leg);
    const { upper, lower } = reach(0, 0, ankleY, ankleZ, leg.upper, leg.lower, leg.bend);
    const [r0, r1, r2] = leg.angles;
    const b0 = r0 - upper;
    const b1 = r1 - lower;
    const b2 = r2 - cannon;
    const b3 = leg.curl - pitch;
    const [hip, knee, ankle, foot] = leg.bones;
    hip.rotation.set(b0, 0, out);
    knee.rotation.set(b1 - b0, 0, 0);
    ankle.rotation.set(b2 - b1, 0, 0);
    // The foot flat again on the body, pitched b3 in its parent's space:
    // the hip swings out (about the body's length) before it bends, so the
    // swing is undone under the bends, not after them (else, on a turn, a
    // long paw's edge would dip).
    foot.quaternion.setFromAxisAngle(X_AXIS, -b2).multiply(q1.setFromAxisAngle(Z_AXIS, -out)).multiply(q2.setFromAxisAngle(X_AXIS, b3));
  }
}

// Rings along a leg's joints: `stations` at `t` along it (0 the root, 1 the
// middle joint, 2 the wrist or hock, 3 the foot's joint; below 0 up past the
// root into the body), each `w` across and `d` front to back (half sizes),
// moved `dz` forward and `dy` up. The rings stand square to the leg, their
// sideways axis out from the body on either side.
export interface Station {
  t: number;
  w: number;
  d: number;
  back?: number; // half depth behind, when it differs from in front
  n?: number;
  dy?: number;
  dz?: number;
}

export function legRings(joints: readonly [V3, V3, V3, V3], stations: readonly Station[]): { at: V3; w: number; up: number; down: number; n?: number }[] {
  return stations.map((s) => {
    const i = Math.max(0, Math.min(2, Math.floor(s.t)));
    const f = s.t - i;
    const at = joints[i].clone().lerp(joints[i + 1], f);
    at.y += s.dy ?? 0;
    at.z += s.dz ?? 0;
    return { at, w: s.w, up: s.d, down: s.back ?? s.d, n: s.n };
  });
}
