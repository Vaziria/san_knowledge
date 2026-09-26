import * as THREE from 'three';
import { inside, measure } from './Follow';
import { shortest, type Member } from './Member';
import type { Sight } from './Sight';

// The host's view (lake_meeting.md item 10): where the camera goes a few
// seconds after a turn, until the follow takes over (Follow.ts). The host
// roams now (task 14, the user's "make the bear roaming like other too"), so
// the view goes with it, wherever it is, rather than to the landing.
//
// - Framed about as wide as the opening shot (Meeting.WIDE): BACK m off,
//   level, and UP m over its feet, looking at its middle, so it is in the
//   middle of the picture; a little ahead of it on the move (LEAD).
// - From in front of it, or from the first of TURNS that is clear: the
//   camera out of every tree, boulder, stone and animal and over the ground
//   and the water, and nothing, another animal neither, between it and the
//   host's middle, head and front (Sight.ts, as Follow.ts checks). A host on
//   its way must be seen from there in a second too. With no side clear from
//   BACK m, it tries nearer (NEARER).
// - It keeps its side to the way the host faces, coming round as it turns
//   (TRACK), and eases after its place by itself (SMOOTH), so the stage puts
//   the camera exactly where this says (the meeting marks each frame a cut):
//   what was checked is what is seen.
// - When something comes between, it cuts to the first clear side, once the
//   last cut has shown SETTLE s. Where the camera itself would be in
//   something solid or another animal, it cuts at once.
// - It starts from where the camera is: gliding from a turn's close-up at the
//   stage's own pace (GLIDE), or cutting when that is more than `cutBeyond` m
//   away, rather than gliding across the lake.

const BACK = 7.9; // m from the host to the camera, level: the opening shot's
const UP = 2.3; // m over its feet the camera stands, from BACK m off (less, nearer)
const LOWEST = 0.5; // m over the ground under it the camera stands at least
const NEARER = [1, 0.7, 0.45] as const; // shares of BACK tried in turn, nearer only when no side is clear
const TURNS = [0, 0.4, -0.4, 0.9, -0.9, 1.6, -1.6, 2.4, -2.4, Math.PI]; // radians from its heading the camera may look from, best first
const LEAD = 0.8; // s ahead of it on the move the camera aims, as the turns' close-ups
const LEAD_MOST = 0.25; // of the camera's distance, at most, so a runner stays in the picture
const TRACK = 1.5; // how fast the camera comes round as it turns, per second
const SMOOTH = 5; // how fast the camera eases after its place, per second
const GLIDE = 2.5; // and gliding in from where the camera was: the stage's own easing (SHOT_EASE)
const ARRIVED = 0.05; // m from its place where a glide in ends
const SETTLE = 1; // s a side shows before a blocked view cuts to another
const AHEAD = 1; // s ahead a side must also be clear, for a host on its way
const RETRY = 0.5; // s before trying again when no side is clear
const GAP = 0.02; // m the camera keeps over the ground, as Follow.ts
const ROOM = 0.1; // m the camera keeps from another animal
const SKIP = 0.15; // m round the host where the ground and what stands there don't count as in the way
const UP_AXIS = new THREE.Vector3(0, 1, 0);

export interface HostShot {
  camera: THREE.Vector3;
  target: THREE.Vector3;
}

interface Plan {
  turn: number; // radians from its heading, signed
  back: number; // m off, level
}

export class HostView {
  private readonly sight: Sight;
  private plan: Plan | null = null;
  private side = 0; // radians round from +z the camera looks from now
  private since = 0; // s since the last cut
  private gliding = false;
  private blocked = false;
  private wait = 0; // s before trying again, when no side was clear
  private readonly eye = new THREE.Vector3(); // on the camera's way to its place
  private readonly shot: HostShot = { camera: new THREE.Vector3(), target: new THREE.Vector3() };

  constructor(sight: Sight) {
    this.sight = sight;
  }

  // The side it is seen from now, radians from its heading, if any.
  get turn(): number | null {
    return this.plan?.turn ?? null;
  }

  // Whether the view shown now is blocked (and will cut once it has shown
  // SETTLE s).
  get isBlocked(): boolean {
    return this.blocked;
  }

  // The next time it is shown, it starts again from where the camera is.
  stop(): void {
    this.plan = null;
    this.wait = 0;
    this.blocked = false;
  }

  // The camera this frame, starting from `from` (where the camera is) the
  // first time. Null while no side of the host is clear.
  update(delta: number, host: Member, animals: Member[], from: HostShot, cutBeyond: number): HostShot | null {
    this.since += delta;
    this.wait -= delta;
    const shot = this.shot;
    let cut = false;
    if (!this.plan) {
      if (this.wait > 0 || !this.choose(host, animals)) {
        if (this.wait <= 0) this.wait = RETRY;
        return null;
      }
      const there = this.view(host, this.side, this.plan!.back, 0).camera;
      if (from.camera.distanceTo(there) > cutBeyond) {
        cut = true;
      } else {
        this.gliding = true;
        this.eye.copy(from.camera);
        shot.camera.copy(from.camera);
        shot.target.copy(from.target);
      }
    } else if (this.blocked && this.since >= SETTLE && this.wait <= 0) {
      if (this.choose(host, animals)) cut = true;
      else this.wait = RETRY;
    }
    const want = host.figure.rotation.y + this.plan!.turn;
    this.side += shortest(want - this.side) * (cut ? 1 : 1 - Math.exp(-TRACK * delta));
    let view = this.view(host, this.side, this.plan!.back, 0);
    if (!cut) {
      // Eased twice over, so that it starts and stops gently.
      const t = 1 - Math.exp(-(this.gliding ? GLIDE : SMOOTH) * delta);
      this.eye.lerp(view.camera, t);
      shot.camera.lerp(this.eye, t);
      shot.target.lerp(view.target, t);
      shot.camera.y = Math.max(shot.camera.y, this.sight.floor(shot.camera.x, shot.camera.z) + GAP);
      if (this.gliding && shot.camera.distanceTo(view.camera) < ARRIVED) this.gliding = false;
      // Never in a tree, a boulder, a stone or another animal, not for a
      // frame: it cuts to the first clear side at once (a side that comes
      // round with the host, or the glide in, can carry it into one).
      if (this.inside(shot.camera, host, animals) && this.wait <= 0) {
        if (this.choose(host, animals)) {
          cut = true;
          view = this.view(host, this.side, this.plan!.back, 0);
        } else {
          this.wait = RETRY;
        }
      }
    }
    if (cut) {
      this.gliding = false;
      this.since = 0;
      this.eye.copy(view.camera);
      shot.camera.copy(view.camera);
      shot.target.copy(view.target);
    }
    this.blocked = !this.clear(host, shot, animals, 0);
    return shot;
  }

  // Whether a camera there is in something solid, under the floor, or in
  // another animal.
  private inside(camera: THREE.Vector3, host: Member, animals: Member[]): boolean {
    if (this.sight.blocked(camera) || camera.y < this.sight.floor(camera.x, camera.z)) return true;
    return animals.some((other) => other !== host && inside(other, camera, ROOM));
  }

  // The first side, from the furthest back, from which the host is seen now
  // (and in a second, on the move). False when none is.
  private choose(host: Member, animals: Member[]): boolean {
    const heading = host.figure.rotation.y;
    for (const share of NEARER) {
      for (const turn of TURNS) {
        const back = BACK * share;
        if (!this.clear(host, this.view(host, heading + turn, back, 0), animals, 0)) continue;
        if (host.moving && !this.clear(host, this.view(host, heading + turn, back, AHEAD), animals, AHEAD)) continue;
        this.plan = { turn, back };
        this.side = heading + turn;
        this.since = 0;
        return true;
      }
    }
    return false;
  }

  // Where the camera is and what it looks at, seen from `side`, `back` m
  // off, with the host `ahead` s further on its way.
  private view(host: Member, side: number, back: number, ahead: number): HostShot {
    const figure = host.figure;
    const heading = figure.rotation.y;
    const forward = new THREE.Vector3(Math.sin(heading), 0, Math.cos(heading));
    const feet = figure.position.clone().addScaledVector(forward, figure.speed * ahead);
    const target = feet.clone().addScaledVector(forward, Math.min(figure.speed * LEAD, LEAD_MOST * back));
    target.y += 0.5 * host.height;
    const camera = new THREE.Vector3(target.x + Math.sin(side) * back, feet.y + (UP * back) / BACK, target.z + Math.cos(side) * back);
    camera.y = Math.max(camera.y, this.sight.floor(camera.x, camera.z) + LOWEST);
    return { camera, target };
  }

  // Whether the camera sees the host (`ahead` s further on its way): out of
  // everything solid and every other animal, and nothing between it and the
  // host's middle, head and front.
  private clear(host: Member, view: HostShot, animals: Member[], ahead: number): boolean {
    const { camera } = view;
    const sight = this.sight;
    if (this.inside(camera, host, animals)) return false;
    const others = animals.filter((m) => m !== host);
    const figure = host.figure;
    const heading = figure.rotation.y;
    const feet = figure.position.clone().addScaledVector(new THREE.Vector3(Math.sin(heading), 0, Math.cos(heading)), figure.speed * ahead);
    const box = measure(figure);
    const middle = box.getCenter(new THREE.Vector3());
    const height = box.max.y - box.min.y;
    const skip = Math.min(host.radius, SKIP);
    const point = new THREE.Vector3();
    for (const local of [
      middle,
      new THREE.Vector3(middle.x, box.min.y + 0.9 * height, middle.z),
      new THREE.Vector3(middle.x, box.min.y + 0.6 * height, box.max.z - 0.15 * (box.max.z - box.min.z)),
    ]) {
      const at = local.clone().applyAxisAngle(UP_AXIS, heading).add(feet);
      if (!sight.sees(at, camera, skip)) return false;
      const steps = Math.max(2, Math.ceil(at.distanceTo(camera) / 0.1));
      for (let s = 1; s < steps; s++) {
        point.lerpVectors(at, camera, s / steps);
        if (others.some((other) => inside(other, point, 0))) return false;
      }
    }
    return true;
  }
}
