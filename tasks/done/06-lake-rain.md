# Rain at the lake, now and then

The user asked (2026-09-25): "add randomly rain in lake environtment". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

Read `docs/3d_modelling/rules.md` first, especially Environments rules 14 (splashes), 17 (weather), 22 (day and night) and 23 (fireflies), and Effects rule 8 (`keepSharp`). Task 02's Done section (`tasks/done/02-lake-day-night.md`) lists the hooks it made: `DayNight.ts`, `sceneAt()`, `Environment.dayNight`, `Stage.applyDaylight()`, `Sky.setDaylight()`, `Water.setNight()`.

## When it rains

- **Showers come and go on their own (default),** like the lake's fog and wind: a shower lasts 40–90 s, with 2–4 min of dry weather between showers.
  - It eases in and out over a few seconds, so nothing snaps.
  - The first shower comes about 40 s after the lake is shown, so it is seen without a long wait.
  - After that the times are seeded, the same on every load (`Weather.ts`).
- **No setting (default),** as the fog and wind have none. To look at a shower, shorten the first time for the screenshot and put it back afterwards (Environments rule 17).
- **Rain falls by day and by night,** whatever the time of day is.

## What it looks like

- **Falling rain:**
  - Streaks fall fast and lean downwind with the wind (`src/environtments/wind.ts`).
  - They only need to fill the space round the camera, so let them move with it and wrap round, as the sky dome does. The walking camera (`?camera=walk`) then walks through rain too.
  - Draw them as one instanced mesh, or lines moved on the graphics card, not one object a drop.
  - Streaks are thin, and the Kuwahara filter averages away anything narrower than its radius. Make them thicker than real, or mark them with `keepSharp()`, whichever reads as rain with the filter on (Effects rules 5 and 8).
- **Rings on the water:** drops landing on the lake make small rings all over its surface, drawn big enough to survive the filter, as the splash rings are (Environments rule 14). They are cheap, instanced and short-lived, and ride the waves at `heightAt()`.
- **Overcast:**
  - The sky greys over and the sun hides, as they do in fog (`Sky.ts`).
  - The daylight dims a step, through the same mixing the stage already does for night (`sceneAt()` and `Stage.applyDaylight()`).
  - The water's glint dulls (`Water.ts`).
  - A light haze may come with a heavy shower.
- **Fireflies shelter from the rain (default):** at night they fade out while it rains and come back after it (`Fireflies.ts`).
- **Colours come from the theme:** the rain is the water's colour lightened toward `light`, as the splash drops are. Add a role only if that can't be made to read (Colour and theme rule 2).
- Nothing else: no puddles forming, no wet ground, no rain sound (Building shapes rule 7).

## Where the code goes

- The shower schedule goes in `Weather.ts`, with the fog and the wind.
- The drops and the rings go in a file of their own, such as `src/environtments/Lake/Rain.ts`.
- The dimming goes through the hooks from task 02.
- `Lake.md` is the user's spec: don't edit it.
- Task 04 (mud pits) may be in progress in another session, and it also changes `Lake.ts`. Other sessions are changing the lake's files, `Stage.ts` and `previews.ts` too. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- Screenshots with headless Edge on a spare port, never 8087 (rules.md, Checking the result):
  - from the lake's preview camera, dry and in a shower, by day and at night (`?time=night`);
  - with the Kuwahara filter on and off, in `felt` and in `studio`;
  - with the walking camera in the rain;
  - the lake meeting in the rain.

  Rain in a still picture is easy to miss: compare a dry frame and a wet one side by side.
- Numeric checks:
  - the showers come and go on the schedule above, seeded;
  - the light and the sky ease with no jump from frame to frame;
  - nothing is NaN.
- The frame rate with the GPU, by day and at night, dry and raining (task 02 measured 60 frames a second at the lake). Keep a shower close to that.
- Add a rain rule to rules.md (Environments), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 18:24

## Done

**What changed**

- **`Lake/Weather.ts`:** showers come and go by themselves. Each lasts 40–90 s, with 2–4 min between, and the first comes 40 s after the lake is shown. They use a seeded generator of their own, so the fog and wind keep their times. `rain` (0 to 1) eases in and out, and a full shower brings a light haze. There is no setting.
- **`Lake/Rain.ts`** (new):
  - **Streaks:** 3000 streaks, one instanced mesh placed on the graphics card. They fall and drift downwind in a box that follows the camera, so the walking camera walks through rain. They face the camera, are folded away below the water or when the shower is light, are 1.2 × 40 cm, `keepSharp`, and lit so they dim at night.
  - **Rings:** 280 instanced rings on the lake that ride the waves. They are `keepSharp` too, because the filter averaged them away as transparent rings.
  - Both are drawn once in the first frame, so they compile with the lake, as the fireflies are.
- **Overcast through task 02's hooks:**
  - `DayNight.overcast` makes `DayNight.dim` a step (30%) toward night. The stage now mixes its light by `Environment.dayNight.dim` (added to the interface in `previews.ts`; `Stage.applyDaylight`). `night` itself is unchanged, so the fireflies don't come out by day in rain.
  - `Sky.setDaylight(..., overcast)`: the sky greys over (up to 75%) and the sun hides.
  - `Water.setNight(night, rain)`: the glint dulls.
  - `Fireflies.update(..., rain)`: they shelter, their lanterns out once the rain is half as hard as it gets.
  - `Lake.ts` now updates the weather first each frame, then the time of day.
- **Docs:** `rules.md` Environments rule 25. Knowledge synced.

**How it was checked**

- `npm run typecheck` passes.
- **Numeric** (Node):
  - Showers fell at 41–111, 239–282, 402–447, 633–718 and 928–983 s, identical on a second run.
  - Over 12 minutes with the day/night cycle and showers, the largest change in one frame was 0.0015 in the dimming, 0.18 of 255 in the sky colour and 0.00036 in the fog density. Nothing was NaN.
  - In a night shower, the fireflies' lanterns were fully out.
- **Screenshots** (headless Edge on the GPU, port 8131): the lake from the penguin's camera, dry against a full shower, by day and at night, in felt and studio, with the filter off and on; the walking camera in the rain; the lake meeting in the rain. With the filter on, the streaks and rings stay visible, the sky greys, and the light dims.
- **Frame rate** (GPU, penguin at the lake, filter on):
  - By day: 44.5 dry, 43.6 in a shower.
  - At night: 47.3 dry, 42.8 in a shower.
  - Other sessions were loading the machine, so even dry was below task 02's 60. The shower's own cost is a few frames a second at most.

**Left for you**

- The streak count, their size and the ring sizes are my choices.
- A shower's cloud dims the light 30% of the way toward night.
- There is no rain sound, and the ground doesn't get wet, as the spec said.
- To watch a shower, wait 40 s after the lake loads.
