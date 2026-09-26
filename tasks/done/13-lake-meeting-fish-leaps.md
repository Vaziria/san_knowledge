After: 12-lake-meeting-keep-position.md

# Lake meeting: the fish leaps out of the water now and then

The user asked (2026-09-25): "in lake meeting, sometimes fish jumpout from water". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

The meeting's fish swims a loop in the water (`FISH` in `src/story/lake_meeting/Meeting.ts`). Today it leaps only for a supporter: a Super Chat, a Super Sticker or a new member (spec item 7, the `support` turn, through `pendingLeap`). The leap is the fish's `JumpOutFromWater(height)` (`src/figures/Fish/Fish.ts`; rules.md, Moving parts rule 12). The water throws up a splash where it leaves and comes back in (Environments rule 14). Read rules.md (Stories) too.

## What changes

- **Leaps of its own, now and then (default):** every 20–60 s, at random, the fish leaps out and falls back in.
- **Lower than a supporter's leap (default):** 0.15–0.4 m, so a supporter's leap stays the big one. Check what heights the supporters' leaps get, and keep these below them.
- **No camera (default):** like a bot's own doings, these leaps don't take a turn or move the camera. They are seen when the fish is in view: its loop is by the landing, where the meeting's cameras look.
- **A supporter's leap always comes first:** no leap of its own while one is waiting (`pendingLeap`) or under way, and the fish already ignores a leap while the last one is still settling. After a supporter's leap, the next leap of its own is at least 20 s away.
- **Day, night and rain alike.** The logs (task 11) and the dock (task 09) keep clear of the fish's loop. Check that its leaps clear them too, if those tasks are done by then.
- **The spec:** add to item 7 of the draft `lake_meeting.md` that the fish also leaps now and then of its own accord. Keep its note at the top that it is a draft for the user to correct.

Tasks 08–12 change `Meeting.ts` too. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- **Numeric checks,** in Node over a long run (30 min of the meeting, with a few supporters' leaps from test events):
  - it leaps every 20–60 s, at 0.15–0.4 m;
  - never while a supporter's leap is waiting or under way, and not within 20 s after one;
  - the fish stays on its loop between leaps;
  - nothing is NaN.
- **Screenshots** with headless Edge on a spare port, never 8087 (rules.md, Checking the result), from the meeting's view with a leap under way and its splash, with the Kuwahara filter on. Catch the moment by driving the frames (Checking the result rule 7).
- Add what you learned to rules.md (Stories), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 19:29

## Done

**What changed**

- `src/story/lake_meeting/FishLeaps.ts` (new): the fish's own leaps.
  - Its time comes 20–40 s after the last, and it leaps at the next place where the leap keeps it on its loop, so leaps come 20–60 s apart.
  - Heights are 0.15–0.28 m, below a supporter's smallest (0.3 m, a Super Chat under $2).
  - None while a supporter's leap waits or its turn is under way, and the next at least 20 s after that turn ends.
  - No turn and no camera.
- `Meeting.ts`: four lines hook it in, plus `fishCanLeap()`. The fish may leap only on its loop's two surface sides, facing along the side, in the first 1.2 m of it. A leap of its own carries it 1.25–1.6 m and a side is 3.2 m long. Leaping anywhere, it overshot its loop's corners and wandered off it up to 3.1 m for up to 28 s, onto the lake bed by the shore.
- `lake_meeting.md` item 7 (the draft note kept), and rules.md Stories rule 18.

**How it was checked**

- `npm run typecheck`.
- **Numerically in Node**, four runs of 30 min with a supporter's leap every 3 min, all ALL OK:
  - 40–42 leaps of its own a run, 0.15–0.28 m high, 25–85 s apart. The gaps over 60 s had a supporter's leap in between.
  - None while a supporter's leap waited or within 20 s after one.
  - After its own leaps the fish was at most 0.69 m off its loop, back on it within 6.6 s, and never on the bed.
  - Nothing was NaN.
- **In headless Edge**, the fish's leap was caught in the air, by watching the fish through a wrapped `Object3D.updateMatrixWorld`.

**Left for the user**

- **From the host's view at the landing, these leaps can't be seen.** The loop lies about 16 m from that view, straight behind the host; in fog neither the fish nor its splash showed. Filmed 4 m from the loop (a view set up in a scratch copy only), the leap and its splash show, small. With task 08 the quiet camera follows animals, so the fish is seldom in view at all. If you want them seen, any of these could be a new task:
  - move the fish's loop to one side of the host;
  - let the quiet camera glance at the fish when it leaps;
  - make its own leaps bigger.
- **A supporter's leap (unchanged) still starts wherever the fish is.** It carries the fish up to 2–3.6 m off its loop for 15–23 s, and onto the lake bed by the shore for some 200 frames a run. It did so before this task too (measured on the same schedule without leaps of its own). Starting supporters' leaps only where they fit, or a longer loop, would fix it.
- Nothing is committed.
