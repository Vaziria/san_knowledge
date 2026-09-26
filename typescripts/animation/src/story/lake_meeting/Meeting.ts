import * as THREE from 'three';
import { fetchMeeting, onMeeting } from '../../chat';
import { FAR_SHORE, type DockPlace } from '../../environtments/Lake/DockSite';
import { Lake } from '../../environtments/Lake/Lake';
import type { LogsPlace } from '../../environtments/Lake/Logs';
import { Fish } from '../../figures/Fish/Fish';
import type { Environment, Shot } from '../../previews';
import type { Theme } from '../../theme';
import { HOST, KINDS, type Command, type Kind, type Member as Info, type MeetingEvent, type MeetingState, type Numbered } from './events';
import { Follow, type FollowShot } from './Follow';
import { disposeFigure, isPenguin } from './kinds';
import { Land, type Circle } from './Land';
import { Member, shortest } from './Member';
import { FishLeaps } from './FishLeaps';
import { HostView, type HostShot } from './HostView';
import { Roam } from './Roam';
import { Sight } from './Sight';

// The lake meeting (lake_meeting.md) as a scene: the host's Bear, the
// viewers' animals and the bots roaming the lake's land all round the water
// (Roam.ts, Land.ts), the host setting off from the landing, and the fish
// swimming a loop in the water. The dev server says who comes and goes and what they say or
// do (chat-bridge.ts, meeting.ts); this shows it.
//
// - Coming and going: an animal appears at a spot anywhere on the land,
//   landing there with a jump; one that leaves walks off to the land's end
//   and runs the last part, out of sight. One destroyed (the oldest, when the
//   meeting is full) or removed by a moderator is gone at once; a destroyed
//   one with a comment or command still waiting goes once it has had its turn.
// - Turns: each comment, command and supporter's leap waits its turn, in the
//   order sent, so nothing is missed: the camera goes to the animal (or the
//   fish), wherever it is, then its bubble pops up or it does the command, and
//   the next turn starts once that is over. An animal stops walking while it
//   talks. The camera comes from in front of the animal, a little to one
//   side, or from another side where a trunk, a boulder, the ground or
//   another animal would be in the way; one sent walking or running it
//   follows from beside it (ALONGSIDE). With more than MANY waiting, or an
//   animal CUT_BEYOND off, the camera cuts instead of gliding. After
//   WIDE_AFTER seconds with nothing to show it goes back to the host's view,
//   wherever the host is (HostView.ts), and after FOLLOW_AFTER it follows one
//   animal after another, from a few angles (Follow.ts), the host not first,
//   until the next turn. A viewer who has
//   moved the camera away (a drag, a picked direction, walking) keeps it,
//   following or not, until the next turn, which claims it back
//   (Shot.claim), so no bubble pops up too far off to read.
// - The sun's shadow follows what the camera looks at (Preview.followShadow),
//   at the size of `bounds`, so an animal far out keeps its shadow.
//
// Units are meters; the origin is the landing (Lake.LANDING), where the stage
// puts a story's figure, and +z points away from the water.

export const CLEAR = 5.6; // m round the landing the lake keeps level and mown for the meeting
const HOST_START = new THREE.Vector3(0, 0, -0.5); // on the landing, facing +z: its first place, which it roams from
const WIDE = { camera: new THREE.Vector3(0, 2.3, 7.4), target: new THREE.Vector3(0, 0.45, 1.3) }; // the opening shot: the landing, the host in the middle
const GLIDE_TIME = 1.2; // s a turn gives the camera to get there
const CUT_TIME = 0.25; // and after a cut
const MANY = 2; // turns waiting behind the next one for the camera to cut
const CUT_BEYOND = 12; // m from where the camera is to the next turn's view past which it cuts
const WIDE_AFTER = 3.5; // s with nothing to show before the camera goes back to the host's view
const FOLLOW_AFTER = 30; // s with nothing to show before it follows the animals in turn (Follow.ts)
const HOLD = { jump: 1.8, stop: 1.2, flap: 1.6, switch: 1.6, go: 14, leap: 7 }; // s a turn lasts at most (go: !walk and !run)
const BUBBLE_HEIGHT = 0.16; // m, a bubble of two or three lines at the penguin's size, before its scale
const LEAD = 0.8; // s ahead of an animal on the move that the camera aims
// m from a speech bubble of scale 1 (the penguin's) at most, so its text is
// some 24 px tall in a 720p stream; a bubble twice the size, twice as far.
const READABLE = 1.05;
const SIDES = [0.35, -0.35, 0.9, -0.9, 1.6, -1.6, 2.4, -2.4, Math.PI]; // radians from an animal's heading the camera may look from, best first
const ALONGSIDE = [1.2, -1.2, 0.6, -0.6]; // radians from its heading the camera follows one walking or running from, best first: beside it, a little ahead
const TRACK = 1.5; // how fast the camera comes round to stay beside it as it turns, per second
const IN_VIEW = 0.3; // m across: an obstacle narrower than this (a stone) doesn't block the camera's view
const FISH = { middle: new THREE.Vector3(0, 0, 3.6), half: 1.6 }; // its loop, in the lake's coordinates
// Where the fish goes, for the dock to keep clear of: LOOP_ROOM m round its
// loop, and along each side the way it swims there, up to LEAP_REACH m on
// and LEAP_BAND m to either side, since a supporter's leap starts wherever
// the fish is and carries it up to 3.5 m on.
const LOOP_ROOM = 1;
const LEAP_REACH = 4;
const LEAP_BAND = 1;
function fishGoes(x: number, z: number): boolean {
  const { middle: c, half } = FISH;
  const [x0, x1, z0, z1] = [c.x - half, c.x + half, c.z - half, c.z + half];
  const box = (ax: number, bx: number, az: number, bz: number) => x > ax && x < bx && z > az && z < bz;
  return (
    box(x0 - LOOP_ROOM, x1 + LOOP_ROOM, z0 - LOOP_ROOM, z1 + LOOP_ROOM) ||
    box(x0, x1 + LEAP_REACH, z1 - LEAP_BAND, z1 + LEAP_BAND) || // along z1 toward +x, on the surface
    box(x1 - LEAP_BAND, x1 + LEAP_BAND, z0 - LEAP_REACH, z1) || // along x1 toward -z, deep
    box(x0 - LEAP_REACH, x1, z0 - LEAP_BAND, z0 + LEAP_BAND) || // along z0 toward -x, deep
    box(x0 - LEAP_BAND, x0 + LEAP_BAND, z0, z1 + LEAP_REACH) // along x0 toward +z, on the surface
  );
}
// The meeting's dock (task 09, as the user asked: "in lake meeting add dock
// in the lake"): looked for from the far shore, across the water from the
// landing, where the cameras look, and clear of where the fish goes
// (lake_meeting.ts builds the lake with it).
export const DOCK: DockPlace = { angle: FAR_SHORE, keepOff: fishGoes };
// The logs floating in the meeting's lake (task 11, as the user asked: "in
// lake metting, add random log in lake"): kept off where the fish goes too
// (lake_meeting.ts builds the lake with them).
export const LOGS: LogsPlace = { keepOff: fishGoes };
const FISH_LEAP_FROM = 1.2; // m into a side of its loop within which the fish may leap of its own accord: a leap of its own carries it 1.25–1.6 m

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
}

interface View {
  camera: THREE.Vector3;
  target: THREE.Vector3;
}

export class Meeting extends THREE.Group {
  static readonly WIDE = WIDE;
  // The size of the sun's shadow round what the camera looks at.
  readonly bounds = new THREE.Box3(new THREE.Vector3(-5, 0, -5), new THREE.Vector3(5, 2.5, 5));
  private readonly theme: Theme;
  private readonly ground: (x: number, z: number) => number;
  private readonly land: Land;
  private readonly roam: Roam;
  private readonly blocks: Circle[]; // trunks and boulders, which block the camera's view
  private readonly sight: Sight; // what a camera may not be in or see through
  private readonly host: Member;
  private readonly members = new Map<string, Member>();
  private readonly leaving = new Set<Member>(); // walking away, gone once at the land's end
  private readonly doomed = new Set<string>(); // destroyed, waiting for their turns first
  private readonly lake = new THREE.Group(); // the lake's own coordinates, where the fish swims
  private readonly fish: Fish;
  private readonly fishLeaps: FishLeaps; // its own leaps, now and then
  private readonly swim: [start: () => void, done: () => boolean][];
  private swimStep = -1;
  private readonly queue: Turn[] = [];
  private active: Active | null = null;
  private pendingLeap = 0; // m, a leap asked for and not yet begun
  private quiet = 0; // s since the last turn ended, or since the meeting began
  private view: View;
  private readonly follow: Follow;
  private following: FollowShot | null = null; // the camera following an animal, while it is quiet
  private readonly hostView: HostView;
  private hostShot: HostShot | null = null; // the host's view, a while after a turn
  private cutNext = false;
  private claimNext = false; // a turn has begun since the stage last asked for the shot
  private seq = 0;
  private buffered: Numbered[] | null = []; // events that came before the meeting as it stood
  private readonly unsubscribe: () => void;
  private disposed = false;

  constructor(theme: Theme, environment: Environment) {
    super();
    this.name = 'lake meeting';
    this.theme = theme;
    const land = environment.land;
    this.ground = (x, z) => (environment.ground?.heightAt(x + land.x, z + land.z) ?? land.y) - land.y;

    // The land the animals roam, as the lake says it is: its ground (under
    // the water below 0) and its dock's deck, how far it reaches, and what
    // stands on it.
    const lake = environment.scenery instanceof Lake ? environment.scenery : null;
    const obstacles = (lake?.obstacles() ?? []).map(({ x, z, radius }) => ({ x: x - land.x, z: z - land.z, radius }));
    const deck = lake?.deck;
    this.land = new Land({
      ground: lake ? (x, z) => Math.max(lake.groundAt(x + land.x, z + land.z), lake.deckAt(x + land.x, z + land.z)) : () => 1,
      middle: new THREE.Vector2(-land.x, -land.z),
      reach: environment.ground?.reach ?? 20,
      obstacles,
      decks: deck ? [{ ...deck, x: deck.x - land.x, z: deck.z - land.z }] : [],
    });
    this.blocks = obstacles.filter((o) => 2 * o.radius >= IN_VIEW);
    const sight = (this.sight = new Sight({ lake, offset: land, ground: this.ground }));
    this.follow = new Follow(sight);
    this.hostView = new HostView(sight);

    // The host starts on the landing, where the opening shot shows it, and
    // roams from there like the others.
    this.host = new Member(HOST, '', 'bear', false, theme);
    this.host.figure.position.copy(HOST_START);
    this.add(this.host.figure);
    this.roam = new Roam(this.land);
    this.roam.start(this.host);

    // The fish, in the lake's own coordinates: its water surface is y = 0 there.
    this.lake.position.set(-land.x, -land.y, -land.z);
    this.add(this.lake);
    const fish = (this.fish = new Fish({ theme }));
    const water = environment.water;
    if (water) fish.addEventListener('splash', ({ x, z, velocity }) => water.splash(x, z, velocity));
    const { middle: c, half } = FISH;
    this.swim = [
      [() => fish.SwimOnSurface('right'), () => fish.position.x > c.x + half],
      [() => fish.SwimOnDepth('back'), () => fish.position.z < c.z - half],
      [() => fish.SwimOnDepth('left'), () => fish.position.x < c.x - half],
      [() => fish.SwimOnSurface('forward'), () => fish.position.z > c.z + half],
    ];
    fish.position.set(c.x - half, fish.surfaceY, c.z + half);
    fish.rotation.y = Math.PI / 2;
    this.lake.add(fish);
    this.fishLeaps = new FishLeaps(fish);

    this.view = { camera: WIDE.camera.clone(), target: WIDE.target.clone() };
    this.unsubscribe = onMeeting((numbered) => (this.buffered ? this.buffered.push(numbered) : this.apply(numbered)));
    void fetchMeeting().then((state) => this.begin(state));
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
    if (this.swimStep < 0 || this.swim[this.swimStep][1]()) {
      this.swimStep = (this.swimStep + 1) % this.swim.length;
      this.swim[this.swimStep][0]();
    }
    if (this.pendingLeap > 0) {
      this.fish.JumpOutFromWater(this.pendingLeap); // ignored until the fish can
      if (this.fish.jumping) this.pendingLeap = 0;
    }
    this.fishLeaps.update(delta, this.pendingLeap > 0 || this.active?.turn.type === 'support', this.fishCanLeap());
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

  // Whether the fish can leap of its own accord now: along one of its loop's
  // sides on the surface (right, then forward), facing that way, in the first
  // FISH_LEAP_FROM m of it, so that the leap lands before the side ends and it
  // swims on round its loop.
  private fishCanLeap(): boolean {
    const { middle: c, half } = FISH;
    const { position: p, rotation: r } = this.fish;
    if (this.swimStep === 0) return Math.abs(shortest(r.y - Math.PI / 2)) < 0.3 && p.x - (c.x - half) < FISH_LEAP_FROM;
    if (this.swimStep === 3) return Math.abs(shortest(r.y)) < 0.3 && p.z - (c.z - half) < FISH_LEAP_FROM;
    return false;
  }

  dispose(): void {
    this.disposed = true;
    this.unsubscribe();
  }

  // The meeting as the dev server has it, then the events since. Without a
  // dev server (a built page), bots of its own.
  private begin(state: MeetingState | null): void {
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

  private apply({ seq, event }: Numbered): void {
    if (seq <= this.seq) return;
    this.seq = seq;
    this.handle(event);
  }

  private handle(event: MeetingEvent): void {
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
  private join(info: Info, jump: boolean): void {
    if (this.members.has(info.id)) return;
    const member = new Member(info.id, info.name, info.kind, info.bot, this.theme);
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
        if (turn.type === 'say') this.roam.hold(member);
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
      this.active = { turn, time: 0, acted: null, cut, side, alongside, leap: null };
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
      const old = member.change(turn.kind, this.theme);
      old.removeFromParent();
      disposeFigure(old);
      this.add(member.figure);
      this.roam.resized(member);
      return;
    }
    switch (turn.command) {
      case 'jump':
        return figure.Jump();
      case 'flap':
        if (isPenguin(figure)) figure.Flap();
        return;
      case 'stop':
        return this.roam.stop(member);
      case 'walk':
      case 'run':
        return this.roam.send(member, turn.command);
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
      default:
        return since > HOLD[turn.command];
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
    camera.y = Math.max(camera.y, this.ground(camera.x, camera.z) + 0.3);
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

  // Whether the camera sees its target: no trunk or boulder, no ground and no
  // other animal in between.
  private clearView({ camera, target }: View, member: Member): boolean {
    // Not in a tree's crown, a trunk, a boulder or a stone, nor seeing
    // through one (Sight.ts): a roaming host under a tree had its owner's
    // close-up in the crown, its bubble over the leaves.
    if (this.sight.blocked(camera) || !this.sight.sees(target, camera, Math.min(member.radius, 0.15))) return false;
    const others = [this.host, ...this.members.values()].filter((m) => m !== member);
    const point = new THREE.Vector3();
    // From the camera to just short of the animal itself.
    for (let s = 0; s <= 10; s++) {
      point.lerpVectors(camera, target, s / 12);
      if (point.y < this.ground(point.x, point.z) + 0.05) return false;
      if (this.blocks.some((b) => Math.hypot(point.x - b.x, point.z - b.z) < b.radius + 0.15)) return false;
      for (const other of others) {
        const at = other.figure.position;
        if (Math.hypot(point.x - at.x, point.z - at.z) < other.radius && point.y < at.y + other.height) return false;
      }
    }
    return true;
  }

  // The fish where it swims, from the shore's side; once it leaps, from the
  // side of its leap (the side toward the shore), far enough back to take in
  // the arc, following the fish.
  private fishShot(leap: Leap | null): View {
    const at = this.lake.position.clone().add(this.fish.position);
    const surface = this.lake.position.y;
    if (!leap) {
      const target = new THREE.Vector3(at.x, surface + 0.5, at.z);
      return { camera: target.clone().add(new THREE.Vector3(1.4, 1, 3.6)), target };
    }
    // A little ahead of it, the way it leaps, since the camera follows behind.
    const target = new THREE.Vector3(at.x, surface + 0.2 + 0.5 * leap.height, at.z).addScaledVector(leap.ahead, 0.3 + 0.8 * leap.height);
    const camera = target.clone().addScaledVector(leap.side, 2.4 + 2.2 * leap.height);
    camera.y += 0.5;
    return { camera, target };
  }

  // Which side the leap is seen from: across the way the fish faces as it
  // leaps, toward the shore.
  private leapSide(height: number): Leap {
    const heading = this.fish.rotation.y;
    const ahead = new THREE.Vector3(Math.sin(heading), 0, Math.cos(heading));
    const side = new THREE.Vector3(ahead.z, 0, -ahead.x);
    if (side.z < 0) side.negate();
    return { ahead, side, height };
  }
}

interface Leap {
  ahead: THREE.Vector3; // the way it leaps
  side: THREE.Vector3; // the way from the fish to the camera
  height: number; // m
}
