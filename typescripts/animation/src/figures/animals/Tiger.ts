import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { TIGER_MODEL } from './tigerModel';

// A tiger, low poly like folded paper, in the shape the user modelled
// (tigerModel.ts, "# Shape Reference" in Tiger.md, from their animal park,
// 2026-09-26): 1 m to the top of its shoulders, big and muscular, with thick
// legs and big paws, a broad head with a short, wide muzzle and small ears,
// and a long tail hanging down. Every face is flat and of one colour, the
// model's colours through the theme: orange (felt's trim) with narrow black
// bands over its body, legs and head, a white belly, brow, muzzle and chin
// (glow toward light), a pink nose (flower toward trim), black and orange
// rings down its tail to a black end, and glossy yellow eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Tiger.md, gives its shape
// only; it has the other animals' behaviours: Jump(), Hold(figure), Walk(),
// Run(), Speech(text) and Stop(). It jumps 80 cm, as the lion does, and
// carries in its mouth a figure up to 40 cm across (a bigger one is scaled
// down).

const TIGER: ModelBuild = {
  name: 'tiger',
  model: TIGER_MODEL,
  colors: (p) => {
    const orange = p.trim;
    const white = mix(p.glow, p.light, 0.95);
    const black = mix(p.body, p.dark, 0.6);
    return {
      coat: orange,
      belly: white,
      legs: orange,
      muzzle: white,
      tail: black,
      horn: p.light,
      orange,
      white,
      black,
      nose: mix(p.flower, p.trim, 0.6),
      eye: mix(mix(p.glow, p.trim, 0.35), p.dark, 0.2),
    };
  },
  jump: 0.8,
  hold: 0.4,
};

export class Tiger extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, TIGER);
  }
}
