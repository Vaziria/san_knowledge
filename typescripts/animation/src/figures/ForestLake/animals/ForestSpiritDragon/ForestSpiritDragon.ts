import * as THREE from 'three';
import { ForestAnimal, type ForestAnimalOptions } from '../ForestAnimal';
import { ease, seededRandom, smooth } from '../parts';
import { Body, BODY_AT } from './Body';
import { Head } from './Head';
import { FRONT_LEG, HIND_LEG, Leg, legRoot } from './Leg';
import { Neck, NECK_AT } from './Neck';
import { aim, dragonMaterials, FOREST } from './parts';
import { Tail, TAIL_AT } from './Tail';
import { Wing, WING_ROOT } from './Wing';

type V = THREE.Vector3;
const v = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z);

// Its calm walk, the four-beat walk of the forest lake's four-legged ones
// (left fore, right fore, left hind, right hind in `offsets`), slow and
// heavy: each foot down 0.7 of a stride, 1 m back under it, half a stride a
// second, so 0.71 m/s.
const WALK = {
  stride: 1.0, // m a planted foot travels back under the body
  cadence: 0.5, // strides a second
  duty: 0.7, // share of a stride each foot is down
  offsets: [0.25, 0.75, 0, 0.5] as const,
  lift: [0.26, 0.22] as const, // m a fore and a hind foot rise as they swing
  curl: [0.55, 0.4] as const, // rad their toes turn down as they swing
  sink: 0.06, // m the body is held lower walking, so its legs reach
  bob: 0.025, // m it rises and falls, twice a stride
  roll: 0.03, // rad it rolls over the planted feet, once a stride
};
const WALK_SPEED = (WALK.stride * WALK.cadence) / WALK.duty;
const GAIT_RATE = 2.5; // how fast it starts and stops walking, a second
const SETTLE = 0.45; // s a foot still in the air takes to come down once it stops

// Flying.
const CRUISE = 8.5; // m/s it flies at
const CLIMB = 3.5; // m/s it climbs at most
const DESCENT = 3; // m/s it comes down at most, gliding
const CRUISE_HEIGHT = 8; // m over the ground it climbs to, taking off, unless given a height
const CROUCH = 0.5; // s it crouches before it springs up
const CROUCH_DEPTH = 0.32; // m it crouches
const LIFT = 4.5; // m/s up it springs, its wings beating down
const PUSH = 0.4; // m it rises before its feet leave the ground
const TAKEOFF = 2.2; // s from the spring until it flies on as in cruise
const SPREAD = 0.95; // rad its wings are lowered about their hinges flying, from raised as it stands
// Its wingbeats (a second, and rad either way): springing up, climbing,
// cruising, braking to land; and held out gliding.
const BEATS = { spring: [1.45, 0.7], climb: [1.2, 0.6], cruise: [0.85, 0.45], brake: [1.5, 0.62], glide: [0.4, 0.04] } as const;
const BANK = 0.7; // rad it banks at most, into a turn
const WING_CLEARANCE = 0.4; // m its wing tips keep over the ground under it as they beat
const TAIL_CLEARANCE = 0.08; // m its tail's underside keeps over the ground
// Landing: it slows evenly to a stand, coming down over at least LAND_TIME
// s, no faster than LAND_SINK m/s at the most, its wings braking and its
// legs reaching for the ground over the last FLARE of the way.
const LAND_TIME = 3.2;
const LAND_SINK = 3.5;
const FLARE = 0.45;
const SETTLE_TIME = 0.9; // s it takes the landing in its legs

// Roaring: it draws its head back, then thrusts it up and roars, its jaws
// wide, and closes them.
const ROAR = { gather: 0.6, roar: 1.8, close: 0.8 };
const ROAR_TIME = ROAR.gather + ROAR.roar + ROAR.close;
const JAW_OPEN = 0.62; // rad

// Its legs folded flying, each ankle's place moved from as built (in the
// body's space) and its foot turned (rad, toes down and back): the forelegs
// drawn up under the chest, the hind legs trailing back under the tail.
const TUCK = {
  front: { ankle: v(-0.06, 0.85, -0.5), toes: 1.6 },
  hind: { ankle: v(-0.05, 0.8, -1.15), toes: 1.7 },
};

type Mode = 'idle' | 'walk' | 'takeoff' | 'fly' | 'land';

interface Foot {
  leg: Leg;
  index: number; // in WALK.offsets
  rest: V; // the middle of its sole as it stands, in the rig's space
  rise: V; // from there to its ankle
  down: boolean;
  plant: V; // where it is planted, in the space the dragon moves in
  from: V; // where it lifted off, in the rig's space
  q: number; // how far through its swing
  sole: V; // where its sole is this frame, in the rig's space
  toes: number; // rad its toes are turned down this frame
  inAir: boolean; // off the ground: folded, or reaching down to land
  air: V; // its ankle in the body's space as it left the ground, to ease from
  airTime: number;
}

const ONE = new THREE.Quaternion();
const X = v(1, 0, 0);
const UP = v(0, 1, 0);

// The forest spirit dragon (ForestSpiritDragon.md), as the user's reference
// sheets draw it: a big, friendly dragon of leaf scales, cream plates, tan
// horns and glowing crystals, 4.5 m to the top of its head and 7.9 m from
// its snout to the end of its tail, standing on all fours with its wings
// raised and half open, as the overview stands it.
//
// Its parts are the overview's modular ones, each its own group with its
// origin where it would turn: the body, and on it the neck (the head on
// its top bone), the two wings, the four legs and the tail. Each part's
// root is sunk in the part it grows from, and scales cover where they meet,
// as the sheets join them. The neck, the tail and the legs bend on bones.
//
// What it does, from its sheet's poses (its spec's Animation Behavior):
// Idle(), Walk() (calmly), Stop(), Fly() (taking off from the ground),
// Land() and Roar(). It moves itself: forward along its own +z at `speed`,
// turned (rotation.y) to steer; flying, it climbs or comes down to
// `flyHeight`, banking into its turns, and lands on `groundHeight`. On the
// ground its feet are planted where they land and stay there while the
// body goes on over them; each leg reaches its foot by inverse kinematics.
//
// Meters. y up, facing +z, its left at -x; the origin on the ground under
// the middle of its body. Every part is BufferGeometry sculpted in code, in
// the sheets' colours, not the theme's.
export class ForestSpiritDragon extends ForestAnimal {
  static readonly WALK_SPEED = WALK_SPEED;
  static readonly CRUISE = CRUISE;
  static readonly CLIMB = CLIMB;
  static readonly DESCENT = DESCENT;
  static readonly ROAR_TIME = ROAR_TIME;

  readonly head: Head;
  readonly neck: Neck;
  readonly body: Body;
  readonly leftWing: Wing;
  readonly rightWing: Wing;
  readonly leftFrontLeg: Leg;
  readonly rightFrontLeg: Leg;
  readonly leftHindLeg: Leg;
  readonly rightHindLeg: Leg;
  readonly tail: Tail;

  // The height it flies at, in its parent's space (its own y): it climbs or
  // comes down to it. Below the ground, it flies CRUISE_HEIGHT m over it.
  flyHeight = -Infinity;
  // The ground's height under it, in its parent's space: where it lands,
  // and what it springs from.
  groundHeight = 0;
  // The ground's height at a point, in its parent's space, when it isn't
  // level: each foot is set down on it, and its tail kept clear of it.
  // Without it, its feet stand level with it (the preview's floor).
  ground: ((x: number, z: number) => number) | null = null;

  private mode: Mode = 'idle';
  private afterLanding: 'idle' | 'walk' = 'idle';
  private modeTime = 0;
  private time = 0;
  private lastYaw = 0;
  private started = false;
  private readonly feet: Foot[];
  private readonly tailWays: V[];
  private tailLow: { mesh: THREE.SkinnedMesh; index: number }[] | null = null; // its tail's underside (liftTail)
  // Eased as it goes.
  private moving = 0; // 0 standing, 1 walking
  private phase = 0; // through its stride
  private air = 0; // 0 standing as on the ground, 1 in the air: wings out, legs folded, tail behind
  private tuck = 0; // its legs folded, 0..1
  private beat = { rate: 0, amp: 0, base: 0, phase: 0 };
  private bank = 0;
  private pitch = 0;
  private lifted = false; // in a take-off, sprung up
  private landing: { time: number; speed: number; height: number } | null = null;
  private settle = -1; // s since it touched down, or -1
  private roarTime = -1; // s into a roar, or -1
  private readonly neckPose = { rear: 0, turn: 0, nod: 0, tilt: 0, jaw: 0 };

  constructor(options: ForestAnimalOptions = {}) {
    super({ forest: FOREST }, options);
    const seed = options.seed ?? 5;
    const palette = this.palette;
    const materials = dragonMaterials();
    // Each part has a seed of its own, so a change to one leaves the
    // others as they were; the head's is the dragon's own.
    const random = (part: number) => seededRandom(seed + 7919 * part);
    const mirror = (p: V) => v(-p.x, p.y, p.z);
    // Each part placed on the body, as the dragon stands.
    const onBody = <T extends THREE.Object3D>(part: T, at: V): T => {
      part.position.copy(at).sub(BODY_AT);
      this.body.add(part);
      return part;
    };

    this.head = new Head(palette, seededRandom(seed), materials);
    this.neck = new Neck(palette, random(1), materials);
    this.body = new Body(palette, random(2), materials);
    this.body.position.copy(BODY_AT);
    this.rig.add(this.body);
    onBody(this.neck, NECK_AT);
    this.neck.top.add(this.head);
    this.rightWing = onBody(new Wing(1, palette, random(3), materials), WING_ROOT);
    this.leftWing = onBody(new Wing(-1, palette, random(4), materials), mirror(WING_ROOT));
    this.rightFrontLeg = onBody(new Leg(FRONT_LEG, 1, palette, random(5), materials), legRoot(FRONT_LEG, 1));
    this.leftFrontLeg = onBody(new Leg(FRONT_LEG, -1, palette, random(6), materials), legRoot(FRONT_LEG, -1));
    this.rightHindLeg = onBody(new Leg(HIND_LEG, 1, palette, random(7), materials), legRoot(HIND_LEG, 1));
    this.leftHindLeg = onBody(new Leg(HIND_LEG, -1, palette, random(8), materials), legRoot(HIND_LEG, -1));
    this.tail = onBody(new Tail(palette, random(9), materials), TAIL_AT);
    this.tailWays = this.tail.ways;
    this.rig.traverse((o) => {
      if (o instanceof THREE.Mesh && o.geometry.userData.tones) this.painting(o);
    });

    const legs = [this.leftFrontLeg, this.rightFrontLeg, this.leftHindLeg, this.rightHindLeg];
    this.feet = legs.map((leg, index) => {
      const root = legRoot(leg.spec, leg.side);
      const rest = leg.paw.clone().add(root);
      return {
        leg,
        index,
        rest,
        rise: leg.ankle.clone().sub(leg.paw),
        down: true,
        plant: new THREE.Vector3(),
        from: rest.clone(),
        q: 1,
        sole: rest.clone(),
        toes: 0,
        inAir: false,
        air: new THREE.Vector3(),
        airTime: 0,
      };
    });
    this.update(0);
  }

  // What it is doing: idle, walk, takeoff, fly or land.
  get doing(): Mode {
    return this.mode;
  }

  // Whether it is off the ground.
  get flying(): boolean {
    return this.mode === 'fly' || this.mode === 'land' || (this.mode === 'takeoff' && this.lifted);
  }

  get roaring(): boolean {
    return this.roarTime >= 0;
  }

  // How far ahead of where it is it would touch down, landing now (Land()):
  // it slows evenly to a stand as it comes down.
  get landingDistance(): number {
    return (this.speedNow * landingTime(this.position.y - this.groundHeight)) / 2;
  }

  // Stands still, breathing, now and then looking about. Flying, it lands
  // and stands. Named as in its sheet's poses.
  Idle(): void {
    this.afterLanding = 'idle';
    if (this.mode === 'land') return;
    if (this.flying) return this.Land();
    this.setMode('idle');
  }

  // Walks calmly forward the way it faces until told otherwise. Flying, it
  // lands first. Named as in its sheet's poses.
  Walk(): void {
    if (this.flying) {
      this.afterLanding = 'walk';
      return this.Land();
    }
    this.setMode('walk');
  }

  // Stands still where it is (beyond its spec, as the other animals' Stop()).
  Stop(): void {
    this.Idle();
  }

  // Takes off from the ground, springing up with its wings beating, and
  // flies on forward the way it faces, climbing to `flyHeight`; turn it to
  // steer. Landing, it flies on again. Named as in its sheet's poses.
  Fly(): void {
    if (this.mode === 'fly' || this.mode === 'takeoff') return;
    if (this.mode === 'land') {
      this.landing = null;
      this.setMode('fly');
      return;
    }
    this.lifted = false;
    this.setMode('takeoff');
  }

  // Comes down to the ground ahead (`groundHeight`), slowing evenly to a
  // stand as it comes, its wings braking and its legs reaching for the
  // ground, and takes the landing in its legs (landingDistance says how
  // far ahead). On the ground, it stands. Named as in its sheet's poses.
  Land(): void {
    if (this.mode === 'land') return;
    if (!this.flying) {
      if (this.mode === 'takeoff') this.setMode('idle');
      return;
    }
    this.landing = { time: landingTime(this.position.y - this.groundHeight), speed: this.speedNow, height: Math.max(0, this.position.y - this.groundHeight) };
    this.setMode('land');
  }

  // Roars: draws its head back, then thrusts it up and roars, its jaws
  // wide, ROAR_TIME s in all, on the ground or in the air, going on with
  // what it was doing. Named as in its sheet's poses.
  Roar(): void {
    if (this.roarTime < 0) this.roarTime = 0;
  }

  // Puts it in the air where it is, flying on at its cruising speed as if
  // it took off long ago, its wings out and its legs folded: for a scene
  // that opens with it on the wing (the forest lake meeting). Beyond its
  // spec's behaviours.
  startFlying(): void {
    this.setMode('fly');
    this.lifted = true;
    this.landing = null;
    this.afterLanding = 'idle';
    this.speedNow = CRUISE;
    this.climbNow = 0;
    this.moving = 0;
    this.air = 1;
    this.tuck = 1;
    Object.assign(this.beat, { rate: BEATS.cruise[0], amp: BEATS.cruise[1], base: SPREAD });
    this.neckPose.rear = -0.5;
    for (const f of this.feet) {
      f.inAir = true;
      f.down = false;
      f.airTime = 1;
      f.air.copy(f.rest).add(f.rise).applyMatrix4(_restToBody);
    }
    this.lastYaw = this.rotation.y;
    this.update(0);
  }

  private setMode(mode: Mode): void {
    if (mode !== this.mode) this.modeTime = 0;
    this.mode = mode;
  }

  update(delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    this.time += dt;
    this.modeTime += dt;
    if (this.roarTime >= 0) {
      this.roarTime += dt;
      if (this.roarTime > ROAR_TIME) this.roarTime = -1;
    }
    // How fast it is being turned (it is steered), to bank into the turn.
    const turning = dt > 0 ? THREE.MathUtils.clamp(wrap(this.rotation.y - this.lastYaw) / dt, -3, 3) : 0;
    this.lastYaw = this.rotation.y;
    if (this.flyHeight < this.groundHeight) this.flyHeight = this.groundHeight + CRUISE_HEIGHT;

    // ------------------------------------------------ going
    let crouch = 0; // 0..1, crouching to spring
    let flare = 0; // 0..1, braking to land
    let level = 0; // 0..1, levelling out at the end of it, to come down on all four feet
    let onGround = true;
    const climbTo = () => THREE.MathUtils.clamp(1.2 * (this.flyHeight - this.position.y), -DESCENT, CLIMB);
    switch (this.mode) {
      case 'idle':
      case 'walk': {
        this.moving = ease(this.moving, this.mode === 'walk' ? 1 : 0, GAIT_RATE, dt);
        this.speedNow = this.moving > 0.02 ? WALK_SPEED * this.moving : 0;
        this.climbNow = 0;
        this.air = ease(this.air, 0, 2.2, dt);
        this.tuck = ease(this.tuck, 0, 3, dt);
        break;
      }
      case 'takeoff': {
        const t = this.modeTime;
        this.moving = ease(this.moving, 0, GAIT_RATE, dt);
        if (t < CROUCH) {
          crouch = smooth(t / CROUCH);
          this.speedNow = ease(this.speedNow, 0, 4, dt);
        } else {
          if (!this.lifted) {
            // Sprung up on a downstroke of its wings.
            this.lifted = true;
            this.climbNow = LIFT;
            this.beat.phase = 0.8;
          }
          crouch = 1 - smooth((t - CROUCH) / 0.3);
          this.speedNow = ease(this.speedNow, CRUISE, 0.75, dt);
          this.climbNow = ease(this.climbNow, Math.max(climbTo(), 0.5), 1.1, dt);
          onGround = this.position.y < this.groundHeight + PUSH;
          if (!onGround) {
            this.air = ease(this.air, 1, 2.5, dt);
            this.tuck = ease(this.tuck, 1, 2.5, dt);
          }
          if (t > CROUCH + TAKEOFF) this.setMode('fly');
        }
        break;
      }
      case 'fly': {
        onGround = false;
        this.speedNow = ease(this.speedNow, CRUISE, 1, dt);
        this.climbNow = ease(this.climbNow, climbTo(), 1.5, dt);
        this.air = ease(this.air, 1, 2.5, dt);
        this.tuck = ease(this.tuck, 1, 2.5, dt);
        break;
      }
      case 'land': {
        onGround = false;
        const plan = this.landing!;
        const s = Math.min(1, this.modeTime / plan.time);
        this.speedNow = plan.speed * (1 - s);
        this.climbNow = 0;
        flare = smooth((s - (1 - FLARE)) / FLARE);
        level = smooth((s - 0.75) / 0.2);
        this.air = ease(this.air, 1 - 0.35 * flare, 3, dt);
        this.tuck = ease(this.tuck, 1 - flare, 4, dt);
        if (s >= 1) {
          this.touchDown();
          onGround = true;
        }
        break;
      }
    }
    this.advance(dt);
    if (this.mode === 'land' && this.landing) {
      const s = Math.min(1, this.modeTime / this.landing.time);
      this.position.y = this.groundHeight + this.landing.height * (1 - s * s * (3 - 2 * s));
    } else if (this.mode === 'takeoff' && !this.lifted) {
      this.position.y = this.groundHeight;
    }
    const airborne = !onGround;
    if (this.settle >= 0) {
      this.settle += dt;
      if (this.settle > SETTLE_TIME) this.settle = -1;
    }

    // ------------------------------------------------ the wings
    const roar = this.roarShape();
    const climbing = this.climbNow / CLIMB;
    let [rate, amp]: readonly [number, number] = BEATS.cruise;
    let base = SPREAD;
    if (!this.flying && this.mode !== 'land') {
      [rate, amp] = [0.4, 0];
      base = this.mode === 'takeoff' ? -0.3 * crouch : -0.16 * roar.open;
    } else if (this.mode === 'takeoff') {
      [rate, amp] = BEATS.spring;
    } else if (this.mode === 'land') {
      [rate, amp] = flare > 0.01 ? BEATS.brake : BEATS.glide;
      base = SPREAD - 0.2 * flare;
    } else if (climbing > 0.3) {
      [rate, amp] = BEATS.climb;
    } else if (climbing < -0.2 || roar.open > 0.1) {
      [rate, amp] = BEATS.glide;
    }
    const b = this.beat;
    b.rate = ease(b.rate, rate, 2, dt);
    b.amp = ease(b.amp, amp, 3, dt);
    b.base = ease(b.base, base, 3, dt);
    b.phase = (b.phase + b.rate * dt) % 1;

    // ------------------------------------------------ the body
    // Banked into its turns and pitched as it climbs, flying; rolling over
    // its feet and rising and falling with its steps, walking.
    const flyingPitch = -0.6 * Math.atan2(this.climbNow, Math.max(this.speedNow, 3)) - 0.32 * (flare - level);
    this.bank = ease(this.bank, airborne ? THREE.MathUtils.clamp(-Math.atan((this.speedNow * turning) / 9.81), -BANK, BANK) * (1 - flare) : 0, 2.5, dt);
    this.pitch = ease(this.pitch, airborne ? flyingPitch : -0.08 * roar.open, this.mode === 'land' ? 8 : 3, dt);
    if (this.moving > 0.02 && onGround && this.mode !== 'takeoff') this.phase = (this.phase + WALK.cadence * dt) % 1;
    const cycle = 2 * Math.PI * this.phase;
    const m = this.moving * (1 - this.air);
    const settleDip = this.settle >= 0 ? Math.sin(Math.PI * Math.min(1, this.settle / SETTLE_TIME)) : 0;
    const breath = 0.008 * Math.sin(this.time * 1.7) * (1 - m) * (1 - this.air);
    let lower = WALK.sink * m - WALK.bob * Math.cos(2 * cycle) * m + CROUCH_DEPTH * crouch + 0.16 * settleDip - breath - 0.05 * roar.open * (1 - this.air);
    lower += 0.09 * b.amp * Math.cos(2 * Math.PI * b.phase) * this.air;
    const roll = WALK.roll * Math.sin(cycle) * m + this.bank;
    this.body.rotation.set(this.pitch, 0, roll);
    this.body.position.set(BODY_AT.x, BODY_AT.y - lower, BODY_AT.z);

    // Each wing beats down only as far as leaves its tips clear of the
    // ground: springing up and coming down to land, a full downstroke would
    // take them into it.
    const stroke = Math.sin(2 * Math.PI * b.phase);
    this.updateMatrix();
    this.rig.updateMatrix();
    this.body.updateMatrix();
    const wingToWorld = new THREE.Matrix4().multiplyMatrices(this.matrix, this.rig.matrix).multiply(this.body.matrix);
    for (const wing of [this.leftWing, this.rightWing]) wing.lower(this.wingRoom(wing, b.base + b.amp * stroke, wingToWorld));

    // ------------------------------------------------ the neck and head
    const p = this.neckPose;
    const look = 0.3 * Math.sin(this.time * 0.29) * smooth(Math.sin(this.time * 0.17) * 1.5) * (1 - this.air) * (1 - m);
    const nodWalk = 0.04 * Math.cos(2 * cycle) * m;
    p.rear = ease(p.rear, -0.5 * this.air + 0.3 * roar.gather - 0.12 * roar.open + 0.15 * crouch - 0.08 * m, 4, dt);
    p.turn = ease(p.turn, look + 0.06 * Math.sin(cycle) * m - 0.5 * this.bank * this.air, 3, dt);
    // The head kept level as the neck reaches: its own nod undoes the
    // body's pitch and the neck's bend, then nods with the steps and lifts
    // to roar.
    const headLevel = -this.pitch * this.air + 0.25 * p.rear;
    p.nod = ease(p.nod, headLevel + nodWalk + 0.22 * roar.gather - 0.6 * roar.open + 0.12 * flare, 5, dt);
    p.tilt = ease(p.tilt, -0.6 * this.bank * this.air, 3, dt);
    p.jaw = ease(p.jaw, 0.12 * roar.gather + JAW_OPEN * roar.open, 8, dt);
    const shake = 0.05 * roar.open * Math.sin(this.time * 38);
    this.neck.bend(p.rear, p.turn + shake, p.nod, p.tilt);
    this.head.jaw.rotation.x = p.jaw;

    // ------------------------------------------------ the tail
    // As it stands it falls to the ground and sweeps round to its left; it
    // streams out straight behind in the air. It sways, a wave running down
    // it to the tip.
    const n = this.tailWays.length;
    const sway = (0.05 + 0.08 * m + 0.06 * this.air) * (1 - 0.5 * roar.open);
    const straight = v(0, -0.2, -1).normalize();
    const turn = new THREE.Quaternion();
    const ways = this.tailWays.map((rest, i) => {
      const way = rest.clone().applyQuaternion(turn.setFromUnitVectors(rest, straight).slerp(ONE, 1 - this.air));
      const along = (i + 1) / n;
      way.applyAxisAngle(UP, sway * along * Math.sin(this.time * (1.1 + 0.6 * this.air) - 0.7 * i) - 0.6 * this.bank * this.air * along);
      way.applyAxisAngle(X, 0.06 * this.air * along * Math.sin(2 * Math.PI * b.phase - 0.8 * i) + 0.08 * flare * along);
      return way;
    });
    aim(this.tail.chain, ways);

    // ------------------------------------------------ the legs
    this.updateMatrix();
    this.rig.updateMatrix();
    const toWorld = new THREE.Matrix4().multiplyMatrices(this.matrix, this.rig.matrix);
    const toRig = toWorld.clone().invert();
    // The ground under a point of the rig's space, in the rig's space.
    const under = (point: V) => {
      const w = point.clone().applyMatrix4(toWorld);
      w.y = this.ground ? this.ground(w.x, w.z) : this.groundHeight;
      return w.applyMatrix4(toRig).y;
    };
    // Planted where its sole is, on the ground there.
    const plant = (f: Foot) => {
      f.down = true;
      f.q = 1;
      f.plant.copy(f.sole).setY(under(f.sole)).applyMatrix4(toWorld);
    };
    if (!this.started) {
      this.started = true;
      for (const f of this.feet) {
        f.sole.copy(f.rest);
        plant(f);
      }
    }
    this.body.updateMatrix();
    const toBody = this.body.matrix.clone().invert();
    const bodyTurn = this.body.quaternion.clone().invert();
    const stride = WALK.stride * this.moving;
    const reachDown = this.mode === 'land' && this.landing ? smooth((this.modeTime / this.landing.time - 0.72) / 0.28) : 0;
    if (reachDown > 0) {
      // Its feet reaching for the ground, the body lowered over them as far
      // as they need, as it is over its feet on the ground.
      for (const f of this.feet) f.sole.copy(f.rest).setY(under(f.rest));
      const need = Math.max(...this.feet.map((f) => this.shortfall(f)));
      if (need > 0) {
        this.body.position.y -= need * reachDown;
        this.body.updateMatrix();
        toBody.copy(this.body.matrix).invert();
      }
    }
    for (const f of this.feet) {
      const front = f.leg.spec.front;
      if (airborne) {
        // Folded under it, or reaching down to land; eased from where it
        // left the ground.
        if (!f.inAir) {
          f.inAir = true;
          f.down = false;
          f.air.copy(f.sole).add(f.rise).applyMatrix4(toBody);
          f.airTime = 0;
        }
        f.airTime += dt;
        const tuck = front ? TUCK.front : TUCK.hind;
        const ankle = f.rest.clone().add(f.rise).applyMatrix4(_restToBody);
        ankle.addScaledVector(v(tuck.ankle.x * f.leg.side, tuck.ankle.y, tuck.ankle.z), this.tuck);
        ankle.lerp(f.air, 1 - smooth(f.airTime / 0.35));
        f.toes = tuck.toes * this.tuck;
        const foot = new THREE.Quaternion().setFromAxisAngle(X, f.toes);
        // Coming down to land, it reaches for the ground under where it will
        // stand, its paw level with it, so it is planted as it touches down.
        const flat = smooth(2 * reachDown);
        if (reachDown > 0) {
          const below = f.rest.clone();
          below.y = under(below);
          ankle.lerp(below.add(f.rise).applyMatrix4(toBody), reachDown);
          foot.slerp(bodyTurn, flat);
        }
        // Never into the ground, its claws neither.
        f.sole.copy(ankle).applyMatrix4(this.body.matrix).sub(f.rise);
        const floor = under(f.sole) + f.leg.toesDrop(f.toes * (1 - flat));
        if (f.sole.y < floor) {
          f.sole.y = floor;
          ankle.copy(f.sole).add(f.rise).applyMatrix4(toBody);
        }
        f.leg.reach(ankle.sub(f.leg.position), foot);
        continue;
      }
      if (f.inAir) {
        // Down from the air: planted where it is.
        f.inAir = false;
        plant(f);
      }
      // Where a swinging foot comes down: as far ahead as the stride, on the
      // ground there.
      const land = f.rest.clone();
      land.z += stride / 2;
      land.y = under(land);
      const walking = this.moving > 0.02 && this.mode !== 'takeoff';
      if (walking) {
        const u = (((this.phase + WALK.offsets[f.index]) % 1) + 1) % 1;
        const wantDown = u < WALK.duty;
        if (wantDown && !f.down) plant(f);
        else if (!wantDown && f.down) {
          f.down = false;
          f.q = 0;
          f.from.copy(f.sole);
        }
        if (!f.down) f.q = (u - WALK.duty) / (1 - WALK.duty);
      } else if (!f.down) {
        f.q = Math.min(1, f.q + dt / SETTLE);
        if (f.q >= 1) plant(f);
      }
      if (f.down) {
        f.sole.copy(f.plant).applyMatrix4(toRig);
        // A foot left far behind (the dragon put somewhere else) is set
        // down under it again.
        if (f.sole.distanceTo(f.rest) > 3) {
          f.sole.copy(f.rest);
          plant(f);
        }
        f.toes = 0;
      } else {
        const arc = Math.sin(Math.PI * f.q);
        f.sole.lerpVectors(f.from, land, smooth(f.q));
        f.toes = WALK.curl[front ? 0 : 1] * arc;
        f.sole.y += WALK.lift[front ? 0 : 1] * arc * Math.min(1, 1.5 * this.moving + 0.3) + f.leg.toesDrop(f.toes);
      }
    }
    if (!airborne) {
      // The body over its feet, lowered as far as they need to reach them.
      const need = Math.max(...this.feet.map((f) => this.shortfall(f)));
      if (need > 0) {
        this.body.position.y -= need;
        this.body.updateMatrix();
        toBody.copy(this.body.matrix).invert();
      }
      for (const f of this.feet) {
        const ankle = f.sole.clone().add(f.rise).applyMatrix4(toBody);
        const foot = bodyTurn.clone().multiply(new THREE.Quaternion().setFromAxisAngle(X, f.toes));
        f.leg.reach(ankle.sub(f.leg.position), foot);
      }
    }
    // Last, once the body is where its feet hold it.
    this.liftTail();
  }

  // The furthest a wing may be lowered toward `angle` (about its hinge) with
  // its tips WING_CLEARANCE m over the ground; `toWorld` takes the body's
  // space to its parent's.
  private wingRoom(wing: Wing, angle: number, toWorld: THREE.Matrix4): number {
    const floor = this.groundHeight + WING_CLEARANCE;
    const turn = new THREE.Quaternion();
    const p = new THREE.Vector3();
    const lowest = (a: number) => {
      turn.setFromAxisAngle(wing.hinge, wing.side * a);
      let low = Infinity;
      for (const tip of wing.tips) low = Math.min(low, p.copy(tip).applyQuaternion(turn).add(wing.position).applyMatrix4(toWorld).y);
      return low;
    };
    if (lowest(angle) >= floor) return angle;
    // Raised step by step until clear, then halved back toward it.
    let high = angle;
    let low = angle;
    for (let i = 0; i < 40 && lowest(high) < floor; i++) {
      low = high;
      high -= 0.08;
    }
    for (let i = 0; i < 8; i++) {
      const middle = (high + low) / 2;
      if (lowest(middle) >= floor) high = middle;
      else low = middle;
    }
    return high;
  }

  // How far a foot on the ground is out of its leg's reach, as the body is
  // held: how much lower the body must be.
  private shortfall(f: Foot): number {
    const hip = f.leg.position.clone().applyMatrix4(this.body.matrix);
    const ankle = f.sole.clone().add(f.rise);
    const across = Math.hypot(ankle.x - hip.x, ankle.z - hip.z);
    const most = f.leg.reachMost * 0.995;
    const height = Math.sqrt(Math.max(0, most * most - across * across));
    return Math.max(0, hip.y - ankle.y - height);
  }

  // Lifts its tail at its root as far as keeps its underside (and its
  // plume's) TAIL_CLEARANCE m over the ground: on ground rising behind it,
  // or coming down to land, it would reach into it.
  private liftTail(): void {
    const ground = this.ground ?? (() => this.groundHeight);
    const root = this.tail.chain.bones[0];
    this.tailLow ??= this.tail.lowest();
    const p = new THREE.Vector3();
    const lift = new THREE.Quaternion().setFromAxisAngle(X, 0.025);
    this.updateMatrixWorld(true);
    // In its parent's space, where the ground is given.
    const toParent = this.parent ? this.parent.matrixWorld.clone().invert() : new THREE.Matrix4();
    for (let i = 0; i < 30; i++) {
      let low = Infinity;
      for (const { mesh, index } of this.tailLow) {
        mesh.getVertexPosition(index, p).applyMatrix4(mesh.matrixWorld).applyMatrix4(toParent);
        low = Math.min(low, p.y - ground(p.x, p.z));
      }
      if (low >= TAIL_CLEARANCE) break;
      root.quaternion.premultiply(lift);
      this.tail.updateMatrixWorld(true);
    }
  }

  // It is down: on the ground, taking the landing in its legs, each foot
  // planted under it as it stands.
  private touchDown(): void {
    this.landing = null;
    this.speedNow = 0;
    this.climbNow = 0;
    this.position.y = this.groundHeight;
    this.settle = 0;
    this.setMode(this.afterLanding);
    this.afterLanding = 'idle';
  }

  // How far through a roar it is: gathering itself (drawing its head back),
  // and roaring (head up, jaws wide); 0 when it isn't.
  private roarShape(): { gather: number; open: number } {
    const t = this.roarTime;
    if (t < 0) return { gather: 0, open: 0 };
    const gather = smooth(t / ROAR.gather) * (1 - smooth((t - ROAR.gather) / 0.35));
    const open = smooth((t - ROAR.gather + 0.1) / 0.35) * (1 - smooth((t - ROAR.gather - ROAR.roar) / ROAR.close));
    return { gather, open };
  }
}

// From the rig's space to the body's, as it stands.
const _restToBody = new THREE.Matrix4().makeTranslation(-BODY_AT.x, -BODY_AT.y, -BODY_AT.z);

// How long a landing from `height` m over the ground takes.
function landingTime(height: number): number {
  return Math.max(LAND_TIME, (1.5 * Math.max(0, height)) / LAND_SINK);
}

function wrap(angle: number): number {
  return Math.atan2(Math.sin(angle), Math.cos(angle));
}
