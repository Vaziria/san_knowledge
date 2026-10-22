# Texture library: ground

`ground.blend`, one family of the texture library. Built by `run.ps1 texture ground` (`scripts/textures/build.py`, the materials from `scripts/common/terrain.py`, `textures.py` and `polyhaven.py`). Each material is an asset in the catalog **Bargawa/Ground**. The collection `Samples` holds a block and a post of the textures, a flat square of the water, and a 1.4 m patch of each bare ground (the spiky ground, the soil).

| Material | Used on | Source | Tile | Palette mix | Palette | Bump |
|---|---|---|---|---|---|---|
| `M_ground` | the terrain, under the grass | Poly Haven `brown_mud_leaves_01` (CC0) | 4.0 m | 35 % | #505C34 (muddy, a green cast) | 0.3 |
| `M_earth` | the paths, the kitchen floor | Poly Haven `forest_ground_04` (CC0) | 2.5 m | 25 % | #785C42 | 0.3 |
| `M_water` | the lake (a stand-in until its water shader) | none | | | #121E20, roughness 0.04 | |
| `M_ground_spiky` | nothing now; Jatisrana's bare ground until 2026-10-21 | the user's photo, `blender/assets/photos/ground_spiky.jpg` | 2.86 × 4.5 m, mirrored | none (movie first, realistic) | | 0.3, from the spikes' ramp |
| `M_ground_soil` | the bare yard round the pendopo and the huts, the far bank round Citrarata's spot | the user's soil photo, `blender/assets/photos/1cb0165c04eb35eb7bd78df3f83cc534.jpg` (seamless) | 5.6 m, repeated | none (movie first, realistic) | | 1.0, distance 0.8, from the photo |

The Poly Haven textures are box-projected in object space. The terrain is one large object, so the ground's tiles run unbroken over the whole valley.

## The spiky ground

The user's look from their `Ground_2.blend` (2026-10-20): the photo's brightness through a Color Ramp is the height. It comes in two parts, both in this file:
- **`M_ground_spiky`**, the material:
  - the photo on the mesh's `UVMap` (in tiles of the photo, mirrored at every tile)
  - Roughness 0.85, Metallic 0, IOR 1.5
  - a bump from the same ramp
  - see-through where the mesh's `fade_out` attribute is high (the patches' edges)
- **`GN_ground_spiky`**, a Geometry Nodes group (the spikes), used as a modifier:
  - Subdivide Mesh (Detail), then Set Position along the normal by (Color Ramp(photo) − Midlevel) × Height × the `spikes` vertex group
  - Ramp black at 0.0, white at 0.427; Height 0.1 m; Midlevel 0
  - Checked against the user's shader displacement: the same spikes.

**What it needs from the mesh**: a fine flat-shaded grid (14 cm, halved by Detail 1 to 7 cm), a `UVMap`, and optionally the `spikes` vertex group and the `fade_out` attribute. Without them it gets full spikes and no fade. `terrain.relief_patch` makes such a mesh. The settings are `SPIKY` in `scripts/common/terrain.py`. How to build it by hand: [references/02](../../references/02-texturing-poly-haven.md), "The spiky ground".

## The soil

The user's `Soil` material from `material_manual.blend` (2026-10-21, "this is better for jatisrana"). Since then it has been Jatisrana's bare ground (`terrain.BARE`), in place of the spiky ground. It is built the same way, in two parts:
- **`M_ground_soil`**, the material:
  - the seamless soil photo with leaf litter on the mesh's `UVMap`, once every 5.6 m (the user's plane: 5.59 m), repeated
  - a **Bump** from the photo itself, Strength 1.0, Distance 0.8, as the user's
  - Roughness 0.85, Metallic 0, IOR 1.5 (the user's own: 0.445, 0.177, 1.0)
  - see-through where `fade_out` is high, as the spiky ground
- **`GN_ground_soil`**, its relief: the same node chain as `GN_ground_spiky`, with the user's ramp, black at 0.655 and white at 0.932, Height 0.1 m, Midlevel 0.

**Checked against the user's plane**:
- With the same shading values, the two render the same (the same mean colour to 0.1/255).
- Their displacement changes nothing: the render is identical with it on or off. The ramp starts at 0.655 in linear brightness. Only 0.03 % of the photo's pixels are that bright (the brightest is 0.817), and none of the plane's 4,225 points lands on one.
- So the look is the photo and its strong bump. `GN_ground_soil` is just as flat (the sample: no point raised). Move the ramp's black stop down, for example to 0.3, for the light leaves and pebbles to rise.

The settings are `SOIL` in `scripts/common/terrain.py`. `BARE` there picks the look the yard and the far bank wear.

**The photo's licence**: it came from the web with a hashed name, like the spiky one. Check it before anything is published.

**Planned**: the S5 burned ground (Poly Haven `burned_ground_01`, environment.md) belongs here with the light states.

**In the game**:
- The ground and the paths are baked with their palette mix, on a box projection from above. The lake gets its own dark, smooth material (`game/scripts/place.gd`).
- The spiky ground's spikes are geometry, so the export carries them.
- Not done yet, since the movie comes first (the user, 2026-10-20):
  - its photo in the game's material (`game.game_material` knows only Poly Haven textures)
  - the lighter 14 cm grid for phones (Detail 0), which the user asked for
