import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { SQUIRREL_MODEL } from './squirrelModel';

// A red squirrel, low poly like folded paper, in the shape the user modelled
// (squirrelModel.ts, "# Shape Reference" in Squirrel.md, from their animal
// park, 2026-09-26): 22 cm from its rump to its nose and 10 cm to the top of
// its shoulders, with big haunches and long feet, small hands, tall tufted
// ears, and a huge bushy tail curling up over its back. Every face is flat
// and of one colour, the model's colours through the theme: red-brown (trim
// toward dark), darker tufts in its tail and behind its ears, a white belly,
// brow, cheeks and chin (glow toward light), a dark nose, and glossy black
// eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Squirrel.md, gives its
// shape only; it has the other animals' behaviours: Jump(), Hold(figure),
// Walk(), Run(), Speech(text) and Stop(). It jumps 30 cm, and carries in its
// mouth a figure up to 6 cm across, as the bird does (a bigger one is scaled
// down).

const SQUIRREL: ModelBuild = {
  name: 'squirrel',
  model: SQUIRREL_MODEL,
  colors: (p) => {
    const red = mix(mix(p.dark, p.trim, 0.5), p.flower, 0.05);
    const white = mix(p.glow, p.light, 0.85);
    return {
      coat: red,
      belly: white,
      legs: red,
      muzzle: white,
      tail: red,
      horn: p.light,
      red,
      dark: mix(mix(p.dark, p.trim, 0.4), p.body, 0.45),
      white,
      nose: mix(p.body, p.fur, 0.2),
      eye: p.dark,
    };
  },
  jump: 0.3,
  hold: 0.06,
};

export class Squirrel extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, SQUIRREL);
  }
}
