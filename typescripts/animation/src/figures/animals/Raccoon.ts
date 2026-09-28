import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { RACCOON_MODEL } from './raccoonModel';

// A raccoon, low poly like folded paper, in the shape the user modelled
// (raccoonModel.ts, "# Shape Reference" in Raccoon.md, from their animal
// park, 2026-09-26): 28 cm to the top of its shoulders and 87 cm from its
// nose to the tip of its tail, round and humped over its hips, walking on
// its whole soles, with a short snout, small ears and a bushy ringed tail.
// Every face is flat and of one colour, the model's colours through the
// theme: grey (felt's stone), lighter underneath and on its cheeks (light
// toward stone), a black mask, nose, feet and tail rings (felt's body), a
// white brow and muzzle, and glossy black eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Raccoon.md, gives its
// shape only; it has the other animals' behaviours: Jump(), Hold(figure),
// Walk(), Run(), Speech(text) and Stop(). It jumps 30 cm, and carries in its
// mouth a figure up to 12 cm across (a bigger one is scaled down).

const RACCOON: ModelBuild = {
  name: 'raccoon',
  model: RACCOON_MODEL,
  colors: (p) => {
    const grey = p.stone;
    const light = mix(p.light, p.stone, 0.45);
    const dark = p.body;
    return {
      coat: grey,
      belly: light,
      legs: dark,
      muzzle: p.light,
      tail: dark,
      horn: p.light,
      grey,
      light,
      dark,
      white: p.light,
      eye: p.dark,
    };
  },
  jump: 0.3,
  hold: 0.12,
};

export class Raccoon extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, RACCOON);
  }
}
