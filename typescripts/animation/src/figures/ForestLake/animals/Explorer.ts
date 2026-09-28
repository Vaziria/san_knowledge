import * as THREE from 'three';
import { halo } from '../parts';
import { ForestAnimal, type ForestAnimalOptions } from './ForestAnimal';
import {
  ANKLE_HEIGHT,
  FOREARM,
  joint,
  sculptFist,
  sculptFoot,
  sculptForearm,
  sculptHand,
  sculptHead,
  sculptLantern,
  sculptPack,
  sculptSeat,
  sculptShin,
  sculptSkin,
  sculptThigh,
  sculptTorso,
  sculptUpperArm,
  SHIN,
  THIGH,
  UPPER_ARM,
  VARIATIONS,
} from './ExplorerBody';
import { EXPRESSIONS, ExplorerFace } from './ExplorerFace';
import { HEIGHT, into, P, U } from './ExplorerKit';
import { buildTools, TOOLS, type ToolName, type Tools } from './ExplorerTools';
import { bump, coat, ease, gloss, glowing, reach, rigidMesh, Sculpt, seededRandom, smooth } from './parts';

// The forest lake's young explorer (Explorer.md), as its character sheet
// draws him: a chibi boy of about ten, his head a third of his height, with
// messy brown hair in chunky pointed locks, big brown eyes with glints, a
// small nose, a smile and rosy cheeks; a cream shirt with rolled sleeves, a
// green scarf over his shoulders falling to a short cape with a leaf on its
// back, a brown belt with a gold buckle and pouches, dark baggy shorts,
// brown boots with turned-down cuffs and leather bracers; a brown backpack
// with a cream bedroll strapped on top, and a small glowing lantern hanging
// at his side. Flat facets in the sheet's own colours.
//
// Meters; he faces +z, his left (as the codebase names it) is -x, and his
// origin is on the ground between his feet. 1.2 m tall to the tip of his
// hair's tuft. Built from the sheet's front and side views in their pixels
// (ExplorerKit: U m each).
//
// The rig is a tree of groups, one at each joint, each holding rigid parts
// that overlap at the joints as a chibi's do: the hips (the body), the spine
// (waist) with the neck and head, each arm (shoulder, elbow, wrist) and each
// leg (hip, knee, ankle). Legs are placed, not swung: a foot planted on the
// ground stays where it is in the world while the body goes on over it, and
// each leg reaches its foot by two-bone inverse kinematics.

type V = THREE.Vector3;

const THIGH_M = THIGH * U;
const SHIN_M = SHIN * U;
const UPPER_M = UPPER_ARM * U;
const FORE_M = FOREARM * U;
const REACH_SHARE = 0.998; // of a leg's length it stretches at most
const TOE_OUT = 0.1; // rad each foot turns out
const REST_FOOT = [43, ANKLE_HEIGHT, 0] as const; // px: where the right ankle stands (the left at -x)
const GAIT_RATE = 3.5; // a second: how fast it starts, stops and changes gait
const SETTLE = 0.25; // s a foot still in the air takes to land once it stops
const HAND = 0.1; // m from the wrist to the fingertips
const HEEL = 27; // px the boot's sole reaches behind the ankle
const TOE = 61; // and in front of it

// How far a foot pitched `pitch` (toes down > 0) must rise so that neither
// its toe nor its heel goes below where its flat sole would be (m).
function footDrop(pitch: number): number {
  const c = Math.cos(pitch);
  const s = Math.sin(pitch);
  const lowest = Math.min(-ANKLE_HEIGHT * c + HEEL * s, -ANKLE_HEIGHT * c - TOE * s);
  return Math.max(0, -ANKLE_HEIGHT - lowest) * U;
}

// A way of going: a planted foot travels `stride` m back under the body,
// `cadence` strides a second, each foot down `duty` of the time; so the
// speed is stride × cadence / duty.
interface Gait {
  stride: number;
  cadence: number;
  duty: number;
  lift: number; // m a swinging foot rises
  bob: number; // m the hips rise and fall each step
  lean: number; // rad forward
  swing: number; // rad the arms swing either way
  elbow: number; // rad the elbows bend
  twist: number; // rad the hips turn with the legs (the shoulders against them)
  sway: number; // m the hips shift over the planted foot
}

const WALK: Gait = { stride: 0.26, cadence: 1.7, duty: 0.6, lift: 0.055, bob: 0.01, lean: 0.07, swing: 0.45, elbow: 0.35, twist: 0.12, sway: 0.012 };
const RUN: Gait = { stride: 0.36, cadence: 2.4, duty: 0.38, lift: 0.11, bob: 0.022, lean: 0.26, swing: 0.95, elbow: 1.45, twist: 0.2, sway: 0.008 };

// The jump: a crouch, `JUMP_HEIGHT` m up (just the time in the air gravity
// gives), and a landing that bends to take it.
const JUMP_HEIGHT = 0.35;
const CROUCH = 0.18;
const AIR = 2 * Math.sqrt((2 * JUMP_HEIGHT) / 9.81);
const LANDING = 0.24;

const WAVE_TIME = 2.6;
const INTERACT_TIME = 4.6;
const SIT_TIME = 0.9;
const RISE_TIME = 0.75;

// Where the body is held beyond the gait's own motion: what each pose
// eases toward.
interface Posture {
  sink: number; // m the hips are lowered
  back: number; // m the hips move back (sitting)
  tip: number; // rad the hips tip forward
  lean: number; // rad the chest leans forward from the hips
  bend: number; // rad it bends toward its right (+x)
  twist: number; // rad it turns (> 0 toward +x)
  nod: number; // rad the head looks down (< 0 up)
  turn: number; // rad the head turns toward +x
  tilt: number; // rad the head tilts
  // Each foot moved from where it stands (m, the rig's space) and pitched
  // (rad, toes down > 0): left then right.
  lx: number;
  ly: number;
  lz: number;
  lp: number;
  rx: number;
  ry: number;
  rz: number;
  rp: number;
  free: number; // 0..1: the feet held off the ground (a jump), moved from where they stand
}

const REST: Readonly<Posture> = { sink: 0, back: 0, tip: 0, lean: 0, bend: 0, twist: 0, nod: 0, turn: 0, tilt: 0, lx: 0, ly: 0, lz: 0, lp: 0, rx: 0, ry: 0, rz: 0, rp: 0, free: 0 };

// An arm's pose: which way the upper arm and the forearm point (unit
// vectors in the chest's space), the wrist's bend and twist, a fist or an
// open hand, and how far it reaches for `target` (the rig's space) instead.
interface ArmPose {
  up: V;
  fore: V;
  wrist: number;
  twist: number;
  fist: number;
  reach: number;
  target: V;
}

function armPose(): ArmPose {
  return { up: new THREE.Vector3(0, -1, 0), fore: new THREE.Vector3(0, -1, 0), wrist: 0, twist: 0, fist: 0, reach: 0, target: new THREE.Vector3() };
}

interface LegRig {
  side: -1 | 1;
  index: 0 | 1; // in the gait: the left foot leads
  hip: THREE.Group;
  knee: THREE.Group;
  ankle: THREE.Group;
  rest: V; // where its ankle stands, in the rig's space
  down: boolean;
  air: boolean; // held off the ground
  plant: V; // where the planted ankle is, in the space the figure moves in
  plantYaw: number;
  from: V; // where it lifted off, in the rig's space
  q: number; // how far through its swing
  base: V; // where the gait puts the ankle this frame, in the rig's space
  target: V; // and with the pose's move
  yaw: number; // the foot's heading, in the space the figure moves in
  pitch: number;
}

interface ArmRig {
  side: -1 | 1;
  shoulder: THREE.Group;
  elbow: THREE.Group;
  wrist: THREE.Group;
  open: THREE.Mesh;
  fist: THREE.Mesh;
  pose: ArmPose;
  goal: ArmPose;
}

const X_AXIS = new THREE.Vector3(1, 0, 0);
const Y_AXIS = new THREE.Vector3(0, 1, 0);
const v1 = new THREE.Vector3();
const v2 = new THREE.Vector3();
const q1 = new THREE.Quaternion();
const q2 = new THREE.Quaternion();
const m1 = new THREE.Matrix4();
const m2 = new THREE.Matrix4();

// Something hanging from a point (the lantern from its hook or a hand, the
// fishing line from the rod's tip): it hangs straight down in the world and
// swings as the point moves, a damped pendulum `length` m long.
class Hanging {
  private readonly bob = new THREE.Vector3();
  private readonly last = new THREE.Vector3();
  private started = false;

  constructor(
    private readonly object: THREE.Object3D,
    private readonly length: number,
    private readonly damping: number,
  ) {}

  reset(): void {
    this.started = false;
  }

  update(dt: number): void {
    const parent = this.object.parent;
    if (!parent) return;
    parent.updateWorldMatrix(true, false);
    const pivot = v1.copy(this.object.position).applyMatrix4(parent.matrixWorld);
    if (!this.started) {
      this.bob.copy(pivot).y -= this.length;
      this.last.copy(this.bob);
      this.started = true;
    }
    if (dt > 0) {
      const moved = v2.subVectors(this.bob, this.last).multiplyScalar(Math.exp(-this.damping * dt));
      this.last.copy(this.bob);
      this.bob.add(moved);
      this.bob.y -= 9.81 * dt * dt;
      const d = v2.subVectors(this.bob, pivot);
      const l = d.length();
      if (!Number.isFinite(l) || l < 1e-9) {
        this.bob.copy(pivot).y -= this.length;
        this.last.copy(this.bob);
      } else this.bob.copy(pivot).addScaledVector(d, this.length / l);
    }
    const down = v2.subVectors(this.bob, pivot).normalize();
    // Turned as its holder faces, then swung.
    const e = parent.matrixWorld.elements;
    const heading = Math.atan2(e[8], e[10]);
    q1.setFromAxisAngle(Y_AXIS, heading);
    q2.setFromUnitVectors(new THREE.Vector3(0, -1, 0), down).multiply(q1);
    parent.getWorldQuaternion(q1);
    this.object.quaternion.copy(q1.invert().multiply(q2));
  }
}

export class Explorer extends ForestAnimal {
  // The sheet's one colour variation.
  static readonly COLORS = Object.keys(VARIATIONS);
  // Its expressions and tools, as SetExpression() and Hold() take them.
  static readonly EXPRESSIONS = Object.keys(EXPRESSIONS);
  static readonly TOOLS: readonly string[] = TOOLS;
  static readonly HEIGHT = HEIGHT;
  // m: how high the seat Sit() sits on is (a log's top).
  static readonly SEAT = 0.17;
  // Where Interact() touches, in its own space: in front of it, low down
  // (a sapling's leaves).
  static readonly TOUCH = new THREE.Vector3(-0.025, 0.215, 0.33);

  readonly head = new THREE.Group();
  readonly body = new THREE.Group(); // the hips and shorts, the torso above them
  readonly leftArm = new THREE.Group();
  readonly rightArm = new THREE.Group();
  readonly leftLeg = new THREE.Group();
  readonly rightLeg = new THREE.Group();
  readonly backpack = new THREE.Group();
  readonly lantern = new THREE.Group();
  readonly holder = new THREE.Group(); // in the left hand: what Hold() gives it

  private readonly spine = new THREE.Group();
  private readonly neck = new THREE.Group();
  private readonly legs: LegRig[];
  private readonly arms: ArmRig[];
  private readonly face: ExplorerFace;
  private readonly tools: Tools;
  private readonly lanternSwing: Hanging;
  private readonly lineSwing: Hanging;
  private readonly posture: Posture = { ...REST };
  private readonly goal: Posture = { ...REST };
  private readonly hipsRest: V;
  private readonly spineRest: V;
  private mode = 'idle';
  private queued: string | null = null;
  private gait: 'stand' | 'walk' | 'run' = 'stand';
  private modeTime = 0;
  private time = 0;
  private moving = 0;
  private running = 0;
  private phase = 0;
  private poseRate = 6;
  private jumpHeight = 0;
  private started = false;
  private face_ = 'happy';
  private tool: ToolName = 'nothing';
  private blinkAt = 2.2;
  private blinkUntil = 0;
  private blinks = 0;

  constructor(options: ForestAnimalOptions = {}) {
    super(VARIATIONS, options);
    const random = seededRandom(options.seed ?? 11);
    const palette = this.palette;
    const material = coat();
    const shine = gloss(0.3);
    const metal = gloss(0.42);
    const glass = glowing();
    const keep = <T extends THREE.Mesh>(mesh: T) => this.painting(mesh);
    const mesh = (s: Sculpt, m: THREE.Material, at?: V, shadows = true) => {
      const g = s.geometry(palette);
      return keep(rigidMesh(at ? into(g, at) : g, m, shadows));
    };

    // The joints, as it stands.
    this.hipsRest = joint('hips');
    this.spineRest = joint('spine');
    this.body.position.copy(this.hipsRest);
    this.body.rotation.order = 'YXZ';
    this.rig.add(this.body);
    this.spine.position.copy(this.spineRest).sub(this.hipsRest);
    this.spine.rotation.order = 'YXZ';
    this.body.add(this.spine);
    this.neck.position.copy(joint('neck')).sub(this.spineRest);
    this.spine.add(this.neck);
    this.head.position.copy(joint('head')).sub(joint('neck'));
    this.head.rotation.order = 'YXZ';
    this.neck.add(this.head);

    // The head: skin and hair, then the face laid onto the skin.
    const skin = new Sculpt(random);
    sculptSkin(skin);
    const headSculpt = new Sculpt(random, 0.025);
    sculptHead(headSculpt, skin);
    this.head.add(mesh(headSculpt, material, joint('head')));
    this.face = new ExplorerFace(palette, skin, random, { coat: material, gloss: shine }, keep);
    this.head.add(this.face.group);

    // The body: the shorts' seat on the hips; the shirt, scarf, cape, belt
    // and pouches on the spine.
    const seat = new Sculpt(random);
    sculptSeat(seat);
    this.body.add(mesh(seat, material, this.hipsRest));
    const torso = new Sculpt(random);
    const glow = new Sculpt(random, 0);
    sculptTorso(torso, glow);
    this.spine.add(mesh(torso, material, this.spineRest));
    this.spine.add(mesh(glow, glass, this.spineRest, false));

    // The backpack, and the lantern on its hook.
    const pack = new Sculpt(random);
    const roll = new Sculpt(random);
    sculptPack(pack, roll);
    this.backpack.add(mesh(pack, material, this.spineRest));
    this.spine.add(this.backpack);
    const lanternSculpt = new Sculpt(random);
    const glassSculpt = new Sculpt(random, 0.02);
    sculptLantern(lanternSculpt, glassSculpt);
    this.lantern.add(mesh(lanternSculpt, material));
    this.lantern.add(mesh(glassSculpt, glass, undefined, false));
    const light = halo(0xffc768, 0.2, 0.8);
    light.position.set(0, -25.5 * U, 0);
    this.lantern.add(light);
    this.lantern.position.copy(joint('hook')).sub(this.spineRest);
    this.backpack.add(this.lantern);
    this.lanternSwing = new Hanging(this.lantern, 0.07, 2.2);

    // The legs, each straight down from its hip joint.
    this.legs = ([-1, 1] as const).map((side, index) => {
      const hip = side < 0 ? this.leftLeg : this.rightLeg;
      hip.position.copy(joint('hip', side)).sub(this.hipsRest);
      hip.rotation.order = 'ZYX';
      this.body.add(hip);
      const thigh = new Sculpt(random);
      sculptThigh(thigh, side);
      hip.add(mesh(thigh, material));
      const knee = new THREE.Group();
      knee.position.set(0, -THIGH_M, 0);
      hip.add(knee);
      const shin = new Sculpt(random);
      sculptShin(shin);
      knee.add(mesh(shin, material));
      const ankle = new THREE.Group();
      ankle.position.set(0, -SHIN_M, 0);
      knee.add(ankle);
      const foot = new Sculpt(random);
      sculptFoot(foot, side);
      const footMesh = mesh(foot, material);
      // Its sole exactly on the ground as it stands (the faceted rings'
      // lowest corners stop a little short of it).
      footMesh.geometry.computeBoundingBox();
      footMesh.geometry.translate(0, -ANKLE_HEIGHT * U - footMesh.geometry.boundingBox!.min.y, 0);
      ankle.add(footMesh);
      const rest = P(side * REST_FOOT[0], REST_FOOT[1], REST_FOOT[2]);
      return {
        side,
        index: index as 0 | 1,
        hip,
        knee,
        ankle,
        rest,
        down: true,
        air: false,
        plant: new THREE.Vector3(),
        plantYaw: 0,
        from: rest.clone(),
        q: 1,
        base: rest.clone(),
        target: rest.clone(),
        yaw: 0,
        pitch: 0,
      } satisfies LegRig;
    });

    // The arms, each straight down from its shoulder.
    this.arms = ([-1, 1] as const).map((side) => {
      const shoulder = side < 0 ? this.leftArm : this.rightArm;
      shoulder.position.copy(joint('shoulder', side)).sub(this.spineRest);
      this.spine.add(shoulder);
      const upper = new Sculpt(random);
      sculptUpperArm(upper, side);
      shoulder.add(mesh(upper, material));
      const elbow = new THREE.Group();
      elbow.position.set(0, -UPPER_M, 0);
      shoulder.add(elbow);
      const fore = new Sculpt(random);
      sculptForearm(fore);
      elbow.add(mesh(fore, material));
      const wrist = new THREE.Group();
      wrist.position.set(0, -FORE_M, 0);
      elbow.add(wrist);
      const hand = new Sculpt(random);
      sculptHand(hand, side);
      const open = mesh(hand, material);
      open.geometry.scale(1.15, 1.15, 1.15); // a chibi's big hands, as the sheet draws them
      const fistSculpt = new Sculpt(random);
      sculptFist(fistSculpt, side);
      const fist = mesh(fistSculpt, material);
      fist.geometry.scale(1.15, 1.15, 1.15);
      fist.visible = false;
      wrist.add(open, fist);
      return { side, shoulder, elbow, wrist, open, fist, pose: armPose(), goal: armPose() } satisfies ArmRig;
    });

    // What it holds, in its left hand: the handle through the fist.
    this.holder.position.copy(P(1.4, -13, 1.1));
    this.holder.rotation.x = Math.PI / 2;
    this.arms[0].wrist.add(this.holder);
    // The tools are kept out of the figure until held, so that they never
    // count in its bounds (the fishing rod is as long as it is tall).
    this.tools = buildTools(palette, random, { coat: material, metal }, keep);
    this.lineSwing = new Hanging(this.tools.line, 0.42, 1.2);

    this.hang(-1, this.arms[0].pose);
    this.hang(1, this.arms[1].pose);
    this.update(0);
  }

  // What it is doing: one of its behaviours, lowercase.
  get doing(): string {
    return this.mode;
  }

  get expression(): string {
    return this.face_;
  }

  get holding(): string {
    return this.tool;
  }

  // Whether it is off the ground in a jump, or crouching for it or landing.
  get jumping(): boolean {
    return this.mode === 'jump';
  }

  // Stands, breathing, blinking and looking about now and then. Named as in
  // its sheet's Animation Preview.
  Idle(): void {
    this.request('idle');
  }

  // Walks forward the way it faces until told otherwise, swinging its arms.
  Walk(): void {
    this.request('walk');
  }

  // Runs forward the way it faces until told otherwise, leaning into it, its
  // fists pumping.
  Run(): void {
    this.request('run');
  }

  // Crouches and jumps 35 cm straight up, arms flung wide and a knee drawn
  // up as the sheet draws it, lands, and stands again. Ignored while one is
  // under way.
  Jump(): void {
    if (this.mode === 'jump') return;
    this.request('jump');
  }

  // Kneels on one knee, reaches down and touches something low before it
  // (as the sheet's picture of it touching a sapling), then stands again.
  Interact(): void {
    if (this.mode === 'interact') return;
    this.request('interact');
  }

  // Raises a hand and waves it, then puts it down: the left hand, as the
  // sheet draws it, or the right when the left holds a tool.
  Wave(): void {
    if (this.mode === 'wave') return;
    this.request('wave');
  }

  // Sits down, as on a log (SEAT high), its hands on its knees, looking
  // about, until told otherwise; then stands up before doing it.
  Sit(): void {
    this.request('sit');
  }

  // Shows one of the sheet's expressions: happy, laugh, surprised, angry,
  // wink or smirk. An unknown one throws, naming them.
  SetExpression(name: string): void {
    if (!Object.hasOwn(EXPRESSIONS, name)) throw new Error(`"${name}" is not one of its expressions: ${Explorer.EXPRESSIONS.join(', ')}`);
    this.face_ = name;
  }

  // Takes one of the sheet's tools in its left hand: sword, fishing-rod,
  // pickaxe, axe or lantern (the one from its side); "nothing" puts it away
  // (the lantern back on its hook). An unknown one throws, naming them.
  Hold(tool: string): void {
    if (!(TOOLS as readonly string[]).includes(tool)) throw new Error(`"${tool}" is not one of its tools: ${TOOLS.join(', ')}`);
    this.tool = tool as ToolName;
    for (const [name, group] of Object.entries(this.tools.groups)) {
      if (name === tool) this.holder.add(group);
      else group.removeFromParent();
    }
    if (tool === 'lantern') {
      if (this.lantern.parent !== this.holder) {
        this.holder.add(this.lantern);
        this.lantern.position.set(0, 0, 0);
        this.lanternSwing.reset();
      }
    } else if (this.lantern.parent !== this.backpack) {
      this.backpack.add(this.lantern);
      this.lantern.position.copy(joint('hook')).sub(this.spineRest);
      this.lanternSwing.reset();
    }
    if (tool === 'fishing-rod') this.lineSwing.reset();
  }

  private request(mode: string): void {
    if (this.mode === 'sit' || this.mode === 'rise') {
      if (mode === 'sit') {
        if (this.mode === 'rise') this.setMode('sit');
        return;
      }
      this.queued = mode;
      if (this.mode !== 'rise') this.setMode('rise');
      return;
    }
    this.setMode(mode);
  }

  private setMode(mode: string): void {
    if (mode !== this.mode) this.modeTime = 0;
    this.mode = mode;
    this.gait = mode === 'walk' ? 'walk' : mode === 'run' ? 'run' : 'stand';
  }

  // ------------------------------------------------------------ poses

  // An arm hanging relaxed, a little out from the body, the elbow soft.
  private hang(side: number, arm: ArmPose): void {
    arm.up.set(side * 0.38, -1, -0.02).normalize();
    arm.fore.set(side * 0.5, -1, 0.12).normalize();
    arm.wrist = 0.12;
    arm.twist = -side * 0.9; // the backs of its hands forward and out, as the sheet draws them
    arm.fist = 0;
    arm.reach = 0;
  }

  // The left arm holding its tool before it.
  private carry(arm: ArmPose): void {
    switch (this.tool) {
      case 'fishing-rod':
        arm.up.set(-0.25, -1, 0.35).normalize();
        arm.fore.set(-0.05, -0.1, 1).normalize();
        arm.wrist = -0.75;
        break;
      case 'lantern':
        arm.up.set(-0.28, -1, 0.3).normalize();
        arm.fore.set(-0.05, 0.05, 1).normalize();
        arm.wrist = 0;
        break;
      default:
        arm.up.set(-0.22, -1, 0.18).normalize();
        arm.fore.set(-0.1, -0.3, 1).normalize();
        arm.wrist = 0.1;
    }
    arm.twist = 0;
    arm.fist = 1;
    arm.reach = 0;
  }

  // The posture and arms each behaviour holds this frame, into `goal` and
  // the arms' goals.
  private pose(goal: Posture): void {
    const t = this.modeTime;
    const [left, right] = this.arms.map((a) => a.goal);
    this.hang(-1, left);
    this.hang(1, right);
    if (this.tool !== 'nothing') this.carry(left);
    this.poseRate = 6;
    const look = () => {
      const s = this.time;
      goal.turn = 0.32 * Math.sin(s * 0.37) * smooth(Math.sin(s * 0.23) * 1.5);
      goal.nod = 0.04 * Math.sin(s * 0.31) - 0.02;
    };
    switch (this.mode) {
      case 'idle':
        look();
        goal.lean = 0.012 * Math.sin(this.time * 2.1);
        break;
      case 'walk':
      case 'run':
        left.fist = right.fist = 1;
        break;
      case 'jump': {
        this.poseRate = 22;
        if (t < CROUCH) {
          const k = smooth(t / CROUCH);
          goal.sink = 0.075 * k;
          goal.lean = 0.28 * k;
          goal.nod = -0.12 * k;
          for (const [arm, side] of [
            [left, -1],
            [right, 1],
          ] as const) {
            if (arm === left && this.tool !== 'nothing') continue;
            arm.up.set(side * 0.25, -1, -0.55).normalize();
            arm.fore.set(side * 0.2, -1, -0.25).normalize();
          }
        } else if (t < CROUCH + AIR) {
          const a = (t - CROUCH) / AIR;
          const s = Math.sin(Math.PI * a);
          goal.free = 1;
          goal.lean = 0.08 - 0.1 * s;
          goal.nod = -0.1;
          // One knee drawn up before it, the other foot kicked back.
          goal.lx = 0.01 * s;
          goal.ly = 0.12 * s;
          goal.lz = 0.1 * s;
          goal.lp = -0.2 * s;
          goal.rx = 0.01 * s;
          goal.ry = 0.14 * s;
          goal.rz = -0.12 * s;
          goal.rp = 0.7 * s;
          for (const [arm, side] of [
            [left, -1],
            [right, 1],
          ] as const) {
            if (arm === left && this.tool !== 'nothing') continue;
            arm.up.set(side * 1, 0.2 + 0.1 * s, 0.12).normalize();
            arm.fore.set(side * 0.9, 0.35 + 0.25 * s, 0.35).normalize();
            arm.fist = 0.3;
          }
        } else {
          const k = Math.min(1, (t - CROUCH - AIR) / LANDING);
          goal.sink = 0.085 * Math.sin(Math.PI * k);
          goal.lean = 0.22 * Math.sin(Math.PI * k);
          if (t > CROUCH + AIR + LANDING + 0.15) this.setMode('idle');
        }
        break;
      }
      case 'wave': {
        this.poseRate = 10;
        const side = this.tool === 'nothing' ? -1 : 1;
        const arm = side < 0 ? left : right;
        const up = smooth(t / 0.35) * (1 - smooth((t - 2.15) / 0.4));
        const wave = 0.38 * Math.sin(2 * Math.PI * 2.1 * Math.max(0, t - 0.3)) * smooth((t - 0.3) / 0.2) * (1 - smooth((t - 2.0) / 0.2));
        const raised = new THREE.Vector3(side * 0.9, 0.62, 0.14).normalize();
        const forearm = new THREE.Vector3(side * 0.1, 1, 0.16).normalize().applyAxisAngle(new THREE.Vector3(0, 0.15, 1).normalize(), -side * wave);
        arm.up.lerp(raised, up).normalize();
        arm.fore.lerp(forearm, up).normalize();
        arm.wrist = 0.12 * (1 - up);
        arm.twist = arm.twist * (1 - up); // the raised arm already holds its palm forward
        arm.fist = 0;
        goal.tilt = -side * 0.1 * up;
        goal.turn = side * 0.08 * up;
        goal.lean = -0.04 * up;
        goal.bend = -side * 0.05 * up;
        if (t > WAVE_TIME + 0.2) this.setMode('idle');
        break;
      }
      case 'interact': {
        this.poseRate = 12;
        const step = (from: number, length: number) => smooth((t - from) / length);
        // Steps the right foot forward, kneels on the left knee, reaches down
        // with the left hand and touches, then back up and the foot back.
        const out = step(0, 0.5) * (1 - step(3.9, 0.5));
        const lifting = bump(Math.min(1, t / 0.5)) + bump(Math.max(0, (t - 3.9) / 0.5));
        const kneel = step(0.3, 0.8) * (1 - step(3.3, 0.7));
        const hand = step(1.0, 0.6) * (1 - step(2.9, 0.5));
        // Kneeling, the knee is as high as the shorts' wide hem round it
        // (8.5 cm), the shin rises back to the ankle and the foot stands on
        // its toe (footDrop lifts the ankle so the toe is on the ground).
        goal.rz = 0.2 * out;
        goal.ry = 0.05 * lifting;
        goal.rx = 0.01 * out;
        goal.sink = 0.12 * kneel;
        goal.lz = -0.137 * kneel;
        goal.lx = 0.012 * kneel;
        goal.lp = 0.85 * kneel;
        goal.lean = 0.3 * kneel + 0.16 * hand;
        goal.tip = 0.1 * kneel;
        goal.twist = 0.12 * kneel;
        goal.nod = 0.3 * kneel;
        goal.turn = -0.06 * kneel;
        left.reach = hand;
        left.fist = 0;
        left.wrist = 0.3;
        left.twist = 0.5 * hand;
        const touch = Explorer.TOUCH.clone();
        touch.y += 0.012 * Math.sin(2 * Math.PI * 1.4 * Math.max(0, t - 1.6)) * hand;
        left.target.copy(touch).addScaledVector(new THREE.Vector3(0.1, 0.6, -0.8).normalize(), HAND * 0.8);
        right.up.set(0.25, -1, 0.35).normalize();
        right.fore.set(0.2, -0.4, 1).normalize();
        if (t > INTERACT_TIME) this.setMode('idle');
        break;
      }
      case 'sit':
      case 'rise': {
        this.poseRate = 9;
        const k = this.mode === 'sit' ? smooth(t / SIT_TIME) : 1 - smooth(t / RISE_TIME);
        const moving = this.mode === 'sit' ? bump(Math.min(1, t / SIT_TIME)) : bump(Math.min(1, t / RISE_TIME));
        // Its seat on the log: the shorts' underside 5.3 cm below the hip
        // joints. It leans a little forward to put its hands on its thighs
        // by the knees (its arms are short).
        const hips = this.hipsRest.y - (Explorer.SEAT + 0.05);
        goal.back = 0.18 * k;
        goal.sink = hips * k;
        goal.lean = 0.2 * k + 0.25 * moving;
        goal.tip = -0.08 * k;
        goal.nod = -0.1 * k;
        if (this.mode === 'sit' && t > SIT_TIME) look();
        for (const [arm, leg] of [
          [left, this.legs[0]],
          [right, this.legs[1]],
        ] as const) {
          arm.reach = k;
          arm.fist = 0;
          arm.wrist = 0.45;
          leg.knee.getWorldPosition(arm.target);
          this.rig.worldToLocal(arm.target);
          arm.target.x -= leg.side * 0.012;
          arm.target.y += 0.105;
          arm.target.z -= 0.04;
        }
        if (this.mode === 'rise' && t > RISE_TIME) {
          const next = this.queued ?? 'idle';
          this.queued = null;
          this.mode = 'idle';
          this.setMode(next);
        }
        break;
      }
    }
  }

  // ------------------------------------------------------------ update

  update(delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    this.time += dt;
    // A pose waits for it to stop going before it starts.
    const posing = this.mode !== 'idle' && this.mode !== 'walk' && this.mode !== 'run';
    if (!posing || this.moving < 0.1) this.modeTime += dt;
    this.moving = ease(this.moving, this.gait === 'stand' ? 0 : 1, GAIT_RATE, dt);
    this.running = ease(this.running, this.gait === 'run' ? 1 : 0, GAIT_RATE, dt);
    const r = this.running;
    const mix = (a: number, b: number) => a + (b - a) * r;
    const cadence = mix(WALK.cadence, RUN.cadence);
    const duty = mix(WALK.duty, RUN.duty);
    const stride = mix(WALK.stride, RUN.stride) * this.moving;

    // The posture: what it is doing.
    Object.assign(this.goal, REST);
    this.pose(this.goal);
    for (const key of Object.keys(REST) as (keyof Posture)[]) this.posture[key] = ease(this.posture[key], this.goal[key], this.poseRate, dt);
    const k = 1 - Math.exp(-this.poseRate * dt);
    for (const arm of this.arms) {
      const p = arm.pose;
      const g = arm.goal;
      p.up.lerp(g.up, k).normalize();
      p.fore.lerp(g.fore, k).normalize();
      p.wrist += (g.wrist - p.wrist) * k;
      p.twist += (g.twist - p.twist) * k;
      p.fist = g.fist; // a hand opens or closes at once
      p.reach += (g.reach - p.reach) * k;
      p.target.copy(g.target);
    }
    const p = this.posture;

    // The jump's height.
    const t = this.modeTime;
    const air = this.mode === 'jump' && t >= CROUCH && t < CROUCH + AIR;
    if (air) {
      const a = (t - CROUCH) / AIR;
      this.jumpHeight = 4 * JUMP_HEIGHT * a * (1 - a);
    } else this.jumpHeight = 0;
    this.rig.position.y = this.jumpHeight;

    // Going: the feet step through the gait's phase, and it moves itself.
    const active = this.moving > 0.02 && !air && p.free < 0.5;
    if (active) this.phase = (this.phase + cadence * dt) % 1;
    this.speedNow = active ? (stride * cadence) / duty : 0;
    this.advance(dt);

    // The feet: planted, swinging, or held off the ground.
    this.updateMatrix();
    this.rig.updateMatrix();
    const toWorld = m1.multiplyMatrices(this.matrix, this.rig.matrix);
    const toRig = m2.copy(toWorld).invert();
    if (!this.started) {
      this.started = true;
      for (const leg of this.legs) this.touchDown(leg, toWorld, leg.rest);
    }
    const lift = mix(WALK.lift, RUN.lift) * Math.min(1, this.moving * 1.5);
    const heading = this.rotation.y;
    for (const leg of this.legs) {
      const offset = leg.side < 0 ? v1.set(p.lx * -1, p.ly, p.lz) : v1.set(p.rx, p.ry, p.rz);
      const pitch = leg.side < 0 ? p.lp : p.rp;
      if (p.free > 0.5 || air) {
        leg.down = false;
        leg.air = true;
        leg.base.copy(leg.rest);
        leg.target.copy(leg.rest).add(offset);
        leg.yaw = heading + leg.side * TOE_OUT;
        leg.pitch = pitch;
        continue;
      }
      if (leg.air) {
        leg.air = false;
        this.touchDown(leg, toWorld, leg.rest);
      }
      const land = v2.copy(leg.rest);
      land.z += stride / 2;
      let swing = 0;
      if (active) {
        const u = (((this.phase + leg.index * 0.5) % 1) + 1) % 1;
        const wantDown = u < duty;
        if (wantDown && !leg.down) this.touchDown(leg, toWorld, leg.base);
        else if (!wantDown && leg.down) this.liftOff(leg);
        if (!leg.down) leg.q = (u - duty) / (1 - duty);
      } else if (!leg.down) {
        leg.q = Math.min(1, leg.q + dt / SETTLE);
        if (leg.q >= 1) this.touchDown(leg, toWorld, land);
      }
      if (leg.down) {
        leg.base.copy(leg.plant).applyMatrix4(toRig);
        // A foot left far behind (the figure put somewhere else) is set down
        // under it again.
        if (leg.base.distanceTo(leg.rest) > 0.6) this.touchDown(leg, toWorld, leg.rest);
        leg.yaw = leg.plantYaw;
      } else {
        const s = smooth(leg.q);
        leg.base.lerpVectors(leg.from, land, s);
        leg.base.y = leg.rest.y + lift * Math.sin(Math.PI * leg.q);
        leg.yaw = heading + leg.side * TOE_OUT;
        // Toes down as it leaves the ground, up as it reaches to land.
        swing = (0.5 * (1 - leg.q) * Math.sin(Math.PI * leg.q) - 0.35 * leg.q * Math.sin(Math.PI * leg.q)) * Math.min(1, this.moving * 1.5) * (1 + r);
      }
      leg.pitch = pitch + swing;
      leg.target.copy(leg.base).add(offset);
      leg.target.y += footDrop(leg.pitch);
    }

    // The body over its feet: the hips bob, sway over the planted foot and
    // turn with the legs; lowered further as far as the feet need.
    const m = this.moving;
    const bob = (WALK.bob * (1 - r) - RUN.bob * r) * m * Math.cos(4 * Math.PI * (this.phase - duty / 2));
    const sway = mix(WALK.sway, RUN.sway) * m * -Math.cos(2 * Math.PI * (this.phase - duty / 2));
    const [lf, rf] = this.legs;
    const half = Math.max(0.05, stride / 2);
    const legTurn = THREE.MathUtils.clamp((lf.base.z - rf.base.z) / (2 * half), -1, 1);
    const hipTwist = mix(WALK.twist, RUN.twist) * m * legTurn * 0.5;
    const lean = mix(WALK.lean, RUN.lean) * m;
    const breath = 0.004 * Math.sin(this.time * 2.1) * (1 - m);
    let slack = 0;
    for (let pass = 0; pass < 5; pass++) {
      this.body.position.set(this.hipsRest.x + sway, this.hipsRest.y - p.sink - slack + bob + breath, this.hipsRest.z - p.back);
      this.body.rotation.set(p.tip + lean * 0.3, hipTwist, 0);
      this.body.updateMatrix();
      let need = 0;
      for (const leg of this.legs) {
        if (leg.air) continue;
        const hip = v1.copy(leg.hip.position).applyMatrix4(this.body.matrix);
        const d = v2.subVectors(leg.target, hip);
        const most = (THIGH_M + SHIN_M) * REACH_SHARE;
        if (d.length() <= most) continue;
        const flat = Math.hypot(d.x, d.z);
        need = Math.max(need, flat < most ? -d.y - Math.sqrt(most * most - flat * flat) : d.length() - most);
      }
      if (need < 1e-7 || slack > 0.2) break;
      slack += need * 1.01 + 1e-6;
    }

    // The chest leans and turns against the hips; the head keeps looking
    // where it looks.
    this.spine.rotation.set(p.lean + lean * 0.7, p.twist - hipTwist * 1.6, -p.bend);
    this.head.rotation.set(p.nod - 0.6 * (p.tip + p.lean + lean), p.turn - p.twist + hipTwist * 0.6, -p.tilt);
    this.rig.updateMatrixWorld(true);

    // The legs reach their feet.
    const intoHips = m2.copy(this.body.matrix).invert();
    for (const leg of this.legs) this.placeLeg(leg, intoHips);

    // The arms: posed, swinging with the stride, or reaching.
    this.rig.updateMatrixWorld(true);
    const intoChest = m2.multiplyMatrices(this.body.matrix, this.spine.matrix).invert();
    const swingAmount = mix(WALK.swing, RUN.swing) * m;
    const elbow = mix(WALK.elbow, RUN.elbow) * m;
    for (const arm of this.arms) {
      const up = v1.copy(arm.pose.up);
      const fore = v2.copy(arm.pose.fore);
      const tool = arm.side < 0 && this.tool !== 'nothing';
      if (m > 0.01) {
        // Swings against the leg on its own side.
        const other = this.legs[arm.side < 0 ? 1 : 0];
        const swing = THREE.MathUtils.clamp((other.base.z - other.rest.z) / half, -1.2, 1.2) * swingAmount * (tool ? 0.3 : 1);
        up.applyAxisAngle(X_AXIS, -swing);
        if (!tool) {
          const bend = Math.acos(THREE.MathUtils.clamp(arm.pose.up.dot(arm.pose.fore), -1, 1));
          fore.copy(up).applyAxisAngle(X_AXIS, -(bend + elbow * (0.85 + 0.15 * swing)));
        } else fore.applyAxisAngle(X_AXIS, -swing);
      }
      if (arm.pose.reach > 0.001) {
        const shoulder = arm.shoulder.position;
        const target = new THREE.Vector3().copy(arm.pose.target).applyMatrix4(intoChest);
        const [ikUp, ikFore] = solveArm(shoulder, target, UPPER_M, FORE_M, new THREE.Vector3(arm.side * 0.7, -0.2, -0.6));
        up.lerp(ikUp, arm.pose.reach).normalize();
        fore.lerp(ikFore, arm.pose.reach).normalize();
      }
      const bend = armFrame(up, fore, arm.shoulder.quaternion);
      arm.elbow.rotation.set(-bend, 0, 0);
      arm.wrist.rotation.set(-arm.pose.wrist, arm.pose.twist, 0);
      const fist = arm.pose.fist > 0.5 || tool;
      arm.fist.visible = fist;
      arm.open.visible = !fist;
    }
    this.rig.updateMatrixWorld(true);

    // The lantern and the fishing line hang.
    this.lanternSwing.update(dt);
    if (this.tool === 'fishing-rod') this.lineSwing.update(dt);

    // The face, blinking now and then.
    if (this.time > this.blinkAt) {
      this.blinkUntil = this.time + 0.13;
      this.blinks++;
      this.blinkAt = this.time + 2.6 + 2.2 * (0.5 + 0.5 * Math.sin(this.blinks * 12.9898));
    }
    this.face.show(this.face_, this.time < this.blinkUntil);
  }

  private touchDown(leg: LegRig, toWorld: THREE.Matrix4, at: V): void {
    leg.down = true;
    leg.q = 1;
    leg.base.copy(at);
    leg.plant.copy(at);
    leg.plant.y = leg.rest.y;
    leg.plant.applyMatrix4(toWorld);
    leg.plantYaw = this.rotation.y + leg.side * TOE_OUT;
  }

  private liftOff(leg: LegRig): void {
    leg.down = false;
    leg.q = 0;
    leg.from.copy(leg.base);
  }

  // Bends a leg to put its ankle on its target and turns its foot as it
  // should face in the world.
  private placeLeg(leg: LegRig, intoHips: THREE.Matrix4): void {
    const target = v1.copy(leg.target).applyMatrix4(intoHips).sub(leg.hip.position);
    const out = Math.atan2(target.x, -target.y);
    const down = -Math.hypot(target.x, target.y);
    const { upper, lower } = reach(0, 0, down, target.z, THIGH_M, SHIN_M, 1);
    leg.hip.rotation.set(-upper, 0, out);
    leg.knee.rotation.set(upper - lower, 0, 0);
    // The foot: its heading and pitch in the space the figure moves in.
    q1.copy(this.quaternion).multiply(this.rig.quaternion).multiply(this.body.quaternion);
    leg.hip.updateMatrix();
    leg.knee.updateMatrix();
    q1.multiply(leg.hip.quaternion).multiply(leg.knee.quaternion);
    q2.setFromAxisAngle(Y_AXIS, leg.yaw).multiply(new THREE.Quaternion().setFromAxisAngle(X_AXIS, leg.pitch));
    leg.ankle.quaternion.copy(q1.invert().multiply(q2));
  }
}

// The upper arm's turn (into `q`) that points it along `up` with its elbow
// bending toward `fore`, and the elbow's bend, for an arm built straight
// down with its elbow bending forward (+z). A straight arm bends toward
// the front.
function armFrame(up: V, fore: V, q: THREE.Quaternion): number {
  const across = fore.clone().addScaledVector(up, -fore.dot(up));
  const along = across.length();
  const front = new THREE.Vector3(0, 0, 1).addScaledVector(up, -up.z);
  if (front.lengthSq() < 1e-8) front.set(0, 1, 0).addScaledVector(up, -up.y);
  front.normalize();
  const w = THREE.MathUtils.smoothstep(along, 0.02, 0.2);
  const toward = front.multiplyScalar(1 - w).addScaledVector(along > 1e-9 ? across.divideScalar(along) : front, w).normalize();
  const x = new THREE.Vector3().crossVectors(toward, up).normalize();
  const y = up.clone().negate();
  const z = new THREE.Vector3().crossVectors(x, y);
  q.setFromRotationMatrix(new THREE.Matrix4().makeBasis(x, y, z));
  return Math.atan2(along, fore.dot(up));
}

// Two bones from `shoulder` reaching `target` (both in the chest's space),
// the elbow toward `pole`: which way each points.
function solveArm(shoulder: V, target: V, a: number, b: number, pole: V): [V, V] {
  const reachTo = target.clone().sub(shoulder);
  const length = reachTo.length();
  const dir = reachTo.divideScalar(length || 1);
  const d = THREE.MathUtils.clamp(length, Math.abs(a - b) + 1e-4, (a + b) * 0.999);
  const cos = THREE.MathUtils.clamp((a * a + d * d - b * b) / (2 * a * d), -1, 1);
  const sin = Math.sqrt(1 - cos * cos);
  const side = pole.clone().addScaledVector(dir, -pole.dot(dir)).normalize();
  const elbow = shoulder.clone().addScaledVector(dir, a * cos).addScaledVector(side, a * sin);
  const wrist = shoulder.clone().addScaledVector(dir, d);
  return [elbow.clone().sub(shoulder).normalize(), wrist.sub(elbow).normalize()];
}

