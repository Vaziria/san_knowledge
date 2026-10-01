After: 19-forest-lake-stone-path.md

# Forest lake: paths that vary more

The user asked (2026-10-01): "add task, make path in forest lake more variative". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

All paths below are in `typescripts/animation`. Read the forest lake's spec `src/environtments/ForestLake/ForestLake.md` (item 6, the forest path), its state `docs/states/forest_lake.md`, and task 19's spec and Done section in `tasks/done/`. Task 19 built the stone path from the user's sheet (`docs/references/stone_path.png`) in five variations, and laid it along the paths.

Today the forest lake has six paths (`PATHS` in `layout.ts`). Each is the same width all along (1.6–2.3 m) with soft, even edges, coloured into the ground as sandy dirt with grit (`Terrain.ts`, `pathAt`). Task 19 gives each stretch one variation of its stone path. So a path looks the same from end to end, and the paths look much like each other.

## What changes

- **Each path changes as it goes (default).** It is a run of stretches 8–30 m long, no two neighbours of the same kind. Where two meet they blend: a stone path breaks up into single stones, then dirt. The kinds:
  - task 19's stone path: regular, mossy, dirty, ruined and stairs;
  - sandy dirt, as now;
  - dirt worn into two tracks, with grass up the middle;
  - stepping stones: single flat stones in the grass, a stride apart (the old `flatStone` in `Scatter.ts` may serve);
  - gravel: pebbles packed in dirt, loose ones spilling onto the grass at its edges;
  - log steps: earth held by a log across the path at each step, on steep stretches of the narrow paths, where task 19 puts stone stairs on the wider ones.
- **Each path has its own character (default).** Its stretches are drawn mostly from a few kinds that suit where it goes:
  - the main path, from the gate past the camp and the landing to the bridge: stone mostly, since it is the most walked;
  - on to the cave along the east shore: mossy stone, and dirt in the forest's shade;
  - round the camp and to the pier: dirty stone, and dirt worn into two tracks;
  - to the ruins: ruined stone, then gravel;
  - along the heights: stepping stones, gravel and log steps.

  Keep which kinds go where in one table, easy to change.
- **Their shape varies (default):**
  - The width changes along each path, about 30% either way. It is wider where paths meet and by the camp, the landing, the gate and the sign posts, and narrower in the forest and on the heights.
  - The edges are uneven: the grass reaches in and bare earth reaches out, and now and then a tuft grows in the path.
  - **The paths' lines stay where they are,** so the fences, lamp posts, sign posts and the gate still stand by them.
- **Things along the way (default),** in their season:
  - roots across the forest stretches;
  - puddles in dips (frozen in winter);
  - moss along the shaded edges;
  - a few stones kicked loose onto the grass;
  - in autumn, leaves drifted along the edges;
  - in winter, a trodden track down the middle of the snow, with footprints.
- **A few narrow trails (default),** 0.6–1 m wide, of trodden dirt or stepping stones, to places that no path reaches today:
  - to the spaceship wreck's ramp, from the nearest path;
  - along the heights, from the watch tower's path to the footbridge over the main river (`SPOTS.footbridge`), which stands with no path to it. Only if the land lets a trail through without a cliff in the way; if not, say so in Done.

  What stood on a trail is taken out after placing, as the wreck's clearing does (`cleared` in `Scatter.ts`), so nothing else moves.
- **Walking:** the log steps' treads become ground, like task 19's stairs. Nothing else on the paths rises 6 cm or more over the ground, so the forest lake meeting's animals walk over it (`LOW` in `story/forest_lake_meeting/ground.ts`).
- **Budget (default):** at most 150,000 triangles and 0.1 s more than task 19 left, instanced as the scatter is. If task 18's piece-by-piece build is in by then, the paths stay one piece under 100 ms.
- **The spec:** the forest lake's spec item 6 says how each path changes along its way, and names the trails. `StonePath.md` stays as task 19 left it.

Other sessions may be changing the forest lake's files, `src/previews.ts` and the forest lake meeting's files. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck`.
- **In Node,** the forest lake in each season, against the code before this task (extracted beside it and deleted after):
  - along each path, its stretches listed with kind, length and width: none shorter than 8 m or longer than 30 m, no two neighbours alike;
  - the width's range along each path;
  - the triangles and the build time added;
  - every scatter copy, landmark, fence and lamp post in the same place, except what the trails took out (say how many);
  - the landing's height unchanged;
  - the forest lake meeting's land: the paths are walked over, and the new trails are open;
  - nothing NaN.
- **Screenshots** in headless Edge on the GPU, on a spare port, never 8087 (rules.md, Checking the result; close the browser afterwards), with the Kuwahara filter on:
  - walk each path with the walking camera and take one every 10 m or so, in spring and in winter, laid out as a strip per path so the change along it shows;
  - each kind of stretch close up, and one blend between two kinds;
  - the trails;
  - the landing view in all four seasons;
  - the frame rate at the landing and along the main path at 1400 × 900, the new forest lake and the one before, one after the other, twice each;
  - no page errors.
- Bring rules.md in line and add what you learned. Update `docs/states/forest_lake.md`, `index.md`, and `forest_lake_meeting.md` if the meeting's land changed (rules.md, Project rule 8).
- Then ask the user before syncing the knowledge graph (CLAUDE.md, Research workflow).
- Don't commit.

Taken: 2026-10-01 02:10

## Done

Finished 2026-10-01. Not committed.

**What changed** (in `typescripts/animation`):

- **The plan, `Paths.ts`** (`planPaths`), after the scatter is placed:
  - **One table, `PATH_KINDS`,** with the kinds each path takes in turn:
    - the main path (camp to bridge): dirty stone, regular, dirt, regular;
    - the east shore to the cave: mossy, dirt;
    - the ruins' branch: ruined, gravel;
    - the pier's spur: two tracks;
    - the heights: stepping stones, gravel;
    - the way in from the gate: regular, tracks, regular.
  - **Stretches** are 8–10.7 m long, no two neighbours alike. The ruins' branch is only 15.8 m long, so its two stretches are 7.9 m each.
  - **Steps** wherever a path climbs more steeply than 1 in 5: stone stairs on a path 2 m wide or more (6 steps by the east shore), log steps on a narrower one (17, on the ruins' branch and the heights). Nothing is laid down the cliff over the cave.
  - **Width** is sampled every 0.5 m, up to 30% either way: wider where paths meet and by the camp, the landing, the gate and the sign posts; narrower in the forest (counted from the scatter's trunks) and on the heights. It never comes within 0.3 m of a post. The paths' lines didn't move.
  - **The terrain's dirt** comes from the plan (`plan.paint` through a new `paths` argument to `Terrain.meshing`): full under stone, dirt and gravel; two worn bands for tracks; a faint line for stepping stones; uneven edges.
  - **What the scatter clears** is decided before it is placed (`pathClearing`). Task 19's band is reproduced exactly, so every copy stays put, and the trails are added.
- **`Wayside.ts`,** new, lays everything that isn't stone, instanced:
  - stepping stones a stride apart;
  - gravel, with loose pebbles spilling up to 0.45 m onto the grass;
  - log steps: a log across, the earth level behind it, drawn with the ground's own colours and material (a new `Terrain.colorAt`), with sides and front down to the ground;
  - tufts up the tracks' middle, grass reaching in at the edges, a tuft now and then in the path;
  - roots across the forest stretches, each a tube along the ground;
  - moss on the shaded edges;
  - a few stones kicked onto the grass;
  - a puddle in a dip (ice in winter);
  - in autumn, leaves along the edges;
  - in winter, a trodden track down the middle of the snow, with footprints.
- **The StonePath asset** (its preview unchanged, triangle for triangle):
  - A stretch can take a width along it and a `fade` at either end.
  - Where it meets another kind, its stones thin out over 1.8 m, and its fill breaks up ahead of them along a smooth edge (`strip`'s `keep`). So a stone path breaks into single stones on the dirt, then dirt.
  - `Flights` holds stairs for both the stone path and the log steps.
- **Two trails** (`TRAILS`), both possible:
  - **The wreck trail:** 18 m of trodden dirt, 0.8 m wide, from the main path's end by the camp, east of the camp's fence, then west between the wreck's debris to its ramp.
  - **The heights trail:** 25.5 m of stepping stones, 0.7 m wide, from the watch tower's path round the tower's east side to the footbridge, meeting its deck between the rails. No cliff is in the way.
  - **What stood on them** is taken out after placing: trees, bushes and rocks within 1.5 m of a trail's middle, and grass and flowers on its strip. That is 33–77 copies by season, and nothing else moved.
  - **Why 1.5 m:** cleared only to the strip's edge, the walking camera's head went through the boughs of the trees beside the trail.
  - **Rerouted after the first try:** the smoothed line had crossed a piece of the wreck's debris at its bend and the footbridge's rail posts at its end.
- **Walking:**
  - The log steps' treads are ground for the walking camera (`ForestLake.landAt`).
  - The meeting's animals stand up the log steps on a ramp, as on the stairs (`ForestLake.rampAt`, `ground.ts`).
  - Stones, gravel and roots are at most 4 cm over the ground, so they are walked over. Tufts and moss are taller, but soft, as the grass everywhere is.
- **Text:**
  - the forest lake's spec, item 6: each path's kinds, the blends, the widths, what lies along them, the two trails by name, walking;
  - rules.md: Environments rule 27 brought in line, a new rule 28 with what I learned, and Checking the result rule 11;
  - the state files `forest_lake.md`, `forest_lake_meeting.md` (its land changed near the wreck trail in winter) and `index.md`.
  - `StonePath.md` is as task 19 left it.

**How it was checked:**

- **`npm run typecheck`:** clean.
- **In Node, the forest lake against task 19's code** (`src` copied beside it with this task's lines taken out; deleted after), in each season:
  - **Each path's stretches,** with kind and length:

    | Path | Stretches | Steps | Width |
    |---|---|---|---|
    | Main (34 m) | dirty 8.4, regular 8.3, dirt 8.5, regular 8.7 | none | 1.76–2.99 m |
    | East shore (16.8 m) | mossy 8.5, dirt 8.3 | stairs 3.5–4.8 m and 5.3–7.0 m | 1.47–2.23 m |
    | Ruins (15.8 m) | ruined 7.9, gravel 7.9 | logs 9.3–11.3 m | 1.33–2.42 m |
    | Pier (9.5 m) | tracks 9.5 | none | 1.34–2.08 m |
    | Heights (19.8 m) | stepping 10.3, gravel 9.5 | logs 9.8–17.5 m; nothing on the last 1.5 m | 1.26–1.75 m |
    | Way in (31.2 m) | regular 10.2, tracks 10.3, regular 10.7 | none | 2.01–2.73 m |
    | Wreck trail | dirt 18.0 | none | 0.8 m |
    | Heights trail | stepping 25.5 | none | 0.7 m |

  - **Triangles:** the paths and what lies along them are 99,800 in spring, 99,700 in summer, 111,100 in autumn and 61,200 in winter. Task 19's stone paths were 82,000–143,000, so this is 21,000–34,000 fewer: dirt, tracks and stepping stones have no fill or verge.
  - **Time:** their piece of the build takes 20–28 ms (plan 2–4, stone 14–18, wayside 4–6). The whole forest lake builds in 1.45–1.70 s, as before.
  - **Nothing else moved:** no scatter copy moved, and the 33–77 taken out are all on the trails. Every landmark, fence and lamp post is where it was, and the landing is still at 0.4241 m.
  - **The meeting's land** is the same cell for cell in spring, summer and autumn. In winter 73 cells by the wreck trail's start have more room, and 29 more can be walked.
  - **The trails are open:** nothing the meeting goes round is left on either trail's walked strip (there were 14 in spring and 76 in winter before).
  - **Other checks:** 17 log steps, 6 stair steps, 1 puddle, nothing NaN.
  - **The StonePath asset's own output** is the same as task 19's in every variation and season.
- **In headless Edge on the GPU** (RTX 3050, test server on port 8156, 1400 × 900, the Kuwahara filter on):
  - every path and both trails walked with the walking camera, a shot every 10 m or so, in spring and winter, laid out as a strip per path;
  - each kind close up in spring, and dirt, gravel, tracks and stepping stones in autumn;
  - two blends, stone into dirt and stone into tracks, along the path and from above;
  - both trails from above in spring and winter, and the wreck trail's bend close up;
  - the landing both ways in all four seasons.
  - No page errors.
  - **Fixed from the shots:**
    - the log steps' treads had been flat, bright orange boards;
    - the stone's fill had ended square at a blend;
    - the roots stood up to 12 cm over the path's slope;
    - the puddle had read as a dark flat stone.
- **Frame rates,** the forest lake now and task 19's (served from its own page), one after the other, twice each:
  - at the landing, 54.9 and 51.5 frames a second, against 48.7 and 51.0;
  - along the main path, 48.5 and 48.5, against 48.4 and 48.9.

  No difference beyond the noise.
- The before-copy (`src-before-20/`, `before-20.html`) is deleted, the test server stopped by its PID, and the browsers closed.

**Left for you:**

- **The kinds and the trail routes are my defaults.** Which kinds each path is made of (`PATH_KINDS`, one table in `Paths.ts`) and where the trails run are my choices from this task's defaults.
- **The meeting doesn't reach the trails.** The forest lake meeting's land ends 39 m from its middle, so its animals walk only the first 7 m of the wreck trail and none of the heights trail.
- **Low boughs:** a pine standing more than 1.5 m off the heights trail still reaches over it at eye height here and there.
- **One puddle:** only one dip along the paths' middles is deep enough to hold one.
- **Knowledge graph:** tracked docs changed (rules.md, the forest lake's spec, the three state files). Syncing is asked about once the task queue is done.
