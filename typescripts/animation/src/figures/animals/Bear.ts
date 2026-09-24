import { mix, type AnimalOptions } from './parts';
import { Quadruped, type Build } from './Quadruped';

// A brown bear, about 1.3 m tall at the hump of its shoulders and 1.5 m long:
// a heavy, deep body with the hump over the shoulders, thick legs on broad
// paws, a big round head carried low with a short muzzle, small round ears
// and a stub of a tail. A dark brown coat (the fur colour, darkened), its
// muzzle paler.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts); this file gives its build. Animation behaviour follows its
// spec, Bear.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps 30 cm, and carries in its mouth a figure up to 45 cm
// across (a bigger one is scaled down).

const BEAR: Build = {
  name: 'bear',
  torso: { length: 1.5, height: 0.72, width: 0.66, hips: 0.9, chest: 1, tuck: 0.88, hump: 0.18 },
  neck: { length: 0.26, rise: 0.12, radius: 0.2 },
  head: {
    length: 0.42,
    width: 0.32,
    height: 0.28,
    snout: 0.38,
    snoutWidth: 0.5,
    snoutHeight: 0.55,
    ears: { kind: 'round', height: 0.09, width: 0.09, out: 0.25, back: 0 },
    eye: 0.017,
    pitch: 0.15,
  },
  tail: { length: 0.1, radius: 0.05, bush: 0, droop: 0.6, curl: 0 },
  front: { upper: 0.35, lower: 0.4, radius: 0.11, foot: 'paw', bend: 1, x: 0.2, y: -0.2, z: 0.5 },
  hind: { upper: 0.38, lower: 0.42, radius: 0.13, foot: 'paw', bend: -1, x: 0.2, y: -0.18, z: -0.5 },
  colors: (p) => {
    const coat = p.fur.clone().multiplyScalar(0.7);
    return {
      coat,
      belly: coat.clone().multiplyScalar(0.9),
      legs: coat.clone().multiplyScalar(0.78),
      muzzle: mix(p.fur, p.light, 0.3),
      tail: coat,
      horn: p.light,
    };
  },
  jump: 0.3,
  hold: 0.45,
};

export class Bear extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, BEAR);
  }
}
