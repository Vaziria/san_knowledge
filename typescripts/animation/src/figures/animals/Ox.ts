import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { OX_MODEL } from './oxModel';

// An ox, low poly like folded paper, in the shape the user modelled
// (oxModel.ts, "# Shape Reference" in Ox.md, from their animal park,
// 2026-09-26): 1.5 m to the top of its shoulders and 2.8 m from its nose to
// the tip of its tail, big and deep-bodied, with long horns going out to the
// sides and curving up, ears standing out beside them, a short, wide snout,
// thick legs with hooves, and a thin tail with a tuft. Every face is flat
// and of one colour, the model's colours through the theme: brown (autumn
// toward dark), a pale muzzle (light toward wood), a pinkish nose, pale horns
// with dark tips, ears pale inside, dark hooves and tuft, and glossy dark
// eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Ox.md, gives its shape
// only; it has the other animals' behaviours: Jump(), Hold(figure), Walk(),
// Run(), Speech(text) and Stop(). It jumps 40 cm, and carries in its mouth a
// figure up to 40 cm across (a bigger one is scaled down).

const OX: ModelBuild = {
  name: 'ox',
  model: OX_MODEL,
  colors: (p) => {
    const brown = mix(p.autumn, p.dark, 0.7);
    const dark = mix(p.dark, p.fur, 0.1);
    const muzzle = mix(p.light, p.wood, 0.65);
    const horn = mix(p.light, p.sand, 0.7);
    return {
      coat: brown,
      belly: brown,
      legs: brown,
      muzzle,
      tail: dark,
      horn,
      brown,
      dark,
      nose: mix(mix(p.dark, p.flower, 0.4), p.wood, 0.4),
      hornTip: mix(p.dark, p.sand, 0.1),
      earIn: mix(p.fur, p.sand, 0.35),
      eye: dark,
    };
  },
  jump: 0.4,
  hold: 0.4,
};

export class Ox extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, OX);
  }
}
