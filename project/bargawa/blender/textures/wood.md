# Texture library: wood

`wood.blend`, one family of the texture library. Built by `run.ps1 texture wood` (`scripts/textures/build.py`, the materials from `scripts/common/textures.py` and `polyhaven.py`). Each material is an asset in the catalog **Bargawa/Wood**. The collection `Samples` holds a block and a post of each.

| Material | Used on | Poly Haven (CC0) | Tile | Palette mix | Palette | Bump | How |
|---|---|---|---|---|---|---|---|
| `M_teak` | pendopo posts and beams, tumpang sari, hut frames, door leaves | `teak_veneer` | 1.2 m | 25 % | #5C3A22 | 0.15 | along each piece's grain (UVs from `mesh.Parts`), each piece its own tone; **weathered**: the layers of the [wood plank video](../../references/video-wood-plank-material-javad.md) (gradient re-colour, fibres, worn edges, dirt from the foot, damp streaks, roughness) |
| `M_teak_dark` | rafters, carved lips, kili, door frames | `teak_veneer` | 1.2 m | 50 % | #3A2416 | 0.15 | as teak, darker |
| `M_teak_turned` | small turned, oiled wood (the damar's stand) | `teak_veneer` | 0.4 m | 55 % | #402818 | 0.05 | along the grain, finer; **no weathering** (handled, oiled household wood) |
| `M_firewood` | logs in the fire samples, the woodpile | `bark_brown_01` | 0.4 m | 30 % | #604C3A | 0.4 | box-projected in object space; `fire.charred` burns it where the mesh's `wear` is high |

**The timber materials need**:
- the mesh's `UVMap` along the grain
- `EdgeUV` (for the worn edges)
- a `tone` attribute per piece

`mesh.Parts` makes all three for every face with a `M_teak*` material. On a mesh without them the texture runs the wrong way.

**In the game**: `common/game.py` bakes each texture with its palette mix into one image, on the grain UVs. The weathering layers and the bump are left out for now.
