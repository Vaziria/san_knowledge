# Texture library: metal

`metal.blend`, one family of the texture library. Built by `run.ps1 texture metal` (`scripts/textures/build.py`, the material from `scripts/common/textures.py`, `aged_metal`). The material is an asset in the catalog **Bargawa/Metal**. The collection `Samples` holds a block and a post.

| Material | Used on | Base | How |
|---|---|---|---|
| `M_bronze` | the damar (Jamadagni's oil lamp) and its dishes | bare bronze #946636, roughness 0.3 | old cast bronze, procedural, no image: a green-grey **patina** (#3E422E) gathered in the hollows (Ambient Occlusion) and in mottled patches; **polished** bright bronze (#DAA862) where hands and cloths rub, from the mesh's `wear` attribute (rims, the socket's lip); patina rougher and less metallic |

`aged_metal(metal, patina, polish, mottle, roughness)` makes other old metals from the same recipe. **Planned**:
- the iron of the kapak's blade
- the brass and gold of the jewellery (the Wilwatikta museum's bronzes and gold pieces, [docs/reference/general/wilwatikta-museum.md](../../docs/reference/general/wilwatikta-museum.md))

**The material needs**: the mesh's `wear` attribute (0 to 1) for the polish. Without it the metal is patinated all over.

**In the game**: a plain colour so far (the bronze's base). Godot has no Ambient Occlusion node: the patina will have to be baked.
