# 2026-10-20: The texture library (blender/textures)

## Request (the user's words)

| Request | The user said |
|---|---|
| Reusable textures | "lets focus on reusable texture place in blender/textures" |
| How to split them | "By family" (asked) |
| The Asset Browser | "Yes, register it" (asked) |

The workflow's model design has a "Texturing" state. Until now every builder (stone, damar, fire, pendopo, Jatisrana) made the whole palette again inside its own file with `mesh.mats()`:
- about 45 materials each time
- so a place held copies (a local `M_teak` beside the pendopo's linked one)
- and no material could be reused by hand

## Done

- **The library**: `project/bargawa/blender/textures/`, one file per family, built by `run.ps1 texture <family>|all` ([scripts/textures/build.py](../../../../project/bargawa/scripts/textures/build.py)):

  | File | Materials |
  |---|---|
  | wood | teak, teak_dark, teak_turned, firewood |
  | stone | stone, stone_kit, stone_dark |
  | plants | bark, bark_dark, thatch, woven |
  | ground | ground, earth, water |
  | metal | bronze |
  | palette | the 27 plain colours |

  - Every material is an **asset**, in a catalog of its family (Bargawa/Wood …, `blender_assets.cats.txt`). Each has a description: its Poly Haven id, tile, palette mix, bump and layers.
  - The collection `Samples` holds a block and a post of each material.
  - The Poly Haven images are linked by relative path.
  - Built in 8 s.
- **Linked everywhere**:
  - `mesh.mats()` now links every material from the library (`textures.link_all`), and `textures.build_family` makes them inside the library files. `FAMILIES` in [common/textures.py](../../../../project/bargawa/scripts/common/textures.py) is the one list.
  - Rebuilt on it: `objects/stone`, `damar` and `fire`, `buildings/pendopo`, and `places/jatisrana`.
  - Checked inside the files: the pendopo, the damar and Jatisrana have **no local `M_` materials and no duplicates**. Jatisrana has 42 materials, all from the library. Only the fire shaders, the damar's flame and the review ground stay in their own files.
- **The asset library is registered** in the user's Blender 5.2 preferences as "Bargawa Textures" (import method Link), with `run.ps1 register` ([tools/register_library.py](../../../../project/bargawa/scripts/tools/register_library.py)). It sits beside the user's User Library and Origami. The job refuses to run while any Blender is open: an open Blender saves its own preferences on exit and would drop the library.
- **Summaries beside each file**, as the workflow asks: [wood](../../../../project/bargawa/blender/textures/wood.md), [stone](../../../../project/bargawa/blender/textures/stone.md), [plants](../../../../project/bargawa/blender/textures/plants.md), [ground](../../../../project/bargawa/blender/textures/ground.md), [metal](../../../../project/bargawa/blender/textures/metal.md), [palette](../../../../project/bargawa/blender/textures/palette.md). Each lists every material, where it is used, its source and settings, what it needs from the mesh, and its game version.
- **The guide** [references/02-texturing-poly-haven.md](../../../../project/bargawa/references/02-texturing-poly-haven.md) has a new section, "Using the texture library": the Asset Browser, dragging a material onto a model, File › Link, how to change a texture for everyone, and how to try changes in a local copy.
- **README**: the layout, `texture all` and `register` in the commands, and the build order (textures, objects, buildings, places).
- **The game**: `make check` re-exported the place from the rebuilt file. It has the same 40 materials, the same 12 textured and the same 10.7 MB, with no errors.

## Notes

- `blender/objects/ornament.blend` has no script and a `.blend1` beside it: the user's own file, saved by hand, so it was left alone.
- Jatisrana was open in the user's Blender with unsaved changes when this work started. The place was rebuilt only after Blender was closed, and after checking that the file had not been saved since the last build (20:51:08, no `.blend1`).
- The assets have no preview pictures: rendering them would mean renders, which the scripts don't do. In Blender, a preview can be made per asset (in the asset's file: right-click › Generate Preview).

## Next

- The texturing state of the model design: improve the textures themselves in the library.
  - Possible: give the stone the modular-wall reference's look, and add the burned ground for S5.
  - Planned: the metal for the kapak and the jewellery, and the game versions of the weathering and the bronze.
