After: 09-lake-meeting-dock.md

# Lake meeting: logs floating in the lake

The user asked (2026-09-25): "in lake metting, add random log in lake". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

Read rules.md first, especially Environments rules 4 (floating) and 5 (keeping water out), and Objects. The log already exists: `src/figures/objects/Log.ts`, with its draft spec `Log.md`. It is a felled trunk with sawn ends and bark, bedded into the ground and cut flat underneath. The meeting is in `src/story/lake_meeting/`. Task 09 put a dock in the meeting's lake: read its Done section in `tasks/done/`.

## What changes

- **In the water (default):** the user calls the shore "sidelake" and the land "the lake environment", so "in lake" here means in the water. The logs float.
- **About 4 logs (default),** each a `Log` of its own seed and size, 1.5–3 m long and 0.25–0.5 m thick. Their places and sizes come from a seeded generator, so the meeting is the same on every load.
- **Only the meeting's lake has them (default),** added through a lake option as the dock is. The lake the other figures show in stays as it is.
- **They float,** as the boat does (Environments rule 4):
  - A log sits a little under half its thickness down in the water.
  - It rides the waves: sample the water's height along it and at its sides, and ease its height, pitch and roll toward them.
  - It rocks more when the wind roughens the water (`Water.setWind`).
- **The log's underside:** it is bedded and cut flat for lying on the ground, and that flat face would show through the see-through water. Give `Log` an option that leaves it round underneath. A log on land is built as now: its preview must not change.
- **They drift, slowly (default):** a few centimeters a second downwind (`src/environtments/wind.ts`), turning a little as they go.
  - Each stays in its own stretch of water. It keeps off the shore by a margin: no log beaches or pokes into the bank.
  - They keep clear of:
    - the dock and its posts;
    - the boulder standing in the water off the west shore;
    - the fish's loop and where it leaps (`FISH` in `Meeting.ts`);
    - each other.
  - Some drift where the meeting's cameras see them.
- A log is solid, not hollow, so nothing has to keep water out of it (Environments rule 5 is for hulls). The rain's rings (task 06) fall round them as on any water.

Other sessions are changing the lake's files, the meeting's files, `src/previews.ts` and `src/Stage.ts`. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- **Numeric checks,** in Node over a long run (30 min of the meeting, with wind):
  - every log stays in the water, off the shore, clear of the dock, the boulder, the fish's loop and the other logs;
  - a log floats at its depth, riding the waves with no jumps;
  - the log's own preview is exactly as before;
  - nothing is NaN.
- **Screenshots** with headless Edge on a spare port, never 8087 (rules.md, Checking the result):
  - the meeting's cameras with logs in view, in calm and in wind;
  - a log close up at the waterline, which shows how deep it sits;

  Take them with the Kuwahara filter on and off, in `felt` and in `studio`, by day and at night.
- Add what you learned to rules.md (Objects and Stories), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 23:58

## Done

Finished 2026-09-26 00:33. Not committed.

**What changed:**
- `src/figures/objects/Log.ts`:
  - `round: true` leaves a log neither bedded nor cut flat, and its trunk then takes all the height the stubs leave, so it stays round.
  - `log.axis` and `log.radius` say where its trunk is at its middle.
  - `fit()` in `objects/parts.ts` now returns the stretch it made.
  - A bedded log is built exactly as before. `Log.md` (the draft spec) names the round log in one sentence.
- `src/environtments/Lake/Logs.ts` (new):
  - The floating logs: each finds a home, floats 45% of its thickness down, and rides the waves as the boat's preview does. Round, it rolls to 0.6 of the slope.
  - Drift: up to 0.5 m downwind (about two thirds of the way in 8 s) and back over 40 s, the same for every log, plus a small eddy and swing of its own. Each stays within 0.71 m of its home.
  - `contains()` says whether a point is inside a log's box, for the cameras.
- `Lake.ts`: `LakeOptions.logs` and `Lake.logs`, built after the dock and moved on in `update` after the waves. `DockSite.ts` has `nearDeck()`.
- The meeting:
  - `LOGS = { keepOff: fishGoes }` in `Meeting.ts`, passed in `lake_meeting.ts`. Only the meeting's lake has logs.
  - `Sight.blocked()` also asks the logs, so every camera of the meeting (turns, the host's view, the follow) keeps out of them and doesn't see through them.
- Seed 6 places all four:
  - three in the far left water across from the landing, 1.5–2.7 m long, and one off the east bank;
  - 10 of 30 seeds placed four with these margins;
  - the first try, counting each log's full drift toward the others with 1 m of it, found room for two.
- rules.md: Objects rule 5 (the round log) and Stories rule 22 (the floating logs; task 14 took 21 meanwhile). State files: `lake_meeting.md`, `environments.md`, `objects.md`, `index.md`.

**How it was checked:**
- `npm run typecheck`: clean at 00:32.
- **The log's preview:** its geometry, and seven more logs of other seeds and sizes, hashed the same before and after, every attribute bit for bit.
- **The lake:** built with the dock alone and with the logs too, every stone, boulder, mud pit, tree, grass plant and the deck were the same. A lake built without `logs` has none.
- **Node, 30 min of the meeting** (8 viewers of every kind and the bots, the weather's own wind, over 0.5 in 24% of the frames), every part of every log every third of a second:
  - never on the bed (49 cm or more under it), 2.1 m or more from the shoreline, 1.8 m from the dock, 1.4 m from a boulder (the west one), 3.3 m from the fish's loop, never over where the fish goes, and 1.29 m or more from another log;
  - 45% of its thickness under the water on average at its middle (24–67% as waves pass along it);
  - at most 0.37 cm of height and 0.39° of tilt in a frame, and second differences under 0.08 cm and 0.1°, so no jumps;
  - it rocks more in wind: roll 0.43° against 0.73°, and height against the water 1.3 against 2.1 cm;
  - it strays up to 0.48 m from home, at up to 3 cm/s (median 0.5 cm/s), downwind on average;
  - nothing NaN;
  - over 54 000 frames the stage's camera was never inside a log, and no log came between it and the animal it followed;
  - a log's middle was in the picture in 24% of the frames checked, the nearest at 6–18 m.
- **Headless Edge on the GPU (port 8142),** the weather held (no fog or rain):
  - the meeting's opening shot calm, in a full wind, and at night;
  - the host's view;
  - a log close up at the waterline in felt and studio, filter off and on, by day and at night, and in wind;
  - another from low by day;
  - one end on, its end grain and round underside showing through the water.
  - The pictures were taken before the push went from 0.4 m over 12 s to 0.5 m over 8 s (the first run's logs went at most 1.8 cm/s, under "a few centimeters a second"). The homes didn't change, so only how far the logs had drifted in the wind shots differs, by some centimeters.
  - The last two close-ups were taken again on a fresh page: the first page stopped there, most likely reloaded by another session's edit.

**Left for the user:**
- The defaults are Claude's, from this task: four logs, their sizes, the 45% draft, the drift and the margins. With these margins the water has room for about four; more would need smaller margins or a smaller drift.
- From the opening shot the logs are 15–23 m off, across the water. The fish's loop and corridors keep them out of the water nearest the landing.
- Knowledge graph: not synced; Claude will ask first (CLAUDE.md).
