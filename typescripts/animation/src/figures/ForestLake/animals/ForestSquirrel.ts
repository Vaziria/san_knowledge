import * as THREE from 'three';
import type { ForestAnimalOptions } from './ForestAnimal';
import { FourLegged, legRings, type Foot, type FourLeggedBuild, type FreeFoot, type Gait, type Posture } from './FourLegged';
import { blend, ear, ellipsoid, eye, gloss, loft, patchy, rigidMesh, Sculpt, seededRandom, smooth, spike, tone, tubeRings, type Palette } from './parts';

// The forest lake's squirrel (ForestSquirrel.md), as its reference sheet
// draws it: a red squirrel of flat facets, bright orange-red, with a white
// chest and belly, a pale muzzle, tufted ears, big dark eyes, dark little
// hands and feet, and a huge plume of a tail curled up its back in an S,
// paler at its tip. At rest it sits up on its haunches holding an acorn in
// its hands, as the sheet draws it; it walks, bounds, leaps and climbs on
// all fours. Its colours are the sheet's five variations.
//
// Meters; it faces +z, its left is -x, and its origin is on the ground
// under its middle. Built in centimeters (U), from the sheet's views: 20 cm
// from nose to rump and a tail as long again, as a red squirrel is.

const U = 0.01; // m per centimeter

function P(z: number, y: number, x = 0): THREE.Vector3 {
  return new THREE.Vector3(x * U, y * U, z * U);
}

// The sheet's five colour variations, picked from its pictures.
const VARIATIONS: Record<string, Palette> = {
  brown: { coat: 0xef7b3a, dark: 0xcc5a33, cream: 0xf5e8e2, inner: 0xf3c4ac, paws: 0x6a3e2c, tuft: 0x7a3a24, nose: 0x3a2420, tip: 0xfecba0, iris: 0x2a1812, pupil: 0x120b09, shine: 0xffffff, nut: 0xc68a48, cap: 0x7a4a28 },
  gray: { coat: 0x8f8488, dark: 0x6e6468, cream: 0xeeeaea, inner: 0xd8c6c4, paws: 0x5a4a48, tuft: 0x4a3e3e, nose: 0x2a2224, tip: 0xd6d0d0, iris: 0x2a1812, pupil: 0x120b09, shine: 0xffffff, nut: 0xc68a48, cap: 0x7a4a28 },
  red: { coat: 0xd8472a, dark: 0xaa3420, cream: 0xf6e6e0, inner: 0xf0b8a6, paws: 0x6a2c1e, tuft: 0x6a2a1a, nose: 0x3a2020, tip: 0xf7b8a0, iris: 0x2a1812, pupil: 0x120b09, shine: 0xffffff, nut: 0xc68a48, cap: 0x7a4a28 },
  black: { coat: 0x3e3230, dark: 0x2c2426, cream: 0x8a7c7a, inner: 0x6a5654, paws: 0x221a1a, tuft: 0x1c1616, nose: 0x161212, tip: 0x5a4e4c, iris: 0x1a100c, pupil: 0x0c0806, shine: 0xffffff, nut: 0xc68a48, cap: 0x7a4a28 },
  white: { coat: 0xfbf1ec, dark: 0xe6d8d2, cream: 0xffffff, inner: 0xf4c8c0, paws: 0xe8d4cc, tuft: 0xe0ccc4, nose: 0xd0a0a0, tip: 0xffffff, iris: 0x2a1812, pupil: 0x120b09, shine: 0xffffff, nut: 0xc68a48, cap: 0x7a4a28 },
};

const UPRIGHT = new THREE.Vector3(0, 0, 1);

const WALK: Gait = {
  stride: 0.035,
  cadence: 2.4,
  duty: 0.55,
  offsets: [0.25, 0.75, 0, 0.5],
  lift: [0.015, 0.018],
  fold: [0.6, 0.6],
  sink: 0.003,
  bob: 0.004,
  bounces: 2,
  rock: 0.03,
  flex: 0.05,
  nod: 0.05,
  lag: 0.1,
};

const RUN: Gait = {
  // Bounding, the hind feet landing ahead of where the fore feet were.
  stride: 0.06,
  cadence: 4.2,
  duty: 0.3,
  offsets: [0.5, 0.55, 0, 0.04],
  lift: [0.03, 0.035],
  fold: [0.8, 0.8],
  sink: 0.008,
  bob: 0.015,
  bounces: 1,
  rock: 0.12,
  flex: 0.2,
  nod: 0.04,
  lag: 0.35,
};

const BUILD: FourLeggedBuild = {
  variations: VARIATIONS,
  pelvis: P(-6, 8),
  chest: P(5, 8.2),
  neck: P(8.5, 9.6),
  head: P(10, 11.2),
  headScale: 1.3,
  body(s: Sculpt) {
    // Long and low, with round haunches.
    const slices = [
      { z: -9.8, y: 8.2, w: 2.00, up: 1.89, down: 2.25 },
      { z: -8.6, y: 8.2, w: 3.88, up: 3.66, down: 4.50 },
      { z: -6, y: 8.2, w: 4.25, up: 4.01, down: 4.88 },
      { z: -2.5, y: 8.4, w: 3.75, up: 3.66, down: 4.13 },
      { z: 1, y: 8.5, w: 3.50, up: 3.54, down: 3.88 },
      { z: 4.5, y: 8.8, w: 3.38, up: 3.42, down: 4.00 },
      { z: 7, y: 9.6, w: 2.88, up: 2.95, down: 3.50 },
      { z: 9.2, y: 10.8, w: 2.38, up: 2.48, down: 2.75 },
    ];
    loft(
      s,
      slices.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U, normal: UPRIGHT })),
      {
        sides: 8,
        start: 0.8 * U,
        end: 'flat',
        paint: (r, angle, k) => {
          const fromBelow = Math.abs(angle - Math.PI);
          if (fromBelow < 1.05) return tone('cream'); // the white belly and chest
          return patchy('coat', 'dark', r, k, 0.35, 0.3);
        },
      },
    );
  },
  buildHead(s: Sculpt, shiny: Sculpt) {
    const rings = [
      { z: 9.3, y: 12, w: 1.8, up: 1.7, down: 1.8 },
      { z: 10.8, y: 12.2, w: 2.3, up: 2.1, down: 2.1 },
      { z: 12.3, y: 12, w: 2.2, up: 1.9, down: 2 },
      { z: 13.8, y: 11.5, w: 1.55, up: 1.35, down: 1.5 },
      { z: 15, y: 11.1, w: 0.95, up: 0.8, down: 0.95 },
    ];
    loft(
      s,
      rings.map((r) => ({ at: P(r.z, r.y), w: r.w * U, up: r.up * U, down: r.down * U })),
      {
        sides: 8,
        turn: Math.PI / 8,
        start: 'flat',
        end: 0.55 * U,
        paint: (r, angle, k) => {
          const top = Math.min(angle, 2 * Math.PI - angle);
          if (r >= 2 && top > 1.2) return tone('cream'); // the pale muzzle and cheeks
          if (top > 2.2) return tone('cream');
          return patchy('coat', 'dark', r, k, 0.3);
        },
      },
    );
    // Big dark eyes, a little forward on the sides of the head.
    for (const side of [-1, 1]) {
      const look = new THREE.Vector3(side * 0.8, 0.12, 0.58).normalize();
      const at = s.onto(P(12.6, 12.5, side * 6), new THREE.Vector3(-side, 0, 0)).addScaledVector(look, -0.08 * U);
      eye(shiny, at, look, new THREE.Vector3(0, 1, 0), 0.6 * U, { lid: tone('pupil'), iris: tone('iris'), pupil: tone('pupil'), shine: tone('shine') }, 8, 0.62, 0.55, 1.12);
    }
    // Tufted ears: pointed, pale inside, a dark tuft at each tip.
    for (const side of [-1, 1]) {
      ear(s, P(10.6, 13.8, side * 1.3), P(10, 17, side * 1.9), new THREE.Vector3(side, 0, -0.2), new THREE.Vector3(side * 0.3, 0.1, 1), 1.8 * U, 0.5 * U, 0.3, tone('coat', 0.95), tone('inner'));
      spike(s, P(10.1, 16.6, side * 1.65), P(9.8, 16.6, side * 2.05), P(9.4, 18.2, side * 2.1), new THREE.Vector3(side * 0.08 * U, 0, -0.08 * U), tone('tuft'), tone('tuft', 0.8));
    }
    // A small dark nose.
    ellipsoid(shiny, P(15.5, 11.25), new THREE.Vector3(0.35, 0.25, 0.25).multiplyScalar(U), new THREE.Matrix4(), 2, 6, () => tone('nose'));
  },
  tail: {
    // Straight up from the rump, curling back over at the top, as the
    // sheet draws it behind its back.
    joints: [P(-9.8, 9), P(-11.1, 11.8), P(-11.6, 15.5), P(-11.6, 19.5), P(-13, 22.8)],
    tip: P(-16.6, 21.4),
    radii: [1.7, 3.8, 5.3, 5.5, 4.6].map((r) => r * U),
    build(s: Sculpt) {
      const path = [P(-9, 8.8), P(-10.2, 10.5), P(-11.2, 13), P(-11.6, 16), P(-11.4, 19), P(-12, 21.8), P(-13.6, 23.4), P(-15.5, 23.2), P(-16.6, 21.4)];
      const radii = [1.4, 3.1, 4.3, 5.2, 5.5, 5.3, 4.6, 3.4, 1.7].map((r) => r * U);
      loft(s, tubeRings(path, radii), {
        sides: 8,
        start: 'flat',
        end: 1.2 * U,
        paint: (r, angle, k) => (r >= 6 ? (angle < Math.PI ? tone('tip') : blend('tip', 'coat', 0.4)) : patchy('coat', 'dark', r, k, 0.4, 0.4)),
      });
    },
  },
  front: {
    hip: P(5.5, 7.2, 2.2),
    knee: P(4.9, 4.3, 2.2),
    ankle: P(5.8, 1.8, 2.2),
    paw: P(6.1, 0.8, 2.2),
    bend: -1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.4, w: 1.2 * U, d: 1.4 * U },
        { t: 0, w: 1.15 * U, d: 1.3 * U },
        { t: 0.6, w: 0.85 * U, d: 0.9 * U },
        { t: 1, w: 0.7 * U, d: 0.75 * U },
        { t: 2, w: 0.55 * U, d: 0.55 * U },
        { t: 3, w: 0.6 * U, d: 0.5 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        paint: (r, angle) => (r >= 3 ? tone('paws') : angle > Math.PI * 1.1 || Math.min(angle, 2 * Math.PI - angle) < 0.9 ? tone('cream') : tone('coat')),
      });
      hand(s, joints[3], side, 1.1);
    },
  },
  hind: {
    hip: P(-6, 8, 3),
    knee: P(-3.2, 5, 3),
    ankle: P(-7.2, 2.2, 3),
    paw: P(-6.4, 0.7, 3),
    bend: 1,
    build(s, side, joints) {
      const rings = legRings(joints, [
        { t: -0.3, w: 3 * U, d: 4 * U },
        { t: 0, w: 3.2 * U, d: 4.3 * U },
        { t: 0.5, w: 2.6 * U, d: 3.2 * U, dz: 0.4 * U },
        { t: 1, w: 1.1 * U, d: 1.2 * U },
        { t: 1.5, w: 0.8 * U, d: 0.9 * U },
        { t: 2, w: 0.6 * U, d: 0.6 * U },
        { t: 3, w: 0.6 * U, d: 0.5 * U },
      ]);
      loft(s, rings, {
        sides: 6,
        side: new THREE.Vector3(side, 0, 0),
        mirror: side < 0,
        start: 'flat',
        end: 'open',
        paint: (r) => (r >= 4 ? tone('paws') : patchy('coat', 'dark', r, 7)),
      });
      hand(s, joints[3], side, 2.2);
    },
  },
  walk: WALK,
  run: RUN,
};

// A squirrel's little dark hand or foot at a foot's joint, `length` cm long.
function hand(s: Sculpt, at: THREE.Vector3, side: number, length: number): void {
  const rings = [
    { z: -0.3, y: at.y / U, w: 0.5, up: 0.35, down: at.y / U },
    { z: length * 0.5, y: 0.35, w: 0.62, up: 0.3, down: 0.35 },
    { z: length * 0.9, y: 0.25, w: 0.5, up: 0.22, down: 0.25 },
  ].map((r) => ({ at: new THREE.Vector3(at.x, r.y * U, at.z + r.z * U), w: r.w * U, up: r.up * U, down: r.down * U }));
  loft(s, rings, { sides: 6, side: new THREE.Vector3(side, 0, 0), mirror: side < 0, start: 'flat', end: 0.2 * U, paint: () => tone('paws') });
}

// The acorn it holds: a light brown nut under a darker scaly cap with a
// little stem.
function acorn(palette: Palette): THREE.Mesh {
  const s = new Sculpt(seededRandom(5), 0.03);
  ellipsoid(s, P(0, -0.1), new THREE.Vector3(0.62, 0.78, 0.62).multiplyScalar(U), new THREE.Matrix4().makeRotationX(-Math.PI / 2), 3, 7, () => tone('nut'));
  ellipsoid(s, P(0, 0.42), new THREE.Vector3(0.7, 0.36, 0.7).multiplyScalar(U), new THREE.Matrix4().makeRotationX(-Math.PI / 2), 2, 7, () => tone('cap'));
  loft(s, [{ at: P(0, 0.7), w: 0.1 * U, up: 0.1 * U }, { at: P(0.05, 1.05), w: 0.07 * U, up: 0.07 * U }], { sides: 4, start: 'flat', end: 'flat', paint: () => tone('cap', 0.8) });
  return rigidMesh(s.geometry(palette), gloss(0.55));
}

// How it climbs, step by step: up onto the trunk (pivoting on its hind
// feet), up it, turned round on it, down it head first, and off it (pivoting
// on its fore feet) onto the ground facing away.
type Climb = 'rise' | 'up' | 'turn' | 'down' | 'land';

// It pivots onto the trunk about the bottom of its rump, and off it about
// its chin, so no part of it goes into the ground: in its own space (in m),
// and how high its origin is up the trunk when each is on the ground.
const RUMP = new THREE.Vector3(0, 0.01, -0.125);
const CHIN = new THREE.Vector3(0, 0.1, 0.19);
const FIRST = 0.15; // m up the trunk as it starts up
const LAST = 0.2; // m up the trunk as it comes off it
// Getting off, it lunges out from the trunk until it is back where it
// started the climb, facing the other way.
const RISE = 0.6; // s to get onto the trunk, and off it
const TURN = 0.8; // s to turn round on it
const NOSE_UP = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(1, 0, 0), -Math.PI / 2);
const NOSE_DOWN = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 0, 1), Math.PI).multiply(NOSE_UP);
const TURNED = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), Math.PI);
const q1 = new THREE.Quaternion();
const p1 = new THREE.Vector3();

export class ForestSquirrel extends FourLegged {
  // The sheet's colour variations, the default (brown, its orange-red) first.
  static readonly COLORS = Object.keys(VARIATIONS);
  // How far up a trunk it climbs, m, before it comes back down.
  static readonly CLIMB = 0.6;
  // How far ahead of it (its origin) the trunk's face must be for Climb(),
  // m: its preview puts a trunk there.
  static readonly TRUNK = 0.22;

  // The acorn it holds sitting up, and carries in its mouth on the move.
  readonly acorn: THREE.Mesh;
  private readonly carried: THREE.Mesh;
  private climb: Climb | null = null;
  private climbTime = 0;
  private height = 0; // m its origin is up the trunk

  constructor(options: ForestAnimalOptions = {}) {
    super(BUILD, options);
    this.acorn = this.painting(acorn(this.palette));
    this.carried = this.painting(acorn(this.palette));
    // Held in both hands before the chest; carried crosswise in the mouth.
    this.acorn.scale.setScalar(1.5);
    this.acorn.position.set(0, -4.3 * U, 3.3 * U);
    this.spine.chest.add(this.acorn);
    this.carried.position.set(0, -0.9 * U, 4.6 * U);
    this.carried.rotation.z = Math.PI / 2;
    this.carried.scale.setScalar(0.85);
    this.head.add(this.carried);
    this.settle();
    this.showAcorn();
  }

  // Forward speed; 0 while it climbs (it goes up and down, not round).
  get speed(): number {
    return this.climb ? 0 : this.speedNow;
  }

  // Whether it is on a trunk.
  get climbing(): boolean {
    return this.climb !== null;
  }

  // Sits up on its haunches, the acorn in its hands, its tail curled up its
  // back, nibbling now and then. Named as in its sheet's poses.
  Idle(): void {
    if (this.climb) return;
    super.Idle();
  }

  Walk(): void {
    if (this.climb) return;
    super.Walk();
  }

  Run(): void {
    if (this.climb) return;
    super.Run();
  }

  // Leaps forward, stretched out, its tail streaming behind. Named as in
  // its sheet's poses.
  Jump(): void {
    if (this.climb) return;
    this.setMode('jump', 'stand');
    this.leap(0.18, 0.45, 0.12);
  }

  // Climbs the trunk of a tree whose face is TRUNK ahead of it: onto it,
  // 60 cm up, turns round and comes down head first, then lunges off onto
  // the ground where it started, facing away, and sits. Named as in its sheet's poses. Its
  // preview puts the trunk there.
  Climb(): void {
    if (this.climb || this.jumping) return;
    this.next('rise', 'stand');
  }

  protected pose(goal: Posture): void {
    super.pose(goal);
    const t = this.modeTime;
    if (this.mode === 'idle') {
      // Sitting up with the acorn: haunches down, chest up, hands free.
      goal.rear = 0.027;
      goal.front = -0.05;
      goal.sit = 1;
      goal.frontFree = 1;
      goal.neck = 0.35;
      goal.head = 0.18 + 0.12 * Math.max(0, Math.sin(t * 1.3)) ** 6; // a nibble now and then
      // The tail kept up its back as the body tips up: half the tip is
      // the rig's, the rest here.
      goal.tail = 0.42;
      goal.tailCurl = 0;
    }
    if (this.mode === 'jump' && !this.jumping && t > 0.2) this.Idle();
    if (this.climb) goal.tail = 0.6; // up, clear of the ground
  }

  // On the move the tail streams out behind, lower running.
  protected gaitPosture(goal: Posture, running: number, moving: number): void {
    super.gaitPosture(goal, running, moving);
    // (Its tail stands up at rest, where the others' hang: back past what
    // the rig lifts the others' by.)
    goal.tail -= (0.12 + 1.5 * running) * moving;
    goal.tailCurl -= 0.12 * running * moving;
  }

  // On the trunk its body moves, not the animal.
  protected advance(delta: number): void {
    if (!this.climb) super.advance(delta);
  }

  // Getting on and off the trunk and turning on it, its feet go with it.
  protected freeFoot(foot: Foot): FreeFoot | null {
    if (this.climb === 'rise' || this.climb === 'turn' || this.climb === 'land') return { at: foot.rest, tilt: 0, curl: 0 };
    return null;
  }

  update(delta: number): void {
    this.stepClimb(Math.max(0, Math.min(delta, 0.1)));
    super.update(delta);
    this.showAcorn();
  }

  // Places the rig on the trunk for this frame.
  private stepClimb(dt: number): void {
    if (!this.climb) return;
    this.climbTime += dt;
    const t = this.climbTime;
    const rig = this.rig;
    // Turned by `q` about a point of the rig, `pivot` (in the rig), which
    // is then at `at` (in the animal).
    const about = (q: THREE.Quaternion, pivot: THREE.Vector3, at: THREE.Vector3) => {
      rig.quaternion.copy(q);
      this.rigOffset.copy(at).sub(p1.copy(pivot).applyQuaternion(q));
    };
    const trunk = ForestSquirrel.TRUNK;
    if (this.climb === 'rise') {
      // Nose up about the bottom of its rump, which steps to the trunk.
      const s = smooth(t / RISE);
      const from = RUMP.clone();
      const to = new THREE.Vector3(0, FIRST + RUMP.z, trunk - RUMP.y);
      about(q1.identity().slerp(NOSE_UP, s), RUMP, from.lerp(to, s));
      if (t >= RISE) {
        this.height = FIRST;
        this.next('up', 'walk');
      }
    } else if (this.climb === 'up') {
      this.height += this.speedNow * dt;
      about(NOSE_UP, new THREE.Vector3(), new THREE.Vector3(0, this.height, trunk));
      if (this.height >= ForestSquirrel.CLIMB) this.next('turn', 'stand');
    } else if (this.climb === 'turn') {
      const s = smooth(t / TURN);
      about(q1.setFromAxisAngle(new THREE.Vector3(0, 0, 1), Math.PI * s).multiply(NOSE_UP), new THREE.Vector3(), new THREE.Vector3(0, this.height, trunk));
      if (t >= TURN) this.next('down', 'walk');
    } else if (this.climb === 'down') {
      this.height = Math.max(LAST, this.height - this.speedNow * dt);
      about(NOSE_DOWN, new THREE.Vector3(), new THREE.Vector3(0, this.height, trunk));
      if (this.height <= LAST) this.next('land', 'stand');
    } else {
      // Level again about its chin, facing away from the trunk, and
      // lunging out from it.
      const s = smooth(t / RISE);
      const from = new THREE.Vector3(0, LAST - CHIN.z, trunk - CHIN.y);
      const to = new THREE.Vector3(0, CHIN.y, -CHIN.z);
      about(q1.copy(NOSE_DOWN).slerp(TURNED, s), CHIN, from.lerp(to, s));
      if (t >= RISE) {
        // Hand the rig's place and turn back to the animal itself.
        this.updateMatrix();
        const origin = this.rigOffset.clone().applyMatrix4(this.matrix);
        this.position.copy(origin).setY(0);
        this.rotation.y += Math.PI;
        rig.quaternion.identity();
        this.rigOffset.set(0, 0, 0);
        this.climb = null;
        this.setMode('idle', 'stand');
      }
    }
  }

  private next(step: Climb, gait: 'stand' | 'walk'): void {
    this.climb = step;
    this.climbTime = 0;
    this.setMode('climb', gait);
  }

  // The acorn shows in its hands while they hold it up, in its mouth when
  // they are down.
  private showAcorn(): void {
    const held = this.posture.frontFree > 0.6;
    this.acorn.visible = held;
    this.carried.visible = !held;
  }
}
