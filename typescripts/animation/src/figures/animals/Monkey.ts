import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { MONKEY_MODEL } from './monkeyModel';

// A monkey walking on all fours, like a macaque, low poly like folded paper,
// in the shape the user modelled (monkeyModel.ts, "# Shape Reference" in
// Monkey.md, from their animal park, 2026-09-26): 50 cm from its rump to its
// face and 28 cm to the top of its shoulders, a deep chest and a flat back,
// a round head with a flat bare face, a short muzzle, forward-looking eyes
// and round ears, arms whose elbows bend back, flat hands and feet, and a
// long tail curling up over its end. Every face is flat and of one colour,
// the model's colours through the theme: brown (fur toward dark), lighter
// underneath (fur toward sand), a darker brow, a bare face, hands and feet
// (light toward trim), a nose (wood toward flower) and mouth, and brown eyes
// (autumn toward dark) round a glossy dark middle.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Monkey.md, gives its
// shape only; it has the other animals' behaviours: Jump(), Hold(figure),
// Walk(), Run(), Speech(text) and Stop(). It jumps 50 cm, and carries in its
// mouth a figure up to 12 cm across (a bigger one is scaled down).

const MONKEY: ModelBuild = {
  name: 'monkey',
  model: MONKEY_MODEL,
  colors: (p) => {
    const fur = mix(p.dark, p.fur, 0.5);
    const light = mix(p.fur, p.sand, 0.2);
    const face = mix(p.light, p.trim, 0.6);
    return {
      coat: fur,
      belly: light,
      legs: fur,
      muzzle: face,
      tail: fur,
      horn: p.light,
      fur,
      dark: mix(p.autumn, p.dark, 0.85),
      light,
      face,
      nose: mix(p.wood, p.flower, 0.1),
      mouth: mix(p.autumn, p.body, 0.8),
      eye: mix(p.autumn, p.dark, 0.5), // the ring round each eye's middle
      pupil: p.dark,
    };
  },
  jump: 0.5,
  hold: 0.12,
};

export class Monkey extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, MONKEY);
  }
}
