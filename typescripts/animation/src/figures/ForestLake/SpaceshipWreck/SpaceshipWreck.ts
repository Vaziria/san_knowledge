import * as THREE from 'three';
import { seededRandom, snowOn, type Season } from '../parts';
import { Cockpit } from './Cockpit';
import { Debris } from './Debris';
import { EngineModule } from './EngineModule';
import { HULL_GROUND, HULL_HIGH, HullModule } from './HullModule';
import { TailModule } from './TailModule';
import { WingModule } from './WingModule';

export interface SpaceshipWreckOptions {
  season?: Season; // spring by default: snow on it in winter, leaves round it in autumn
  debris?: boolean; // what lies and grows round it; true by default
  seed?: number;
}

// Where the hull's axis is over the ground: each module sunk 0.3 m into it.
const AXIS = -HULL_GROUND;

// A spaceship wreck (SpaceshipWreck.md), as the user's reference sheets
// draw it: a ship crashed and broken in pieces in a clearing, grown over a
// little. From its nose (-x) to its tail (+x): the cockpit, broken off and
// turned aside, its nose dug into the ground; the hull's front module, torn
// open where the cockpit was; the middle module, its side and top torn wide
// open (+z) on its lit hold, its ramp let down; the rear module, with the
// right wing still on it, drooping to the ground, and the tail's fin on its
// top; and the engine, its fan toward the back. The left wing lies torn off
// on the ground in front, and all round lie plates, a torn piece of hull,
// cargo, drums, cables and rocks, grass, bushes and two dead trees.
//
// Meters; about 24 m long, 18 m across its wings and what lies round it,
// the fin's tip 8 m up. y up, x along it (its nose at -x), its torn-open
// side facing +z; the origin in the middle of its length, on the ground.
//
// Every part is low poly BufferGeometry made in code: plates over a dark
// frame, rust, the insides lit by their lamps (baked). The navigation light
// blinks, the engine's core glows brighter and dimmer and a lamp in the hold
// flickers; nothing else moves, and it has no behaviours.
export class SpaceshipWreck extends THREE.Group {
  static readonly LENGTH = 24;
  static readonly WIDTH = 18;
  static readonly HEIGHT = 8;

  readonly cockpit: Cockpit;
  readonly hullFront: HullModule;
  readonly hullMiddle: HullModule;
  readonly hullRear: HullModule;
  readonly engine: EngineModule;
  readonly tail: TailModule;
  readonly leftWing: WingModule; // torn off, on the ground in front (+z)
  readonly rightWing: WingModule; // still on, drooping behind (-z)
  readonly debris: THREE.Group;

  constructor(options: SpaceshipWreckOptions = {}) {
    super();
    const season = options.season ?? 'spring';
    const seed = options.seed ?? 3;
    const random = seededRandom(seed * 7919 + 11);
    const place = <T extends THREE.Object3D>(part: T, x: number, y: number, z: number, roll = 0, yaw = 0, pitch = 0): T => {
      part.position.set(x, y, z);
      // Rolled about its own length first, then pitched, then turned.
      part.rotation.set(roll, yaw, pitch, 'YZX');
      this.add(part);
      return part;
    };

    this.cockpit = place(new Cockpit({ season, seed: seed + 10 }), -6.95, AXIS - 0.12, 0.2, 0.1, 0.14, 0.09);
    this.hullFront = place(new HullModule({ kind: 'front', length: 4.5, season, seed: seed + 20 }), -6.2, AXIS, 0, 0, -0.03, 0.015);
    this.hullMiddle = place(new HullModule({ kind: 'middle', length: 5.0, season, seed: seed + 30 }), -1.62, AXIS, 0);
    this.hullRear = place(new HullModule({ kind: 'rear', length: 3.85, season, seed: seed + 40 }), 3.46, AXIS, 0);
    this.engine = place(new EngineModule({ season, seed: seed + 50 }), 7.42, 1.88, 0.05, 0, -0.05, -0.03);
    this.tail = place(new TailModule({ season, seed: seed + 60 }), 3.72, AXIS + HULL_HIGH - 0.12, 0, -0.07);
    this.rightWing = place(new WingModule({ side: -1, season, seed: seed + 70 }), 5.6, 1.95, -1.78, -0.27);
    this.leftWing = place(new WingModule({ side: 1, broken: true, season, seed: seed + 80 }), 4.9, 0.2, 2.95, -0.06, 0.5, 0.07);
    // Snow on it, each part by the way it lies here.
    if (season === 'winter') snowOn(this, random, 0.4, 0.75);
    this.debris = (options.debris ?? true) ? new Debris({ season, seed: seed + 90 }) : new THREE.Group();
    this.add(this.debris);
  }

  // The floor to stand on at a point of it (its own x and z): the hold's
  // floor and its ramp; NaN off them.
  floorAt(x: number, z: number): number {
    const middle = this.hullMiddle;
    return middle.floorAt(x - middle.position.x, z - middle.position.z) + middle.position.y;
  }

  // The navigation light blinks, the core glows, a lamp flickers.
  update(delta: number): void {
    this.hullMiddle.update(delta);
    this.engine.update(delta);
    this.tail.update(delta);
  }
}
