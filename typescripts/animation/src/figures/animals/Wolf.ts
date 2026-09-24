import { mix, type AnimalOptions } from './parts';
import { Quadruped, type Build } from './Quadruped';

// A grey wolf, about 78 cm tall at the shoulder and a meter long without its
// 45 cm tail: long-legged and deep-chested, with a thick ruff round its neck,
// a long muzzle, pointed ears and a bushy tail hanging down. A grey coat
// (between the dark and light colours, warmed by the fur colour), paler
// underneath, on its legs and muzzle, the tail tipped dark.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts); this file gives its build. Animation behaviour follows its
// spec, Wolf.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps 60 cm, and carries in its mouth a figure up to 35 cm
// across (a bigger one is scaled down).

const WOLF: Build = {
  name: 'wolf',
  torso: { length: 1, height: 0.36, width: 0.3, hips: 0.82, chest: 1, tuck: 0.7, hump: 0.04 },
  neck: { length: 0.2, rise: 0.55, radius: 0.1 },
  head: {
    length: 0.3,
    width: 0.17,
    height: 0.15,
    snout: 0.45,
    snoutWidth: 0.5,
    snoutHeight: 0.5,
    ears: { kind: 'pointed', height: 0.09, width: 0.075, out: 0.15, back: 0.1 },
    eye: 0.016,
    pitch: 0.2,
  },
  tail: { length: 0.45, radius: 0.058, bush: 0.8, droop: 0.9, curl: 0.3 },
  front: { upper: 0.3, lower: 0.32, radius: 0.05, foot: 'paw', bend: 1, x: 0.1, y: -0.08, z: 0.33 },
  hind: { upper: 0.34, lower: 0.36, radius: 0.064, foot: 'paw', bend: -1, x: 0.1, y: -0.04, z: -0.36 },
  colors: (p) => {
    const coat = mix(mix(p.dark, p.light, 0.5), p.fur, 0.25);
    return {
      coat,
      belly: mix(coat, p.light, 0.6),
      legs: mix(coat, p.light, 0.35),
      muzzle: mix(coat, p.light, 0.55),
      tail: p.dark,
      horn: p.light,
    };
  },
  jump: 0.6,
  hold: 0.35,
};

export class Wolf extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, WOLF);
  }
}
