# Texture library: plants

`plants.blend`, one family of the texture library: bark, and the plant materials that buildings are made of. Built by `run.ps1 texture plants` (`scripts/textures/build.py`, the materials from `scripts/common/textures.py`, `polyhaven.py` and `mesh.woven_material`). Each material is an asset in the catalog **Bargawa/Plants**. The collection `Samples` holds a block and a post of each, and a 1.5 m piece of thatched roof with its relief.

| Material | Used on | Source | Tile | Palette mix | Palette | Bump | Note |
|---|---|---|---|---|---|---|---|
| `M_bark` | the banyan, the kamboja, the forest trees | Poly Haven `tree_bark_03` (CC0) | 1.8 m | 55 % | #5C5246 | 0.5 | at 20 % of the palette colour the banyan's grey bark almost glowed |
| `M_bark_dark` | the banyan's hanging roots | Poly Haven `bark_platanus` (CC0) | 1.4 m | 20 % | #3E362E | 0.4 | |
| `M_thatch` | alang-alang roofs (pendopo, huts, pawon, kandhang) | the user's sasak photo, `blender/assets/photos/sasak.jpg` | 0.686 × 1.2 m, on UVs down each slope | none (movie first, realistic) | | 1.0, distance 0.2 | the user's own material (2026-10-20), with real relief, `GN_thatch`; before, Poly Haven `reed_roof_04` |
| `M_woven` | gedhek, the woven bamboo walls | Poly Haven `bamboo_wall_03` (CC0) | 1.0 m | 15 % | #AA9C82 | 0.3 | without the texture files, a two-tone checker |

The bark and the woven walls are box-projected in object space.

## The sasak thatch

The user's `sasak` material from `material_manual.blend`, with the realistic values the user chose: Roughness 0.85, Metallic 0, IOR 1.5. It comes in two parts:
- **`M_thatch`**: the photo on the mesh's `UVMap` in metres.
  - `mesh._slope_uvs` gives every roof face U along the eave and V up the slope, so the straw hangs down on all four sides of a hip roof. Box projection turned it sideways on two of them.
  - V drops by the `thatch_inner_depth` attribute × 0.25 m, so the photo runs on down the cut edge of the thickened roof as straw ends.
- **`GN_thatch`**, the relief: a Geometry Nodes modifier after the roof's Solidify (`textures.thatch_relief`).
  - The Solidify marks its inner shell `thatch_inner`. The nodes subdivide the rest 3 times (Detail 3: faces of 0.3 m become 3.75 cm), then raise each point along its normal by Color Ramp(photo, 0 → 1) × Height 0.1 m (Midlevel 0).
  - They store `thatch_inner_depth` for the material.
  - The underside stays as it is.

**What it needs from the mesh**:
- a shell in M_thatch with faces of about 0.3 m (`ROOF_STEP` in `buildings.py` and `joglo.py`)
- a Solidify
- `textures.thatch_relief`

**Cost**: about 915 000 triangles over the roofs of Jatisrana, as the user chose (the pendopo 624 000). How to build it by hand: [references/02](../../references/02-texturing-poly-haven.md), "The sasak thatch". The settings are `THATCH` in `scripts/common/textures.py`.

**In the game**:
- `common/game.py` bakes each Poly Haven texture with its palette mix into one image, on a box projection of the same tile size. The bump is left out for now.
- Not done yet, since the movie comes first:
  - The thatch is now a photo, so the game gets its palette colour.
  - Its relief would need Detail 0.

The leaves, grass, flowers and plumes are plain colours, in [palette](palette.md).
