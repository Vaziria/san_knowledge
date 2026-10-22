# Texture library: palette

`palette.blend`, the plain colours of the look-development palette, one family of the texture library. Built by `run.ps1 texture palette` (`scripts/textures/build.py`, the colours from `PALETTE` in `scripts/common/mesh.py`). Each material is an asset in the catalog **Bargawa/Palette**. The collection `Samples` holds a block and a post of each.

The palette is stylized and slightly desaturated, picked by eye in sRGB. The textured families mix these colours in too, so the palette, not the photos, sets the mood.

| Material | Colour | Roughness | Used on |
|---|---|---|---|
| `M_ridge` | #463828 | 0.9 | ridge caps |
| `M_bamboo` | #AA9262 | 0.8 | bamboo rails, fences, the kandhang, benches |
| `M_bamboo_culm` | #768042 | 0.6 | living bamboo culms |
| `M_door` | #1E1610 | 0.9 | dark openings: the sanggar's niche, the stove's mouths |
| `M_clay` | #9E5C3C | 0.8 | pots, the kendi, the gentong |
| `M_clay_dark` | #6E422C | 0.8 | the stove (luweng) |
| `M_mat` | #B09C70 | 0.9 | pandan mats |
| `M_soot` | #2C2622 | 0.7 | blackened pots, the wok |
| `M_oil` | #2E220E | 0.05 | coconut oil in the lamp's dishes, dark and glossy |
| `M_wick` | #A8926A | 0.8 | twisted cotton, soaked with oil (clean white glared) |
| `M_mountain` | #606E7C | 1.0 | the far mountains, in haze |
| `M_leaf` | #344E28 | 0.8 | leaves |
| `M_leaf_dark` | #22361E | 0.8 | leaves, and the dark inner leaf masses |
| `M_leaf_light` | #5C7634 | 0.8 | leaves |
| `M_dry_leaf` | #80623C | 0.9 | dead banana leaves |
| `M_banana_stem` | #96A060 | 0.7 | banana stems |
| `M_grass` | #547034 | 0.85 | grass |
| `M_grass_dark` | #3E5A2A | 0.85 | grass |
| `M_grass_dry` | #8C884C | 0.9 | dry grass |
| `M_plume` | #D6CEBC | 0.9 | glagah plumes |
| `M_reed_plume` | #96786C | 0.9 | reed plumes |
| `M_lotus` | #DE96AA | 0.6 | lotus flowers |
| `M_lotus_light` | #F4CED6 | 0.6 | lotus flowers, light |
| `M_lotus_pad` | #3A5C32 | 0.4 | lotus pads |
| `M_flower` | #F0ECE2 | 0.7 | jasmine and kamboja flowers |
| `M_flower_center` | #F0C450 | 0.7 | kamboja and lotus centres |
| `M_figure` | #C85028 | 0.7 | scale figures (review only) |

**To change a colour**: change `PALETTE` in `mesh.py`, then run `run.ps1 texture palette`, then rebuild what uses it. The textured families (wood, stone, plants, ground) mix the palette colours of their own keys, so rebuild them too after changing one of those.
