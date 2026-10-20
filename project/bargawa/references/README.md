# References: doing it by hand

Step-by-step guides for building what the scripts build, by hand in Blender, for learning and for manual edits (the user, 2026-10-18). Each guide uses the **same numbers as the scripts**, so your hand-made result can be compared with the script's model.

**Both of us learn from this folder** (the user: "and you can learn from that too"):
- **The user** follows the guides by hand in Blender.
- **Claude** reads them before building or changing a model. They hold the method, the numbers and the pitfalls the builds taught us.
- The user can drop their own references here too: tutorial notes, links, pictures, things learned by hand. Claude reads and uses them.
- The knowledge graph tracks every `.md` here, so `knowledge_explain` finds them.

## How to use a guide

- Every step says **what** the tool is, **why** we use it, **where** it is in the menus (shortcut in brackets), and **what you should see** afterwards.
- Work in a new file of your own, for example `project/bargawa/blender/practice/pendopo_manual.blend` (git-ignored like every .blend). Never edit the generated files in `blender/buildings/` or `blender/places/`: the scripts overwrite them.
- To compare with the script's model, bring it into your file as a reference:
  1. **File › Append**, then open `blender/buildings/pendopo.blend` › `Collection` › `Pendopo`.
  2. In the Outliner, hide it with the eye icon to work, and show it to compare.
- Blender 5.2, metric units, metres. X is east, Y is north, Z is up; the building faces south (−Y).
- Where the script does something you can't easily do by hand, the guide says so and gives the simpler manual way.

## Guides

| # | Guide | What you learn | Status |
|---|---|---|---|
| 01 | [Modelling the pendopo](01-modelling-pendopo.md) | Exact sizes, origin and scale, Edit Mode, Extrude, Inset, Mirror / Array / Solidify modifiers, snapping, a hip roof, the tumpang sari ceiling | written |
| 02 | [Texturing with Poly Haven](02-texturing-poly-haven.md) | Downloading PBR maps, the Shading workspace, Image Texture with Box projection (no UV map needed), Mapping scale, roughness, bump, mixing in a palette colour | written |
| 03 | Linking a building into a place | Link vs Append, collection instances, reloading a library | planned |
| 04 | Grass with particles | Hair particles, instancing a collection, density from a vertex group, orientation | planned |
| 05 | Plants: leaf clumps | A stylized tree from blobs and leaf cards | planned |
| 06 | Light and mood | Sun, sky, the seven light states of the hermitage | planned (after milestone 5) |
| 07 | [Modelling the damar (oil lamp)](07-modelling-damar.md) | Screw modifier (turned shapes), Bezier curves with a bevel, Proportional Editing, Solidify, Simple Deform, a moving flame drawn by its shader on crossed cards, a flickering light (drivers), a collection instance | written |
| 08 | [Fire samples (wood fire, embers, sparks)](08-fire-samples.md) | A wood fire drawn by its shader (rising turbulence over a fading envelope), the Add Shader, sparks in Geometry Nodes from the time alone, glowing cracks with Voronoi Distance to Edge, a painted attribute a material reads, a light that flickers at the fire's rate and sways | written |

## External references

Videos, articles and pictures the user added, summarised: what they show, and what we can use.

| Reference | What it shows | For |
|---|---|---|
| [Modular stone floor and wall (brainchildpl, YouTube)](video-modular-stone-wall-brainchild.md) | Sculpt high-poly stones, model low-poly, bake, texture in Substance, assemble and light in Unreal | Stylized chipped stone for the shrine, gate, umpak and steps; a modular stone kit; baking in Blender |
| [Beautiful fire in Blender (Max Hay, YouTube)](video-fire-max-hay.md) | Fire footage on planes keyed to alpha and tinted, a point light the fire's size, Volume Scatter haze, sparks from particles in a turbulence field, motion blur, copies offset in time | The damar's flames and the fire samples in `fire.blend` (done, drawn by the shader since stock video is out; sparks in Geometry Nodes); haze in the light states |
| [Wood plank material breakdown (Javad Rajabzade, YouTube)](video-wood-plank-material-javad.md) | A weathered wood-plank smart material in Substance Painter, layer by layer: fibres, gradient re-colour, plank generator and variation, gaps, worn edges, dirt, stains, roughness, colour balance | Layered teak in Blender's shader nodes; tone per piece; worn and dirty timber; the wood's ages across the light states |

## Keeping them true

When a script changes (for example `scripts/common/joglo.py`), the guide that mirrors it is updated in the same step, so the numbers here always match the built files.
