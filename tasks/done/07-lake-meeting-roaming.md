# Lake meeting: the animals roam the map

The user asked (2026-09-25): "in lake meeting story, make animal walk randomly at the map". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

The story is in `src/story/lake_meeting/`. Read `lake_meeting.md` (a draft spec written by Claude, not yet confirmed by the user), `Meeting.ts` and `Member.ts`, and rules.md (Stories, Animals, Environments).

Today the viewers' animals and the bots stand in two rows facing each other down an aisle on the landing, with the host bear at its head. Bots step out into the aisle and back now and then. `Member` already walks a path of points on the uneven ground, leaning with its slope.

## What changes

- **Every animal but the host roams the lake's land (default):** viewers' animals and bots alike.
  - It walks to a random spot, stands there a while (3–15 s), sometimes jumps, then picks another.
  - Now and then it runs instead.
  - The rows and the aisle are no longer used.
- **The host bear stays at the landing (default),** where it is now. The spec says it is always at the meeting.
- **Across every part of the lake's land.** The user asked again, before this task was taken: "roaming animal make wider, accross all lake section".
  - They go to the near shore by the landing, both sides of the lake, the far shore across the water, and up among the hills and trees, out to the land's end.
  - Pick spots evenly by area over all of that land, so that at any moment the animals are scattered all round the lake, not bunched near the landing.
  - Now and then, send one on a long walk to another part of the lake.
- **Round the lake, never across it:** the water is in the middle, so a straight line to the far shore goes into it. Walk round on the land instead, for example by a ring of points following the shore a few meters in from the water, joined to the start and the end.
- **Where they may go:**
  - on land, never into the water; keep them above the waterline with a margin;
  - short of the land's end and its border (`Environment.ground.reach`);
  - off ground that is too steep.
- **What they must not walk through:**
  - tree trunks, boulders (`boulders.near()`) and the big stones;
  - each other.

  Ask the lake for what stands where, rather than copying positions. Task 04 (mud pits, in progress) and task 05 (more stones) are changing the lake's land, and whatever they add should be avoided whenever it arrives. Mud pits may be walked through.
- **New animals appear at a random spot on the map.** The user asked, before this task was taken: "in lake meeting, animal spawn randowmly in map".
  - A viewer's animal, and a bot that comes back, appears at a spot picked like a roaming destination: anywhere on the lake's land, evenly by area, off steep ground, and clear of trunks, boulders, big stones and the other animals.
  - **It appears with a jump (default),** its own `Jump()`, as if it had just landed there, then starts to roam.
  - The camera's turn for its first message goes there, as for any other turn.
  - The host stays at the landing, and one that leaves walks away as now.
- **Talking:** when its turn comes, the camera goes to the animal wherever it is, as now (`READABLE`, `LEAD`). **It stops walking while its bubble shows (default),** so the words stay steady, then goes on roaming.
- **Commands (default):**
  - `!walk` walks to a new random spot, and `!run` runs to one;
  - `!stop` stops it where it is, and it starts roaming again after a while;
  - `!jump` and `!flap` are as now.
- **When the chat is quiet:** after a few seconds the camera goes back to the host's view at the landing, as now (spec item 10). Task 08 adds following animals from a few angles after 30 s without a comment: leave that to it.
- **Shadows:** the sun's shadow now covers only the meeting's `bounds` round the aisle (Checking the result rules 5 and 6), so an animal that walks off loses its shadow. Make the shadow follow what the camera is looking at, at about the size it has now, rather than stretching it over the whole map, which would blur every shadow.
- **The spec:** update the draft `lake_meeting.md` to match: item 3 (a new animal appears at a random spot rather than walking in), item 5 (commands) and item 11 (bots). Keep its note at the top that it is a draft for the user to correct.

Other sessions are changing the lake's files, `src/previews.ts` and `src/Stage.ts`. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- **Numeric checks,** over a long run in Node (30 min of the meeting, with 7 bots and a few viewers from test messages):
  - no animal is ever in the water, in a trunk, boulder or big stone, past the land's reach, or inside another animal;
  - over many arrivals, new animals appear all round the lake, never in the water, in an obstacle or in another animal;
  - planted feet don't slide (Animals rule 4);
  - nothing is NaN.
- **Screenshots** with headless Edge on a spare port, never 8087 (rules.md, Checking the result), driving the story with the Story tab's test messages (spec item 9):
  - a comment from an animal far from the landing: the camera goes there and the bubble reads;
  - a command.

  Take them with the Kuwahara filter on, by day and at night (`?time=night`). An animal far out must keep its shadow.
- **The frame rate** with 12 animals and the host at the lake, compared with now.
- Add what you learned to rules.md (Stories), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 18:36

## Done

**What changed**

- `src/story/lake_meeting/Land.ts` (new): the lake's land as the animals see it.
  - A 0.5 m grid of clearance to the water, the wet bank, steep ground, the border (7 m in from the land's reach) and `Lake.obstacles()`.
  - Spots picked evenly by area, anywhere or within 12 m.
  - A* ways round the lake, never across it, then straightened.
  - `nearestFree()` and `edgeToward()`.
- `src/story/lake_meeting/Roam.ts` (new): the roaming.
  - Each animal rests 3–15 s (sometimes jumping), then walks (20% runs) to a near spot or, 15% of the time, anywhere on the land.
  - Keeping apart: see rules.md Stories rule 17. Nothing ever pushes an animal.
  - `hold`/`release` while talking; `!stop`, `!walk`, `!run`.
  - Appearing at a random spot with a jump.
  - Leaving: walking to the land's end.
  - A switch to a bigger kind appears at the nearest spot where it fits.
- `src/story/lake_meeting/Meeting.ts`:
  - The rows and aisle are gone; the host stays at the landing.
  - The camera goes to an animal wherever it is. It comes from the first side with a clear line of sight (trunks, boulders, the ground, other animals) and stays above the ground.
  - It cuts to an animal more than 12 m away, and follows a walking or running animal from beside it.
- `src/story/lake_meeting/Member.ts`:
  - `radius` and `width`, and `stop(settle)`.
  - It turns close to a corner rather than cutting it.
  - A runner walks round sharp corners and while it faces away from its next point.
- `src/environtments/Lake/Lake.ts`: `obstacles()`, which measures each tree's wood near the ground from its geometry (a trunk's flare, or a spruce's low branches).
- `src/previews.ts` `Preview.followShadow` and `src/Stage.ts` `moveShadow()`: the shadow follows the camera's target at the size of `bounds`, in whole texels.
- `lake_meeting.md` items 2–5, 11 and a new 12 (the draft note kept), and rules.md Stories rules 11, 13 and a new 17.

**How it was checked**

- `npm run typecheck`.
- **Numerically in Node on the real modules**, twelve runs of 30 min (7 bots, 5 viewers, commands, comings, goings, switches), every frame:
  - Never in the water: at least 0.12 m above it.
  - Never in a trunk, boulder or big stone: at least 0.38 m clear.
  - Never past the reach, never NaN.
  - Never moved sideways.
  - Animals: eleven runs had no two footprints overlapping. One had 7 cm of footprint overlap (a wolf passing a snake just arriving to rest), with the bodies 0.31 m apart; bodies never touched.
  - New animals appeared all round the lake, none in the water.
  - Earlier versions failed these checks; what fixed each is in rule 17.
- **Headless Edge on the graphics card**, Kuwahara on, by day and at night:
  - A new viewer's comment: the camera cut to their animal far out (a penguin by the cliffs, a robin on a hill at night), with a readable bubble and its own shadow.
  - `!run` followed from beside the runner.
  - No page errors.
- **Frame rate** with twelve viewers and the host: 60 (the cap), against 37 before, measured the same way, both while the user's stream was also drawing.

**Left for the user**

- Pages each place the animals that were already there at their own spots, so the panel's page and the stream's hidden page show them in different places. Only the stream matters for viewers.
- The wide view at the landing shows the host and few animals now that they spread out. Task 08 (following animals when the chat is quiet) is next in the queue.
- Nothing is committed.
