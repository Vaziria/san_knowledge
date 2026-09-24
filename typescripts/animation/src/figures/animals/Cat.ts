import { mix, type AnimalOptions } from './parts';
import { Quadruped, type Build } from './Quadruped';

// A domestic cat, about 25 cm tall at the shoulder and 42 cm long without
// its 28 cm tail: a lithe body, a round head with a short muzzle and pointed
// ears, and a long, thin tail carried up in a curve. A ginger coat (the fur
// colour toward trim), paler underneath and on its muzzle and paws.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts); this file gives its build. Animation behaviour follows its
// spec, Cat.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps half a meter, and carries in its mouth a figure up to
// 14 cm across (a bigger one is scaled down).

const CAT: Build = {
  name: 'cat',
  torso: { length: 0.42, height: 0.15, width: 0.125, hips: 0.92, chest: 0.95, tuck: 0.72, hump: 0 },
  neck: { length: 0.07, rise: 0.75, radius: 0.034 },
  head: {
    length: 0.1,
    width: 0.085,
    height: 0.075,
    snout: 0.3,
    snoutWidth: 0.6,
    snoutHeight: 0.5,
    ears: { kind: 'pointed', height: 0.045, width: 0.04, out: 0.3, back: 0.1 },
    eye: 0.011,
    pitch: 0.1,
  },
  tail: { length: 0.28, radius: 0.013, bush: 0, droop: -0.2, curl: 1.3 },
  front: { upper: 0.11, lower: 0.11, radius: 0.021, foot: 'paw', bend: 1, x: 0.042, y: -0.03, z: 0.14 },
  hind: { upper: 0.12, lower: 0.13, radius: 0.027, foot: 'paw', bend: -1, x: 0.042, y: -0.012, z: -0.15 },
  colors: (p) => {
    const coat = mix(p.fur, p.trim, 0.45);
    return {
      coat,
      belly: mix(coat, p.light, 0.7),
      legs: mix(coat, p.light, 0.55),
      muzzle: mix(coat, p.light, 0.75),
      tail: coat.clone().multiplyScalar(0.8),
      horn: p.light,
    };
  },
  jump: 0.5,
  hold: 0.14,
};

export class Cat extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, CAT);
  }
}
