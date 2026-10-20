# Video: beautiful fire in Blender (Max Hay)

An external reference the user added for making fire (2026-10-18: "i have new reference for creating fire").

| | |
|---|---|
| **Video** | [How to Make Beautiful Fire in Blender](https://www.youtube.com/watch?v=azM_ZlToG-M) |
| **Channel** | Max Hay (environment artist; sells a 3D environments course and asset packs) |
| **Published** | 2023-04-19, 23:06 long |
| **Tool** | Blender with Cycles (GPU), on a photo-scanned tree stump by a pool, at night |

**How this summary was made**:
- The video is **narrated**: its English captions (about 5,000 words) give the steps.
- Settings were read from **one frame every 30 seconds** at 720p (46 frames), and from the node editor and particle panel at the steps that set them.
- The video and the frames were deleted afterwards.

## What it builds

A campfire on a tree stump at night: real flames, a warm flickering light on the roots and rocks around it, a light haze in the air, and sparks drifting up and around. A camera moves slowly towards it for 150 frames.

**The idea**: "take a video of fire, slap it on an image plane, and wrap the plane around some object". He calls it a bit of a cheat, but the best-looking way to get fire in Blender "99% of the time". It breaks only when the camera goes round the fire or looks down on it, because the plane is flat.

## The steps, in the video's chapters

| Time | Chapter | What he does |
|---|---|---|
| 0:00 | Introduction | The test scene: photo scans of rocks, roots and a stump; a camera with two keyframes moving forward; a 150-frame timeline |
| 1:04 | Setting up the fire footage | A **plane** turned 90° to face the camera. A **video of fire** from Pexels (a free stock site with videos), dragged into the Shader Editor. **Slow-motion fire looks best** in 3D. He also suggests filming your own at night on a phone: the background comes out black |
| 2:59 | Texturing and transparency | UV › **Cube Projection**, then scaled so the footage keeps its aspect ratio. The footage's **Color into Emission** (Principled BSDF), strength turned up. The **Color into Alpha**: the black background disappears. A **Color Ramp** between them, its **black handle pulled up a little**, cuts the faint grey edge where the background isn't fully black |
| 5:33 | Animating and coloring | The movie's **Frames** set to the timeline's length (150), **Cyclic** and **Auto Refresh** on, so it plays and loops. A **Mix Color** node set to **Multiply** with a rich orange, to colour the fire. A Hue/Saturation node can change its colour completely, for example to blue |
| 7:50 | Scene lighting and volumetrics | The fire alone lights the scene and flickers a lot, which is realistic but heavy. So the emission goes down, and a **Point light about the fire's size** (radius about 0.6 m, orange) does most of the lighting. Emissive lighting is also noisy to render. **Multiple Importance** turned off on the light, so its ball doesn't show as a reflection in the water. **Volumetrics**: a big cube shown as Bounds, with **Volume Scatter**, Density **0.02**, Anisotropy "turned up a little" (0.47 on screen; he says ".7"), for a soft haze round the light |
| 10:29 | Adding sparks with particles | A **spark**: an Icosphere with 1 subdivision and an Emission material (strength about 5), scaled down. An **emitter**: a plane at the fire. **Particle system**: Emitter, Number 1000, **Gravity 0** (Field Weights), Normal velocity 1–2 m/s, Lifetime 150. Rendered as **Object** (the icosphere), with a little scale randomness. A **Turbulence** force field, Strength about **2.5**, makes them swirl. **Frame Start −200** and **Lifetime 400**, so the sparks are already in the air on frame 1 |
| 16:30 | (motion blur) | **Motion Blur** on (shutter about 0.7–0.8): with the turbulence, the sparks streak at different speeds. **Show Emitter** off in the particle Render panel. **Bake** the particles (Cache) or they glitch or don't render |
| 17:20 | Refining and duplicating | More fire from the same clip: Shift+D, make the material **single-user**, and **Offset** the movie by about 50 frames, so copies side by side are not the same flame. Subdivide a plane and bend it with **Proportional Editing** to wrap fire round an object, with **Correct Face Attributes** off |
| 20:20 | Rendering the final animation | Depth of field. 2000 × 2000, 24 fps ("more cinematic"), 100 samples, a **PNG sequence** (a crash loses nothing). **Persistent Data** on (Performance) for faster frames. Afterwards: the flames were a bit too big, so he'd scale the planes down and use more of them |

## What we can use

The project's sources are Blender and Poly Haven only, so a stock video of fire is out. Two ways remain:
- **The shader draws the footage** (done for the damar): everything else in the method stays, but the "video" is a procedural flame.
- **Our own footage**: as he suggests, a slow-motion phone video of a flame filmed at night. It would go into the same cards in place of the procedural flame. That is the user's to film, if a fire needs it.

| From the video | Where it goes in the story |
|---|---|
| Fire on cards; brightness keyed into alpha; an orange multiplied in; the footage looping; copies offset in time | **Done: the damar's four flames** ([objects/damar.md](../../../docs/stories/ramabargawa/objects/damar.md)), and **the fire samples** in `blender/objects/fire.blend` ([objects/fire.md](../../../docs/stories/ramabargawa/objects/fire.md), guide [08](08-fire-samples.md)): Renuka's cooking fire, a campfire, smouldering embers and a big fire for the pyre. The wood fires are wider crossed cards whose tongues come from rising turbulence, and they add their light to what is behind them |
| A point light the fire's size does the lighting; the emission stays low | Done: the damar 0.5 W, 2 cm radius; the samples 30 W (hearth) to 900 W (large), radius a third of the fire's width |
| A flickering light | Done: the damar's light wavers ±12%; each fire sample's ±25% at its own puffing rate (1.68 / √D Hz), and moves a little so the shadows sway |
| **Volume Scatter haze** (density 0.02) | The light states (milestone 5): S4's moonlit pendopo with the lamp's glow; S5's smoke; S6's night rain round the pyre |
| **Sparks**: particles with no gravity in a turbulence field, already in the air on frame 1 | **Done in the fire samples**, in Geometry Nodes rather than particles: worked out from the time alone, so no bake, and they work inside a linked collection instance, where particles and force fields don't. For the **pyre** (S6, "sparks rising" in [environment.md](../../../docs/stories/ramabargawa/environment.md)) |
| Motion blur, a PNG sequence, Persistent Data | The render setup with the story cameras, later |

**Kept different on purpose**:
- **Multiple Importance stays on** for the damar's light. His light is 0.6 m across, and its ball showed in the water. Ours is the flame's own size, so its glint on the bronze is the flame's reflection.
- **Cards, not one plane facing the camera.** Story cameras will move round the lamp, so the flame is drawn on three upright cards through its axis. From any side it reads as one flame. Straight down from above it still shows almost nothing, as he warns.
- **The time is a driver on the frame** (`frame / 24`), not a movie's Cyclic and Offset: the procedural flame never repeats. Each instance starts at its own moment (Object Info › Random), so no copies of the material are needed.

How it was applied, by hand: [07-modelling-damar.md](07-modelling-damar.md), Parts 7 and 8 (the lamp flame); [08-fire-samples.md](08-fire-samples.md) (wood fires, sparks, embers). In code: `scripts/common/fire.py`.
