import * as THREE from 'three';
import { fetchMeeting, onMeeting } from '../../chat';
import { ForestLake } from '../../environtments/ForestLake/ForestLake';
import { ForestFish } from '../../figures/ForestLake/animals/ForestFish';
import type { Season } from '../../figures/ForestLake/parts';
import type { Environment, Shot } from '../../previews';
import type { Theme } from '../../theme';
import { HOST } from '../lake_meeting/events';
import { FishLeaps } from '../lake_meeting/FishLeaps';
import { Follow, type FollowShot } from '../lake_meeting/Follow';
import { HostView, type HostShot } from '../lake_meeting/HostView';
import { disposeFigure } from '../lake_meeting/kinds';
import type { Circle, Land } from '../lake_meeting/Land';
import { Member, shortest } from '../lake_meeting/Member';
import { Roam } from '../lake_meeting/Roam';
import { KINDS, type Command, type ForestEvent, type ForestMember, type ForestNumbered, type ForestState, type Kind } from './events';
import { ForestSight } from './ForestSight';
import { forestGround } from './ground';
import { isGuest, makeGuest } from './guests';

// The forest lake meeting (forest_lake_meeting.md) as a scene: the lake
// meeting (lake_meeting/Meeting.ts) at the forest lake, with its own
// animals. The explorer is the host; the viewers' animals and the bots are
// the forest lake's wolf, deer, fox, boar, rabbit, squirrel, duck, otter and
// frog (guests.ts); a trout swims a circle in the open water north of the
// landing. The dev server says who comes and goes and what they say or do
// (chat-bridge.ts, meeting.ts), from the same chat as the lake meeting; this
// shows it.
//
// It plays as the lake meeting plays, with its modules: every animal roams
// the land (Roam.ts, on the forest lake's land: ground.ts), turns take the
// camera one after another in the order sent (below, as Meeting.ts), the
// host's view comes back when the chat is quiet (HostView.ts) and the follow
// after 30 s (Follow.ts), never in or through anything (ForestSight.ts). Its
// own:
//
// - Each kind's tricks, from its sheet (events.ts, TRICKS; !howl, !flap,
//   !sit, !look, !eat, !stand, !wave): the animal stops and does it, and its
//   turn lasts as long as the trick.
// - The fish swims round FISH m from the open water's middle (in winter the
//   hole in the ice). For a supporter's leap it first turns to face the
//   circle's middle, so that it leaps across the open water, not onto the
//   ice; its own small leaps go the way it swims. A leap's height is the
//   lake meeting's, 0.3–1.5 m, brought within what the forest fish leaps,
//   0.3–1 m (LEAP).
//
// Units are meters; the origin is the landing (ForestLake.LANDING), where
// the stage puts a story's figure, and +z points away from the water.

const MIDDLE = new THREE.Vector2(0.5, -6); // the lake's middle, in its own coordinates
const REACH = 46; // m from it the land the animals roam reaches, the last 7 m kept off (Land.ts)
const LOOK_OUT = 12; // m past that the cameras' sight is measured
const HOST_START = new THREE.Vector3(0, 0, -0.5); // on the landing, facing +z: its first place, which it roams from
const WIDE = { camera: new THREE.Vector3(0, 2.3, 7.4), target: new THREE.Vector3(0, 0.55, 1.3) }; // the opening shot: the landing, the host in the middle
const GLIDE_TIME = 1.2; // s a turn gives the camera to get there
const CUT_TIME = 0.25; // and after a cut
const MANY = 2; // turns waiting behind the next one for the camera to cut
const CUT_BEYOND = 12; // m from where the camera is to the next turn's view past which it cuts
const WIDE_AFTER = 3.5; // s with nothing to show before the camera goes back to the host's view
const FOLLOW_AFTER = 30; // s with nothing to show before it follows the animals in turn (Follow.ts)
const HOLD = { jump: 1.8, stop: 1.2, switch: 1.6, go: 14, leap: 8 }; // s a turn lasts at most (go: !walk and !run); a trick, as long as it takes
const BUBBLE_HEIGHT = 0.16; // m, a bubble of two or three lines at the penguin's size, before its scale
const LEAD = 0.8; // s ahead of an animal on the move that the camera aims
// m from a speech bubble of scale 1 (the lake penguin's) at most, so its
// text is some 24 px tall in a 720p stream; a bubble twice the size, twice
// as far.
const READABLE = 1.05;
const SIDES = [0.35, -0.35, 0.9, -0.9, 1.6, -1.6, 2.4, -2.4, Math.PI]; // radians from an animal's heading the camera may look from, best first
const ALONGSIDE = [1.2, -1.2, 0.6, -0.6]; // radians from its heading the camera follows one walking or running from, best first
const TRACK = 1.5; // how fast the camera comes round to stay beside it as it turns, per second
const IN_VIEW = 0.3; // m across: an obstacle narrower than this doesn't block the camera's view
const FISH = { radius: 1.4, ahead: 0.9, turn: 1.2 }; // m round the open water's middle it swims; radians ahead on its circle it steers for; radians a second it turns at most
const AIMED = 0.2; // radians off facing the middle within which a supporter's leap starts
const LEAP = { least: 0.3, most: 1 }; // m the forest fish leaps for a supporter: the lake meeting's 0.3–1.5 m, brought within these
const PLAIN = new Set<Command>(['jump', 'walk', 'run', 'stop']); // every animal's commands; the rest are tricks

type Turn =
  | { type: 'say'; id: string; name: string; text: string; message: string }
  | { type: 'command'; id: string; command: Command }
  | { type: 'switch'; id: string; kind: Kind }
  | { type: 'support'; id: string; name: string; what: string; height: number };

interface Active {
  turn: Turn;
  time: number; // s since it began
  acted: number | null; // s into it when the animal spoke or acted
  cut: boolean;
  side: number; // radians round from +z the camera looks at its animal from
  alongside: number | null; // for one sent walking or running: radians from its heading the camera keeps to
  leap: Leap | null; // the fish's leap, once asked for: the side it is seen from
  trick: number; // s its trick lasts, for a trick
}

interface View {
  camera: THREE.Vector3;
  target: THREE.Vector3;
}

export class ForestMeeting extends THREE.Group {
  static readonly WIDE = WIDE;
  // The size of the sun's shadow round what the camera looks at.
  readonly bounds = new THREE.Box3(new THREE.Vector3(-5, 0, -5), new THREE.Vector3(5, 2.5, 5));
  private readonly theme: Theme;
  private readonly season: Season;
  private readonly ground: (x: number, z: number) => number;
  private readonly land: Land;
  private readonly roam: Roam;
  private readonly blocks: Circle[]; // trunks, rocks and footprints, which block the camera's view
  private readonly sight: ForestSight; // what a camera may not be in or see through
  private readonly host: Member;
  private readonly members = new Map<string, Member>();
  private readonly leaving = new Set<Member>(); // walking away, gone once at the land's end
  private readonly doomed = new Set<string>(); // destroyed, waiting for their turns first
  private readonly water = new THREE.Group(); // the lake's own coordinates, where the fish swims
  private readonly fish: ForestFish;
  private readonly middle: THREE.Vector2; // of the fish's circle, in the lake's coordinates
  private readonly fishLeaps: FishLeaps; // its own leaps, now and then
  private readonly queue: Turn[] = [];
  private active: Active | null = null;
  private pendingLeap = 0; // m, a supporter's leap asked for and not yet begun
  private quiet = 0; // s since the last turn ended, or since the meeting began
  private view: View;
  private readonly follow: Follow;
  private following: FollowShot | null = null; // the camera following an animal, while it is quiet
  private readonly hostView: HostView;
  private hostShot: HostShot | null = null; // the host's view, a while after a turn
  private cutNext = false;
  private claimNext = false; // a turn has begun since the stage last asked for the shot
  private seq = 0;
  private buffered: ForestNumbered[] | null = []; // events that came before the meeting as it stood
  private readonly unsubscribe: () => void;
  private disposed = false;

  constructor(theme: Theme, environment: Environment) {
    super();
    this.name = 'forest lake meeting';
    this.theme = theme;
    const lake = environment.scenery;
    if (!(lake instanceof ForestLake)) throw new Error('the forest lake meeting plays at the forest lake');
    this.season = lake.season;
    const land = environment.land;

    // The land the animals roam, and what the cameras see.
    const ground = forestGround(lake, land, MIDDLE, REACH);
    this.ground = ground.standAt;
    this.land = ground.land;
    this.blocks = ground.obstacles.filter((o) => 2 * o.radius >= IN_VIEW);
    const sight = (this.sight = new ForestSight({ lake, offset: land, middle: MIDDLE, half: REACH + LOOK_OUT }));
    this.follow = new Follow(sight);
    this.hostView = new HostView(sight);

    // The host starts on the landing, where the opening shot shows it, and
    // roams from there like the others; standing at a spot, an animal now
    // and then does a trick of its own.
    this.host = new Member(HOST, '', 'explorer', false, makeGuest('explorer', HOST, this.season, theme));
    this.host.figure.position.copy(HOST_START);
    this.add(this.host.figure);
    this.roam = new Roam(this.land, Math.random, (member, random) => {
      const figure = member.figure;
      if (isGuest(figure)) figure.Pastime(random);
      else figure.Jump();
    });
    this.roam.start(this.host);

    // The fish, in the lake's own coordinates: its water's still surface is
    // y = 0 there. It swims round the open water north of the landing, in
    // winter the hole kept open in the ice.
    this.water.position.set(-land.x, -land.y, -land.z);
    this.add(this.water);
    this.middle = new THREE.Vector2(ForestLake.OPEN_WATER.x, ForestLake.OPEN_WATER.z);
    const fish = (this.fish = new ForestFish());
    fish.addEventListener('splash', ({ x, z, velocity }) => lake.water.splash(x, z, velocity));
    fish.position.set(this.middle.x + FISH.radius, fish.swimY, this.middle.y);
    fish.rotation.y = Math.PI; // on its way round
    fish.Swim();
    this.water.add(fish);
    this.fishLeaps = new FishLeaps({ get jumping() { return fish.jumping; }, JumpOutFromWater: (height) => fish.Jump(height) });

    this.view = { camera: WIDE.camera.clone(), target: WIDE.target.clone() };
    this.unsubscribe = onMeeting((numbered) => (this.buffered ? this.buffered.push(numbered) : this.apply(numbered)), 'forest_lake_meeting');
    void fetchMeeting('forest_lake_meeting').then((state) => this.begin(state));
  }

  // Where the camera should be now (Preview.shot).
  shot(): Shot {
    const { cutNext: cut, claimNext: claim } = this;
    this.cutNext = false;
    this.claimNext = false;
    return { camera: this.view.camera, target: this.view.target, cut, claim };
  }

  update(delta: number): void {
    const all = [this.host, ...this.members.values(), ...this.leaving];
    this.roam.update(delta, all);
    for (const member of all) member.update(delta, this.ground);
    this.swim(delta);
    this.fishLeaps.update(delta, this.pendingLeap > 0 || this.active?.turn.type === 'support', this.fish.doing === 'swim');
    this.fish.update(delta);

    if (!this.active) this.next();
    const active = this.active;
    if (active) {
      active.time += delta;
      // Beside one walking or running, coming round as it turns.
      const walker = active.alongside !== null ? this.member(active.turn.id) : undefined;
      if (walker && active.alongside !== null) active.side += shortest(walker.figure.rotation.y + active.alongside - active.side) * (1 - Math.exp(-TRACK * delta));
      if (active.acted === null && active.time >= (active.cut ? CUT_TIME : GLIDE_TIME)) {
        this.act(active);
        active.acted = active.time;
      }
      if (active.acted !== null && this.over(active, active.time - active.acted)) this.finish(active);
    } else {
      this.quiet += delta;
    }
    // Quiet for WIDE_AFTER s: the host's view, wherever the host is; for
    // FOLLOW_AFTER s: the camera follows the animals in turn, the host not
    // first, which the host's view has just shown. Both move smoothly by
    // themselves, and the stage puts the camera exactly there.
    const animals = [this.host, ...this.members.values()];
    if (!this.active && this.quiet > FOLLOW_AFTER) {
      this.hostView.stop();
      this.hostShot = null;
      this.following = this.follow.update(delta, animals);
      if (this.following) this.cutNext = true;
    } else {
      this.follow.stop(this.host);
      this.following = null;
      if (!this.active && this.quiet > WIDE_AFTER) {
        this.hostShot = this.hostView.update(delta, this.host, animals, this.view, CUT_BEYOND);
        if (this.hostShot) this.cutNext = true;
      } else {
        this.hostView.stop();
        this.hostShot = null;
      }
    }
    this.frame();
  }

  dispose(): void {
    this.disposed = true;
    this.unsubscribe();
  }

  // The fish round its circle, steering for a point on it a little ahead;
  // for a supporter's leap, it first turns to face the circle's middle, then
  // leaps across the open water.
  private swim(delta: number): void {
    const fish = this.fish;
    if (fish.jumping || fish.doing === 'turn') return;
    const c = this.middle;
    const p = fish.position;
    if (this.pendingLeap > 0) {
      const off = shortest(Math.atan2(c.x - p.x, c.y - p.z) - fish.rotation.y);
      if (Math.abs(off) > AIMED) return fish.Turn(THREE.MathUtils.radToDeg(off));
      fish.Jump(THREE.MathUtils.clamp(LEAP.least + ((this.pendingLeap - 0.3) * (LEAP.most - LEAP.least)) / 1.2, LEAP.least, LEAP.most));
      if (fish.jumping) this.pendingLeap = 0;
      return;
    }
    if (fish.doing !== 'swim') fish.Swim();
    const round = Math.atan2(p.z - c.y, p.x - c.x) + FISH.ahead;
    const want = Math.atan2(c.x + FISH.radius * Math.cos(round) - p.x, c.y + FISH.radius * Math.sin(round) - p.z);
    const most = FISH.turn * delta;
    fish.rotation.y += THREE.MathUtils.clamp(shortest(want - fish.rotation.y), -most, most);
  }

  // The meeting as the dev server has it, then the events since. Without a
  // dev server (a built page), bots of its own.
  private begin(state: ForestState | null): void {
    if (this.disposed) return;
    const early = this.buffered ?? [];
    this.buffered = null;
    if (!state) {
      for (let i = 0; i < 7; i++) {
        this.join({ id: `bot-here-${i}`, name: '', kind: KINDS[Math.floor(Math.random() * KINDS.length)], bot: true }, false);
      }
      return;
    }
    this.seq = state.seq;
    for (const member of state.members) this.join(member, false);
    for (const numbered of early) this.apply(numbered);
  }

  private apply({ seq, event }: ForestNumbered): void {
    if (seq <= this.seq) return;
    this.seq = seq;
    this.handle(event);
  }

  private handle(event: ForestEvent): void {
    switch (event.type) {
      case 'join':
        return this.join(event.member, true);
      case 'leave':
        return this.leave(event.id, event.how);
      case 'say':
      case 'command':
      case 'switch':
      case 'support':
        this.queue.push(event);
        return;
      case 'delete': {
        const at = this.queue.findIndex((t) => t.type === 'say' && t.message === event.message);
        if (at >= 0) this.queue.splice(at, 1);
        const active = this.active;
        if (active?.turn.type === 'say' && active.turn.message === event.message) {
          this.member(active.turn.id)?.figure.Speech('');
          this.finish(active);
        }
        return;
      }
    }
  }

  private member(id: string): Member | undefined {
    return id === HOST ? this.host : this.members.get(id);
  }

  // An animal comes: at a spot anywhere on the land, landing there with a
  // jump (`jump`), or standing there already as the meeting begins.
  private join(info: ForestMember, jump: boolean): void {
    if (this.members.has(info.id)) return;
    const member = new Member(info.id, info.name, info.kind, info.bot, makeGuest(info.kind, info.id, this.season, this.theme));
    this.members.set(info.id, member);
    this.add(member.figure);
    this.roam.place(member, jump);
  }

  private leave(id: string, how: 'walk' | 'destroy' | 'remove'): void {
    const member = this.members.get(id);
    if (!member) return;
    if (how === 'remove') {
      for (let i = this.queue.length - 1; i >= 0; i--) if (this.queue[i].id === id) this.queue.splice(i, 1);
      if (this.active?.turn.id === id) this.finish(this.active);
      return this.dismiss(member);
    }
    if (how === 'destroy') {
      if (this.hasTurns(id)) this.doomed.add(id);
      else this.dismiss(member);
      return;
    }
    // Walks off to the land's end and runs the last part, out of sight.
    this.members.delete(id);
    this.leaving.add(member);
    this.roam.leave(member, () => {
      this.leaving.delete(member);
      member.dispose();
    });
  }

  private dismiss(member: Member): void {
    this.members.delete(member.id);
    this.doomed.delete(member.id);
    this.roam.remove(member);
    member.dispose();
  }

  private hasTurns(id: string): boolean {
    return this.active?.turn.id === id || this.queue.some((t) => t.id === id);
  }

  // The next turn whose animal is still here. One that talks stops walking,
  // so its words stay steady.
  private next(): void {
    while (this.queue.length > 0) {
      const turn = this.queue.shift()!;
      const member = turn.type === 'support' ? null : this.member(turn.id);
      if (turn.type !== 'support' && !member) continue;
      let cut = this.queue.length >= MANY;
      let side = 0;
      let alongside: number | null = null;
      if (member) {
        // One that talks or does a trick stops at once, so the camera finds
        // it facing the way it was seen from.
        if (turn.type === 'say' || (turn.type === 'command' && !PLAIN.has(turn.command))) this.roam.hold(member);
        const moves = turn.type === 'command' && (turn.command === 'walk' || turn.command === 'run');
        if (moves) {
          alongside = this.sideFor(member, ALONGSIDE) - member.figure.rotation.y;
          side = member.figure.rotation.y + alongside;
        } else {
          side = this.sideFor(member);
        }
        cut ||= this.view.camera.distanceTo(this.closeUp(member, side).camera) > CUT_BEYOND;
      } else {
        cut ||= this.view.camera.distanceTo(this.fishShot(null).camera) > CUT_BEYOND;
      }
      this.active = { turn, time: 0, acted: null, cut, side, alongside, leap: null, trick: 0 };
      this.cutNext = cut;
      this.claimNext = true;
      return;
    }
  }

  // Once the camera is there: the animal says it, or does it, or the fish
  // leaps.
  private act(active: Active): void {
    const turn = active.turn;
    if (turn.type === 'support') {
      this.pendingLeap = turn.height;
      active.leap = this.leapSide(turn.height);
      return;
    }
    const member = this.member(turn.id);
    if (!member) return;
    const figure = member.figure;
    if (turn.type === 'say') return figure.Speech(turn.text, turn.name);
    if (turn.type === 'switch') {
      const old = member.change(turn.kind, makeGuest(turn.kind, member.id, this.season, this.theme));
      old.removeFromParent();
      disposeFigure(old);
      this.add(member.figure);
      this.roam.resized(member);
      return;
    }
    switch (turn.command) {
      case 'jump':
        return figure.Jump();
      case 'stop':
        return this.roam.stop(member);
      case 'walk':
      case 'run':
        return this.roam.send(member, turn.command);
      default: {
        // A trick of its kind, where it stopped as its turn began; then it
        // roams on. Without one (it has since become another kind), it roams
        // on at once.
        const seconds = isGuest(figure) ? figure.trickLength(turn.command) : 0;
        if (!seconds || !isGuest(figure)) return this.roam.release(member);
        this.roam.hold(member, seconds + 0.3);
        figure.Trick(turn.command);
        active.trick = seconds;
      }
    }
  }

  // Whether a turn is over, `since` seconds after the animal acted.
  private over(active: Active, since: number): boolean {
    const turn = active.turn;
    if (turn.type === 'support') return (this.pendingLeap === 0 && !this.fish.jumping && since > 1) || since > HOLD.leap;
    const member = this.member(turn.id);
    if (!member) return true;
    if (turn.type === 'say') return !member.figure.speaking && since > 0.4;
    if (turn.type === 'switch') return since > HOLD.switch;
    switch (turn.command) {
      case 'walk':
      case 'run':
        return (since > 0.5 && !this.roam.moving(member)) || since > HOLD.go;
      case 'jump':
      case 'stop':
        return since > HOLD[turn.command];
      default:
        return since > active.trick;
    }
  }

  private finish(active: Active): void {
    if (this.active !== active) return;
    this.active = null;
    this.quiet = 0;
    const id = active.turn.id;
    const member = this.member(id);
    if (member && active.turn.type === 'say') this.roam.release(member);
    if (member && this.doomed.has(id) && !this.hasTurns(id)) this.dismiss(member);
  }

  // Where the camera goes this frame: the turn's animal, or the fish, or the
  // host's view once it has been quiet a while, or an animal it follows once
  // it has been quiet longer. Until then it stays where the turn left it, or
  // at the opening shot.
  private frame(): void {
    const active = this.active;
    if (active?.turn.type === 'support') return this.set(this.fishShot(active.leap));
    const member = active && this.member(active.turn.id);
    if (member) return this.set(this.closeUp(member, active.side));
    if (this.following) return this.set(this.following);
    if (this.hostShot) this.set(this.hostShot);
  }

  private set(view: View): void {
    this.view.camera.copy(view.camera);
    this.view.target.copy(view.target);
  }

  // An animal and what it says above it, from `side` (radians round from +z):
  // as far back as it takes to fit both in, but never so far that the words
  // are too small to read; then the bubble is kept at the top of the
  // picture, and a tall animal's feet may be out of it. One on the move is
  // framed a little ahead of it, the way it goes, so the camera, following,
  // keeps it in the picture. The camera stays above the ground.
  private closeUp(member: Member, side: number): View {
    const figure = member.figure;
    const bubble = figure.speechBubble;
    const top = Math.max(member.height, bubble.position.y + BUBBLE_HEIGHT * bubble.scale.y);
    const distance = THREE.MathUtils.clamp(Math.min(1.3 * top, READABLE * bubble.scale.y), 0.6, 6);
    const seen = 2 * distance * Math.tan(THREE.MathUtils.degToRad(25)); // m of height in the picture there
    const target = new THREE.Vector3(figure.position.x, figure.position.y + Math.max(0.5 * top, top - 0.42 * seen), figure.position.z);
    const heading = figure.rotation.y;
    target.x += Math.sin(heading) * figure.speed * LEAD;
    target.z += Math.cos(heading) * figure.speed * LEAD;
    const camera = target.clone().add(new THREE.Vector3(Math.sin(side) * distance, 0.25 * top + 0.15, Math.cos(side) * distance));
    camera.y = Math.max(camera.y, this.sight.floor(camera.x, camera.z) + 0.3);
    return { camera, target };
  }

  // The side to see an animal from: the first of `sides` (from its heading)
  // from which nothing is in the way; by default in front of it, a little
  // to one side.
  private sideFor(member: Member, sides = SIDES): number {
    const heading = member.figure.rotation.y;
    for (const turn of sides) {
      const side = heading + turn;
      if (this.clearView(this.closeUp(member, side), member)) return side;
    }
    return heading + sides[0];
  }

  // Whether the camera sees its target: nothing solid, no ground and no
  // other animal in between.
  private clearView({ camera, target }: View, member: Member): boolean {
    if (this.sight.blocked(camera) || !this.sight.sees(target, camera, Math.min(member.radius, 0.15))) return false;
    const others = [this.host, ...this.members.values()].filter((m) => m !== member);
    const point = new THREE.Vector3();
    // From the camera to just short of the animal itself.
    for (let s = 0; s <= 10; s++) {
      point.lerpVectors(camera, target, s / 12);
      if (point.y < this.ground(point.x, point.z) + 0.05) return false;
      if (this.blocks.some((b) => Math.hypot(point.x - b.x, point.z - b.z) < b.radius + 0.15 && point.y < this.ground(b.x, b.z) + 1.3)) return false;
      for (const other of others) {
        const at = other.figure.position;
        if (Math.hypot(point.x - at.x, point.z - at.z) < other.radius && point.y < at.y + other.height) return false;
      }
    }
    return true;
  }

  // The fish where it swims, from the shore's side; once it leaps, from the
  // side of its leap (the side toward the shore), far enough back to take in
  // the arc (up to 3.4 m long), following the fish. Nearer than the lake
  // meeting's camera to its salmon: the trout is 40 cm long.
  private fishShot(leap: Leap | null): View {
    const at = this.water.position.clone().add(this.fish.position);
    const surface = this.water.position.y;
    if (!leap) {
      const target = new THREE.Vector3(at.x, surface + 0.35, at.z);
      return { camera: target.clone().add(new THREE.Vector3(1, 0.8, 2.6)), target };
    }
    // A little ahead of it, the way it leaps, since the camera follows behind.
    const target = new THREE.Vector3(at.x, surface + 0.2 + 0.5 * leap.height, at.z).addScaledVector(leap.ahead, 0.3 + 0.8 * leap.height);
    const camera = target.clone().addScaledVector(leap.side, 1.9 + 1.7 * leap.height);
    camera.y = Math.max(camera.y + 0.5, this.sight.floor(camera.x, camera.z) + 0.3);
    return { camera, target };
  }

  // Which side a supporter's leap is seen from: across the way the fish will
  // leap (toward its circle's middle), toward the shore.
  private leapSide(height: number): Leap {
    const p = this.fish.position;
    const ahead = new THREE.Vector3(this.middle.x - p.x, 0, this.middle.y - p.z);
    if (ahead.lengthSq() < 1e-6) ahead.set(Math.sin(this.fish.rotation.y), 0, Math.cos(this.fish.rotation.y));
    ahead.normalize();
    const side = new THREE.Vector3(ahead.z, 0, -ahead.x);
    if (side.z < 0) side.negate();
    return { ahead, side, height: THREE.MathUtils.clamp(LEAP.least + ((height - 0.3) * (LEAP.most - LEAP.least)) / 1.2, LEAP.least, LEAP.most) };
  }
}

interface Leap {
  ahead: THREE.Vector3; // the way it leaps
  side: THREE.Vector3; // the way from the fish to the camera
  height: number; // m
}
