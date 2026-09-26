After: 04-mud-pit.md

# More stones along the lakeside

The user asked (2026-09-25): "add more stone in sidelake". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

The lakeside stones are in `src/environtments/Lake/Stones.ts` (rules.md, Environments rule 11): about 75 rocks in 16 clumps around the shore, from pebbles to 0.7 m, some up the bank and some with their feet in the water. Six shared shapes are drawn by one seeded generator, and each stone is its own mesh. Read rules.md first, especially Environments.

## What changes

- **About three times as many stones (default),** around 220, so the change shows from the lake's cameras:
  - more clumps all round the shore, and a few more big stones (0.5–0.7 m) where the preview cameras look;
  - loose pebbles and small stones scattered along the waterline between the clumps, so the shore reads as stony all the way round;
  - as now, some up the bank and some at the water's edge with their feet in the water, each bedded into the ground.
- **The 75 stones there now keep their spots (default).** Add the new ones after them, from the same generator or a seeded one of their own, and check in numbers that the old ones haven't moved.
- **Keep clear of:**
  - `Lake.LANDING`'s clearing (`lake.clearing`, which the lake meeting widens with `LakeOptions.clear`), with the margin the stones keep now;
  - the four boulders (`boulders.near()`);
  - the tree trunks;
  - the mud pits (task 04).

  Nothing else in the lake may move: the trees, boulders and mud pits keep their spots.
- **Grass mustn't grow up through them.** The sward and the meadow keep off the boulders (Environments rules 16 and 18). Check whether they need to keep off the stones too, now there are many more.
- **Keep it cheap:**
  - With every stone its own mesh, three times as many stones is three times as many draw calls. Draw them as one `InstancedMesh` per shape instead: 6 draw calls however many stones there are. The stage already frees instanced meshes (Environments rule 16).
  - Measure the lake's build time (about 0.62 s, rule 13) and its frame rate, before and after.
- **Look the same:** the colour stays `theme.scene.stone`, and the stones keep the flat faces they have now. Check them at night too (task 02).

`Lake.md` is the user's spec: don't edit it. Other sessions are changing the lake's files. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- Screenshots with headless Edge on a spare port, never 8087 (rules.md, Checking the result):
  - the lake from its preview camera (the penguin's) and from the boat's, before and after;
  - the shore close up with the walking camera (`?camera=walk`);
  - with the Kuwahara filter on and off, in `felt` and in `studio`, by day and at night.
- Numeric checks:
  - the old 75 stones, the trees, the boulders and the mud pits are where they were;
  - no stone lies in the clearing, on a boulder, at a trunk or in a mud pit;
  - the build time and draw calls are measured before and after.
- Update Environments rule 11 in rules.md, then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 18:07

## Done

**What changed**

- **`src/environtments/Lake/Stones.ts`:**
  - **The first 75 stones** are planned in the constructor with the same random numbers in the same order, and are kept in `placed`.
  - **`scatter(blocked)` adds 142 more** from a generator of its own (seed 71), for 217 in all:
    - 14 more clumps;
    - 90 loose pebbles and small stones along the waterline;
    - 6 big stones, 0.5–0.7 m across, where the preview cameras look: the near bank either side of the landing and the far shore.
  - **Keeping clear:** the new stones keep off the clearing (same margin as the old ones), the boulders, the trunks and the mud pits. Stones are still bedded into the ground, with the same colour and flat faces.
  - **Instanced:** all stones are drawn as one `InstancedMesh` per shape, so 6 draw calls in all.
  - **New API:** `spots` and a grid-bucketed `near()`.
- **`Lake.ts`:**
  - **Build order:** old stones, then boulders, trees and mud pits (placed against the old stones only, via `stones.spots`, so they don't move), then `stones.scatter(...)`, then the grass.
  - **Grass:** it now keeps off the stones too, along with the boulders and pits.
  - **`MudPits`** takes the stones' spots instead of their meshes.
- **Docs:**
  - `rules.md` Environments rule 11 extended.
  - Checking the result, rule 7: print the renderer when measuring a frame rate.
  - Knowledge synced.

**How it was checked**

- `npm run typecheck` passes.
- **Numeric** (Node):
  - The old 75 stones' matrices are identical (largest difference 0). The trees, boulders and mud pits are unchanged.
  - 217 stones in the default lake and 197 in the meeting's wider clearing. Of these, 20 and 25 are 0.5 m across or more.
  - 0 stones lie in the clearing, on a boulder, at a trunk or in a mud pit.
  - Grass growing up through a stone over 20 cm across: 33 tufts before the grass kept off the stones, 11 after.
- **Draw calls** (browser):
  - The penguin's lake view went from 187 to 140, and the boat's from 111 to 83.
- **Build time:** 0.89–1.02 s before, 0.81–1.03 s after, both measured while other sessions loaded the machine. Checking the stones by a 1 m grid removed a 90 ms cost of the grass asking all 217 stones at every spot.
- **Frame rate** (penguin at the lake, filter on):
  - GPU: 60 frames a second by day and by night.
  - SwiftShader: 0.5 by day and by night, against 0.6 in task 02, while the machine was busier.
- **Screenshots** (headless Edge, port 8131):
  - Before and after, from the penguin's and the boat's views: more stones along both shores.
  - The walking camera on the near bank.
  - felt and studio, by day and at night, with the filter off and on: the stones look as before, including in studio at night.

**Left for you**

- The number and places of the new stones are my choices: 14 clumps, 90 pebbles, and the 6 big ones' spots.
- Stones are now also something the sward and meadow keep off, so some grass near the shore moved. The trees, boulders and pits did not move.
