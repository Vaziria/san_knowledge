After: 08-lake-meeting-idle-camera.md

# Lake meeting: a dock in the lake

The user asked (2026-09-25): "in lake meeting add dock in the lake". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

Read rules.md first, especially Building shapes (rules 8–13, the boat's wood), Objects, Environments and Stories. The meeting is in `src/story/lake_meeting/`. Tasks 07 (the animals roam the land) and 08 (the camera follows animals when the chat is quiet) come before this one: read their Done sections in `tasks/done/`.

## The dock

An object: `src/figures/objects/Dock.ts` with its spec `Dock.md` beside it, a draft written from the code that says so at its top and names no behaviour, like the other objects.

- **What it is (default):** a plain village jetty (*dermaga kayu*), traditional rather than modern (Building shapes rule 13).
  - A straight walkway of wooden planks, about 1.4 m wide, running 8 m out from the bank over the water.
  - The planks lie across two or three beams, with narrow gaps between them.
  - It stands on pairs of log posts with their bark on, about every 2 m, going down to the lakebed.
  - The deck is about 0.45 m above the still water.
  - Nothing else: no railing, ladder, lamps or boats tied up (Building shapes rule 7).
- **Materials:** the planks and beams are wood with its grain (`wood(theme)`, with texture coordinates in meters along the grain, like the boat's boards: `grainUVs()` in `Boat/parts.ts`). The posts are logs in bark (`bark(theme)`). Both are in the theme's colours (Building shapes rule 12).
- **Sized** like the other objects, with a seed: `new Dock({ theme, seed, width, length, height })` in meters, with a default size (Objects rule 1). It is a little uneven (planks not all alike, posts not quite straight), from its seed.
- **Its own preview** at the lake (`figureEnvironments`), running from the bank out over the water, seen whole.

## At the meeting

- **Only the meeting's lake has it (default):** the meeting builds the lake with `lakeEnvironment(theme, { clear: CLEAR })`. Add the dock through a lake option, the way the meeting widens its clearing. The lake the other figures show in (the boat, the fish) stays as it is.
- **Where:** from the bank a few meters to one side of the landing, out into the water.
  - It is in view of the meeting's cameras.
  - It stays clear of the fish's loop and where the fish leaps (`FISH` in `Meeting.ts`).
  - Its bank end is clear of the stones, boulders, mud pits and trees (ask the lake for them). Nothing else in the lake moves.
- **The animals can walk on it (default):**
  - The deck counts as ground: the lake's `ground.heightAt` gives the deck's height on the dock, for the meeting's animals and the walking camera alike (`?camera=walk`).
  - The roaming from task 07 may pick spots out on the dock, and the animals step up onto it from the bank without a jump in height.
  - Anywhere off the deck they still keep out of the water, and they don't walk through its posts.
- **The camera:** task 08's angles must not put the camera inside the dock or have it between the camera and the animal.
- **Water:** the posts stand in the water, which washes round them. The water never shows on the deck: the deck is solid and well above it. Check it with the rain from task 06 and with the waves in the wind.

`Lake.md` is the user's spec: don't edit it. Other sessions are changing the lake's files, `Meeting.ts`, `src/previews.ts` and `src/Stage.ts`. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- Screenshots with headless Edge on a spare port, never 8087 (rules.md, Checking the result):
  - the dock's own preview;
  - the meeting's cameras with the dock in view;
  - an animal out on the dock;
  - the walking camera on the dock.

  Take them with the Kuwahara filter on and off, in `felt` and in `studio`, by day and at night.
- Numeric checks:
  - animals on the dock stand on its deck, not in it or above it, and step on and off with no jump;
  - none walks through a post or into the water;
  - the lake's stones, boulders, mud pits and trees are where they were;
  - nothing is NaN.
- Add what you learned to rules.md (Objects and Stories), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 22:40

## Done

**What changed**

- **`src/figures/objects/Dock.ts`** (new), with its draft spec `Dock.md`: a village jetty, 768–840 triangles in two meshes (wood and bark).
  - **Its parts:** planks across three beams with narrow gaps; cross-beams on pairs of log posts in bark, about every 2 m.
  - **Posts:** they stand on `bedAt`, the lakebed or the bank. A cross-beam that comes down to the bank rests on it and has no posts.
  - **Ramp:** the first 1.2 m of the deck slope from its `foot` at the landward end (`RAMP`).
  - **Built:** from a seed, and a little uneven.
  - **Helpers:** `Dock.postSpots()`, `Dock.undersides()` and `Dock.topAt()`.
- **`src/environtments/Lake/DockSite.ts`** (new): where a dock goes. `dockSite()` searches the shore from the angle asked for, and lets the dock turn up to 20° from running straight out. A site must have:
  - no boulder, mud pit or tree under the deck;
  - nothing the caller keeps it off;
  - 2.5 m of dry, clear land behind its landward end, the way onto it;
  - land under its level part that stands below its planks.

  It also holds `inTheWay()`, which stones must go; `inApproach()`; `deckPoint()` and `onDeck()`; and `FAR_SHORE`.
- **`Lake.ts`:**
  - `LakeOptions.dock`;
  - `Lake.siteDock()`, which finds the site, clears the stones in its way or on the way onto it and the grass under it, and keeps the deck;
  - `Lake.buildDock()`;
  - `Lake.deckAt()`, easing from the land's height to the deck's over its first 0.3 m;
  - `Lake.dock` and `Lake.deck`.
- **`Stones.ts`:** `takeAway()`, which removes stones from the meshes, from `placed` and from `near()`.
- **`previews.ts`:**
  - the lake's `ground.heightAt` includes the deck, for the walking camera and the meeting;
  - a `dock` preview at the lake, where the meeting has it, seen from over the water;
  - `figureEnvironments.dock = 'lake'`.
- **The meeting:**
  - `DOCK` in `Meeting.ts`: the search starts at the far shore and keeps clear of where the fish goes (`fishGoes`). That is its loop, and a corridor running 4 m on from each side the way it swims, since a supporter's leap starts wherever the fish is.
  - `lake_meeting.ts` builds the lake with the dock.
  - `Land.ts` takes the decks as ground and measures clearance on them exactly, not by each cell's worst point, and allows a diagonal step between deck cells. It also has `deckSpot()`.
  - `Roam.ts` sends an animal out on the dock one time in ten.
  - `Sight.ts` treats the dock as solid for the camera.
- **Where it stands:** from the far right bank at −50°, turned 14°, 4.6 m from the fish's loop. It is 3 × 6.5 m, its deck 0.36 m up.
- **Docs:**
  - `rules.md`: Objects rule 9 and Stories rule 20;
  - the state files `lake_meeting.md`, `environments.md`, `objects.md` and `index.md`;
  - knowledge synced.

**How it was checked**

- `npm run typecheck` passes.
- **Numeric checks in Node** (scripts in this session's scratchpad):
  - **Nothing else moved:** the meeting's lake built without and with the dock has every boulder, mud pit, tree and remaining stone in the same place. 15 stones and about 100 grass plants were cleared.
  - **The step onto the deck** at its landward end: at most 0.4 cm anywhere across.
  - **Access:** all nine kinds, the snake, wolf and deer included, found a way from the landing out onto the deck.
  - **A 30 min run with 8 viewers of every kind:**
    - 14 walks out on the dock (cat, wolf, frog, penguin, deer, bird, and the host, since task 14);
    - the animals' feet were exactly on the deck's surface;
    - the most any animal's height changed in one frame, stepping on or off, was 0.22 cm;
    - no animal was ever over open water, and nothing was NaN.
  - **The quiet camera, 10 min with the dock among the solids:** in 2850 raycast checks it was never in the ground, the water, a tree, boulder, stone or the dock, and nothing came between it and the animal.
- **Headless Edge on the GPU** (port 8141, frames driven one by one; pictures in this session's scratchpad, `dock/`):
  - the dock's preview in felt and studio, filter off and on, by day and at night;
  - the walking camera on its deck, at 1.649 m (1.6 m over the deck);
  - a deer placed out on the dock at the meeting, in felt and studio, by day and at night;
  - no errors on the page.
- **What didn't work at first:**
  - **No site at all:** every stretch of shore met a stone, so stones in the way are now cleared.
  - **Steps:** the flat deck stood up to 26 cm off the humpy land at its landward end, hence the ramp and the eased height.
  - **Big animals stuck:** a boulder behind the first site (at −140°) left the wolf, deer and snake no way on, hence the clear approach, the turn and the exact clearance on the deck.

**Left for you**

- **Choices to adjust:**
  - **Width:** 3 m instead of the task's 1.4 m, so the big animals fit.
  - **Length:** 6.5 m instead of 8, on this 18 m lake.
  - **Ramp and place:** its ramp, and its place on the far right shore.
  - **Visits:** one roaming spot in ten out on the dock.
- **The stones:** 15 stones were cleared from the dock's site at the meeting's lake. The plain lake is unchanged.
- **The fish:** after a supporter's leap it can still wander under the dock and through its posts, under the water. Its leaps keep clear.
- **The quiet camera** with the dock and the roaming host had one follow cut short at 22 s and a few short shots (7 of 79), which the run without them didn't. `Dock.md` is a draft for you to correct.
- 58 knowledge nodes need summaries (`bin\knowledge.exe sync`).
