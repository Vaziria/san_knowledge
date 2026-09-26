import * as THREE from 'three';
import type { Figure } from './kinds';
import { shortest, type Member } from './Member';
import type { Sight } from './Sight';

// The camera when the chat is quiet (lake_meeting.md item 10), as the user
// asked ("when no comment new in 30 second, camera randomly follow animal and
// take few different angle"; "when iddle camera follow random animal for 30
// second, and after that follow random animal 30 second, and random again,
// and so on"). The meeting starts it once nothing has been shown for 30 s,
// and stops it at the next turn, which takes the camera as ever.
//
// - One animal at a time, picked at random from everyone at the meeting, the
//   host and the bots too, never the one followed just before, for EACH s.
// - Its EACH s are about four shots of 6–9 s, each from another of the
//   ANGLES: three-quarter front at its eye level, side-on, high looking down,
//   and low from the ground looking up while it stands, or behind it while it
//   walks (low behind a walker, the humps of the ground came between). Each
//   new shot, and each new animal, is a cut. Within a shot the camera keeps
//   its angle to the way the animal faces, coming round as it turns, and aims
//   a little ahead of one on the move (LEAD), as a turn's close-up does.
// - The animal fills about a third to half of the stream's 16:9 picture
//   (FILL), measured by projecting its bounds, so a frog is seen from a hand's
//   width and the standing bear from meters off.
// - Never through things (Sight.ts): an angle is taken only if the camera is
//   out of every tree, boulder, stone and other animal, over the ground and
//   the water, and sees the animal's middle, head and front with nothing in
//   between, now and where the animal will be in a second. One that becomes
//   blocked is cut from, to another angle, once it has shown SETTLE s. An
//   animal with no clear angle is passed over for another.
// - The stage puts the camera exactly where this says, every frame (the
//   meeting marks each one a cut, Shot.cut), so what was checked is what is
//   seen. Easing after it, the stage's camera trailed a low one by up to half
//   a meter and, cutting the corners of its way, went 8 cm into a hump of the
//   ground behind a walking wolf. The camera eases after its place by itself
//   (SMOOTH), so a runner stopping short doesn't jolt it, and is then kept
//   over the ground and checked.

const EACH = 30; // s on each animal
const SHOT = [6, 9] as const; // s a shot lasts
const FILL = [0.36, 0.46] as const; // share of the picture the animal fills, across or up, whichever is more
const FOV = 50; // degrees up and down, the stage's camera's
const ASPECT = 16 / 9; // the stream's picture
const LEAD = 0.8; // s ahead of an animal on the move the camera aims, as Meeting.ts's close-ups
const LEAD_MOST = 0.3; // of the camera's distance, at most, so a runner stays in the picture
const TRACK = 1.5; // how fast the camera comes round as the animal turns, per second
const GAP = 0.02; // m the camera keeps over the ground
const ROOM = 0.1; // m the camera keeps from another animal
const AHEAD = 1; // s ahead an angle must also be clear, for one on the move
const FARTHEST = 15; // m the camera stands from an animal at most
const NEAR_END = 1.3; // the camera stands at least this many times the animal's half length (or width) from its middle, the way it looks
const STRETCHED = 0.6; // share of the picture the animal fills at most, stretched out: it is framed as it stood when the shot began
const SMOOTH = 5; // how fast the camera eases after its place, per second (the stage's own easing is 2.5)
const DOLLY = 3; // how fast the camera eases back as the animal stretches, per second
const SETTLE = 1; // s a shot shows before a blocked view cuts to another
const LAST = 2; // s: blocked with less than this left on an animal, the camera goes on to the next
const RETRY = 1; // s before trying again when no animal can be seen
const UP = new THREE.Vector3(0, 1, 0);
// Radians above level the camera looks down from at least, but for the low
// angle: at a snake's or a frog's eye level, a camera a few centimeters up
// had the humps of the ground between it and the animal.
const LOOK_DOWN = 0.18;
const SKIP = 0.15; // m round the animal, at most, where the ground and what stands there don't count as in the way: it stands there

type Name = 'front' | 'side' | 'low' | 'high' | 'behind';

interface Angle {
  name: Name;
  turn: readonly [number, number]; // radians from the way the animal faces, to either side
  eye?: number; // the camera's height over its feet, as a share of the animal's height
  rise?: readonly [number, number]; // or radians above level the camera looks down from
  least?: number; // radians above level it looks down from at least
  aim: number; // the height the camera looks at, as a share of the animal's height
  when?: 'walking' | 'still'; // only for one on its way, or standing
}

const ANGLES: readonly Angle[] = [
  { name: 'front', turn: [0.55, 0.9], eye: 0.85, aim: 0.55, least: LOOK_DOWN }, // three-quarter front, at its eye level
  { name: 'side', turn: [1.4, 1.75], eye: 0.5, aim: 0.5, least: LOOK_DOWN }, // side-on
  { name: 'low', turn: [0.25, 1.1], eye: 0.08, aim: 0.6, when: 'still' }, // low from the ground, looking up
  { name: 'high', turn: [0.3, 2.3], rise: [0.8, 1.05], aim: 0.4 }, // high, looking down
  { name: 'behind', turn: [2.8, Math.PI], eye: 1.25, aim: 0.6, least: LOOK_DOWN, when: 'walking' }, // behind it as it walks
];

interface Plan {
  angle: Angle;
  turn: number; // radians from its heading, signed
  rise: number;
  fill: number; // the share of the picture it is to fill
  // Its bounds in its own coordinates: as the shot began, and grown as it
  // stretches (a frog's legs flung out behind it, hopping; the bear standing
  // up), and what the camera frames, easing after them.
  box: THREE.Box3;
  easing: THREE.Box3;
  frame: THREE.Box3;
  distance: number; // m from the animal, level, now
  wanted: number; // where it eases to
  toward: number; // and on the way there
}

export interface FollowShot {
  camera: THREE.Vector3;
  target: THREE.Vector3;
  cut: boolean;
}

export class Follow {
  private readonly sight: Sight;
  private readonly random: () => number;
  private readonly lens = new THREE.PerspectiveCamera(FOV, ASPECT, 0.01, 200);
  private member: Member | null = null;
  private last: Member | null = null; // followed just before, not followed next
  private plan: Plan | null = null;
  private time = 0; // s on this animal
  private shotStart = 0; // s on this animal when this shot began
  private shotEnd = 0; // and when it ends
  private used = new Set<Name>(); // the angles it has been seen from
  private side = 0; // radians round from +z the camera looks from now
  private blocked = false;
  private readonly eye = new THREE.Vector3(); // on the camera's way to its place
  private wait = 0; // s before trying again, when no animal could be seen
  private readonly shot: FollowShot = { camera: new THREE.Vector3(), target: new THREE.Vector3(), cut: false };

  constructor(sight: Sight, random: () => number = Math.random) {
    this.sight = sight;
    this.random = random;
  }

  // The animal followed now, if any.
  get following(): Member | null {
    return this.member;
  }

  // The angle it is seen from now, if any.
  get angle(): Name | null {
    return this.plan?.angle.name ?? null;
  }

  // Stops following; the next update() picks another animal, not `seen` (the
  // one the camera was on instead: the host, in its view).
  stop(seen: Member | null = null): void {
    if (seen) this.last = seen;
    if (this.member) this.last = this.member;
    this.member = null;
    this.plan = null;
    this.wait = 0;
  }

  // Moves the camera on by delta seconds; `animals` is everyone at the
  // meeting now. Null when no animal can be seen.
  update(delta: number, animals: Member[]): FollowShot | null {
    this.time += delta;
    this.wait -= delta;
    let cut = false;
    const gone = !this.member || !animals.includes(this.member);
    if (gone || this.time >= EACH) {
      if (this.member) this.last = this.member;
      this.member = null;
      this.plan = null;
      if (this.wait > 0 || !this.begin(animals)) {
        if (this.wait <= 0) this.wait = RETRY;
        return null;
      }
      cut = true;
    } else if (this.time >= this.shotEnd || (this.blocked && this.time - this.shotStart >= SETTLE)) {
      // Blocked near its end: on to another animal rather than a moment's
      // shot (a deer running under a crown left 0.7 s for another angle).
      if (this.time > EACH - LAST || !this.nextShot(this.member!, animals)) {
        // No clear angle left on it: another animal.
        this.stop();
        if (!this.begin(animals)) {
          this.wait = RETRY;
          return null;
        }
      }
      cut = true;
    }
    const member = this.member!;
    const plan = this.plan!;
    const want = member.figure.rotation.y + plan.turn;
    this.side += shortest(want - this.side) * (cut ? 1 : 1 - Math.exp(-TRACK * delta));
    // As it stretches past STRETCHED of the picture, the camera eases back: a
    // frog's legs, flung out 8 cm behind it, came at a camera 22 cm off.
    const now = measure(member.figure);
    if (!plan.box.containsBox(now)) {
      plan.box.union(now);
      plan.wanted = Math.max(plan.wanted, this.fit(member, plan, want, plan.box, STRETCHED));
    }
    // Eased twice over, so that it starts and stops gently: eased once, the
    // camera beside a running wolf set off at once, 38 m/s² in a frame.
    const ease = 1 - Math.exp(-DOLLY * delta);
    plan.toward += (plan.wanted - plan.toward) * ease;
    plan.distance += (plan.toward - plan.distance) * ease;
    plan.easing.min.lerp(plan.box.min, ease);
    plan.easing.max.lerp(plan.box.max, ease);
    plan.frame.min.lerp(plan.easing.min, ease);
    plan.frame.max.lerp(plan.easing.max, ease);
    // The camera eases after its place, twice over so that it starts and
    // stops gently, and what it looks at once, then keeps over the ground.
    const view = this.view(member, plan, this.side, 0);
    const shot = this.shot;
    if (cut) {
      this.eye.copy(view.camera);
      shot.camera.copy(view.camera);
      shot.target.copy(view.target);
    } else {
      const t = 1 - Math.exp(-SMOOTH * delta);
      this.eye.lerp(view.camera, t);
      shot.camera.lerp(this.eye, t);
      shot.target.lerp(view.target, t);
      shot.camera.y = Math.max(shot.camera.y, this.sight.floor(shot.camera.x, shot.camera.z) + GAP);
    }
    this.blocked = !this.clear(member, plan, shot, animals, 0);
    shot.cut = cut;
    return shot;
  }

  // Picks an animal to follow, at random, never the last one, with a clear
  // first shot.
  private begin(animals: Member[]): boolean {
    const others = animals.filter((m) => m !== this.last);
    const order = shuffle(others.length > 0 ? others : animals, this.random);
    for (const member of order) {
      this.member = member;
      this.time = 0;
      this.shotEnd = 0;
      this.used.clear();
      this.plan = null;
      if (this.nextShot(member, animals)) return true;
    }
    this.member = null;
    this.plan = null;
    return false;
  }

  // The next shot of it: an angle it hasn't been seen from yet (another than
  // the last, when all have been used), clear now and in a second, lasting
  // so that the shots fill EACH s. False when no angle is clear.
  private nextShot(member: Member, animals: Member[]): boolean {
    const walking = member.moving;
    const current = this.plan?.angle;
    const usable = ANGLES.filter((a) => a !== current && (!a.when || a.when === (walking ? 'walking' : 'still')));
    let fresh = usable.filter((a) => !this.used.has(a.name));
    if (fresh.length === 0) {
      this.used.clear();
      fresh = usable;
    }
    const box = measure(member.figure);
    for (const angle of shuffle(fresh, this.random)) {
      const first = this.random() < 0.5 ? 1 : -1;
      for (const sign of [first, -first]) {
        const plan = this.planFor(member, angle, sign, box);
        const side = member.figure.rotation.y + plan.turn;
        if (!this.clear(member, plan, this.view(member, plan, side, 0), animals, 0)) continue;
        if (walking && !this.clear(member, plan, this.view(member, plan, side, AHEAD), animals, AHEAD)) continue;
        this.plan = plan;
        this.side = side;
        this.used.add(angle.name);
        this.blocked = false;
        // As long as leaves the rest of its time to whole shots of 6–9 s.
        const left = EACH - this.time;
        const shots = Math.max(1, Math.round(left / ((SHOT[0] + SHOT[1]) / 2)));
        const least = Math.max(SHOT[0], left - SHOT[1] * (shots - 1));
        const most = Math.min(SHOT[1], left - SHOT[0] * (shots - 1));
        let length = shots === 1 || least > most ? left / shots : least + (most - least) * this.random();
        if (left - length < SHOT[0] / 2) length = left;
        this.shotStart = this.time;
        this.shotEnd = this.time + length;
        return true;
      }
    }
    return false;
  }

  // An angle's shot of the animal: how far back the camera stands for it to
  // fill its share of the picture.
  private planFor(member: Member, angle: Angle, sign: number, box: THREE.Box3): Plan {
    const random = this.random;
    const turn = sign * THREE.MathUtils.lerp(angle.turn[0], angle.turn[1], random());
    const rise = angle.rise ? THREE.MathUtils.lerp(angle.rise[0], angle.rise[1], random()) : 0;
    const fill = THREE.MathUtils.lerp(FILL[0], FILL[1], random());
    const plan: Plan = { angle, turn, rise, fill, box, easing: box.clone(), frame: box.clone(), distance: 0, wanted: 0, toward: 0 };
    plan.distance = plan.wanted = plan.toward = this.fit(member, plan, member.figure.rotation.y + turn, box, fill);
    return plan;
  }

  // How far back the camera stands for bounds of the animal to fill `fill`
  // of the picture, seen from `side`. The share falls as the camera stands
  // further off, so it is found by halving: near a long animal seen end on,
  // its near end looms, and the share goes nothing like one over the
  // distance.
  private fit(member: Member, plan: Plan, side: number, box: THREE.Box3, fill: number): number {
    const turn = plan.turn;
    const size = box.getSize(new THREE.Vector3());
    // Never over the animal itself: a snake seen from behind, a thin line
    // 1.5 m long, fills a third of the picture only from above its tail.
    const least = NEAR_END * ((size.x / 2) * Math.abs(Math.sin(turn)) + (size.z / 2) * Math.abs(Math.cos(turn))) + 0.05;
    const share = (distance: number) => this.filled(member.figure, box, this.view(member, plan, side, 0, distance, box));
    if (share(least) <= fill) return least;
    let near = least;
    let far = FARTHEST;
    for (let i = 0; i < 12; i++) {
      const middle = Math.sqrt(near * far);
      if (share(middle) > fill) near = middle;
      else far = middle;
    }
    return far;
  }

  // Where the camera is and what it looks at, for a plan seen from `side`,
  // with the animal `ahead` s further on its way: `distance` m from it,
  // framing `box`.
  private view(member: Member, plan: Plan, side: number, ahead: number, distance = plan.distance, box = plan.frame): { camera: THREE.Vector3; target: THREE.Vector3 } {
    const figure = member.figure;
    const heading = figure.rotation.y;
    const forward = new THREE.Vector3(Math.sin(heading), 0, Math.cos(heading));
    const angle = plan.angle;
    const feet = figure.position.clone().addScaledVector(forward, figure.speed * ahead);
    const height = box.max.y - box.min.y;
    const middle = box.getCenter(new THREE.Vector3()).setY(0).applyAxisAngle(UP, heading);
    const aim = feet.clone().add(middle);
    aim.y = feet.y + box.min.y + angle.aim * height;
    // The camera keeps its distance from the animal, and looks a little
    // ahead of one on the move: measured from where it looks, the camera
    // behind a runner came 30% closer.
    const camera = new THREE.Vector3(aim.x + Math.sin(side) * distance, 0, aim.z + Math.cos(side) * distance);
    camera.y = angle.rise ? aim.y + Math.tan(plan.rise) * distance : feet.y + box.min.y + (angle.eye ?? 0.5) * height;
    if (angle.least) camera.y = Math.max(camera.y, aim.y + Math.tan(angle.least) * distance);
    const target = aim.addScaledVector(forward, Math.min(figure.speed * LEAD, LEAD_MOST * distance));
    camera.y = Math.max(camera.y, this.sight.floor(camera.x, camera.z) + GAP);
    return { camera, target };
  }

  // The share of the picture the animal fills from a view, across or up,
  // whichever is more.
  private filled(figure: Figure, box: THREE.Box3, view: { camera: THREE.Vector3; target: THREE.Vector3 }): number {
    const lens = this.lens;
    lens.position.copy(view.camera);
    lens.lookAt(view.target);
    lens.updateMatrixWorld();
    figure.updateMatrix();
    const corner = new THREE.Vector3();
    let left = Infinity;
    let right = -Infinity;
    let bottom = Infinity;
    let top = -Infinity;
    for (let i = 0; i < 8; i++) {
      corner.set(i & 1 ? box.max.x : box.min.x, i & 2 ? box.max.y : box.min.y, i & 4 ? box.max.z : box.min.z);
      corner.applyMatrix4(figure.matrix).applyMatrix4(lens.matrixWorldInverse);
      if (corner.z > -lens.near) return Infinity; // it reaches past the camera: too close
      corner.applyMatrix4(lens.projectionMatrix);
      left = Math.min(left, corner.x);
      right = Math.max(right, corner.x);
      bottom = Math.min(bottom, corner.y);
      top = Math.max(top, corner.y);
    }
    return Math.max(right - left, top - bottom) / 2;
  }

  // Whether the camera sees the animal (`ahead` s further on its way): out
  // of everything solid and every other animal, and nothing between it and
  // the animal's middle, head and front.
  private clear(member: Member, plan: Plan, view: { camera: THREE.Vector3; target: THREE.Vector3 }, animals: Member[], ahead: number): boolean {
    const { camera } = view;
    const sight = this.sight;
    if (sight.blocked(camera)) return false;
    const others = animals.filter((m) => m !== member);
    if (others.some((other) => inside(other, camera, ROOM))) return false;
    const figure = member.figure;
    const heading = figure.rotation.y;
    const feet = figure.position.clone().addScaledVector(new THREE.Vector3(Math.sin(heading), 0, Math.cos(heading)), figure.speed * ahead);
    const box = plan.box;
    const middle = box.getCenter(new THREE.Vector3());
    const height = box.max.y - box.min.y;
    const skip = Math.min(member.radius, SKIP);
    const point = new THREE.Vector3();
    for (const local of [
      middle,
      new THREE.Vector3(middle.x, box.min.y + 0.9 * height, middle.z),
      new THREE.Vector3(middle.x, box.min.y + 0.6 * height, box.max.z - 0.15 * (box.max.z - box.min.z)),
    ]) {
      const at = local.clone().applyAxisAngle(UP, heading).add(feet);
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

// Whether a point is within an animal: in the circle it takes up on the
// ground and below its height, and `room` m more.
export function inside(member: Member, point: THREE.Vector3, room: number): boolean {
  const at = member.figure.position;
  return Math.hypot(point.x - at.x, point.z - at.z) < member.radius + room && point.y < at.y + member.height + room && point.y > at.y - room;
}

// A figure's bounds as it stands now, in its own coordinates, without its
// speech bubble.
export function measure(figure: Figure): THREE.Box3 {
  figure.updateMatrixWorld(true);
  const inverse = figure.matrixWorld.clone().invert();
  const bubble = new Set<THREE.Object3D>();
  figure.speechBubble.traverse((object) => bubble.add(object));
  const box = new THREE.Box3();
  const part = new THREE.Box3();
  const matrix = new THREE.Matrix4();
  figure.traverseVisible((object) => {
    if (!(object instanceof THREE.Mesh) || bubble.has(object)) return;
    const geometry: THREE.BufferGeometry = object.geometry;
    if (!geometry.boundingBox) geometry.computeBoundingBox();
    box.union(part.copy(geometry.boundingBox!).applyMatrix4(matrix.multiplyMatrices(inverse, object.matrixWorld)));
  });
  return box;
}

function shuffle<T>(items: readonly T[], random: () => number): T[] {
  const list = [...items];
  for (let i = list.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1));
    [list[i], list[j]] = [list[j], list[i]];
  }
  return list;
}
