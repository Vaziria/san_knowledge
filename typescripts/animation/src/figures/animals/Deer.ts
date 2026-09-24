import { mix, type AnimalOptions } from './parts';
import { Quadruped, type Build } from './Quadruped';

// A white-tailed deer, a buck, about 95 cm tall at the shoulder and 1.05 m
// long: slender legs on small dark hooves, a long neck carrying the head
// high, big ears set out to the sides, antlers sweeping up and forward with
// tines rising from them, and a short tail, white beneath. A tan coat (the
// fur colour toward trim and light), light underneath and on its throat and
// muzzle; antlers are bone, between the wood and light colours.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts); this file gives its build. Animation behaviour follows its
// spec, Deer.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps 90 cm, and carries in its mouth a figure up to 30 cm
// across (a bigger one is scaled down).

const DEER: Build = {
  name: 'deer',
  torso: { length: 1.05, height: 0.42, width: 0.3, hips: 0.9, chest: 1, tuck: 0.78, hump: 0.03 },
  neck: { length: 0.42, rise: 0.95, radius: 0.08 },
  head: {
    length: 0.3,
    width: 0.13,
    height: 0.13,
    snout: 0.45,
    snoutWidth: 0.6,
    snoutHeight: 0.6,
    ears: { kind: 'broad', height: 0.15, width: 0.08, out: 1.05, back: 0.3 },
    eye: 0.018,
    pitch: 0.55,
    antlers: { height: 0.38, spread: 0.2, radius: 0.017 },
  },
  tail: { length: 0.14, radius: 0.04, bush: 0.3, droop: 0.5, curl: 0.2 },
  front: { upper: 0.45, lower: 0.5, radius: 0.045, foot: 'hoof', bend: 1, x: 0.1, y: -0.1, z: 0.36 },
  hind: { upper: 0.5, lower: 0.55, radius: 0.058, foot: 'hoof', bend: -1, x: 0.1, y: -0.05, z: -0.38 },
  colors: (p) => {
    const coat = mix(mix(p.fur, p.trim, 0.25), p.light, 0.12);
    return { coat, belly: p.light, legs: coat, muzzle: p.light, tail: p.light, horn: mix(p.wood, p.light, 0.45) };
  },
  jump: 0.9,
  hold: 0.3,
};

export class Deer extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, DEER);
  }
}
