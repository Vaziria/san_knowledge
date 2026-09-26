import * as THREE from 'three';
import { isPenguin } from './kinds';
import { GAP, type Circle, type Land } from './Land';
import type { Member } from './Member';

// How the meeting's animals roam the lake's land (Meeting.ts), the host too,
// as the user asked ("make animal walk randomly at the map"; "make the bear
// roaming like other too"): each
// walks to a spot, stands there a while (sometimes jumping), then picks
// another, now and then running. Most spots are within NEAR m, so a walk
// stays in one part of the lake; now and then (FAR) the spot is anywhere on
// the land, evenly by area, a long walk round to another part; and now and
// then (DOCK) it is out on the lake's dock, if it has one. The way there
// goes round the lake and what stands on its land (Land.ts).
//
// Keeping apart: a spot is picked clear of every other animal and of where
// each is going. A way keeps CLOSE m more than it
// needs from those standing still. On the way, an animal that finds another
// standing in its way goes round it (a new way), or stops first when it is
// too close to turn aside. Of two on their way into each other, the one the
// other is further ahead of gives way (head on, by their ids), and the other
// goes round where it will come to a stand. Before all that, one going toward
// another already within CLOSE m and its stopping distance of it stops at
// once. Two waiting for each other take turns, and after WAIT s one goes
// round the other. Nothing ever pushes an animal, so its feet keep to its
// steps.
//
// A new animal appears at a spot anywhere on the land, landing there with a
// jump; the host starts from the landing, where it stands as the meeting
// begins. One that leaves walks to the land's end the way it is from the
// lake's middle, then goes (Meeting.ts). One that talks stops while its
// bubble shows (hold, release); !stop stops it for a while; !walk and !run
// send it to a new spot near it.

const REST = [3, 15] as const; // s it stands at a spot
const FIRST = [1, 6] as const; // s before one that has just appeared, or was there already, sets off
const AFTER_HOLD = [0.5, 2] as const; // s before one that was held sets off again
const JUMP = 0.3; // chance it jumps while it stands
const RUN = 0.2; // chance it runs to its next spot
const FAR = 0.15; // chance its next spot is anywhere on the land
const DOCK = 0.1; // chance its next spot is out on the dock, where the lake has one: evenly by area, one spot in 200 would be
const NEAR = 12; // m: otherwise its next spot is within this
const MARGIN = { walk: 0.15, run: 0.5 }; // m a way keeps from everything beyond what the animal needs: running, it swings wider round corners
const LOOK = 1.5; // s ahead an animal on its way looks for another in its way
const CLOSE = 0.4; // m more than two animals need between them that counts as too close, on the way: room for one to ease to a stand
const FACING = 15; // s at most one waits for another standing close in front of it before it finds a way round anyway
const STOP_TIME = 0.3; // s: an animal told to stop goes on about its speed times this (it eases down at 4 a second)
const WAIT = 2; // s it waits for a moving one to pass before going round it
const CALM = 0.8; // s after finding a new way before it finds another round one standing in it (it waits instead)
const HELD = [4, 10] as const; // s one stopped by !stop stands before it roams on
const ROOM = 0.35; // m more a spot keeps from everything than the animal needs: arriving, it may stop a little off it
const PAST = 40; // of its last stops a new spot keeps PAST_ROOM m from, and from its first place: it never goes back
const PAST_ROOM = 1; // m
const SHIFT = 2.5; // m at most that one switched to a bigger kind appears from where it stood, to fit

type State = 'rest' | 'go' | 'wait' | 'held' | 'leave';

interface Roaming {
  state: State;
  time: number; // s: left to rest or be held; waited so far
  jumpAt: number; // s of rest left when it jumps; below 0 for none
  to: THREE.Vector2 | null; // where it is going
  way: THREE.Vector2[]; // the points it walks through to get there
  gait: 'walk' | 'run';
  calm: number; // s left before it looks for standing ones in its way
  blocker: Member | null; // the one it waits for
  gone: (() => void) | null; // for one leaving: once it is there
}

// Where each has stopped: its first place, always, and the last PAST places
// it came to a stand (at a spot, talking, told to stop, or waiting for
// another), latest last.
const stops = new WeakMap<Member, THREE.Vector2[]>();

function stopped(member: Member): void {
  const [first, ...since] = stops.get(member) ?? [member.at];
  stops.set(member, [first, ...[...since, member.at].slice(-PAST)]);
}

export class Roam {
  private readonly land: Land;
  private readonly random: () => number;
  private readonly roaming = new Map<Member, Roaming>();

  constructor(land: Land, random: () => number = Math.random) {
    this.land = land;
    this.random = random;
  }

  // An animal standing where it is as the meeting begins (the host, at the
  // landing): its first place, which it never goes back to, and it roams
  // from there after a moment, as those already there do.
  start(member: Member): void {
    stops.set(member, [member.at]);
    this.roaming.set(member, { ...idle(), time: between(this.random, FIRST) });
  }

  // An animal that has just come, or was there as the meeting began: at a
  // free spot anywhere on the land, landing there with a jump (`jump`), and
  // roaming from there after a moment.
  place(member: Member, jump: boolean): void {
    const spot = this.land.spot(this.random, member.radius + ROOM, this.kept(member)) ?? this.land.spot(this.random, member.radius + ROOM, []);
    const figure = member.figure;
    if (spot) figure.position.set(spot.x, 0, spot.y);
    figure.rotation.y = 2 * Math.PI * this.random();
    if (jump) figure.Jump();
    stops.set(member, [member.at]);
    this.roaming.set(member, { ...idle(), time: between(this.random, FIRST) });
  }

  // It no longer roams: gone, or destroyed.
  remove(member: Member): void {
    this.roaming.delete(member);
  }

  // It stops where it is: while it talks (until release()), or for a while
  // (!stop).
  hold(member: Member, seconds = Infinity): void {
    const roaming = this.roaming.get(member);
    if (!roaming || roaming.state === 'leave') return;
    member.stop();
    stopped(member);
    Object.assign(roaming, { ...idle(), state: 'held', time: seconds });
  }

  // It roams on after a hold.
  release(member: Member): void {
    const roaming = this.roaming.get(member);
    if (roaming?.state === 'held') Object.assign(roaming, { ...idle(), time: between(this.random, AFTER_HOLD) });
  }

  // !stop: it stands a while where it is.
  stop(member: Member): void {
    this.hold(member, between(this.random, HELD));
  }

  // !walk, !run: to a new spot near it.
  send(member: Member, gait: 'walk' | 'run'): void {
    const roaming = this.roaming.get(member);
    if (!roaming || roaming.state === 'leave') return;
    this.setOff(member, roaming, gait, false);
  }

  // It became another kind in its place, of another size. Bigger, where it
  // no longer fits (against another animal or a trunk), the new one appears
  // at the nearest spot where it does, SHIFT m at most: a new figure
  // appearing, not one sliding. On its way, it finds a way that fits it.
  resized(member: Member): void {
    const roaming = this.roaming.get(member);
    const others: Circle[] = [...this.roaming.keys()].filter((other) => other !== member).map(circle);
    const at = member.at;
    if (!this.land.free(at.x, at.y, member.radius, others)) {
      const spot = this.land.nearestFree(at, member.radius, others, SHIFT);
      if (spot) member.figure.position.set(spot.x, member.figure.position.y, spot.y);
    }
    if (roaming && (roaming.state === 'go' || roaming.state === 'leave') && roaming.to) this.reroute(member, roaming, null);
  }

  // Walks to the land's end, the way it is from the lake's middle, running
  // the last part; `gone` runs once it is there. Without a way there, gone at
  // once.
  leave(member: Member, gone: () => void): void {
    const roaming = this.roaming.get(member) ?? idle();
    this.roaming.set(member, roaming);
    const edge = this.land.edgeToward(member.at, member.radius, this.random);
    const way = edge && this.land.path(member.at, edge, member.radius, MARGIN.walk, this.standing(member));
    if (!edge || !way) {
      this.roaming.delete(member);
      return gone();
    }
    Object.assign(roaming, { ...idle(), state: 'leave', to: edge, gait: 'walk', gone });
    this.follow(member, roaming, way);
  }

  // Whether it is on its way somewhere.
  moving(member: Member): boolean {
    const state = this.roaming.get(member)?.state;
    return state === 'go' || state === 'leave';
  }

  // Moves every roaming animal on; `all` is every animal on the land, to
  // keep apart from.
  update(delta: number, all: Member[]): void {
    for (const [member, roaming] of this.roaming) {
      switch (roaming.state) {
        case 'rest':
          roaming.time -= delta;
          if (roaming.jumpAt >= 0 && roaming.time <= roaming.jumpAt) {
            roaming.jumpAt = -1;
            if (isPenguin(member.figure) && this.random() < 0.4) member.figure.Flap();
            else member.figure.Jump();
          }
          if (roaming.time <= 0) this.setOff(member, roaming, this.random() < RUN ? 'run' : 'walk', this.random() < FAR);
          break;
        case 'held':
          roaming.time -= delta;
          if (roaming.time <= 0) Object.assign(roaming, { ...idle(), time: between(this.random, AFTER_HOLD) });
          break;
        case 'wait':
          roaming.time += delta;
          this.waitOn(member, roaming, all);
          break;
        case 'go':
        case 'leave':
          roaming.calm -= delta;
          this.keepApart(member, roaming, all);
          break;
      }
    }
  }

  // Sets off to a new spot: near it, or anywhere on the land (`far`).
  private setOff(member: Member, roaming: Roaming, gait: 'walk' | 'run', far: boolean): void {
    // Clear of the others, and of where it has been: it keeps each new spot
    // and walks on from there, never back.
    const kept = [...this.kept(member), ...(stops.get(member) ?? []).map(({ x, y }) => ({ x, z: y, radius: PAST_ROOM }))];
    // Now and then out on the dock; otherwise near it, or, with nowhere new
    // left near, anywhere.
    let to = this.random() < DOCK ? this.land.deckSpot(this.random, member.radius + ROOM, kept) : null;
    to ??= far ? null : this.land.spot(this.random, member.radius + ROOM, kept, member.at, NEAR);
    to ??= this.land.spot(this.random, member.radius + ROOM, kept);
    const way = to && (this.land.path(member.at, to, member.radius, MARGIN[gait], this.standing(member)) ?? this.land.path(member.at, to, member.radius, MARGIN.walk, this.standing(member)));
    if (!to || !way) {
      // Nowhere to go from here just now: it tries again soon.
      Object.assign(roaming, { ...idle(), time: between(this.random, FIRST) });
      return;
    }
    Object.assign(roaming, { ...idle(), state: 'go', to, gait });
    this.follow(member, roaming, way);
  }

  // Walks the way; at its end it rests, or, leaving, is gone.
  private follow(member: Member, roaming: Roaming, way: THREE.Vector2[]): void {
    const gait = roaming.gait;
    roaming.way = way;
    const last = way.length - 1;
    member.walk(
      way.map((point, i) => ({ to: new THREE.Vector3(point.x, 0, point.y), gait: roaming.state === 'leave' && i === last ? 'run' : gait })),
      () => {
        if (roaming.state === 'leave') {
          this.roaming.delete(member);
          roaming.gone?.();
          return;
        }
        stopped(member);
        const time = between(this.random, REST);
        Object.assign(roaming, { ...idle(), time, jumpAt: this.random() < JUMP ? time * this.random() : -1 });
      },
    );
    roaming.calm = CALM;
  }

  // A new way to where it is going, from where it stands, round the standing
  // ones and `also` (one it waited for, or where one will stand). Without
  // one, it stands and picks another spot soon, or, leaving, goes on as it
  // can.
  private reroute(member: Member, roaming: Roaming, also: Member | Circle | null): void {
    const to = roaming.to;
    const others = this.standing(member);
    if (also) others.push('figure' in also ? round(also) : { ...also, radius: also.radius + CLOSE });
    if (roaming.state === 'wait') roaming.state = roaming.gone ? 'leave' : 'go';
    roaming.blocker = null;
    let way = to && this.land.path(member.at, to, member.radius, MARGIN[roaming.gait], others);
    if (to && !way && roaming.gait === 'run') {
      roaming.gait = 'walk';
      way = this.land.path(member.at, to, member.radius, MARGIN.walk, others);
    }
    if (!way) {
      if (roaming.state === 'leave') {
        this.roaming.delete(member);
        return roaming.gone?.();
      }
      member.stop();
      stopped(member);
      Object.assign(roaming, { ...idle(), time: between(this.random, AFTER_HOLD) });
      return;
    }
    this.follow(member, roaming, way);
  }

  // On its way: another standing on its way is gone round, or, when it is
  // too close to turn aside in time (or it has only just found a way), it
  // stops for it first. Of two on their way into each other, one gives way.
  private keepApart(member: Member, roaming: Roaming, all: Member[]): void {
    // First, room round each: going toward one already this close, it stops
    // at once, whatever the ways say (they bend, and the guesses along them
    // come late).
    for (const other of all) {
      if (other === member) continue;
      const offset = other.at.sub(member.at);
      const room = member.radius + other.radius + GAP + CLOSE + stopping(member);
      if (offset.length() < room && velocity(member).dot(offset) > 0) return this.wait(member, roaming, other);
    }
    for (const other of all) {
      if (other === member) continue;
      if (!this.moving(other)) {
        const near = this.onMyWay(member, roaming, circle(other));
        if (near === null) continue;
        const room = near - stopping(member);
        if (roaming.calm <= 0 && room > 0.5) return this.reroute(member, roaming, null);
        return this.wait(member, roaming, other);
      }
      if (!this.meeting(member, other)) continue;
      if (this.givesWay(member, other)) return this.wait(member, roaming, other);
      // It has the way, and the other stops for it: but where the other comes
      // to a stand must not be on its way.
      const heading = other.figure.rotation.y;
      const stand = { x: other.figure.position.x + Math.sin(heading) * stopping(other), z: other.figure.position.z + Math.cos(heading) * stopping(other), radius: other.radius };
      if (this.onMyWay(member, roaming, stand) !== null) {
        if (roaming.calm <= 0) return this.reroute(member, roaming, stand);
        return this.wait(member, roaming, other);
      }
    }
  }

  // Waiting for another: once it is out of the way, on again; one standing
  // is gone round once this one has stopped; after WAIT s, round whichever
  // it is.
  private waitOn(member: Member, roaming: Roaming, all: Member[]): void {
    const other = roaming.blocker;
    if (!other || !all.includes(other)) return this.reroute(member, roaming, null);
    const stopped = member.figure.speed < 0.05;
    if (!this.moving(other)) {
      // Two waiting for each other take turns: the one whose id comes first
      // goes round, and the other waits until it has gone.
      // Facing it this close, a new way would set it walking on into it (it
      // can't turn on the spot), and the first check of keepApart stop it
      // again at once, a little further in each time: a snake crept 0.75 m
      // into a fox that way in 20 s. So it waits for the other to go.
      const theirs = this.roaming.get(other);
      const mine = facing(member, other);
      if (theirs?.state === 'wait' && theirs.blocker === member) {
        // Two waiting for each other take turns: the one not facing the
        // other goes round, since it walks off without walking into it (a
        // deer facing a frog crept into it, going round by its id); with
        // both or neither facing, the one whose id comes first.
        const goes = mine !== facing(other, member) ? !mine : member.id < other.id;
        if (!goes) return;
      } else if (roaming.time < FACING && mine) {
        return;
      }
      if (stopped) this.reroute(member, roaming, null);
      return;
    }
    if (roaming.time > WAIT && stopped) return this.reroute(member, roaming, other);
    if (stopped && this.onMyWay(member, roaming, circle(other)) === null && !this.meeting(member, other, true)) this.reroute(member, roaming, null);
  }

  private wait(member: Member, roaming: Roaming, other: Member): void {
    member.stop();
    stopped(member);
    Object.assign(roaming, { state: 'wait', time: 0, blocker: other });
  }

  // How far along its way ahead, over the next LOOK s and the distance it
  // needs to stop, it comes too close to another standing there (its
  // footprint), or null.
  private onMyWay(member: Member, roaming: Roaming, other: Circle): number | null {
    const need = member.radius + other.radius + GAP + CLOSE;
    const at = member.at;
    const there = new THREE.Vector2(other.x, other.z);
    const reach = Math.max(member.figure.speed, 0.6) * LOOK + stopping(member) + member.radius;
    if (at.distanceTo(there) > reach + need) return null;
    // Along the rest of its way: the next of its points onward.
    const ahead = member.ahead.length ? member.ahead.map((step) => new THREE.Vector2(step.to.x, step.to.z)) : roaming.way;
    let from = at;
    let gone = 0;
    for (const point of ahead) {
      const length = from.distanceTo(point);
      for (let s = 0; s <= length && gone + s <= reach; s += 0.1) {
        const x = from.x + ((point.x - from.x) * s) / Math.max(length, 1e-6);
        const z = from.y + ((point.y - from.y) * s) / Math.max(length, 1e-6);
        if (Math.hypot(x - there.x, z - there.y) < need && gone + s > 0) return gone + s;
      }
      gone += length;
      if (gone > reach) break;
      from = point;
    }
    return null;
  }

  // Whether two on their way would come too close over the next LOOK s,
  // going on as they go now (for one standing still waiting: as if it walked
  // on toward its next point).
  private meeting(member: Member, other: Member, asIfWalking = false): boolean {
    const need = member.radius + other.radius + GAP + CLOSE;
    const offset = other.at.sub(member.at);
    const mine = velocity(member);
    if (asIfWalking && mine.lengthSq() < 0.01) {
      const heading = member.figure.rotation.y;
      mine.set(Math.sin(heading), Math.cos(heading)).multiplyScalar(0.8);
    }
    const relative = velocity(other).sub(mine);
    const speed2 = relative.lengthSq();
    const t = speed2 < 1e-6 ? 0 : THREE.MathUtils.clamp(-offset.dot(relative) / speed2, 0, LOOK);
    return offset.clone().addScaledVector(relative, t).length() < need;
  }

  // Of two on their way into each other, the one the other is further ahead
  // of gives way (the other would reach the crossing first); head on, the one
  // whose id comes first. Never one that the other is already waiting for.
  private givesWay(member: Member, other: Member): boolean {
    const theirs = this.roaming.get(other);
    if (theirs?.state === 'wait' && theirs.blocker === member) return false;
    const offset = other.at.sub(member.at);
    const mine = new THREE.Vector2(Math.sin(member.figure.rotation.y), Math.cos(member.figure.rotation.y));
    const their = new THREE.Vector2(Math.sin(other.figure.rotation.y), Math.cos(other.figure.rotation.y));
    const aheadOfMe = offset.dot(mine); // how far ahead of this one the other is
    const aheadOfThem = -offset.dot(their);
    if (Math.abs(aheadOfMe - aheadOfThem) > 0.3) return aheadOfMe > aheadOfThem;
    return member.id < other.id;
  }

  // What a spot for it must keep clear of: every other animal, and where each
  // is going.
  private kept(member: Member): Circle[] {
    const kept: Circle[] = [];
    for (const [other, roaming] of this.roaming) {
      if (other === member) continue;
      kept.push(circle(other));
      if (roaming.to) kept.push({ x: roaming.to.x, z: roaming.to.y, radius: other.radius });
    }
    return kept;
  }

  // The animals standing still, which a way goes round, keeping CLOSE more
  // from each: passing nearer, it would have to stop for it (keepApart).
  private standing(member: Member): Circle[] {
    const standing: Circle[] = [];
    for (const other of this.roaming.keys()) {
      if (other !== member && !this.moving(other)) standing.push(round(other));
    }
    return standing;
  }
}

// Whether it faces another within the room the first check of keepApart
// keeps: a new way would set it walking on into it.
function facing(member: Member, other: Member): boolean {
  const offset = other.at.sub(member.at);
  const heading = member.figure.rotation.y;
  const room = member.radius + other.radius + GAP + CLOSE + stopping(member);
  return offset.length() < room && offset.x * Math.sin(heading) + offset.y * Math.cos(heading) > 0;
}

// How far it goes on before it stands, told to stop now.
function stopping(member: Member): number {
  return member.figure.speed * STOP_TIME;
}

function idle(): Roaming {
  return { state: 'rest', time: 0, jumpAt: -1, to: null, way: [], gait: 'walk', calm: 0, blocker: null, gone: null };
}

// Which way it faces, and how fast it goes that way, as a velocity on the
// ground.
function velocity(member: Member): THREE.Vector2 {
  const heading = member.figure.rotation.y;
  return new THREE.Vector2(Math.sin(heading), Math.cos(heading)).multiplyScalar(member.figure.speed);
}

// Its footprint and the room kept round it on the way (CLOSE).
function round(member: Member): Circle {
  const { x, z } = member.figure.position;
  return { x, z, radius: member.radius + CLOSE };
}

function circle(member: Member): Circle {
  const { x, z } = member.figure.position;
  return { x, z, radius: member.radius };
}

function between(random: () => number, [least, most]: readonly [number, number]): number {
  return least + (most - least) * random();
}
