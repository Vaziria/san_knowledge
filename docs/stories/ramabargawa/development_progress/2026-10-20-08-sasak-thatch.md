# 2026-10-20: The sasak thatch on every roof

## Request (the user's words)

| Request | The user said |
|---|---|
| The material | "im created better sasak roof", "im rename ground_2 to material_manual.blend", "and inside i have better sasak roof material, implement it" |
| Which roofs | "All thatch" (asked) |
| Relief | "Real displacement" (asked; the other choice was bump only) |
| Shading | "Realistic" (asked: Metallic 0, Roughness 0.85, IOR 1.5 instead of their 0.18 / 0.45 / 1.0) |

The user's `sasak` material, in `blender/textures/material_manual.blend`:
- The photo `blender/assets/photos/sasak.jpg` (384 × 672 px, golden straw in seven courses), on the plane's UVs, into Base Color.
- The photo also feeds a Bump: Strength 1.0, Distance 0.8.
- RGB to BW → Color Ramp (0 → 1) → Displacement (Scale 0.1, Midlevel 0), set to Displacement and Bump.

## Done

- **Shared builders** in `common/textures.py`, for the user's photo materials:
  - `photo_material`: a photo on the UVMap, matte, with a bump, an optional fade and an optional depth shift
  - `relief_nodes`: the user's node chain as Geometry Nodes, with a weight or a flat inner shell
  - The spiky ground now uses them too. Rebuilt, its sample is identical: the same spike heights, checksum and all.
- **`M_thatch` is the sasak photo** (`textures.thatch_material`, settings `THATCH`):
  - tile 0.686 × 1.2 m, the photo's own shape: courses of about 17 cm, straw about 7 mm
  - no palette colour
  - Roughness 0.85, Metallic 0, IOR 1.5
  - Bump 1.0, Distance 0.2 (the user's 0.8, scaled from their 4.5 m plane to this tile)
  - `reed_roof_04` left `polyhaven.TEXTURES`.
- **Straw down every slope**: `mesh._slope_uvs`.
  - Thatch faces get UVs: U along the eave, V up the slope, in metres. Each face is projected onto a 40° plane facing its own way, so one side has no seams.
  - Box projection would have turned the straw sideways on the pendopo's east and west slopes.
- **Real relief**: `GN_thatch`, a modifier after each roof's Solidify (`textures.thatch_relief`).
  - The Solidify marks its new inner shell `thatch_inner`. The nodes leave those faces alone, subdivide the rest 3 times and raise them by the user's chain.
  - The roofs are now built in faces of about 0.3 m:
    - `buildings._roof_strip` resamples its profile and columns (`ROOF_STEP`).
    - `joglo._roof_shell` has 40 points per side (was 16) and a ring every 0.3 m (`_dense_rings`).
    - The shapes are the same.
  - Roofs: the pendopo, both huts, the pawon, the kandhang.
- **The cut edge**: the first stills showed the eaves' cut edge in vertical bands, like book spines, because Solidify only copies the UVs there. So the nodes now store `thatch_inner_depth` (0 outside, 1 underneath), and the material moves V down by depth × 0.25 m. The edge now shows straw ends.
- **Library sample**: a 1.5 m piece of thatched roof with its relief, in `plants.blend` › Samples.
- **Rebuilt**:
  - `run.ps1 texture plants` and `texture ground`
  - `run.ps1 building pendopo`
  - `run.ps1 place jatisrana`: done once with the new roofs. The last run was refused because the user had the file open in Blender, but it wasn't needed: the cut-edge fix is all in the library, which the place links.
- **Docs**:
  - [plants.md](../../../../project/bargawa/blender/textures/plants.md)
  - [jatisrana.md](../../../../project/bargawa/blender/places/jatisrana.md), "Texturing"
  - [references/02](../../../../project/bargawa/references/02-texturing-poly-haven.md): "The sasak thatch", a note on the old thatch walk-through, and pitfalls
  - [references/01](../../../../project/bargawa/references/01-modelling-pendopo.md): finer faces and the shell group for the relief
  - the README

## Checks

- **Sample**: the outside rises 0 to 8.1 cm (2.6 cm on average). The inner shell stays exactly 0.24 m under it. V runs −0.3 to 1.27 m up the 1.5 m slope.
- **In the place** (`jatisrana.blend`, evaluated): every roof has `GN_thatch` and a UVMap, and there are no broken points.

  | Roof | Film triangles |
  |---|---|
  | Pendopo | 623 514 |
  | Sons' hut | 107 424 |
  | Elders' hut | 85 088 |
  | Pawon | 66 804 |
  | Kandhang | 32 124 |
  | **All** | **914 954** |

  The user was told about 0.9 million when choosing.
- **Small stills** (`renders/checks/jatisrana/`): `roof_eave` (new, the pendopo's south-west eave), `pendapa`, `huts`, `pawon`, `pendapa_inside`.
  - Golden courses on every slope.
  - Relief at the eave corner, and straw ends at the cut edge.
  - The courses show between the rafters from inside.

## Decisions and findings

- **Where the user's old ground material went**: in `material_manual.blend` (and its `.blend1` backup) the "Ground" material was turned into `sasak`. Its look survives as `M_ground_spiky` and `GN_ground_spiky` in `ground.blend`. The user was shown how to append both back.
- **Solidify's shell group** (Output Vertex Groups › Shell) gets weight 1 on the new inner shell (tested), and Geometry Nodes after it can read it. A shader can't read vertex groups, hence the stored `thatch_inner_depth`.
- The fresh golden straw is the photo as it is. An older, greyer roof (for the waiting years, or the burned S5 pendopo) would be a variant of this material.

## Open questions

- The user's critique of the thatch: the colour (fresh golden, or more weathered), the course size (17 cm), and how strong the relief is.
- The game, when it comes back:
  - the thatch is a photo now, so the game gets its palette colour
  - the relief would need Detail 0 there (about 915k triangles at Detail 3)

## Next

- The user's critique of the thatch and the spiky ground.
- By the workflow: lighting, then the Scene Movie Design.
