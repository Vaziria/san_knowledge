import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { WOLF_MODEL } from './wolfModel';

// A grey wolf, low poly like folded paper, in the shape the user modelled
// (wolfModel.ts): 78 cm tall at the shoulder and 2 m from its nose to the tip
// of its tail, with a deep chest, long legs, tall pointed ears, a long muzzle
// and a thick tail hanging in a curve. Every face is flat and of one colour:
// one grey coat all over (between the dark and light colours, warmed by the
// fur colour), as the model is one paper colour, a darker grey inside the
// ears and on the tail's tip (the model's mid grey), a dark nose and dark,
// glossy eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Animation behaviour follows its
// spec, Wolf.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps 60 cm, and carries in its mouth a figure up to 35 cm
// across (a bigger one is scaled down).

const WOLF: ModelBuild = {
  name: 'wolf',
  model: WOLF_MODEL,
  colors: (p) => {
    const coat = mix(mix(p.dark, p.light, 0.5), p.fur, 0.25);
    const mid = mix(coat, p.dark, 0.6); // the model's mid grey, as far from its coat toward its dark
    return { coat, belly: coat, legs: coat, muzzle: mid, tail: mid, horn: p.light, nose: p.dark };
  },
  jump: 0.6,
  hold: 0.35,
};

export class Wolf extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, WOLF);
  }
}
