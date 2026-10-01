# Forest lake meeting: the season changes every minute

The user asked (2026-10-01): "add task, @typescripts/animation/src/story/forest_lake_meeting/scenarios/season_change.md". That scenario file says: "every 1 minute, season in forest lake meeting changes". Those lines are the user's; the rest of this file was written by Claude from them. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

The forest lake meeting is `src/story/forest_lake_meeting/` in `typescripts/animation`. Read its draft spec `forest_lake_meeting.md` (item 2 is about the season) and `ForestMeeting.ts`, the state files `docs/states/forest_lake_meeting.md` and `docs/states/forest_lake.md`, and `docs/3d_modelling/rules.md` (Stories, Checking the result). The scenario file is the user's: leave it as they wrote it.

Today the season is set in the Environments tab (`?season=`, spring by default) and stays put. Picking another one starts everything over: `Stage.show()` (`src/Stage.ts`) rebuilds the environment and the meeting together. The host goes back to the landing and the others appear at new spots. It also blocks the page for about 2.4 s: some 1.6–1.7 s to build the forest lake, and 0.8 s for the meeting to measure its land (`ground.ts`) and its cameras' sight (`ForestSight.ts`) from the new meshes.

## What changes

- **Every minute the next season comes:** spring, summer, autumn, winter, then spring again.
  - **(default)** The minute is the meeting's own time, counted in its `update(delta)`. A page left in the background doesn't catch up with several changes when it comes back.
  - **(default)** It starts from the season the Season setting picks. The first change comes a minute after the meeting has started.
- **Only in the meeting (default).** The forest lake shown with a figure, or on its own, keeps the season you pick, as now.
- **The meeting carries on through a change:**
  - Every animal stays where it is, facing the same way, doing what it was doing. The host is not sent back to the landing.
  - The bubbles, the queue of turns, the camera and its shot, the bots and the viewers' animals all carry on.
  - Its land, its sight and the fish's water are those of the new season from then on. Winter has bare trees among the pines, and they shift where other things stand, so the new season's footprints differ.
  - **(default)** An animal the new season puts inside something, or over water, steps out to the nearest clear spot. It walks there; it doesn't jump. Count in the checks how often this happens.
  - **(default)** A fish's leap under way lands first, and the change waits for it: at most a few seconds. In winter the open water is only the hole in the ice.
- **Coats follow the season (default).** An animal whose sheet has a coat for the new season changes into the colour it would have been given in that season (`makeGuest` in `guests.ts`, the same pick from its id). It changes where it stands, without the jump a `!wolf` switch makes. An animal whose colour stays the same keeps its figure.
- **No freeze on the stream (default).**
  - Build the next season ahead, during the minute before its change, in pieces between frames. That covers the forest lake, and the meeting's land and sight measured from it. Compile its shaders before it is shown (for instance with `renderer.compileAsync`), then swap it in ready.
  - **The target:** no frame, at the change or in the minute before it, takes over 100 ms.
  - If a piece can't get under 100 ms without rewriting how the forest lake builds, keep the best you reach and don't rewrite the builders in this task. Say in Done where the time goes, with the numbers.
  - Keep at most two seasons built, the one shown and the next, and dispose of the old one's geometries, materials and textures after the swap.
- **The change is a cut (default).** At the minute, the scenery, the sky, the lights (`Environment.light`) and the fog swap in one frame. Describe in Done how it looks, so the user can ask for a fade or for the swap to wait for a camera cut.
- **The Season setting still picks:**
  - It picks the season the meeting starts in. Picked while the meeting plays, that season comes at once, through the same no-restart swap, and the next minute counts from there.
  - `?season=` is not rewritten every minute (default).
  - The page the dev server streams from (`?renderer=`, `streamRenderer.ts`) follows the panel's settings. Check that it changes season once a minute on its own clock, not twice, and not back to the panel's setting.
  - The Story tab's hint for the story (`Panel.tsx`, "in the Season picked in Environments") says it starts in that season and changes every minute.
- **`Stage` needs a way to swap an environment under a figure it keeps.** `show()` only rebuilds both. The swap brings the new scenery, placed as `show()` places it, with its light, fog and season, and tells the meeting so it can take the new land, sight, water and coats.
- **The spec:** in the draft `forest_lake_meeting.md`, item 2 says the season changes every minute, links the scenario file, and says the meeting carries on through the change. Keep its note at the top that it is a draft for the user to correct. Bring the comments in `forest_lake_meeting.ts` and `ForestMeeting.ts` in line.

Other sessions may be changing the forest lake's files (`src/environtments/ForestLake/`), `src/previews.ts`, `src/Stage.ts` and the lake meeting's modules. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- **Numeric checks,** in Node over 10 minutes of the meeting (ten changes), with 7 bots and some viewers from test messages:
  - it changes every 60 s of the meeting's time, in order, starting from the picked season;
  - across each change, no animal moves further than a normal frame's step, and the host is not put back on the landing;
  - afterwards none stands inside anything or over water for longer than it takes to step out. Say how many had to step out at each change;
  - the members, bubbles and turns waiting are the same just before and just after;
  - after ten changes the counts of geometries, materials and textures are back where they were after the first. Nothing grows;
  - nothing is NaN.
- **In headless Edge on the GPU**, on a spare port, never 8087 (rules.md, Checking the result; close the browser afterwards):
  - At 1280×720 with the Kuwahara filter on, record every frame's time over three minutes spanning three changes: the longest frame, and the frames over 50 ms, before and at each change.
  - Take screenshots just before and just after each of the four changes, with a comment's turn under way at one of them.
  - Pick a season in the Environments tab while it plays.
  - Watch the `?renderer=` page through at least two changes.
  - No page errors.
- Bring rules.md (Stories) in line, and add what you learned.
- Update the state files `docs/states/forest_lake_meeting.md`, `forest_lake.md` (its open question "Should the season also be able to turn on its own") and `index.md` (rules.md, Project rule 8).
- Then ask the user before syncing the knowledge graph (CLAUDE.md, Research workflow).
- Don't commit.

Taken: 2026-10-01 00:30

## Done

Finished 2026-10-01. Not committed.

**What changed** (all in `typescripts/animation`):

- **The season changes every minute of the meeting's own time:** spring, summer, autumn, winter, then spring again, from the Season picked. The minute is counted in the meeting's `update`, so a page left in the background doesn't catch up.
  - Code: `SEASON_TIME`, `nextSeason`, `seasonDue`, `pick`, `prepare` and `take` in `ForestMeeting.ts`, and `changes` in `forest_lake_meeting.ts`.
  - The change waits while the fish is in the air: 0.6–0.7 s, once or twice in ten minutes.
  - Only the meeting's season turns. The forest lake shown with a figure keeps the one picked, and `?season=` isn't rewritten.
- **The stage swaps an environment under a figure it keeps** (`Stage.ts`; `Preview.changes` / `EnvironmentChanges` in `previews.ts`):
  - It builds the next one ahead after each frame: 8 ms a frame, or 40 ms once it is due. It rests 0.7 s after a long piece and 2.5 s after a swap.
  - The story measures what it needs from it (`prepare`).
  - The shaders are compiled with `compileAsync`. The scenery shown is hidden meanwhile, so its fires don't count twice among the lights. With the filter on, they compile for the composer's target.
  - At the start of a frame, once due, it swaps it in, placed, fogged and lit as `show()` does (`useEnvironment` and `placeScenery` came out of `show()`).
  - It frees what the old season used and nothing shown uses, textures too (`disposeUnused`). At most two seasons exist, the shown and the next.
  - `show()` with only a new environment asks the story first (`pick`). That is how a season picked while it plays comes at once, with the next minute counting from there.
- **Building a piece at a time:** `src/stepwise.ts` (`Stepwise`, `finish`) and:
  - `ForestLake.build()`: the constructor's body moved into `building()`, which yields between modules;
  - `Terrain.sample()` and `Terrain.meshing()`: heights, ground mesh and baked light, eight rows at a time;
  - `forestGroundBuild()`, `Land.build()`, and `ForestSight.build()` with `triangles()`.

  `new ForestLake()`, `new Terrain()`, `mesh()`, `forestGround()`, `new Land()` and `new ForestSight()` still build everything at once, as before.
- **The meeting carries on through a change** (`take`):
  - The land, the sight and the fish's water and splash are the new season's. The cameras reach the sight through `seeing`, so `Follow` and `HostView` carry on.
  - **`Roam.reland()`:** every animal stays where it is, doing what it did.
    - One the new season puts inside something or over water walks out to the nearest spot where it fits.
    - One on its way finds a new way if its own is no longer clear.
    - Stepping out uses the same room a roaming spot keeps (`ROOM`) and doesn't stop for others. A hold that comes meanwhile (a talking turn, `!stop`) takes effect once it is out.
- **Coats** (`coatOf` in `guests.ts`): an animal whose colour differs in the new season is repainted where it stands with the figure's own `SetColor()`, so it keeps its pose, stride and bubble. Only wolves, deer and boars have seasonal coats.
  - **Changed:** an animal's plain colour is now the same in every season. Before, an odd pick in autumn or winter ran over a list that included the season's coat, so a brown wolf could turn black in winter.
- **Fixed: the frog hops level** (`guests.ts`). Leaned with the slope while it hopped, a foot on the uphill side never came down. On a slope of just 0.1 across its way it stood mid-hop for good, in ordinary roaming too. The meeting still leans it after it hops.
- **Text:** the spec's item 2 (and item 4 on coats), the Story tab's hint ("starts in the Season picked in Environments; a new season every minute"), and the comments in `ForestMeeting.ts`, `forest_lake_meeting.ts`, `guests.ts`, `Stage.ts` and `ForestSight.ts`. rules.md has Stories rule 23 and Checking the result rule 9. The state files are updated: `forest_lake_meeting.md`, `forest_lake.md` (its open question answered for the meeting), `control_panel.md`, `lake_meeting.md` and `index.md`.

**How it was checked:**

- `npm run typecheck`: clean.
- **Node,** on the real modules, with the stage's swap done as `swapIfDue` does it. Ten minutes from spring and from winter, with 7 bots, 7 test viewers, a supporter every 45–75 s, and three comments and a `!howl` just before each change:
  - ten changes in order, every 60.03 s, once 60.63 s (waiting for the fish);
  - no animal moved at any swap, and the host was never sent back to the landing;
  - members, queue, current turn and bubbles identical before and after;
  - 0–5 stepped out a change, and none stood inside something longer than 1.8 s in one run and 7.4 s in the other (an otter walking out after winter came);
  - the geometries, materials and textures in use came back to the same counts for each season on every cycle;
  - nothing NaN.

  Two runs before the fixes found a fox left inside a tree for a minute, a squirrel stepping into a cell with no room, a deer held half inside a trunk, and the frog stuck on a slope.
- **Headless Edge on the GPU** (RTX 3050, 1280×720, Kuwahara on, test server on port 8156, browsers closed after):
  - **Frame times over 200 s and three changes:** 59.5 frames a second, median 16.7 ms, 99th percentile 16.9 ms, swap frames at most 33 ms.
    - 7 frames over 50 ms: the landmarks (267–283 ms) about 3 s after each change, the scatter's placement (150–167 ms) a second later, and one of 83 ms while the first season ahead was built.
    - No other frame took over 34 ms.
  - **The renderer page** (`?renderer=1280x720`; its `/__stream/*` posts answered in the test browser, since no stream was running) through four changes, with screenshots just before and just after each: spring to summer, summer to autumn with a viewer's comment in its bubble across the change, autumn to winter, winter to spring.
  - Given the panel's settings with another setting changed, the renderer page kept its own season.
  - **Autumn picked in the Environments tab** while both pages played spring: the URL wrote `?season=autumn`, and the panel page was in autumn 2.7 s later. The renderer page, given the new settings, followed 3.8 s after the pick. Each counted its next minute from there, and neither went back.
  - No page errors.
- **Screenshots** are in this session's scratchpad (`shot-*.png`).

**Where the time goes, and what is left for you:**

- **The 100 ms target is missed by two pieces:**
  - **Landmarks:** about 0.27 s in the browser (0.3–0.5 s in Node). It builds some 30 landmarks, the wreck among them, and merges them in one constructor.
  - **The scatter's placement:** about 0.16 s (0.2–0.4 s in Node).

  Everything else comes in slices under 25 ms:
  - the terrain (0.23 s in one piece before);
  - the ground mesh (0.6 s before);
  - the water, the cliffs and the backdrop (5–30 ms);
  - the meeting's land (0.6 s in Node before) and sight.

  Bringing those two under 100 ms means making their constructors stop between parts, which this task said not to do. Say if you want it.
- **How the change looks:** a cut. The scenery, sky, lights and fog change in one frame, the camera stays where it was, and the animals stand where they were, some changing coat.
  - Spring to summer turns the cherry blossom green and the pink water lilies white.
  - Autumn brings gold grass and fallen leaves.
  - Winter covers everything in snow and adds bare trees, which is when animals have to step out.

  Ask for a fade, or for the change to wait for a camera cut, if you want either.
- **A season picked while it plays** takes 2.7–3.8 s to build, at 40 ms a frame plus the two long pieces. The stream runs at about half its frame rate for those seconds.
- **The renderer page was checked with its posts answered by the test browser,** not with a real stream to YouTube. It was handed the settings the way `stream:follow` does.
- **The coats follow the pick from each viewer's id,** so only about half the wolves and boars go white in winter. Say if every one should.
- Other sessions changed the forest lake (the wreck, grass) during this task. My changes to its files were limited to `ForestLake.ts` (the constructor into `building()`) and `Terrain.ts` (`sample()`, `meshing()`, and `bakeLight` made a generator).
