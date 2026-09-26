# Grasses state

Updated 2026-09-25 by task 17 (the first state record), from the code as it is today.

In progress: listing each plant's `leaves` and `stems` as parts in the Figures tree, to preview one on its own (`src/parts.ts`, task 16), being built by another session.

## What works

- **Seven kinds**, each a class next to its spec in `src/figures/Grass/`: [CockFootGrass](../../src/figures/Grass/CockFootGrass.ts), [BermudaGrass](../../src/figures/Grass/BermudaGrass.ts), [TimothyGrass](../../src/figures/Grass/TimothyGrass.ts), [MeadowFoxtailGrass](../../src/figures/Grass/MeadowFoxtailGrass.ts), [RedFescueGrass](../../src/figures/Grass/RedFescueGrass.ts), [ChivesGrass](../../src/figures/Grass/ChivesGrass.ts) and [RosemaryGrass](../../src/figures/Grass/RosemaryGrass.ts). Chives and rosemary are herbs, named as the user listed them. The rules are in [rules.md, Grasses](../../../../docs/3d_modelling/rules.md#grasses).
- **Specs:** [CockFootGrass.md](../../src/figures/Grass/CockFootGrass.md), [BermudaGrass.md](../../src/figures/Grass/BermudaGrass.md), [TimothyGrass.md](../../src/figures/Grass/TimothyGrass.md), [MeadowFoxtailGrass.md](../../src/figures/Grass/MeadowFoxtailGrass.md), [RedFescueGrass.md](../../src/figures/Grass/RedFescueGrass.md), [ChivesGrass.md](../../src/figures/Grass/ChivesGrass.md) and [RosemaryGrass.md](../../src/figures/Grass/RosemaryGrass.md). Each is a draft written from the code and says so at its top. The sizes and shapes they give match the code.
- **Sized in meters:** `new TimothyGrass({ theme, seed, height, width })`, with each kind's default in `TimothyGrass.SIZE`, from Bermuda's 0.2 m mat to timothy's 0.95 m. With only a height, a plant keeps its shape.
  - `fit()` in [parts.ts](../../src/figures/Grass/parts.ts) stretches heights and spreads to exactly the size asked for. Widths and radii grow only by the square root of the mean stretch.
  - A wider plant grows more leaves and stems (`crowd` from `sizeOf()`, clamped to 0.4–4 times).
- **Shared parts:**
  - [Leaves.ts](../../src/figures/Grass/Leaves.ts): every leaf, one mesh.
  - [Stems.ts](../../src/figures/Grass/Stems.ts): stems with their seed heads or flowers, one mesh, plus a bark mesh for rosemary's woody stems.
  - [habit.ts](../../src/figures/Grass/habit.ts): tufts, sprouts and arching lines.
  - [grow.ts](../../src/figures/Grass/grow.ts): plan the plant, fit it, coarsen it, build it.
  - [parts.ts](../../src/figures/Grass/parts.ts): the options, materials, palette, and the blade, tube and blob geometry.
  - The blobs (`clumpGeometry`), rosemary's woody stems (`limbGeometry`) and the seeded random generator come from [Tree/parts.ts](../../src/figures/Tree/parts.ts) (see [trees.md](trees.md)).
  - Each plant exposes `readonly leaves` and `stems`, with its origin on the ground at the middle of its foot. So a plant is two draw calls.
- **Shapes:** leaves are ribbons folded into a V along an arching midrib, drawn front and back with normals leaning up. Stems and seed heads are tubes roughened by `bumps`. Spikelet clusters and florets are the trees' lumpy clumps, and a blob under 2 cm uses the 80-face sphere. Thin parts are drawn thicker than real ("over twice a real stem's" in the code), so the Kuwahara filter keeps them.
- **Colours from the theme:** one white material in the theme's finish, times vertex colours from `palette()`. Leaves are `grass`, seed heads ripen toward straw (40% toward `wood`), and chives' flowers are `flower`. Rosemary's stems are `bark(theme)`.
- **Detail for plants seen from afar:** `detail` in `GrassOptions` (1 by default) is the share of leaves and stems grown. Under 1, `coarsen()` also thins the blades and the tubes. The lake's meadow uses it. See [environments.md](environments.md) and [rules.md, Environments](../../../../docs/3d_modelling/rules.md#environments) rule 16 for how the lake places and sways them.
- **Seeded:** the same seed always grows the same plant, and each kind has its own default seed.
- **Preview:** `cock-foot-grass` … `rosemary-grass` in [previews.ts](../../src/previews.ts), through `plant()`, which looks at the whole plant from a little above. They start on the floor, not the lawn.
- **No behaviour:** in the preview a plant stands still (`update() {}`). Only the lake's wind bends them.

## How it was last checked

- 2026-09-25 (task 17, while writing this file): a numeric run in Node ([rules.md, Checking the result](../../../../docs/3d_modelling/rules.md#checking-the-result) rule 8).
  - Every kind builds as two meshes.
  - The top and the width come within about 1 mm of the size asked for, at the default size, at 1.1 × 0.6 m and at twice the height. The largest miss is 1.1 mm, chives at 1.1 m. The plants' feet reach about 1 cm below the ground.
  - Triangles run from 10.6 k (foxtail) to 81 k (rosemary). At `detail: 0.2`, timothy drops from 13.6 k to 1.1 k and Bermuda from 61.6 k to 6.5 k, as rules.md Grasses rule 8 says.
  - Warm builds took 7–67 ms.
- 2026-09-25, task 17: `npm run typecheck` is clean at 23:10, with other sessions' unfinished work in the tree (earlier that evening it failed for a few minutes on the dock's half-built lines, task 09).
- 2026-09-24 (278ad67): no dated check is recorded.
  - rules.md Grasses rule 1 records the numeric size check. It found that `fit()` scaled a shared point twice: a fescue asked to be 1.1 m tall came out 2.5 m. That was fixed.
  - Rule 4 records looking at the plants under the Kuwahara filter. Stems at real thickness were wiped out, and rosemary first looked like brown sticks.
  - No screenshot is recorded.

## Known issues and left for the user

- No task in `tasks/done/` changed the grass figures, so no Done section leaves anything for the user.
- The specs are Claude's drafts. The user hasn't corrected or confirmed them.
- rules.md Grasses rule 4 says stems are 5–7 mm across. Chives' flower stalks are 4.4 mm (radius 0.0022) and Bermuda's upright shoots 3.6 mm.

## Open questions

- Should the grasses move? No draft names a behaviour. The lake's wind bends them, but the preview doesn't.
- The drafts need the user's word, including the names `ChivesGrass` and `RosemaryGrass` for two herbs.
- No draft mentions `detail`. Sizing was the user's request, but nothing records the user asking for `detail`. It serves the lake's meadow.
- Task 16 (in `tasks/doing/`) will make `leaves` and `stems` pickable parts.

## History

- 2026-09-24, 278ad67: added the seven kinds with sizing, `detail`, the shared parts, their previews, draft specs, and rules.md Grasses.
