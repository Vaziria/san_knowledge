After: 01-firefly-figure.md

# Day and night at the lake, with fireflies at night

The user asked (2026-09-25): "in lake add day and night, and if night, add firefly". That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

Read `docs/3d_modelling/rules.md` first, especially Environments and Colour and theme. The fireflies are the firefly figure from task 01 (`src/figures/animals/Firefly.ts`).

## Day and night

- **They take turns on their own (default),** like the lake's weather (`Lake/Weather.ts`, Environments rule 17): day 6 min, dusk 1 min, night 4 min, dawn 1 min. The first dusk comes about a minute after the lake is shown, so night is seen without a long wait, as the first fog is. Dusk and dawn ease from one to the other, so nothing snaps.
- **A setting to pick it (default):** Time of day, with `cycle` (the default), `day` or `night`, on the Environments tab and kept in the URL as `?time=` (rules.md, Control panel rule 5). It goes to the stream's hidden page with the other settings (Control panel rule 10). Only the lake has night: other environments stay in day.
- **What night is (default):** the lake the way it looks now, darkened.
  - The sky dome, the fog, and the far land's fade into the sky all turn to a night sky colour. `Sky.ts`, `Border.ts` and `haze.ts` take the sky's colour when they are built, so make that colour follow the time of day.
  - The hemisphere light and the key light dim and turn cool. The key light becomes moonlight from the same direction: moving the sun would move the shadows, and that isn't part of this task.
  - The sun and its glow fade out at dusk and come back at dawn. No moon and no stars: the user didn't ask for them (Building shapes rule 7).
  - The clouds darken with the sky, and the water's glint dulls.
- **Dim, not black:** a figure at `Lake.LANDING` must still read from its preview camera at night.
- **Night colours come from the theme:** add night values to every theme's `scene` (sky and background, the two hemisphere colours, the key light, and their intensities), picked like the other scene colours. In the brand themes, use the brand's palette (`docs/3d_modelling/brand_getresolved.md`).
- **The stage keeps owning the lights** (Control panel rule 1). The environment tells it how far into the night it is, as it hands over its fog (`Environment.fog` in `previews.ts`), and the stage applies that every frame.

## Fireflies at night

- **About 20 fireflies (default)** come out at dusk and go at dawn, their lanterns fading in and out. There are none by day.
- **Build them once and hide them by day.** Don't add and remove them at dusk and dawn: three.js recompiles every material in the scene when the number of lights changes.
- **Where:** over the grass along the shore and round `Lake.LANDING`, where the cameras look, 0.3–2 m above the ground, with a few over the water's edge. Keep them out of the figure's own space at the landing.
- **How they move:** each wanders, flying forward and steered by turning it (`rotation.y`), as the animals' demo steers them. It turns now and then, rises and sinks, and stays in its area. The figure's `Walk()` is only 4 cm/s and `Run()` 12 cm/s, which look frozen from cameras several meters away, so let the swarm fly as fast as reads as drifting from there. Place each by its height: the figure's origin is on the ground, its body 2.2 cm above it. Each one flashes on its own timing, not in step with the others.
- **Seeded:** placed and timed by a seeded random generator, so the lake is the same on every load (Environments rules 11 and 17).
- **Changes to the firefly figure:** `Firefly.ts` (done in task 01, see `tasks/done/01-firefly-figure.md`) takes only `{ theme }`. So every firefly flashes at the same moment, and each carries a point light (`firefly.glow`, 4 cm reach). Give it options for its own flash timing (a seed or an offset), its speed if the swarm needs it, and no light. Its preview must stay as it is.
- **Keep night as cheap as day:**
  - Every point light adds cost to every lit surface in the scene. The stream's hidden renderer without the graphics card already draws the fox scene at about 12 pictures a second (Control panel rule 10). So give lights to none of the fireflies, or only a few.
  - Keep the number of lights the same by day and by night: turn a light's intensity down to 0, don't hide it, because a hidden light no longer counts and the count changes. Task 01 measured the stall: the first firefly light on the lake froze the page for 1.8 s while the scene recompiled.
  - Measure the frame rate at the lake by day and by night, with the GPU setting on and off.
- **They must be visible:** from the lake's cameras a 2 cm firefly is a few pixels, and the Kuwahara filter averages away anything narrower than its radius (Effects rules 5 and 8). The lanterns must still read as points of light:
  - mark the lantern material with `keepSharp()`;
  - if that is not enough, give each lantern a soft halo in the glow colour;
  - check from the lake's preview camera with the filter on.

## Where the code goes

- The time of day goes with the lake's weather, in `Weather.ts` or a file of its own in `src/environtments/Lake/`. The swarm goes in its own file there, such as `Fireflies.ts`.
- `Lake.md` is the user's spec: don't edit it. Say in the Done section that the user may want to add day and night to it.
- Other sessions are changing many of the files this touches right now: `Lake.ts` and other lake files, `Sky.ts`, `Border.ts`, `Stage.ts`, `previews.ts`, `theme.ts`, `settings.ts` and the panel. Change only the lines you need, and never restore any of them from a copy.
- The lake meeting story (`src/story/lake_meeting/`) plays at the lake, so check it at night too.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- Screenshots with headless Edge on a spare port, never 8087 (rules.md, Checking the result), from the lake's preview camera (the penguin's), by day, at dusk and at night with the fireflies. Shoot with the Kuwahara filter on and off, in `felt` and in `studio`. Test `?time=night` and the panel's control.
- Numeric checks:
  - over a whole cycle, the lights and the sky colour change smoothly, with no jump from frame to frame;
  - over a long run the fireflies stay in their area and are all hidden by day;
  - the frame rate is measured, as above.
- Add what you learned to rules.md (Environments), then call `knowledge_sync`.
- Don't commit.

Taken: 2026-09-25 17:16

## Done

**What changed**

- **`src/environtments/Lake/DayNight.ts`** (new): the time of day.
  - By itself it runs 6 min of day, 1 min of dusk, 4 min of night and 1 min of dawn, easing between them. The first dusk comes 1 min after the lake is shown.
  - `set(mode)` holds it at `day` or `night`. A change eases there over a few seconds; a lake built with `?time=night` starts at night.
  - `night` (0 to 1) and `scene` (the mixed colours and intensities) are what the lake and the stage read.
- **`src/theme.ts`**:
  - `scene.night` in every theme: horizon, zenith, the two hemisphere colours, the moonlight, and their intensities. `felt` gets a dusky blue-grey under navy; `pastel` and `purple` get their own hue deepened; `studio` is a cooler step down, with brighter moonlight because its figures are dark; both getresolved themes get the brand's dark mode.
  - `sceneAt(theme, night)` mixes the day and night values.
- **`src/previews.ts`**: `TIMES_OF_DAY` and `TimeOfDay`, plus `Environment.dayNight`, which the lake sets.
- **`src/settings.ts`, `main.tsx`, `panel/Panel.tsx`**: the Time of day setting, `?time=`. Being a setting, it also goes to the stream's hidden page. On the Environments tab it is a radio list (cycle, day, night) with a note that only the lake has night.
- **`src/Stage.ts`**:
  - `setTimeOfDay()`, which also hands the time to every new environment before its first frame.
  - `applyDaylight()` mixes the background, the hemisphere light and the key light every frame. The stage keeps owning its lights; the key light becomes moonlight from the sun's direction, so shadows don't move.
- **Sky colours that were fixed when the lake was built now follow the time of day:**
  - `haze.ts`: the sky colour is a uniform. A `THREE.Color` passed in is followed as it changes; any other colour stays fixed. The grass environment is unchanged.
  - `Sky.ts`: `setDaylight()` sets the dome and the clouds' skylight, and a `skySunShow` uniform fades the sun out over the first half of dusk and back in over the last half of dawn.
  - `Border.ts`: `sky` option. The cliffs' haze and the mist re-tint every frame, and the mist is lightened less at night. The day looks exactly as before, because the cliffs' shared material is still hazed as if once per mesh.
  - `Ground.ts`: a `sky` parameter.
  - `Water.ts`: `setNight()` dulls the glint.
  - `Lake.ts`: shares one horizon colour among all of these, and sets the fog's colour from it.
- **`src/environtments/Lake/Fireflies.ts`** (new): 20 fireflies.
  - **Where:** over the grass either side of the landing and in front of it, where the preview cameras look, and 4 over the water's edge.
  - **How they fly:** each wanders within its own area at 0.25–0.45 m/s, steered by turning it (`rotation.y`), rising and sinking between 0.3 and 2 m above the ground. They keep 1.8 m clear of the landing.
  - **Flashing:** each flashes on its own timing.
  - **No lights**, so the light count is the same by day and by night.
  - **Visible from meters away:** the lantern uses `keepSharp()`, and a 16 cm glow halo is blended so the filter still keeps the lantern sharp.
  - **Built once, hidden by day.** They are drawn dark in the first frame so their materials compile with the lake's.
- **`Firefly.ts`**: `FireflyOptions` adds `seed` (its own flash timing), `flight` (speeds), `light: false` and `lit` (lantern level), plus a `flash` getter and `Firefly.HOVER`. Without options the preview is unchanged: the same numbers as before (box, hover, flash every 2.51 s from 0.65 s).
- **Docs:**
  - `docs/3d_modelling/rules.md`: Environments rules 22 (day and night) and 23 (fireflies), plus night values in Colour and theme rule 3.
  - `brand_getresolved.md`: a night row.
  - Knowledge synced.

**How it was checked**

- `npm run typecheck` passes.
- **Screenshots:** headless Edge on the GPU, port 8131, from the penguin's lake camera.
  - Day, dusk (night = 0.5) and night, in `felt` and `studio` with the filter off and on, and in `getresolved`.
  - The lake meeting at night.
  - At night the scene is dusky blue, the penguin still reads, and 4–5 fireflies are in view as glowing dots, also with the filter on.
- **Panel:** in the browser, clicking the Time of day's night gave `?time=night` and full night after about 4 s. Clicking cycle removed the parameter and eased back to day. `?time=night` on the floor and the grass stays in day.
- **Numeric checks in Node:**
  - Over a cycle and a half, with the setting switched along the way, the largest change in one frame is 0.014 of the way to night, 3 of 255 in the sky colour, and 0.02 in the key light's intensity: no jumps.
  - Over 30 min of night the fireflies stay at most 1.56 m from home and 1.97 m or more from the landing, 0.30–1.96 m above the ground, with no NaN. By day nothing of them is drawn.
  - The first steering let one stray 4.3 m and fly within 0.22 m of the landing; that is fixed.
- **Frame rate** (penguin at the lake, filter on, 1000×700):
  - GPU: 60 frames a second by day and by night.
  - GPU off (SwiftShader): 0.6 by day and by night, which is the lake's own cost there.
  - Night adds about 90 draw calls (182 → 275) and 3 k triangles.
  - The first night used to stall the page 0.2 s while the fireflies' materials compiled; with them drawn dark in the first frame there is no stall.

**Left for you**

- `Lake.md` is your spec and I didn't edit it. You may want to add day and night, the fireflies and the Time of day setting to it.
- These are my choices to adjust: the night colours and intensities, the halo's size (16 cm, larger than real, so it survives the filter), and the fireflies' speed (0.25–0.45 m/s).
- There is no moon and there are no stars, since they weren't asked for.
- Not checked: the stream end to end at night. `?time=` goes to the hidden page through the settings query like every other setting.
- 11 knowledge nodes need summaries (`bin\knowledge.exe sync`).
