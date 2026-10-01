1. Ambient light is too strong
2. Shadows are too weak
3. Ground is very uniformly bright
Sky / ambient
      ↓
   soft fill
      ↓
TREE ──────── sunlight
 ↓               ↘
shadow            ground
 ↓
grass / rocks


4. No strong light hierarchy


Recomendation:
1. Use one dominant sun
              ☀️
             ↘
              ↘
       TREE    ↘
                 🧍
────────────────────────
          ground

2. Reduce ambient light

Your current image feels like:

Sun        ████████
Ambient    ████████

I'd aim for:

Sun        ████████
Ambient    ████


3. Add stronger contact shadows

This is especially important for your low-poly style.

I would emphasize shadows around:

🌳 tree trunk
   ↓
  ███       ← contact shadow

🪨 rock
 ████       ← dark base

🌿 grass
 ███        ← subtle AO

🧍 character
 █████      ← soft foot shadow

🌉 bridge
 ███████    ← underside shadow

This will make the scene feel much less like objects were simply placed onto the terrain.

4. Make the waterfall less white

Currently the waterfall attracts too much attention.

I'd make it approximately:

Current:
██████████  white

Recommended:
███████░░░  blue-white

Keep the brightest white only on:

falling water edges
foam
splash
highlights

The main water body should have visible blue/cyan structure.


7. Add subtle atmospheric perspective

Your cliff is very close to the camera visually despite being far away.

A very subtle fog would help:

Foreground
████████████

Middle
██████████

Background cliff
████████

Don't use heavy fog. You want just enough to reduce contrast with distance.











Taken: 2026-10-01 21:07

## Done

Taken to be the forest lake (its landing's views: the waterfalls, the cliffs, the trees, the bridge and figures on its grass), since that is where all of it is. What changed, in `typescripts/animation/src/environtments/ForestLake/`:

1. **Ambient too strong, no light hierarchy:** each season's sky light is about half what it was (0.95 in spring, summer and autumn, 1.1 in winter, from 1.6 and 1.75) and the sun a little stronger (2.95, 3.05, 2.9, 2.5, from 2.65, 2.75, 2.6, 2.2), so the sun is about three times the sky light (`ForestLake.light()`).
2. **Shadows too weak:** the ground's baked light keeps 48% of the light in a tree's shadow (60% before); and in the sun's own shadow, lit by the sky light alone, everything is about 40% darker than before.
3. **Contact shadows:** the ground right at the foot of a trunk, a rock or a bush is 52% as light (72% before), fading out as (1 − d/foot)^1.6 (`SHADED`, `CONTACT` in Terrain.ts), and rocks from 0.3 m across cast them (over 0.7 m before, Scatter.ts). The grass and flowers growing there take the ground's light, so they darken with it (the "subtle AO"). The figures' foot shadows are the sun's, darker now; bridges' undersides are lit by less sky light.
4. **Ground very uniformly bright:** the grass is patched lighter and darker in two sizes (about 20 m and 6 m), its darks deeper (Terrain.ts).
5. **Waterfall less white:** blue-white: deep blue and cyan in its body, pale along its streaks, white only on the brightest streaks, along its edges, at its lip and where it lands, in its foam and mist (Water.ts). Measured from near-white (#d9eaf3, #eff5f7) to blue-white (#a9d3e4, #c9dfe8).
6. **Atmospheric perspective:** the fog is a thin haze from 10 m to 360 m (from 70 m before, so nothing in the valley was hazed): the cliffs 40–50 m off about a tenth of the way toward the sky's colour, the far forest a fifth or more (`FOG` in ForestLake.ts).

**Checked:** `npm run typecheck` clean. In headless Edge on the GPU, the landing's views before and after (sunlit grass about 5% darker, shade about 40% darker, the figure's shadow dark and distinct, the lawn patched, rocks' and bush's feet darker, the cliffs and far trees standing back), and the forest lake meeting's opening in spring, autumn and winter; 60 frames a second at the landing in spring and winter. In Node the forest lake builds in 1.65–1.82 s by season (about 1.72 s before). State files updated: forest_lake.md and index.md; the spec's waterfall and light paragraphs (ForestLake.md, a draft).

**Left for the user:** whether the strengths are where you want them (the sky light, the shadows' depth, the haze, the waterfalls' blue). Contact shadows are baked into the ground only: the water under the bridges and piers gets none. Not committed.
