import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { CAT_MODEL } from './catModel';

// A grey tabby cat, low poly like folded paper, in the shape the user
// modelled (catModel.ts): 25 cm to the top of its shoulders and 52 cm from
// its nose to the back of its tail, slim, with a round face, a short muzzle,
// big forward-facing eyes, tall pointed ears, three whiskers a side and a
// long tail rising and curling forward. Every face is flat and of one
// colour, the model's colours through the theme: grey fur (between the dark
// and light colours) with darker tabby stripes on its back, sides, legs,
// forehead and tail, a white chest, muzzle and paws, a pink nose and ear
// insides (light, flower and trim), glossy green eyes (the grass colour
// toward glow) and light whiskers.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Animation behaviour follows its
// spec, Cat.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps half a meter, and carries in its mouth a figure up to
// 14 cm across (a bigger one is scaled down).

const CAT: ModelBuild = {
  name: 'cat',
  model: CAT_MODEL,
  colors: (p) => {
    const grey = mix(p.dark, p.light, 0.39);
    return {
      coat: grey,
      belly: p.light,
      legs: grey,
      muzzle: p.light,
      tail: grey,
      horn: p.light,
      grey,
      stripe: mix(p.dark, p.light, 0.08),
      white: p.light,
      pink: mix(mix(p.light, p.flower, 0.5), p.trim, 0.5),
      eye: mix(p.grass, p.glow, 0.25),
      whisker: p.light,
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
