import * as THREE from 'three';
import { Log } from '../../figures/objects/Log';
import { between, seededRandom } from '../../figures/objects/parts';
import type { Theme } from '../../theme';
import type { Wind } from '../wind';
import { LAKE_RADIUS } from './Ground';

// Logs floating in the lake (LakeOptions.logs; the lake meeting's, as the
// user asked: "in lake metting, add random log in lake"): COUNT of them, each
// a Log of its own seed and size, round underneath (Log's `round`: a flat
// face would show through the water). Their places and sizes come from a
// seeded generator, so the lake is the same on every load.
//
// - Floating: a log lies DRAFT of its trunk's thickness down in the water. It
//   rides the waves as the boat's preview does (Environments rule 4): it
//   feels the water toward either end and at its sides, and its height,
//   pitch and roll ease toward it, so it rocks more when the wind roughens
//   the water (Water.setWind). Round, it rolls to only ROLL of the water's
//   slope across it.
// - Drifting: each keeps to its own stretch of water, within ROAM m of its
//   home. The wind pushes it downwind, a few centimeters a second in a full
//   wind, and turns it a little (DRIFT, TURN); in still air it comes back,
//   slower, and a slow eddy wanders it round (EDDY). It swings a little on
//   its own too (SWING).
// - Its home is found so that wherever it drifts and however it turns, all
//   of it stays over water deep enough that it never touches the bed
//   (UNDER), which keeps it well off the shore, and clear of the boulders,
//   the dock, whatever the lake is asked to keep them off (the meeting's
//   fish) and each other (ROOM).
//
// A log is solid, not hollow, so nothing keeps water out of it (Environments
// rule 5 is for hulls). Units are meters, in the lake's coordinates: its
// middle at the origin, y = 0 the still water.

// Where the logs go (LakeOptions.logs).
export interface LogsPlace {
  keepOff?: (x: number, z: number) => boolean; // a point of the lake no part of a log may drift over
  count?: number; // COUNT when left out
  seed?: number;
}

export interface LogsOptions {
  theme: Theme;
  place: LogsPlace;
  bedAt(x: number, z: number): number; // the lake bed's height, or the land's
  near(x: number, z: number, gap: number): boolean; // whether a boulder or the dock is within `gap` m of a point
}

// A log on the water.
export interface FloatingLog {
  readonly log: Log;
  // At the middle of its trunk's axis: turned (rotation.y), then pitched
  // (rotation.z, > 0 lifts its +x end), then rolled (rotation.x, > 0 dips its
  // +z side). The log hangs from it by its axis.
  readonly body: THREE.Group;
  readonly home: { readonly x: number; readonly z: number; readonly yaw: number };
  readonly length: number;
  readonly height: number; // from its bottom to the top of its stubs
  readonly depth: number; // across, bow and stubs and all
}

interface Drift {
  eddy: { radius: number; speed: [number, number]; phase: [number, number] };
  swing: { angle: number; speed: number; phase: number };
  spin: number; // which way the wind turns it, 1 or -1
  shift: THREE.Vector2; // how far the wind has pushed it from its home
  toLog: THREE.Matrix4; // from the lake's coordinates to its body's
}

const COUNT = 4;
const SEED = 6; // all four find room, spread over the water the cameras look across
const LENGTH = [1.5, 3] as const; // m
const THICK = [0.25, 0.5] as const; // m, its height
const DEEP = 1.08; // its depth over its height, as the log's own default size has it: room for its bow and stubs
const DRAFT = 0.45; // share of its trunk's thickness under the water
// m between its underside and the bed at least, on still water: the waves'
// troughs (8 cm in a full wind), its ends dipping as it pitches, and more.
const UNDER = 0.25;
const EDDY = { radius: [0.08, 0.15] as const, period: [240, 420] as const }; // m either way, and s round: a slow wander in still air
const DRIFT = { reach: 0.5, push: 8, back: 40 }; // m downwind it goes in a lasting full wind; s to go about two thirds of the way there, and to come back
const SWING = { angle: [0.1, 0.22] as const, period: [180, 320] as const }; // radians it swings either way on its own, and s a swing takes
const TURN = 0.12; // radians the wind turns it, pushed all the way
// m its middle strays from its home at most: its eddy (up to √2 of its
// radius) and the wind's push. The wind pushes every log alike, so two logs
// close in on each other by their eddies only.
const ROAM = Math.SQRT2 * EDDY.radius[1] + DRIFT.reach;
const FEEL = 0.4; // share of its length from its middle, toward either end, at which it feels the water
const FOLLOW = 2.5; // how quickly it follows the water, per second (the boat's preview: 3)
const ROLL = 0.6;
const ROOM = { solid: 0.4, keepOff: 0.3, log: 0.4 }; // m kept from a boulder or the dock, from what it is kept off, and between two logs
// Finding a home: tries for each log; m between the points checked along
// it; steps either way across its turning; points round each one; and m for
// what lies between the points checked.
const SEARCH = { tries: 400, step: 0.2, turns: 4, rim: 12, slack: 0.15 };

export class Logs extends THREE.Group {
  readonly floating: FloatingLog[] = [];
  private readonly drifts: Drift[] = [];
  private time = 0;
  private settled = false;
  private readonly target = new THREE.Vector2();
  private readonly point = new THREE.Vector3();

  constructor({ theme, place, bedAt, near }: LogsOptions) {
    super();
    this.name = 'logs';
    const random = seededRandom(place.seed ?? SEED);
    const count = place.count ?? COUNT;
    const placed: { points: { x: number; z: number }[]; room: number }[] = [];
    for (let n = 0; n < count; n++) {
      const seed = Math.floor(random() * 1e6);
      const length = between(random, ...LENGTH);
      const height = between(random, ...THICK);
      const depth = height * DEEP;
      const drift: Drift = {
        eddy: {
          radius: between(random, ...EDDY.radius),
          speed: [(2 * Math.PI) / between(random, ...EDDY.period), (2 * Math.PI) / between(random, ...EDDY.period)],
          phase: [2 * Math.PI * random(), 2 * Math.PI * random()],
        },
        swing: { angle: between(random, ...SWING.angle), speed: (2 * Math.PI) / between(random, ...SWING.period), phase: 2 * Math.PI * random() },
        spin: random() < 0.5 ? -1 : 1,
        shift: new THREE.Vector2(),
        toLog: new THREE.Matrix4(),
      };
      const log = new Log({ theme, seed, width: length, height, depth, round: true });
      // Round each point checked: half its depth, what lies between the
      // points, and how far its middle strays: all of ROAM, and toward
      // another log only its eddy.
      const own = depth / 2 + SEARCH.slack;
      const reach = own + ROAM;
      const room = own + Math.SQRT2 * drift.eddy.radius;
      const deepest = -(2 * DRAFT * log.radius + UNDER); // the bed must lie at least this low
      let home: { x: number; z: number; yaw: number } | null = null;
      let points: { x: number; z: number }[] = [];
      for (let t = 0; t < SEARCH.tries && !home; t++) {
        const r = LAKE_RADIUS * Math.sqrt(random());
        const angle = 2 * Math.PI * random();
        const at = { x: r * Math.cos(angle), z: r * Math.sin(angle), yaw: 2 * Math.PI * random() };
        points = reachOf(at, length);
        // The bed is a bowl, so its shallowest under a circle is on its rim.
        // What it is kept off may be a box, whose corner could reach in
        // between points further apart than twice as many.
        const fits =
          points.every(
            (p) =>
              !near(p.x, p.z, reach + ROOM.solid) &&
              round(p, reach, SEARCH.rim, (x, z) => bedAt(x, z) <= deepest) &&
              round(p, reach + ROOM.keepOff, 2 * SEARCH.rim, (x, z) => !place.keepOff?.(x, z)),
          ) && placed.every((other) => apart(points, other.points, room + other.room + ROOM.log));
        if (fits) home = at;
      }
      if (!home) {
        log.bark.geometry.dispose();
        log.cuts.geometry.dispose();
        continue;
      }
      placed.push({ points, room });

      const body = new THREE.Group();
      body.rotation.order = 'YZX';
      body.position.set(home.x, log.radius * (1 - 2 * DRAFT), home.z);
      body.rotation.y = home.yaw;
      log.position.y = -log.axis;
      body.add(log);
      this.add(body);
      this.floating.push({ log, body, home, length, height, depth });
      this.drifts.push(drift);
    }
  }

  // Moves the logs on by delta seconds, on the water now (the lake's, after
  // its waves have moved) in the wind now.
  update(delta: number, water: { heightAt(x: number, z: number): number }, wind: Wind): void {
    this.time += delta;
    const t = this.time;
    const follow = this.settled ? 1 - Math.exp(-FOLLOW * delta) : 1; // the first time, straight onto the water
    this.settled = true;
    const blow = this.target.copy(wind.direction.value).multiplyScalar(DRIFT.reach * wind.strength.value);
    this.floating.forEach((f, i) => {
      const d = this.drifts[i];
      // Pushed downwind, coming back slower: the same for every log.
      const back = blow.lengthSq() < d.shift.lengthSq();
      d.shift.lerp(blow, 1 - Math.exp(-delta / (back ? DRIFT.back : DRIFT.push)));
      const e = d.eddy;
      const x = f.home.x + e.radius * Math.cos(e.speed[0] * t + e.phase[0]) + d.shift.x;
      const z = f.home.z + e.radius * Math.sin(e.speed[1] * t + e.phase[1]) + d.shift.y;
      const yaw = f.home.yaw + d.swing.angle * Math.sin(d.swing.speed * t + d.swing.phase) + TURN * d.spin * Math.min(1, d.shift.length() / DRIFT.reach);

      // The water under it: toward either end, and at its sides.
      const c = Math.cos(yaw);
      const s = Math.sin(yaw);
      const reach = FEEL * f.length;
      const side = f.log.radius;
      const ahead = water.heightAt(x + reach * c, z - reach * s); // its +x end
      const behind = water.heightAt(x - reach * c, z + reach * s);
      const left = water.heightAt(x + side * s, z + side * c); // its +z side
      const right = water.heightAt(x - side * s, z - side * c);
      const body = f.body;
      body.position.x = x;
      body.position.z = z;
      body.rotation.y = yaw;
      body.position.y += ((ahead + behind + left + right) / 4 + f.log.radius * (1 - 2 * DRAFT) - body.position.y) * follow;
      body.rotation.z += (Math.atan2(ahead - behind, 2 * reach) - body.rotation.z) * follow;
      body.rotation.x += (-ROLL * Math.atan2(left - right, 2 * side) - body.rotation.x) * follow;
      body.updateMatrix();
      d.toLog.copy(body.matrix).invert();
    });
  }

  // Whether a point of the lake is inside a log's box, or within `room` m of
  // it: a camera keeps out of them and can't see through them (the lake
  // meeting's Sight.ts).
  contains(x: number, y: number, z: number, room = 0): boolean {
    for (let i = 0; i < this.floating.length; i++) {
      const f = this.floating[i];
      const at = f.body.position;
      if (Math.hypot(x - at.x, z - at.z) > f.length / 2 + f.depth + room) continue;
      const p = this.point.set(x, y, z).applyMatrix4(this.drifts[i].toLog);
      const axis = f.log.axis;
      if (Math.abs(p.x) <= f.length / 2 + room && Math.abs(p.z) <= f.depth / 2 + room && p.y >= -axis - room && p.y <= f.height - axis + room) return true;
    }
    return false;
  }
}

// Points along a log at home, at every turn it may take from there (its
// swing and the wind's turn, either way), the room round it is checked from.
function reachOf(home: { x: number; z: number; yaw: number }, length: number): { x: number; z: number }[] {
  const most = SWING.angle[1] + TURN;
  const along = Math.ceil(length / SEARCH.step);
  const points: { x: number; z: number }[] = [];
  for (let k = -SEARCH.turns; k <= SEARCH.turns; k++) {
    const yaw = home.yaw + (most * k) / SEARCH.turns;
    const c = Math.cos(yaw);
    const s = Math.sin(yaw);
    for (let i = 0; i <= along; i++) {
      const a = length * (i / along - 0.5);
      points.push({ x: home.x + a * c, z: home.z - a * s });
    }
  }
  return points;
}

// Whether `ok` holds at a point and at `rim` points round it, `radius` m off.
function round(p: { x: number; z: number }, radius: number, rim: number, ok: (x: number, z: number) => boolean): boolean {
  if (!ok(p.x, p.z)) return false;
  for (let k = 0; k < rim; k++) {
    const a = (2 * Math.PI * k) / rim;
    if (!ok(p.x + radius * Math.cos(a), p.z + radius * Math.sin(a))) return false;
  }
  return true;
}

// Whether every point of one set is at least `gap` m from every point of the
// other.
function apart(a: { x: number; z: number }[], b: { x: number; z: number }[], gap: number): boolean {
  return a.every((p) => b.every((q) => Math.hypot(p.x - q.x, p.z - q.z) >= gap));
}
