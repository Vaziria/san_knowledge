import { mix, type AnimalOptions } from './parts';
import { Quadruped, type ModelBuild } from './Quadruped';
import { COYOTE_MODEL } from './coyoteModel';

// A coyote, low poly like folded paper, in the shape the user modelled
// (coyoteModel.ts, "# Shape Reference" in Coyote.md, from their animal park,
// 2026-09-26): 60 cm to the top of its shoulders and 1.36 m from its nose to
// the tip of its tail, lean, with a narrow head, a long thin snout, tall
// ears, thin legs and a bushy tail hanging low. Every face is flat and of
// one colour, the model's colours through the theme: tawny (fur toward
// sand), grey along its back and on top of its head (fur toward stone), a
// cream belly, muzzle and ear insides (light toward sand), a black nose and
// tail tip, and glossy yellow eyes.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts and movements are the four-legged animals'
// (Quadruped.ts), its parts built from the model; this file gives its
// colours and what it jumps and carries. Its spec, Coyote.md, gives its
// shape only; it has the other animals' behaviours: Jump(), Hold(figure),
// Walk(), Run(), Speech(text) and Stop(). It jumps 60 cm, as the wolf does,
// and carries in its mouth a figure up to 25 cm across (a bigger one is
// scaled down).

const COYOTE: ModelBuild = {
  name: 'coyote',
  model: COYOTE_MODEL,
  colors: (p) => {
    const fur = mix(p.fur, p.sand, 0.3);
    const cream = mix(p.light, p.sand, 0.45);
    const black = mix(p.body, p.dark, 0.55);
    return {
      coat: fur,
      belly: cream,
      legs: fur,
      muzzle: cream,
      tail: black,
      horn: p.light,
      fur,
      grey: mix(p.fur, p.stone, 0.75),
      cream,
      black,
      eye: mix(mix(p.body, p.trim, 0.6), p.glow, 0.4),
    };
  },
  jump: 0.6,
  hold: 0.25,
};

export class Coyote extends Quadruped {
  constructor(options: AnimalOptions = {}) {
    super(options, COYOTE);
  }
}
