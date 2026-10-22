# 2026-10-20: The spiky ground (the user's photo, in Jatisrana)

## Request (the user's words)

| Request | The user said |
|---|---|
| The look | Built by hand in `blender/textures/Ground_2.blend`: a photo as Base Color and, through a Color Ramp, as displacement on a 64 × 64 plane. Then "no i like this spiky" |
| Use it in Jatisrana | "can our jatisrana ground make from this ?", then "a": dense patches where the camera comes close (not bump only over the whole valley) |
| Where | "Far bank, Pendopo yard" (asked) |
| Height source | "My photo" (asked; the other choice was a Poly Haven height map) |
| The game | "Yes, lighter" (asked: real geometry, a 14 cm grid on phones) |
| The workflow | "im change bargawa_game.md we focus on creating realistic movie first" |

## The workflow changed

`project/bargawa/bargawa_game.md` now has a movie branch: Model Design and Story Design → **Scene Movie Design** (story, camera direction) → the user's review → **Render Movie .mp4**. The game implementation is still in the diagram, but the movie comes first.

What that changed here:
- The spiky ground is built at film quality.
- Its photo goes in as it is, without the palette colour the other textures mix in.
- The game side is left for later.

## Done

- **The photo** was copied from Downloads into the project, as `project/bargawa/blender/assets/photos/ground_spiky.jpg` (kept in git). Before that, `Ground_2.blend` only held its path, so deleting the download would have broken it. The user was told to pack it or point it at the copy.
- **The library** (`run.ps1 texture ground`), in `blender/textures/ground.blend`:
  - `M_ground_spiky`, the material:
    - the photo on the mesh's UVs, in tiles of the photo's own shape (2.86 × 4.5 m), mirrored
    - matte: Roughness 0.85, Metallic 0, IOR 1.5
    - a bump from the same ramp as the spikes
    - see-through where the mesh's `fade_out` attribute is high
  - `GN_ground_spiky`, the spikes: the user's node chain as Geometry Nodes.
    - Subdivide Mesh (Detail), then Set Position along the normal by (Color Ramp(photo) − Midlevel) × Height × the `spikes` vertex group.
    - Ramp 0 → 0.427, Height 0.1 m, Midlevel 0: the user's own numbers.
  - A sample patch in `Samples`.
  - Summary: [ground.md](../../../../project/bargawa/blender/textures/ground.md).
- **Jatisrana** (`run.ps1 place jatisrana`, 8 s):
  - `SpikyGround_Yard`: round the pendopo with its stair and the two huts. 14 755 faces, 118 040 triangles in the film.
  - `SpikyGround_FarBank`: about 5 m round Citrarata's spot. 2 920 faces, 23 360 triangles.
  - The patches lie 2 cm over the terrain's own triangles, found by rays.
  - Their last metre fades out, with wavy edges.
  - No spikes by the paths or at the water's edge, and no faces under the buildings.
  - The grass is taken out of the yard.
  - Summary: [jatisrana.md](../../../../project/bargawa/blender/places/jatisrana.md), "Texturing".
- **Code**:
  - `common/jatisrana.py`: `YARD`, `yard()`, `far_bank()`
  - `common/terrain.py`: `SPIKY`, `spiky_material`, `spiky_patch`, `spiky_ground`, `spiky_sample` (its Geometry Nodes come from `textures.relief_nodes` since note 08)
  - `common/textures.py`: `ground_spiky` in `FAMILIES`, `link_group`
  - `common/grass.py`: no grass in the yard
  - `common/paths.py`: `PHOTOS`
  - `textures/build.py`: the sample
  - `places/jatisrana.py`
  - `tools/views.py`: new views `yard` and `far_bank`
- **The guide** [references/02](../../../../project/bargawa/references/02-texturing-poly-haven.md) has a new section, "The spiky ground": the user's shader version, the project's Geometry Nodes version step by step, the vertex group to paint, the material. It also has new pitfalls.

## Checks

- **Same spikes as the user's.** Both were rendered from the same camera: the user's plane with its shader displacement, and a copy with `GN_ground_spiky` on the same grid. The peaks stand in the same places at the same heights. The user's plane looks darker and shinier because of its Metallic 0.159, IOR 1.0 and Roughness 0.48.
- **The nodes read the photo as the shader does**: linear colour, luminance, mirrored tiles. Correlation with the sampled photo: 0.97.
- **In the place**:
  - Both patches link their group and material from the library, and the photo's path is relative.
  - Points within 0.65 m of a path stay at the 2 cm lift (the paths lie at 4 cm).
  - The grass weight in the yard is 0.
- **Small stills** (`renders/checks/jatisrana/`: `yard`, `far_bank`, `huts`):
  - The yard reads as dark, rough soil round the pendopo.
  - The far bank shows a dark patch round Citrarata's spot that fades into the ground.

## Decisions and findings

- **Geometry Nodes, not the Displace modifier.** The old Displace modifier's texture Color Ramp reads the image's alpha, not its brightness: every point went to full height. Shader displacement (the user's version) doesn't move the mesh, so the game export would not see it.
- **Blender 5.2**: a Geometry Nodes modifier's inputs are set as `md.properties.inputs.<Socket_N>.value` (no longer `md["Socket_N"]`). The game export will need this for Detail 0.
- **The photo is dark** (average linear brightness 0.05). With the ramp's white at 0.427, most spikes are 1–2 cm and the brightest reach 10 cm, as on the user's plane.

## Open questions

- The patches read darker than the terrain's ground texture round them, which is clearest on the far bank. The user may want the photo lighter (a Brightness/Contrast in the material), a wider fade, or the terrain's ground brought closer to the photo.
- The photo's licence: it came from the web (a hashed file name in Downloads). Check it before anything is published.
- The yard took out the grass round the pendopo and the huts. Its size is `J.YARD` and `YARD_ROUND`.

## Next

- The user's critique of the spiky ground.
- The game side, when the game comes back. Already in `scripts/game/jatisrana.py`: the patches count as ground with no collider, since the player walks on the terrain under them. Still to do:
  - the photo in `game.game_material`
  - Detail 0 on the patches in `scripts/game/jatisrana.py` (the lighter grid the user asked for)
  - a fresh `run.ps1 game jatisrana` (the current export predates the patches)
- By the new workflow: the Scene Movie Design (story, camera direction) after the model design.
