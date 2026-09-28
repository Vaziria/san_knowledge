import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { PIG_MODEL } from './pigModel';

// A pig, low poly like folded paper, in the shape the user modelled
// (pigModel.ts, "# Shape Reference" in Pig.md, from their animal park,
// 2026-09-26): 70 cm to the top of its shoulders and 1.6 m from its snout to
// its tail, a round barrel on short legs with flat hooves, a short, wide
// snout with two nostrils, big floppy ears flopping forward, and a little
// curly tail. Every face is flat and of one colour, the model's colours
// through the theme, which has no pink: pink from light, trim and flower, a
// darker pink underneath and inside the ears, a pinker snout, dark nostrils,
// grey-brown hooves, and glossy dark eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Pig.md, gives its shape
// only; it has the other animals' behaviours: Jump(), Hold(figure), Walk(),
// Run(), Speech(text) and Stop(). It jumps 30 cm, and carries in its mouth a
// figure up to 25 cm across (a bigger one is scaled down).

const PIG: ModelBuild = {
  name: 'pig',
  model: PIG_MODEL,
  colors: (p) => {
    const pink = mix(mix(p.light, p.trim, 0.5), p.flower, 0.1);
    const darkPink = mix(mix(p.flower, p.light, 0.15), p.trim, 0.5);
    return {
      coat: pink,
      belly: darkPink,
      legs: pink,
      muzzle: pink,
      tail: pink,
      horn: p.light,
      pink,
      darkPink,
      snout: mix(mix(p.flower, p.trim, 0.65), p.light, 0.2),
      nostril: mix(mix(p.flower, p.trim, 0.5), p.dark, 0.7),
      hoof: mix(mix(p.flower, p.wood, 0.7), p.body, 0.6),
      eye: mix(p.body, p.dark, 0.45),
    };
  },
  jump: 0.3,
  hold: 0.25,
};

export class Pig extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, PIG);
  }
}
