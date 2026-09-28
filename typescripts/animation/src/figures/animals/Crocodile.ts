import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { CROCODILE_MODEL } from './crocodileModel';

// A crocodile, low poly like folded paper, in the shape the user modelled
// (crocodileModel.ts, "# Shape Reference" in Crocodile.md, from their animal
// park, 2026-09-26): 3.5 m from its nose to the tip of its tail, long and
// low, on short legs turned out from its sides, with a long snout lined with
// teeth, its eyes on bumps on top of its head, scutes along its back and a
// long tail with a row of spikes. Every face is flat and of one colour, the
// model's colours through the theme: dark green (grass toward dark), darker
// bands and feet, a pale yellow belly and jaw (glow toward water), light
// teeth, and yellow eyes (glow toward trim) round a glossy dark middle.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Crocodile.md, gives its
// shape only; it has the other animals' behaviours: Jump(), Hold(figure),
// Walk(), Run(), Speech(text) and Stop(). It jumps 30 cm, and carries in its
// jaws a figure up to 40 cm across (a bigger one is scaled down).

const CROCODILE: ModelBuild = {
  name: 'crocodile',
  model: CROCODILE_MODEL,
  colors: (p) => {
    const green = mix(p.dark, p.grass, 0.45);
    const belly = mix(p.glow, p.water, 0.5);
    return {
      coat: green,
      belly,
      legs: green,
      muzzle: belly,
      tail: green,
      horn: p.light,
      green,
      dark: mix(p.dark, p.grass, 0.15),
      eye: mix(p.glow, p.trim, 0.35), // the ring round each eye's middle
      pupil: p.dark,
      tooth: p.light,
    };
  },
  jump: 0.3,
  hold: 0.4,
  pitch: 0.15, // long and low: its tail and snout are 15 and 17 cm off the ground
};

export class Crocodile extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, CROCODILE);
  }
}
