# Trees state

Updated 2026-09-25 by task 17 (the first state record), from the code as it is today.

In progress: listing each tree's `trunk` and `crown` as parts in the Figures tree, to preview one on its own (`src/parts.ts`, task 16), being built by another session.

## What works

- **Eight kinds**, each a class next to its spec in `src/figures/Tree/`: [OakTree](../../src/figures/Tree/OakTree.ts), [CedarTree](../../src/figures/Tree/CedarTree.ts), [MapleTree](../../src/figures/Tree/MapleTree.ts), [PineTree](../../src/figures/Tree/PineTree.ts), [ChestnutTree](../../src/figures/Tree/ChestnutTree.ts), [SpruceTree](../../src/figures/Tree/SpruceTree.ts), [ElmTree](../../src/figures/Tree/ElmTree.ts) and [WillowTree](../../src/figures/Tree/WillowTree.ts). The rules are in [rules.md, Trees](../../../../docs/3d_modelling/rules.md#trees).
- **Specs:** [OakTree.md](../../src/figures/Tree/OakTree.md), [CedarTree.md](../../src/figures/Tree/CedarTree.md), [MapleTree.md](../../src/figures/Tree/MapleTree.md), [PineTree.md](../../src/figures/Tree/PineTree.md), [ChestnutTree.md](../../src/figures/Tree/ChestnutTree.md), [SpruceTree.md](../../src/figures/Tree/SpruceTree.md), [ElmTree.md](../../src/figures/Tree/ElmTree.md) and [WillowTree.md](../../src/figures/Tree/WillowTree.md) are all empty files. The code follows its own comments and rules.md.
- **Shared parts:**
  - [Trunk.ts](../../src/figures/Tree/Trunk.ts): the trunk and every branch, one bark mesh, with the foot flared into roots.
  - [Crown.ts](../../src/figures/Tree/Crown.ts): every clump of leaves or needles, one mesh, darker low down and toward the trunk.
  - [skeleton.ts](../../src/figures/Tree/skeleton.ts): the wood grown by space colonization, with the big limbs drawn by hand (`fan()`) and radii by the pipe model.
  - [parts.ts](../../src/figures/Tree/parts.ts): `TreeOptions`, the materials, the seeded random generator, and the limb and clump geometry.
  - Each tree exposes `readonly trunk` and `crown`, with its origin on the ground at the foot of the trunk. So a tree is two draw calls.
- **Sizes** at the default seed, measured today: oak 12.4 m tall and 16.7 m across, cedar 15.2 m, maple 15.6 m, pine 18.0 m, chestnut 16.5 m, spruce 20.4 m, elm 19.5 m, willow 10.6 m. The spruce is built directly, in whorls every 60 cm, not grown. The willow's curtain is cascades of clumps, capped at 9 clumps each (`most`).
- **Seeded:** `new OakTree({ theme, seed })` always grows the same tree from the same seed. Each kind has its own default seed.
- **Colours from the theme:** bark is `bark(theme)`, the wood colour with the bark texture made in code ([bark.ts](../../src/textures/bark.ts)). Leaves are the `grass` colour times the kind's `FOLIAGE_SHADE`, darker for the conifers.
- **Autumn:** with `autumn: true` in `TreeOptions`, the leaves are the theme's `autumn` colour, and each clump gets one of four tints at random (`AUTUMN_TINTS` in parts.ts, `tint()` in Crown.ts). An autumn tree has the same wood as the green tree grown from the same seed. Only the clumps differ. A tree grown without `autumn` draws the same random numbers as before. Only the lake uses it, for one maple and one oak.
- **Preview:** `oak-tree` … `willow-tree` in [previews.ts](../../src/previews.ts), through `tree()`, which moves the camera back by the tree's height and width. Trees start on the `grass` environment.
- **No behaviour:** in the preview a tree stands still (`update() {}`). At the lake the wind sways the crowns through their material, and the wood stays still. See [environments.md](environments.md) for how the lake places them.
- **Reused elsewhere:** the grasses build on `clumpGeometry`, `limbGeometry` and `seededRandom` from parts.ts (see [grasses.md](grasses.md)). Lake and sky code borrows `seededRandom` and `between`.

## How it was last checked

- 2026-09-25 (task 17, while writing this file): a numeric run in Node ([rules.md, Checking the result](../../../../docs/3d_modelling/rules.md#checking-the-result) rule 8).
  - Every kind builds as two meshes with no NaN, and the same seed twice gives the same crown.
  - All 20 other seeds tried per kind build, and each kind keeps its height (the oak's range is 12.2–13.1 m). `autumn: true` builds for every kind, with the same wood.
  - Triangles: from 22 k (pine) to 69 k (elm), and 131 k for the willow. Warm builds took 20–120 ms while other sessions loaded the machine.
- 2026-09-25, task 17: `npm run typecheck` is clean at 23:10, with other sessions' unfinished work in the tree (earlier that evening it failed for a few minutes on the dock's half-built lines, task 09).
- 2026-09-24 (278ad67): no dated check is recorded. rules.md Trees rules 5–6 record that every kind was built in Node with many seeds, which found the willow's endless cascade. They also give the costs. No screenshot is recorded.
- Autumn (2026-09-24, not committed): rules.md Trees rule 4 records a change made after a look at the dark `studio` theme, where faded clumps showed nearly white. Environments rule 13 records that both autumn trees stand in view on the far shore. There is no dated screenshot.

## Known issues and left for the user

- No task in `tasks/done/` changed the tree figures, so no Done section leaves anything for the user.
- The specs are empty, so nothing the user wrote says what a tree is. The grasses got drafts written from the code ([rules.md, Figure specs](../../../../docs/3d_modelling/rules.md#figure-specs) rule 3). The trees did not, because their files exist and are empty.
- rules.md Trees rule 6 gives 20–80 k triangles and 10–60 ms per tree. Today the willow has 131 k triangles, and warm builds of the chestnut and the willow took over 100 ms.
- No preview shows an autumn tree: `tree()` builds each kind with `{ theme }` only, so `?figure=maple-tree` is always green. Autumn trees appear only at the lake.

## Open questions

- What should the trees do? Their specs name no behaviour, so they stand still. The preview doesn't sway them, though the lake does.
- `BirchTree.md` and `PoplarTree.md` exist, empty, with no figure. Should a birch and a poplar be built?
- `autumn` is in no spec. It came from the user's request for autumn trees at the lake (the comment in `Lake/Trees.ts`). Should the specs name it, and should the preview offer it?
- Task 16 (in `tasks/doing/`) will make `trunk` and `crown` pickable parts.

## History

- 2026-09-24, after 278ad67, not committed yet, with no task file: added `autumn` in `TreeOptions` (the autumn colour and mottled tints), for the lake's autumn maple and oak.
- 2026-09-24, 278ad67: added the eight kinds on the shared trunk, crown and skeleton, their previews, and rules.md Trees. `BirchTree.md` and `PoplarTree.md` were added as empty specs.
