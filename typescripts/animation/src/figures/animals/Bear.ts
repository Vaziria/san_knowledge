import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { BEAR_MODEL } from './bearModel';

// A brown bear, low poly like folded paper, in the shape the user modelled
// (bearModel.ts): 1.2 m to the top of its shoulder hump and 2.2 m from its
// nose to the tip of its tail, big and heavy, with thick legs and big flat
// paws, a round head with a short, broad muzzle and small round ears, and a
// tiny stub of a tail. Every face is flat and of one colour, the model's
// colours through the theme: brown fur (the fur colour toward dark), darker
// on its lower legs and low on its sides, a tan muzzle and ear insides (wood
// toward sand), and a dark nose and glossy dark eyes.
//
// It stands on all fours, as the model does: the user chose that on
// 2026-09-26, when it had stood up on its hind legs since their papercraft
// picture.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Animation behaviour follows its
// spec, Bear.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps 30 cm, and carries in its mouth a figure up to 45 cm
// across (a bigger one is scaled down).

const BEAR: ModelBuild = {
  name: 'bear',
  model: BEAR_MODEL,
  colors: (p) => {
    const brown = mix(p.fur, p.dark, 0.45);
    const tan = mix(p.wood, p.sand, 0.2);
    return { coat: brown, belly: brown, legs: brown, muzzle: tan, tail: brown, horn: p.light, brown, dark: mix(p.fur, p.dark, 0.8), tan, black: p.dark };
  },
  jump: 0.3,
  hold: 0.45,
};

export class Bear extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, BEAR);
  }
}
