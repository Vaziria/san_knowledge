# Texture library: stone

`stone.blend`, one family of the texture library. Built by `run.ps1 texture stone` (`scripts/textures/build.py`, the materials from `scripts/common/textures.py` and `polyhaven.py`). Each material is an asset in the catalog **Bargawa/Stone**. The collection `Samples` holds a block and a post of each.

This is not `blender/objects/stone.blend`, which holds the chipped stone pieces (umpak, the pendopo's stair) that use these materials.

| Material | Used on | Poly Haven (CC0) | Tile | Palette mix | Palette | Bump | How |
|---|---|---|---|---|---|---|---|
| `M_stone` | plinths, the lake steps, the gate, hut steps | `rock_surface` | 2.0 m | 30 % | #807C74 | 0.4 | box-projected in object space |
| `M_stone_kit` | the chipped stone blocks (umpak, the pendopo's stair) | `rock_surface` | 1.4 m | 35 % | #88847C | 0.35 | box-projected; **worn edges lighter** where the mesh's `wear` attribute is high (rounded edges, chip facets), 0.45 of #CECAC0 |
| `M_stone_dark` | andesite: the sanggar pamujan, umpak, foot courses, hut bases | `rock_wall_08` | 1.6 m | 25 % | #565450 | 0.5 | box-projected |

**Box projection**: the texture is laid from the object's own axes. Scaling an object scales its texture too, and a rotated object takes its texture with it.

**In the game**: `common/game.py` bakes each texture with its palette mix into one image, on a box projection of the same tile size. The wear tint and the bump are left out for now. `rock_surface` reads as an even grey at these tiles, in Blender as in the game.
