import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { DEER_MODEL } from './deerModel';

// A buck, low poly like folded paper, in the shape the user modelled
// (deerModel.ts): 95 cm to the top of its shoulders and 1.56 m from its nose
// to its tail, slender, with long thin legs on black hooves, a long neck
// rising at the front, a long narrow face, big ears sticking out sideways,
// antlers of a curved beam and three tines each, and a short tail. Every
// face is flat and of one colour, the model's colours through the theme: a
// tan-brown coat (the fur colour a little toward trim), a white belly,
// throat, chin and underside of the tail (light toward sand), ears pinkish
// inside (autumn toward light), antlers bone (sand), and a dark nose, dark
// hooves and glossy dark eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Animation behaviour follows its
// spec, Deer.md: Jump(), Hold(figure), Walk(), Run() and Speech(text), plus
// Stop(). It jumps 90 cm, and carries in its mouth a figure up to 30 cm
// across (a bigger one is scaled down).

const DEER: ModelBuild = {
  name: 'deer',
  model: DEER_MODEL,
  colors: (p) => {
    const fur = mix(p.fur, p.trim, 0.15);
    const white = mix(p.light, p.sand, 0.2);
    return {
      coat: fur,
      belly: white,
      legs: fur,
      muzzle: white,
      tail: white,
      horn: p.sand,
      fur,
      white,
      black: mix(p.body, p.dark, 0.6),
      antler: p.sand,
      inner: mix(p.autumn, p.light, 0.35),
    };
  },
  jump: 0.9,
  hold: 0.3,
};

export class Deer extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, DEER);
  }
}
