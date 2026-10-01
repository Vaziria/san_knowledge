# Forest lake: a wider lake, a bigger island with a great crystal tree, trees of more sizes

The user asked (2026-10-01):

> add task 1. the lake is wider
> 2. the island in the middle of the lake is bigger and make the tree a cristall tree, make the tree big, like the nuance of a majestic tree that guards the forest
> 3. the size of the trees in the area is more varied

Those lines are the user's; the rest of this file was written by Claude from them. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

All paths below are in `typescripts/animation`. Read:
- the forest lake's spec `src/environtments/ForestLake/ForestLake.md` and its state `docs/states/forest_lake.md`;
- the crystal tree: `src/figures/Tree/CrystalTree.md` (the user's own spec, a three.js page under "# Shape Reference"), `CrystalTree.ts`, `crystalTreeModel.ts`, and `docs/states/trees.md`;
- the forest lake meeting's state `docs/states/forest_lake_meeting.md`, since its animals, cameras and fish stand on this map;
- `docs/3d_modelling/rules.md` (Figure specs, Checking the result).

**Today** (`layout.ts`, `Landmarks.ts`, `Scatter.ts`):
- **The lake** (`LAKE`) is about 40 m east to west (x −20.3 to 19.5) and 43 m north to south. The cliffs and both waterfalls are to the north, the landing is on the south shore (`LANDING`), the camp, the pier and the ruins are to the west, and the outlet's bridge, the east pier and the cave are to the east.
- **The island** (`ISLAND`) is a plinth of rock about 10 m across, its top 1.2 m over the water (`ISLAND_TOP`), round (6.6, −3).
  - On it (`Landmarks.ts`, "The island, on its plinth of rock"): a cherry tree 7.2 m tall in the middle, a pine behind it, a bush, four flower clusters, two double lamps, a sign, a fence round two sides, and steps up from its jetty.
  - The spring petals fall round the cherry.
- **The forest's trees** are each drawn at an even random size in a narrow range: the pines 5.6–11.6 m tall (8 m × 0.7–1.45), the broadleaf trees 4.8–7.5 m, the cherries 4.4–6.6 m. So the forest's top is fairly flat and even.

## What changes

### 1. A wider lake

- **About 12 m wider east to west (default),** some 52 m across in place of 40, keeping its shape. The west shore moves about 6 m west and the east shore about 6 m east.
- **What stands along those shores moves out with them,** so it keeps its place by the water:
  - on the west: the pier, the boat, the camp, the ruins, the paths there;
  - on the east: the outlet with its cascade, the bridge, the stream, the east pier, the cave, the cliffs' rim on that side, the watch tower and the heights' path.
- **What stays (default):**
  - the landing, the south shore in front of it and the view north from it;
  - the cliffs to the north with both waterfalls and their rivers. The ends of the northern cliffs follow the shores out.
- **The spaceship wreck** moves with the camp, and keeps its place behind the camp's fence.
- **Paths:** tasks 19 and 20 lay stone paths and vary them along `PATHS`. If they are done before this task, their paths move with the shores and keep their stretches and trails. Check that their tables still fit the longer paths.
- **What depends on the map's size grows to fit:**
  - the terrain's fine sampling round the lake;
  - the scatter's grids;
  - the forest lake meeting's land (39 m round the lake's middle) and its cameras' sight (a 58 m square);
  - the walking camera's reach (75 m).

### 2. A bigger island with a great crystal tree

- **The island about 1.7 times across (default),** some 17 m, still a plinth of rock straight out of deep water, 1.2 m high.
  - It stays in the middle of the lake as seen from the landing. It grows mostly north and east, so that the open water north of the landing stays open: at least 4 m of water between the island's rock and the fish's circle. That open water is `ForestLake.OPEN_WATER`, where the meeting's fish swims and a floating figure goes, and where winter's hole in the ice is.
  - The jetty and its steps move out with its south side.
- **Its tree becomes the crystal tree:** the user's own model in `CrystalTree.ts`, grown from the same seed (7), in place of the cherry.
- **Big and majestic, the tree that guards the forest (default):**
  - About 22 m tall, some four times its preview's 5.7 m, its crown about as wide as it is tall, reaching out over the water on all sides. It is the tallest thing in the valley by a clear margin, over the tallest forest trees (at most 16 m, below) and over the cliffs by the waterfalls (about 10 m).
  - The trunk and the five roots grow with it, so they are massive. The roots grip the island's top and run down over the edge of its rock toward the water.
  - It stands alone, with nothing near it as tall. The pine behind it goes. The bush, the flowers, the two double lamps, the sign, the fence and the steps stay on the bigger island, set round the tree. The lamps' crossbars still run round it, as they do round the cherry.
  - It is seen from everywhere round the lake, and from the landing it fills the view north. Check that the forest lake meeting's opening shot and the host's view still show the host clearly with it behind.
- **Its model stays the user's (default):** the same branches, forks and crystal clusters, grown bigger rather than redrawn. Leave `CrystalTree.md` as the user wrote it. Add what the forest lake needs to `CrystalTree.ts` as options, described in its comments and in `trees.md`: its height, the colours below, a season, and a ground of its own or none.
- **Its colours (default):**
  - The crystals in its page's own blue and violet (the colours in `CrystalTree.md`'s page), not felt's muted ones, since the forest lake has its own look and not the theme's.
  - The bark and anything growing round it take the forest lake's palette (`parts.ts`).
  - Its preview on the floor keeps felt's colours, as now.
- **Its glow:**
  - The crystals glow as now.
  - Its two lights are strengthened so they light the tree and the island round it at its new size.
  - Its hanging crystals swing and its sparkles drift round the whole crown.
  - The island's falling spring petals give way to its sparkles, in every season (default).
- **Round its roots (default):** no grassy disk of its own; the island's ground is its ground. A few clusters of crystals grow from the island's ground and rock round its roots, as they grow from its disk.
- **In each season:**
  - the crystals are the same all year and glow in every season;
  - in winter, snow lies on its trunk's, roots' and branches' upturned faces (`snowOn()`), and the island's things take the season as they do now.
- The forest lake's spec (the island) and the layout's comments say it is the crystal tree now.

### 3. Trees of more sizes

- **Each kind's height drawn from a wider range, in three ages (default):**
  - **young trees,** about one in six, 2–4 m tall: at the forest's edges, in clearings, along the paths and by the shore, where light reaches;
  - **grown trees,** most of them, 6–12 m;
  - **old trees,** about one in ten, 13–16 m: deeper in the forest and on the heights, standing up out of the forest's top.

  The broadleaf trees and the cherries run the same way in their own ranges: cherries 3–8 m, broadleaf trees 3–11 m. The winter's bare trees and the far forest's pines vary too.
- **They are not just smaller or bigger copies (default).** Young ones are slender and old ones broad: an old tree's trunk and crown are wider than its height alone would make them. If scaling one template can't give that, add a young and an old template to each kind, as the scatter already keeps a few per kind.
- **The forest's top is uneven:** old trees stand out of it, young ones fill its edges.
- **What else follows the sizes:** the shades under the trees, the trunks' circles the meeting's animals go round (`obstacles`), and the falling petals and leaves round the trees by the landing.
- **Kept as now (default):**
  - the view north from the landing stays open (`inView`);
  - the trees stand where they stand, only their sizes change;
  - the sizes come from the seeded draws, so the forest is the same on every load.

### Budget

- **At most 15% more (default)** in the forest lake's triangles (about 2.2 M today) and in its build time (about 1.7 s).
- **No frame-rate cost** you can measure at the landing.

Task 18 (in `tasks/doing/` when this was written) is making the forest lake build in pieces between frames, each under 100 ms, so the forest lake meeting can change season every minute. If it is in by then, keep each piece this task grows under 100 ms. Tasks 19 and 20 also change the forest lake's paths, `Scatter.ts` and `layout.ts`. Other sessions may be changing the forest lake's files, the crystal tree's, `src/previews.ts` and the forest lake meeting's. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck`.
- **In Node, the forest lake** in each season:
  - the lake's width east to west and length north to south, and the island's size;
  - the landing's height and place unchanged;
  - nothing standing in the water that stood on land before;
  - no path, pier or bridge left short of its shore or off it;
  - the waterfalls' pools still meeting the lake;
  - the open water round `OPEN_WATER` at least 4 m from the island's rock;
  - the crystal tree's height and crown width, with nothing on the island inside its trunk or roots;
  - the forest's heights, as a histogram per kind (young, grown, old), and the tallest tree against the crystal tree;
  - the triangles and build time against the code before this task (extracted beside it and deleted after);
  - nothing NaN.
- **In Node, the forest lake meeting:**
  - its land covers the wider lake's shores;
  - no animal can reach the island;
  - its fish's circle and its leaps (a supporter's up to 1 m) stay in open water, in winter in the hole;
  - its cameras' sight has the crystal tree and the bigger trees in it.
- **Screenshots** in headless Edge on the GPU, on a spare port, never 8087 (rules.md, Checking the result; close the browser afterwards), with the Kuwahara filter on:
  - the overview from above and from the south, in all four seasons, beside the same views before this task;
  - the crystal tree from the landing, from each shore and close to its roots;
  - at night if the forest lake has night by then (it has none today), to show its glow;
  - the forest's edge with young and old trees, from the landing and from the heights;
  - the forest lake meeting's opening shot and a turn with the host on the south shore;
  - the frame rate at the landing at 1400 × 900, the new forest lake and the one before, one after the other, twice each;
  - no page errors.
- Bring rules.md in line and add what you learned. Update the state files `docs/states/forest_lake.md`, `trees.md`, `forest_lake_meeting.md` and `index.md` (rules.md, Project rule 8).
- Then ask the user before syncing the knowledge graph (CLAUDE.md, Research workflow).
- Don't commit.

Taken: 2026-10-01 03:06

## Done

Finished 2026-10-01. Not committed.

**What changed** (in `typescripts/animation`):

- **1. A wider lake** (`layout.ts`): about 52 m east to west, where it was 40 m (x −26.3 to 25.5), still 43 m north to south.
  - **One warp** moved the map. Everything west of the landing moves up to 6 m west: all of it from x −10, fading out north of z −17, so the small waterfall and the northern cliffs stay. Everything east of the main waterfall moves up to 6 m east, all of it from x 13. The warped numbers are written back into `layout.ts` as the map.
  - **What moved out with its shore:** on the west, the pier, the boat, the camp, the ruins, the wreck, the paths there and the wreck trail; on the east, the outlet, the bridge, the stream, the east pier, the cave, the eastern cliffs' rim, the watch tower, the heights' path and the heights trail. The heights trail still ends at the footbridge's deck.
  - **What stayed:** the landing, the south shore in front of it, the way in from the gate, both waterfalls and their rivers.
  - **Literals outside the map, moved with it:** a sign post; the tower's turn (now toward its path's point); and the fence by the bridge, now measured from the path's end, since the main path grew from 34 to 45.9 m. The paths' kinds and trails re-plan on the longer paths.
  - **What grows to fit the map:**
    - the terrain's fine sampling (x ±68, was ±62), and its hills rise from further out east and west;
    - the reed and lily grids;
    - the walking camera's reach (81 m, was 75);
    - the forest lake meeting's land (`REACH` 52 m, so 45 m walked; was 46 and 39);
    - the meeting's cameras' sight: a square 64 m each way (was 58), and 28 m up (was 17), for the crystal tree.
- **2. A bigger island with the great crystal tree:**
  - **The island:** 16.3 × 16.8 m, grown 1.7 times north and east from its south-west. The jetty and its steps moved out with its south side. Its rock is 4.25 m from the fish's circle.
  - **The tree:** the user's crystal tree from seed 7, 22 m tall (`CRYSTAL_TREE` in `Landmarks.ts`), with a crown 20.6 × 16.9 m reaching out over the water. The cherry and the pine are gone.
  - **Its roots** are massive and grip the top, then run 4.8–8.6 m out and down over the rock toward the water. The one toward the jetty's steps stops short of them.
  - **Its colours:** crystals in the page's own blue and violet, bark in the forest lake's colours, snow on the bark's upturned faces in winter.
  - **Its glow:** its two lights reach as far as it has grown. Its hanging crystals swing, and its sparkles drift round the whole crown in every season, in place of the island's falling petals.
  - **Round it:** no disk of its own. Some of its crystals of the ground stand on the island's ground and rock. Between its roots stand the bush, four flower clusters and the two double lamps (their crossbars round the tree), with the sign by the steps. The fence runs round two sides, with a gap wherever a root passes under it.
  - **`CrystalTree.ts`,** new options, the preview unchanged:
    - `height`;
    - `colors`, any of its colours;
    - `bark`, a material;
    - `season`;
    - `on`, other ground to stand on and run its roots over;
    - and `roots` out, each root's direction and reach.
  - **`crystalTreeModel.ts`** takes the other ground (`CrystalGround`). `CrystalTree.md` is as you wrote it.
- **3. Trees of three ages** (`Scatter.ts`):
  - **The ages** (`AGES`):
    - young, 2–4 m pines, slender, where light reaches (edges, clearings, paths, shore);
    - grown, 6–12 m;
    - old, 13–16 m pines, broad in trunk and crown (`WIDE`), deep in the forest and on the heights.
  - **Other kinds' ranges:** broadleaf trees 3–11 m, cherries 3–8 m. The bare trees and the far pines vary too.
  - **Same places:** each age comes from a hash of where the tree stands, so every tree kept its place.
  - **True heights:** heights are scaled to each template's measured top (`TOPS`); a default cherry is 4.86 m, not 5.5.
  - **What follows the sizes:** the shades, the trunks' circles and the falling leaves and petals.
- **Text:**
  - the forest lake's spec (the lake, the island and its crystal tree, the forest's ages, spring on the island);
  - the layout's comments;
  - rules.md: Environments rule 28's last line brought in line, and a new rule 29;
  - the state files `forest_lake.md`, `trees.md`, `forest_lake_meeting.md` and `index.md`.

**How it was checked:**

- **`npm run typecheck`:** clean.
- **In Node, the forest lake against the code before** (`src` copied beside it; deleted after), in each season:
  - **Sizes:** the lake 39.8 × 43.1 m → 51.8 × 43.1 m; the island 9.6 × 9.9 m → 16.3 × 16.8 m.
  - **The landing** is at (−2, 0.4241, 14.4), unchanged.
  - **Water:** no tree and no landmark in the water (but the piers, the boat and the footbridge, which belong there).
  - **Piers and bridges:** the piers' ends are as before (landward 0.45 and 0.35 m over the water, the jetty just off the island as before); the bridge's ends are on land.
  - **Paths and pools:** every path dry; both falls' pools in the lake.
  - **Open water:** the island's rock is 4.25 m from the fish's circle.
  - **The crystal tree:** 22.0 m, nothing on the island inside its trunk or on a root.
  - **The forest's heights** (spring; the other seasons alike):

    | Kind | Trees | Young | Old | Heights |
    |---|---|---|---|---|
    | Pines | 2,141 | 15% | 10% | 2.0–16.0 m |
    | Broadleaf trees | 977 | 16% | 12% | 3.0–11.0 m |
    | Cherries | 47 | 19% | 4% | 3.2–8.0 m |

    Before, every tree was 3.9–12.9 m. The tallest forest tree is now 16.0 m, against the crystal tree's 22 m and the cliffs' 9.6–11 m.
  - **Cost:** triangles 0.1–0.8% fewer (fewer trees round the wider lake); the build 1–4.5% longer (1.52–1.82 s).
  - Nothing NaN.
  - **The crystal tree's preview** is the same as before, corner for corner (6,895 triangles, the same bounds, the same sums of every position).
- **In Node, the forest lake meeting:**
  - **Land:** walkable cells 3,381 → 5,064; the shore 1.5 m in walked to at 31% of it, where it was 25%.
  - **The island:** no cell of it can be reached.
  - **The fish:** its circle and a leap from it stay 3.25 m from the island's rock and 5.3 m from the shore, inside winter's hole.
  - **Sight:** all 304 of the crown's crystals more than 14 m up are solid to its cameras.
  - **Trails:** both trails' walked strips are still clear.
- **In headless Edge on the GPU** (RTX 3050, test server on port 8156, 1400 × 900, the Kuwahara filter on):
  - the overview from above and from the south in all four seasons, beside the same views before;
  - the crystal tree from the landing, the west, east and north shores, and by its roots, in spring and winter;
  - the forest's edge from the landing and from the heights;
  - the forest lake meeting's opening shot and its turns over its first 48 s, now and before: the host and the animals still show clearly.
  - No page errors.
  - The forest lake has no night yet, so the tree's glow wasn't seen in the dark.
- **Frame rates,** the forest lake now and before, one after the other, twice each:
  - at the landing, 49.1 and 49.3 frames a second, against 49.0 and 49.1;
  - along the path, 48.1 and 48.5, against 48.6 and 48.9.

  The crystal tree's two lights cost nothing measurable.
- The before-copy (`src-before-21/`, `before-21.html`) is deleted, the test server stopped by its PID, and the browsers closed.

**Left for you:**

- **Sizes and shares are defaults.** The widening (12 m), the island's size (1.7 times), the tree's 22 m and the ages' shares are this task's defaults.
- **The trunk is hollow to the meeting's cameras.** Only its surface counts as solid. No camera goes near it, since the island is out of the animals' reach.
- **The sparkles** barely show by day.
- **The crystal tree's preview** on the floor keeps felt's colours; only the island shows the page's own. `trees.md` keeps the open question.
- **Knowledge graph:** tracked docs changed. Syncing is asked about once the task queue is done.
