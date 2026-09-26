import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { FOX_MODEL } from './foxModel';

// A red fox, low poly like folded paper, in the shape the user modelled
// (foxModel.ts): 38 cm to the top of its shoulders and 1.2 m from its nose
// to the tip of its tail, slender, with a long, thin snout, big pointed ears,
// slanted eyes, a white cheek ruff and a big bushy tail. Every face is flat
// and of one colour, the model's colours through the theme: an orange coat
// (the fur colour far toward trim), a white chest, belly, lower face, ear
// insides and tail tip (light), black "socks", ear backs and nose (body), and
// glossy dark eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Animation behaviour follows its
// spec, Fox.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps 45 cm, and carries in its mouth a figure up to 20 cm
// across (a bigger one is scaled down).

const FOX: ModelBuild = {
  name: 'fox',
  model: FOX_MODEL,
  colors: (p) => {
    const orange = mix(p.fur, p.trim, 0.7);
    return { coat: orange, belly: p.light, legs: p.body, muzzle: p.light, tail: p.light, horn: p.light, orange, white: p.light, black: p.body, dark: p.dark };
  },
  jump: 0.45,
  hold: 0.2,
};

export class Fox extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, FOX);
  }
}
