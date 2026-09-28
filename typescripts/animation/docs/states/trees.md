# Trees state

Updated 2026-09-28 by the crystal tree, the user's own model in its spec (asked for directly: "continue implementing figure", with CrystalTree.md open; no task file). First written 2026-09-25 by task 17, from the code as it was then.

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
- **Parts:** each tree's `trunk` and `crown` can be shown on its own (`?part=`, [parts.ts](../../src/parts.ts)).
- **The crystal tree** ([CrystalTree.md](../../src/figures/Tree/CrystalTree.md), [CrystalTree.ts](../../src/figures/Tree/CrystalTree.ts), [crystalTreeModel.ts](../../src/figures/Tree/crystalTreeModel.ts), `crystal-tree`), not grown like the eight: the user's own low poly model, the three.js page under "# Shape Reference" in its spec, built as the page builds it, corner for corner ([rules.md, Trees](../../../../docs/3d_modelling/rules.md#trees) rule 7).
  - A fantasy tree 5.7 m tall on its own grassy disk 6.2 m across: an S-curved trunk with five roots, five branches forking twice, star-like clusters of crystal shards at their ends and on top of the crown, crystals hanging on strings from the outer clusters, rocks, crystals and tufts of grass on the disk, and sparkles round it all. 6,895 triangles in 17 meshes at the page's seed (7), built in about 7 ms.
  - The page's colours mapped onto felt: bark `wood` toward `autumn`, crystals `zenith` and `flower` darkened to the page's lightness, the disk `grass` toward `sand`, rocks `stone`. The crystals are glossy and glow in each face's colour, and two lights, `flower` in the crown and `zenith` beside it, light the tree as the page's do.
  - It plays the page's animation (the hanging crystals swing, the sparkles drift round and twinkle). The spec names no behaviour; the page's Glow slider is `SetGlow(amount)`, a button in the preview. Options: `seed`, `glow`, `ground: false` (no disk), `lights: false`.
  - Parts: `trunk`, `crystals`, `pendants`, `ground`, `sparkles`. Its preview frames it as the trees' does (`treeView()`).
- **Reused elsewhere:** the grasses build on `clumpGeometry`, `limbGeometry` and `seededRandom` from parts.ts (see [grasses.md](grasses.md)). Lake and sky code borrows `seededRandom` and `between`.

## How it was last checked

- 2026-09-28, the crystal tree:
  - The page's own code run in Node beside `crystalTreeModel.ts` for seeds 7, 1, 42 and 123456: every corner, colour, hanging crystal and sparkle the same (6,895, 7,073, 6,847 and 6,771 triangles).
  - The figure in Node: built in 6.7–7.8 ms; no NaN over a minute of its animation; its options and `SetGlow` (clamped to 0–1).
  - Seen beside the page's own front view in headless Edge, with and without the painterly filter; `npm run typecheck` clean.
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
- The crystal tree's crystals are felt's muted sky blue and lilac, not the page's vivid blue and violet, and nothing here blooms as the page does; its sparkles barely show against the day sky (the page's show on its dark background). Its trunk's tilted foot and its disk's soil edge reach 16–20 cm below the ground, as on the page.
- rules.md Trees rule 6 gives 20–80 k triangles and 10–60 ms per tree. Today the willow has 131 k triangles, and warm builds of the chestnut and the willow took over 100 ms.
- No preview shows an autumn tree: `tree()` builds each kind with `{ theme }` only, so `?figure=maple-tree` is always green. Autumn trees appear only at the lake.

## Open questions

- What should the trees do? Their specs name no behaviour, so they stand still. The preview doesn't sway them, though the lake does.
- `BirchTree.md` and `PoplarTree.md` exist, empty, with no figure. Should a birch and a poplar be built?
- `autumn` is in no spec. It came from the user's request for autumn trees at the lake (the comment in `Lake/Trees.ts`). Should the specs name it, and should the preview offer it?
- Should the crystal tree keep felt's colours, or the page's own (`MODEL_COLORS` in crystalTreeModel.ts, one line)? And should its spec name `SetGlow`?

## History

- 2026-09-28, no task file, not committed: the crystal tree, from the user's model in CrystalTree.md, with its preview and parts; the trees' camera framing shared as `treeView()`.
- 2026-09-24, after 278ad67, not committed yet, with no task file: added `autumn` in `TreeOptions` (the autumn colour and mottled tints), for the lake's autumn maple and oak.
- 2026-09-24, 278ad67: added the eight kinds on the shared trunk, crown and skeleton, their previews, and rules.md Trees. `BirchTree.md` and `PoplarTree.md` were added as empty specs.
