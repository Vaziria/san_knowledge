# E01–E03 Pertapan Jatisrana, the game version

Part of the [model design](index.md). The place's design: [environment.md](../../../../../docs/stories/ramabargawa/environment.md). The built Blender place: `blender/places/jatisrana.blend` (`run.ps1 place jatisrana`). It links the pendopo, the damar and the stone pieces, and has:
- the terrain and the lake
- the huts, the pawon, the sanggar pamujan, the gate, the kandhang and the lake steps
- the plants and grass
- Poly Haven textures
- the S1 dressing

## From Blender to Godot

A new job, `run.ps1 game jatisrana`, exports the game version from the same Blender files. Nothing in the game is edited by hand: change the place, then export again. Endless Samurai's arena did the same: a GLB, plus a JSON file of placements that `arena.gd` turned into MultiMeshes.

| Part | Game version |
|---|---|
| **Buildings** | one GLB each: pendopo, two huts, pawon, sanggar, gate, kandhang, steps. Textures 1K, 2K for the soko and umpak seen close |
| **Terrain** | a mesh in chunks: fine near the hermitage, coarse far away. One shader mixes four ground textures (soil, grass, path, mud) from a painted mask |
| **Plants** | one GLB per kind (banyan, bamboo, banana, kamboja, jasmine, lotus, glagah, reeds, tree ferns), each with simpler distant versions. Their placements go in `jatisrana.json`, and Godot draws them as MultiMeshes in chunks |
| **Grass** | cards within about 25 m of where the player can walk, fading out with distance; beyond that the ground texture carries it |
| **Far forest and mountain** | flat billboards and a backdrop in the haze, one draw call per chunk |
| **Lake** | a flat surface with a water shader: dark and reflective (a reflection probe and fresnel), with ripples, and rain rings in S6 |
| **Colliders** | simple shapes made from the JSON: boxes for the buildings, cylinders for the soko and trunks, a height map for the ground |

**Where the player can walk**:
- the clearing, the paths, the steps, the bathing stone, and the shore below the steps
- the forest edge and the water are invisible walls
- the gate lets him out only in S7

## The seven light states in Godot

- **One scene per state**, S1 to S7, all using the same place. Each scene has:
  - its sky, fog, glow and colour grade (WorldEnvironment)
  - the sun or the moon, with shadows
  - its own baked indirect light (LightmapGI)
  - its own set of variants switched on and off
- **The colours** are the colour script of environment.md: warm, then cold, then the hot red of S7.
- **Godot's Mobile renderer has no volumetric fog.** So mist is depth and height fog, plus soft mist cards near the camera.
- **Wind** sways the plants in their shader, with one strength per state. A single global setting stops it at the key moments (S3 after the order, S5, the end of S6).

## Story variants still to build in Blender

These are environment milestone 6, not built yet:

| Variant | From |
|---|---|
| the burned pendopo: charred posts, the roof partly gone | S5 |
| the banyan scorched on one side | S5 |
| ash on the ground (Poly Haven `burned_ground_01`), smoke columns | S5 |
| the kandhang's broken fence and open gate | S5 |
| the lotus closed (S3), loose petals on the water (S6) | S3, S6 |
| the jasmine's fallen flowers (S4), the kamboja's flowers falling (S5–S6) | S4–S6 |
| the pyre's place on the shore below the steps | S6 |

## Budget for the place

- About 250k triangles in view and 200 draw calls, leaving room for the characters.
- Textures 1K, 2K for the hero surfaces.
- Only one state's scene is loaded at a time.
- The Blender place has a lot of plants and grass for film. How many the game can afford is measured at the first export.
