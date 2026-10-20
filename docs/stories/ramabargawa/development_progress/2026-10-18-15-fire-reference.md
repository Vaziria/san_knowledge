# 2026-10-18: Fire after Max Hay's video, on the damar's flames

## Request (the user's words)

| Request | The user said |
|---|---|
| A new reference for fire | "i have new reference for creating fire https://www.youtube.com/watch?v=azM_ZlToG-M" |

A reference the user gives is applied in the same step. The only fire built so far is the damar's four flames, so it was applied there.

## Done

- **The video, summarised**: [references/video-fire-max-hay.md](../../../../project/bargawa/references/video-fire-max-hay.md).
  - Max Hay, "How to Make Beautiful Fire in Blender" (2023, 23 min, Cycles).
  - Read from its captions (narrated) and one frame every 30 seconds. The downloads were deleted afterwards.
  - **The method**: fire footage on a plane, keyed to alpha through a Color Ramp, an orange multiplied in, looping; a point light the fire's size does the lighting; Volume Scatter haze; sparks from particles in a turbulence field; motion blur; copies offset in time.
  - The table "What we can use" says where each part goes in the story.
- **The damar's flames, redone** (`scripts/common/fire.py`, new):
  - **Cards**: `geo.cards`, three upright cards through the flame's axis, replace the lathed teardrop.
  - **`fire.flame_material`** draws the flame on the cards and moves it with time:
    - a slow sway and breathing, small rising licks
    - each instance at its own moment (Object Info › Random)
    - alpha keyed from the brightness, the black handle raised
    - the video's orange, multiplied in (`Flame_Tint`)
    - a touch of blue at the root
  - **`fire.flicker`**: the light wavers ±12% (a driver of sines on the frame).
  - `mesh.flame_material` was removed.
- **Not from stock video**: our sources are Blender and Poly Haven, so the shader draws the "footage". The user's own slow-motion video of a flame at night (the video's own tip) could go on the same cards if a fire ever needs it.
- **`views.py`**: a `{"frame": n}` option. New damar stills: `flame`, `flame_f7`, `flame_f15` (one flame at frames 1, 7 and 15) and `flame_night`.
- **Docs**:
  - guide 07: the cards, the flicker driver and the flame shader by hand, step by step, and the new pitfalls
  - [objects/damar.md](../objects/damar.md)
  - [environment.md](../environment.md): the effects table points to the video
  - references/README.md

## Checks

Stills in `renders/checks/damar/` (all views, plus the four flame stills) and `renders/checks/jatisrana/` (`lamp`, `lamp_night`). Fixed along the way:
1. **A solid orange band above the tip**: the width curve (Float Curve) ran on past its end and went negative. It now extends flat, and the width is kept above zero.
2. **A thin line through the flame**: a card seen nearly edge-on. Cards now fade with the viewing angle (Layer Weight › Facing); one card is always within 30° of facing the camera.
3. **A red outline round the flame**: the alpha came from the shape, so the dim rim was opaque. Keyed from the brightness, as in the video, the rim fades out.
4. **The rim read pink** over the pale sky: a deep red at low alpha. It is orange now.

**Result**: a soft flame, pale gold inside with an orange rim. Between frames 1, 7 and 15 it leans, stretches and licks. The four flames differ in height and lean. At night each one lights its dish. Seen from straight above, the flame is almost gone, as the video warns.

`damar.blend` was rebuilt (`-Force`: it was open in Blender, so it needs File › Revert). `jatisrana.blend` links it and needed no rebuild.

## Next

- The pawon's fire (S1) and the pyre (S6) with `fire.py`: wider cards, several tongues, the video's sparks.
- Haze (Volume Scatter) with the light states (milestone 5).
- Motion blur with the story cameras.
