import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { LION_MODEL } from './lionModel';

// A male lion, low poly like folded paper, in the shape the user modelled
// (lionModel.ts, "# Shape Reference" in Lion.md, 2026-09-26): 1.2 m to the
// top of its shoulders and 2.4 m from its nose to the tip of its tail, big
// and muscular, with thick legs and big paws, a broad head like a big cat's
// with a long, wide muzzle and small rounded ears, a shaggy mane round its
// face, three whiskers a side, and a long tail ending in a tuft. Every face
// is flat and of one colour, the model's colours through the theme: golden
// tawny fur (glow and light toward autumn), a cream belly, muzzle and ear
// insides (glow toward light), a brown nose and dark ear backs (fur toward
// dark), glossy amber eyes (trim toward glow), a mane in three browns and a
// darker tuft (autumn toward dark), and light whiskers.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Lion.md, gives its shape
// only; it has the other animals' behaviours: Jump(), Hold(figure), Walk(),
// Run(), Speech(text) and Stop(). It jumps 80 cm, and carries in its mouth
// a figure up to 40 cm across (a bigger one is scaled down).

const LION: ModelBuild = {
  name: 'lion',
  model: LION_MODEL,
  colors: (p) => {
    const tawny = mix(mix(p.glow, p.light, 0.2), p.autumn, 0.7);
    const cream = mix(p.glow, p.light, 0.45);
    return {
      coat: tawny,
      belly: cream,
      legs: tawny,
      muzzle: cream,
      tail: tawny,
      horn: p.light,
      tawny,
      cream,
      nose: mix(p.fur, p.dark, 0.7),
      eye: mix(p.trim, p.glow, 0.2),
      earBack: mix(p.fur, p.dark, 0.6),
      maneDark: mix(p.autumn, p.dark, 0.8),
      mane: mix(p.autumn, p.dark, 0.55),
      maneLight: mix(p.autumn, p.dark, 0.2),
      tuft: mix(p.autumn, p.dark, 0.85),
      whisker: p.light,
    };
  },
  jump: 0.8,
  hold: 0.4,
};

export class Lion extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, LION);
  }
}
