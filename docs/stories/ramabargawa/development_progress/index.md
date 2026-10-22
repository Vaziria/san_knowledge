# Ramabargawa: Development Progress

A note for every piece of development work on the Ramabargawa story, so the knowledge is recorded and the next session can pick up from here (the user, 2026-10-18). One file per session, named `YYYY-MM-DD-NN-topic.md`, newest at the top of the list below.

## Current state

- **Phase**: building the hermitage, Pertapan Jatisrana: setting and models first, cameras later.
- **Built** (scripts in `project/bargawa/scripts/`, see its README):
  - `blender/objects/stone.blend`: the chipped stone pieces (umpak, stair) in their own file (`run.ps1 object stone`); buildings link them.
  - `blender/objects/soko.blend`: the pendopo's teak posts, 4 soko guru and 4 soko penanggap, in their own file (`run.ps1 object soko`, 2026-10-21); the pendopo links them onto their umpak.
  - `blender/objects/damar.blend`: Jamadagni's oil lamp, a bronze pole lamp with four flames (`run.ps1 object damar`); the place links it. Its flames move with time, after the fire video (`common/fire.py`).
  - `blender/objects/fire.blend`: fire samples to link, one collection each (`run.ps1 object fire`): the lamp flame, Renuka's cooking fire for the luweng, a campfire, smouldering embers (S5), a big fire for the pyre (S6); with sparks in Geometry Nodes ([objects/fire.md](../objects/fire.md)).
  - `blender/buildings/pendopo.blend`: the pendopo, a joglo, in its own file (`run.ps1 building pendopo`), linking the stone and the soko.
  - `blender/places/jatisrana.blend`: the hermitage, linking the pendopo (`run.ps1 place jatisrana`):
    - terrain, lake and layout
    - detailed huts, pawon, sanggar pamujan, gate, kandhang and lake steps
    - real plants (banyan, bamboo, banana, kamboja, jasmine, lotus, glagah, reeds, tree ferns) and grass
    - Poly Haven textures
    - the S1 dressing
  - Builds overwrite the real files: no test copies, no `.blend1` (the user, about disk space).
- **Designed**: the model list ([models.md](../models.md)) and the environment ([environment.md](../environment.md)).
- **Researched**: the pendopo ([buildings/pendopo.md](../buildings/pendopo.md)); the model was corrected against it (inner tumpang sari and nanasan, dudur, reng, sunduk and kili, padma umpak), its stair and soko built in chipped stone, its teak weathered after the wood reference, and its proportions fixed to the pakem soko (Kawruh Kalang umpak rule, Tamansiswa sizes).
- **Researched and built**: Jamadagni's lamp ([objects/damar.md](../objects/damar.md)), standing in the pendopo's sacred centre.
- **Next**: the pendopo's open choices (sacred centre, ornament, tiers, a stone-faced plinth); then the light states (milestone 5) or the story variants (milestone 6).
- **The movie comes first** (the user, 2026-10-20: "we focus on creating realistic movie first"). [bargawa_game.md](../../../../project/bargawa/bargawa_game.md) now runs Model Design → **Scene Movie Design** (story, camera direction) → the user's review → **Render Movie .mp4**; the game follows later.
- **The sasak thatch** (2026-10-20): the user's own roof material (their photo `sasak.jpg`) on every roof: the pendopo, the huts, the pawon and the kandhang.
  - The straw hangs down every slope.
  - Real relief by Geometry Nodes (`GN_thatch`), about 915k triangles.
  - Realistic shading.
  - Awaiting the user's critique ([note 08](2026-10-20-08-sasak-thatch.md)).
- **The bare ground** of Jatisrana: the yard round the pendopo and the huts (the grass was taken out there) and the far bank round Citrarata's spot.
  - Since 2026-10-21 it is **the user's soil** (`M_ground_soil`, `GN_ground_soil`), from their `material_manual.blend`: "this is better for jatisrana".
  - The soil's ramp raises no point, on their plane too, so its look is the photo and a strong bump.
  - Open: the relief and the shading (matte or theirs). Awaiting the user's critique ([note 21-02](2026-10-21-02-soil-ground.md)).
  - Before that it was **the spiky ground** (2026-10-20, [note 07](2026-10-20-07-spiky-ground.md)), the user's photo through a Color Ramp as height, still in the texture library (`M_ground_spiky`, `GN_ground_spiky`).
- **The game** (since 2026-10-20): an Android narrative adventure of Chapter 1 (beats 1–3 at Jatisrana) in Godot 4, after the user's workflow in [bargawa_game.md](../../../../project/bargawa/bargawa_game.md).
  - The requirements ([general.md](../../../../project/bargawa/docs/general.md), the user's file) are accepted.
  - Design v1 is accepted (2026-10-20): [the preview](../../../../project/bargawa/preview/design.html), the [story design](../../../../project/bargawa/docs/summaries/story.md), and the [model design](../../../../project/bargawa/docs/summaries/models/index.md).
  - **Milestone 1 done**: the place exported for the game (`run.ps1 game jatisrana`) and walked in Godot 4.7.2 (`project/bargawa/game/`) at about 128 fps on this PC.
  - `make play` (in `project/bargawa`) plays it; the Makefile wraps the game jobs of `scripts/run.ps1`.
  - **Next, by the updated workflow**: the model design in Blender (proportion and size, texturing, lighting, a model summary) for the user to accept, starting with Ramabargawa. References go in `project/bargawa/docs/reference/general/`.
  - **Proportion and size of Jatisrana checked** (`run.ps1 proportions jatisrana`). The summary is beside the place: [blender/places/jatisrana.md](../../../../project/bargawa/blender/places/jatisrana.md).
    - Six findings, the biggest that the planned characters are too tall for the huts.
    - Fixed with the user's OK: people nearer Javanese size (Ramabargawa 1.80 m, Renuka 1.55 m), 25 equal lake steps, an even sanggar stair, the kendi halved, the kamboja 4.6 m, the porch posts beside the doors, and the sons' hut porch raised to 1.70 m.
    - Reference added: [the Wilwatikta (Majapahit) museum](../../../../project/bargawa/docs/reference/general/wilwatikta-museum.md).
  - **The texture library**: `blender/textures/<family>.blend` (wood, stone, plants, ground, metal, palette), built by `run.ps1 texture all`.
    - Every file links its materials from it (build order: textures, objects, buildings, places).
    - It is registered in Blender's Asset Browser as "Bargawa Textures".
    - The summaries are beside the files.

## Notes

- [2026-10-21-02: The user's soil on Jatisrana's bare ground](2026-10-21-02-soil-ground.md)

- [2026-10-21-01: The soko in their own file (soko.blend)](2026-10-21-01-soko-file.md)

- [2026-10-20-08: The sasak thatch on every roof](2026-10-20-08-sasak-thatch.md)

- [2026-10-20-07: The spiky ground (the user's photo, in Jatisrana)](2026-10-20-07-spiky-ground.md)

- [2026-10-20-06: The texture library (blender/textures)](2026-10-20-06-texture-library.md)

- [2026-10-20-05: Proportion and size check of Jatisrana, and the Wilwatikta reference](2026-10-20-05-proportion-check.md)

- [2026-10-20-04: make play, and the updated workflow](2026-10-20-04-makefile-and-new-flow.md)

- [2026-10-20-03: The game's first walk (milestone 1)](2026-10-20-03-game-first-walk.md)

- [2026-10-20-02: The Bargawa game, design v1 (Chapter 1)](2026-10-20-02-game-design.md)
- [2026-10-20-01: The Bargawa game, first requirements](2026-10-20-01-game-requirements.md)
- [2026-10-18-16: The fire samples (fire.blend)](2026-10-18-16-fire-samples.md)
- [2026-10-18-15: Fire after Max Hay's video, on the damar's flames](2026-10-18-15-fire-reference.md)
- [2026-10-18-14: The damar, Jamadagni's oil lamp, built](2026-10-18-14-damar-built.md)
- [2026-10-18-13: The stone pieces in their own file](2026-10-18-13-stone-objects.md)
- [2026-10-18-12: Research for the Javanese lamp (damar)](2026-10-18-12-damar-research.md)
- [2026-10-18-11: The pendopo's proportions, fixed to the pakem](2026-10-18-11-pakem-proportions.md)
- [2026-10-18-10: The soko's wood, textured correctly](2026-10-18-10-teak-grain.md)
- [2026-10-18-09: Pakem soko, the post sizes](2026-10-18-09-pakem-soko.md)
- [2026-10-18-08: The soko separated, their umpak in chipped stone](2026-10-18-08-soko-separate-stone.md)
- [2026-10-18-07: The pendopo's stair, in chipped stone](2026-10-18-07-stone-stair.md)
- [2026-10-18-06: References for doing it by hand](2026-10-18-06-references.md)
- [2026-10-18-05: Pendopo corrected against the research](2026-10-18-05-pendopo-corrections.md)
- [2026-10-18-04: Pendopo research](2026-10-18-04-pendopo-research.md)
- [2026-10-18-03: The hermitage in detail (buildings, plants, grass, textures)](2026-10-18-03-hermitage-detail.md)
- [2026-10-18-02: The hermitage, first build (setting and models)](2026-10-18-02-hermitage-first-build.md)
- [2026-10-18-01: Research, model breakdown, environment design](2026-10-18-01-research-and-design.md)
