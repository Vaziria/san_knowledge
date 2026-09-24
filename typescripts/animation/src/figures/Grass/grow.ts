import { defaultTheme } from '../../theme';
import { Leaves } from './Leaves';
import { coarsen, createMaterials, fit, palette, seededRandom, sizeOf, type GrassOptions, type Palette, type Plan, type Size } from './parts';
import { Stems } from './Stems';

// What a kind of grass says about itself: its default seed and size, and how
// it plans a plant, in the theme's colours (see palette() in parts.ts), with
// more or fewer leaves and stems by `crowd` (see sizeOf()).
export interface Kind {
  seed: number;
  size: Size;
  plan(colors: Palette, random: () => number, crowd: number): Plan;
}

// Grows a plant of a kind: plans it from its seed, sizes the plan to the
// height and width asked for (fit() in parts.ts), coarsens it when it is to
// be seen from afar (a detail under 1), and builds its parts.
export function grow(options: GrassOptions, kind: Kind): { leaves: Leaves; stems: Stems } {
  const theme = options.theme ?? defaultTheme;
  const random = seededRandom(options.seed ?? kind.seed);
  const size = sizeOf(options, kind.size);
  const plan = kind.plan(palette(theme), random, size.crowd);
  fit(plan, size);
  if ((options.detail ?? 1) < 1) coarsen(plan);
  const m = createMaterials(theme);
  return { leaves: new Leaves(m, plan.leaves, random), stems: new Stems(m, plan.stems, plan.wood, random) };
}
