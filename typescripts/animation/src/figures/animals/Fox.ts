import { mix, type AnimalOptions } from './parts';
import { Quadruped, type Build } from './Quadruped';

// A red fox, about 38 cm tall at the shoulder and 55 cm long without its
// 42 cm tail: slender, with a long, narrow muzzle, big pointed ears and a
// thick brush of a tail held low. A red coat (the fur colour far toward
// trim), with a light chest, belly, muzzle and tail tip, and dark legs, its
// "stockings".
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts); this file gives its build. Animation behaviour follows its
// spec, Fox.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps 45 cm, and carries in its mouth a figure up to 20 cm
// across (a bigger one is scaled down).

const FOX: Build = {
  name: 'fox',
  torso: { length: 0.55, height: 0.19, width: 0.15, hips: 0.9, chest: 1, tuck: 0.72, hump: 0 },
  neck: { length: 0.1, rise: 0.75, radius: 0.045 },
  head: {
    length: 0.17,
    width: 0.1,
    height: 0.085,
    snout: 0.48,
    snoutWidth: 0.45,
    snoutHeight: 0.45,
    ears: { kind: 'pointed', height: 0.075, width: 0.056, out: 0.22, back: 0.05 },
    eye: 0.012,
    pitch: 0.1,
  },
  tail: { length: 0.42, radius: 0.045, bush: 1, droop: 0.35, curl: 0.25 },
  front: { upper: 0.17, lower: 0.17, radius: 0.027, foot: 'paw', bend: 1, x: 0.05, y: -0.04, z: 0.19 },
  hind: { upper: 0.19, lower: 0.2, radius: 0.034, foot: 'paw', bend: -1, x: 0.052, y: -0.02, z: -0.2 },
  colors: (p) => ({
    coat: mix(p.fur, p.trim, 0.8),
    belly: p.light,
    legs: p.dark,
    muzzle: p.light,
    tail: p.light,
    horn: p.light,
  }),
  jump: 0.45,
  hold: 0.2,
};

export class Fox extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, FOX);
  }
}
