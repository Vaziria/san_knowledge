import * as THREE from 'three';
import { ForestSpiritDragon } from '../../figures/ForestLake/animals/ForestSpiritDragon/ForestSpiritDragon';
import { SpeechBubble } from '../../speech/SpeechBubble';
import type { Theme } from '../../theme';
import type { Figure } from '../lake_meeting/kinds';
import { Member, shortest } from '../lake_meeting/Member';
import type { Seeing } from '../lake_meeting/Sight';

// The forest spirit dragon at the forest lake meeting (forest_lake_meeting.md
// item 14), as the user asked ("in forest lake meeting, its fly and roar
// around our cristal tree, and sometimes its walk calm in ground"). It is the
// first viewer's to comment while it is no one's ("make it can talk",
// "dragon is special for one that first comment"; the dev server decides,
// meeting.ts): it says what they write in a speech bubble over its head,
// headed by their name, and takes no commands.
//
// - While its viewer talks (hold() to release(), the meeting's turn), it
//   keeps round the tree without roaring or going down to the meadow, or on
//   the ground stands still, as an animal stops to talk. The camera follows
//   it exactly from the first of TALK_VIEWS that is clear (talkShot()),
//   picked again whenever that one isn't.
//
// - It flies round the island's great crystal tree, ROUND.radius m out from
//   its trunk, clear of its crown (10.75 m out at its widest, 18–20 m up)
//   and of the shores, rising over the east shore's trees and cliffs and
//   coming down low over the open water to the west, banking into the turn.
// - Now and then it roars (ROAR_EVERY), only on the side over the open water,
//   where a camera has room round it. The meeting films it when nothing else
//   is being shown (ForestMeeting.ts: a turn of its own, roar()); otherwise
//   it roars unseen.
// - Every few minutes (VISIT) it leaves the tree and flies south over the
//   landing to the meadow behind it, the open ground kept for the landing's
//   view south, beyond where the animals roam (they keep within 24.6 m of the
//   landing that way): it comes down there heading south (TOUCHDOWN), walks
//   about it calmly (MEADOW), now and then standing still, and once its time
//   there is up it turns to face the lake, takes off and flies back to the
//   tree.
//
// The cameras see it as one of the animals at the meeting (`member`): the
// quiet camera may follow it, and none is put inside it or looks through it.
//
// Meters, in the meeting's coordinates (its origin the landing, +z away from
// the water); `tree` is the foot of the crystal tree's trunk, `water` the
// lake's still surface.

const ROUND = {
  radius: 16, // m from the trunk: its crown and the dragon's wings (3.6 m) with a meter to spare
  wobble: 0.6, // m it drifts in and out
  low: 10, // m over the water at its lowest, to the west
  high: 20, // and at its highest, to the east, over the shore's trees and cliffs
  drift: 1.5, // m it rises and falls besides
  ahead: 0.45, // rad round the circle ahead of it that it steers for
};
const TURN = 0.9; // rad a second it turns at most, flying
const WALK_TURN = 0.3; // and walking
const ROAR_EVERY = [40, 80] as const; // s between its roars
const ROAR_LOOK = ForestSpiritDragon.ROAR_TIME + 0.7; // s ahead the camera on its roar must stay clear
const CAMERA_ROOM = 1; // m round that camera kept clear too: it climbs and comes down a little behind its foreseen height
const ROAR_LATE = 30; // s overdue after which it roars unseen, no camera having had room
const RETRY = 0.3; // s between looks for a camera with room
const STEP = 0.1; // s between the places it is foreseen at, for that camera
const ON_ROUND = 2; // m off its circle round the tree within which it roars, filmed
const ROAR_WEST = 1.0; // rad either side of due west round the tree where it roars
const VISIT = [120, 220] as const; // s round the tree before it goes down to the meadow
const STAY = [35, 60] as const; // s on the ground
const PAUSE = [4, 8] as const; // s it stands now and then on the ground
// Where it comes down: straight south over the landing, touching down here
// heading south, from APPROACH m over the ground; the gate it lines up at.
const TOUCHDOWN = new THREE.Vector3(0, 0, 33);
const GATE = new THREE.Vector3(0, 0, 9);
const APPROACH = 9;
// Where it walks: its middle kept in this box (x, z), its 4.8 m round it in
// the open meadow, and 1 m clear of the farthest an animal roams to.
const MEADOW = { x: [-6, 5] as const, z: [31.5, 38] as const };
const RADIUS = 4.8; // m round its middle it takes up on the ground: its tail's end is 4.7 m behind it
const REACHED = 1.2; // m from a point on the ground that counts as there
// The camera on its roar, in its own frame (ahead, out from the tree, up),
// the first of these that is clear: ahead of it looking back, from the side,
// from behind.
const ROAR_VIEWS = [
  new THREE.Vector3(13, 4, -1.5),
  new THREE.Vector3(13, -2, -1),
  new THREE.Vector3(4, 13, -1.5),
  new THREE.Vector3(-10, 5, 1),
] as const;
// Its speech bubble: BUBBLE_UP m over its head's pivot (over its horns, whose
// tips are 1.5 m up), wherever its neck puts it, and BUBBLE_SCALE times the
// lake penguin's, about 4 m wide, so its words read from the camera on it,
// at most 11.1 m from the bubble's middle (a bubble of scale 1 reads from
// 1.05 m, READABLE in ForestMeeting.ts).
const BUBBLE_UP = 1.7;
const BUBBLE_SCALE = 11;
// The camera on its words, in its own frame (ahead, to its side, up) from
// AIM_DOWN m under its bubble's tip, which it looks at, so that standing,
// all of it is in the picture under its words. The first of these that is
// clear: ahead of it looking back at its face, from one side or the other,
// from behind (level: from higher up, the bubble's top was cut off). Round
// the tree the first side is the one away from it.
const TALK_VIEWS = [
  new THREE.Vector3(9, 4, -1.5),
  new THREE.Vector3(9, -4, -1.5),
  new THREE.Vector3(4, 9, -1),
  new THREE.Vector3(4, -9, -1),
  new THREE.Vector3(-8, 6, 0),
  new THREE.Vector3(-8, -6, 0),
] as const;
const AIM_DOWN = 1.5;
const TALK_LOOK = 2; // s ahead a camera on its words must stay clear, when it is picked
const OVER_FLOOR = 0.5; // m a camera on it keeps over the ground or the water
const LIFT_EASE = 3; // how fast the camera's aim follows its bubble as its head moves, a second

type Phase = 'round' | 'approach' | 'landing' | 'ground' | 'leave';

// The dragon as the meeting's cameras see a figure (lake_meeting/kinds.ts):
// it says what its viewer writes in a speech bubble over its head (Speech()
// is the meeting's, not its spec's, as the animals' are, guests.ts), and
// runs as it walks.
class Resident extends ForestSpiritDragon implements Figure {
  private readonly words: SpeechBubble;
  private readonly overHead = new THREE.Vector3();

  constructor(theme: Theme) {
    super();
    this.words = new SpeechBubble(theme);
    this.words.scale.setScalar(BUBBLE_SCALE);
    this.add(this.words);
  }

  get speaking(): boolean {
    return this.words.speaking;
  }

  get speechBubble(): THREE.Object3D {
    return this.words;
  }

  Speech(text: string, speaker = ''): void {
    this.words.say(text, speaker);
  }

  Jump(): void {}
  Run(): void {
    this.Walk();
  }

  update(delta: number): void {
    super.update(delta);
    // (Its figure's constructor updates it before its bubble is made.)
    if (!this.words) return;
    // Over its head, straight up from it however it banks or leans.
    this.head.getWorldPosition(this.overHead);
    this.overHead.y += BUBBLE_UP;
    this.words.position.copy(this.worldToLocal(this.overHead));
    this.words.update(delta);
  }
}

// The meeting's member for it: on its way whenever it moves, flying too, so
// a camera following it looks ahead of it.
class Flier extends Member {
  get moving(): boolean {
    return this.figure.speed > 0.05;
  }
}

export interface DragonView {
  camera: THREE.Vector3;
  target: THREE.Vector3;
}

export class Dragon {
  readonly figure: Resident;
  readonly member: Member;
  private readonly tree: THREE.Vector3;
  private readonly water: number;
  private readonly random: () => number;
  private phase: Phase = 'round';
  private time = 0;
  private left: number; // s before it goes down to the meadow, or before it leaves it
  private roarIn: number; // s before it would roar
  private pause = 0; // s it stands still on the ground
  private walkTo: THREE.Vector2 | null = null;
  private view: THREE.Vector3 | null = null; // the camera on its roar, in its own frame
  private retryAt = 0; // s of its time before it looks for a camera for its roar again
  private held = false; // its viewer is talking
  private talkView: THREE.Vector3 = TALK_VIEWS[0]; // the camera on its words, in its own frame
  private talkCheck = 0; // s of its time when that camera is next checked
  // Its bubble's tip from its middle (along its heading, up), eased, so the
  // camera on its words doesn't bob with its head; null before its first frame.
  private lift: THREE.Vector2 | null = null;

  constructor(tree: THREE.Vector3, water: number, theme: Theme, random: () => number = Math.random) {
    this.tree = tree.clone();
    this.water = water;
    this.random = random;
    const figure = (this.figure = new Resident(theme));
    figure.rotation.order = 'YXZ'; // its heading, then leaning with the ground
    this.member = new Flier('dragon', '', 'dragon', true, figure);
    this.member.radius = RADIUS;
    // It starts on its way round the tree, west of it, flying.
    const angle = Math.PI;
    figure.position.set(tree.x + ROUND.radius * Math.cos(angle), this.heightAt(angle), tree.z + ROUND.radius * Math.sin(angle));
    figure.rotation.y = this.headingRound(figure.position);
    figure.groundHeight = water;
    figure.flyHeight = figure.position.y;
    figure.startFlying();
    this.left = between(random, VISIT);
    this.roarIn = between(random, ROAR_EVERY) / 2;
  }

  // Whether it would roar now, filmed: round the tree, on the side over the
  // open water, its time come, and not roaring already.
  get wantsRoar(): boolean {
    const p = this.figure.position;
    const onRound = Math.abs(Math.hypot(p.x - this.tree.x, p.z - this.tree.z) - ROUND.radius) < ON_ROUND;
    return this.phase === 'round' && this.roarIn <= 0 && !this.figure.roaring && !this.held && this.westward && onRound;
  }

  get flying(): boolean {
    return this.figure.flying;
  }

  // Its viewer's turn to talk begins: round the tree it keeps going round,
  // neither roaring nor going down to the meadow, and on the ground it stands
  // still, until release(). The camera on its words is picked.
  hold(sight: Seeing): void {
    this.held = true;
    this.talkView = this.pickTalk(sight);
    this.talkCheck = this.time + RETRY;
  }

  release(): void {
    this.held = false;
  }

  // Where the camera is on its words: following it exactly, from where
  // hold() put it, or, once that isn't clear (looked at every RETRY s), from
  // the first of TALK_VIEWS that is.
  talkShot(sight: Seeing): DragonView {
    if (this.time >= this.talkCheck) {
      this.talkCheck = this.time + RETRY;
      if (!this.clear(sight, this.talkFrom(this.talkView, this.pose()))) this.talkView = this.pickTalk(sight);
    }
    return this.talkFrom(this.talkView, this.pose());
  }

  // The first of TALK_VIEWS clear for the next TALK_LOOK s, where it will be;
  // or else the first clear now; or else the one it has.
  private pickTalk(sight: Seeing): THREE.Vector3 {
    const path = this.ahead(TALK_LOOK);
    return (
      TALK_VIEWS.find((offset) => path.every((pose) => this.clear(sight, this.talkFrom(offset, pose)))) ??
      TALK_VIEWS.find((offset) => this.clear(sight, this.talkFrom(offset, path[0]))) ??
      this.talkView
    );
  }

  // Whether a camera is out of everything solid, with CAMERA_ROOM round it,
  // over the floor, and sees what it looks at.
  private clear(sight: Seeing, { camera, target }: DragonView): boolean {
    if (camera.y < sight.floor(camera.x, camera.z) + OVER_FLOOR) return false;
    for (const s of [-1, 1]) {
      for (const axis of [0, 1, 2]) {
        const point = camera.clone();
        point.setComponent(axis, point.getComponent(axis) + s * CAMERA_ROOM);
        if (sight.blocked(point)) return false;
      }
    }
    return !sight.blocked(camera) && sight.sees(target, camera, 1);
  }

  // The camera `offset` in its own frame (ahead, to its side, up) from
  // AIM_DOWN under its bubble's tip, where it is at `pose`. Round the tree,
  // its side away from the tree is +.
  private talkFrom(offset: THREE.Vector3, pose: { at: THREE.Vector3; heading: number }): DragonView {
    const forward = new THREE.Vector3(Math.sin(pose.heading), 0, Math.cos(pose.heading));
    const side = new THREE.Vector3(forward.z, 0, -forward.x);
    if (this.phase === 'round' && side.x * (pose.at.x - this.tree.x) + side.z * (pose.at.z - this.tree.z) < 0) side.negate();
    const lift = this.lift ?? new THREE.Vector2(1.6, 5.3);
    const target = pose.at.clone().addScaledVector(forward, lift.x);
    target.y += lift.y - AIM_DOWN;
    const camera = target.clone().addScaledVector(forward, offset.x).addScaledVector(side, offset.y);
    camera.y += offset.z;
    return { camera, target };
  }

  private pose(): { at: THREE.Vector3; heading: number } {
    return { at: this.figure.position.clone(), heading: this.figure.rotation.y };
  }

  // It roars now, as its turn begins (or unseen, the meeting busy); the
  // next comes ROAR_EVERY s on.
  roar(): void {
    this.figure.Roar();
    this.roarIn = between(this.random, ROAR_EVERY);
  }

  // Picks the camera for its roar: the first of ROAR_VIEWS out of everything
  // solid and seeing it all through the roar, where it will be as it goes
  // round the tree. False when none is.
  frame(sight: Seeing): boolean {
    this.view = null;
    // (Tried again only every RETRY s: it looks a few seconds ahead.)
    if (this.time < this.retryAt) return false;
    this.retryAt = this.time + RETRY;
    const path = this.ahead(ROAR_LOOK);
    const round = [new THREE.Vector3(), ...[-1, 1].flatMap((s) => [new THREE.Vector3(s * CAMERA_ROOM, 0, 0), new THREE.Vector3(0, s * CAMERA_ROOM, 0), new THREE.Vector3(0, 0, s * CAMERA_ROOM)])];
    for (const offset of ROAR_VIEWS) {
      const clear = path.every((pose) => {
        const { camera, target } = this.shotFrom(offset, pose);
        return round.every((r) => !sight.blocked(camera.clone().add(r))) && sight.sees(target, camera, 1);
      });
      if (!clear) continue;
      this.view = offset;
      return true;
    }
    return false;
  }

  // Where the camera is on its roar: following it, from where frame() put it.
  shot(): DragonView {
    const figure = this.figure;
    return this.shotFrom(this.view ?? ROAR_VIEWS[0], { at: figure.position.clone(), heading: figure.rotation.y });
  }

  // Where it will be over the next `seconds`, every STEP s: flying round
  // the tree as it does, steered as update() steers it, at its speed, and
  // climbing or coming down toward its height; otherwise straight on at its
  // speed and height.
  private ahead(seconds: number): { at: THREE.Vector3; heading: number }[] {
    const figure = this.figure;
    const round = this.phase === 'round';
    const at = figure.position.clone();
    let heading = figure.rotation.y;
    const poses = [{ at: at.clone(), heading }];
    for (let t = STEP; t <= seconds + 1e-9; t += STEP) {
      const most = TURN * STEP;
      if (round) heading += THREE.MathUtils.clamp(shortest(this.headingRound(at, this.time + t) - heading), -most, most);
      at.x += Math.sin(heading) * figure.speed * STEP;
      at.z += Math.cos(heading) * figure.speed * STEP;
      if (round) at.y += THREE.MathUtils.clamp(1.2 * (this.heightAt(this.angleOf(at), this.time + t) - at.y), -ForestSpiritDragon.DESCENT, ForestSpiritDragon.CLIMB) * STEP;
      poses.push({ at: at.clone(), heading });
    }
    return poses;
  }

  // The camera `offset` from it in its own frame (ahead, out from the tree,
  // up), where it is at `pose`.
  private shotFrom(offset: THREE.Vector3, pose: { at: THREE.Vector3; heading: number }): DragonView {
    const forward = new THREE.Vector3(Math.sin(pose.heading), 0, Math.cos(pose.heading));
    const target = pose.at.clone().add(new THREE.Vector3(0, 2.2, 0));
    const out = new THREE.Vector3(pose.at.x - this.tree.x, 0, pose.at.z - this.tree.z).normalize();
    const camera = target.clone().addScaledVector(forward, offset.x).addScaledVector(out, offset.y);
    camera.y += offset.z;
    return { camera, target };
  }

  // Moves it on: `ground` is the height to stand at over the land, `floor`
  // the land's or the water's surface (the cameras' floor).
  update(delta: number, ground: (x: number, z: number) => number, floor: (x: number, z: number) => number): void {
    const figure = this.figure;
    this.time += delta;
    this.left -= delta;
    figure.ground = ground;
    const p = figure.position;
    switch (this.phase) {
      case 'round': {
        // Round the tree, at its height for where it is round it; its next
        // roar coming (counted only here), unseen when no camera has had room
        // on it for a long while. Not while its viewer talks.
        this.roarIn -= delta;
        if (this.roarIn < -ROAR_LATE && !this.held) this.roar();
        this.steer(this.headingRound(p), TURN, delta);
        figure.flyHeight = this.heightAt(this.angleOf(p));
        figure.groundHeight = floor(p.x, p.z);
        if (this.left <= 0 && this.southward && !this.held) this.phase = 'approach';
        break;
      }
      case 'approach': {
        // To the gate, then straight on south to where it touches down,
        // coming down to APPROACH m over the ground there; it lands once it
        // is as far from there as it takes to land.
        const ground0 = ground(TOUCHDOWN.x, TOUCHDOWN.z);
        const lined = p.z > GATE.z - 1;
        const aim = lined ? TOUCHDOWN : GATE;
        this.steer(Math.atan2(aim.x - p.x, aim.z - p.z), TURN, delta);
        figure.flyHeight = ground0 + APPROACH;
        figure.groundHeight = lined ? ground0 : floor(p.x, p.z);
        const off = Math.hypot(TOUCHDOWN.x - p.x, TOUCHDOWN.z - p.z);
        if (lined && off <= figure.landingDistance + 0.2) {
          figure.groundHeight = ground0;
          figure.Land();
          this.phase = 'landing';
        }
        break;
      }
      case 'landing': {
        // Kept heading for where it touches down.
        if (figure.speed > 1) this.steer(Math.atan2(TOUCHDOWN.x - p.x, TOUCHDOWN.z - p.z), TURN, delta);
        if (!figure.flying) {
          this.phase = 'ground';
          this.left = between(this.random, STAY);
          this.pause = between(this.random, PAUSE) / 2;
          this.walkTo = null;
        }
        break;
      }
      case 'ground': {
        this.onGround(delta, ground);
        break;
      }
      case 'leave': {
        // Up and away north over the landing, then round the tree again
        // once it is well up.
        figure.flyHeight = this.heightAt(this.angleOf(p));
        figure.groundHeight = floor(p.x, p.z);
        if (p.y - ground(p.x, p.z) > 6 || p.z < 5) this.steer(this.headingRound(p), TURN, delta);
        if (p.z < 0) {
          this.phase = 'round';
          this.left = between(this.random, VISIT);
        }
        break;
      }
    }
    if (!figure.flying && figure.doing !== 'takeoff') {
      // On the ground, leaning with its slope along and across its length
      // (before it moves itself, which plants its feet on the ground and
      // keeps its tail off it, as it then stands).
      const { x, z } = figure.position;
      figure.position.y = ground(x, z);
      const heading = figure.rotation.y;
      const ahead = new THREE.Vector2(Math.sin(heading), Math.cos(heading));
      const e = 1.5;
      const along = (ground(x + ahead.x * e, z + ahead.y * e) - ground(x - ahead.x * e, z - ahead.y * e)) / (2 * e);
      const across = (ground(x + ahead.y * e, z - ahead.x * e) - ground(x - ahead.y * e, z + ahead.x * e)) / (2 * e);
      const lean = 1 - Math.exp(-4 * delta);
      figure.rotation.x += (-Math.atan(along) - figure.rotation.x) * lean;
      figure.rotation.z += (Math.atan(across) - figure.rotation.z) * lean;
    } else {
      figure.rotation.x *= Math.exp(-4 * delta);
      figure.rotation.z *= Math.exp(-4 * delta);
    }
    figure.update(delta);
    // Where its bubble's tip is from its middle, for the camera on its words.
    figure.updateMatrix();
    const tip = figure.speechBubble.position.clone().applyMatrix4(figure.matrix);
    const heading = figure.rotation.y;
    const along = (tip.x - p.x) * Math.sin(heading) + (tip.z - p.z) * Math.cos(heading);
    const now = new THREE.Vector2(along, tip.y - p.y);
    if (this.lift) this.lift.lerp(now, 1 - Math.exp(-LIFT_EASE * delta));
    else this.lift = now;
  }

  // On the ground: walking calmly about the meadow, standing still now and
  // then, and once its time there is up, facing the lake and taking off.
  private onGround(delta: number, ground: (x: number, z: number) => number): void {
    const figure = this.figure;
    const p = figure.position;
    figure.groundHeight = ground(p.x, p.z);
    if (figure.doing === 'takeoff' || figure.flying) {
      figure.flyHeight = this.heightAt(this.angleOf(p));
      if (figure.flying && p.y - figure.groundHeight > 1) this.phase = 'leave';
      return;
    }
    if (this.held) {
      // Its viewer talks: it stands still while it says it.
      figure.Idle();
      return;
    }
    const facingLake = Math.abs(shortest(figure.rotation.y - Math.PI)) < 0.15;
    if (this.left <= 0 && facingLake) {
      // Up and away, toward the lake.
      figure.flyHeight = figure.groundHeight + 12;
      figure.Fly();
      return;
    }
    if (this.pause > 0) {
      this.pause -= delta;
      figure.Idle();
      return;
    }
    if (this.left <= 0) {
      // Its time up: round to face the lake, walking on, and up once it
      // does (above).
      this.walkTo = new THREE.Vector2(THREE.MathUtils.clamp(p.x, MEADOW.x[0], MEADOW.x[1]), MEADOW.z[0] - 30);
    } else if (!this.walkTo || Math.hypot(this.walkTo.x - p.x, this.walkTo.y - p.z) < REACHED) {
      if (this.walkTo) {
        this.pause = this.random() < 0.6 ? between(this.random, PAUSE) : 0;
        this.walkTo = null;
        if (this.pause > 0) return;
      }
      // A new spot in the meadow, some meters from where it is.
      for (let i = 0; i < 20; i++) {
        const to = new THREE.Vector2(between(this.random, MEADOW.x), between(this.random, MEADOW.z));
        if (to.distanceTo(new THREE.Vector2(p.x, p.z)) > 4) {
          this.walkTo = to;
          break;
        }
      }
    }
    if (!this.walkTo) return;
    // Walked to, turning gently; never out of the meadow: heading back in
    // as it nears its edge.
    let want = Math.atan2(this.walkTo.x - p.x, this.walkTo.y - p.z);
    const margin = 0.6;
    const outX = p.x < MEADOW.x[0] - margin || p.x > MEADOW.x[1] + margin;
    // (Turning to leave, it may go a little nearer the lake, its head still
    // 2 m from the farthest an animal roams to.)
    const outZ = p.z > MEADOW.z[1] + margin || p.z < MEADOW.z[0] - (this.left > 0 ? margin : 2);
    if (outX || outZ) want = Math.atan2((MEADOW.x[0] + MEADOW.x[1]) / 2 - p.x, (MEADOW.z[0] + MEADOW.z[1]) / 2 - p.z);
    this.steer(want, WALK_TURN, delta);
    figure.Walk();
  }

  // Turns it toward a heading, at most `rate` rad a second.
  private steer(want: number, rate: number, delta: number): void {
    const most = rate * delta;
    this.figure.rotation.y += THREE.MathUtils.clamp(shortest(want - this.figure.rotation.y), -most, most);
  }

  // Its angle round the tree from the east (+x), toward the south (+z).
  private angleOf(p: THREE.Vector3): number {
    return Math.atan2(p.z - this.tree.z, p.x - this.tree.x);
  }

  // The heading that keeps it round the tree: for a point on its circle a
  // little ahead of it, the way it goes round (from the east to the south,
  // the west and the north).
  private headingRound(p: THREE.Vector3, time = this.time): number {
    const radius = ROUND.radius + ROUND.wobble * Math.sin(time * 0.11);
    const round = this.angleOf(p) + ROUND.ahead;
    const x = this.tree.x + radius * Math.cos(round);
    const z = this.tree.z + radius * Math.sin(round);
    return Math.atan2(x - p.x, z - p.z);
  }

  // The height it flies at, at an angle round the tree: high over the east
  // shore, low over the open water to the west, rising and falling slowly
  // besides.
  private heightAt(angle: number, time = this.time): number {
    const east = (1 + Math.cos(angle)) / 2;
    return this.water + ROUND.low + (ROUND.high - ROUND.low) * east + ROUND.drift * Math.sin(time * 0.13);
  }

  // Over the open water to the west of the tree, where a camera has room.
  private get westward(): boolean {
    return Math.abs(shortest(this.angleOf(this.figure.position) - Math.PI)) < ROAR_WEST;
  }

  // South of the tree, on its way round toward the landing.
  private get southward(): boolean {
    const a = this.angleOf(this.figure.position);
    return Math.abs(shortest(a - Math.PI / 2)) < 0.5;
  }
}

function between(random: () => number, [least, most]: readonly [number, number]): number {
  return least + (most - least) * random();
}
