# Forest lake: a stone path from the user's reference sheet

The user asked (2026-10-01): "add task," with one reference sheet, saved as `typescripts/animation/docs/references/stone_path.png`. Open it before anything else. That line and the sheet are the user's; the rest of this file was written by Claude from them. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

The sheet is a stone path of flat, chunky flagstones in the forest lake's look. Its main picture has the forest lake's own fence, lamp post with lantern, sign post, boulders and waterfall. Its panels:
- **Variation:** regular, mossy, dirty, ruined and stairs, each a straight strip.
- **Tile example (top view):** four square pieces.
- **Components:** the stones one by one, from big slabs to small chips, some mossy.
- **Edge & wireframe:** a stone is a low-poly slab with bevelled, faceted edges.
- **Material / texture:** albedo, normal, roughness and AO maps.
- **Environment usage:** the path in summer, autumn and winter.

Read the forest lake's spec `src/environtments/ForestLake/ForestLake.md` (item 6 is the forest path) and its state `docs/states/forest_lake.md`. Its Known issues say "the references' stone steps, wooden stairs and cliff path are not built". Read `src/figures/ForestLake/SpaceshipWreck/SpaceshipWreck.md` as the latest asset built from a sheet, and `docs/3d_modelling/rules.md` (Project rules 2–7, Figure specs, Checking the result). All paths below are in `typescripts/animation`.

Today the forest path is sandy dirt coloured into the ground (`Terrain.ts`), with flat stones and pebbles scattered in it (`flatStone` in `Scatter.ts`).

## What changes

- **A new asset, `StonePath`,** one of the forest lake's (the Figures tab's ForestLake folder).
  - **Where it goes:** its own folder `src/figures/ForestLake/StonePath/`, with a spec `StonePath.md` written from the sheet as a draft for the user to correct, as the wreck's is.
  - **Its parts (Project rule 4):** the stones, what fills between them, and the verge along its sides. Each can be previewed alone (`src/parts.ts`).
  - **Its options:** `variation` (`regular`, `mossy`, `dirty`, `ruined`, `stairs`), `length`, `width`, `season` and `seed`.
  - **The variations,** as the sheet draws them:
    - **regular:** stones laid close, grass in the joints and tufts along the edges.
    - **mossy:** wider joints full of moss and plants, moss on some stones.
    - **dirty:** smaller stones spaced out in brown dirt, pebbles between.
    - **ruined:** broken stones, tilted and raised, some missing, rubble round them.
    - **stairs:** oblong stone blocks, two to a step, grass at the sides.
  - **A stone (default)** is a slab with a top of 5–8 corners, a little domed and faceted. Its edge is bevelled and its sides taper out to its foot, as the wireframe panel shows: about 40–80 triangles. There are a few shapes of each size, as the components panel shows. Its colours are picked from the sheet: grey with a cool tint, paler on top, darker at the edges, some with a patch of moss.
  - **No texture maps (default).** The forest lake is flat-shaded triangles with a colour at each corner (`parts.ts`). What the sheet's maps show goes in that way:
    - the albedo is the stones' colours;
    - the normal map is the facets;
    - the AO map is darkness baked into the colours at the joints and round each stone's foot.
  - **Sizes (default):** a path 1.4–1.8 m wide; stones 25–70 cm across and 8–12 cm thick, their tops at most 5 cm over the ground. A step rises about 15 cm and is about 35 cm deep.
  - **In each season,** as the sheet's Environment usage row shows:
    - spring and summer: green joints and verge, a few flowers (more in spring);
    - autumn: gold grass, orange and red leaves lying on and between the stones;
    - winter: snow filling the joints and banked along the sides, the stones' tops showing with a little snow on them (`snowOn()`).
- **Its preview,** `?figure=stone-path`: the five variations side by side, as the sheet's Variation row lays them out (default). It shows the forest lake's season when shown there, spring anywhere else, as the other assets do (`forestAsset` in `src/previews.ts`).
- **In the forest lake: its paths become stone paths (default).** They are laid along the paths' lines in `layout.ts`, following their curves rather than repeating square tiles. The scattered flat stones go, since the path takes their place, and the ground's path colour stays under it.
  - **(default) Which variation where:**
    - regular: the main path from the gate through the camp, along the south shore past the landing, to the bridge;
    - mossy: north along the east shore to the cave, in the forest's shade;
    - dirty: round the camp and its branch to the pier;
    - ruined: the branch to the ruins, more broken the nearer it gets;
    - stairs: where a path climbs more steeply than 1 in 5, measured along `layout.ts`'s paths. If no stretch is that steep, the stairs appear only in the asset and its preview, and Done says so.

    Keep which variation goes where in one table: task 20 then mixes the variations along each path, with other kinds of path between them.
  - **Nothing else moves (default).** The rest of the scatter, the landmarks, the fences and the lamp posts stay where they are, as the wreck's task checked. What stood on a path before still does.
  - **Walking:**
    - The stones' tops stay under 6 cm over the ground, so the forest lake meeting's animals walk over them (`LOW` in `story/forest_lake_meeting/ground.ts`). The stones aren't solid to its cameras either (`ForestSight.ts`).
    - The stairs are different: their treads become ground (`ForestLake.landAt()` and `groundAt()`), so the walking camera and the meeting's animals climb them, as the walking camera climbs the island's steps.
  - **Budget (default):**
    - The paths are instanced, as the scatter is.
    - They add at most 300,000 triangles and 0.15 s to the forest lake's build (about 2.2 M triangles and 1.7 s today).
    - They add no frame-rate cost you can measure.
  - The forest lake's spec item 6 and the forest path's lines in `ForestLake.md` say it is a stone path, and which variation is where.

Task 18 (in `tasks/doing/` when this was written) is changing how the forest lake builds: in pieces between frames, each under 100 ms, so the forest lake meeting can change season every minute. Build the asset first. If 18 is still being done when you place the paths, change only the lines you need, and keep the paths' build a piece of its own that fits in 100 ms. Other sessions may be changing the forest lake's files, `src/previews.ts` and `src/parts.ts`. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck`.
- **In Node, the asset** in each variation and season:
  - its triangles and meshes;
  - its size;
  - its stones' tops never more than 5 cm over its ground (not counting the stairs);
  - nothing NaN.
- **In Node, the forest lake** in each season, against the code before this task (extracted beside it and deleted after, as the wreck's task did):
  - the triangles and the build time added;
  - every scatter copy off the paths in the same place;
  - the landing's height unchanged;
  - the forest lake meeting's land unchanged off the stairs: none of the paths' stones counted as something to go round.
- **Screenshots** in headless Edge on the GPU, on a spare port, never 8087 (rules.md, Checking the result; close the browser afterwards), with the Kuwahara filter on:
  - the preview in each season, from the sheet's angle and from straight above like its tile panel;
  - one component stone close up, against the sheet's components and wireframe;
  - in the forest lake, the path by the landing with its fence and a lamp post, from about where the sheet's main picture stands, in all four seasons, beside the sheet's Environment usage row;
  - the mossy, dirty and ruined stretches, and the stairs if there are any;
  - the frame rate at the landing and along the path at 1400 × 900, the new forest lake and the one before, one after the other, twice each;
  - no page errors.
- Bring rules.md in line and add what you learned. Update the state files `docs/states/forest_lake.md` (its assets, and the Known issue about the paths), `index.md`, and `forest_lake_meeting.md` if the meeting's land changed (rules.md, Project rule 8).
- Then ask the user before syncing the knowledge graph (CLAUDE.md, Research workflow).
- Don't commit. The sheet in `docs/references/` is new to the repo; leave it for the user to decide whether to keep it.

Taken: 2026-10-01 01:30

## Done

Finished 2026-10-01. Not committed. The sheet in `docs/references/` is left for you to decide whether to keep.

**What changed** (in `typescripts/animation`):

- **The asset, `StonePath`,** in `src/figures/ForestLake/StonePath/`:
  - its spec `StonePath.md`, a draft from the sheet;
  - `StonePath.ts`, the figure, and `StonePathVariations`, its preview's five strips;
  - its parts `Stones.ts`, `Joints.ts` (the ground's fill and what grows or lies in the joints) and `Verge.ts`;
  - `parts.ts`: the shapes, the colours, the courses and the laying.
  - Options `variation`, `length`, `width`, `season`, `seed`, and `stretches` (plus `light`) for laying it along the forest lake's paths.
  - The preview is `?figure=stone-path`, the five variations side by side, and its parts preview on their own (`src/parts.ts`).
- **A flagstone** is a slab with 5–8 corners: a square with its corners well off it, some clipped or a side bowed, a little domed and faceted round its middle, bevelled, its sides tapering out, 35–56 triangles.
  - The stones are laid in rows. Each is scaled across and along to fill its own place less a joint, turned to the ground's slope, its top at most 5 cm over the ground.
  - Each variation is as the sheet draws it, from close regular paving to the spaced-out dirty path and the broken, tilted ruined one with rubble.
  - The stairs are oblong blocks, two to a step. A step rises about 15 cm and is as deep as the slope takes (0.34–0.8 m).
  - The sheet's maps are baked into the corner colours: the albedo, the facets for its normal map, and darker joints and stone feet for its ambient occlusion.
  - The seasons follow its Environment usage row: flowers in spring (fewer in summer), gold grass and leaves on and between the stones in autumn, snow in the joints and banked along the sides in winter with a little on some of the stones' facets.
- **In the forest lake its paths are stone paths** (`Paths.ts`; `ForestLake.ts`):
  - They follow the paths' curves, instanced: about 33 draw calls for all of them.
  - **The one table** (`PATH_VARIATIONS`):
    - regular along the south shore to the bridge, and on the way in from the gate;
    - mossy along the east shore to the cave, and on the heights path (not named in the task: my choice);
    - ruined on the branch to the ruins, more broken nearer them;
    - dirty on the pier's spur.
  - **The rules over it:** dirty round the camp; stairs where a path climbs more steeply than 1 in 5; nothing where the heights path runs down the cliff over the cave (its last 2 m).
  - **Stairs:** 24 steps. 7 on the east-shore path's rise by the cliffs, 3 on the ruins' branch, 14 down the heights.
  - **What goes, what stays:**
    - The ground's path colour stays under the stones.
    - The scatter's flat stones in the paths go, with the grass, flowers and leaves under the stones, taken out after everything is placed.
    - What stood in a path (rocks, trees) still does.
    - The scatter's pebbles in the paths stay.
- **The stairs' treads are ground** (`ForestLake.landAt`), so the walking camera climbs them.
  - The forest lake meeting's land is measured without them (`landAt(x, z, false)`). A 15 cm tread's edge counted as ground too steep on the land's half-meter cells, and the three steps on the ruins' branch lost their room.
  - Its animals stand on the stairs up a ramp through the risers' middles (`StonePath.rampAt`, `ground.ts`), within half a riser of each tread.
- **Text:** the forest lake's spec item 6; rules.md Environments rule 27 and Checking the result rule 10; the state files `forest_lake.md`, `forest_lake_meeting.md` and `index.md`.

**How it was checked:**

- `npm run typecheck`: clean.
- **In Node, the asset** in each variation and season:
  - 3,100–5,000 triangles a strip (2,200–3,700 in winter), 7–24 meshes;
  - about 1.6 × 4.2 m, or 2.4–2.8 m across with its verge and winter's banks;
  - its stones' tops at most 4.5 cm over the ground (the stairs aside);
  - 9 steps on the stairs' strip;
  - nothing NaN;
  - the preview 20,000 triangles.
- **In Node, the forest lake against the code before this task** (`src` copied beside it with this task's lines taken out, deleted after), in each season:
  - the stone paths 80,000–133,000 triangles and 27–33 ms to build, a piece of their own in `ForestLake.build`, so under task 18's 100 ms;
  - the whole build's time within the run-to-run noise;
  - no scatter copy moved; 40–118 taken out, all under the paths;
  - the landing's height unchanged;
  - the forest lake meeting's land the same cell for cell, with the same count of obstacles to go round;
  - the room on every step the same as before.
- **Headless Edge on the GPU** (RTX 3050, port 8156, 1400 × 900, Kuwahara on; screenshots in the session's scratchpad):
  - the preview in each season, from the sheet's angle and from above like its tile panel;
  - a flagstone close up, against the components and wireframe;
  - the path by the landing with its fence, lamp post and sign post, from each way along it, in all four seasons, beside the Environment usage row;
  - the mossy, dirty and ruined stretches;
  - the stairs by the east shore and down the heights;
  - the way in through the gate;
  - no page errors.
- **Frame rates,** the new forest lake and the one before (served from a page of its own), one after the other, twice each:
  - at the landing 54.6 and 49.4 frames a second, against 50.1 and 51.4 before;
  - along the path 49.4 and 51.6, against 49.2 and 50.0.
  - No cost to measure. The first version's verge was twice as thick, and along the path it measured 48.7–49.2 against 52.4–52.7, so the verge was thinned (118,000 triangles to 55,000).

**Left for you:**

- The spec and the table of which variation is where are drafts. Is the heights path's mossy what you want?
- **The stairs' rises:**
  - The ruins' branch climbs only 1 m, so its stair is 3 steps.
  - The east-shore path's 7 steps go up and over a hump by the cliffs. There, as down the heights, the meeting's animals had no room before and still have none.
  - On the stairs the animals' feet are up to half a riser (7.5 cm) off the treads.
- **Things not changed:**
  - The heights path's dirt colour still runs down the cliff over the cave, where no stone is laid (the ground's colour is the terrain's).
  - The scatter's pebbles still lie in the paths.
  - The Scatter's flat stone template is still built, though all its copies are taken out; removing it touches Scatter.ts, which the grass session was changing.
