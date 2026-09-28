import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { ANTELOPE_MODEL } from './antelopeModel';

// An antelope, a gazelle, low poly like folded paper, in the shape the user
// modelled (antelopeModel.ts, "# Shape Reference" in Antelope.md, from their
// animal park, 2026-09-26): 80 cm to the top of its shoulders and 1.5 m to
// the tips of its long ringed horns, slim, with a long neck, long thin legs
// with black hooves, and a short black tail. Every face is flat and of one
// colour, the model's colours through the theme: tan (trim toward stone), a
// white belly, throat, brow and chin (glow toward light), a dark stripe
// along each side and down its face, black horns ringed in dark grey, a
// black nose and hooves, and glossy black eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Antelope.md, gives its
// shape only; it has the other animals' behaviours: Jump(), Hold(figure),
// Walk(), Run(), Speech(text) and Stop(). It jumps 1 m, as a gazelle bounds,
// and carries in its mouth a figure up to 25 cm across (a bigger one is
// scaled down).

const ANTELOPE: ModelBuild = {
  name: 'antelope',
  model: ANTELOPE_MODEL,
  colors: (p) => {
    const tan = mix(p.stone, p.trim, 0.65);
    const white = mix(p.glow, p.light, 0.9);
    const black = mix(p.body, p.dark, 0.75);
    return {
      coat: tan,
      belly: white,
      legs: tan,
      muzzle: white,
      tail: black,
      horn: mix(p.body, p.fur, 0.05),
      tan,
      white,
      dark: mix(p.dark, p.wood, 0.1),
      ring: mix(p.dark, p.sand, 0.15),
      black,
      eye: black,
    };
  },
  jump: 1,
  hold: 0.25,
};

export class Antelope extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, ANTELOPE);
  }
}
